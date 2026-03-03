package pricing

import (
	"context"
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

// Supports checks if this plugin supports a given resource type.
// Currently returns false for all resource types as Azure pricing lookup
// is not yet implemented.
func (c *Calculator) Supports(
	ctx context.Context,
	_ *finfocusv1.SupportsRequest,
) (*finfocusv1.SupportsResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling Supports request")

	return &finfocusv1.SupportsResponse{
		Supported: false,
		Reason:    "not yet implemented",
	}, nil
}

// EstimateCost is a stub that returns Unimplemented status.
// Azure pricing lookup is not yet implemented.
func (c *Calculator) EstimateCost(
	ctx context.Context,
	req *finfocusv1.EstimateCostRequest,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling EstimateCost request")

	query, ok := estimateQueryFromRequest(req)
	if !ok || c.cachedClient == nil {
		return nil, status.Error(codes.Unimplemented, "not yet implemented")
	}

	result, err := c.cachedClient.GetPrices(ctx, query)
	if err != nil {
		return nil, MapToGRPCStatus(err).Err()
	}

	unitPrice, currency, err := unitPriceAndCurrency(result.Items)
	if err != nil {
		return nil, MapToGRPCStatus(err).Err()
	}

	return pluginsdk.NewEstimateCostResponse(
		pluginsdk.WithEstimateCost(currency, unitPrice*pluginsdk.HoursPerMonth),
	), nil
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

func estimateQueryFromRequest(req *finfocusv1.EstimateCostRequest) (azureclient.PriceQuery, bool) {
	if req == nil || req.GetAttributes() == nil {
		return azureclient.PriceQuery{}, false
	}

	attributes := req.GetAttributes().AsMap()
	query := azureclient.PriceQuery{
		ArmRegionName: firstNonEmptyMapValue(attributes, "location", "region"),
		ArmSkuName:    firstNonEmptyMapValue(attributes, "vmSize", "sku", "armSkuName"),
		ServiceName:   firstNonEmptyMapValue(attributes, "serviceName"),
		ProductName:   firstNonEmptyMapValue(attributes, "productName"),
		CurrencyCode:  firstNonEmptyMapValue(attributes, "currencyCode", "currency"),
	}
	if query.CurrencyCode == "" {
		query.CurrencyCode = "USD"
	}
	if query.ServiceName == "" {
		query.ServiceName = defaultServiceName
	}
	if query.ArmRegionName == "" || query.ArmSkuName == "" {
		return azureclient.PriceQuery{}, false
	}

	return query, true
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
		query.CurrencyCode = "USD"
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
		query.CurrencyCode = "USD"
	}
	if query.ServiceName == "" {
		query.ServiceName = defaultServiceName
	}
	if query.ArmRegionName == "" || query.ArmSkuName == "" {
		return azureclient.PriceQuery{}, false
	}

	return query, true
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
		currency = "USD"
	}

	return price, currency, nil
}
