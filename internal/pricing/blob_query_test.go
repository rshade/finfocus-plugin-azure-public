package pricing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestQuoteBlob_Query_UsesGeneralBlockBlobV2Product(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		filters []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		filters = append(filters, r.URL.Query().Get("$filter"))
		mu.Unlock()
		resp := azureclient.PriceResponse{Items: []azureclient.PriceItem{{
			ProductName:  generalBlockBlobV2Product,
			SkuName:      "Hot ZRS",
			MeterName:    "Hot ZRS Data Stored",
			RetailPrice:  0.026,
			CurrencyCode: "USD",
		}}, Count: 1}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	calc := NewCalculator(zerolog.Nop(), cached)

	if _, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "storage/BlobStorage",
			Region:       "eastus",
			Sku:          "Hot ZRS",
			Tags:         map[string]string{"size_gb": "100"},
		},
	}); err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(filters) != 1 {
		t.Fatalf("requests = %d, want 1", len(filters))
	}
	want := "productName eq '" + generalBlockBlobV2Product + "'"
	if !strings.Contains(filters[0], want) {
		t.Fatalf("$filter = %q, want it to contain %q", filters[0], want)
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
