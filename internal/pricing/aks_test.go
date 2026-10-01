package pricing

import (
	"context"
	"encoding/json"
	"errors"
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

const (
	aksFixturePath = "testdata/retail/aks/eastus_consumption.json"
	vmFixturePath  = "testdata/retail/spot/standard_d2s_v3_eastus.json"

	aksCanonicalType = "containerservice/KubernetesCluster"
	aksPulumiType    = "azure:containerservice/kubernetesCluster:KubernetesCluster"

	aksStandardMeter = "Standard Uptime SLA"
	aksLTSMeter      = "Standard Long Term Support"
	aksFreeMeter     = "FreeTierInfrastructureCost Uptime SLA"

	aksComponentControl = "control_plane"
)

func TestGetProjectedCostAKSStandardTwoPools(t *testing.T) {
	t.Parallel()

	aksLoaded := loadRetailFixture(t, aksFixturePath)
	vmLoaded := loadRetailFixture(t, vmFixturePath)
	if aksLoaded.NextPageLink != "" {
		t.Fatalf("NextPageLink = %q, want one page", aksLoaded.NextPageLink)
	}
	control := requireOpenAKSMeter(t, aksLoaded.Items, aksStandardMeter)
	vm := fixtureVMItem(t, vmLoaded.Items, false)
	spot := fixtureVMItem(t, vmLoaded.Items, true)
	if control.RetailPrice == 0 || vm.RetailPrice == 0 {
		t.Fatal("control plane or on-demand retail price is zero")
	}
	if vm.RetailPrice <= spot.RetailPrice {
		t.Fatalf("on-demand %v is not above spot %v", vm.RetailPrice, spot.RetailPrice)
	}

	node := vm.RetailPrice * pluginsdk.HoursPerMonth
	spotNode := spot.RetailPrice * pluginsdk.HoursPerMonth
	controlMonthly := control.RetailPrice * pluginsdk.HoursPerMonth
	tests := []struct {
		name         string
		resourceType string
		tags         map[string]string
		want         map[string]float64
	}{
		{
			name:         "canonical defaults",
			resourceType: aksCanonicalType,
			tags:         twoPoolTags(vm.ArmSkuName, nil),
			want: map[string]float64{
				aksComponentControl: controlMonthly,
				"node_pool_pool_1":  node * 2,
				"node_pool_pool_2":  node,
			},
		},
		{
			name:         "pulumi type and named pool",
			resourceType: aksPulumiType,
			tags: twoPoolTags(vm.ArmSkuName, map[string]string{
				"node_pool_1_name": "System",
			}),
			want: map[string]float64{
				aksComponentControl: controlMonthly,
				"node_pool_system":  node * 2,
				"node_pool_pool_2":  node,
			},
		},
		{
			name:         "priority spot does not price nodes as spot",
			resourceType: "CONTAINERSERVICE/KUBERNETESCLUSTER",
			tags:         twoPoolTags(vm.ArmSkuName, map[string]string{"priority": "Spot"}),
			want: map[string]float64{
				aksComponentControl: controlMonthly,
				"node_pool_pool_1":  node * 2,
				"node_pool_pool_2":  node,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calc, filters := newAKSPricingCalc(t, aksLoaded.Items, vmLoaded.Items)
			resp, err := calc.GetProjectedCost(context.Background(), aksRequest(tt.resourceType, "Standard", tt.tags))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			assertAKSQuote(t, resp, control.RetailPrice, tt.want)
			if resp.GetCurrency() != control.CurrencyCode {
				t.Fatalf("currency = %q, want %q", resp.GetCurrency(), control.CurrencyCode)
			}
			if resp.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD {
				t.Fatalf("pricing_category = %s", resp.GetPricingCategory())
			}
			detail := resp.GetBillingDetail()
			if !strings.Contains(detail, "Standard") ||
				strings.Contains(detail, "Long Term") ||
				!strings.Contains(detail, "730") {
				t.Fatalf("billing_detail = %q", detail)
			}
			poolKey := "node_pool_pool_1"
			if _, named := tt.want["node_pool_system"]; named {
				poolKey = "node_pool_system"
			}
			if math.Abs(resp.GetCostBreakdown()[poolKey]-(spotNode*2)) <= 1e-9 {
				t.Fatal("node pool used the spot monthly price")
			}
			assertAKSPriceFilter(t, filters.aks)
			if !strings.Contains(filters.vm, "serviceName eq 'Virtual Machines'") ||
				!strings.Contains(filters.vm, "armSkuName eq '"+vm.ArmSkuName+"'") {
				t.Fatalf("vm filter = %q", filters.vm)
			}
		})
	}
}

func TestGetProjectedCostAKSFreeOpenMeter(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	open, closed := splitAKSMeter(t, loaded.Items, aksFreeMeter)
	if len(closed) == 0 {
		t.Fatal("free meter has no row with effectiveEndDate set")
	}
	if open.RetailPrice == 0 {
		t.Fatal("open free retail price is zero")
	}

	calc, _ := newAKSPricingCalc(t, loaded.Items, nil)
	resp, err := calc.GetProjectedCost(context.Background(), aksRequest(aksCanonicalType, "Free", nil))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if resp.GetCostPerMonth() == 0 || resp.GetUnitPrice() == 0 {
		t.Fatal("free tier result is zero")
	}
	assertAKSQuote(t, resp, open.RetailPrice, map[string]float64{
		aksComponentControl: open.RetailPrice * pluginsdk.HoursPerMonth,
	})
	for _, item := range closed {
		if math.Abs(resp.GetUnitPrice()-item.RetailPrice) <= 1e-9 {
			t.Fatalf("used closed free row price %v", item.RetailPrice)
		}
	}
	if !strings.Contains(resp.GetBillingDetail(), "Free") {
		t.Fatalf("billing_detail = %q", resp.GetBillingDetail())
	}
}

func TestGetProjectedCostAKSLongTermSupport(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	standard := requireOpenAKSMeter(t, loaded.Items, aksStandardMeter)
	lts := requireOpenAKSMeter(t, loaded.Items, aksLTSMeter)
	if lts.RetailPrice == standard.RetailPrice {
		t.Fatal("long term support price matches standard uptime")
	}

	calc, _ := newAKSPricingCalc(t, loaded.Items, nil)
	resp, err := calc.GetProjectedCost(context.Background(), aksRequest(
		aksCanonicalType,
		"standard",
		map[string]string{"support": "LTS"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	assertAKSQuote(t, resp, lts.RetailPrice, map[string]float64{
		aksComponentControl: lts.RetailPrice * pluginsdk.HoursPerMonth,
	})
	if !strings.Contains(resp.GetBillingDetail(), "Long Term") {
		t.Fatalf("billing_detail = %q", resp.GetBillingDetail())
	}
}

func TestGetProjectedCostAKSTierSources(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	standard := requireOpenAKSMeter(t, loaded.Items, aksStandardMeter)
	free := requireOpenAKSMeter(t, loaded.Items, aksFreeMeter)

	tests := []struct {
		name string
		sku  string
		tags map[string]string
		want azureclient.PriceItem
	}{
		{
			name: "tag tier",
			tags: map[string]string{"tier": "free"},
			want: free,
		},
		{
			name: "tag sku",
			tags: map[string]string{"sku": "FREE"},
			want: free,
		},
		{
			name: "sku beats tier tag",
			sku:  "Standard",
			tags: map[string]string{"tier": "Free"},
			want: standard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calc, _ := newAKSPricingCalc(t, loaded.Items, nil)
			resp, err := calc.GetProjectedCost(context.Background(), aksRequest(aksCanonicalType, tt.sku, tt.tags))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			assertAKSQuote(t, resp, tt.want.RetailPrice, map[string]float64{
				aksComponentControl: tt.want.RetailPrice * pluginsdk.HoursPerMonth,
			})
		})
	}
}

func TestGetProjectedCostAKSRejectsTier(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	tests := []struct {
		name string
		sku  string
		tags map[string]string
		text string
	}{
		{name: "missing tier", text: "tier"},
		{name: "automatic", sku: "Automatic", text: "Automatic"},
		{name: "automatic tag", tags: map[string]string{"tier": "automatic"}, text: "automatic"},
		{
			name: "free long term support",
			sku:  "Free",
			tags: map[string]string{"support": "lts"},
			text: "lts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calc, _ := newAKSPricingCalc(t, loaded.Items, nil)
			_, err := calc.GetProjectedCost(context.Background(), aksRequest(aksCanonicalType, tt.sku, tt.tags))
			assertInvalidArgument(t, err, tt.text)
		})
	}
}

func TestGetProjectedCostAKSRejectsNodePools(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	vmLoaded := loadRetailFixture(t, vmFixturePath)
	vm := fixtureVMItem(t, vmLoaded.Items, false)
	tests := []struct {
		name string
		tags map[string]string
		text string
	}{
		{
			name: "missing count",
			tags: map[string]string{"node_pool_1_sku": vm.ArmSkuName},
			text: "node_pool_1_count",
		},
		{
			name: "zero count",
			tags: map[string]string{"node_pool_1_sku": vm.ArmSkuName, "node_pool_1_count": "0"},
			text: "node_pool_1_count",
		},
		{
			name: "invalid count",
			tags: map[string]string{"node_pool_1_sku": vm.ArmSkuName, "node_pool_1_count": "1.5"},
			text: "node_pool_1_count",
		},
		{
			name: "missing sku",
			tags: map[string]string{"node_pool_1_count": "1"},
			text: "node_pool_1_sku",
		},
		{
			name: "gap",
			tags: map[string]string{
				"node_pool_2_sku":   vm.ArmSkuName,
				"node_pool_2_count": "1",
			},
			text: "node_pool_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calc, _ := newAKSPricingCalc(t, loaded.Items, vmLoaded.Items)
			_, err := calc.GetProjectedCost(context.Background(), aksRequest(aksCanonicalType, "Standard", tt.tags))
			assertInvalidArgument(t, err, tt.text)
		})
	}
}

func TestGetProjectedCostAKSNodeNotFound(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	calc, _ := newAKSPricingCalc(t, loaded.Items, []azureclient.PriceItem{})
	_, err := calc.GetProjectedCost(context.Background(), aksRequest(
		aksCanonicalType,
		"Standard",
		twoPoolTags("Standard_D2s_v3", nil),
	))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (err=%v)", status.Code(err), err)
	}
}

