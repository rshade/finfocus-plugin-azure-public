package pricing

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
	"github.com/rshade/finfocus-plugin-azure-public/internal/estimation"
)

func TestPriceOptionProtosUsesTheRequestedBasis(t *testing.T) {
	t.Parallel()

	rows := []advisoryPrice{{
		category: finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_COMMITTED,
		model:    priceModelReservation,
		term:     "1 Year",
		unit:     2,
		monthly:  10,
		upfront:  50,
	}}

	unitBasis := priceOptionProtos(rows, 4, 100, false)
	if len(unitBasis) != 1 {
		t.Fatalf("unit options = %d", len(unitBasis))
	}
	if unitBasis[0].GetSavingsFraction() != 0.5 {
		t.Fatalf("unit savings_fraction = %v, want 0.5", unitBasis[0].GetSavingsFraction())
	}

	monthBasis := priceOptionProtos(rows, 4, 100, true)
	if len(monthBasis) != 1 {
		t.Fatalf("monthly options = %d", len(monthBasis))
	}
	if monthBasis[0].GetSavingsFraction() != 0.9 {
		t.Fatalf("monthly savings_fraction = %v, want 0.9", monthBasis[0].GetSavingsFraction())
	}

	zero := priceOptionProtos(rows, 0, 0, false)
	if len(zero) != 1 || zero[0].GetSavingsFraction() != 0 {
		t.Fatalf("zero primary = %+v, want fraction 0", zero)
	}
}

func TestGetProjectedCostVMAlternativesFromFixtures(t *testing.T) {
	t.Parallel()

	stable := readPriceItems(t, "testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption.json")
	preview := readPriceItems(
		t,
		"testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption_preview.json",
	)
	reservation := readPriceItems(
		t,
		"testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_reservation.json",
	)
	if len(reservation) != 0 {
		t.Fatalf("reservation fixture items = %d, want 0", len(reservation))
	}

	calc := newRoutedPricingCalc(t, func(r *http.Request) []azureclient.PriceItem {
		filter := r.URL.Query().Get("$filter")
		switch {
		case strings.Contains(filter, "priceType eq 'Reservation'"):
			return reservation
		case r.URL.Query().Get("api-version") != "":
			return preview
		default:
			return stable
		}
	})
	client := dialPricingClient(t, calc)

	resp, err := client.GetProjectedCost(
		context.Background(),
		vmProjectedRequest("eastus", "Standard_D2s_v3", ""),
	)
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	const wantUnit = 0.096
	wantMonthly := wantUnit * 730
	if math.Abs(resp.GetCostPerMonth()-wantMonthly) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), wantMonthly)
	}
	if math.Abs(resp.GetCostBreakdown()["compute"]-wantMonthly) > 1e-9 {
		t.Fatalf("compute breakdown = %v, want the selected price only", resp.GetCostBreakdown()["compute"])
	}
	if len(resp.GetRegionPrices()) != 0 {
		t.Fatalf("region_prices = %v, want none for a single region", resp.GetRegionPrices())
	}

	options := resp.GetPriceOptions()
	assertPriceOption(t, options, priceModelConsumption, "", wantUnit, 0, 0)
	assertPriceOption(t, options, vmPrioritySpot, "", 0.018816, 0, (wantUnit-0.018816)/wantUnit)
	assertPriceOption(t, options, priceModelSavingsPlan, "1 Year", 0.06624, 0, (wantUnit-0.06624)/wantUnit)
	assertPriceOption(t, options, priceModelSavingsPlan, "3 Years", 0.04512, 0, (wantUnit-0.04512)/wantUnit)
	if optionByModel(options, priceModelReservation) != nil {
		t.Fatal("reservation option set from an empty reservation fixture")
	}
	for _, opt := range options {
		if opt.GetUnitPrice() == 0.188 || opt.GetUnitPrice() == 0.075 || opt.GetUnitPrice() == 0.019 {
			t.Fatalf("option used a Windows or Low Priority row: %+v", opt)
		}
	}

	estimate, err := client.EstimateCost(context.Background(), estimateVMRequest(t, ""))
	if err != nil {
		t.Fatalf("EstimateCost() failed: %v", err)
	}
	if math.Abs(estimate.GetCostMonthly()-wantMonthly) > 1e-9 {
		t.Fatalf("estimate cost_monthly = %v, want %v", estimate.GetCostMonthly(), wantMonthly)
	}
	spot := optionByModel(estimate.GetPriceOptions(), vmPrioritySpot)
	if spot == nil {
		t.Fatal("estimate omitted the spot option")
	}
	wantFraction := (estimate.GetCostMonthly() - spot.GetMonthlyCost()) / estimate.GetCostMonthly()
	if math.Abs(spot.GetSavingsFraction()-wantFraction) > 1e-12 {
		t.Fatalf("estimate savings_fraction = %v, want %v", spot.GetSavingsFraction(), wantFraction)
	}
}

