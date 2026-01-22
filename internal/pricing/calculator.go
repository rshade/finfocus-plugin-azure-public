package pricing

import (
	"context"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// Calculator implements the FinFocus plugin interface for azure-public.
type Calculator struct {
	*pluginsdk.BasePlugin
}

// NewCalculator creates a new azure-public cost calculator plugin.
func NewCalculator() *Calculator {
	base := pluginsdk.NewBasePlugin("azure-public")
	
	// Configure supported providers
	providers := []string{"azure"}
	for _, provider := range providers {
		base.Matcher().AddProvider(provider)
	}

	return &Calculator{
		BasePlugin: base,
	}
}

// GetProjectedCost calculates projected costs for resources.
func (c *Calculator) GetProjectedCost(ctx context.Context, req *pbc.GetProjectedCostRequest) (*pbc.GetProjectedCostResponse, error) {
	// Check if we support this resource
	if !c.Matcher().Supports(req.Resource) {
		return nil, pluginsdk.NotSupportedError(req.Resource)
	}

	// [TEMPLATE] Implementation Required: Pricing Logic
	// Calculate the unit price based on the resource type and tags.
	// The example below demonstrates how to handle AWS EC2 instances.
	// Replace this with your provider's specific logic.
	unitPrice := 0.0
	billingDetail := "Pricing not implemented"

	// Example: Basic EC2 instance pricing
	switch req.Resource.ResourceType {
	case "aws:ec2:Instance":
		unitPrice = c.calculateEC2InstanceCost(req.Resource)
		billingDetail = "EC2 instance hourly cost"
	default:
		return nil, pluginsdk.NotSupportedError(req.Resource)
	}

	return c.Calculator().CreateProjectedCostResponse("USD", unitPrice, billingDetail), nil
}

// GetActualCost retrieves actual historical costs.
func (c *Calculator) GetActualCost(ctx context.Context, req *pbc.GetActualCostRequest) (*pbc.GetActualCostResponse, error) {
	// [TEMPLATE] Implementation Required: Actual Cost Retrieval
	// Connect to your cloud provider's billing API to retrieve historical cost data.
	// If actual cost retrieval is not supported, keep returning NoDataError.
	return nil, pluginsdk.NoDataError(req.ResourceId)
}

// calculateEC2InstanceCost is an example pricing calculation.
func (c *Calculator) calculateEC2InstanceCost(resource *pbc.ResourceDescriptor) float64 {
	// [TEMPLATE] Implementation Required: Pricing Calculation
	// This is a simplified example. A real implementation should:
	// 1. Parse instance type from resource properties
	// 2. Look up pricing from your provider's Pricing API or local pricing data
	// 3. Consider region, operating system, tenancy, etc.
	
	instanceType := resource.Tags["instanceType"]
	if instanceType == "" {
		instanceType = "t3.micro" // default
	}

	// Simplified pricing - replace with real pricing data
	switch instanceType {
	case "t3.micro":
		return 0.0104 // $0.0104/hour
	case "t3.small":
		return 0.0208 // $0.0208/hour
	case "t3.medium":
		return 0.0416 // $0.0416/hour
	default:
		return 0.0104 // fallback
	}
}