func TestGetProjectedCostAKSMissingMeterIsNotFound(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	calc, _ := newAKSPricingCalc(t, withoutMeterName(loaded.Items, aksStandardMeter), nil)
	_, err := calc.GetProjectedCost(context.Background(), aksRequest(aksCanonicalType, "Standard", nil))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (err=%v)", status.Code(err), err)
	}
	if !strings.Contains(status.Convert(err).Message(), aksStandardMeter) {
		t.Fatalf("message %q does not name %q", status.Convert(err).Message(), aksStandardMeter)
	}
}

func TestGetProjectedCostAKSFreeSelectionErrors(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	open := requireOpenAKSMeter(t, loaded.Items, aksFreeMeter)

	closed := append([]azureclient.PriceItem(nil), loaded.Items...)
	for i := range closed {
		if closed[i].MeterName == aksFreeMeter {
			closed[i].EffectiveEndDate = "2020-01-01T00:00:00Z"
		}
	}
	other := open
	other.RetailPrice = open.RetailPrice + 1
	other.MeterID += "-other"
	ambiguous := append(append([]azureclient.PriceItem(nil), loaded.Items...), other)
	same := open
	same.MeterID += "-same"
	duplicates := append(append([]azureclient.PriceItem(nil), loaded.Items...), same)
	otherCurrency := open
	otherCurrency.CurrencyCode = "EUR"
	otherCurrency.MeterID += "-eur"
	mixedCurrency := append(append([]azureclient.PriceItem(nil), loaded.Items...), otherCurrency)

	tests := []struct {
		name  string
		items []azureclient.PriceItem
		code  codes.Code
	}{
		{name: "no open row", items: closed, code: codes.InvalidArgument},
		{name: "different open prices", items: ambiguous, code: codes.InvalidArgument},
		{name: "different open currencies", items: mixedCurrency, code: codes.InvalidArgument},
		{name: "same open price", items: duplicates, code: codes.OK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calc, _ := newAKSPricingCalc(t, tt.items, nil)
			resp, err := calc.GetProjectedCost(context.Background(), aksRequest(aksCanonicalType, "Free", nil))
			if tt.code != codes.OK {
				assertInvalidArgument(t, err, aksFreeMeter)
				return
			}
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			assertAKSQuote(t, resp, open.RetailPrice, map[string]float64{
				aksComponentControl: open.RetailPrice * pluginsdk.HoursPerMonth,
			})
		})
	}
}

