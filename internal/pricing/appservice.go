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
	appServiceName              = "Azure App Service"
	functionsServiceName        = "Functions"
	appServicePlanSegment       = "web/appserviceplan"
	appServicePulumiPlanSegment = "appservice/plan"
	functionAppSegment          = "web/functionapp"
	functionAppPulumiSegment    = "appservice/functionapp"
	canonicalAppServicePlan     = "web/AppServicePlan"
	canonicalFunctionApp        = "web/FunctionApp"

	appServiceUnitHour       = "1 Hour"
	appServiceMeterSuffix    = " App"
	appServiceLinuxMarker    = "linux"
	appServiceOSWindows      = "Windows"
	appServiceOSLinux        = "Linux"
	appServiceOSTag          = "os"
	appServiceOSTypeTag      = "osType"
	functionsPremiumProduct  = "Premium Functions"
	functionsProductStandard = "Functions"
	functionsMeterExecutions = "Standard Total Executions"
	functionsMeterDuration   = "Standard Execution Time"
	functionsMeterVCPU       = "Premium vCPU Duration"
	functionsMeterMemory     = "Premium Memory Duration"
	functionsUnitExecutions  = "10"
	functionsUnitGBSecond    = "1 GB Second"
	functionsUnitMemory      = "1 GiB Hour"

	// Classic Consumption grant from the Azure Functions pricing page.
	// It is per subscription per month. This plugin applies the full grant
	// to the single resource being priced and does not apply the Flex grant.
	functionsFreeExecutions = 1_000_000
	functionsFreeGBSeconds  = 400_000
	// functionsExecutionUnit is the divisor for unit "10". It is not parsed
	// from the meter. A different unit is an error.
	functionsExecutionUnit = 10

	functionsModelConsumption = "consumption"
	functionsModelPremium     = "premium"
	functionsSKUPremium       = "premium"

	kindFunctionConsumption = "function-consumption"
	kindFunctionPremium     = "function-premium"
	kindFunctionDedicated   = "function-dedicated"

	breakdownExecutions = "executions"
	breakdownGBSeconds  = "gb_seconds"
	breakdownVCPU       = "vcpu"
	breakdownMemory     = "memory"

	tagExecutions   = "executions"
	tagGBSeconds    = "gb_seconds"
	tagVCPUCount    = "vcpu_count"
	tagMemoryGiB    = "memory_gib"
	tagPricingModel = "pricing_model"
)

func (c *Calculator) quoteAppServicePlan(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	region := descriptorRegion(resource)
	sku := appServicePlanSKU(resource)
	if err := requireFields(region, sku); err != nil {
		return monthlyQuote{}, err
	}
	windows, err := descriptorWindows(resource)
	if err != nil {
		return monthlyQuote{}, err
	}
	workers, workerNote, err := appServicePlanWorkers(resource.GetTags())
	if err != nil {
		return monthlyQuote{}, err
	}

	result, err := c.fetchServicePrices(ctx, resource, appServiceName, taskID)
	if err != nil {
		return monthlyQuote{}, err
	}
	item, err := selectAppServicePlanItem(result.Items, sku, windows)
	if err != nil {
		return monthlyQuote{}, err
	}

	return monthlyQuote{
		unitPrice:     item.RetailPrice,
		monthly:       item.RetailPrice * pluginsdk.HoursPerMonth * float64(workers),
		currency:      itemCurrency(item),
		billingDetail: appServiceBillingDetail(resource, windows, sku, region, workers, workerNote),
		breakdownKey:  breakdownCompute,
		expiresAt:     result.ExpiresAt,
		region:        region,
		sku:           sku,
		resourceType:  resource.GetResourceType(),
		meters: []quoteMeter{{
			key:   breakdownCompute,
			price: item.RetailPrice,
			unit:  item.UnitOfMeasure,
		}},
	}, nil
}

