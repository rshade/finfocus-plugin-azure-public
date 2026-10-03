package pricing

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
	"github.com/rshade/finfocus-plugin-azure-public/internal/logging"
)

const (
	kindVM             = "vm"
	kindDisk           = "disk"
	kindBlob           = "blob"
	kindStorageAccount = "storageaccount"
	kindAppServicePlan = "appserviceplan"
	kindFunctionApp    = "functionapp"

	breakdownCompute = "compute"
	breakdownStorage = "storage"

	usageUnitHours   = "hours"
	actualSourceName = "azure-retail-prices"

	confidenceHigh   = "HIGH"
	confidenceMedium = "MEDIUM"
	confidenceLow    = "LOW"

	taskProjected = "AZ-2.1"
	taskActual    = "AZ-2.2"
	taskEstimate  = "AZ-6.7"

	blobDataStoredMeter = "data stored"
	missingFieldRegion  = "region"
)

// monthlyQuote is a priced resource on a monthly basis, plus the unit price
// that produced it (hourly for VMs, monthly for disks, per GB-month for blobs).
type monthlyQuote struct {
	unitPrice     float64
	monthly       float64
	currency      string
	billingDetail string
	breakdownKey  string
	// components, when non-empty, is the projected breakdown and monthly is
	// its sum. An empty map keeps breakdownKey as the only breakdown entry.
	components   map[string]float64
	expiresAt    time.Time
	region       string
	sku          string
	resourceType string
	// meters are the selected retail rows. price is the meter retail price,
	// not the monthly total. unit is that row's unitOfMeasure.
	meters []quoteMeter
	// advisories and regions are VM alternatives. They are not part of monthly.
	advisories []advisoryPrice
	regions    []advisoryRegion
}

// quoteMeter is one selected retail row behind a quote component.
type quoteMeter struct {
	key   string
	price float64
	unit  string
	// count is how many units of this meter the quote bills at once, such as
	// the instances of a scale set. Zero means one.
	count float64
}

type actualWindow struct {
	start      time.Time
	hours      float64
	confidence string
	hasStart   bool
}

// GetActualCost returns a runtime-scaled cost from public retail rates.
// It is not an Azure billing export. Confidence is carried in Source because
// ActualCostResult has no confidence field in finfocus-spec v0.7.0:
// HIGH when both timestamps are set, MEDIUM when only start is set (end
// defaults to now), and LOW when neither is set (a full 730-hour month).
func (c *Calculator) GetActualCost(
	ctx context.Context,
	req *finfocusv1.GetActualCostRequest,
) (*finfocusv1.GetActualCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetActualCost request")

	window, err := resolveActualWindow(req, time.Now().UTC())
	if err != nil {
		log.Warn().Str("result_status", "error").Err(err).Msg("GetActualCost validation failed")
		return nil, err
	}

	resource := resourceFromActual(req)
	quote, err := c.quoteResource(ctx, resource, taskActual)
	if err != nil {
		log.Warn().Str("result_status", "error").Err(err).Msg("GetActualCost pricing failed")
		return nil, err
	}

	cost := quote.monthly * (window.hours / pluginsdk.HoursPerMonth)
	ts := timestamppb.Now()
	if window.hasStart {
		ts = timestamppb.New(window.start)
	}

	// billing_account_id on the request wins. An empty request id uses the
	// process setting. Empty fails validation, so FocusRecord stays nil.
	// Dry run ignores the request id. Neither source is invented.
	record, focusErr := buildFocusRecord(
		resource,
		quote,
		window,
		c.billingAccountFor(req),
		req.GetResourceId(),
	)
	if focusErr != nil {
		log.Warn().Err(focusErr).Msg("leaving FocusRecord unset")
		record = nil
	}

	result := &finfocusv1.ActualCostResult{
		Timestamp:   ts,
		Cost:        cost,
		UsageAmount: window.hours,
		UsageUnit:   usageUnitHours,
		Source:      fmt.Sprintf("%s[confidence:%s]", actualSourceName, window.confidence),
		FocusRecord: record,
	}
	pluginsdk.ApplyActualCostResultOptions(
		result,
		pluginsdk.WithActualCostResultExpiresAt(quote.expiresAt),
	)

	log.Info().
		Str("region", quote.region).
		Str("sku", quote.sku).
		Str("resource_type", quote.resourceType).
		Float64("cost", cost).
		Float64("runtime_hours", window.hours).
		Str("confidence", window.confidence).
		Str("result_status", "success").
		Msg("GetActualCost completed")

	return pluginsdk.NewActualCostResponse(
		pluginsdk.WithResults([]*finfocusv1.ActualCostResult{result}),
		pluginsdk.WithTotalCount(1),
	), nil
}

