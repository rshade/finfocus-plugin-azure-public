package pricing

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const goldenCostTolerance = 1e-9

func TestGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		golden   string
		exact    bool
		fixtures []string
		request  func(t *testing.T) *finfocusv1.GetProjectedCostRequest
	}{
		{
			name:     "compute/VirtualMachine",
			golden:   "testdata/golden/compute_virtual_machine.txt",
			fixtures: []string{"testdata/retail/spot/standard_d2s_v3_eastus.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				loaded := loadRetailFixture(t, "testdata/retail/spot/standard_d2s_v3_eastus.json")
				return vmProjectedRequest(loaded.Items[0].ArmRegionName, loaded.Items[0].ArmSkuName, "")
			},
		},
		{
			name:     "storage/ManagedDisk",
			golden:   "testdata/golden/storage_managed_disk.txt",
			exact:    true,
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
		},
		{
			name:     "storage/BlobStorage",
			golden:   "testdata/golden/storage_blob_storage.txt",
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
		},
		{
			name:     "storage/StorageAccount",
			golden:   "testdata/golden/storage_storage_account.txt",
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
		},
		{
			name:     "web/AppServicePlan",
			golden:   "testdata/golden/web_app_service_plan.txt",
			fixtures: []string{"testdata/retail/appservice/eastus_consumption.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return pricedRequest("web/AppServicePlan", "P1v3", nil)
			},
		},
		{
			name:     "web/FunctionApp",
			golden:   "testdata/golden/web_function_app.txt",
			fixtures: []string{"testdata/retail/functions/eastus_consumption.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return pricedRequest("web/FunctionApp", "Standard", map[string]string{
					"executions": strconv.Itoa(testFreeExecutions + testExecutionsPerPrice),
					"gb_seconds": strconv.Itoa(testFreeGBSeconds + 1),
				})
			},
		},
		{
			name:     "containerservice/KubernetesCluster",
			golden:   "testdata/golden/containerservice_kubernetes_cluster.txt",
			fixtures: []string{"testdata/retail/aks/eastus_consumption.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return aksRequest(aksCanonicalType, "Standard", nil)
			},
		},
		{
			name:   "sql/Database",
			golden: "testdata/golden/sql_database.txt",
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
		},
		{
			name:     "cosmosdb/Account",
			golden:   "testdata/golden/cosmosdb_account.txt",
			fixtures: []string{"testdata/retail/cosmosdb/eastus_consumption.json"},
			request: func(t *testing.T) *finfocusv1.GetProjectedCostRequest {
				t.Helper()
				return cosmosProjectedRequest(
					cosmosTestCanonical,
					"eastus",
					map[string]string{"ru_per_second": "400", "size_gb": "10"},
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want := readGoldenCost(t, tt.golden)
			calc := newGoldenCalc(t, tt.fixtures...)
			resp, err := dialPricingClient(t, calc).GetProjectedCost(context.Background(), tt.request(t))
			if err != nil {
				t.Fatalf("%s GetProjectedCost() failed: %v", tt.name, err)
			}
			got := resp.GetCostPerMonth()
			if !goldenCostsMatch(got, want, tt.exact) {
				t.Fatalf("%s cost_per_month = %v, want %v", tt.name, got, want)
			}
		})
	}
}

func goldenCostsMatch(got, want float64, exact bool) bool {
	if exact {
		return got == want
	}
	return math.Abs(got-want) <= goldenCostTolerance
}

func readGoldenCost(t *testing.T, path string) float64 {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden cost: %v", err)
	}
	text := strings.TrimSpace(string(data))
	cost, err := strconv.ParseFloat(text, 64)
	if err != nil {
		t.Fatalf("parse golden cost %q: %v", text, err)
	}
	return cost
}

func newGoldenCalc(t *testing.T, paths ...string) *Calculator {
	t.Helper()

	if len(paths) == 0 || len(paths) > 2 {
		t.Fatalf("golden fixtures = %d, want 1 or 2", len(paths))
	}
	pages := make([]azureclient.PriceResponse, len(paths))
	for i, path := range paths {
		pages[i] = loadRetailFixture(t, path)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := retailFakeItems(r.URL.Query().Get("$filter"), pages[0].Items)
		if len(pages) == 2 {
			items = sqlItemsForFilter(r.URL.Query().Get("$filter"), pages[0].Items, pages[1].Items)
		}
		resp := azureclient.PriceResponse{Items: items, Count: len(items)}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	return NewCalculator(zerolog.Nop(), cached)
}
