package pricing

import (
	"testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

func TestCalculatorName(t *testing.T) {
	plugin := NewCalculator()
	testPlugin := pluginsdk.NewTestPlugin(t, plugin)
	testPlugin.TestName("azure-public")
}

func TestProjectedCostSupported(t *testing.T) {
	t.Skip("Skipping: GetProjectedCost not implemented yet. Azure pricing lookup requires implementation.")

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

	if resp.GetCurrency() != "USD" {
		t.Errorf("Expected currency USD, got %s", resp.GetCurrency())
	}

	if resp.GetUnitPrice() <= 0 {
		t.Errorf("Expected positive unit price, got %f", resp.GetUnitPrice())
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

// TODO: Add Azure-specific pricing tests when Azure pricing lookup is implemented.
// The following test was removed because it tested AWS EC2 instances (incorrect for Azure plugin)
// and referenced a non-existent calculateEC2InstanceCost method.
