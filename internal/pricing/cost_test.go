package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestGetProjectedCostVMIncludesBreakdownAndCategory(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
		ServiceName:   "Virtual Machines",
		CurrencyCode:  "USD",
		RetailPrice:   0.0104,
	}})

	resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "azure:compute/virtualMachine:VirtualMachine",
			Region:       "eastus",
			Sku:          "Standard_B1s",
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if err := pluginsdk.ValidateGetProjectedCostResponse(resp); err != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", err)
	}

	wantMonthly := 0.0104 * pluginsdk.HoursPerMonth
	if math.Abs(resp.GetCostPerMonth()-wantMonthly) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), wantMonthly)
	}
	if resp.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD {
		t.Fatalf("pricing category = %s", resp.GetPricingCategory())
	}
	if resp.GetBillingDetail() == "" {
		t.Fatal("expected billing_detail")
	}
	if got := resp.GetCostBreakdown()["compute"]; got != resp.GetCostPerMonth() {
		t.Fatalf("compute breakdown = %v, cost_per_month = %v", got, resp.GetCostPerMonth())
	}
}

func TestGetProjectedCostDiskUsesMonthlyTierPrice(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		{
			ProductName:  "Premium SSD Managed Disks",
			SkuName:      "P4 LRS",
			MeterName:    "P4 LRS Disk",
			RetailPrice:  5.28,
			CurrencyCode: "USD",
		},
		{
			ProductName:  "Premium SSD Managed Disks",
			SkuName:      "P10 LRS",
			MeterName:    "P10 LRS Disk",
			RetailPrice:  19.71,
			CurrencyCode: "USD",
		},
	})

	resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "azure:storage/managedDisk:ManagedDisk",
			Region:       "eastus",
			Sku:          "Premium_SSD_LRS",
			Tags:         map[string]string{"size_gb": "100"},
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() disk failed: %v", err)
	}
	if err := pluginsdk.ValidateGetProjectedCostResponse(resp); err != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", err)
	}
	if resp.GetCostPerMonth() != 19.71 {
		t.Fatalf("cost_per_month = %v, want 19.71", resp.GetCostPerMonth())
	}
	if resp.GetCostBreakdown()["storage"] != 19.71 {
		t.Fatalf("storage breakdown = %v", resp.GetCostBreakdown()["storage"])
	}
}

func TestGetProjectedCostBlobScalesBySizeAndPrefersDataStored(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		{
			ProductName:  generalBlockBlobV2Product,
			SkuName:      "Hot LRS",
			MeterName:    "Hot LRS Write Operations",
			RetailPrice:  0.0001,
			CurrencyCode: "USD",
		},
		{
			ProductName:      generalBlockBlobV2Product,
			SkuName:          "Hot LRS",
			MeterName:        "Hot LRS Data Stored",
			RetailPrice:      0.019136,
			CurrencyCode:     "USD",
			TierMinimumUnits: 512000,
		},
		{
			ProductName:  generalBlockBlobV2Product,
			SkuName:      "Hot LRS",
			MeterName:    "Hot LRS Data Stored",
			RetailPrice:  0.0208,
			CurrencyCode: "USD",
		},
		{
			ProductName:      generalBlockBlobV2Product,
			SkuName:          "Hot LRS",
			MeterName:        "Hot LRS Data Stored",
			RetailPrice:      0.019968,
			CurrencyCode:     "USD",
			TierMinimumUnits: 51200,
		},
	})

	resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "storage/BlobStorage",
			Region:       "eastus",
			Sku:          "Hot LRS",
			Tags:         map[string]string{"size_gb": "100"},
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() blob failed: %v", err)
	}
	if err := pluginsdk.ValidateGetProjectedCostResponse(resp); err != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", err)
	}

	want := 0.0208 * 100
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), want)
	}
	if resp.GetUnitPrice() != 0.0208 {
		t.Fatalf("unit_price = %v, want per-GB 0.0208", resp.GetUnitPrice())
	}

	banded, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "storage/BlobStorage",
			Region:       "eastus",
			Sku:          "Hot LRS",
			Tags:         map[string]string{"size_gb": "60000"},
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() banded blob failed: %v", err)
	}
	// 51200 GB at 0.0208 plus 8800 GB at 0.019968.
	wantBanded := 51200*0.0208 + 8800*0.019968
	if math.Abs(banded.GetCostPerMonth()-wantBanded) > 1e-6 {
		t.Fatalf("banded cost_per_month = %v, want %v", banded.GetCostPerMonth(), wantBanded)
	}
}

func TestChosenBlobStoredWriteOperationsReturnsNotFound(t *testing.T) {
	t.Parallel()

	items := []azureclient.PriceItem{
		{MeterName: "Hot LRS Write Operations", RetailPrice: 0.0001, CurrencyCode: "USD"},
		{MeterName: "Hot LRS List Operations", RetailPrice: 0.0005, CurrencyCode: "USD"},
	}
	_, err := chosenBlobStored(items)
	if !errors.Is(err, azureclient.ErrNotFound) {
		t.Fatalf("chosenBlobStored() error = %v, want ErrNotFound", err)
	}
}