func TestGetProjectedCostAKSOverGRPC(t *testing.T) {
	t.Parallel()

	aksLoaded := loadRetailFixture(t, aksFixturePath)
	vmLoaded := loadRetailFixture(t, vmFixturePath)
	control := requireOpenAKSMeter(t, aksLoaded.Items, aksStandardMeter)
	vm := fixtureVMItem(t, vmLoaded.Items, false)
	node := vm.RetailPrice * pluginsdk.HoursPerMonth
	calc, _ := newAKSPricingCalc(t, aksLoaded.Items, vmLoaded.Items)
	client := dialPricingClient(t, calc)

	resp, err := client.GetProjectedCost(context.Background(), aksRequest(
		aksCanonicalType,
		"Standard",
		twoPoolTags(vm.ArmSkuName, nil),
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	assertAKSQuote(t, resp, control.RetailPrice, map[string]float64{
		aksComponentControl: control.RetailPrice * pluginsdk.HoursPerMonth,
		"node_pool_pool_1":  node * 2,
		"node_pool_pool_2":  node,
	})
}

func TestGetProjectedCostAKSPrefixIsUnsupported(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	_, err := calc.GetProjectedCost(context.Background(), aksRequest(
		"containerservice/kubernetesclusterextra",
		"Standard",
		nil,
	))
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("code = %s, want Unimplemented (err=%v)", status.Code(err), err)
	}
}

func TestMapDescriptorToQueryAKS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desc    *finfocusv1.ResourceDescriptor
		wantErr error
		tier    string
	}{
		{
			name: "canonical",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: aksCanonicalType,
				Region:       "eastus",
				Sku:          "Standard",
			},
		},
		{
			name: "pulumi",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: aksPulumiType,
				Region:       "eastus",
				Sku:          "Free",
			},
		},
		{
			name: "tier tag",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "ContainerService/KubernetesCluster",
				Region:       "eastus",
				Tags:         map[string]string{"tier": "Standard"},
			},
			tier: "tier",
		},
		{
			name: "missing tier",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: aksCanonicalType,
				Region:       "eastus",
			},
			wantErr: ErrMissingRequiredFields,
			tier:    "tier",
		},
		{
			name: "prefix is unsupported",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "containerservice/kubernetesclusterextra",
				Region:       "eastus",
				Sku:          "Standard",
			},
			wantErr: ErrUnsupportedResourceType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			query, err := MapDescriptorToQuery(tt.desc)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if tt.tier != "" && !strings.Contains(err.Error(), tt.tier) {
					t.Fatalf("err = %v, want it to name %s", err, tt.tier)
				}
				return
			}
			if err != nil {
				t.Fatalf("MapDescriptorToQuery() failed: %v", err)
			}
			if query.ServiceName != "Azure Kubernetes Service" {
				t.Fatalf("ServiceName = %q", query.ServiceName)
			}
			if query.ArmSkuName != "" || query.ProductName != "" {
				t.Fatalf("ArmSkuName = %q ProductName = %q, want empty", query.ArmSkuName, query.ProductName)
			}
			if query.ArmRegionName != "eastus" || query.CurrencyCode != "USD" {
				t.Fatalf("region = %q currency = %q", query.ArmRegionName, query.CurrencyCode)
			}
		})
	}
}