func (c *Calculator) quoteFunctionApp(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	kind, err := functionQuoteKind(resource.GetTags()[tagPricingModel], descriptorSKU(resource))
	if err != nil {
		return monthlyQuote{}, err
	}
	switch kind {
	case kindFunctionConsumption:
		return c.quoteFunctionConsumption(ctx, resource, taskID)
	case kindFunctionPremium:
		return c.quoteFunctionPremium(ctx, resource, taskID)
	case kindFunctionDedicated:
		return c.quoteAppServicePlan(ctx, resource, taskID)
	default:
		return monthlyQuote{}, status.Errorf(codes.InvalidArgument, "unsupported pricing_model %q", kind)
	}
}

func (c *Calculator) quoteFunctionConsumption(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	executions, gbSeconds, err := functionConsumptionQuantities(resource.GetTags())
	if err != nil {
		return monthlyQuote{}, err
	}
	result, err := c.fetchServicePrices(ctx, resource, functionsServiceName, taskID)
	if err != nil {
		return monthlyQuote{}, err
	}

	execItem, err := selectFunctionMeter(
		result.Items, functionsProductStandard, functionsMeterExecutions, functionsUnitExecutions, true,
	)
	if err != nil {
		return monthlyQuote{}, functionPriceStatus(err)
	}
	gbItem, err := selectFunctionMeter(
		result.Items, functionsProductStandard, functionsMeterDuration, functionsUnitGBSecond, true,
	)
	if err != nil {
		return monthlyQuote{}, functionPriceStatus(err)
	}
	err = sameCurrency(execItem, gbItem)
	if err != nil {
		return monthlyQuote{}, err
	}

	execCost, gbCost := consumptionCosts(executions, gbSeconds, execItem.RetailPrice, gbItem.RetailPrice)
	quote := meterQuote(
		resource,
		descriptorSKU(resource),
		itemCurrency(execItem),
		fmt.Sprintf("Functions consumption in %s", descriptorRegion(resource)),
		execCost+gbCost,
		result.ExpiresAt,
		map[string]float64{
			breakdownExecutions: execCost,
			breakdownGBSeconds:  gbCost,
		},
	)
	quote.meters = []quoteMeter{
		{key: breakdownExecutions, price: execItem.RetailPrice, unit: execItem.UnitOfMeasure},
		{key: breakdownGBSeconds, price: gbItem.RetailPrice, unit: gbItem.UnitOfMeasure},
	}
	return quote, nil
}

func (c *Calculator) quoteFunctionPremium(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	vcpuCount, memoryGiB, err := functionPremiumQuantities(resource.GetTags())
	if err != nil {
		return monthlyQuote{}, err
	}
	result, err := c.fetchServicePrices(ctx, resource, functionsServiceName, taskID)
	if err != nil {
		return monthlyQuote{}, err
	}

	vcpuItem, err := selectFunctionMeter(
		result.Items, functionsPremiumProduct, functionsMeterVCPU, appServiceUnitHour, false,
	)
	if err != nil {
		return monthlyQuote{}, functionPriceStatus(err)
	}
	memoryItem, err := selectFunctionMeter(
		result.Items, functionsPremiumProduct, functionsMeterMemory, functionsUnitMemory, false,
	)
	if err != nil {
		return monthlyQuote{}, functionPriceStatus(err)
	}
	err = sameCurrency(vcpuItem, memoryItem)
	if err != nil {
		return monthlyQuote{}, err
	}

	vcpuCost := vcpuCount * vcpuItem.RetailPrice * pluginsdk.HoursPerMonth
	memoryCost := memoryGiB * memoryItem.RetailPrice * pluginsdk.HoursPerMonth
	quote := meterQuote(
		resource,
		descriptorSKU(resource),
		itemCurrency(vcpuItem),
		fmt.Sprintf("Functions premium in %s, 730 hrs/month", descriptorRegion(resource)),
		vcpuCost+memoryCost,
		result.ExpiresAt,
		map[string]float64{
			breakdownVCPU:   vcpuCost,
			breakdownMemory: memoryCost,
		},
	)
	quote.meters = []quoteMeter{
		{key: breakdownVCPU, price: vcpuItem.RetailPrice, unit: vcpuItem.UnitOfMeasure},
		{key: breakdownMemory, price: memoryItem.RetailPrice, unit: memoryItem.UnitOfMeasure},
	}
	return quote, nil
}

