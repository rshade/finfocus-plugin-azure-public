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
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	specBillingPerHour    = "per_hour"
	specBillingPerGBMonth = "per_gb_month"
	specBillingPerSecond  = "per_second"
	specUnitHour          = "hour"
	specUnitGBMonth       = "GB-month"
	specSourceRetail      = "azure-retail-prices"
)

type pricingSpecWant struct {
	billingMode string
	unit        string
	rate        float64
	hints       map[string]string
	reject      []float64
}

type pricingSpecFX struct {
	vmB1s      []azureclient.PriceItem
	vm         []azureclient.PriceItem
	disk       []azureclient.PriceItem
	blob       []azureclient.PriceItem
	storage    []azureclient.PriceItem
	app        []azureclient.PriceItem
	functions  []azureclient.PriceItem
	aks        []azureclient.PriceItem
	sqlCompute []azureclient.PriceItem
	sqlStorage []azureclient.PriceItem
	cosmos     []azureclient.PriceItem

	vmItem         azureclient.PriceItem
	diskItem       azureclient.PriceItem
	blobItem       azureclient.PriceItem
	storageItem    azureclient.PriceItem
	appItem        azureclient.PriceItem
	execItem       azureclient.PriceItem
	gbItem         azureclient.PriceItem
	vcpuItem       azureclient.PriceItem
	memoryItem     azureclient.PriceItem
	aksItem        azureclient.PriceItem
	nodeItem       azureclient.PriceItem
	sqlComputeItem azureclient.PriceItem
	sqlStorageItem azureclient.PriceItem
	cosmosRU       azureclient.PriceItem
	cosmosStored   azureclient.PriceItem
}

func TestGetPricingSpecEverySupportedType(t *testing.T) {
	t.Parallel()

	fx := loadPricingSpecFX(t)
	wants := pricingSpecWants(t, fx)
	calc := newPricingSpecCalc(t, fx)
	descriptors := dryRunDescriptors()
	seen := make(map[string]struct{}, len(SupportedResourceTypes()))

	for _, resourceType := range SupportedResourceTypes() {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()

			desc, ok := descriptors[resourceType]
			if !ok {
				t.Fatalf("no projected-cost descriptor for %s", resourceType)
			}
			want, ok := wants[resourceType]
			if !ok {
				t.Fatalf("no pricing spec expectation for %s", resourceType)
			}
			if desc.GetResourceType() != resourceType {
				t.Fatalf("descriptor type = %q, want %q", desc.GetResourceType(), resourceType)
			}

			resp, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{
				Resource: desc,
			})
			if err != nil {
				t.Fatalf("GetPricingSpec() error = %v", err)
			}
			assertPricingSpec(t, resp, desc, want)
		})
		seen[resourceType] = struct{}{}
	}
	if len(seen) != len(SupportedResourceTypes()) || len(seen) != len(descriptors) || len(seen) != len(wants) {
		t.Fatalf(
			"covered %d types, supported %d, descriptors %d, expectations %d",
			len(seen),
			len(SupportedResourceTypes()),
			len(descriptors),
			len(wants),
		)
	}
}

func TestGetPricingSpecPremiumFunctions(t *testing.T) {
	t.Parallel()

	fx := loadPricingSpecFX(t)
	if fx.vcpuItem.UnitOfMeasure != "1 Hour" || fx.memoryItem.UnitOfMeasure != "1 GiB Hour" {
		t.Fatalf("premium units vcpu=%q memory=%q", fx.vcpuItem.UnitOfMeasure, fx.memoryItem.UnitOfMeasure)
	}
	if fx.vcpuItem.RetailPrice == fx.memoryItem.RetailPrice {
		t.Fatal("premium fixture cannot distinguish vcpu from memory")
	}

	calc := newPricingSpecCalc(t, fx)
	resp, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "web/FunctionApp",
			Region:       "eastus",
			Sku:          "Premium",
			Tags: map[string]string{
				"vcpu_count": "2",
				"memory_gib": "4",
			},
		},
	})
	if err != nil {
		t.Fatalf("GetPricingSpec() error = %v", err)
	}
	assertPricingSpec(t, resp, &finfocusv1.ResourceDescriptor{
		ResourceType: "web/FunctionApp",
		Region:       "eastus",
	}, pricingSpecWant{
		billingMode: specBillingPerHour,
		unit:        specUnitHour,
		rate:        fx.vcpuItem.RetailPrice,
		hints: map[string]string{
			"vcpu":   fx.vcpuItem.UnitOfMeasure,
			"memory": fx.memoryItem.UnitOfMeasure,
		},
		reject: []float64{
			fx.memoryItem.RetailPrice,
			fx.vcpuItem.RetailPrice * pluginsdk.HoursPerMonth,
		},
	})
}

