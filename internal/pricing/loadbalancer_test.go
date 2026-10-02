package pricing

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const loadBalancerFixturePath = "testdata/retail/loadbalancer/global_standard.json"

func TestGetProjectedCostLoadBalancerIncludedRules(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, loadBalancerFixturePath)
	included := requireLoadBalancerMeter(t, loaded.Items, "Standard Included LB Rules and Outbound Rules", "1 Hour")
	if included.RetailPrice == 0 {
		t.Fatal("included rules retail price is zero")
	}
	calls := 0
	calc := newLoadBalancerCalc(t, loaded, &calls)
	resp, err := calc.GetProjectedCost(context.Background(), loadBalancerRequest(
		"network/LoadBalancer",
		"",
		nil,
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if calls < 2 {
		t.Fatalf("price calls = %d, want a regional miss and a Global read", calls)
	}
	want := included.RetailPrice * pluginsdk.HoursPerMonth
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), want)
	}
	if resp.GetCostBreakdown()["rules"] != want {
		t.Fatalf("rules = %v, want %v", resp.GetCostBreakdown()["rules"], want)
	}
	if _, ok := resp.GetCostBreakdown()["rule_overage"]; ok {
		t.Fatal("omitted rule_count billed overage")
	}
	if _, ok := resp.GetCostBreakdown()["data_processed"]; ok {
		t.Fatal("omitted data processed billed data")
	}
	detail := resp.GetBillingDetail()
	if !strings.Contains(detail, "Global") || !strings.Contains(detail, "omitted") {
		t.Fatalf("billing_detail = %q", detail)
	}
	if err := pluginsdk.ValidateGetProjectedCostResponse(resp); err != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", err)
	}
}

func TestGetProjectedCostLoadBalancerOverageAndData(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, loadBalancerFixturePath)
	included := requireLoadBalancerMeter(t, loaded.Items, "Standard Included LB Rules and Outbound Rules", "1 Hour")
	overage := requireLoadBalancerMeter(t, loaded.Items, "Standard Overage LB Rules and Outbound Rules", "1/Hour")
	data := requireLoadBalancerMeter(t, loaded.Items, "Standard Data Processed", "1 GB")
	calc := newLoadBalancerCalc(t, loaded, nil)
	resp, err := calc.GetProjectedCost(context.Background(), loadBalancerRequest(
		"azure:lb/loadBalancer:LoadBalancer",
		"Standard",
		map[string]string{"rule_count": "7", "data_processed_gb": "100"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	rules := included.RetailPrice * pluginsdk.HoursPerMonth
	extra := 2 * overage.RetailPrice * pluginsdk.HoursPerMonth
	processed := 100 * data.RetailPrice
	want := rules + extra + processed
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), want)
	}
	if math.Abs(resp.GetCostBreakdown()["rules"]-rules) > 1e-9 ||
		math.Abs(resp.GetCostBreakdown()["rule_overage"]-extra) > 1e-9 ||
		math.Abs(resp.GetCostBreakdown()["data_processed"]-processed) > 1e-9 {
		t.Fatalf("breakdown = %v", resp.GetCostBreakdown())
	}
	if !strings.Contains(resp.GetBillingDetail(), "above the included 5") {
		t.Fatalf("billing_detail = %q", resp.GetBillingDetail())
	}
}

func TestGetProjectedCostLoadBalancerNoRules(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, loadBalancerFixturePath)
	calc := newLoadBalancerCalc(t, loaded, nil)
	resp, err := calc.GetProjectedCost(context.Background(), loadBalancerRequest(
		"network/LoadBalancer",
		"Standard",
		map[string]string{"rule_count": "0"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if resp.GetCostPerMonth() != 0 {
		t.Fatalf("cost_per_month = %v, want 0", resp.GetCostPerMonth())
	}
	if !strings.Contains(resp.GetBillingDetail(), "no hourly charge") {
		t.Fatalf("billing_detail = %q", resp.GetBillingDetail())
	}
	if err := pluginsdk.ValidateGetProjectedCostResponse(resp); err != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", err)
	}
}

func TestGetProjectedCostLoadBalancerRejectsOtherSKU(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, loadBalancerFixturePath)
	calc := newLoadBalancerCalc(t, loaded, nil)
	_, err := calc.GetProjectedCost(context.Background(), loadBalancerRequest(
		"network/LoadBalancer",
		"Gateway",
		nil,
	))
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
	if !strings.Contains(status.Convert(err).Message(), "Gateway") {
		t.Fatalf("message = %q", status.Convert(err).Message())
	}
}

func TestGetProjectedCostLoadBalancerUsesRegionalPage(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, loadBalancerFixturePath)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		filter := r.URL.Query().Get("$filter")
		if !strings.Contains(filter, "eastus") {
			t.Errorf("filter = %s", filter)
		}
		writePricePage(t, w, loaded)
	}))
	t.Cleanup(server.Close)
	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	calc := NewCalculator(zerolog.Nop(), cached)

	resp, err := calc.GetProjectedCost(context.Background(), loadBalancerRequest(
		"azure-native:network/loadBalancer:LoadBalancer",
		"Standard",
		nil,
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("price calls = %d, want 1", calls)
	}
	if resp.GetCostPerMonth() == 0 {
		t.Fatal("regional page produced a zero cost")
	}
}

func loadBalancerRequest(resourceType, sku string, tags map[string]string) *finfocusv1.GetProjectedCostRequest {
	return &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: resourceType,
			Region:       "eastus",
			Sku:          sku,
			Tags:         tags,
		},
	}
}

func newLoadBalancerCalc(t *testing.T, page azureclient.PriceResponse, calls *int) *Calculator {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			*calls++
		}
		filter := r.URL.Query().Get("$filter")
		if strings.Contains(filter, "Global") {
			writePricePage(t, w, page)
			return
		}
		writePricePage(t, w, azureclient.PriceResponse{})
	}))
	t.Cleanup(server.Close)
	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	return NewCalculator(zerolog.Nop(), cached)
}

func writePricePage(t *testing.T, w http.ResponseWriter, page azureclient.PriceResponse) {
	t.Helper()

	if page.Count == 0 {
		page.Count = len(page.Items)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(page); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func requireLoadBalancerMeter(t *testing.T, items []azureclient.PriceItem, meter, unit string) azureclient.PriceItem {
	t.Helper()

	for _, item := range items {
		if item.MeterName == meter && item.UnitOfMeasure == unit && item.RetailPrice > 0 {
			return item
		}
	}
	t.Fatalf("fixture missing %s %s", meter, unit)
	return azureclient.PriceItem{}
}
