package pricing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
	"github.com/rshade/finfocus-plugin-azure-public/internal/logging"
)

// specVersion is the version of the finfocus-spec this plugin implements.
const specVersion = "1.0.0"

const defaultServiceName = "Virtual Machines"

// Calculator implements finfocus.v1.CostSourceServiceServer.
type Calculator struct {
	finfocusv1.UnimplementedCostSourceServiceServer

	logger       zerolog.Logger
	cachedClient *azureclient.CachedClient
}

// NewCalculator creates a new instance of Calculator with the provided logger.
func NewCalculator(logger zerolog.Logger, cachedClient ...*azureclient.CachedClient) *Calculator {
	var cc *azureclient.CachedClient
	if len(cachedClient) > 0 {
		cc = cachedClient[0]
	}

	return &Calculator{
		logger:       logger,
		cachedClient: cc,
	}
}

// Name returns the name of the plugin for the SDK.
// The SDK wraps this and provides the gRPC Name RPC implementation.
func (c *Calculator) Name() string {
	return "azure-public"
}

// GetPluginInfo returns metadata about the plugin including name, version,
// spec version, and supported cloud providers.
func (c *Calculator) GetPluginInfo(
	ctx context.Context,
	_ *finfocusv1.GetPluginInfoRequest,
) (*finfocusv1.GetPluginInfoResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetPluginInfo request")

	return &finfocusv1.GetPluginInfoResponse{
		Name:        "azure-public",
		Version:     "0.1.0",
		SpecVersion: specVersion,
		Providers:   []string{"azure"},
	}, nil
}

// Supports checks if this plugin supports a given resource type by attempting
// to map the resource descriptor to an Azure pricing query. Returns
// Supported:true if the mapping succeeds, or Supported:false with a reason
// describing why the resource cannot be priced.
func (c *Calculator) Supports(
	ctx context.Context,
	req *finfocusv1.SupportsRequest,
) (*finfocusv1.SupportsResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling Supports request")

	_, err := MapDescriptorToQuery(req.GetResource())
	if err != nil {
		log.Debug().Err(err).Msg("resource not supported")
		return &finfocusv1.SupportsResponse{
			Supported: false,
			Reason:    err.Error(),
		}, nil
	}

	return &finfocusv1.SupportsResponse{
		Supported: true,
	}, nil
}

// EstimateCost estimates monthly cost from Azure Retail Prices data.
// Supports VM and Managed Disk resource types via resource-type routing.
// The request must contain the appropriate attributes for the resource type.
// Returns InvalidArgument for missing required fields, Unimplemented for
// unsupported resource types, and mapped gRPC status codes for Azure API
// failures.
func (c *Calculator) EstimateCost(
	ctx context.Context,
	req *finfocusv1.EstimateCostRequest,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)

	resourceType := strings.TrimSpace(req.GetResourceType())
	lowerType := strings.ToLower(resourceType)

	log.Info().
		Str("resource_type", resourceType).
		Msg("handling EstimateCost request")

	// Route by resource type: disk → VM → backward compat (empty) → reject
	switch {
	case isManagedDiskResourceType(lowerType):
		return c.estimateDiskCost(ctx, req, resourceType)
	case resourceType == "" || isVirtualMachineResourceType(lowerType):
		return c.estimateVMCost(ctx, req, resourceType)
	default:
		err := status.Errorf(codes.Unimplemented, "unsupported resource type: %s", resourceType)
		log.Warn().
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost validation failed")
		return nil, err
	}
}