func TestGetProjectedCostVMReservationOptionsFromFixtures(t *testing.T) {
	t.Parallel()

	primary := readPriceItems(
		t,
		"testdata/retail/savingsplan/eastus_standard_d2als_v7_nofilter_preview.json",
	)
	reservation := append([]azureclient.PriceItem{{
		Type:            priceModelReservation,
		ProductName:     "Virtual Machines Dalsv7 Series Windows",
		SkuName:         "D2als v7",
		MeterName:       "D2als v7",
		RetailPrice:     999,
		ReservationTerm: "1 Year",
		ArmRegionName:   "eastus",
		CurrencyCode:    "USD",
		UnitOfMeasure:   "1 Hour",
	}}, readPriceItems(
		t,
		"testdata/retail/savingsplan/eastus_standard_d2als_v7_pricetype_reservation.json",
	)...)
	reservation = append(reservation, azureclient.PriceItem{
		Type:            priceModelReservation,
		ProductName:     "Virtual Machines Dalsv7 Series",
		SkuName:         "D2als v7 Low Priority",
		MeterName:       "D2als v7 Low Priority",
		RetailPrice:     111,
		ReservationTerm: "2 Years",
		ArmRegionName:   "eastus",
		CurrencyCode:    "USD",
		UnitOfMeasure:   "1 Hour",
	})

	calc := newRoutedPricingCalc(t, func(r *http.Request) []azureclient.PriceItem {
		if strings.Contains(r.URL.Query().Get("$filter"), "priceType eq 'Reservation'") {
			return reservation
		}
		return primary
	})

	resp, err := calc.GetProjectedCost(
		context.Background(),
		vmProjectedRequest("eastus", "Standard_D2als_v7", ""),
	)
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	const wantUnit = 0.0804
	if math.Abs(resp.GetUnitPrice()-wantUnit) > 1e-12 {
		t.Fatalf("unit_price = %v, want %v", resp.GetUnitPrice(), wantUnit)
	}
	if math.Abs(resp.GetCostPerMonth()-wantUnit*730) > 1e-9 {
		t.Fatalf("cost_per_month = %v", resp.GetCostPerMonth())
	}

	oneYear := 416.0 / float64(estimation.HoursPerYear)
	threeYear := 803.0 / float64(estimation.HoursPerYear*3)
	assertPriceOption(
		t,
		resp.GetPriceOptions(),
		priceModelReservation,
		"1 Year",
		oneYear,
		416,
		(wantUnit-oneYear)/wantUnit,
	)
	assertPriceOption(
		t,
		resp.GetPriceOptions(),
		priceModelReservation,
		"3 Years",
		threeYear,
		803,
		(wantUnit-threeYear)/wantUnit,
	)
	for _, opt := range resp.GetPriceOptions() {
		if opt.GetModel() == priceModelReservation && (opt.GetUpfrontCost() == 999 || opt.GetUpfrontCost() == 111) {
			t.Fatalf("reservation option = %+v", opt)
		}
	}
}

