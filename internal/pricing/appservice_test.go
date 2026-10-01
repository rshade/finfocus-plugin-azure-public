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
	"strconv"
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

const (
	appServiceFixturePath = "testdata/retail/appservice/eastus_consumption.json"
	functionsFixturePath  = "testdata/retail/functions/eastus_consumption.json"

	// Classic Consumption grant, per subscription per month. Applied in full to
	// the one resource under test. These are quantities, not prices.
	testFreeExecutions     = 1_000_000
	testFreeGBSeconds      = 400_000
	testExecutionsPerPrice = 10
	testPremiumVCPU        = 2.0
	testPremiumMemoryGiB   = 4.0
)

func TestGetProjectedCostAppServicePlanFromFixture(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, appServiceFixturePath)
	linuxP1 := fixtureAppPlan(t, loaded.Items, "P1v3", false)
	windowsP1 := fixtureAppPlan(t, loaded.Items, "P1 v3", true)
	linuxB1 := fixtureAppPlan(t, loaded.Items, "B1", false)
	windowsB1 := fixtureAppPlan(t, loaded.Items, "B1", true)
	linuxV4 := fixtureAppPlan(t, loaded.Items, "P1v4", false)
	if linuxP1.RetailPrice == windowsP1.RetailPrice || linuxP1.ProductName == windowsP1.ProductName {
		t.Fatal("linux and windows P1 v3 are not different rows")
	}
	if linuxP1.MeterName != linuxP1.SkuName+" App" {
		t.Fatalf("linux P1 v3 meter = %q, want sku + App", linuxP1.MeterName)
	}
	if linuxB1.MeterName != linuxB1.SkuName {
		t.Fatalf("linux B1 meter = %q, want sku %q", linuxB1.MeterName, linuxB1.SkuName)
	}
	if windowsB1.MeterName != windowsB1.SkuName+" App" {
		t.Fatalf("windows B1 meter = %q, want sku + App", windowsB1.MeterName)
	}
	if linuxV4.MeterName != linuxV4.SkuName {
		t.Fatalf("v4 meter = %q, want sku %q", linuxV4.MeterName, linuxV4.SkuName)
	}
	if !strings.Contains(strings.ToLower(linuxP1.ProductName), "linux") {
		t.Fatalf("linux product = %q", linuxP1.ProductName)
	}
	if strings.Contains(strings.ToLower(windowsP1.ProductName), "linux") {
		t.Fatalf("windows product = %q", windowsP1.ProductName)
	}

	calc := newPricingCalc(t, loaded.Items)
	tests := []struct {
		name         string
		resourceType string
		sku          string
		os           string
		want         azureclient.PriceItem
	}{
		{name: "linux P1v3", resourceType: "web/AppServicePlan", sku: "P1v3", want: linuxP1},
		{name: "spaced sku", resourceType: "web/AppServicePlan", sku: "P1 v3", want: linuxP1},
		{name: "lower sku", resourceType: "web/AppServicePlan", sku: "p1v3", want: linuxP1},
		{
			name:         "pulumi plan",
			resourceType: "azure:appservice/plan:Plan",
			sku:          "P1v3",
			want:         linuxP1,
		},
		{
			name:         "windows P1 v3",
			resourceType: "web/AppServicePlan",
			sku:          "P1 v3",
			os:           "Windows",
			want:         windowsP1,
		},
		{
			name:         "windows any case",
			resourceType: "web/AppServicePlan",
			sku:          "P1v3",
			os:           "windows",
			want:         windowsP1,
		},
		{name: "linux B1", resourceType: "web/AppServicePlan", sku: "B1", want: linuxB1},
		{
			name:         "windows B1",
			resourceType: "web/AppServicePlan",
			sku:          "b1",
			os:           "WINDOWS",
			want:         windowsB1,
		},
		{name: "linux P1v4", resourceType: "web/AppServicePlan", sku: "P1v4", want: linuxV4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tags := map[string]string{}
			if tt.os != "" {
				tags["os"] = tt.os
			}
			resp, err := calc.GetProjectedCost(context.Background(), pricedRequest(
				tt.resourceType, tt.sku, tags,
			))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
				t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", validateErr)
			}
			assertHourlyMonth(t, resp, tt.want)
			if got := resp.GetCostBreakdown()["compute"]; math.Abs(got-resp.GetCostPerMonth()) > 1e-9 {
				t.Fatalf("compute breakdown = %v, cost_per_month = %v", got, resp.GetCostPerMonth())
			}
			if len(resp.GetCostBreakdown()) != 1 {
				t.Fatalf("breakdown = %v, want only compute", resp.GetCostBreakdown())
			}
		})
	}
}

