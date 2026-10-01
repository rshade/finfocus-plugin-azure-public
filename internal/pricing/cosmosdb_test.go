package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	cosmosTestFile              = "testdata/retail/cosmosdb/eastus_consumption.json"
	cosmosTestService           = "Azure Cosmos DB"
	cosmosTestProduct           = "Azure Cosmos DB"
	cosmosTestAutoscaleProduct  = "Azure Cosmos DB autoscale"
	cosmosTestServerlessProduct = "Azure Cosmos DB serverless"
	cosmosTestSKURU             = "RUs"
	cosmosTestSKUMRU            = "mRUs"
	cosmosTestSKUFreeTier       = "Free Tier"
	cosmosTestMeterRU           = "100 RU/s"
	cosmosTestMeterMulti        = "100 Multi-master RU/s"
	cosmosTestMeterStored       = "Data Stored"
	cosmosTestMeterServerless   = "1M RUs"
	cosmosTestMeterPerMinute    = "1000 RU/m"
	cosmosTestUnitHour          = "1/Hour"
	cosmosTestUnitGBMonth       = "1 GB/Month"
	cosmosTestUnitMillion       = "1M"
	cosmosTestAutoscaleSuffix   = "100 RUs"
	cosmosTestCanonical         = "cosmosdb/Account"
	cosmosTestPulumi            = "azure:cosmosdb/account:Account"
	cosmosTestRU                = 400.0
	cosmosTestGB                = 10.0
	cosmosTestRequestUnits      = 2_000_000.0
	cosmosTestAnalyticsProduct  = "Azure Cosmos DB Analytics Storage"
	cosmosTestAnalyticsMeter    = "Standard Data Stored"
	cosmosTestEntryMeter        = "AP1 Entry Price"
	cosmosTestEntrySKU          = "AP1"
)

func TestGetProjectedCostCosmosProvisionedFromFixture(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	ruItem := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU, cosmosTestUnitHour,
	)
	stored := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
	)
	free := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKUFreeTier, cosmosTestMeterRU, cosmosTestUnitHour,
	)
	if ruItem.RetailPrice == 0 || stored.RetailPrice == 0 {
		t.Fatal("fixture retail price is zero")
	}
	if free.RetailPrice != 0 {
		t.Fatal("free tier 100 RU/s retail price is not zero")
	}
	block := cosmosTestLeadingBlock(t, ruItem.MeterName)
	ruMonthly := (cosmosTestRU / block) * ruItem.RetailPrice * pluginsdk.HoursPerMonth
	undivided := cosmosTestRU * ruItem.RetailPrice * pluginsdk.HoursPerMonth
	storageMonthly := stored.RetailPrice * cosmosTestGB
	storageTimes730 := storageMonthly * pluginsdk.HoursPerMonth
	if math.Abs(undivided-ruMonthly) <= 1e-6 {
		t.Fatal("fixture cannot distinguish an RU block divisor")
	}
	if math.Abs(storageTimes730-storageMonthly) <= 1e-6 {
		t.Fatal("fixture cannot distinguish storage multiplied by 730")
	}
	wantMonthly := ruMonthly + storageMonthly
	want := map[string]float64{"ru": ruMonthly, "storage": storageMonthly}

	calc := newCosmosCalc(t, loaded.Items)
	tests := []struct {
		name         string
		resourceType string
		tags         map[string]string
	}{
		{
			name:         "canonical",
			resourceType: cosmosTestCanonical,
			tags:         map[string]string{"ru_per_second": "400", "size_gb": "10"},
		},
		{
			name:         "pulumi",
			resourceType: cosmosTestPulumi,
			tags:         map[string]string{"ru_per_second": "400", "size_gb": "10"},
		},
		{
			name:         "rus and sizeGb aliases",
			resourceType: "CosmosDB/Account",
			tags:         map[string]string{"rus": "400", "sizeGb": "10"},
		},
		{
			name:         "ru_per_second beats rus",
			resourceType: cosmosTestCanonical,
			tags:         map[string]string{"ru_per_second": "400", "rus": "800", "size_gb": "10"},
		},
		{
			name:         "multi_master false stays single master",
			resourceType: cosmosTestCanonical,
			tags: map[string]string{
				"ru_per_second": "400",
				"size_gb":       "10",
				"multi_master":  "false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.GetProjectedCost(
				context.Background(),
				cosmosProjectedRequest(tt.resourceType, "eastus", tt.tags),
			)
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			assertCosmosBreakdown(t, resp, want, ruItem.RetailPrice)
			got := resp.GetCostPerMonth()
			if math.Abs(got-wantMonthly) > 1e-9 ||
				math.Abs(got-storageMonthly) <= 1e-9 ||
				math.Abs(got-(ruMonthly+storageTimes730)) <= 1e-9 ||
				math.Abs(got-(undivided+storageMonthly)) <= 1e-9 {
				t.Fatalf(
					"cost_per_month = %v, want %v; free-tier %v storage*730 %v undivided %v",
					got,
					wantMonthly,
					storageMonthly,
					ruMonthly+storageTimes730,
					undivided+storageMonthly,
				)
			}
		})
	}
}