func TestGetProjectedCostVMRegionPricesFromFixtures(t *testing.T) {
	t.Parallel()

	eastus := readPriceItems(t, "testdata/retail/regions/standard_b1s_eastus.json")
	westus := readPriceItems(t, "testdata/retail/regions/standard_b1s_westus2.json")
	north := readPriceItems(t, "testdata/retail/regions/standard_b1s_northeurope.json")
	regions := append(append(append([]azureclient.PriceItem{}, eastus...), westus...), north...)
	regions = append(regions, azureclient.PriceItem{
		ArmRegionName: "switzerlandnorth",
		ArmSkuName:    "Standard_B1s",
		ProductName:   "Virtual Machines BS Series Windows",
		SkuName:       "B1s",
		MeterName:     "B1s",
		RetailPrice:   0.02,
		CurrencyCode:  "USD",
		Type:          priceModelConsumption,
		UnitOfMeasure: "1 Hour",
	})

	calc := newRoutedPricingCalc(t, func(r *http.Request) []azureclient.PriceItem {
		filter := r.URL.Query().Get("$filter")
		if strings.Contains(filter, "priceType eq 'Reservation'") || r.URL.Query().Get("api-version") != "" {
			return nil
		}
		if !strings.Contains(filter, "armRegionName") {
			return regions
		}
		return eastus
	})

	resp, err := calc.GetProjectedCost(context.Background(), vmProjectedRequest("eastus", "Standard_B1s", ""))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if math.Abs(resp.GetUnitPrice()-0.0104) > 1e-12 {
		t.Fatalf("unit_price = %v, want eastus 0.0104", resp.GetUnitPrice())
	}
	got := resp.GetRegionPrices()
	if len(got) != 2 {
		t.Fatalf("region_prices = %+v, want westus2 and northeurope", got)
	}
	assertRegionPrice(t, got[0], "westus2", 0.0104)
	assertRegionPrice(t, got[1], "northeurope", 0.0113)
}

func TestVMQuoteIgnoresAdvisoryFetchErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		if strings.Contains(filter, "priceType eq 'Reservation'") ||
			r.URL.Query().Get("api-version") != "" ||
			!strings.Contains(filter, "armRegionName") {
			http.Error(w, "advisory down", http.StatusInternalServerError)
			return
		}
		resp := azureclient.PriceResponse{
			Items: []azureclient.PriceItem{{
				ArmRegionName: "eastus",
				ArmSkuName:    "Standard_B1s",
				ProductName:   "Virtual Machines BS Series",
				SkuName:       "B1s",
				MeterName:     "B1s",
				RetailPrice:   0.0104,
				CurrencyCode:  "USD",
				Type:          priceModelConsumption,
				UnitOfMeasure: "1 Hour",
			}},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	cached := newCalculatorTestCachedClient(t, server.URL)
	t.Cleanup(func() { cached.Close() })
	failing := NewCalculator(zerolog.Nop(), cached)

	resp, err := failing.GetProjectedCost(context.Background(), vmProjectedRequest("eastus", "Standard_B1s", ""))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if math.Abs(resp.GetCostPerMonth()-0.0104*730) > 1e-9 {
		t.Fatalf("cost_per_month = %v", resp.GetCostPerMonth())
	}
	if len(resp.GetRegionPrices()) != 0 {
		t.Fatalf("region_prices = %+v", resp.GetRegionPrices())
	}
	assertPriceOption(t, resp.GetPriceOptions(), priceModelConsumption, "", 0.0104, 0, 0)
}