func (c *Calculator) fetchServicePrices(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	service, taskID string,
) (azureclient.CachedResult, error) {
	region := descriptorRegion(resource)
	if region == "" {
		return azureclient.CachedResult{}, missingFieldsError([]string{missingFieldRegion})
	}
	return c.fetchPrices(ctx, azureclient.PriceQuery{
		ArmRegionName: region,
		ServiceName:   service,
		CurrencyCode:  descriptorCurrency(resource),
	}, taskID)
}

// isAppServicePlanResourceType reports whether lower contains web/appserviceplan
// or the Pulumi appservice/plan segment.
func isAppServicePlanResourceType(lower string) bool {
	return resourceSegment(lower, appServicePlanSegment) ||
		resourceSegment(lower, appServicePulumiPlanSegment) ||
		resourceSegment(lower, "appservice/serviceplan") ||
		tokenSuffix(lower, "web", "appserviceplan")
}

// isFunctionAppResourceType reports whether lower contains web/functionapp or
// the Pulumi appservice/functionapp segment.
func isFunctionAppResourceType(lower string) bool {
	return resourceSegment(lower, functionAppSegment) ||
		resourceSegment(lower, functionAppPulumiSegment)
}

// isNativeFunctionWebApp reports an Azure Native WebApp whose kind is a function app.
// A WebApp without that kind is a site, not this quote.
func isNativeFunctionWebApp(lower string, tags map[string]string) bool {
	if !tokenSuffix(lower, "web", "webapp") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(tags["kind"]), "FunctionApp")
}

func functionQuoteKind(model, sku string) (string, error) {
	model = strings.TrimSpace(model)
	switch {
	case model == "":
		return functionKindFromSKU(sku), nil
	case strings.EqualFold(model, functionsModelConsumption):
		if sku == "" || functionConsumptionSKU(sku) {
			return kindFunctionConsumption, nil
		}
		return "", status.Errorf(codes.InvalidArgument, "unsupported sku %q", sku)
	case strings.EqualFold(model, functionsModelPremium):
		return kindFunctionPremium, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "unsupported pricing_model %q", model)
	}
}

func functionKindFromSKU(sku string) string {
	switch {
	case sku == "" || functionConsumptionSKU(sku):
		return kindFunctionConsumption
	case strings.EqualFold(sku, functionsSKUPremium):
		return kindFunctionPremium
	default:
		return kindFunctionDedicated
	}
}

func functionConsumptionSKU(sku string) bool {
	switch strings.ToLower(sku) {
	case "standard", "y1", "dynamic", functionsModelConsumption:
		return true
	default:
		return false
	}
}

// descriptorWindows reports whether tag os selects Windows. An empty os is
// Linux. Any other non-empty value is InvalidArgument and names that value.
// When os is empty, the classic Pulumi osType is read: Windows or Linux.
// WindowsContainer is InvalidArgument.
func descriptorWindows(resource *finfocusv1.ResourceDescriptor) (bool, error) {
	tags := resource.GetTags()
	osName := strings.TrimSpace(tags[appServiceOSTag])
	if osName == "" {
		return pulumiOSTypeWindows(pulumiTag(tags, appServiceOSTypeTag))
	}
	if strings.EqualFold(osName, appServiceOSWindows) {
		return true, nil
	}
	return false, status.Errorf(codes.InvalidArgument, "unsupported os %q", osName)
}

func pulumiOSTypeWindows(osType string) (bool, error) {
	switch {
	case osType == "" || strings.EqualFold(osType, appServiceOSLinux):
		return false, nil
	case strings.EqualFold(osType, appServiceOSWindows):
		return true, nil
	default:
		return false, status.Errorf(codes.InvalidArgument, "unsupported %s %q", appServiceOSTypeTag, osType)
	}
}

// selectAppServicePlanItem chooses the hourly Consumption plan meter.
// SKU matching ignores spaces and case. The meter is skuName or skuName plus
// " App". Linux is the default product. Stamp, SSL, Domain, and ASIP rows
// are not plan prices. A matching price of 0 is kept. Differing prices are
// an error that names the SKU.
func selectAppServicePlanItem(items []azureclient.PriceItem, sku string, windows bool) (azureclient.PriceItem, error) {
	norm := normalizeAppSKU(sku)
	var matched []azureclient.PriceItem
	for i := range items {
		if appServicePlanRow(items[i], norm, windows) {
			matched = append(matched, items[i])
		}
	}
	if len(matched) == 0 {
		return azureclient.PriceItem{}, status.Errorf(codes.InvalidArgument, "unknown app service sku %q", sku)
	}
	return samePlanPrice(matched, sku)
}