func TestGetProjectedCostAppServiceF1IsZeroRow(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, appServiceFixturePath)
	linux := fixtureAppPlan(t, loaded.Items, "F1", false)
	windows := fixtureAppPlan(t, loaded.Items, "F1", true)
	if linux.RetailPrice != 0 || windows.RetailPrice != 0 {
		t.Fatal("fixture F1 retail price is not zero")
	}

	calc := newPricingCalc(t, loaded.Items)
	for _, tt := range []struct {
		name string
		os   string
		want azureclient.PriceItem
	}{
		{name: "linux default", want: linux},
		{name: "windows", os: "Windows", want: windows},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tags := map[string]string{}
			if tt.os != "" {
				tags["os"] = tt.os
			}
			resp, err := calc.GetProjectedCost(context.Background(), pricedRequest(
				"web/AppServicePlan", "F1", tags,
			))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			if resp.GetCostPerMonth() != 0 || resp.GetUnitPrice() != 0 {
				t.Fatalf("cost = %v unit = %v, want 0 from the free row", resp.GetCostPerMonth(), resp.GetUnitPrice())
			}
			if got := resp.GetCostBreakdown()["compute"]; got != 0 {
				t.Fatalf("compute breakdown = %v, want 0", got)
			}
		})
	}
}

func TestGetProjectedCostAppServiceUnknownSKU(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, appServiceFixturePath)
	calc := newPricingCalc(t, loaded.Items)
	tests := []struct {
		name string
		sku  string
		os   string
		want string
	}{
		{name: "unknown plan", sku: "NoSuchPlan", want: "NoSuchPlan"},
		{name: "asip is not a plan", sku: "ASIP", want: "ASIP"},
		{name: "bad os", sku: "P1v3", os: "FreeBSD", want: "FreeBSD"},
		{name: "explicit linux is not a tag", sku: "P1v3", os: "Linux", want: "Linux"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tags := map[string]string{}
			if tt.os != "" {
				tags["os"] = tt.os
			}
			_, err := calc.GetProjectedCost(context.Background(), pricedRequest(
				"web/AppServicePlan", tt.sku, tags,
			))
			assertInvalidArgument(t, err, tt.want)
		})
	}
}

func TestGetProjectedCostAppServiceAmbiguousPrice(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, appServiceFixturePath)
	linux := fixtureAppPlan(t, loaded.Items, "P1v3", false)
	if linux.RetailPrice == 0 {
		t.Fatal("fixture linux P1 v3 price is zero")
	}
	items := append([]azureclient.PriceItem(nil), loaded.Items...)
	clone := linux
	clone.RetailPrice += linux.RetailPrice
	clone.MeterID += "-copy"
	items = append(items, clone)

	calc := newPricingCalc(t, items)
	_, err := calc.GetProjectedCost(context.Background(), pricedRequest(
		"web/AppServicePlan", "P1v3", nil,
	))
	assertInvalidArgument(t, err, "P1v3")
}

func TestGetProjectedCostAppServiceOverGRPC(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, appServiceFixturePath)
	item := fixtureAppPlan(t, loaded.Items, "P1v3", false)
	want := item.RetailPrice * pluginsdk.HoursPerMonth
	calc := newPricingCalc(t, loaded.Items)
	client := dialPricingClient(t, calc)

	resp, err := client.GetProjectedCost(context.Background(), pricedRequest(
		"web/AppServicePlan", "P1v3", nil,
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 || math.Abs(resp.GetCostPerMonth()-item.RetailPrice) <= 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v (hourly * 730)", resp.GetCostPerMonth(), want)
	}
}

