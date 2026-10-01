package pricing

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	kindCosmosDB = "cosmosdb"

	cosmosServiceName      = "Azure Cosmos DB"
	cosmosAccountSegment   = "cosmosdb/account"
	canonicalCosmosAccount = "cosmosdb/Account"

	cosmosProductProvisioned = "Azure Cosmos DB"
	cosmosProductAutoscale   = "Azure Cosmos DB autoscale"
	cosmosProductServerless  = "Azure Cosmos DB serverless"

	cosmosSKURU  = "RUs"
	cosmosSKUMRU = "mRUs"

	cosmosMeterRU         = "100 RU/s"
	cosmosMeterMulti      = "100 Multi-master RU/s"
	cosmosMeterStored     = "Data Stored"
	cosmosMeterServerless = "1M RUs"
	cosmosMeterPerMinute  = "1000 RU/m"
	cosmosAutoscaleSuffix = "100 RUs"

	cosmosUnitPerHour = "1/Hour"
	cosmosUnitMillion = "1M"

	cosmosTagRUPerSecond  = "ru_per_second"
	cosmosTagRUs          = "rus"
	cosmosTagRequestUnits = "request_units"
	cosmosTagMultiMaster  = "multi_master"
	cosmosMultiTrue       = "true"

	cosmosModelManual     = "manual"
	cosmosModelAutoscale  = "autoscale"
	cosmosModelServerless = "serverless"

	cosmosComponentRU     = "ru"
	cosmosUnitsPerMillion = 1_000_000
	cosmosFreeMark        = "free"
)

// quoteCosmosDB prices a Cosmos DB account from the retail page for service
// Azure Cosmos DB. Manual provisioned throughput is the default. Serverless
// and autoscale are selected with pricing_model. ArmSkuName stays empty.
func (c *Calculator) quoteCosmosDB(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	spec, err := cosmosSpecFrom(resource)
	if err != nil {
		return monthlyQuote{}, err
	}
	result, err := c.fetchServicePrices(ctx, resource, cosmosServiceName, taskID)
	if err != nil {
		return monthlyQuote{}, err
	}
	if spec.model == cosmosModelServerless {
		return cosmosServerlessQuote(resource, spec, result)
	}
	return cosmosProvisionedQuote(resource, spec, result)
}

type cosmosRequest struct {
	region       string
	model        string
	ru           float64
	sizeGB       float64
	requestUnits float64
	multi        bool
}

func cosmosSpecFrom(resource *finfocusv1.ResourceDescriptor) (cosmosRequest, error) {
	model, err := cosmosPricingModel(resource.GetTags()[tagPricingModel])
	if err != nil {
		return cosmosRequest{}, err
	}
	if model == cosmosModelServerless {
		return cosmosServerlessSpec(resource)
	}
	return cosmosManualSpec(resource, model)
}

func cosmosPricingModel(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	switch strings.ToLower(trimmed) {
	case "":
		return cosmosModelManual, nil
	case cosmosModelServerless:
		return cosmosModelServerless, nil
	case cosmosModelAutoscale:
		return cosmosModelAutoscale, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "invalid pricing_model %q", trimmed)
	}
}

func cosmosManualSpec(resource *finfocusv1.ResourceDescriptor, model string) (cosmosRequest, error) {
	region := descriptorRegion(resource)
	sizeGB, sizeSet, err := descriptorSizeGB(resource)
	if err != nil {
		return cosmosRequest{}, status.Error(codes.InvalidArgument, err.Error())
	}
	raw, field := cosmosRawRU(resource.GetTags())
	missing := cosmosMissingProvisioned(region, raw, sizeSet)
	if len(missing) > 0 {
		return cosmosRequest{}, missingFieldsError(missing)
	}
	ru, err := cosmosPositive(raw, field)
	if err != nil {
		return cosmosRequest{}, err
	}
	return cosmosRequest{
		region: region,
		model:  model,
		ru:     ru,
		sizeGB: sizeGB,
		multi:  model == cosmosModelManual && cosmosMultiMaster(resource.GetTags()),
	}, nil
}

func cosmosMissingProvisioned(region, rawRU string, sizeSet bool) []string {
	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	if rawRU == "" {
		missing = append(missing, cosmosTagRUPerSecond)
	}
	if !sizeSet {
		missing = append(missing, "size_gb")
	}
	return missing
}

func cosmosServerlessSpec(resource *finfocusv1.ResourceDescriptor) (cosmosRequest, error) {
	region := descriptorRegion(resource)
	raw := strings.TrimSpace(resource.GetTags()[cosmosTagRequestUnits])
	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	if raw == "" {
		missing = append(missing, cosmosTagRequestUnits)
	}
	if len(missing) > 0 {
		return cosmosRequest{}, missingFieldsError(missing)
	}
	units, err := cosmosPositive(raw, cosmosTagRequestUnits)
	if err != nil {
		return cosmosRequest{}, err
	}
	return cosmosRequest{
		region:       region,
		model:        cosmosModelServerless,
		requestUnits: units,
	}, nil
}