// GetProjectedCost returns a monthly public-price projection for a supported
// Azure resource. Validation failures are InvalidArgument. Unsupported
// resource types are Unimplemented. A missing pricing client is Unimplemented
// and names AZ-2.1.
func (c *Calculator) GetProjectedCost(
	ctx context.Context,
	req *finfocusv1.GetProjectedCostRequest,
) (*finfocusv1.GetProjectedCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetProjectedCost request")

	resource, err := projectedResource(req)
	if err != nil {
		log.Warn().Str("result_status", "error").Err(err).Msg("GetProjectedCost validation failed")
		return nil, err
	}

	quote, err := c.quoteResource(ctx, resource, taskProjected)
	if err != nil {
		log.Warn().
			Str("resource_type", resource.GetResourceType()).
			Str("result_status", "error").
			Err(err).
			Msg("GetProjectedCost pricing failed")
		return nil, err
	}

	category := projectedPricingCategory(resource)
	if category == finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC {
		log.Warn().
			Str("resource_type", resource.GetResourceType()).
			Msg("spot interruption risk is unknown; score left at 0")
	}
	resp := pluginsdk.NewGetProjectedCostResponse(projectedOptions(quote, category)...)
	if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
		err = status.Errorf(codes.Internal, "invalid projected cost response: %v", validateErr)
		log.Error().Str("result_status", "error").Err(err).Msg("GetProjectedCost response rejected")
		return nil, err
	}

	log.Info().
		Str("region", quote.region).
		Str("sku", quote.sku).
		Str("resource_type", quote.resourceType).
		Float64("cost_monthly", quote.monthly).
		Str("currency", quote.currency).
		Str("result_status", "success").
		Msg("GetProjectedCost completed")

	return resp, nil
}

func projectedOptions(
	quote monthlyQuote,
	category finfocusv1.FocusPricingCategory,
) []pluginsdk.GetProjectedCostResponseOption {
	opts := []pluginsdk.GetProjectedCostResponseOption{
		pluginsdk.WithProjectedCostDetails(
			quote.unitPrice,
			quote.currency,
			quote.monthly,
			quote.billingDetail,
		),
		pluginsdk.WithProjectedCostPricingCategory(category),
		pluginsdk.WithProjectedCostBreakdown(projectedBreakdown(quote)),
		pluginsdk.WithProjectedCostExpiresAt(quote.expiresAt),
	}
	if priceOpts := priceOptionProtos(quote.advisories, quote.unitPrice, quote.monthly, false); len(priceOpts) > 0 {
		opts = append(opts, pluginsdk.WithProjectedCostPriceOptions(priceOpts...))
	}
	if regions := regionPriceProtos(quote.regions); len(regions) > 0 {
		opts = append(opts, pluginsdk.WithProjectedCostRegionPrices(regions...))
	}
	return opts
}

// billingAccountFor uses the request id when the caller sent one.
// Dry run ignores that id. An empty request id keeps the process setting.
func (c *Calculator) billingAccountFor(req *finfocusv1.GetActualCostRequest) string {
	if c == nil {
		return ""
	}
	if req != nil && !req.GetDryRun() {
		if id := strings.TrimSpace(req.GetBillingAccountId()); id != "" {
			return id
		}
	}
	return c.billingAccountID
}

func projectedPricingCategory(resource *finfocusv1.ResourceDescriptor) finfocusv1.FocusPricingCategory {
	if resource == nil || !isVirtualMachineResourceType(strings.ToLower(resource.GetResourceType())) {
		return finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
	}
	return focusPricingCategory(resource)
}