func TestGetProjectedCostValidation(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	vm := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		Region:       "eastus",
		Sku:          "Standard_B1s",
	}

	tests := []struct {
		name string
		req  *finfocusv1.GetProjectedCostRequest
		code codes.Code
	}{
		{
			name: "missing sku",
			req: &finfocusv1.GetProjectedCostRequest{
				Resource: &finfocusv1.ResourceDescriptor{
					Provider:     "azure",
					ResourceType: "compute/VirtualMachine",
					Region:       "eastus",
				},
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing region",
			req: &finfocusv1.GetProjectedCostRequest{
				Resource: &finfocusv1.ResourceDescriptor{
					Provider:     "azure",
					ResourceType: "compute/VirtualMachine",
					Sku:          "Standard_B1s",
				},
			},
			code: codes.InvalidArgument,
		},
		{
			name: "unsupported type",
			req: &finfocusv1.GetProjectedCostRequest{
				Resource: &finfocusv1.ResourceDescriptor{
					Provider:     "azure",
					ResourceType: "compute/VirtualMachineScaleSet",
					Region:       "eastus",
					Sku:          "Standard_B1s",
				},
			},
			code: codes.Unimplemented,
		},
		{
			name: "wrong provider",
			req: &finfocusv1.GetProjectedCostRequest{
				Resource: &finfocusv1.ResourceDescriptor{
					Provider:     "aws",
					ResourceType: "compute/VirtualMachine",
					Region:       "eastus",
					Sku:          "Standard_B1s",
				},
			},
			code: codes.InvalidArgument,
		},
		{
			name: "client not configured",
			req:  &finfocusv1.GetProjectedCostRequest{Resource: vm},
			code: codes.Unimplemented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := calc.GetProjectedCost(context.Background(), tt.req)
			if status.Code(err) != tt.code {
				t.Fatalf("code = %s, want %s (err=%v)", status.Code(err), tt.code, err)
			}
		})
	}
}

func TestGetProjectedCostNilClientNamesTask(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	_, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "compute/VirtualMachine",
			Region:       "eastus",
			Sku:          "Standard_B1s",
		},
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unimplemented {
		t.Fatalf("expected Unimplemented, got %v", err)
	}
	if !strings.Contains(st.Message(), "AZ-2.1") {
		t.Fatalf("message %q does not name AZ-2.1", st.Message())
	}
}

func TestGetActualCostNilClientNamesTask(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	_, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags: map[string]string{
			"region": "eastus",
			"sku":    "Standard_B1s",
		},
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unimplemented {
		t.Fatalf("expected Unimplemented, got %v", err)
	}
	if !strings.Contains(st.Message(), "AZ-2.2") {
		t.Fatalf("message %q does not name AZ-2.2", st.Message())
	}
}

func TestGetActualCostScalesByRuntimeAndConfidence(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{{
		RetailPrice:  0.0104,
		CurrencyCode: "USD",
	}})

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	resp, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags: map[string]string{
			"region": "eastus",
			"sku":    "Standard_B1s",
		},
		Start: timestamppb.New(start),
		End:   timestamppb.New(end),
	})
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	if len(resp.GetResults()) != 1 {
		t.Fatalf("results = %d", len(resp.GetResults()))
	}

	result := resp.GetResults()[0]
	want := 0.0104 * 24
	if math.Abs(result.GetCost()-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", result.GetCost(), want)
	}
	if result.GetUsageAmount() != 24 {
		t.Fatalf("usage_amount = %v, want 24", result.GetUsageAmount())
	}
	if result.GetUsageUnit() != "hours" {
		t.Fatalf("usage_unit = %q", result.GetUsageUnit())
	}
	if result.GetSource() != "azure-retail-prices[confidence:HIGH]" {
		t.Fatalf("source = %q", result.GetSource())
	}
}

func TestGetActualCostConfidenceLevels(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{{
		RetailPrice:  0.02,
		CurrencyCode: "USD",
	}})

	startOnly, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags:  map[string]string{"region": "eastus", "sku": "Standard_B1s"},
		Start: timestamppb.New(time.Now().UTC().Add(-2 * time.Hour)),
	})
	if err != nil {
		t.Fatalf("start-only GetActualCost() failed: %v", err)
	}
	if startOnly.GetResults()[0].GetSource() != "azure-retail-prices[confidence:MEDIUM]" {
		t.Fatalf("start-only source = %q", startOnly.GetResults()[0].GetSource())
	}

	neither, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags: map[string]string{"region": "eastus", "sku": "Standard_B1s"},
	})
	if err != nil {
		t.Fatalf("no-window GetActualCost() failed: %v", err)
	}
	got := neither.GetResults()[0]
	if got.GetSource() != "azure-retail-prices[confidence:LOW]" {
		t.Fatalf("no-window source = %q", got.GetSource())
	}
	want := 0.02 * pluginsdk.HoursPerMonth
	if math.Abs(got.GetCost()-want) > 1e-9 {
		t.Fatalf("assumed-month cost = %v, want %v", got.GetCost(), want)
	}
}

