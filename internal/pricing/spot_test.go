package pricing

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestGetProjectedCostSpotVMFromFixture(t *testing.T) {
	t.Parallel()

	loaded := loadSpotRetailFixture(t)
	onDemand := fixtureVMItem(t, loaded.Items, false)
	spot := fixtureVMItem(t, loaded.Items, true)
	onDemandMonthly := onDemand.RetailPrice * pluginsdk.HoursPerMonth
	spotMonthly := spot.RetailPrice * pluginsdk.HoursPerMonth
	if !(onDemandMonthly > spotMonthly) {
		t.Fatalf("on-demand monthly %v is not greater than spot monthly %v", onDemandMonthly, spotMonthly)
	}

	calc := newPricingCalc(t, loaded.Items)
	region := loaded.Items[0].ArmRegionName
	sku := loaded.Items[0].ArmSkuName

	tests := []struct {
		name     string
		priority string
		want     azureclient.PriceItem
		wantSpot bool
		code     codes.Code
	}{
		{
			name: "empty priority is on-demand",
			want: onDemand,
		},
		{
			name:     "priority=Spot",
			priority: "Spot",
			want:     spot,
			wantSpot: true,
		},
		{
			name:     "priority=spot",
			priority: "spot",
			want:     spot,
			wantSpot: true,
		},
		{
			name:     "priority=LowPriority",
			priority: "LowPriority",
			code:     codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.GetProjectedCost(
				context.Background(),
				vmProjectedRequest(region, sku, tt.priority),
			)
			if tt.code != codes.OK {
				if status.Code(err) != tt.code {
					t.Fatalf("code = %s, want %s (err=%v)", status.Code(err), tt.code, err)
				}
				if !strings.Contains(status.Convert(err).Message(), tt.priority) {
					t.Fatalf("message %q does not name %q", status.Convert(err).Message(), tt.priority)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
				t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", validateErr)
			}

			wantMonthly := tt.want.RetailPrice * pluginsdk.HoursPerMonth
			if math.Abs(resp.GetCostPerMonth()-wantMonthly) > 1e-9 {
				t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), wantMonthly)
			}
			if math.Abs(resp.GetUnitPrice()-tt.want.RetailPrice) > 1e-9 {
				t.Fatalf("unit_price = %v, want %v", resp.GetUnitPrice(), tt.want.RetailPrice)
			}
			if resp.GetCurrency() != tt.want.CurrencyCode {
				t.Fatalf("currency = %q, want %q", resp.GetCurrency(), tt.want.CurrencyCode)
			}
			if got := resp.GetCostBreakdown()["compute"]; math.Abs(got-resp.GetCostPerMonth()) > 1e-9 {
				t.Fatalf("compute breakdown = %v, cost_per_month = %v", got, resp.GetCostPerMonth())
			}
			wantCategory := finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD
			if tt.wantSpot {
				wantCategory = finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
			}
			if resp.GetPricingCategory() != wantCategory {
				t.Fatalf("pricing_category = %s, want %s", resp.GetPricingCategory(), wantCategory)
			}
			detail := resp.GetBillingDetail()
			if tt.wantSpot {
				if !strings.Contains(detail, "Spot") || strings.Contains(detail, "On-demand") {
					t.Fatalf("billing_detail = %q, want Spot", detail)
				}
				return
			}
			if !strings.Contains(detail, "On-demand") || strings.Contains(detail, "Spot") {
				t.Fatalf("billing_detail = %q, want On-demand", detail)
			}
		})
	}
}