func TestGetProjectedCostCosmosRUBlockComesFromMeterName(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	ruItem := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU, cosmosTestUnitHour,
	)
	stored := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
	)
	const renamed = "50 RU/s"
	items := renameCosmosMeter(loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU, renamed)
	block := cosmosTestLeadingBlock(t, renamed)
	original := cosmosTestLeadingBlock(t, ruItem.MeterName)
	if block == original {
		t.Fatal("renamed block matches the fixture meter")
	}
	ruMonthly := (cosmosTestRU / block) * ruItem.RetailPrice * pluginsdk.HoursPerMonth
	hardCoded := (cosmosTestRU / original) * ruItem.RetailPrice * pluginsdk.HoursPerMonth
	want := map[string]float64{
		"ru":      ruMonthly,
		"storage": stored.RetailPrice * cosmosTestGB,
	}

	calc := newCosmosCalc(t, items)
	resp, err := calc.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		cosmosTestCanonical, "eastus", map[string]string{"ru_per_second": "400", "size_gb": "10"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	assertCosmosBreakdown(t, resp, want, ruItem.RetailPrice)
	if math.Abs(resp.GetCostBreakdown()["ru"]-hardCoded) <= 1e-9 {
		t.Fatal("RU block stayed on the original meter integer")
	}
}

func TestGetProjectedCostCosmosMultiMasterFromFixture(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	single := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU, cosmosTestUnitHour,
	)
	multi := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKUMRU, cosmosTestMeterMulti, cosmosTestUnitHour,
	)
	stored := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKUMRU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
	)
	if multi.RetailPrice == 0 || single.RetailPrice == multi.RetailPrice {
		t.Fatal("multi-master meter does not differ from the single-master meter")
	}
	block := cosmosTestLeadingBlock(t, multi.MeterName)
	singleBlock := cosmosTestLeadingBlock(t, single.MeterName)
	ruMonthly := (cosmosTestRU / block) * multi.RetailPrice * pluginsdk.HoursPerMonth
	singleMonthly := (cosmosTestRU / singleBlock) * single.RetailPrice * pluginsdk.HoursPerMonth
	if math.Abs(ruMonthly-singleMonthly) <= 1e-9 {
		t.Fatal("multi-master monthly RU matches single-master")
	}
	want := map[string]float64{
		"ru":      ruMonthly,
		"storage": stored.RetailPrice * cosmosTestGB,
	}

	calc := newCosmosCalc(t, loaded.Items)
	resp, err := calc.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		cosmosTestCanonical,
		"eastus",
		map[string]string{"ru_per_second": "400", "size_gb": "10", "multi_master": "true"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	assertCosmosBreakdown(t, resp, want, multi.RetailPrice)
	if math.Abs(resp.GetCostBreakdown()["ru"]-singleMonthly) <= 1e-9 {
		t.Fatal("multi_master used the single-master meter")
	}
}

