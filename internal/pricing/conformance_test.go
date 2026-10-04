package pricing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	sdktesting "github.com/rshade/finfocus-spec/sdk/go/testing"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// TestSDKConformance_AzureSample_PassesEveryLevel serves the calculator the way
// cmd/ does and runs the finfocus-spec conformance suite with an Azure sample
// resource. The fixture answers every price query with one Linux B1s row.
func TestSDKConformance_AzureSample_PassesEveryLevel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := azureclient.PriceResponse{
			Items: []azureclient.PriceItem{{
				ArmRegionName: "eastus",
				ArmSkuName:    "Standard_B1s",
				ServiceName:   "Virtual Machines",
				ProductName:   "Virtual Machines BS Series",
				SkuName:       "B1s",
				MeterName:     "B1s",
				Type:          "Consumption",
				UnitOfMeasure: "1 Hour",
				CurrencyCode:  "USD",
				RetailPrice:   0.0104,
			}},
			Count: 1,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	cachedClient := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(cachedClient.Close)
	served := pluginsdk.NewServerWithOptions(NewCalculator(zerolog.Nop(), cachedClient), nil, nil, PluginInfo())
	sample := &finfocusv1.ResourceDescriptor{
		Provider:     providerAzure,
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
		Region:       "eastus",
		Sku:          "Standard_B1s",
	}

	for _, level := range []sdktesting.ConformanceLevel{
		sdktesting.ConformanceLevelBasic,
		sdktesting.ConformanceLevelStandard,
		sdktesting.ConformanceLevelAdvanced,
	} {
		result, err := sdktesting.RunConformance(served, level, sdktesting.WithSampleResource(sample))
		if err != nil {
			t.Fatalf("RunConformance(%v) error = %v", level, err)
		}
		for _, category := range result.Categories {
			for _, check := range category.Results {
				if !check.Success {
					t.Errorf("%v: %s failed: %v %s", level, check.Method, check.Error, check.Details)
				}
			}
		}
		if result.Summary.Failed != 0 || result.Summary.Passed == 0 {
			t.Errorf("%v: passed=%d failed=%d, want all checks passing", level,
				result.Summary.Passed, result.Summary.Failed)
		}
	}
}

func TestUnservedRPCs_OverGRPC_ReturnUnimplemented(t *testing.T) {
	t.Parallel()

	client := newDiscoveryClient(t, NewCalculator(zerolog.Nop()))
	ctx := context.Background()

	_, budgetsErr := client.GetBudgets(ctx, &finfocusv1.GetBudgetsRequest{})
	_, dismissErr := client.DismissRecommendation(ctx, &finfocusv1.DismissRecommendationRequest{
		RecommendationId: "rec-1",
	})
	for name, err := range map[string]error{"GetBudgets": budgetsErr, "DismissRecommendation": dismissErr} {
		if status.Code(err) != codes.Unimplemented {
			t.Errorf("%s status = %v (%v), want Unimplemented", name, status.Code(err), err)
		}
	}
}