func cosmosRawRU(tags map[string]string) (string, string) {
	if raw := strings.TrimSpace(tags[cosmosTagRUPerSecond]); raw != "" {
		return raw, cosmosTagRUPerSecond
	}
	if raw := strings.TrimSpace(tags[cosmosTagRUs]); raw != "" {
		return raw, cosmosTagRUs
	}
	return "", cosmosTagRUPerSecond
}

func cosmosPositive(raw, field string) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, status.Errorf(codes.InvalidArgument, "invalid %s %q", field, raw)
	}
	return value, nil
}

func cosmosMultiMaster(tags map[string]string) bool {
	return strings.EqualFold(strings.TrimSpace(tags[cosmosTagMultiMaster]), cosmosMultiTrue)
}

func cosmosProvisionedQuote(
	resource *finfocusv1.ResourceDescriptor,
	spec cosmosRequest,
	result azureclient.CachedResult,
) (monthlyQuote, error) {
	ruItem, block, err := cosmosRUItem(result.Items, spec)
	if err != nil {
		return monthlyQuote{}, cosmosStatus(err)
	}
	stored, err := cosmosStorageItem(result.Items, spec)
	if err != nil {
		return monthlyQuote{}, cosmosStatus(err)
	}
	if itemCurrency(stored) != itemCurrency(ruItem) {
		return monthlyQuote{}, status.Error(codes.InvalidArgument, "cosmos meters use different currencies")
	}
	ruCost := spec.ru / float64(block) * ruItem.RetailPrice * pluginsdk.HoursPerMonth
	storageCost := stored.RetailPrice * spec.sizeGB
	quote := cosmosQuote(resource, spec, ruItem, result.ExpiresAt, map[string]float64{
		cosmosComponentRU: ruCost,
		breakdownStorage:  storageCost,
	})
	quote.meters = []quoteMeter{
		{key: cosmosComponentRU, price: ruItem.RetailPrice, unit: ruItem.UnitOfMeasure},
		{key: breakdownStorage, price: stored.RetailPrice, unit: stored.UnitOfMeasure},
	}
	return quote, nil
}

func cosmosServerlessQuote(
	resource *finfocusv1.ResourceDescriptor,
	spec cosmosRequest,
	result azureclient.CachedResult,
) (monthlyQuote, error) {
	item, err := selectCosmosServerless(result.Items)
	if err != nil {
		return monthlyQuote{}, cosmosStatus(err)
	}
	cost := spec.requestUnits / cosmosUnitsPerMillion * item.RetailPrice
	quote := cosmosQuote(resource, spec, item, result.ExpiresAt, map[string]float64{
		cosmosComponentRU: cost,
	})
	quote.meters = []quoteMeter{
		{key: cosmosComponentRU, price: item.RetailPrice, unit: item.UnitOfMeasure},
	}
	return quote, nil
}

func cosmosQuote(
	resource *finfocusv1.ResourceDescriptor,
	spec cosmosRequest,
	ruItem azureclient.PriceItem,
	expiresAt time.Time,
	components map[string]float64,
) monthlyQuote {
	var monthly float64
	for _, cost := range components {
		monthly += cost
	}
	return monthlyQuote{
		unitPrice:     ruItem.RetailPrice,
		monthly:       monthly,
		currency:      itemCurrency(ruItem),
		billingDetail: cosmosBillingDetail(spec),
		components:    components,
		expiresAt:     expiresAt,
		region:        spec.region,
		sku:           ruItem.MeterName,
		resourceType:  resource.GetResourceType(),
	}
}

func cosmosRUItem(items []azureclient.PriceItem, spec cosmosRequest) (azureclient.PriceItem, int, error) {
	if spec.model == cosmosModelAutoscale {
		return selectCosmosAutoscale(items)
	}
	sku := cosmosSKURU
	meter := cosmosMeterRU
	if spec.multi {
		sku = cosmosSKUMRU
		meter = cosmosMeterMulti
	}
	return selectCosmosHourly(items, sku, meter)
}

func cosmosStorageItem(items []azureclient.PriceItem, spec cosmosRequest) (azureclient.PriceItem, error) {
	if spec.model == cosmosModelAutoscale {
		item, ok, err := selectCosmosAutoscaleStored(items)
		if err != nil || ok {
			return item, err
		}
		return selectCosmosStored(items, cosmosSKURU)
	}
	sku := cosmosSKURU
	if spec.multi {
		sku = cosmosSKUMRU
	}
	return selectCosmosStored(items, sku)
}