func TestGetPricingSpecMissingFieldNamesTheField(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	tests := []struct {
		name  string
		desc  *finfocusv1.ResourceDescriptor
		field string
	}{
		{
			name: "vm sku",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "compute/VirtualMachine",
				Region:       "eastus",
			},
			field: "sku",
		},
		{
			name: "disk size",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "storage/ManagedDisk",
				Region:       "eastus",
				Sku:          "Premium_SSD_LRS",
			},
			field: "size_gb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{
				Resource: tt.desc,
			})
			assertInvalidArgument(t, err, tt.field)
		})
	}
}

func TestGetPricingSpecUnsupportedType(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	_, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "compute/VirtualMachineScaleSet",
			Region:       "eastus",
			Sku:          "Standard_B1s",
		},
	})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("code = %s, want Unimplemented (%v)", status.Code(err), err)
	}
	if !strings.Contains(status.Convert(err).Message(), "compute/VirtualMachineScaleSet") {
		t.Fatalf("message %q does not name the type", status.Convert(err).Message())
	}
}

func TestGetPricingSpecNilClientNamesTask(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	_, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{
		Resource: dryRunDescriptors()["compute/VirtualMachine"],
	})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("code = %s, want Unimplemented (%v)", status.Code(err), err)
	}
	if !strings.Contains(status.Convert(err).Message(), "AZ-2.11") {
		t.Fatalf("message %q does not name AZ-2.11", status.Convert(err).Message())
	}
}

func assertPricingSpec(
	t *testing.T,
	resp *finfocusv1.GetPricingSpecResponse,
	desc *finfocusv1.ResourceDescriptor,
	want pricingSpecWant,
) {
	t.Helper()

	if resp == nil || resp.GetSpec() == nil {
		t.Fatal("spec is nil")
	}
	spec := resp.GetSpec()
	if err := plugintesting.ValidatePricingSpec(spec); err != nil {
		t.Fatalf("ValidatePricingSpec() error = %v", err)
	}
	if spec.GetProvider() != "azure" {
		t.Fatalf("provider = %q, want azure", spec.GetProvider())
	}
	if spec.GetResourceType() != desc.GetResourceType() {
		t.Fatalf("resource_type = %q, want %q", spec.GetResourceType(), desc.GetResourceType())
	}
	if spec.GetCurrency() != "USD" {
		t.Fatalf("currency = %q, want USD", spec.GetCurrency())
	}
	if spec.GetSource() != specSourceRetail {
		t.Fatalf("source = %q, want %s", spec.GetSource(), specSourceRetail)
	}
	if spec.GetRegion() != desc.GetRegion() {
		t.Fatalf("region = %q, want %q", spec.GetRegion(), desc.GetRegion())
	}
	if spec.GetSku() == "" {
		t.Fatal("sku is empty")
	}
	if spec.GetBillingMode() != want.billingMode {
		t.Fatalf("billing_mode = %q, want %q", spec.GetBillingMode(), want.billingMode)
	}
	if spec.GetUnit() != want.unit {
		t.Fatalf("unit = %q, want %q", spec.GetUnit(), want.unit)
	}
	if spec.GetRatePerUnit() == 0 {
		t.Fatal("rate_per_unit is zero")
	}
	if math.Abs(spec.GetRatePerUnit()-want.rate) > 1e-9 {
		t.Fatalf("rate_per_unit = %v, want meter retail %v", spec.GetRatePerUnit(), want.rate)
	}
	for _, other := range want.reject {
		if math.Abs(other-want.rate) <= 1e-9 {
			t.Fatalf("fixture cannot distinguish retail %v from %v", want.rate, other)
		}
		if math.Abs(spec.GetRatePerUnit()-other) <= 1e-9 {
			t.Fatalf("rate_per_unit = %v, want meter retail %v", spec.GetRatePerUnit(), want.rate)
		}
	}
	assertMetricHints(t, spec.GetMetricHints(), want.hints)
}