func TestGetProjectedCostCosmosServerlessFromFixture(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	item := requireCosmosItem(
		t, loaded.Items, cosmosTestServerlessProduct, cosmosTestSKURU, cosmosTestMeterServerless, "",
	)
	if item.RetailPrice == 0 {
		t.Fatal("serverless retail price is zero")
	}
	divisor := cosmosTestUnitDivisor(t, item.UnitOfMeasure)
	wantRU := (cosmosTestRequestUnits / divisor) * item.RetailPrice
	if math.Abs(wantRU-2*item.RetailPrice) > 1e-9 {
		t.Fatalf("2,000,000 request units = %v, want 2 * retailPrice %v", wantRU, 2*item.RetailPrice)
	}
	times730 := wantRU * pluginsdk.HoursPerMonth
	if math.Abs(times730-wantRU) <= 1e-6 {
		t.Fatal("fixture cannot distinguish a 730 multiplier")
	}
	want := map[string]float64{"ru": wantRU}

	calc := newCosmosCalc(t, loaded.Items)
	tests := []struct {
		name string
		tags map[string]string
	}{
		{
			name: "request units only",
			tags: map[string]string{"pricing_model": "serverless", "request_units": "2000000"},
		},
		{
			name: "size_gb is ignored",
			tags: map[string]string{
				"pricing_model": "Serverless",
				"request_units": "2000000",
				"size_gb":       "10",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.GetProjectedCost(
				context.Background(),
				cosmosProjectedRequest(cosmosTestPulumi, "eastus", tt.tags),
			)
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			assertCosmosBreakdown(t, resp, want, item.RetailPrice)
			if _, ok := resp.GetCostBreakdown()["storage"]; ok {
				t.Fatal("serverless breakdown has storage")
			}
			if math.Abs(resp.GetCostPerMonth()-times730) <= 1e-9 {
				t.Fatal("serverless cost was multiplied by 730")
			}
		})
	}
}

func TestGetProjectedCostCosmosServerlessOmitsStorageMeter(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	item := requireCosmosItem(
		t, loaded.Items, cosmosTestServerlessProduct, cosmosTestSKURU, cosmosTestMeterServerless, cosmosTestUnitMillion,
	)
	stored := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
	)
	if !cosmosFixtureLacksServerlessStorage(loaded.Items) {
		t.Fatal("saved fixture contains a serverless storage meter; record it in the findings")
	}
	extra := stored
	extra.ProductName = cosmosTestServerlessProduct
	extra.SkuName = cosmosTestSKURU
	extra.MeterName = cosmosTestMeterStored
	items := append(append([]azureclient.PriceItem(nil), loaded.Items...), extra)
	want := (cosmosTestRequestUnits / cosmosTestUnitDivisor(t, item.UnitOfMeasure)) * item.RetailPrice

	calc := newCosmosCalc(t, items)
	resp, err := calc.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		cosmosTestCanonical,
		"eastus",
		map[string]string{"pricing_model": "serverless", "request_units": "2000000", "size_gb": "10"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if _, ok := resp.GetCostBreakdown()["storage"]; ok {
		t.Fatalf("serverless included storage: %v", resp.GetCostBreakdown())
	}
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v without storage", resp.GetCostPerMonth(), want)
	}
}

func TestGetProjectedCostCosmosAutoscaleFromFixture(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	rows := cosmosAutoscaleRUItems(t, loaded.Items)
	stored := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
	)
	entry := requireCosmosItem(
		t, loaded.Items, cosmosTestAutoscaleProduct, cosmosTestEntrySKU, cosmosTestEntryMeter, cosmosTestUnitHour,
	)
	price := rows[0].RetailPrice
	if price == 0 || price == entry.RetailPrice {
		t.Fatal("autoscale RU price matches the entry price or is zero")
	}
	if cosmosFixtureHasAutoscaleStorage(loaded.Items) {
		t.Fatal("saved fixture has an autoscale storage meter; the quote must use that meter")
	}
	block := cosmosTestSuffixBlock(t, rows[0].MeterName)
	ruMonthly := (cosmosTestRU / block) * price * pluginsdk.HoursPerMonth
	scaled := ruMonthly * 1.5
	storageMonthly := stored.RetailPrice * cosmosTestGB
	if math.Abs(scaled-ruMonthly) <= 1e-6 {
		t.Fatal("cannot distinguish a 1.5 multiplier")
	}
	if math.Abs(storageMonthly*pluginsdk.HoursPerMonth-storageMonthly) <= 1e-6 {
		t.Fatal("fixture cannot distinguish storage multiplied by 730")
	}
	want := map[string]float64{"ru": ruMonthly, "storage": storageMonthly}

	calc := newCosmosCalc(t, loaded.Items)
	for _, tags := range []map[string]string{
		{"pricing_model": "autoscale", "ru_per_second": "400", "size_gb": "10"},
		{"pricing_model": "AutoScale", "rus": "400", "size_gb": "10", "multi_master": "false"},
	} {
		resp, err := calc.GetProjectedCost(
			context.Background(),
			cosmosProjectedRequest(cosmosTestCanonical, "eastus", tags),
		)
		if err != nil {
			t.Fatalf("GetProjectedCost() failed: %v", err)
		}
		assertCosmosBreakdown(t, resp, want, price)
		if math.Abs(resp.GetCostBreakdown()["ru"]-scaled) <= 1e-9 {
			t.Fatal("autoscale RU was multiplied by 1.5")
		}
		if math.Abs(resp.GetCostBreakdown()["storage"]-storageMonthly*pluginsdk.HoursPerMonth) <= 1e-9 {
			t.Fatal("autoscale storage was multiplied by 730")
		}
	}
}