func selectCosmosHourly(items []azureclient.PriceItem, sku, meter string) (azureclient.PriceItem, int, error) {
	matched, err := cosmosHourlyMatches(items, sku)
	if err != nil || len(matched) == 0 {
		if err != nil {
			return azureclient.PriceItem{}, 0, err
		}
		return azureclient.PriceItem{}, 0, fmt.Errorf(
			"no cosmos price for sku %s meter %s: %w",
			sku,
			meter,
			azureclient.ErrNotFound,
		)
	}
	chosen, err := cosmosSamePrice(matched)
	if err != nil {
		return azureclient.PriceItem{}, 0, err
	}
	block, err := cosmosIntegerBlock(chosen.MeterName)
	if err != nil {
		return azureclient.PriceItem{}, 0, err
	}
	if err = cosmosSameBlock(matched, block); err != nil {
		return azureclient.PriceItem{}, 0, err
	}
	return chosen, block, nil
}

func cosmosHourlyMatches(items []azureclient.PriceItem, sku string) ([]azureclient.PriceItem, error) {
	matched := make([]azureclient.PriceItem, 0, 1)
	for i := range items {
		item := items[i]
		if !cosmosHourlyRow(item, sku) {
			continue
		}
		if _, err := cosmosIntegerBlock(item.MeterName); err != nil {
			return nil, err
		}
		matched = append(matched, item)
	}
	return matched, nil
}

func cosmosHourlyRow(item azureclient.PriceItem, sku string) bool {
	if item.ProductName != cosmosProductProvisioned || item.SkuName != sku {
		return false
	}
	if item.Type != storagePriceTypeConsumption || item.UnitOfMeasure != cosmosUnitPerHour {
		return false
	}
	// 1000 RU/m is a per-minute unit, not a block of RU/s.
	return item.MeterName != cosmosMeterPerMinute
}

func cosmosIntegerBlock(meter string) (int, error) {
	block, ok := cosmosLeadingBlock(meter)
	if !ok {
		return 0, status.Errorf(
			codes.InvalidArgument,
			"cosmos meter %q does not start with an integer",
			meter,
		)
	}
	return block, nil
}

