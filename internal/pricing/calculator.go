package pricing

import (
	"context"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/logging"
)

// specVersion is the version of the finfocus-spec this plugin implements.
const specVersion = "1.0.0"

// Calculator implements finfocus.v1.CostSourceServiceServer.
type Calculator struct {
	finfocusv1.UnimplementedCostSourceServiceServer

	logger zerolog.Logger
}

// NewCalculator creates a new instance of Calculator with the provided logger.
func NewCalculator(logger zerolog.Logger) *Calculator {
	return &Calculator{
		logger: logger,
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
	_ *finfocusv1.EstimateCostRequest,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling EstimateCost request")

	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// GetActualCost is a stub that returns Unimplemented status.
// Cost history retrieval is not yet implemented.
func (c *Calculator) GetActualCost(
	ctx context.Context,
	_ *finfocusv1.GetActualCostRequest,
) (*finfocusv1.GetActualCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetActualCost request")

	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// GetProjectedCost is a stub that returns Unimplemented status.
// Azure pricing lookup is not yet implemented.
func (c *Calculator) GetProjectedCost(
	ctx context.Context,
	_ *finfocusv1.GetProjectedCostRequest,
) (*finfocusv1.GetProjectedCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetProjectedCost request")

	return nil, status.Error(codes.Unimplemented, "not yet implemented")
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