func TestGetActualCostAppServicePlanDefaultWindow(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, appServiceFixturePath)
	item := fixtureAppPlan(t, loaded.Items, "P1v3", false)
	want := item.RetailPrice * pluginsdk.HoursPerMonth
	if math.Abs(want-item.RetailPrice) <= 1e-9 {
		t.Fatal("fixture price cannot distinguish a 730 multiplier")
	}
	calc := newPricingCalc(t, loaded.Items)

	resp, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags: map[string]string{
			"region":        "eastus",
			"sku":           "P1v3",
			"resource_type": "web/AppServicePlan",
		},
	})
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	results := resp.GetResults()
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if math.Abs(results[0].GetCost()-want) > 1e-9 {
		t.Fatalf("cost = %v, want monthly %v for the default window", results[0].GetCost(), want)
	}
	if math.Abs(results[0].GetUsageAmount()-pluginsdk.HoursPerMonth) > 1e-9 {
		t.Fatalf("usage_hours = %v, want %v", results[0].GetUsageAmount(), pluginsdk.HoursPerMonth)
	}
}

func TestAppServiceQueryOmitsArmSKU(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, appServiceFixturePath)
	filter := capturePriceFilter(t, loaded.Items, pricedRequest(
		"web/AppServicePlan", "P1v3", nil,
	))
	assertPriceFilter(t, filter, "Azure App Service", "P1v3")
}

func TestGetProjectedCostFunctionsConsumptionOverage(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, functionsFixturePath)
	execItem := fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Total Executions", "10")
	gbItem := fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Execution Time", "1 GB Second")
	executions := testFreeExecutions + testExecutionsPerPrice
	gbSeconds := testFreeGBSeconds + 1
	wantExec := (float64(executions-testFreeExecutions) / testExecutionsPerPrice) * execItem.RetailPrice
	wantGB := float64(gbSeconds-testFreeGBSeconds) * gbItem.RetailPrice
	if wantExec == 0 || wantGB == 0 {
		t.Fatal("overage cannot distinguish the zero sibling")
	}
	undivided := float64(executions-testFreeExecutions) * execItem.RetailPrice
	if math.Abs(wantExec-undivided) <= 1e-12 {
		t.Fatal("execution price cannot distinguish a per-10 unit")
	}
	if math.Abs((wantExec+wantGB)*pluginsdk.HoursPerMonth-(wantExec+wantGB)) <= 1e-12 {
		t.Fatal("consumption price cannot distinguish a 730 multiplier")
	}

	calc := newPricingCalc(t, loaded.Items)
	tests := []struct {
		name         string
		resourceType string
		sku          string
		model        string
	}{
		{name: "standard", resourceType: "web/FunctionApp", sku: "Standard"},
		{name: "empty sku", resourceType: "web/FunctionApp"},
		{name: "y1", resourceType: "web/FunctionApp", sku: "Y1"},
		{name: "dynamic", resourceType: "web/FunctionApp", sku: "dynamic"},
		{name: "consumption sku", resourceType: "web/FunctionApp", sku: "Consumption"},
		{
			name:         "pricing model",
			resourceType: "web/FunctionApp",
			model:        "consumption",
		},
		{
			name:         "pulumi",
			resourceType: "azure:appservice/functionApp:FunctionApp",
			sku:          "Y1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tags := map[string]string{
				"executions": strconv.Itoa(executions),
				"gb_seconds": strconv.Itoa(gbSeconds),
			}
			if tt.model != "" {
				tags["pricing_model"] = tt.model
			}
			resp, err := calc.GetProjectedCost(context.Background(), pricedRequest(
				tt.resourceType, tt.sku, tags,
			))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			assertConsumptionBreakdown(t, resp, wantExec, wantGB)
		})
	}
}

func TestGetProjectedCostFunctionsConsumptionInsideGrant(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, functionsFixturePath)
	fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Total Executions", "10")
	fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Execution Time", "1 GB Second")
	calc := newPricingCalc(t, loaded.Items)

	tests := []struct {
		name       string
		executions int
		gbSeconds  int
	}{
		{name: "at grant", executions: testFreeExecutions, gbSeconds: testFreeGBSeconds},
		{name: "below grant", executions: testExecutionsPerPrice, gbSeconds: 1},
		{
			name:       "executions only over",
			executions: testFreeExecutions + testExecutionsPerPrice,
			gbSeconds:  testFreeGBSeconds,
		},
	}
	execItem := fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Total Executions", "10")
	gbItem := fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Execution Time", "1 GB Second")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wantExec := 0.0
			if tt.executions > testFreeExecutions {
				wantExec = (float64(tt.executions-testFreeExecutions) / testExecutionsPerPrice) * execItem.RetailPrice
			}
			wantGB := 0.0
			if tt.gbSeconds > testFreeGBSeconds {
				wantGB = float64(tt.gbSeconds-testFreeGBSeconds) * gbItem.RetailPrice
			}
			resp, err := calc.GetProjectedCost(context.Background(), pricedRequest(
				"web/FunctionApp",
				"Standard",
				map[string]string{
					"executions": strconv.Itoa(tt.executions),
					"gb_seconds": strconv.Itoa(tt.gbSeconds),
				},
			))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			assertConsumptionBreakdown(t, resp, wantExec, wantGB)
		})
	}
}

