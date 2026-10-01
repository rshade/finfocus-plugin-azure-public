package pricing

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"testing"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// calculatorAccuracyTolerance is 5 percent of the owner-supplied monthly cost.
// A plugin total at exactly this limit is inside the band.
const calculatorAccuracyTolerance = 0.05

func TestCalculatorAccuracy(t *testing.T) {
	t.Parallel()

	// ownerMonthly is owner-supplied. Leave it nil.
	// An empty field is not zero. Do not write a number here.
	// Do not copy the golden monthly cost into this field.
	tests := []struct {
		name         string
		fixtures     []string
		request      func(t *testing.T) *finfocusv1.GetProjectedCostRequest
		ownerMonthly *float64
	}{
		{
			name:     "compute/VirtualMachine",
			fixtures: []string{"testdata/retail/spot/standard_d2s_v3_eastus.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				loaded := loadRetailFixture(t, "testdata/retail/spot/standard_d2s_v3_eastus.json")
				return vmProjectedRequest(loaded.Items[0].ArmRegionName, loaded.Items[0].ArmSkuName, "")
			},
			ownerMonthly: nil, // owner-supplied
		},
		{
			name:     "storage/ManagedDisk",
			fixtures: []string{"testdata/retail/disk/premium_ssd_lrs_eastus.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return &finfocusv1.GetProjectedCostRequest{
					Resource: &finfocusv1.ResourceDescriptor{
						Provider:     "azure",
						ResourceType: "azure:storage/managedDisk:ManagedDisk",
						Region:       "eastus",
						Sku:          "Premium_SSD_LRS",
						Tags:         map[string]string{"size_gb": "100"},
					},
				}
			},
			ownerMonthly: nil, // owner-supplied
		},
		{
			name:     "storage/BlobStorage",
			fixtures: []string{"testdata/retail/blob/hot_lrs_eastus.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return &finfocusv1.GetProjectedCostRequest{
					Resource: &finfocusv1.ResourceDescriptor{
						Provider:     "azure",
						ResourceType: "storage/BlobStorage",
						Region:       "eastus",
						Sku:          "Hot LRS",
						Tags:         map[string]string{"size_gb": "100"},
					},
				}
			},
			ownerMonthly: nil, // owner-supplied
		},
		{
			name:     "storage/StorageAccount",
			fixtures: []string{"testdata/retail/storageaccount/general_block_blob_v2_eastus.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return storageAccountRequest(
					"storage/StorageAccount",
					"eastus",
					"Hot LRS",
					map[string]string{"size_gb": "100"},
				)
			},
			ownerMonthly: nil, // owner-supplied
		},
		{
			name:     "web/AppServicePlan",
			fixtures: []string{"testdata/retail/appservice/eastus_consumption.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return pricedRequest("web/AppServicePlan", "P1v3", nil)
			},
			ownerMonthly: nil, // owner-supplied
		},
		{
			name:     "web/FunctionApp",
			fixtures: []string{"testdata/retail/functions/eastus_consumption.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return pricedRequest("web/FunctionApp", "Standard", map[string]string{
					"executions": strconv.Itoa(testFreeExecutions + testExecutionsPerPrice),
					"gb_seconds": strconv.Itoa(testFreeGBSeconds + 1),
				})
			},
			ownerMonthly: nil, // owner-supplied
		},
		{
			name:     "containerservice/KubernetesCluster",
			fixtures: []string{"testdata/retail/aks/eastus_consumption.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return aksRequest(aksCanonicalType, "Standard", nil)
			},
			ownerMonthly: nil, // owner-supplied
		},
		{
			name: "sql/Database",
			fixtures: []string{
				"testdata/retail/sqldb/gp_gen5_compute_eastus.json",
				"testdata/retail/sqldb/gp_storage_eastus.json",
			},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return sqlProjectedRequest(
					sqlTestCanonicalType,
					"eastus",
					"GP_Gen5_2",
					map[string]string{"size_gb": "100"},
				)
			},
			ownerMonthly: nil, // owner-supplied
		},
		{
			name:     "cosmosdb/Account",
			fixtures: []string{"testdata/retail/cosmosdb/eastus_consumption.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return cosmosProjectedRequest(
					cosmosTestCanonical,
					"eastus",
					map[string]string{"ru_per_second": "400", "size_gb": "10"},
				)
			},
			ownerMonthly: nil, // owner-supplied
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			owner := skipEmptyOwnerCalculatorCost(t, tt.name, tt.ownerMonthly)
			calc := newGoldenCalc(t, tt.fixtures...)
			resp, err := dialPricingClient(t, calc).GetProjectedCost(context.Background(), tt.request(t))
			if err != nil {
				t.Fatalf("%s GetProjectedCost() failed: %v", tt.name, err)
			}
			if msg := calculatorMonthlyMismatch(resp.GetCostPerMonth(), owner); msg != "" {
				t.Fatalf("%s %s", tt.name, msg)
			}
		})
	}
}

func TestCalculatorAccuracyOutsideBandFails(t *testing.T) {
	t.Parallel()

	// Test-local pair. Not an owner-table row. Not an Azure Pricing Calculator value.
	// |100-110| = 10, and 5 percent of 110 is 5.5, so the case must fail.
	const pluginMonthly = 100.0
	const ownerMonthly = 110.0
	if msg := calculatorMonthlyMismatch(pluginMonthly, ownerMonthly); msg == "" {
		t.Fatal("plugin 100 owner 110 is outside plus or minus 5 percent and must fail the case")
	}
}

func TestCalculatorAccuracyInsideBandPasses(t *testing.T) {
	t.Parallel()

	// Test-local pairs. Not owner-table rows. Not Azure Pricing Calculator values.
	// |100-104| = 4, and 5 percent of 104 is 5.2, so the case passes.
	const pluginMonthly = 100.0
	const ownerMonthly = 104.0
	if msg := calculatorMonthlyMismatch(pluginMonthly, ownerMonthly); msg != "" {
		t.Fatalf("plugin 100 owner 104 is inside plus or minus 5 percent and must pass: %s", msg)
	}

	// |105-100| = 5, and 5 percent of 100 is 5. The limit is inclusive.
	if msg := calculatorMonthlyMismatch(105, 100); msg != "" {
		t.Fatalf("plugin 105 owner 100 is exactly 5 percent and must pass: %s", msg)
	}
}

func skipEmptyOwnerCalculatorCost(t *testing.T, name string, ownerMonthly *float64) float64 {
	t.Helper()
	if ownerMonthly == nil {
		t.Skipf("%s: owner-supplied calculator monthly cost is empty", name)
	}
	return *ownerMonthly
}

func calculatorMonthlyMismatch(plugin, owner float64) string {
	limit := math.Abs(owner) * calculatorAccuracyTolerance
	delta := math.Abs(plugin - owner)
	if delta <= limit {
		return ""
	}
	return fmt.Sprintf(
		"cost_per_month = %v, owner calculator = %v, outside plus or minus 5 percent (delta %v, limit %v)",
		plugin,
		owner,
		delta,
		limit,
	)
}
