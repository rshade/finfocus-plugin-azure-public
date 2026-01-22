package pricing

import (
	"testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

func TestCalculatorName(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)
	testPlugin.TestName("azure-public")
}

func TestProjectedCostSupported(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test supported resource
	resource := pluginsdk.CreateTestResource("aws", "aws:ec2:Instance", map[string]string{
		"instanceType": "t3.micro",
		"region":       "us-east-1",
	})

	resp := testPlugin.TestProjectedCost(resource, false)
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", resp.Currency)
	}

	if resp.UnitPrice <= 0 {
		t.Errorf("Expected positive unit price, got %f", resp.UnitPrice)
	}
}

func TestProjectedCostUnsupported(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test unsupported resource
	resource := pluginsdk.CreateTestResource("unsupported", "unsupported:resource:Type", nil)
	testPlugin.TestProjectedCost(resource, true) // Expect error
}

func TestActualCost(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)

	// Test actual cost (should return error since not implemented)
	testPlugin.TestActualCost("resource-id-123", 1640995200, 1641081600, true) // Expect error
}

// Example of more specific test cases
func TestEC2InstancePricing(t *testing.T) {
	calculator := NewCalculator()

	testCases := []struct {
		name         string
		instanceType string
		expectedCost float64
	}{
		{"t3.micro", "t3.micro", 0.0104},
		{"t3.small", "t3.small", 0.0208},
		{"t3.medium", "t3.medium", 0.0416},
		{"unknown", "unknown-type", 0.0104}, // fallback
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := &pbc.ResourceDescriptor{
				Provider:     "aws",
				ResourceType: "aws:ec2:Instance",
				Tags: map[string]string{
					"instanceType": tc.instanceType,
				},
			}

			cost := calculator.calculateEC2InstanceCost(resource)
			if cost != tc.expectedCost {
				t.Errorf("Expected cost %f for %s, got %f", tc.expectedCost, tc.instanceType, cost)
			}
		})
	}
}
