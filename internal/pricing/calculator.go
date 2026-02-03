package pricing

import (
	"context"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/logging"
)

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

// Name returns the name of the plugin.
func (c *Calculator) Name() string {
	return "azure-public"
}

// GetPluginInfo returns metadata about the plugin.
func (c *Calculator) GetPluginInfo(
	ctx context.Context,
	_ *finfocusv1.GetPluginInfoRequest,
) (*finfocusv1.GetPluginInfoResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling GetPluginInfo request")

	return &finfocusv1.GetPluginInfoResponse{
		Name:    "azure-public",
		Version: "0.1.0",
	}, nil
}

// EstimateCost calculates the estimated cost for a given resource.
func (c *Calculator) EstimateCost(
	ctx context.Context,
	_ *finfocusv1.EstimateCostRequest,
) (*finfocusv1.EstimateCostResponse, error) {
	log := logging.RequestLogger(ctx, c.logger)
	log.Info().Msg("handling EstimateCost request")

	return &finfocusv1.EstimateCostResponse{}, nil
}