func (c *Calculator) quoteResource(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	kind, err := classifyResource(resource)
	if err != nil {
		return monthlyQuote{}, err
	}

	switch kind {
	case kindVM:
		return c.quoteVM(ctx, resource, taskID)
	case kindDisk:
		return c.quoteDisk(ctx, resource, taskID)
	case kindBlob:
		return c.quoteBlob(ctx, resource, taskID)
	case kindStorageAccount:
		return c.quoteStorageAccount(ctx, resource, taskID)
	case kindAppServicePlan:
		return c.quoteAppServicePlan(ctx, resource, taskID)
	case kindFunctionApp:
		return c.quoteFunctionApp(ctx, resource, taskID)
	case kindAKS:
		return c.quoteAKS(ctx, resource, taskID)
	case kindSQLDatabase:
		return c.quoteSQLDatabase(ctx, resource, taskID)
	case kindCosmosDB:
		return c.quoteCosmosDB(ctx, resource, taskID)
	case kindLoadBalancer:
		return c.quoteLoadBalancer(ctx, resource, taskID)
	default:
		return monthlyQuote{}, status.Errorf(
			codes.Unimplemented,
			"unsupported resource type: %s",
			resource.GetResourceType(),
		)
	}
}

func (c *Calculator) quoteVM(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	region := descriptorRegion(resource)
	sku := vmSKU(resource)
	if err := requireFields(region, sku); err != nil {
		return monthlyQuote{}, err
	}

	spot, err := descriptorSpot(resource)
	if err != nil {
		return monthlyQuote{}, err
	}
	count, err := vmInstanceCount(resource.GetTags())
	if err != nil {
		return monthlyQuote{}, err
	}
	license, hybrid := vmHybridLicense(resource)

	service := firstNonEmptyTag(resource.GetTags(), "service", "serviceName")
	if service == "" {
		service = defaultServiceName
	}
	query := azureclient.PriceQuery{
		ArmRegionName: region,
		ArmSkuName:    sku,
		ServiceName:   service,
		CurrencyCode:  descriptorCurrency(resource),
	}

	result, err := c.fetchPrices(ctx, query, taskID)
	if err != nil {
		return monthlyQuote{}, err
	}

	item, err := selectVMProduct(result.Items, spot, vmOSWindows(resource) && !hybrid)
	if err != nil {
		return monthlyQuote{}, MapToGRPCStatus(err).Err()
	}

	unit, currency, err := unitPriceAndCurrency([]azureclient.PriceItem{item})
	if err != nil {
		return monthlyQuote{}, MapToGRPCStatus(err).Err()
	}

	monthly := unit * pluginsdk.HoursPerMonth * float64(count)
	advisories, regions := c.vmAdvisories(ctx, query, result.Items, spot, taskID)
	return monthlyQuote{
		unitPrice:     unit,
		monthly:       monthly,
		currency:      currency,
		billingDetail: vmQuoteDetail(spot, service, sku, region, license, count),
		breakdownKey:  breakdownCompute,
		expiresAt:     result.ExpiresAt,
		region:        region,
		sku:           sku,
		resourceType:  resource.GetResourceType(),
		meters: []quoteMeter{{
			key:   breakdownCompute,
			price: unit,
			unit:  item.UnitOfMeasure,
			count: float64(count),
		}},
		advisories: advisories,
		regions:    regions,
	}, nil
}

func (c *Calculator) quoteDisk(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	region := descriptorRegion(resource)
	diskType := descriptorSKU(resource)
	sizeGB, sizeSet, err := descriptorSizeGB(resource)
	if err != nil {
		return monthlyQuote{}, status.Error(codes.InvalidArgument, err.Error())
	}

	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	if diskType == "" {
		missing = append(missing, "disk_type")
	}
	if !sizeSet {
		missing = append(missing, "size_gb")
	}
	if len(missing) > 0 {
		return monthlyQuote{}, missingFieldsError(missing)
	}

	info, err := normalizeDiskType(diskType)
	if err != nil {
		return monthlyQuote{}, status.Error(codes.InvalidArgument, err.Error())
	}

	tierName, err := tierForSize(info.TierPrefix, sizeGB)
	if err != nil {
		return monthlyQuote{}, status.Errorf(
			codes.NotFound,
			"no disk tier found for %.0f GB with type %s",
			sizeGB,
			info.ArmSkuName,
		)
	}

	result, err := c.fetchPrices(ctx, diskRetailQuery(
		region,
		descriptorCurrency(resource),
		info,
		tierName,
	), taskID)
	if err != nil {
		return monthlyQuote{}, err
	}

	monthly, currency, err := selectDiskTierPrice(result.Items, tierName, info.Redundancy)
	if err != nil {
		return monthlyQuote{}, MapToGRPCStatus(err).Err()
	}

	return monthlyQuote{
		unitPrice: monthly,
		monthly:   monthly,
		currency:  currency,
		billingDetail: fmt.Sprintf(
			"Managed disk %s %s in %s, monthly",
			info.ArmSkuName,
			tierName,
			region,
		),
		breakdownKey: breakdownStorage,
		expiresAt:    result.ExpiresAt,
		region:       region,
		sku:          info.ArmSkuName,
		resourceType: resource.GetResourceType(),
		meters: []quoteMeter{{
			key:   breakdownStorage,
			price: monthly,
			unit:  diskMeterUnit(result.Items, tierName, info.Redundancy),
		}},
	}, nil
}