func TestGetProjectedCostFunctionsMissingNonZeroExecutionIsNotFound(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, functionsFixturePath)
	items := withoutPositiveMeter(loaded.Items, "Standard Total Executions")
	if len(items) == len(loaded.Items) || !meterHasZero(items, "Standard Total Executions") {
		t.Fatalf("filter left %d of %d items", len(items), len(loaded.Items))
	}
	calc := newPricingCalc(t, items)

	_, err := calc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp",
		"Standard",
		map[string]string{
			"executions": strconv.Itoa(testFreeExecutions + testExecutionsPerPrice),
			"gb_seconds": strconv.Itoa(testFreeGBSeconds + 1),
		},
	))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (err=%v)", status.Code(err), err)
	}
}

func TestGetProjectedCostFunctionsRejectsExecutionUnit(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, functionsFixturePath)
	items := append([]azureclient.PriceItem(nil), loaded.Items...)
	changed := false
	for i := range items {
		if items[i].MeterName == "Standard Total Executions" && items[i].RetailPrice > 0 {
			items[i].UnitOfMeasure = "1"
			changed = true
		}
	}
	if !changed {
		t.Fatal("fixture has no positive execution row")
	}
	calc := newPricingCalc(t, items)

	_, err := calc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp",
		"",
		map[string]string{
			"executions": strconv.Itoa(testFreeExecutions + testExecutionsPerPrice),
			"gb_seconds": strconv.Itoa(testFreeGBSeconds + 1),
		},
	))
	assertInvalidArgument(t, err, "1")
}

func TestGetProjectedCostFunctionsPremiumComponents(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, functionsFixturePath)
	vcpu := fixturePositiveMeter(t, loaded.Items, "Premium Functions", "Premium vCPU Duration", "1 Hour")
	memory := fixturePositiveMeter(t, loaded.Items, "Premium Functions", "Premium Memory Duration", "1 GiB Hour")
	wantVCPU := testPremiumVCPU * vcpu.RetailPrice * pluginsdk.HoursPerMonth
	wantMemory := testPremiumMemoryGiB * memory.RetailPrice * pluginsdk.HoursPerMonth
	want := wantVCPU + wantMemory
	if math.Abs(wantVCPU-testPremiumVCPU*vcpu.RetailPrice) <= 1e-9 {
		t.Fatal("fixture cannot distinguish a missing 730 factor")
	}
	if math.Abs(want-want*pluginsdk.HoursPerMonth) <= 1e-6 {
		t.Fatal("fixture cannot distinguish a doubled 730 factor")
	}

	calc := newPricingCalc(t, loaded.Items)
	tests := []struct {
		name         string
		resourceType string
		sku          string
		model        string
	}{
		{name: "sku premium", resourceType: "web/FunctionApp", sku: "Premium"},
		{name: "pricing model", resourceType: "web/FunctionApp", model: "premium"},
		{
			name:         "pulumi model",
			resourceType: "azure:appservice/functionApp:FunctionApp",
			model:        "Premium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tags := map[string]string{
				"vcpu_count": strconv.FormatFloat(testPremiumVCPU, 'f', -1, 64),
				"memory_gib": strconv.FormatFloat(testPremiumMemoryGiB, 'f', -1, 64),
			}
			if tt.model != "" {
				tags["pricing_model"] = tt.model
			}
			resp, err := calc.GetProjectedCost(context.Background(), pricedRequest(
				tt.resourceType, tt.sku, tags,
			))
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
				t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", validateErr)
			}
			gotVCPU := resp.GetCostBreakdown()["vcpu"]
			gotMemory := resp.GetCostBreakdown()["memory"]
			if math.Abs(gotVCPU-wantVCPU) > 1e-9 || math.Abs(gotMemory-wantMemory) > 1e-9 {
				t.Fatalf("breakdown vcpu=%v memory=%v, want %v and %v", gotVCPU, gotMemory, wantVCPU, wantMemory)
			}
			sum := gotVCPU + gotMemory
			if math.Abs(sum-resp.GetCostPerMonth()) > 1e-9 || math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
				t.Fatalf("cost_per_month = %v, breakdown sum = %v, want %v", resp.GetCostPerMonth(), sum, want)
			}
			if _, ok := resp.GetCostBreakdown()["compute"]; ok {
				t.Fatalf("premium breakdown includes compute: %v", resp.GetCostBreakdown())
			}
		})
	}
}