func TestSupportsAKS(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	supported, err := calc.Supports(context.Background(), &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: aksPulumiType,
			Region:       "eastus",
			Tags:         map[string]string{"tier": "Standard"},
		},
	})
	if err != nil {
		t.Fatalf("Supports() error = %v", err)
	}
	if !supported.GetSupported() {
		t.Fatalf("Supported = false, reason %q", supported.GetReason())
	}

	unknown, err := calc.Supports(context.Background(), &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "containerservice/kubernetesclusterextra",
			Region:       "eastus",
			Sku:          "Standard",
		},
	})
	if err != nil {
		t.Fatalf("Supports() unknown error = %v", err)
	}
	if unknown.GetSupported() {
		t.Fatal("prefix type is supported")
	}
}

func assertAKSQuote(t *testing.T, resp *finfocusv1.GetProjectedCostResponse, unit float64, want map[string]float64) {
	t.Helper()

	if err := pluginsdk.ValidateGetProjectedCostResponse(resp); err != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", err)
	}
	if math.Abs(resp.GetUnitPrice()-unit) > 1e-9 {
		t.Fatalf("unit_price = %v, want %v", resp.GetUnitPrice(), unit)
	}
	if _, ok := resp.GetCostBreakdown()["compute"]; ok {
		t.Fatalf("breakdown includes compute: %v", resp.GetCostBreakdown())
	}
	if len(resp.GetCostBreakdown()) != len(want) {
		t.Fatalf("breakdown = %v, want %v", resp.GetCostBreakdown(), want)
	}
	var sum float64
	for key, wantCost := range want {
		got := resp.GetCostBreakdown()[key]
		if math.Abs(got-wantCost) > 1e-9 {
			t.Fatalf("breakdown %s = %v, want %v (all %v)", key, got, wantCost, resp.GetCostBreakdown())
		}
		sum += wantCost
	}
	if math.Abs(sum-resp.GetCostPerMonth()) > 1e-9 {
		t.Fatalf("cost_per_month = %v, component sum = %v", resp.GetCostPerMonth(), sum)
	}
}