func (c *Calculator) quoteBlob(
	ctx context.Context,
	resource *finfocusv1.ResourceDescriptor,
	taskID string,
) (monthlyQuote, error) {
	region := descriptorRegion(resource)
	sku := descriptorSKU(resource)
	sizeGB, sizeSet, err := descriptorSizeGB(resource)
	if err != nil {
		return monthlyQuote{}, status.Error(codes.InvalidArgument, err.Error())
	}

	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	if sku == "" {
		missing = append(missing, "sku")
	}
	if !sizeSet {
		missing = append(missing, "size_gb")
	}
	if len(missing) > 0 {
		return monthlyQuote{}, missingFieldsError(missing)
	}

	result, err := c.fetchPrices(ctx, azureclient.PriceQuery{
		ArmRegionName: region,
		ServiceName:   storageServiceName,
		ProductName:   "Blob Storage",
		SkuName:       sku,
		CurrencyCode:  descriptorCurrency(resource),
	}, taskID)
	if err != nil {
		return monthlyQuote{}, err
	}

	monthly, perGB, currency, unit, err := selectBlobStoredMonthly(result.Items, sizeGB)
	if err != nil {
		return monthlyQuote{}, MapToGRPCStatus(err).Err()
	}

	return monthlyQuote{
		unitPrice: perGB,
		monthly:   monthly,
		currency:  currency,
		billingDetail: fmt.Sprintf(
			"Blob storage %s %.0f GB-month in %s",
			sku,
			sizeGB,
			region,
		),
		breakdownKey: breakdownStorage,
		expiresAt:    result.ExpiresAt,
		region:       region,
		sku:          sku,
		resourceType: resource.GetResourceType(),
		meters: []quoteMeter{{
			key:   breakdownStorage,
			price: perGB,
			unit:  unit,
		}},
	}, nil
}

func (c *Calculator) fetchPrices(
	ctx context.Context,
	query azureclient.PriceQuery,
	taskID string,
) (azureclient.CachedResult, error) {
	if c.cachedClient == nil {
		return azureclient.CachedResult{}, status.Errorf(
			codes.Unimplemented,
			"%s: pricing client is not configured",
			taskID,
		)
	}

	result, err := c.cachedClient.GetPrices(ctx, query)
	if err != nil {
		return azureclient.CachedResult{}, MapToGRPCStatus(err).Err()
	}
	return result, nil
}

func projectedBreakdown(quote monthlyQuote) map[string]float64 {
	if len(quote.components) > 0 {
		return quote.components
	}
	return map[string]float64{
		quote.breakdownKey: quote.monthly,
	}
}

func projectedResource(req *finfocusv1.GetProjectedCostRequest) (*finfocusv1.ResourceDescriptor, error) {
	if req == nil || req.GetResource() == nil {
		return nil, missingFieldsError([]string{"resource"})
	}
	return req.GetResource(), nil
}

func resourceFromActual(req *finfocusv1.GetActualCostRequest) *finfocusv1.ResourceDescriptor {
	tags := req.GetTags()
	provider := firstNonEmptyTag(tags, "provider")
	if provider == "" {
		provider = "azure"
	}
	resourceType := firstNonEmptyTag(tags, "resource_type", "resourceType", "type")
	if resourceType == "" {
		resourceType = "compute/VirtualMachine"
	}

	return &finfocusv1.ResourceDescriptor{
		Provider:     provider,
		ResourceType: resourceType,
		Region:       firstNonEmptyTag(tags, "region", "location"),
		Sku:          firstNonEmptyTag(tags, "sku", "vmSize", "armSkuName", "disk_type", "diskType"),
		Tags:         tags,
	}
}

