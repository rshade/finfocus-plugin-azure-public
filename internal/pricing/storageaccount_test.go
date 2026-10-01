package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestGetProjectedCostStorageAccountHotLRSFromFixture(t *testing.T) {
	t.Parallel()

	loaded := loadStorageAccountFixture(t)
	item := fixtureStorageBaseItem(t, loaded.Items, "Hot LRS")
	const sizeGB = 100.0
	perGBMonth := item.RetailPrice * sizeGB
	times730 := perGBMonth * pluginsdk.HoursPerMonth
	if math.Abs(perGBMonth-times730) <= 1e-9 {
		t.Fatal("fixture price cannot distinguish a 730 multiplier")
	}

	calc := newPricingCalc(t, loaded.Items)
	tests := []struct {
		name         string
		resourceType string
		region       string
		sku          string
		tags         map[string]string
	}{
		{
			name:         "canonical Hot LRS",
			resourceType: "storage/StorageAccount",
			region:       "eastus",
			sku:          "Hot LRS",
			tags:         map[string]string{"size_gb": "100"},
		},
		{
			name:         "pulumi type and lower-case sku",
			resourceType: "azure:storage/storageAccount:StorageAccount",
			region:       "eastus",
			sku:          "hot lrs",
			tags:         map[string]string{"size_gb": "100"},
		},
		{
			name:         "tier and redundancy tags",
			resourceType: "storage/StorageAccount",
			region:       "eastus",
			tags: map[string]string{
				"tier":       "Hot",
				"redundancy": "LRS",
				"size_gb":    "100",
			},
		},
		{
			name:         "access_tier tag",
			resourceType: "storage/StorageAccount",
			region:       "eastus",
			tags: map[string]string{
				"access_tier": "hot",
				"redundancy":  "lrs",
				"size_gb":     "100",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.GetProjectedCost(context.Background(), storageAccountRequest(
				tt.resourceType, tt.region, tt.sku, tt.tags,
			))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
				t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", validateErr)
			}

			got := resp.GetCostPerMonth()
			if math.Abs(got-perGBMonth) > 1e-9 || math.Abs(got-times730) <= 1e-9 {
				t.Fatalf(
					"cost_per_month = %v, want %v (retailPrice * size_gb), not %v (times 730)",
					got, perGBMonth, times730,
				)
			}
			if math.Abs(resp.GetUnitPrice()-item.RetailPrice) > 1e-9 {
				t.Fatalf("unit_price = %v, want retailPrice %v", resp.GetUnitPrice(), item.RetailPrice)
			}
			if resp.GetCurrency() != item.CurrencyCode {
				t.Fatalf("currency = %q, want %q", resp.GetCurrency(), item.CurrencyCode)
			}
			if stored := resp.GetCostBreakdown()["storage"]; math.Abs(stored-got) > 1e-9 {
				t.Fatalf("storage breakdown = %v, cost_per_month = %v", stored, got)
			}
		})
	}
}

func TestGetProjectedCostStorageAccountOverGRPC(t *testing.T) {
	t.Parallel()

	loaded := loadStorageAccountFixture(t)
	item := fixtureStorageBaseItem(t, loaded.Items, "Hot LRS")
	const sizeGB = 100.0
	wantMonthly := item.RetailPrice * sizeGB
	calc := newPricingCalc(t, loaded.Items)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	finfocusv1.RegisterCostSourceServiceServer(server, pluginsdk.NewServer(calc))
	go func() {
		_ = server.Serve(lis)
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///"+lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc client: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	resp, err := finfocusv1.NewCostSourceServiceClient(conn).GetProjectedCost(
		context.Background(),
		storageAccountRequest(
			"storage/StorageAccount",
			"eastus",
			"Hot LRS",
			map[string]string{"size_gb": "100"},
		),
	)
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	got := resp.GetCostPerMonth()
	if math.Abs(got-wantMonthly) > 1e-9 || math.Abs(got-wantMonthly*pluginsdk.HoursPerMonth) <= 1e-9 {
		t.Fatalf(
			"cost_per_month = %v, want %v (retailPrice * size_gb), not times 730",
			got, wantMonthly,
		)
	}
}