func cosmosLeadingBlock(meter string) (int, bool) {
	end := 0
	for end < len(meter) && meter[end] >= '0' && meter[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	block, err := strconv.Atoi(meter[:end])
	if err != nil || block < 1 {
		return 0, false
	}
	return block, true
}

func cosmosSameBlock(items []azureclient.PriceItem, block int) error {
	for i := range items {
		other, err := cosmosIntegerBlock(items[i].MeterName)
		if err != nil {
			return err
		}
		if other != block {
			return status.Errorf(
				codes.InvalidArgument,
				"ambiguous cosmos price for meter %s",
				items[i].MeterName,
			)
		}
	}
	return nil
}

func selectCosmosStored(items []azureclient.PriceItem, sku string) (azureclient.PriceItem, error) {
	matched := cosmosStoredMatches(items, sku)
	if len(matched) == 0 {
		return azureclient.PriceItem{}, fmt.Errorf(
			"no cosmos price for sku %s meter %s: %w",
			sku,
			cosmosMeterStored,
			azureclient.ErrNotFound,
		)
	}
	return cosmosSamePrice(matched)
}

func cosmosStoredMatches(items []azureclient.PriceItem, sku string) []azureclient.PriceItem {
	matched := make([]azureclient.PriceItem, 0, 1)
	for i := range items {
		item := items[i]
		if item.ProductName != cosmosProductProvisioned || item.SkuName != sku || item.MeterName != cosmosMeterStored {
			continue
		}
		if item.Type != storagePriceTypeConsumption || item.UnitOfMeasure != storageUnitGBMonth {
			continue
		}
		matched = append(matched, item)
	}
	return matched
}

func selectCosmosAutoscale(items []azureclient.PriceItem) (azureclient.PriceItem, int, error) {
	matched := cosmosAutoscaleRUMatches(items)
	if len(matched) == 0 {
		return azureclient.PriceItem{}, 0, fmt.Errorf(
			"no cosmos price for meter %s: %w",
			cosmosAutoscaleSuffix,
			azureclient.ErrNotFound,
		)
	}
	chosen, err := cosmosSamePrice(matched)
	if err != nil {
		return azureclient.PriceItem{}, 0, err
	}
	block, ok := cosmosSuffixBlock(chosen.MeterName)
	if !ok {
		return azureclient.PriceItem{}, 0, status.Errorf(
			codes.InvalidArgument,
			"cosmos meter %q has no RU block",
			chosen.MeterName,
		)
	}
	return chosen, block, nil
}

func cosmosAutoscaleRUMatches(items []azureclient.PriceItem) []azureclient.PriceItem {
	matched := make([]azureclient.PriceItem, 0, len(items))
	for i := range items {
		item := items[i]
		if item.ProductName != cosmosProductAutoscale || item.Type != storagePriceTypeConsumption {
			continue
		}
		if item.UnitOfMeasure != cosmosUnitPerHour || !strings.HasSuffix(item.MeterName, cosmosAutoscaleSuffix) {
			continue
		}
		matched = append(matched, item)
	}
	return matched
}

func cosmosSuffixBlock(meter string) (int, bool) {
	if !strings.HasSuffix(meter, cosmosAutoscaleSuffix) {
		return 0, false
	}
	fields := strings.Fields(cosmosAutoscaleSuffix)
	if len(fields) == 0 {
		return 0, false
	}
	block, err := strconv.Atoi(fields[0])
	if err != nil || block < 1 {
		return 0, false
	}
	return block, true
}

func selectCosmosAutoscaleStored(items []azureclient.PriceItem) (azureclient.PriceItem, bool, error) {
	matched := cosmosAutoscaleStoredMatches(items)
	if len(matched) == 0 {
		return azureclient.PriceItem{}, false, nil
	}
	chosen, err := cosmosSamePrice(matched)
	if err != nil {
		return azureclient.PriceItem{}, true, err
	}
	return chosen, true, nil
}

func cosmosAutoscaleStoredMatches(items []azureclient.PriceItem) []azureclient.PriceItem {
	matched := make([]azureclient.PriceItem, 0, 1)
	for i := range items {
		item := items[i]
		if !cosmosAutoscaleStoredRow(item) {
			continue
		}
		matched = append(matched, item)
	}
	return matched
}

func cosmosAutoscaleStoredRow(item azureclient.PriceItem) bool {
	if item.ProductName != cosmosProductAutoscale || item.Type != storagePriceTypeConsumption {
		return false
	}
	if item.UnitOfMeasure != storageUnitGBMonth || !strings.Contains(item.MeterName, cosmosMeterStored) {
		return false
	}
	name := strings.ToLower(item.MeterName + " " + item.SkuName)
	return !strings.Contains(name, cosmosFreeMark)
}

func selectCosmosServerless(items []azureclient.PriceItem) (azureclient.PriceItem, error) {
	matched := make([]azureclient.PriceItem, 0, 1)
	for i := range items {
		item := items[i]
		if !cosmosServerlessRURow(item) {
			continue
		}
		if item.UnitOfMeasure != cosmosUnitMillion {
			return azureclient.PriceItem{}, status.Errorf(
				codes.InvalidArgument,
				"unsupported cosmos unit %q",
				item.UnitOfMeasure,
			)
		}
		matched = append(matched, item)
	}
	if len(matched) == 0 {
		return azureclient.PriceItem{}, fmt.Errorf(
			"no cosmos price for meter %s: %w",
			cosmosMeterServerless,
			azureclient.ErrNotFound,
		)
	}
	return cosmosSamePrice(matched)
}

func cosmosServerlessRURow(item azureclient.PriceItem) bool {
	if item.ProductName != cosmosProductServerless || item.SkuName != cosmosSKURU {
		return false
	}
	if item.Type != storagePriceTypeConsumption || item.MeterName == cosmosMeterPerMinute {
		return false
	}
	return !strings.Contains(item.MeterName, cosmosMeterStored)
}

func cosmosSamePrice(items []azureclient.PriceItem) (azureclient.PriceItem, error) {
	chosen := items[0]
	currency := itemCurrency(chosen)
	for _, item := range items[1:] {
		if item.RetailPrice != chosen.RetailPrice || itemCurrency(item) != currency {
			return azureclient.PriceItem{}, status.Errorf(
				codes.InvalidArgument,
				"ambiguous cosmos price for meter %s",
				chosen.MeterName,
			)
		}
	}
	return chosen, nil
}

func cosmosBillingDetail(spec cosmosRequest) string {
	switch spec.model {
	case cosmosModelServerless:
		return fmt.Sprintf("Cosmos DB serverless %g request units in %s", spec.requestUnits, spec.region)
	case cosmosModelAutoscale:
		return fmt.Sprintf("Cosmos DB autoscale %g RU/s %g GB in %s", spec.ru, spec.sizeGB, spec.region)
	default:
		kind := "provisioned"
		if spec.multi {
			kind = "multi-master"
		}
		return fmt.Sprintf("Cosmos DB %s %g RU/s %g GB in %s", kind, spec.ru, spec.sizeGB, spec.region)
	}
}

func cosmosStatus(err error) error {
	if err == nil || status.Code(err) != codes.Unknown {
		return err
	}
	return MapToGRPCStatus(err).Err()
}

// isCosmosAccountResourceType reports whether lower contains cosmosdb/account,
// including Pulumi azure:cosmosdb/account:Account.
func isCosmosAccountResourceType(lower string) bool {
	return resourceSegment(lower, cosmosAccountSegment)
}