func assertAKSPriceFilter(t *testing.T, filter string) {
	t.Helper()

	if strings.Contains(filter, "armSkuName") || strings.Contains(filter, "productName") {
		t.Fatalf("aks filter includes sku or product: %s", filter)
	}
	for _, want := range []string{
		"armRegionName eq 'eastus'",
		"serviceName eq 'Azure Kubernetes Service'",
		"priceType eq 'Consumption'",
		"currencyCode eq 'USD'",
	} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter %q does not contain %q", filter, want)
		}
	}
}

func newAKSPricingCalc(
	t *testing.T,
	aksItems, vmItems []azureclient.PriceItem,
) (*Calculator, *aksCapturedFilters) {
	t.Helper()

	got := &aksCapturedFilters{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		var items []azureclient.PriceItem
		switch {
		case strings.Contains(filter, "Azure Kubernetes Service"):
			got.aks = filter
			items = aksItems
		case strings.Contains(filter, "Virtual Machines"):
			got.vm = filter
			items = vmItems
		default:
			t.Errorf("unexpected price filter %s", filter)
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
	return NewCalculator(zerolog.Nop(), cached), got
}

type aksCapturedFilters struct {
	aks string
	vm  string
}

func aksRequest(resourceType, sku string, tags map[string]string) *finfocusv1.GetProjectedCostRequest {
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

func twoPoolTags(sku string, extra map[string]string) map[string]string {
	tags := map[string]string{
		"node_pool_1_sku":   sku,
		"node_pool_1_count": "2",
		"node_pool_2_sku":   sku,
		"node_pool_2_count": "1",
	}
	for key, value := range extra {
		tags[key] = value
	}
	return tags
}

func requireOpenAKSMeter(t *testing.T, items []azureclient.PriceItem, meter string) azureclient.PriceItem {
	t.Helper()

	open, _ := splitAKSMeter(t, items, meter)
	return open
}

func splitAKSMeter(
	t *testing.T,
	items []azureclient.PriceItem,
	meter string,
) (azureclient.PriceItem, []azureclient.PriceItem) {
	t.Helper()

	var open []azureclient.PriceItem
	var closed []azureclient.PriceItem
	for _, item := range items {
		if item.ProductName != "Azure Kubernetes Service" || item.MeterName != meter {
			continue
		}
		if item.UnitOfMeasure != "1 Hour" || item.Type != "Consumption" {
			continue
		}
		if strings.TrimSpace(item.EffectiveEndDate) == "" {
			open = append(open, item)
			continue
		}
		closed = append(closed, item)
	}
	if len(open) != 1 {
		t.Fatalf("meter %s open rows = %d, want 1", meter, len(open))
	}
	if open[0].RetailPrice == 0 {
		t.Fatalf("meter %s open retail price is zero", meter)
	}
	for _, item := range closed {
		if item.RetailPrice == open[0].RetailPrice {
			t.Fatalf("meter %s closed row repeats open price %v", meter, item.RetailPrice)
		}
	}
	return open[0], closed
}

func withoutMeterName(items []azureclient.PriceItem, meter string) []azureclient.PriceItem {
	kept := make([]azureclient.PriceItem, 0, len(items))
	for _, item := range items {
		if item.MeterName == meter {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}