func TestPricingModelSpotAlias(t *testing.T) {
	t.Parallel()

	loaded := loadSpotRetailFixture(t)
	onDemand := fixtureVMItem(t, loaded.Items, false)
	spot := fixtureVMItem(t, loaded.Items, true)
	calc := newPricingCalc(t, loaded.Items)
	region := loaded.Items[0].ArmRegionName
	sku := loaded.Items[0].ArmSkuName

	spotReq := vmProjectedRequest(region, sku, "")
	spotReq.Resource.Tags = map[string]string{"pricing_model": "spot"}
	resp, err := calc.GetProjectedCost(context.Background(), spotReq)
	if err != nil {
		t.Fatalf("pricing_model=spot failed: %v", err)
	}
	want := spot.RetailPrice * pluginsdk.HoursPerMonth
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want spot %v", resp.GetCostPerMonth(), want)
	}
	if resp.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC {
		t.Fatalf("pricing_category = %s, want DYNAMIC", resp.GetPricingCategory())
	}

	demandReq := vmProjectedRequest(region, sku, "")
	demandReq.Resource.Tags = map[string]string{"pricing_model": "consumption"}
	resp, err = calc.GetProjectedCost(context.Background(), demandReq)
	if err != nil {
		t.Fatalf("pricing_model=consumption failed: %v", err)
	}
	want = onDemand.RetailPrice * pluginsdk.HoursPerMonth
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want on-demand %v", resp.GetCostPerMonth(), want)
	}

	priorityWins := vmProjectedRequest(region, sku, "Spot")
	priorityWins.Resource.Tags["pricing_model"] = "consumption"
	resp, err = calc.GetProjectedCost(context.Background(), priorityWins)
	if err != nil {
		t.Fatalf("priority over pricing_model failed: %v", err)
	}
	want = spot.RetailPrice * pluginsdk.HoursPerMonth
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("priority cost_per_month = %v, want spot %v", resp.GetCostPerMonth(), want)
	}

	bad := vmProjectedRequest(region, sku, "")
	bad.Resource.Tags = map[string]string{"pricing_model": "reserved"}
	_, err = calc.GetProjectedCost(context.Background(), bad)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (err=%v)", status.Code(err), err)
	}
	if !strings.Contains(status.Convert(err).Message(), "reserved") {
		t.Fatalf("message %q does not name reserved", status.Convert(err).Message())
	}
}

func TestGetProjectedCostSpotOverGRPC(t *testing.T) {
	t.Parallel()

	loaded := loadSpotRetailFixture(t)
	spot := fixtureVMItem(t, loaded.Items, true)
	wantMonthly := spot.RetailPrice * pluginsdk.HoursPerMonth
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
		vmProjectedRequest(loaded.Items[0].ArmRegionName, loaded.Items[0].ArmSkuName, "Spot"),
	)
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if math.Abs(resp.GetCostPerMonth()-wantMonthly) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), wantMonthly)
	}
}

func TestEstimateCostSpotD2sV3Eastus(t *testing.T) {
	t.Parallel()

	loaded := loadSpotRetailFixture(t)
	onDemand := fixtureVMItem(t, loaded.Items, false)
	spot := fixtureVMItem(t, loaded.Items, true)
	spotMonthly := spot.RetailPrice * pluginsdk.HoursPerMonth
	// 13.74 is the oracle monthly for Standard_D2s_v3 eastus Spot, rounded to cents.
	if math.Abs(spotMonthly-13.74) > 0.01 {
		t.Fatalf("fixture spot monthly = %v, want about 13.74", spotMonthly)
	}
	onDemandMonthly := onDemand.RetailPrice * pluginsdk.HoursPerMonth

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
	client := finfocusv1.NewCostSourceServiceClient(conn)

	spotResp, err := client.EstimateCost(context.Background(), estimateVMRequest(t, "Spot"))
	if err != nil {
		t.Fatalf("spot EstimateCost() failed: %v", err)
	}
	if math.Abs(spotResp.GetCostMonthly()-spotMonthly) > 1e-6 {
		t.Fatalf("spot cost_monthly = %v, want %v", spotResp.GetCostMonthly(), spotMonthly)
	}
	if spotResp.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC {
		t.Fatalf("spot pricing_category = %s, want DYNAMIC", spotResp.GetPricingCategory())
	}

	demandResp, err := client.EstimateCost(context.Background(), estimateVMRequest(t, ""))
	if err != nil {
		t.Fatalf("on-demand EstimateCost() failed: %v", err)
	}
	if math.Abs(demandResp.GetCostMonthly()-onDemandMonthly) > 1e-6 {
		t.Fatalf("on-demand cost_monthly = %v, want %v", demandResp.GetCostMonthly(), onDemandMonthly)
	}
	if demandResp.GetPricingCategory() != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD {
		t.Fatalf("on-demand pricing_category = %s, want STANDARD", demandResp.GetPricingCategory())
	}

	_, err = client.EstimateCost(context.Background(), estimateVMRequest(t, "LowPriority"))
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (err=%v)", status.Code(err), err)
	}
	if !strings.Contains(status.Convert(err).Message(), "LowPriority") {
		t.Fatalf("message %q does not name LowPriority", status.Convert(err).Message())
	}
}