func assertMetricHints(t *testing.T, hints []*finfocusv1.UsageMetricHint, want map[string]string) {
	t.Helper()

	if len(hints) != len(want) {
		t.Fatalf("metric_hints = %v, want %v", hints, want)
	}
	got := make(map[string]string, len(hints))
	for _, hint := range hints {
		if _, dup := got[hint.GetMetric()]; dup {
			t.Fatalf("duplicate metric %q", hint.GetMetric())
		}
		got[hint.GetMetric()] = hint.GetUnit()
	}
	for metric, unit := range want {
		if got[metric] != unit {
			t.Fatalf("metric %s unit = %q, want %q (all %v)", metric, got[metric], unit, got)
		}
	}
}

func pricingSpecWants(t *testing.T, fx pricingSpecFX) map[string]pricingSpecWant {
	t.Helper()

	wants := map[string]pricingSpecWant{}
	addComputeStorageWants(t, fx, wants)
	addPlatformWants(t, fx, wants)
	addDataWants(t, fx, wants)
	return wants
}

func addComputeStorageWants(t *testing.T, fx pricingSpecFX, wants map[string]pricingSpecWant) {
	t.Helper()

	requireUSD(t, fx.vmItem, fx.diskItem, fx.blobItem, fx.storageItem)
	if fx.vmItem.UnitOfMeasure != "1 Hour" {
		t.Fatalf("vm unit = %q", fx.vmItem.UnitOfMeasure)
	}
	if fx.blobItem.UnitOfMeasure != "1 GB/Month" || fx.storageItem.UnitOfMeasure != "1 GB/Month" {
		t.Fatalf("blob unit = %q storage unit = %q", fx.blobItem.UnitOfMeasure, fx.storageItem.UnitOfMeasure)
	}

	wants["compute/VirtualMachine"] = hourlySpec(fx.vmItem, map[string]string{
		"compute": fx.vmItem.UnitOfMeasure,
	})
	wants["storage/ManagedDisk"] = gbMonthSpec(fx.diskItem, map[string]string{
		"storage": fx.diskItem.UnitOfMeasure,
	}, fx.diskItem.RetailPrice*pluginsdk.HoursPerMonth)
	wants["storage/BlobStorage"] = gbMonthSpec(fx.blobItem, map[string]string{
		"storage": fx.blobItem.UnitOfMeasure,
	}, fx.blobItem.RetailPrice*100)
	wants["storage/StorageAccount"] = gbMonthSpec(fx.storageItem, map[string]string{
		"storage": fx.storageItem.UnitOfMeasure,
	}, fx.storageItem.RetailPrice*100)
}

func addPlatformWants(t *testing.T, fx pricingSpecFX, wants map[string]pricingSpecWant) {
	t.Helper()

	requireUSD(t, fx.appItem, fx.execItem, fx.gbItem, fx.aksItem, fx.nodeItem)
	if fx.appItem.UnitOfMeasure != "1 Hour" ||
		fx.aksItem.UnitOfMeasure != "1 Hour" ||
		fx.nodeItem.UnitOfMeasure != "1 Hour" {
		t.Fatalf(
			"hour units app=%q aks=%q node=%q",
			fx.appItem.UnitOfMeasure,
			fx.aksItem.UnitOfMeasure,
			fx.nodeItem.UnitOfMeasure,
		)
	}
	if fx.execItem.UnitOfMeasure != "10" || fx.gbItem.UnitOfMeasure != "1 GB Second" {
		t.Fatalf("function units exec=%q gb=%q", fx.execItem.UnitOfMeasure, fx.gbItem.UnitOfMeasure)
	}
	if fx.aksItem.RetailPrice == fx.nodeItem.RetailPrice {
		t.Fatal("aks fixture cannot distinguish control plane from the node")
	}

	wants["web/AppServicePlan"] = hourlySpec(fx.appItem, map[string]string{
		"compute": fx.appItem.UnitOfMeasure,
	})
	wants["web/FunctionApp"] = pricingSpecWant{
		billingMode: specBillingPerSecond,
		unit:        fx.gbItem.UnitOfMeasure,
		rate:        fx.gbItem.RetailPrice,
		hints: map[string]string{
			"executions": fx.execItem.UnitOfMeasure,
			"gb_seconds": fx.gbItem.UnitOfMeasure,
		},
		reject: []float64{0, fx.execItem.RetailPrice, fx.gbItem.RetailPrice * pluginsdk.HoursPerMonth},
	}
	wants["containerservice/KubernetesCluster"] = pricingSpecWant{
		billingMode: specBillingPerHour,
		unit:        specUnitHour,
		rate:        fx.aksItem.RetailPrice,
		hints: map[string]string{
			"control_plane":    fx.aksItem.UnitOfMeasure,
			"node_pool_pool_1": fx.nodeItem.UnitOfMeasure,
			"node_pool_pool_2": fx.nodeItem.UnitOfMeasure,
		},
		reject: []float64{
			fx.nodeItem.RetailPrice,
			fx.aksItem.RetailPrice * pluginsdk.HoursPerMonth,
		},
	}
}