func TestGetProjectedCostVMZeroPrimarySavingsFraction(t *testing.T) {
	t.Parallel()

	items := []azureclient.PriceItem{
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ProductName:   "Virtual Machines BS Series",
			SkuName:       "B1s",
			MeterName:     "B1s",
			RetailPrice:   0,
			CurrencyCode:  "USD",
			Type:          priceModelConsumption,
			UnitOfMeasure: "1 Hour",
		},
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_B1s",
			ProductName:   "Virtual Machines BS Series",
			SkuName:       "B1s Spot",
			MeterName:     "B1s Spot",
			RetailPrice:   0.01,
			CurrencyCode:  "USD",
			Type:          priceModelConsumption,
			UnitOfMeasure: "1 Hour",
		},
	}
	calc := newRoutedPricingCalc(t, func(r *http.Request) []azureclient.PriceItem {
		filter := r.URL.Query().Get("$filter")
		if strings.Contains(filter, "priceType eq 'Reservation'") || r.URL.Query().Get("api-version") != "" {
			return nil
		}
		return items
	})

	resp, err := calc.GetProjectedCost(context.Background(), vmProjectedRequest("eastus", "Standard_B1s", ""))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if resp.GetCostPerMonth() != 0 {
		t.Fatalf("cost_per_month = %v, want 0", resp.GetCostPerMonth())
	}
	spot := optionByModel(resp.GetPriceOptions(), vmPrioritySpot)
	if spot == nil {
		t.Fatal("missing spot option")
	}
	if spot.GetSavingsFraction() != 0 {
		t.Fatalf("savings_fraction = %v, want 0", spot.GetSavingsFraction())
	}
}

func newRoutedPricingCalc(t *testing.T, route func(*http.Request) []azureclient.PriceItem) *Calculator {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := route(r)
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

func readPriceItems(t *testing.T, path string) []azureclient.PriceItem {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var resp azureclient.PriceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return resp.Items
}

func assertPriceOption(
	t *testing.T,
	options []*finfocusv1.PriceOption,
	model, term string,
	unit, upfront, fraction float64,
) {
	t.Helper()

	for _, opt := range options {
		if opt.GetModel() != model || opt.GetTerm() != term {
			continue
		}
		if math.Abs(opt.GetUnitPrice()-unit) > 1e-12 {
			t.Fatalf("%s %s unit_price = %v, want %v", model, term, opt.GetUnitPrice(), unit)
		}
		if math.Abs(opt.GetMonthlyCost()-unit*730) > 1e-9 {
			t.Fatalf("%s %s monthly_cost = %v, want %v", model, term, opt.GetMonthlyCost(), unit*730)
		}
		if math.Abs(opt.GetUpfrontCost()-upfront) > 1e-9 {
			t.Fatalf("%s %s upfront_cost = %v, want %v", model, term, opt.GetUpfrontCost(), upfront)
		}
		if math.Abs(opt.GetSavingsFraction()-fraction) > 1e-12 {
			t.Fatalf("%s %s savings_fraction = %v, want %v", model, term, opt.GetSavingsFraction(), fraction)
		}
		return
	}
	t.Fatalf("missing price option model %s term %q", model, term)
}

func optionByModel(options []*finfocusv1.PriceOption, model string) *finfocusv1.PriceOption {
	for _, opt := range options {
		if opt.GetModel() == model {
			return opt
		}
	}
	return nil
}

func assertRegionPrice(t *testing.T, row *finfocusv1.RegionPrice, region string, unit float64) {
	t.Helper()

	if row.GetRegion() != region {
		t.Fatalf("region = %q, want %q", row.GetRegion(), region)
	}
	if math.Abs(row.GetUnitPrice()-unit) > 1e-12 {
		t.Fatalf("%s unit_price = %v, want %v", region, row.GetUnitPrice(), unit)
	}
	if math.Abs(row.GetMonthlyCost()-unit*730) > 1e-9 {
		t.Fatalf("%s monthly_cost = %v, want %v", region, row.GetMonthlyCost(), unit*730)
	}
	if row.GetCurrency() != "USD" {
		t.Fatalf("%s currency = %q", region, row.GetCurrency())
	}
}