func TestGetProjectedCostCosmosAutoscaleStorageMeter(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	rows := cosmosAutoscaleRUItems(t, loaded.Items)
	provisioned := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
	)
	analytics := requireCosmosItem(
		t, loaded.Items, cosmosTestAnalyticsProduct, "Standard", cosmosTestAnalyticsMeter, cosmosTestUnitGBMonth,
	)
	if analytics.RetailPrice == 0 || analytics.RetailPrice == provisioned.RetailPrice {
		t.Fatal("analytics storage price cannot distinguish autoscale storage")
	}
	extra := analytics
	extra.ProductName = cosmosTestAutoscaleProduct
	extra.SkuName = cosmosTestEntrySKU
	extra.MeterName = cosmosTestMeterStored
	extra.UnitOfMeasure = cosmosTestUnitGBMonth
	extra.Type = "Consumption"
	items := append(append([]azureclient.PriceItem(nil), loaded.Items...), extra)

	block := cosmosTestSuffixBlock(t, rows[0].MeterName)
	ruMonthly := (cosmosTestRU / block) * rows[0].RetailPrice * pluginsdk.HoursPerMonth
	wantStorage := analytics.RetailPrice * cosmosTestGB
	want := map[string]float64{"ru": ruMonthly, "storage": wantStorage}

	calc := newCosmosCalc(t, items)
	resp, err := calc.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		cosmosTestCanonical,
		"eastus",
		map[string]string{"pricing_model": "autoscale", "ru_per_second": "400", "size_gb": "10"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	assertCosmosBreakdown(t, resp, want, rows[0].RetailPrice)
	if math.Abs(resp.GetCostBreakdown()["storage"]-provisioned.RetailPrice*cosmosTestGB) <= 1e-9 {
		t.Fatal("autoscale used the provisioned Data Stored meter")
	}
}

func TestGetProjectedCostCosmosAutoscalePricesMustAgree(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	items := append([]azureclient.PriceItem(nil), loaded.Items...)
	changed := false
	var meter string
	for i := range items {
		if items[i].ProductName != cosmosTestAutoscaleProduct ||
			!strings.HasSuffix(items[i].MeterName, cosmosTestAutoscaleSuffix) {
			continue
		}
		items[i].RetailPrice *= 2
		meter = items[i].MeterName
		changed = true
		break
	}
	if !changed || meter == "" {
		t.Fatal("no autoscale RU meter to split")
	}

	calc := newCosmosCalc(t, items)
	_, err := calc.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		cosmosTestCanonical,
		"eastus",
		map[string]string{"pricing_model": "autoscale", "ru_per_second": "400", "size_gb": "10"},
	))
	assertCosmosStatus(t, err, codes.InvalidArgument, meter)
}

func TestGetProjectedCostCosmosMeterMustStartWithInteger(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	const renamed = "RU/s"
	items := renameCosmosMeter(loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU, renamed)
	calc := newCosmosCalc(t, items)
	_, err := calc.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		cosmosTestCanonical, "eastus", map[string]string{"ru_per_second": "400", "size_gb": "10"},
	))
	assertCosmosStatus(t, err, codes.InvalidArgument, renamed)
}

func TestGetProjectedCostCosmosServerlessRejectsUnit(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	const badUnit = "1 Hour"
	items := append([]azureclient.PriceItem(nil), loaded.Items...)
	found := false
	for i := range items {
		if items[i].ProductName == cosmosTestServerlessProduct && items[i].MeterName == cosmosTestMeterServerless {
			items[i].UnitOfMeasure = badUnit
			found = true
		}
	}
	if !found {
		t.Fatal("serverless meter is missing")
	}
	calc := newCosmosCalc(t, items)
	_, err := calc.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		cosmosTestCanonical,
		"eastus",
		map[string]string{"pricing_model": "serverless", "request_units": "2000000"},
	))
	assertCosmosStatus(t, err, codes.InvalidArgument, badUnit)
}