func addDataWants(t *testing.T, fx pricingSpecFX, wants map[string]pricingSpecWant) {
	t.Helper()

	requireUSD(t, fx.sqlComputeItem, fx.sqlStorageItem, fx.cosmosRU, fx.cosmosStored)
	if fx.sqlComputeItem.UnitOfMeasure != "1 Hour" || fx.sqlStorageItem.UnitOfMeasure != "1 GB/Month" {
		t.Fatalf("sql units compute=%q storage=%q", fx.sqlComputeItem.UnitOfMeasure, fx.sqlStorageItem.UnitOfMeasure)
	}
	if fx.cosmosRU.UnitOfMeasure != "1/Hour" || fx.cosmosStored.UnitOfMeasure != "1 GB/Month" {
		t.Fatalf("cosmos units ru=%q storage=%q", fx.cosmosRU.UnitOfMeasure, fx.cosmosStored.UnitOfMeasure)
	}

	wants["sql/Database"] = pricingSpecWant{
		billingMode: specBillingPerHour,
		unit:        specUnitHour,
		rate:        fx.sqlComputeItem.RetailPrice,
		hints: map[string]string{
			"compute": fx.sqlComputeItem.UnitOfMeasure,
			"storage": fx.sqlStorageItem.UnitOfMeasure,
		},
		reject: []float64{
			fx.sqlStorageItem.RetailPrice,
			fx.sqlComputeItem.RetailPrice * pluginsdk.HoursPerMonth,
		},
	}
	wants["cosmosdb/Account"] = pricingSpecWant{
		billingMode: specBillingPerHour,
		unit:        specUnitHour,
		rate:        fx.cosmosRU.RetailPrice,
		hints: map[string]string{
			"ru":      fx.cosmosRU.UnitOfMeasure,
			"storage": fx.cosmosStored.UnitOfMeasure,
		},
		reject: []float64{
			fx.cosmosStored.RetailPrice,
			fx.cosmosRU.RetailPrice * pluginsdk.HoursPerMonth,
		},
	}
}

func hourlySpec(item azureclient.PriceItem, hints map[string]string) pricingSpecWant {
	return pricingSpecWant{
		billingMode: specBillingPerHour,
		unit:        specUnitHour,
		rate:        item.RetailPrice,
		hints:       hints,
		reject:      []float64{item.RetailPrice * pluginsdk.HoursPerMonth},
	}
}

func gbMonthSpec(item azureclient.PriceItem, hints map[string]string, other float64) pricingSpecWant {
	return pricingSpecWant{
		billingMode: specBillingPerGBMonth,
		unit:        specUnitGBMonth,
		rate:        item.RetailPrice,
		hints:       hints,
		reject:      []float64{other},
	}
}

func requireUSD(t *testing.T, items ...azureclient.PriceItem) {
	t.Helper()

	for _, item := range items {
		if item.CurrencyCode != "USD" || item.RetailPrice == 0 {
			t.Fatalf(
				"fixture currency=%q retail=%v, want USD and a non-zero retail price",
				item.CurrencyCode,
				item.RetailPrice,
			)
		}
	}
}