// estimateVMCost handles VM cost estimation (existing path).
func (c *Calculator) estimateVMCost(
	ctx context.Context,
	req *finfocusv1.EstimateCostRequest,
	resourceType string,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)

	query, err := estimateQueryFromRequest(req)
	if err != nil {
		err = status.Error(codes.InvalidArgument, err.Error())
		log.Warn().
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost validation failed")
		return nil, err
	}

	if c.cachedClient == nil {
		unimplementedErr := status.Error(codes.Unimplemented, "not yet implemented")
		log.Warn().
			Str("region", query.ArmRegionName).
			Str("sku", query.ArmSkuName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(unimplementedErr).
			Msg("EstimateCost unavailable")
		return nil, unimplementedErr
	}

	result, err := c.cachedClient.GetPrices(ctx, query)
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("sku", query.ArmSkuName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost pricing lookup failed")
		return nil, err
	}

	unitPrice, currency, err := unitPriceAndCurrency(result.Items)
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("sku", query.ArmSkuName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost response mapping failed")
		return nil, err
	}

	costMonthly := unitPrice * pluginsdk.HoursPerMonth

	log.Info().
		Str("region", query.ArmRegionName).
		Str("sku", query.ArmSkuName).
		Str("resource_type", resourceType).
		Float64("cost_monthly", costMonthly).
		Str("currency", currency).
		Str("result_status", "success").
		Msg("EstimateCost completed")

	return pluginsdk.NewEstimateCostResponse(
		pluginsdk.WithEstimateCost(currency, costMonthly),
		pluginsdk.WithPricingCategory(
			finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD,
		),
	), nil
}

// estimateDiskCost handles Managed Disk cost estimation.
// Disk pricing is monthly (not hourly like VMs), so retailPrice is used directly.
func (c *Calculator) estimateDiskCost(
	ctx context.Context,
	req *finfocusv1.EstimateCostRequest,
	resourceType string,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)

	query, diskInfo, sizeGB, err := estimateDiskQueryFromRequest(req)
	if err != nil {
		err = status.Error(codes.InvalidArgument, err.Error())
		log.Warn().
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost disk validation failed")
		return nil, err
	}

	if c.cachedClient == nil {
		unimplementedErr := status.Error(codes.Unimplemented, "not yet implemented")
		log.Warn().
			Str("region", query.ArmRegionName).
			Str("disk_type", diskInfo.ArmSkuName).
			Float64("size_gb", sizeGB).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(unimplementedErr).
			Msg("EstimateCost unavailable")
		return nil, unimplementedErr
	}

	result, err := c.cachedClient.GetPrices(ctx, query)
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("disk_type", diskInfo.ArmSkuName).
			Float64("size_gb", sizeGB).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost disk pricing lookup failed")
		return nil, err
	}

	tierName, err := tierForSize(diskInfo.TierPrefix, sizeGB)
	if err != nil {
		notFoundErr := status.Errorf(codes.NotFound,
			"no disk tier found for %.0f GB with type %s", sizeGB, diskInfo.ArmSkuName)
		log.Warn().
			Str("region", query.ArmRegionName).
			Str("disk_type", diskInfo.ArmSkuName).
			Float64("size_gb", sizeGB).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(notFoundErr).
			Msg("EstimateCost disk tier lookup failed")
		return nil, notFoundErr
	}

	costMonthly, currency, err := selectDiskTierPrice(result.Items, tierName, diskInfo.Redundancy)
	if err != nil {
		err = MapToGRPCStatus(err).Err()
		log.Error().
			Str("region", query.ArmRegionName).
			Str("disk_type", diskInfo.ArmSkuName).
			Float64("size_gb", sizeGB).
			Str("tier", tierName).
			Str("resource_type", resourceType).
			Str("result_status", "error").
			Err(err).
			Msg("EstimateCost disk tier price lookup failed")
		return nil, err
	}

	log.Info().
		Str("region", query.ArmRegionName).
		Str("disk_type", diskInfo.ArmSkuName).
		Float64("size_gb", sizeGB).
		Str("tier", tierName).
		Str("resource_type", resourceType).
		Float64("cost_monthly", costMonthly).
		Str("currency", currency).
		Str("result_status", "success").
		Msg("EstimateCost disk completed")

	return pluginsdk.NewEstimateCostResponse(
		pluginsdk.WithEstimateCost(currency, costMonthly),
		pluginsdk.WithPricingCategory(
			finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD,
		),
	), nil
}