func TestGetProjectedCostStorageAccountSKUBeatsTags(t *testing.T) {
	t.Parallel()

	loaded := loadStorageAccountFixture(t)
	item := fixtureStorageBaseItem(t, loaded.Items, "Cool LRS")
	const sizeGB = 100.0
	calc := newPricingCalc(t, loaded.Items)

	resp, err := calc.GetProjectedCost(context.Background(), storageAccountRequest(
		"storage/StorageAccount",
		"eastus",
		"Cool LRS",
		map[string]string{
			"tier":       "Hot",
			"redundancy": "LRS",
			"size_gb":    "100",
		},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	want := item.RetailPrice * sizeGB
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want Cool LRS %v", resp.GetCostPerMonth(), want)
	}
}

func TestGetProjectedCostStorageAccountMissingMeterIsNotFound(t *testing.T) {
	t.Parallel()

	loaded := loadStorageAccountFixture(t)
	const sku = "Archive ZRS"
	items := loaded.Items
	if storageDataStoredPresent(items, sku) {
		items = withoutStorageDataStored(items, sku)
	}
	if storageDataStoredPresent(items, sku) {
		t.Fatalf("removed %s but a data stored row remains", sku)
	}

	calc := newPricingCalc(t, items)
	_, err := calc.GetProjectedCost(context.Background(), storageAccountRequest(
		"storage/StorageAccount",
		"eastus",
		sku,
		map[string]string{"size_gb": "10"},
	))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (err=%v)", status.Code(err), err)
	}
	message := status.Convert(err).Message()
	if !strings.Contains(message, "Archive") || !strings.Contains(message, "ZRS") {
		t.Fatalf("message %q does not name tier Archive and redundancy ZRS", message)
	}
}

func TestGetProjectedCostStorageAccountRejectsUnknownSKU(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, nil)
	tests := []struct {
		name string
		sku  string
		tags map[string]string
		want string
	}{
		{name: "unknown tier", sku: "Premium LRS", want: "Premium"},
		{name: "unknown redundancy", sku: "Hot LOCAL", want: "LOCAL"},
		{
			name: "unknown access tier tag",
			tags: map[string]string{"access_tier": "Frozen", "redundancy": "LRS", "size_gb": "10"},
			want: "Frozen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tags := tt.tags
			if tags == nil {
				tags = map[string]string{"size_gb": "10"}
			}
			_, err := calc.GetProjectedCost(context.Background(), storageAccountRequest(
				"storage/StorageAccount", "eastus", tt.sku, tags,
			))
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %s, want InvalidArgument (err=%v)", status.Code(err), err)
			}
			if !strings.Contains(status.Convert(err).Message(), tt.want) {
				t.Fatalf("message %q does not name %q", status.Convert(err).Message(), tt.want)
			}
		})
	}
}

func TestGetProjectedCostStorageAccountMissingSize(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, nil)
	_, err := calc.GetProjectedCost(context.Background(), storageAccountRequest(
		"storage/StorageAccount", "eastus", "Hot LRS", nil,
	))
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (err=%v)", status.Code(err), err)
	}
	if !strings.Contains(status.Convert(err).Message(), "size_gb") {
		t.Fatalf("message %q does not name size_gb", status.Convert(err).Message())
	}
}