func TestGetProjectedCostFunctionDedicatedUsesPlan(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, appServiceFixturePath)
	linux := fixtureAppPlan(t, loaded.Items, "P1v3", false)
	windows := fixtureAppPlan(t, loaded.Items, "B1", true)
	calc := newPricingCalc(t, loaded.Items)

	linuxResp, err := calc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp", "P1v3", nil,
	))
	if err != nil {
		t.Fatalf("linux dedicated failed: %v", err)
	}
	assertHourlyMonth(t, linuxResp, linux)
	compute := linuxResp.GetCostBreakdown()["compute"]
	if len(linuxResp.GetCostBreakdown()) != 1 || math.Abs(compute-linuxResp.GetCostPerMonth()) > 1e-9 {
		t.Fatalf("breakdown = %v, want compute", linuxResp.GetCostBreakdown())
	}

	windowsResp, err := calc.GetProjectedCost(context.Background(), pricedRequest(
		"azure:appservice/functionApp:FunctionApp",
		"B1",
		map[string]string{"os": "Windows"},
	))
	if err != nil {
		t.Fatalf("windows dedicated failed: %v", err)
	}
	assertHourlyMonth(t, windowsResp, windows)
}

func TestGetProjectedCostFunctionsRejectsUnknown(t *testing.T) {
	t.Parallel()

	plans := loadRetailFixture(t, appServiceFixturePath)
	planCalc := newPricingCalc(t, plans.Items)
	_, err := planCalc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp", "EP1", nil,
	))
	assertInvalidArgument(t, err, "EP1")

	_, err = planCalc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp", "P1v3", map[string]string{"pricing_model": "flex"},
	))
	assertInvalidArgument(t, err, "flex")

	functions := loadRetailFixture(t, functionsFixturePath)
	fnCalc := newPricingCalc(t, functions.Items)
	_, err = fnCalc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp", "Standard", map[string]string{"gb_seconds": "1"},
	))
	assertInvalidArgument(t, err, "executions")

	_, err = fnCalc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp", "", map[string]string{"executions": "1"},
	))
	assertInvalidArgument(t, err, "gb_seconds")

	_, err = fnCalc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp", "Premium", map[string]string{"memory_gib": "1"},
	))
	assertInvalidArgument(t, err, "vcpu_count")

	_, err = fnCalc.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp",
		"",
		map[string]string{"pricing_model": "premium", "vcpu_count": "1"},
	))
	assertInvalidArgument(t, err, "memory_gib")
}

func TestGetProjectedCostFunctionsOverGRPC(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, functionsFixturePath)
	execItem := fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Total Executions", "10")
	gbItem := fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Execution Time", "1 GB Second")
	executions := testFreeExecutions + testExecutionsPerPrice
	gbSeconds := testFreeGBSeconds + 1
	want := (float64(executions-testFreeExecutions)/testExecutionsPerPrice)*execItem.RetailPrice +
		float64(gbSeconds-testFreeGBSeconds)*gbItem.RetailPrice
	calc := newPricingCalc(t, loaded.Items)
	client := dialPricingClient(t, calc)

	resp, err := client.GetProjectedCost(context.Background(), pricedRequest(
		"web/FunctionApp",
		"Y1",
		map[string]string{
			"executions": strconv.Itoa(executions),
			"gb_seconds": strconv.Itoa(gbSeconds),
		},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 || resp.GetCostPerMonth() == 0 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), want)
	}
}