func appServicePlanRow(item azureclient.PriceItem, norm string, windows bool) bool {
	if item.Type != storagePriceTypeConsumption || item.UnitOfMeasure != appServiceUnitHour {
		return false
	}
	if appServiceExcluded(item) || normalizeAppSKU(item.SkuName) != norm {
		return false
	}
	if item.MeterName != item.SkuName && item.MeterName != item.SkuName+appServiceMeterSuffix {
		return false
	}
	return appServiceProductOS(item.ProductName, windows)
}

func appServiceExcluded(item azureclient.PriceItem) bool {
	name := strings.ToLower(item.SkuName + " " + item.MeterName)
	return strings.Contains(name, "stamp") ||
		strings.Contains(name, "ssl") ||
		strings.Contains(name, "domain") ||
		strings.Contains(name, "asip")
}

func appServiceProductOS(product string, windows bool) bool {
	linux := strings.Contains(strings.ToLower(product), appServiceLinuxMarker)
	if windows {
		return !linux
	}
	return linux
}

func normalizeAppSKU(sku string) string {
	return strings.ToLower(strings.ReplaceAll(sku, " ", ""))
}

func samePlanPrice(items []azureclient.PriceItem, sku string) (azureclient.PriceItem, error) {
	chosen := items[0]
	currency := itemCurrency(chosen)
	for _, item := range items[1:] {
		if item.RetailPrice != chosen.RetailPrice || itemCurrency(item) != currency {
			return azureclient.PriceItem{}, status.Errorf(
				codes.InvalidArgument,
				"ambiguous app service price for sku %q",
				sku,
			)
		}
	}
	return chosen, nil
}

// selectFunctionMeter applies the zero-sibling rule. A positive retailPrice
// wins over a 0 row for the same meter. Consumption meters require that
// positive row; a lone 0 is NotFound, not a free price. Premium meters may
// stay 0 when no positive sibling exists. The unit must match exactly.
func selectFunctionMeter(
	items []azureclient.PriceItem,
	product, meter, unit string,
	requirePositive bool,
) (azureclient.PriceItem, error) {
	var positives []azureclient.PriceItem
	var zeros []azureclient.PriceItem
	for i := range items {
		item := items[i]
		if item.ProductName != product || item.MeterName != meter || item.Type != storagePriceTypeConsumption {
			continue
		}
		if item.RetailPrice > 0 {
			positives = append(positives, item)
			continue
		}
		if item.RetailPrice == 0 {
			zeros = append(zeros, item)
		}
	}
	if len(positives) > 0 {
		return consistentMeter(positives, unit, meter)
	}
	if requirePositive || len(zeros) == 0 {
		return azureclient.PriceItem{}, fmt.Errorf(
			"no functions price for meter %s: %w",
			meter,
			azureclient.ErrNotFound,
		)
	}
	return consistentMeter(zeros, unit, meter)
}

func consistentMeter(items []azureclient.PriceItem, unit, meter string) (azureclient.PriceItem, error) {
	chosen := items[0]
	currency := itemCurrency(chosen)
	for _, item := range items[1:] {
		if item.RetailPrice != chosen.RetailPrice || itemCurrency(item) != currency {
			return azureclient.PriceItem{}, status.Errorf(codes.InvalidArgument, "ambiguous price for %q", meter)
		}
		if item.UnitOfMeasure != chosen.UnitOfMeasure {
			return azureclient.PriceItem{}, status.Errorf(
				codes.InvalidArgument,
				"unsupported unit %q",
				item.UnitOfMeasure,
			)
		}
	}
	if chosen.UnitOfMeasure != unit {
		return azureclient.PriceItem{}, status.Errorf(
			codes.InvalidArgument,
			"unsupported unit %q",
			chosen.UnitOfMeasure,
		)
	}
	return chosen, nil
}