func TestStorageAccountQueryOmitsArmSKU(t *testing.T) {
	t.Parallel()

	loaded := loadStorageAccountFixture(t)
	var filter string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter = r.URL.Query().Get("$filter")
		resp := azureclient.PriceResponse{Items: loaded.Items, Count: len(loaded.Items)}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	calc := NewCalculator(zerolog.Nop(), cached)

	_, err := calc.GetProjectedCost(context.Background(), storageAccountRequest(
		"storage/StorageAccount",
		"eastus",
		"Hot LRS",
		map[string]string{"size_gb": "100", "currency": "EUR"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if strings.Contains(filter, "armSkuName") {
		t.Fatalf("filter includes armSkuName: %s", filter)
	}
	for _, want := range []string{
		"armRegionName eq 'eastus'",
		"serviceName eq 'Storage'",
		"productName eq 'General Block Blob v2'",
		"priceType eq 'Consumption'",
		"currencyCode eq 'EUR'",
	} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter %q does not contain %q", filter, want)
		}
	}
}

func TestMapDescriptorToQueryStorageAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desc    *finfocusv1.ResourceDescriptor
		wantErr string
	}{
		{
			name: "sku field",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "storage/StorageAccount",
				Region:       "eastus",
				Sku:          "Hot LRS",
			},
		},
		{
			name: "tier and redundancy tags",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "storage/StorageAccount",
				Region:       "eastus",
				Tags:         map[string]string{"tier": "Hot", "redundancy": "LRS"},
			},
		},
		{
			name: "pulumi type",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "azure:storage/storageAccount:StorageAccount",
				Region:       "eastus",
				Sku:          "Hot LRS",
			},
		},
		{
			name: "missing sku",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "storage/StorageAccount",
				Region:       "eastus",
			},
			wantErr: "sku",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			query, err := MapDescriptorToQuery(tt.desc)
			if tt.wantErr != "" {
				if !errors.Is(err, ErrMissingRequiredFields) {
					t.Fatalf("err = %v, want ErrMissingRequiredFields", err)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not name %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MapDescriptorToQuery() failed: %v", err)
			}
			if query.ArmSkuName != "" {
				t.Fatalf("ArmSkuName = %q, want empty", query.ArmSkuName)
			}
			if query.ServiceName != "Storage" {
				t.Fatalf("ServiceName = %q, want Storage", query.ServiceName)
			}
			if query.ProductName != "General Block Blob v2" {
				t.Fatalf("ProductName = %q, want General Block Blob v2", query.ProductName)
			}
			if query.ArmRegionName != "eastus" {
				t.Fatalf("ArmRegionName = %q, want eastus", query.ArmRegionName)
			}
		})
	}
}

func TestSupportsStorageAccountFromTags(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	resp, err := calc.Supports(context.Background(), &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "storage/StorageAccount",
			Region:       "eastus",
			Tags:         map[string]string{"tier": "Hot", "redundancy": "LRS"},
		},
	})
	if err != nil {
		t.Fatalf("Supports() failed: %v", err)
	}
	if !resp.GetSupported() {
		t.Fatalf("supported = false, reason %q", resp.GetReason())
	}
}

func loadStorageAccountFixture(t *testing.T) azureclient.PriceResponse {
	t.Helper()

	data, err := os.ReadFile("testdata/retail/storageaccount/general_block_blob_v2_eastus.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var resp azureclient.PriceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(resp.Items) == 0 {
		t.Fatal("fixture has no items")
	}
	return resp
}

func fixtureStorageBaseItem(t *testing.T, items []azureclient.PriceItem, sku string) azureclient.PriceItem {
	t.Helper()

	var found []azureclient.PriceItem
	for _, item := range items {
		if !storageBaseDataStored(item, sku) {
			continue
		}
		found = append(found, item)
	}
	if len(found) != 1 {
		t.Fatalf("base %s data stored rows = %d, want 1", sku, len(found))
	}
	if found[0].RetailPrice == 0 {
		t.Fatal("fixture retail price is zero")
	}
	return found[0]
}

func storageBaseDataStored(item azureclient.PriceItem, sku string) bool {
	if item.ProductName != "General Block Blob v2" || item.SkuName != sku {
		return false
	}
	if item.MeterName != sku+" Data Stored" || item.UnitOfMeasure != "1 GB/Month" {
		return false
	}
	if item.Type != "Consumption" || item.TierMinimumUnits != 0 {
		return false
	}
	name := item.SkuName + " " + item.MeterName
	return !strings.Contains(name, "TB") && !strings.Contains(name, "PB")
}

func storageDataStoredPresent(items []azureclient.PriceItem, sku string) bool {
	for _, item := range items {
		if item.SkuName == sku && item.MeterName == sku+" Data Stored" {
			return true
		}
	}
	return false
}

func withoutStorageDataStored(items []azureclient.PriceItem, sku string) []azureclient.PriceItem {
	meter := sku + " Data Stored"
	kept := make([]azureclient.PriceItem, 0, len(items))
	for _, item := range items {
		if item.SkuName == sku && item.MeterName == meter {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func storageAccountRequest(
	resourceType, region, sku string,
	tags map[string]string,
) *finfocusv1.GetProjectedCostRequest {
	return &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: resourceType,
			Region:       region,
			Sku:          sku,
			Tags:         tags,
		},
	}
}