func TestFunctionsQueryOmitsArmSKU(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, functionsFixturePath)
	filter := capturePriceFilter(t, loaded.Items, pricedRequest(
		"web/FunctionApp",
		"Standard",
		map[string]string{"executions": "1", "gb_seconds": "1"},
	))
	assertPriceFilter(t, filter, "Functions", "Standard")

	plans := loadRetailFixture(t, appServiceFixturePath)
	dedicated := capturePriceFilter(t, plans.Items, pricedRequest(
		"web/FunctionApp", "P1v3", nil,
	))
	assertPriceFilter(t, dedicated, "Azure App Service", "P1v3")
}

func TestMapDescriptorToQueryAppServiceAndFunctions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desc    *finfocusv1.ResourceDescriptor
		service string
		wantErr error
	}{
		{
			name: "app service plan",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "web/AppServicePlan",
				Region:       "eastus",
				Sku:          "P1v3",
			},
			service: "Azure App Service",
		},
		{
			name: "pulumi plan",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "azure:appservice/plan:Plan",
				Region:       "eastus",
				Sku:          "B1",
			},
			service: "Azure App Service",
		},
		{
			name: "plan missing sku",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "web/AppServicePlan",
				Region:       "eastus",
			},
			wantErr: ErrMissingRequiredFields,
		},
		{
			name: "function app",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "web/FunctionApp",
				Region:       "eastus",
				Sku:          "Y1",
			},
			service: "Functions",
		},
		{
			name: "function empty sku",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "WEB/FUNCTIONAPP",
				Region:       "eastus",
			},
			service: "Functions",
		},
		{
			name: "pulumi function",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "azure:appservice/functionApp:FunctionApp",
				Region:       "eastus",
			},
			service: "Functions",
		},
		{
			name: "plan prefix is not a plan",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "web/AppServicePlanExtra",
				Region:       "eastus",
				Sku:          "P1v3",
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
				return
			}
			if err != nil {
				t.Fatalf("MapDescriptorToQuery() failed: %v", err)
			}
			if query.ArmSkuName != "" {
				t.Fatalf("ArmSkuName = %q, want empty", query.ArmSkuName)
			}
			if query.ServiceName != tt.service {
				t.Fatalf("ServiceName = %q, want %q", query.ServiceName, tt.service)
			}
			if query.ArmRegionName != "eastus" {
				t.Fatalf("ArmRegionName = %q, want eastus", query.ArmRegionName)
			}
			if query.ProductName != "" {
				t.Fatalf("ProductName = %q, want empty", query.ProductName)
			}
		})
	}
}

func assertHourlyMonth(t *testing.T, resp *finfocusv1.GetProjectedCostResponse, item azureclient.PriceItem) {
	t.Helper()

	want := item.RetailPrice * pluginsdk.HoursPerMonth
	if item.RetailPrice != 0 && math.Abs(item.RetailPrice-want) <= 1e-9 {
		t.Fatal("fixture price cannot distinguish a 730 multiplier")
	}
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), want)
	}
	if math.Abs(resp.GetUnitPrice()-item.RetailPrice) > 1e-9 {
		t.Fatalf("unit_price = %v, want retail %v", resp.GetUnitPrice(), item.RetailPrice)
	}
	if resp.GetCurrency() != item.CurrencyCode {
		t.Fatalf("currency = %q, want %q", resp.GetCurrency(), item.CurrencyCode)
	}
}

func assertConsumptionBreakdown(t *testing.T, resp *finfocusv1.GetProjectedCostResponse, wantExec, wantGB float64) {
	t.Helper()

	if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", validateErr)
	}
	gotExec := resp.GetCostBreakdown()["executions"]
	gotGB := resp.GetCostBreakdown()["gb_seconds"]
	if math.Abs(gotExec-wantExec) > 1e-12 || math.Abs(gotGB-wantGB) > 1e-12 {
		t.Fatalf("breakdown executions=%v gb_seconds=%v, want %v and %v", gotExec, gotGB, wantExec, wantGB)
	}
	sum := gotExec + gotGB
	if math.Abs(sum-resp.GetCostPerMonth()) > 1e-12 {
		t.Fatalf("cost_per_month = %v, component sum = %v", resp.GetCostPerMonth(), sum)
	}
	if _, ok := resp.GetCostBreakdown()["compute"]; ok {
		t.Fatalf("consumption breakdown includes compute: %v", resp.GetCostBreakdown())
	}
}

func assertInvalidArgument(t *testing.T, err error, name string) {
	t.Helper()

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (err=%v)", status.Code(err), err)
	}
	if !strings.Contains(status.Convert(err).Message(), name) {
		t.Fatalf("message %q does not name %q", status.Convert(err).Message(), name)
	}
}