func TestGetProjectedCostCosmosMissingMeterIsNotFound(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	tests := []struct {
		name  string
		items []azureclient.PriceItem
		tags  map[string]string
		names []string
	}{
		{
			name: "paid 100 RU/s",
			items: withoutCosmosRow(
				loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU,
			),
			tags:  map[string]string{"ru_per_second": "400", "size_gb": "10"},
			names: []string{cosmosTestMeterRU},
		},
		{
			name: "paid data stored",
			items: withoutCosmosRow(
				loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored,
			),
			tags:  map[string]string{"ru_per_second": "400", "size_gb": "10"},
			names: []string{cosmosTestMeterStored},
		},
		{
			name: "multi-master ru",
			items: withoutCosmosRow(
				loaded.Items, cosmosTestProduct, cosmosTestSKUMRU, cosmosTestMeterMulti,
			),
			tags: map[string]string{
				"ru_per_second": "400",
				"size_gb":       "10",
				"multi_master":  "true",
			},
			names: []string{cosmosTestMeterMulti},
		},
		{
			name: "multi-master storage",
			items: withoutCosmosRow(
				loaded.Items, cosmosTestProduct, cosmosTestSKUMRU, cosmosTestMeterStored,
			),
			tags: map[string]string{
				"ru_per_second": "400",
				"size_gb":       "10",
				"multi_master":  "true",
			},
			names: []string{cosmosTestSKUMRU, cosmosTestMeterStored},
		},
		{
			name: "serverless",
			items: withoutCosmosRow(
				loaded.Items, cosmosTestServerlessProduct, cosmosTestSKURU, cosmosTestMeterServerless,
			),
			tags:  map[string]string{"pricing_model": "serverless", "request_units": "2000000"},
			names: []string{cosmosTestMeterServerless},
		},
		{
			name:  "autoscale",
			items: withoutAutoscaleRU(loaded.Items),
			tags:  map[string]string{"pricing_model": "autoscale", "ru_per_second": "400", "size_gb": "10"},
			names: []string{cosmosTestAutoscaleSuffix},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calc := newCosmosCalc(t, tt.items)
			_, err := calc.GetProjectedCost(
				context.Background(),
				cosmosProjectedRequest(cosmosTestCanonical, "eastus", tt.tags),
			)
			assertCosmosStatus(t, err, codes.NotFound, tt.names...)
		})
	}
}

func TestGetProjectedCostCosmosInvalidArgument(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	calc := newCosmosCalc(t, loaded.Items)
	tests := []struct {
		name   string
		region string
		tags   map[string]string
		names  []string
		absent []string
	}{
		{
			name:   "missing ru_per_second",
			region: "eastus",
			tags:   map[string]string{"size_gb": "10"},
			names:  []string{"ru_per_second"},
		},
		{
			name:   "missing size_gb",
			region: "eastus",
			tags:   map[string]string{"ru_per_second": "400"},
			names:  []string{"size_gb"},
		},
		{
			name:   "missing ru and size",
			region: "eastus",
			tags:   map[string]string{},
			names:  []string{"ru_per_second", "size_gb"},
		},
		{
			name:   "missing region",
			region: "",
			tags:   map[string]string{"ru_per_second": "400", "size_gb": "10"},
			names:  []string{"region"},
		},
		{
			name:   "invalid ru_per_second",
			region: "eastus",
			tags:   map[string]string{"ru_per_second": "many", "size_gb": "10"},
			names:  []string{"ru_per_second"},
		},
		{
			name:   "invalid rus",
			region: "eastus",
			tags:   map[string]string{"rus": "0", "size_gb": "10"},
			names:  []string{"rus"},
		},
		{
			name:   "invalid size_gb",
			region: "eastus",
			tags:   map[string]string{"ru_per_second": "400", "size_gb": "0"},
			names:  []string{"size_gb"},
		},
		{
			name:   "bad pricing_model",
			region: "eastus",
			tags:   map[string]string{"pricing_model": "reserved", "ru_per_second": "400", "size_gb": "10"},
			names:  []string{"reserved"},
		},
		{
			name:   "manual is not a pricing_model value",
			region: "eastus",
			tags:   map[string]string{"pricing_model": "manual", "ru_per_second": "400", "size_gb": "10"},
			names:  []string{"manual"},
		},
		{
			name:   "serverless missing request_units",
			region: "eastus",
			tags:   map[string]string{"pricing_model": "serverless"},
			names:  []string{"request_units"},
			absent: []string{"size_gb"},
		},
		{
			name:   "autoscale missing size_gb",
			region: "eastus",
			tags:   map[string]string{"pricing_model": "autoscale", "ru_per_second": "400"},
			names:  []string{"size_gb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := calc.GetProjectedCost(
				context.Background(),
				cosmosProjectedRequest(cosmosTestCanonical, tt.region, tt.tags),
			)
			assertCosmosStatus(t, err, codes.InvalidArgument, tt.names...)
			msg := status.Convert(err).Message()
			for _, absent := range tt.absent {
				if strings.Contains(msg, absent) {
					t.Fatalf("message %q names %s", msg, absent)
				}
			}
		})
	}
}