func resolveActualWindow(req *finfocusv1.GetActualCostRequest, now time.Time) (actualWindow, error) {
	if req == nil {
		return actualWindow{}, missingFieldsError([]string{"request"})
	}

	hasStart := req.GetStart() != nil
	hasEnd := req.GetEnd() != nil
	switch {
	case hasStart && hasEnd:
		start := req.GetStart().AsTime()
		end := req.GetEnd().AsTime()
		if end.Before(start) {
			return actualWindow{}, status.Error(codes.InvalidArgument, "end time is before start time")
		}
		return actualWindow{
			start:      start,
			hours:      end.Sub(start).Hours(),
			confidence: confidenceHigh,
			hasStart:   true,
		}, nil
	case hasStart:
		start := req.GetStart().AsTime()
		if now.Before(start) {
			return actualWindow{}, status.Error(codes.InvalidArgument, "start time is in the future")
		}
		return actualWindow{
			start:      start,
			hours:      now.Sub(start).Hours(),
			confidence: confidenceMedium,
			hasStart:   true,
		}, nil
	case hasEnd:
		return actualWindow{}, status.Error(codes.InvalidArgument, "start time is required when end time is set")
	default:
		return actualWindow{
			hours:      pluginsdk.HoursPerMonth,
			confidence: confidenceLow,
		}, nil
	}
}

func classifyResource(resource *finfocusv1.ResourceDescriptor) (string, error) {
	if resource == nil {
		return "", missingFieldsError([]string{"resource"})
	}

	provider := strings.TrimSpace(resource.GetProvider())
	if provider == "" {
		return "", missingFieldsError([]string{"provider"})
	}
	if !acceptedAzureProvider(provider) {
		return "", status.Errorf(codes.InvalidArgument, "unsupported provider: %s", provider)
	}

	resourceType := strings.TrimSpace(resource.GetResourceType())
	if resourceType == "" {
		return "", missingFieldsError([]string{"resource_type"})
	}

	lower := strings.ToLower(resourceType)
	switch {
	case isPricedVMResourceType(lower):
		return kindVM, nil
	case isManagedDiskResourceType(lower):
		return kindDisk, nil
	case isBlobStorageResourceType(lower):
		return kindBlob, nil
	case isStorageAccountResourceType(lower):
		return kindStorageAccount, nil
	case isAppServicePlanResourceType(lower):
		return kindAppServicePlan, nil
	case isFunctionAppResourceType(lower) || isNativeFunctionWebApp(lower, resource.GetTags()):
		return kindFunctionApp, nil
	case isAKSResourceType(lower):
		return kindAKS, nil
	case isSQLDatabaseResourceType(lower):
		return kindSQLDatabase, nil
	case isCosmosAccountResourceType(lower):
		return kindCosmosDB, nil
	case isLoadBalancerResourceType(lower):
		return kindLoadBalancer, nil
	default:
		return "", status.Errorf(codes.Unimplemented, "unsupported resource type: %s", resourceType)
	}
}

func isBlobStorageResourceType(lower string) bool {
	return resourceSegment(lower, "storage/blobstorage") ||
		resourceSegment(lower, "storage/blob") ||
		tokenSuffix(lower, "storage", "blob")
}

// acceptedAzureProvider reports whether core's provider is this plugin's cloud.
// azure-native is the Pulumi token prefix. The priced cloud is still Azure.
func acceptedAzureProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case providerAzure, providerAzureNative:
		return true
	default:
		return false
	}
}

// tokenSuffix reports whether lower ends in :module:name, the Azure Native token shape.
func tokenSuffix(lower, module, name string) bool {
	return strings.HasSuffix(lower, ":"+module+":"+name)
}

func resourceSegment(lower, segment string) bool {
	idx := strings.Index(lower, segment)
	if idx < 0 {
		return false
	}
	end := idx + len(segment)
	if end == len(lower) {
		return true
	}
	next := lower[end]
	return next == ':' || next == '/' || next == ' '
}

func descriptorRegion(resource *finfocusv1.ResourceDescriptor) string {
	if region := strings.TrimSpace(resource.GetRegion()); region != "" {
		return region
	}
	return firstNonEmptyTag(resource.GetTags(), "region", "location")
}

func descriptorSKU(resource *finfocusv1.ResourceDescriptor) string {
	if sku := strings.TrimSpace(resource.GetSku()); sku != "" {
		return sku
	}
	return firstNonEmptyTag(resource.GetTags(), "sku", "vmSize", "armSkuName", "disk_type", "diskType")
}