func loadPricingSpecFX(t *testing.T) pricingSpecFX {
	t.Helper()

	vm := loadSpotRetailFixture(t)
	storage := loadStorageAccountFixture(t)
	app := loadRetailFixture(t, appServiceFixturePath)
	functions := loadRetailFixture(t, functionsFixturePath)
	aks := loadRetailFixture(t, aksFixturePath)
	sqlCompute, sqlStorage := loadSQLFixtures(t)
	cosmos := loadRetailFixture(t, cosmosTestFile)

	vmItem := azureclient.PriceItem{
		CurrencyCode:  "USD",
		ArmSkuName:    "Standard_B1s",
		ProductName:   "Virtual Machines B Series",
		MeterName:     "B1s",
		UnitOfMeasure: "1 Hour",
		Type:          "Consumption",
		RetailPrice:   0.0104,
	}
	diskItem := azureclient.PriceItem{
		CurrencyCode:  "USD",
		MeterName:     "P10 LRS Disk",
		UnitOfMeasure: "1/Month",
		RetailPrice:   19.71,
	}
	blobItem := azureclient.PriceItem{
		CurrencyCode:  "USD",
		MeterName:     "Hot LRS Data Stored",
		UnitOfMeasure: "1 GB/Month",
		RetailPrice:   0.0208,
	}

	return pricingSpecFX{
		vmB1s: []azureclient.PriceItem{
			{
				CurrencyCode:  "USD",
				ProductName:   "Virtual Machines B Series Windows",
				MeterName:     "B1s",
				UnitOfMeasure: "1 Hour",
				Type:          "Consumption",
				RetailPrice:   0.0192,
			},
			vmItem,
		},
		vm: vm.Items,
		disk: []azureclient.PriceItem{
			{MeterName: "P4 LRS Disk", RetailPrice: 5.28, CurrencyCode: "USD", UnitOfMeasure: "1/Month"},
			diskItem,
		},
		blob: []azureclient.PriceItem{
			{MeterName: "Hot LRS Write Operations", RetailPrice: 0.0001, CurrencyCode: "USD", UnitOfMeasure: "10K"},
			blobItem,
		},
		storage:     storage.Items,
		app:         app.Items,
		functions:   functions.Items,
		aks:         aks.Items,
		sqlCompute:  sqlCompute.Items,
		sqlStorage:  sqlStorage.Items,
		cosmos:      cosmos.Items,
		vmItem:      vmItem,
		diskItem:    diskItem,
		blobItem:    blobItem,
		storageItem: fixtureStorageBaseItem(t, storage.Items, "Hot LRS"),
		appItem:     fixtureAppPlan(t, app.Items, "P1v3", false),
		execItem:    fixturePositiveMeter(t, functions.Items, "Functions", "Standard Total Executions", "10"),
		gbItem:      fixturePositiveMeter(t, functions.Items, "Functions", "Standard Execution Time", "1 GB Second"),
		vcpuItem:    fixturePositiveMeter(t, functions.Items, "Premium Functions", "Premium vCPU Duration", "1 Hour"),
		memoryItem: fixturePositiveMeter(
			t, functions.Items, "Premium Functions", "Premium Memory Duration", "1 GiB Hour",
		),
		aksItem:  requireOpenAKSMeter(t, aks.Items, aksStandardMeter),
		nodeItem: fixtureVMItem(t, vm.Items, false),
		sqlComputeItem: requireSQLItem(
			t,
			sqlCompute.Items,
			sqlTestComputeProduct,
			"2 vCore",
			sqlTestMeterVCore,
			sqlTestUnitHour,
		),
		sqlStorageItem: requireSQLItem(
			t,
			sqlStorage.Items,
			sqlTestStorageProduct,
			sqlTestStorageSKU,
			sqlTestMeterStored,
			sqlTestUnitGBMonth,
		),
		cosmosRU: requireCosmosItem(
			t, cosmos.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterRU, cosmosTestUnitHour,
		),
		cosmosStored: requireCosmosItem(
			t, cosmos.Items, cosmosTestProduct, cosmosTestSKURU, cosmosTestMeterStored, cosmosTestUnitGBMonth,
		),
	}
}

func newPricingSpecCalc(t *testing.T, fx pricingSpecFX) *Calculator {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		items := pricingSpecItems(filter, fx)
		if items == nil {
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
	return NewCalculator(zerolog.Nop(), cached)
}

func pricingSpecItems(filter string, fx pricingSpecFX) []azureclient.PriceItem {
	switch {
	case strings.Contains(filter, "Azure Kubernetes Service"):
		return fx.aks
	case strings.Contains(filter, "Virtual Machines") && strings.Contains(filter, "Standard_B1s"):
		return fx.vmB1s
	case strings.Contains(filter, "Virtual Machines"):
		return fx.vm
	case strings.Contains(filter, "Managed Disks"), strings.Contains(filter, "Premium SSD Managed Disks"):
		return fx.disk
	case strings.Contains(filter, "Blob Storage"):
		return fx.blob
	case strings.Contains(filter, "General Block Blob v2"):
		return fx.storage
	case strings.Contains(filter, "serviceName eq 'Storage'"):
		return fx.blob
	case strings.Contains(filter, "Azure App Service"):
		return fx.app
	case strings.Contains(filter, "serviceName eq 'Functions'"):
		return fx.functions
	case strings.Contains(filter, sqlTestComputeProduct):
		return fx.sqlCompute
	case strings.Contains(filter, sqlTestStorageProduct):
		return fx.sqlStorage
	case strings.Contains(filter, "Azure Cosmos DB"):
		return fx.cosmos
	default:
		return nil
	}
}