func consumptionCosts(executions, gbSeconds, execRetail, gbRetail float64) (float64, float64) {
	billableExec := aboveGrant(executions, functionsFreeExecutions)
	billableGB := aboveGrant(gbSeconds, functionsFreeGBSeconds)
	return (billableExec / functionsExecutionUnit) * execRetail, billableGB * gbRetail
}

func aboveGrant(value, grant float64) float64 {
	if value <= grant {
		return 0
	}
	return value - grant
}

func functionConsumptionQuantities(tags map[string]string) (float64, float64, error) {
	execRaw := strings.TrimSpace(tags[tagExecutions])
	gbRaw := strings.TrimSpace(tags[tagGBSeconds])
	var missing []string
	if execRaw == "" {
		missing = append(missing, tagExecutions)
	}
	if gbRaw == "" {
		missing = append(missing, tagGBSeconds)
	}
	if len(missing) > 0 {
		return 0, 0, missingFieldsError(missing)
	}
	executions, err := parseNonNegative(tagExecutions, execRaw)
	if err != nil {
		return 0, 0, err
	}
	gbSeconds, err := parseNonNegative(tagGBSeconds, gbRaw)
	if err != nil {
		return 0, 0, err
	}
	return executions, gbSeconds, nil
}

func functionPremiumQuantities(tags map[string]string) (float64, float64, error) {
	vcpuRaw := strings.TrimSpace(tags[tagVCPUCount])
	memoryRaw := strings.TrimSpace(tags[tagMemoryGiB])
	var missing []string
	if vcpuRaw == "" {
		missing = append(missing, tagVCPUCount)
	}
	if memoryRaw == "" {
		missing = append(missing, tagMemoryGiB)
	}
	if len(missing) > 0 {
		return 0, 0, missingFieldsError(missing)
	}
	vcpuCount, err := parseNonNegative(tagVCPUCount, vcpuRaw)
	if err != nil {
		return 0, 0, err
	}
	memoryGiB, err := parseNonNegative(tagMemoryGiB, memoryRaw)
	if err != nil {
		return 0, 0, err
	}
	return vcpuCount, memoryGiB, nil
}

func parseNonNegative(name, raw string) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, status.Errorf(codes.InvalidArgument, "invalid %s %q", name, raw)
	}
	return value, nil
}

func sameCurrency(left, right azureclient.PriceItem) error {
	if itemCurrency(left) == itemCurrency(right) {
		return nil
	}
	return status.Error(codes.InvalidArgument, "functions meters use different currencies")
}

func functionPriceStatus(err error) error {
	if err == nil || status.Code(err) != codes.Unknown {
		return err
	}
	return MapToGRPCStatus(err).Err()
}

func meterQuote(
	resource *finfocusv1.ResourceDescriptor,
	sku, currency, detail string,
	monthly float64,
	expires time.Time,
	components map[string]float64,
) monthlyQuote {
	return monthlyQuote{
		monthly:       monthly,
		currency:      currency,
		billingDetail: detail,
		components:    components,
		expiresAt:     expires,
		region:        descriptorRegion(resource),
		sku:           sku,
		resourceType:  resource.GetResourceType(),
	}
}

func appServiceBillingDetail(
	resource *finfocusv1.ResourceDescriptor,
	windows bool,
	sku, region string,
	workers int,
	workerNote string,
) string {
	osName := "Linux"
	if windows {
		osName = appServiceOSWindows
	}
	kind := "App Service plan"
	if isFunctionAppResourceType(strings.ToLower(resource.GetResourceType())) {
		kind = "Function App dedicated plan"
	}
	detail := fmt.Sprintf("%s %s %s in %s, 730 hrs/month", kind, osName, sku, region)
	if workers > 1 {
		detail = fmt.Sprintf("%s, %d workers", detail, workers)
	}
	if workerNote != "" {
		detail = fmt.Sprintf("%s, %s", detail, workerNote)
	}
	return detail
}

func itemCurrency(item azureclient.PriceItem) string {
	currency := strings.TrimSpace(item.CurrencyCode)
	if currency == "" {
		return defaultCurrency
	}
	return currency
}
