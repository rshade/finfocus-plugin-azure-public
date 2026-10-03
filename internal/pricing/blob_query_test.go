package pricing

import (
	"context"
	"math"
	"strings"
	"testing"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestQuoteBlob_Query_UsesGeneralBlockBlobV2Product(t *testing.T) {
	t.Parallel()

	calc, filters := newCapturingPricingCalc(t, []azureclient.PriceItem{{
		ProductName:  generalBlockBlobV2Product,
		SkuName:      "Hot ZRS",
		MeterName:    "Hot ZRS Data Stored",
		RetailPrice:  0.026,
		CurrencyCode: "USD",
	}})

	if _, err := calc.GetProjectedCost(context.Background(), blobRequest("eastus", "Hot ZRS")); err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}

	seen := filters()
	if len(seen) != 1 {
		t.Fatalf("requests = %d, want 1", len(seen))
	}
	for _, want := range []string{
		"productName eq '" + generalBlockBlobV2Product + "'",
		"skuName eq 'Hot ZRS'",
	} {
		if !strings.Contains(seen[0], want) {
			t.Fatalf("$filter = %q, want it to contain %q", seen[0], want)
		}
	}
}

// Live rows for israelcentral, read 2026-10-03: General Block Blob v2 sells no
// Hot GRS there, and the legacy Blob Storage product does.
func TestQuoteBlob_ProductWithoutSKU_FallsBackToLegacyProduct(t *testing.T) {
	t.Parallel()

	legacyHotGRS := azureclient.PriceItem{
		ProductName:  legacyBlobStorageProduct,
		SkuName:      "Hot GRS",
		MeterName:    "Hot GRS Data Stored",
		RetailPrice:  0.065494,
		CurrencyCode: "USD",
	}
	gpv2HotGRS := legacyHotGRS
	gpv2HotGRS.ProductName = generalBlockBlobV2Product
	gpv2HotGRS.RetailPrice = 0.05

	tests := []struct {
		name         string
		items        []azureclient.PriceItem
		wantMonthly  float64
		wantRequests int
		wantLegacy   bool
	}{
		{
			name:         "general block blob v2 row wins",
			items:        []azureclient.PriceItem{legacyHotGRS, gpv2HotGRS},
			wantMonthly:  5.0,
			wantRequests: 1,
		},
		{
			name:         "legacy row prices a sku general block blob v2 does not sell",
			items:        []azureclient.PriceItem{legacyHotGRS},
			wantMonthly:  6.5494,
			wantRequests: 2,
			wantLegacy:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calc, filters := newCapturingPricingCalc(t, tt.items)
			resp, err := calc.GetProjectedCost(context.Background(), blobRequest("israelcentral", "Hot GRS"))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			if math.Abs(resp.GetCostPerMonth()-tt.wantMonthly) > 1e-9 {
				t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), tt.wantMonthly)
			}
			if got := len(filters()); got != tt.wantRequests {
				t.Fatalf("requests = %d, want %d", got, tt.wantRequests)
			}
			hasNote := strings.Contains(resp.GetBillingDetail(), legacyBlobStorageProduct)
			if hasNote != tt.wantLegacy {
				t.Fatalf("billing_detail = %q, legacy note present = %v, want %v",
					resp.GetBillingDetail(), hasNote, tt.wantLegacy)
			}
		})
	}
}

func TestQuoteBlob_NeitherProductSellsSKU_ReturnsNotFoundNamingSKU(t *testing.T) {
	t.Parallel()

	calc, filters := newCapturingPricingCalc(t, []azureclient.PriceItem{{
		ProductName:  generalBlockBlobV2Product,
		SkuName:      "Hot LRS",
		MeterName:    "Hot LRS Data Stored",
		RetailPrice:  0.021,
		CurrencyCode: "USD",
	}})

	_, err := calc.GetProjectedCost(context.Background(), blobRequest("israelcentral", "Cold RA-GRS"))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound (err %v)", status.Code(err), err)
	}
	if !strings.Contains(err.Error(), "Cold RA-GRS") {
		t.Fatalf("error %q does not name the sku", err)
	}
	if got := len(filters()); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func blobRequest(region, sku string) *finfocusv1.GetProjectedCostRequest {
	return &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "storage/BlobStorage",
			Region:       region,
			Sku:          sku,
			Tags:         map[string]string{"size_gb": "100"},
		},
	}
}

func TestRetailFakeItems_FilterFields_DropMismatchedRows(t *testing.T) {
	t.Parallel()

	items := []azureclient.PriceItem{
		{ProductName: "Blob Storage", SkuName: "Hot LRS", MeterName: "legacy"},
		{ProductName: generalBlockBlobV2Product, SkuName: "Hot LRS", MeterName: "gpv2 lrs"},
		{ProductName: generalBlockBlobV2Product, SkuName: "Hot ZRS", MeterName: "gpv2 zrs"},
		{ProductName: "SQL Server O'Brien Edition", MeterName: "quoted"},
	}

	tests := []struct {
		name   string
		filter string
		want   []string
	}{
		{
			name:   "no product or sku condition keeps every row",
			filter: "armRegionName eq 'eastus' and serviceName eq 'Storage'",
			want:   []string{"legacy", "gpv2 lrs", "gpv2 zrs", "quoted"},
		},
		{
			name:   "product condition drops other products",
			filter: "productName eq 'General Block Blob v2' and serviceName eq 'Storage'",
			want:   []string{"gpv2 lrs", "gpv2 zrs"},
		},
		{
			name:   "product and sku conditions both apply",
			filter: "productName eq 'Blob Storage' and skuName eq 'Hot ZRS'",
			want:   nil,
		},
		{
			name:   "escaped quote in a value is unescaped",
			filter: "productName eq 'SQL Server O''Brien Edition'",
			want:   []string{"quoted"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := retailFakeItems(tt.filter, items)
			names := make([]string, 0, len(got))
			for _, item := range got {
				names = append(names, item.MeterName)
			}
			if strings.Join(names, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("rows = %v, want %v", names, tt.want)
			}
		})
	}
}