func TestGetProjectedCostCosmosOverGRPC(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	ruItem := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU, cosmosTestUnitHour,
	)
	stored := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
	)
	block := cosmosTestLeadingBlock(t, ruItem.MeterName)
	wantRU := (cosmosTestRU / block) * ruItem.RetailPrice * pluginsdk.HoursPerMonth
	wantStorage := stored.RetailPrice * cosmosTestGB
	calc := newCosmosCalc(t, loaded.Items)
	client := dialPricingClient(t, calc)

	resp, err := client.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		cosmosTestCanonical, "eastus", map[string]string{"ru_per_second": "400", "size_gb": "10"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", validateErr)
	}
	if math.Abs(resp.GetCostBreakdown()["ru"]-wantRU) > 1e-9 {
		t.Fatalf("ru = %v, want %v", resp.GetCostBreakdown()["ru"], wantRU)
	}
	if math.Abs(resp.GetCostBreakdown()["storage"]-wantStorage) > 1e-9 {
		t.Fatalf("storage = %v, want %v", resp.GetCostBreakdown()["storage"], wantStorage)
	}
	if math.Abs(resp.GetCostPerMonth()-(wantRU+wantStorage)) > 1e-9 {
		t.Fatalf("cost_per_month = %v", resp.GetCostPerMonth())
	}
	if math.Abs(resp.GetCostBreakdown()["storage"]-wantStorage*pluginsdk.HoursPerMonth) <= 1e-9 {
		t.Fatal("storage was multiplied by 730")
	}
}

func TestGetActualCostCosmosDefaultWindow(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	ruItem := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU, cosmosTestUnitHour,
	)
	stored := requireCosmosItem(
		t, loaded.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
	)
	block := cosmosTestLeadingBlock(t, ruItem.MeterName)
	want := (cosmosTestRU/block)*ruItem.RetailPrice*pluginsdk.HoursPerMonth + stored.RetailPrice*cosmosTestGB
	calc := newCosmosCalc(t, loaded.Items)

	resp, err := calc.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		Tags: map[string]string{
			"region":        "eastus",
			"resource_type": cosmosTestCanonical,
			"ru_per_second": "400",
			"size_gb":       "10",
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
		t.Fatalf("cost = %v, want monthly %v", results[0].GetCost(), want)
	}
	if math.Abs(results[0].GetUsageAmount()-pluginsdk.HoursPerMonth) > 1e-9 {
		t.Fatalf("usage_hours = %v, want %v", results[0].GetUsageAmount(), pluginsdk.HoursPerMonth)
	}
}

func TestCosmosQueryOmitsArmSKU(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, cosmosTestFile)
	var mu sync.Mutex
	var filters []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		filters = append(filters, r.URL.Query().Get("$filter"))
		mu.Unlock()
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
	req := cosmosProjectedRequest(cosmosTestCanonical, "eastus", map[string]string{
		"ru_per_second": "400",
		"size_gb":       "10",
		"currency":      "EUR",
	})
	req.Resource.Sku = cosmosTestMeterPerMinute

	_, err := calc.GetProjectedCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	mu.Lock()
	got := append([]string(nil), filters...)
	mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("queries = %d, want 1: %v", len(got), got)
	}
	filter := got[0]
	if strings.Contains(filter, "armSkuName") || strings.Contains(filter, "productName") ||
		strings.Contains(filter, cosmosTestMeterPerMinute) {
		t.Fatalf("filter includes sku or product: %s", filter)
	}
	for _, want := range []string{
		"armRegionName eq 'eastus'",
		"serviceName eq '" + cosmosTestService + "'",
		"priceType eq 'Consumption'",
		"currencyCode eq 'EUR'",
	} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter %q does not contain %q", filter, want)
		}
	}
}

func TestMapDescriptorToQueryCosmosAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desc    *finfocusv1.ResourceDescriptor
		wantErr error
		names   string
	}{
		{
			name: "canonical without sku",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: cosmosTestCanonical,
				Region:       "eastus",
			},
		},
		{
			name: "pulumi sku is not an arm sku",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: cosmosTestPulumi,
				Region:       "eastus",
				Sku:          "RUm",
			},
		},
		{
			name: "missing region",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: cosmosTestCanonical,
			},
			wantErr: ErrMissingRequiredFields,
			names:   "region",
		},
		{
			name: "prefix is unsupported",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "cosmosdb/accountextra",
				Region:       "eastus",
			},
			wantErr: ErrUnsupportedResourceType,
		},
		{
			name: "database is not an account",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "azure:cosmosdb/database:Database",
				Region:       "eastus",
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
				if tt.names != "" && !strings.Contains(err.Error(), tt.names) {
					t.Fatalf("error %q does not name %q", err.Error(), tt.names)
				}
				return
			}
			if err != nil {
				t.Fatalf("MapDescriptorToQuery() failed: %v", err)
			}
			if query.ArmSkuName != "" || query.ProductName != "" {
				t.Fatalf("ArmSkuName = %q ProductName = %q, want empty", query.ArmSkuName, query.ProductName)
			}
			if query.ServiceName != cosmosTestService {
				t.Fatalf("ServiceName = %q, want %s", query.ServiceName, cosmosTestService)
			}
			if query.ArmRegionName != "eastus" || query.CurrencyCode != "USD" {
				t.Fatalf("region = %q currency = %q", query.ArmRegionName, query.CurrencyCode)
			}
		})
	}
}

func TestGetProjectedCostCosmosPrefixIsUnsupported(t *testing.T) {
	t.Parallel()

	calc := newCosmosCalc(t, nil)
	_, err := calc.GetProjectedCost(context.Background(), cosmosProjectedRequest(
		"cosmosdb/accountextra", "eastus", map[string]string{"ru_per_second": "400", "size_gb": "10"},
	))
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("code = %s, want Unimplemented (err=%v)", status.Code(err), err)
	}
}

func assertCosmosBreakdown(
	t *testing.T,
	resp *finfocusv1.GetProjectedCostResponse,
	want map[string]float64,
	unit float64,
) {
	t.Helper()

	if err := pluginsdk.ValidateGetProjectedCostResponse(resp); err != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", err)
	}
	if math.Abs(resp.GetUnitPrice()-unit) > 1e-9 {
		t.Fatalf("unit_price = %v, want retailPrice %v", resp.GetUnitPrice(), unit)
	}
	if resp.GetCurrency() == "" {
		t.Fatal("currency is empty")
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

func assertCosmosStatus(t *testing.T, err error, code codes.Code, names ...string) {
	t.Helper()

	if status.Code(err) != code {
		t.Fatalf("code = %s, want %s (err=%v)", status.Code(err), code, err)
	}
	msg := status.Convert(err).Message()
	for _, name := range names {
		if !strings.Contains(msg, name) {
			t.Fatalf("message %q does not name %s", msg, name)
		}
	}
}

func newCosmosCalc(t *testing.T, items []azureclient.PriceItem) *Calculator {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func cosmosProjectedRequest(resourceType, region string, tags map[string]string) *finfocusv1.GetProjectedCostRequest {
	return &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: resourceType,
			Region:       region,
			Tags:         tags,
		},
	}
}

func requireCosmosItem(
	t *testing.T,
	items []azureclient.PriceItem,
	product, sku, meter, unit string,
) azureclient.PriceItem {
	t.Helper()

	var found []azureclient.PriceItem
	for _, item := range items {
		if item.ProductName != product || item.SkuName != sku || item.MeterName != meter {
			continue
		}
		if unit != "" && item.UnitOfMeasure != unit {
			continue
		}
		if item.Type != "Consumption" {
			continue
		}
		found = append(found, item)
	}
	if len(found) == 0 {
		t.Fatalf("fixture missing sku %q meter %q unit %q", sku, meter, unit)
	}
	price := found[0].RetailPrice
	currency := found[0].CurrencyCode
	for _, item := range found[1:] {
		if item.RetailPrice != price || item.CurrencyCode != currency {
			t.Fatalf("fixture prices differ for sku %q meter %q", sku, meter)
		}
	}
	return found[0]
}