func estimateVMRequest(t testing.TB, priority string) *finfocusv1.EstimateCostRequest {
	t.Helper()

	attrs := map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_D2s_v3",
	}
	if priority != "" {
		attrs["priority"] = priority
	}
	fields, err := structpb.NewStruct(attrs)
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	return &finfocusv1.EstimateCostRequest{
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
		Attributes:   fields,
	}
}

func TestGetProjectedCostSpotMissingIsNotFound(t *testing.T) {
	t.Parallel()

	loaded := loadSpotRetailFixture(t)
	items := withoutSpotItems(loaded.Items)
	if len(items) == 0 || len(items) == len(loaded.Items) {
		t.Fatalf("spot filter left %d of %d items", len(items), len(loaded.Items))
	}

	calc := newPricingCalc(t, items)
	_, err := calc.GetProjectedCost(context.Background(), vmProjectedRequest(
		loaded.Items[0].ArmRegionName,
		loaded.Items[0].ArmSkuName,
		"Spot",
	))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (err=%v)", status.Code(err), err)
	}
}

func TestSelectVMItemEmptyProductNamePricesItem(t *testing.T) {
	t.Parallel()

	loaded := loadSpotRetailFixture(t)
	source := fixtureVMItem(t, loaded.Items, false)
	single := azureclient.PriceItem{
		RetailPrice:  source.RetailPrice,
		UnitPrice:    source.UnitPrice,
		CurrencyCode: source.CurrencyCode,
	}

	got, err := selectVMItem([]azureclient.PriceItem{single}, false)
	if err != nil {
		t.Fatalf("selectVMItem() error = %v", err)
	}
	if !reflect.DeepEqual(got, single) {
		t.Fatalf("selectVMItem() = %+v, want %+v", got, single)
	}

	price, currency, err := unitPriceAndCurrency([]azureclient.PriceItem{got})
	if err != nil {
		t.Fatalf("unitPriceAndCurrency() error = %v", err)
	}
	if price != single.RetailPrice || currency != single.CurrencyCode {
		t.Fatalf("priced %v %s, want %v %s", price, currency, single.RetailPrice, single.CurrencyCode)
	}
}