func descriptorCurrency(resource *finfocusv1.ResourceDescriptor) string {
	currency := firstNonEmptyTag(resource.GetTags(), "currency", "currencyCode")
	if currency == "" {
		return defaultCurrency
	}
	return currency
}

func descriptorSizeGB(resource *finfocusv1.ResourceDescriptor) (float64, bool, error) {
	raw := firstNonEmptyTag(resource.GetTags(), "size_gb", "sizeGb", "diskSizeGb", "capacity_gb")
	if raw == "" {
		return 0, false, nil
	}
	sizeGB, err := parseSizeGB(raw)
	if err != nil {
		return 0, true, err
	}
	return sizeGB, true, nil
}

func requireFields(region, sku string) error {
	var missing []string
	if region == "" {
		missing = append(missing, "region")
	}
	if sku == "" {
		missing = append(missing, "sku")
	}
	if len(missing) > 0 {
		return missingFieldsError(missing)
	}
	return nil
}

func missingFieldsError(fields []string) error {
	return status.Errorf(
		codes.InvalidArgument,
		"missing required field(s): %s",
		strings.Join(fields, ", "),
	)
}

// selectBlobStoredMonthly bills Data Stored in marginal volume bands.
// tierMinimumUnits is the start of a band. Each GB is priced by the band it
// falls in. The returned unit price is the first band, which is the list rate.
// No Data Stored meter is ErrNotFound.
func selectBlobStoredMonthly(
	items []azureclient.PriceItem,
	sizeGB float64,
) (float64, float64, string, string, error) {
	chosen, err := chosenBlobStored(items)
	if err != nil {
		return 0, 0, "", "", err
	}

	var bands []azureclient.PriceItem
	for i := range items {
		item := items[i]
		if item.MeterName != chosen.MeterName {
			continue
		}
		if !strings.Contains(strings.ToLower(item.MeterName), blobDataStoredMeter) {
			continue
		}
		bands = append(bands, item)
	}
	sort.SliceStable(bands, func(i, j int) bool {
		return bands[i].TierMinimumUnits < bands[j].TierMinimumUnits
	})
	bands = uniqueTierBands(bands)

	currency := chosen.CurrencyCode
	if strings.TrimSpace(currency) == "" {
		currency = defaultCurrency
	}
	return marginalGBMonth(bands, sizeGB), retailOrUnit(chosen), currency, chosen.UnitOfMeasure, nil
}

// marginalGBMonth prices sizeGB across bands ordered by TierMinimumUnits.
// The next band's minimum is this band's end. A later duplicate minimum is dropped
// by uniqueTierBands before this runs.
func marginalGBMonth(bands []azureclient.PriceItem, sizeGB float64) float64 {
	var total float64
	for i, band := range bands {
		start := band.TierMinimumUnits
		end := math.Inf(1)
		if i+1 < len(bands) {
			end = bands[i+1].TierMinimumUnits
		}
		if sizeGB <= start {
			break
		}
		qty := math.Min(sizeGB, end) - start
		total += qty * retailOrUnit(band)
	}
	return total
}

func uniqueTierBands(bands []azureclient.PriceItem) []azureclient.PriceItem {
	if len(bands) == 0 {
		return nil
	}
	unique := []azureclient.PriceItem{bands[0]}
	for _, band := range bands[1:] {
		if band.TierMinimumUnits == unique[len(unique)-1].TierMinimumUnits {
			continue
		}
		unique = append(unique, band)
	}
	return unique
}

func retailOrUnit(item azureclient.PriceItem) float64 {
	if item.RetailPrice == 0 {
		return item.UnitPrice
	}
	return item.RetailPrice
}

func chosenBlobStored(items []azureclient.PriceItem) (azureclient.PriceItem, error) {
	if len(items) == 0 {
		return azureclient.PriceItem{}, azureclient.ErrNotFound
	}

	var chosen *azureclient.PriceItem
	for i := range items {
		item := items[i]
		if !strings.Contains(strings.ToLower(item.MeterName), blobDataStoredMeter) {
			continue
		}
		if chosen != nil && item.TierMinimumUnits >= chosen.TierMinimumUnits {
			continue
		}
		match := item
		chosen = &match
	}
	if chosen == nil {
		return azureclient.PriceItem{}, azureclient.ErrNotFound
	}
	return *chosen, nil
}