func cosmosAutoscaleRUItems(t *testing.T, items []azureclient.PriceItem) []azureclient.PriceItem {
	t.Helper()

	var found []azureclient.PriceItem
	for _, item := range items {
		if item.ProductName != cosmosTestAutoscaleProduct || item.Type != "Consumption" {
			continue
		}
		if item.UnitOfMeasure != cosmosTestUnitHour || !strings.HasSuffix(item.MeterName, cosmosTestAutoscaleSuffix) {
			continue
		}
		if item.MeterName != "" && item.MeterName[0] >= '0' && item.MeterName[0] <= '9' {
			t.Fatalf("autoscale meter %q starts with an integer", item.MeterName)
		}
		found = append(found, item)
	}
	if len(found) == 0 {
		t.Fatal("fixture has no autoscale 100 RUs meters")
	}
	price := found[0].RetailPrice
	for _, item := range found[1:] {
		if item.RetailPrice != price {
			t.Fatalf("fixture autoscale prices disagree: %v vs %v", price, item.RetailPrice)
		}
	}
	return found
}

func cosmosFixtureLacksServerlessStorage(items []azureclient.PriceItem) bool {
	for _, item := range items {
		if item.ProductName != cosmosTestServerlessProduct {
			continue
		}
		if strings.Contains(item.MeterName, cosmosTestMeterStored) || item.UnitOfMeasure == cosmosTestUnitGBMonth {
			return false
		}
	}
	return true
}

func cosmosFixtureHasAutoscaleStorage(items []azureclient.PriceItem) bool {
	for _, item := range items {
		if item.ProductName != cosmosTestAutoscaleProduct || item.Type != "Consumption" {
			continue
		}
		if item.UnitOfMeasure == cosmosTestUnitGBMonth && strings.Contains(item.MeterName, cosmosTestMeterStored) {
			return true
		}
	}
	return false
}

func withoutCosmosRow(items []azureclient.PriceItem, product, sku, meter string) []azureclient.PriceItem {
	var kept []azureclient.PriceItem
	for _, item := range items {
		if item.ProductName == product && item.SkuName == sku && item.MeterName == meter {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func withoutAutoscaleRU(items []azureclient.PriceItem) []azureclient.PriceItem {
	var kept []azureclient.PriceItem
	for _, item := range items {
		if item.ProductName == cosmosTestAutoscaleProduct &&
			strings.HasSuffix(item.MeterName, cosmosTestAutoscaleSuffix) {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func renameCosmosMeter(items []azureclient.PriceItem, product, sku, meter, renamed string) []azureclient.PriceItem {
	out := append([]azureclient.PriceItem(nil), items...)
	for i := range out {
		if out[i].ProductName == product && out[i].SkuName == sku && out[i].MeterName == meter {
			out[i].MeterName = renamed
		}
	}
	return out
}

func cosmosTestLeadingBlock(t *testing.T, meter string) float64 {
	t.Helper()

	i := 0
	for i < len(meter) && meter[i] >= '0' && meter[i] <= '9' {
		i++
	}
	if i == 0 {
		t.Fatalf("meter %q does not start with an integer", meter)
	}
	n, err := strconv.Atoi(meter[:i])
	if err != nil || n < 1 {
		t.Fatalf("meter %q block: %v", meter, err)
	}
	return float64(n)
}

func cosmosTestSuffixBlock(t *testing.T, meter string) float64 {
	t.Helper()

	if !strings.HasSuffix(meter, cosmosTestAutoscaleSuffix) {
		t.Fatalf("meter %q does not end with %s", meter, cosmosTestAutoscaleSuffix)
	}
	fields := strings.Fields(cosmosTestAutoscaleSuffix)
	n, err := strconv.Atoi(fields[0])
	if err != nil || n < 1 {
		t.Fatalf("suffix %q: %v", cosmosTestAutoscaleSuffix, err)
	}
	return float64(n)
}

func cosmosTestUnitDivisor(t *testing.T, unit string) float64 {
	t.Helper()

	if unit != cosmosTestUnitMillion {
		t.Fatalf("unitOfMeasure = %q, want %s; not guessing a divisor", unit, cosmosTestUnitMillion)
	}
	return 1_000_000
}
