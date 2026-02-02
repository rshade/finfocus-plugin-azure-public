package pricing

import (
	"context"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// Calculator implements finfocus.v1.CostSourceServiceServer.
type Calculator struct {
	finfocusv1.UnimplementedCostSourceServiceServer
}

// NewCalculator creates a new instance of Calculator.
func NewCalculator() *Calculator {
	return &Calculator{}
}

// Name returns the name of the plugin.
func (c *Calculator) Name() string {
	return "azure-public"
}

// GetPluginInfo returns metadata about the plugin.
func (c *Calculator) GetPluginInfo(
	_ context.Context,
	_ *finfocusv1.GetPluginInfoRequest,
) (*finfocusv1.GetPluginInfoResponse, error) {
	return &finfocusv1.GetPluginInfoResponse{
		Name:    "azure-public",
		Version: "0.1.0",
	}, nil
}

// EstimateCost calculates the estimated cost for a given resource.
func (c *Calculator) EstimateCost(
	_ context.Context,
	_ *finfocusv1.EstimateCostRequest,
) (*finfocusv1.EstimateCostResponse, error) {
	return &finfocusv1.EstimateCostResponse{}, nil
}