// estimateDiskQueryFromRequest extracts and validates disk-specific attributes
// from an EstimateCostRequest. Returns the PriceQuery, diskTypeInfo, sizeGB,
// or an error listing all missing/invalid fields.
func estimateDiskQueryFromRequest(
	req *finfocusv1.EstimateCostRequest,
) (azureclient.PriceQuery, diskTypeInfo, float64, error) {
	attributes := map[string]any{}
	if req != nil && req.GetAttributes() != nil {
		attributes = req.GetAttributes().AsMap()
	}

	region := firstNonEmptyMapValue(attributes, "location", "region")
	diskTypeStr := firstNonEmptyMapValue(attributes, "diskType", "disk_type", "sku")
	sizeGBStr := firstNonEmptyMapValue(attributes, "sizeGb", "size_gb", "diskSizeGb")
	currency := firstNonEmptyMapValue(attributes, "currencyCode", "currency")
	if currency == "" {
		currency = defaultCurrency
	}

	// Validate required fields — report all missing in one error.
	var missingFields []string
	if region == "" {
		missingFields = append(missingFields, "region")
	}
	if diskTypeStr == "" {
		missingFields = append(missingFields, "disk_type")
	}
	if sizeGBStr == "" {
		missingFields = append(missingFields, "size_gb")
	}
	if len(missingFields) > 0 {
		return azureclient.PriceQuery{}, diskTypeInfo{}, 0,
			fmt.Errorf("missing required field(s): %s", strings.Join(missingFields, ", "))
	}

	// Parse and validate size_gb.
	sizeGB, err := parseSizeGB(sizeGBStr)
	if err != nil {
		return azureclient.PriceQuery{}, diskTypeInfo{}, 0, err
	}

	// Validate and normalize disk type.
	diskInfo, err := normalizeDiskType(diskTypeStr)
	if err != nil {
		return azureclient.PriceQuery{}, diskTypeInfo{}, 0, err
	}

	query := azureclient.PriceQuery{
		ArmRegionName: region,
		ArmSkuName:    diskInfo.ArmSkuName,
		ServiceName:   "Managed Disks",
		CurrencyCode:  currency,
	}

	return query, diskInfo, sizeGB, nil
}

// parseSizeGB parses and validates the size_gb attribute value.
func parseSizeGB(value string) (float64, error) {
	var sizeGB float64
	_, err := fmt.Sscanf(value, "%f", &sizeGB)
	if err != nil {
		return 0, fmt.Errorf("size_gb must be a valid number: %s", value)
	}
	if sizeGB <= 0 {
		return 0, errors.New("size_gb must be greater than 0")
	}
	return sizeGB, nil
}

// GetActualCost is a stub that returns Unimplemented status.
// Cost history retrieval is not yet implemented.
func (c *Calculator) GetActualCost(
	ctx context.Context,
	req *finfocusv1.GetActualCostRequest,
) (*finfocusv1.GetActualCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetActualCost request")

	query, ok := actualQueryFromRequest(req)
	if !ok || c.cachedClient == nil {
		return nil, status.Error(codes.Unimplemented, "not yet implemented")
	}

	cachedResult, err := c.cachedClient.GetPrices(ctx, query)
	if err != nil {
		return nil, MapToGRPCStatus(err).Err()
	}

	unitPrice, _, err := unitPriceAndCurrency(cachedResult.Items)
	if err != nil {
		return nil, MapToGRPCStatus(err).Err()
	}

	result := &finfocusv1.ActualCostResult{
		Timestamp:   timestamppb.Now(),
		Cost:        unitPrice,
		UsageAmount: 1,
		UsageUnit:   "hour",
		Source:      "azure-retail-prices",
	}
	pluginsdk.ApplyActualCostResultOptions(
		result,
		pluginsdk.WithActualCostResultExpiresAt(cachedResult.ExpiresAt),
	)

	return pluginsdk.NewActualCostResponse(
		pluginsdk.WithResults([]*finfocusv1.ActualCostResult{result}),
		pluginsdk.WithTotalCount(1),
	), nil
}