func TestGetActualCostRejectsInvertedWindowAndMissingSKU(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	start := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	_, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags:  map[string]string{"region": "eastus", "sku": "Standard_B1s"},
		Start: timestamppb.New(start),
		End:   timestamppb.New(end),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("inverted window code = %s, err=%v", status.Code(err), err)
	}

	_, err = calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags: map[string]string{
			"region":        "eastus",
			"resource_type": "compute/VirtualMachine",
		},
		Start: timestamppb.New(end),
		End:   timestamppb.New(start),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("missing sku code = %s, err=%v", status.Code(err), err)
	}
}

func TestGetActualCostDiskAndBlob(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		{
			ProductName:  "Premium SSD Managed Disks",
			SkuName:      "P10 LRS",
			MeterName:    "P10 LRS Disk",
			RetailPrice:  19.71,
			CurrencyCode: "USD",
		},
		{
			ProductName:  generalBlockBlobV2Product,
			SkuName:      "Hot LRS",
			MeterName:    "Hot LRS Data Stored",
			RetailPrice:  0.02,
			CurrencyCode: "USD",
		},
	})

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(730 * time.Hour)

	disk, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags: map[string]string{
			"region":        "eastus",
			"sku":           "Premium_SSD_LRS",
			"resource_type": "storage/ManagedDisk",
			"size_gb":       "128",
		},
		Start: timestamppb.New(start),
		End:   timestamppb.New(end),
	})
	if err != nil {
		t.Fatalf("disk GetActualCost() failed: %v", err)
	}
	if math.Abs(disk.GetResults()[0].GetCost()-19.71) > 1e-9 {
		t.Fatalf("disk cost = %v, want 19.71", disk.GetResults()[0].GetCost())
	}

	blob, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags: map[string]string{
			"region":        "eastus",
			"sku":           "Hot LRS",
			"resource_type": "storage/BlobStorage",
			"size_gb":       "50",
		},
		Start: timestamppb.New(start),
		End:   timestamppb.New(end),
	})
	if err != nil {
		t.Fatalf("blob GetActualCost() failed: %v", err)
	}
	if math.Abs(blob.GetResults()[0].GetCost()-1.0) > 1e-9 {
		t.Fatalf("blob cost = %v, want 1.0", blob.GetResults()[0].GetCost())
	}
}

func newPricingCalc(t *testing.T, items []azureclient.PriceItem) *Calculator {
	t.Helper()

	calc, _ := newCapturingPricingCalc(t, items)
	return calc
}

// newCapturingPricingCalc is newPricingCalc that also returns the $filter of
// every request the fake server received, in order.
func newCapturingPricingCalc(t *testing.T, items []azureclient.PriceItem) (*Calculator, func() []string) {
	t.Helper()

	var (
		mu      sync.Mutex
		filters []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		mu.Lock()
		filters = append(filters, filter)
		mu.Unlock()
		matched := retailFakeItems(filter, items)
		resp := azureclient.PriceResponse{Items: matched, Count: len(matched)}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	seen := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), filters...)
	}
	return NewCalculator(zerolog.Nop(), cached), seen
}

var (
	productNameFilter = regexp.MustCompile(`\bproductName eq '((?:[^']|'')*)'`)
	skuNameFilter     = regexp.MustCompile(`\bskuName eq '((?:[^']|'')*)'`)
)

// retailFakeItems keeps the rows that match the productName and skuName
// conditions of an OData filter, comparing exactly (the live API is
// case-sensitive). Only those two fields are applied, only the first
// condition for each field is read, and an or between conditions is not
// understood. Every other field is left to the selectors under test.
func retailFakeItems(filter string, items []azureclient.PriceItem) []azureclient.PriceItem {
	product, hasProduct := filterEquals(filter, productNameFilter)
	sku, hasSKU := filterEquals(filter, skuNameFilter)
	if !hasProduct && !hasSKU {
		return items
	}

	kept := make([]azureclient.PriceItem, 0, len(items))
	for _, item := range items {
		if hasProduct && item.ProductName != product {
			continue
		}
		if hasSKU && item.SkuName != sku {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func filterEquals(filter string, pattern *regexp.Regexp) (string, bool) {
	match := pattern.FindStringSubmatch(filter)
	if match == nil {
		return "", false
	}
	return strings.ReplaceAll(match[1], "''", "'"), true
}