func TestSelectVMItemSkipsWindowsAndEmbeddedSpot(t *testing.T) {
	t.Parallel()

	loaded := loadSpotRetailFixture(t)
	linuxOnDemand := fixtureVMItem(t, loaded.Items, false)
	linuxSpot := fixtureVMItem(t, loaded.Items, true)
	windowsOnDemand := fixtureVMItemByProduct(t, loaded.Items, false, true)
	windowsSpot := fixtureVMItemByProduct(t, loaded.Items, true, true)

	got, err := selectVMItem([]azureclient.PriceItem{windowsOnDemand, linuxOnDemand}, false)
	if err != nil {
		t.Fatalf("on-demand selectVMItem() error = %v", err)
	}
	if got.RetailPrice != linuxOnDemand.RetailPrice || strings.Contains(strings.ToLower(got.ProductName), "windows") {
		t.Fatalf("on-demand item = %+v, want Linux retail %v", got, linuxOnDemand.RetailPrice)
	}

	got, err = selectVMItem([]azureclient.PriceItem{windowsSpot, linuxSpot}, true)
	if err != nil {
		t.Fatalf("spot selectVMItem() error = %v", err)
	}
	if got.RetailPrice != linuxSpot.RetailPrice || strings.Contains(strings.ToLower(got.ProductName), "windows") {
		t.Fatalf("spot item = %+v, want Linux retail %v", got, linuxSpot.RetailPrice)
	}

	embedded := linuxOnDemand
	embedded.ProductName = ""
	embedded.SkuName = strings.Replace(linuxSpot.SkuName, "Spot", "Spotlight", 1)
	embedded.MeterName = strings.Replace(linuxSpot.MeterName, "Spot", "Spotlight", 1)
	if embedded.SkuName == linuxSpot.SkuName && embedded.MeterName == linuxSpot.MeterName {
		t.Fatal("fixture spot name has no Spot word to embed")
	}
	got, err = selectVMItem([]azureclient.PriceItem{embedded, linuxSpot}, true)
	if err != nil {
		t.Fatalf("embedded spot selectVMItem() error = %v", err)
	}
	if got.RetailPrice != linuxSpot.RetailPrice || got.MeterName != linuxSpot.MeterName {
		t.Fatalf("embedded spot item = %+v, want meter %q", got, linuxSpot.MeterName)
	}
}

func loadSpotRetailFixture(t *testing.T) azureclient.PriceResponse {
	t.Helper()

	data, err := os.ReadFile("testdata/retail/spot/standard_d2s_v3_eastus.json")
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

func fixtureVMItem(t *testing.T, items []azureclient.PriceItem, spot bool) azureclient.PriceItem {
	t.Helper()
	return fixtureVMItemByProduct(t, items, spot, false)
}

func fixtureVMItemByProduct(
	t *testing.T,
	items []azureclient.PriceItem,
	spot bool,
	windows bool,
) azureclient.PriceItem {
	t.Helper()

	var matches []azureclient.PriceItem
	for _, item := range items {
		if fixtureLowPriority(item) || fixtureSpot(item) != spot {
			continue
		}
		isWindows := strings.Contains(strings.ToLower(item.ProductName), "windows")
		if isWindows != windows {
			continue
		}
		matches = append(matches, item)
	}
	if len(matches) != 1 {
		t.Fatalf("spot=%v windows=%v matches = %d, want 1", spot, windows, len(matches))
	}
	if matches[0].RetailPrice == 0 {
		t.Fatal("fixture retail price is zero")
	}
	return matches[0]
}

func fixtureSpot(item azureclient.PriceItem) bool {
	return fixtureWholeWord(item.SkuName, "spot") || fixtureWholeWord(item.MeterName, "spot")
}

func fixtureWholeWord(value, word string) bool {
	return strings.Contains(" "+strings.ToLower(value)+" ", " "+word+" ")
}

func fixtureLowPriority(item azureclient.PriceItem) bool {
	name := strings.ToLower(item.SkuName + " " + item.MeterName)
	return strings.Contains(name, "low priority")
}

func withoutSpotItems(items []azureclient.PriceItem) []azureclient.PriceItem {
	kept := make([]azureclient.PriceItem, 0, len(items))
	for _, item := range items {
		if fixtureSpot(item) {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func vmProjectedRequest(region, sku, priority string) *finfocusv1.GetProjectedCostRequest {
	resource := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		Region:       region,
		Sku:          sku,
	}
	if priority != "" {
		resource.Tags = map[string]string{"priority": priority}
	}
	return &finfocusv1.GetProjectedCostRequest{Resource: resource}
}