// GetProjectedCost is a stub that returns Unimplemented status.
// Azure pricing lookup is not yet implemented.
func (c *Calculator) GetProjectedCost(
	ctx context.Context,
	req *finfocusv1.GetProjectedCostRequest,
) (*finfocusv1.GetProjectedCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetProjectedCost request")

	query, ok := projectedQueryFromRequest(req)
	if !ok || c.cachedClient == nil {
		return nil, status.Error(codes.Unimplemented, "not yet implemented")
	}

	cachedResult, err := c.cachedClient.GetPrices(ctx, query)
	if err != nil {
		return nil, MapToGRPCStatus(err).Err()
	}

	unitPrice, currency, err := unitPriceAndCurrency(cachedResult.Items)
	if err != nil {
		return nil, MapToGRPCStatus(err).Err()
	}

	return pluginsdk.NewGetProjectedCostResponse(
		pluginsdk.WithProjectedCostDetails(
			unitPrice,
			currency,
			unitPrice*pluginsdk.HoursPerMonth,
			"azure-retail-prices",
		),
		pluginsdk.WithProjectedCostExpiresAt(cachedResult.ExpiresAt),
	), nil
}

// GetPricingSpec is a stub that returns Unimplemented status.
// Pricing schema is not yet implemented.
func (c *Calculator) GetPricingSpec(
	ctx context.Context,
	_ *finfocusv1.GetPricingSpecRequest,
) (*finfocusv1.GetPricingSpecResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetPricingSpec request")

	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// GetRecommendations is a stub that returns Unimplemented status.
// Recommendations are not supported by the public API.
func (c *Calculator) GetRecommendations(
	ctx context.Context,
	_ *finfocusv1.GetRecommendationsRequest,
) (*finfocusv1.GetRecommendationsResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetRecommendations request")

	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// DismissRecommendation is a stub that returns Unimplemented status.
// Recommendations are not supported.
func (c *Calculator) DismissRecommendation(
	ctx context.Context,
	_ *finfocusv1.DismissRecommendationRequest,
) (*finfocusv1.DismissRecommendationResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling DismissRecommendation request")

	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// GetBudgets is a stub that returns Unimplemented status.
// Budgets are not supported by the public API.
func (c *Calculator) GetBudgets(
	ctx context.Context,
	_ *finfocusv1.GetBudgetsRequest,
) (*finfocusv1.GetBudgetsResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetBudgets request")

	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// DryRun is a stub that returns Unimplemented status.
// Field mapping is not yet implemented.
func (c *Calculator) DryRun(
	ctx context.Context,
	_ *finfocusv1.DryRunRequest,
) (*finfocusv1.DryRunResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling DryRun request")

	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// estimateQueryFromRequest extracts an Azure pricing query from EstimateCost
// request attributes. Supported keys include location/region and
// vmSize/sku/armSkuName with defaults for serviceName and currencyCode.
// Returns an error in the format "missing required field(s): ..." when
// required fields are missing.
func estimateQueryFromRequest(req *finfocusv1.EstimateCostRequest) (azureclient.PriceQuery, error) {
	attributes := map[string]any{}
	if req != nil && req.GetAttributes() != nil {
		attributes = req.GetAttributes().AsMap()
	}

	query := azureclient.PriceQuery{
		ArmRegionName: firstNonEmptyMapValue(attributes, "location", "region"),
		ArmSkuName:    firstNonEmptyMapValue(attributes, "vmSize", "sku", "armSkuName"),
		ServiceName:   firstNonEmptyMapValue(attributes, "serviceName"),
		ProductName:   firstNonEmptyMapValue(attributes, "productName"),
		CurrencyCode:  firstNonEmptyMapValue(attributes, "currencyCode", "currency"),
	}
	if query.CurrencyCode == "" {
		query.CurrencyCode = defaultCurrency
	}
	if query.ServiceName == "" {
		query.ServiceName = defaultServiceName
	}
	var missingFields []string
	if query.ArmRegionName == "" {
		missingFields = append(missingFields, "region")
	}
	if query.ArmSkuName == "" {
		missingFields = append(missingFields, "sku")
	}
	if len(missingFields) > 0 {
		return azureclient.PriceQuery{}, fmt.Errorf(
			"missing required field(s): %s",
			strings.Join(missingFields, ", "),
		)
	}

	return query, nil
}

func actualQueryFromRequest(req *finfocusv1.GetActualCostRequest) (azureclient.PriceQuery, bool) {
	if req == nil {
		return azureclient.PriceQuery{}, false
	}

	tags := req.GetTags()
	query := azureclient.PriceQuery{
		ArmRegionName: firstNonEmptyTag(tags, "region", "location"),
		ArmSkuName:    firstNonEmptyTag(tags, "sku", "vmSize", "armSkuName"),
		ServiceName:   firstNonEmptyTag(tags, "service", "serviceName"),
		ProductName:   firstNonEmptyTag(tags, "product", "productName"),
		CurrencyCode:  firstNonEmptyTag(tags, "currency", "currencyCode"),
	}
	if query.CurrencyCode == "" {
		query.CurrencyCode = defaultCurrency
	}
	if query.ServiceName == "" {
		query.ServiceName = defaultServiceName
	}
	if query.ArmRegionName == "" || query.ArmSkuName == "" {
		return azureclient.PriceQuery{}, false
	}

	return query, true
}

func projectedQueryFromRequest(req *finfocusv1.GetProjectedCostRequest) (azureclient.PriceQuery, bool) {
	if req == nil || req.GetResource() == nil {
		return azureclient.PriceQuery{}, false
	}
	resource := req.GetResource()
	if !strings.EqualFold(resource.GetProvider(), "azure") {
		return azureclient.PriceQuery{}, false
	}

	query := azureclient.PriceQuery{
		ArmRegionName: resource.GetRegion(),
		ArmSkuName:    resource.GetSku(),
		CurrencyCode:  firstNonEmptyTag(resource.GetTags(), "currency", "currencyCode"),
		ServiceName:   firstNonEmptyTag(resource.GetTags(), "service", "serviceName"),
		ProductName:   firstNonEmptyTag(resource.GetTags(), "product", "productName"),
	}
	if query.CurrencyCode == "" {
		query.CurrencyCode = defaultCurrency
	}
	if query.ServiceName == "" {
		query.ServiceName = defaultServiceName
	}
	if query.ArmRegionName == "" || query.ArmSkuName == "" {
		return azureclient.PriceQuery{}, false
	}

	return query, true
}

// isVirtualMachineResourceType checks whether the lowercased resource type
// refers to compute/virtualmachine as a full segment (not a prefix of e.g.
// "compute/virtualmachinescaleset").
func isVirtualMachineResourceType(lower string) bool {
	const segment = "compute/virtualmachine"
	idx := strings.Index(lower, segment)
	if idx < 0 {
		return false
	}
	end := idx + len(segment)
	if end == len(lower) {
		return true
	}
	// Next char must be a segment separator, not a continuation letter/digit.
	next := lower[end]
	return next == ':' || next == '/' || next == ' '
}

func firstNonEmptyTag(tags map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(tags[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyMapValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprintf("%v", raw))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func unitPriceAndCurrency(items []azureclient.PriceItem) (float64, string, error) {
	if len(items) == 0 {
		return 0, "", azureclient.ErrNotFound
	}

	item := items[0]
	price := item.RetailPrice
	if price == 0 {
		price = item.UnitPrice
	}
	currency := item.CurrencyCode
	if strings.TrimSpace(currency) == "" {
		currency = defaultCurrency
	}

	return price, currency, nil
}