func assertPriceFilter(t *testing.T, filter, service, absentSKU string) {
	t.Helper()

	if strings.Contains(filter, "armSkuName") || strings.Contains(filter, absentSKU) {
		t.Fatalf("filter includes sku %q: %s", absentSKU, filter)
	}
	for _, want := range []string{
		"armRegionName eq 'eastus'",
		"serviceName eq '" + service + "'",
		"priceType eq 'Consumption'",
		"currencyCode eq 'USD'",
	} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter %q does not contain %q", filter, want)
		}
	}
}

func capturePriceFilter(t *testing.T, items []azureclient.PriceItem, req *finfocusv1.GetProjectedCostRequest) string {
	t.Helper()

	var filter string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter = r.URL.Query().Get("$filter")
		resp := azureclient.PriceResponse{Items: items, Count: len(items)}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	calc := NewCalculator(zerolog.Nop(), cached)
	if _, err := calc.GetProjectedCost(context.Background(), req); err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	return filter
}

func dialPricingClient(t *testing.T, calc *Calculator) finfocusv1.CostSourceServiceClient {
	t.Helper()

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
	return finfocusv1.NewCostSourceServiceClient(conn)
}

func loadRetailFixture(t *testing.T, path string) azureclient.PriceResponse {
	t.Helper()

	data, err := os.ReadFile(path)
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

func fixtureAppPlan(t *testing.T, items []azureclient.PriceItem, sku string, windows bool) azureclient.PriceItem {
	t.Helper()

	norm := strings.ToLower(strings.ReplaceAll(sku, " ", ""))
	var found []azureclient.PriceItem
	for _, item := range items {
		if fixtureAppPlanRow(item, norm, windows) {
			found = append(found, item)
		}
	}
	if len(found) == 0 {
		t.Fatalf("no app service plan row for sku %s windows=%v", sku, windows)
	}
	price := found[0].RetailPrice
	for _, item := range found[1:] {
		if item.RetailPrice != price {
			t.Fatalf("app service sku %s windows=%v has different prices", sku, windows)
		}
	}
	return found[0]
}

func fixtureAppPlanRow(item azureclient.PriceItem, norm string, windows bool) bool {
	if item.Type != "Consumption" || item.UnitOfMeasure != "1 Hour" {
		return false
	}
	name := strings.ToLower(item.SkuName + " " + item.MeterName)
	if strings.Contains(name, "stamp") || strings.Contains(name, "ssl") ||
		strings.Contains(name, "domain") || strings.Contains(name, "asip") {
		return false
	}
	if strings.ToLower(strings.ReplaceAll(item.SkuName, " ", "")) != norm {
		return false
	}
	if item.MeterName != item.SkuName && item.MeterName != item.SkuName+" App" {
		return false
	}
	linux := strings.Contains(strings.ToLower(item.ProductName), "linux")
	if windows {
		return !linux
	}
	return linux
}

func fixturePositiveMeter(
	t *testing.T,
	items []azureclient.PriceItem,
	product, meter, unit string,
) azureclient.PriceItem {
	t.Helper()

	var found []azureclient.PriceItem
	for _, item := range items {
		if item.ProductName != product || item.MeterName != meter || item.UnitOfMeasure != unit {
			continue
		}
		if item.RetailPrice <= 0 {
			continue
		}
		found = append(found, item)
	}
	if len(found) == 0 {
		t.Fatalf("no positive %s / %s / %s row", product, meter, unit)
	}
	price := found[0].RetailPrice
	for _, item := range found[1:] {
		if item.RetailPrice != price {
			t.Fatalf("different positive prices for %s", meter)
		}
	}
	return found[0]
}

func withoutPositiveMeter(items []azureclient.PriceItem, meter string) []azureclient.PriceItem {
	kept := make([]azureclient.PriceItem, 0, len(items))
	for _, item := range items {
		if item.MeterName == meter && item.RetailPrice > 0 {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func meterHasZero(items []azureclient.PriceItem, meter string) bool {
	for _, item := range items {
		if item.MeterName == meter && item.RetailPrice == 0 {
			return true
		}
	}
	return false
}

func pricedRequest(resourceType, sku string, tags map[string]string) *finfocusv1.GetProjectedCostRequest {
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
