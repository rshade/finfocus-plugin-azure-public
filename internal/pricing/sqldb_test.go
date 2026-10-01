package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	sqlTestComputeProduct  = "SQL Database Single/Elastic Pool General Purpose - Compute Gen5"
	sqlTestStorageProduct  = "SQL Database Single/Elastic Pool General Purpose - Storage"
	sqlTestService         = "SQL Database"
	sqlTestMeterVCore      = "vCore"
	sqlTestMeterZone       = "Zone Redundancy vCore"
	sqlTestMeterStored     = "General Purpose Data Stored"
	sqlTestMeterStoredFree = "General Purpose Data Stored - Free"
	sqlTestMeterZoneStored = "General Purpose Zone Redundancy Data Stored"
	sqlTestUnitHour        = "1 Hour"
	sqlTestUnitGBMonth     = "1 GB/Month"
	sqlTestStorageSKU      = "General Purpose"
	sqlTestZoneStorageSKU  = "General Purpose Zone Redundancy"
	sqlTestComputeFile     = "testdata/retail/sqldb/gp_gen5_compute_eastus.json"
	sqlTestStorageFile     = "testdata/retail/sqldb/gp_storage_eastus.json"
	sqlTestCanonicalType   = "sql/Database"
	sqlTestPulumiType      = "azure:sql/database:Database"
	sqlTestTask            = "AZ-2.7"
	sqlTestSizeGB          = 100.0
	sqlTestVCores          = 2
)

func TestGetProjectedCostSQLGPGen5FromFixture(t *testing.T) {
	t.Parallel()

	compute, storage := loadSQLFixtures(t)
	computeItem := requireSQLItem(
		t, compute.Items, sqlTestComputeProduct, "2 vCore", sqlTestMeterVCore, sqlTestUnitHour,
	)
	storageItem := requireSQLItem(
		t, storage.Items, sqlTestStorageProduct, sqlTestStorageSKU, sqlTestMeterStored, sqlTestUnitGBMonth,
	)
	if computeItem.RetailPrice == 0 || storageItem.RetailPrice == 0 {
		t.Fatal("fixture retail price is zero")
	}

	computeMonthly := computeItem.RetailPrice * pluginsdk.HoursPerMonth
	storageMonthly := storageItem.RetailPrice * sqlTestSizeGB
	doubledCompute := computeItem.RetailPrice * float64(sqlTestVCores) * pluginsdk.HoursPerMonth
	storageTimes730 := storageMonthly * pluginsdk.HoursPerMonth
	if math.Abs(doubledCompute-computeMonthly) <= 1e-6 {
		t.Fatal("fixture cannot distinguish compute multiplied by the vCore count")
	}
	if math.Abs(storageTimes730-storageMonthly) <= 1e-6 {
		t.Fatal("fixture cannot distinguish storage multiplied by 730")
	}
	wantMonthly := computeMonthly + storageMonthly
	want := map[string]float64{
		"compute": computeMonthly,
		"storage": storageMonthly,
	}

	calc := newSQLCalc(t, compute.Items, storage.Items)
	tests := []struct {
		name         string
		resourceType string
		sku          string
		tags         map[string]string
	}{
		{
			name:         "canonical",
			resourceType: sqlTestCanonicalType,
			sku:          "GP_Gen5_2",
			tags:         map[string]string{"size_gb": "100"},
		},
		{
			name:         "pulumi type and lower-case sku",
			resourceType: sqlTestPulumiType,
			sku:          "gp_gen5_2",
			tags:         map[string]string{"size_gb": "100"},
		},
		{
			name:         "tier hardware and vcores tags",
			resourceType: sqlTestCanonicalType,
			tags: map[string]string{
				"tier":     "GeneralPurpose",
				"hardware": "Gen5",
				"vcores":   "2",
				"size_gb":  "100",
			},
		},
		{
			name:         "gp tier alias and sizeGb",
			resourceType: sqlTestCanonicalType,
			tags: map[string]string{
				"tier":     "GP",
				"hardware": "gen5",
				"vcores":   "2",
				"sizeGb":   "100",
			},
		},
		{
			name:         "zone_redundant false omits zone components",
			resourceType: sqlTestCanonicalType,
			sku:          "GP_Gen5_2",
			tags:         map[string]string{"size_gb": "100", "zone_redundant": "false"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.GetProjectedCost(
				context.Background(),
				sqlProjectedRequest(tt.resourceType, "eastus", tt.sku, tt.tags),
			)
			if err != nil {
				t.Fatalf("GetProjectedCost() failed: %v", err)
			}
			assertSQLBreakdown(t, resp, want, computeItem.RetailPrice)
			got := resp.GetCostPerMonth()
			if math.Abs(got-wantMonthly) > 1e-9 ||
				math.Abs(got-(doubledCompute+storageMonthly)) <= 1e-9 ||
				math.Abs(got-(computeMonthly+storageTimes730)) <= 1e-9 {
				t.Fatalf(
					"cost_per_month = %v, want %v (2 vCore * 730 + stored * 100), not doubled compute %v or storage*730 %v",
					got,
					wantMonthly,
					doubledCompute+storageMonthly,
					computeMonthly+storageTimes730,
				)
			}
			if _, ok := resp.GetCostBreakdown()["zone_redundancy_compute"]; ok {
				t.Fatal("zone_redundancy_compute included without zone_redundant=true")
			}
			if _, ok := resp.GetCostBreakdown()["zone_redundancy_storage"]; ok {
				t.Fatal("zone_redundancy_storage included without zone_redundant=true")
			}
		})
	}
}

func TestGetProjectedCostSQLSKUBeatsTags(t *testing.T) {
	t.Parallel()

	compute, storage := loadSQLFixtures(t)
	item := requireSQLItem(t, compute.Items, sqlTestComputeProduct, "2 vCore", sqlTestMeterVCore, sqlTestUnitHour)
	eight := requireSQLItem(t, compute.Items, sqlTestComputeProduct, "8 vCore", sqlTestMeterVCore, sqlTestUnitHour)
	if item.RetailPrice == eight.RetailPrice {
		t.Fatal("2 vCore and 8 vCore retail prices match")
	}
	calc := newSQLCalc(t, compute.Items, storage.Items)

	resp, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
		sqlTestCanonicalType,
		"eastus",
		"GP_Gen5_2",
		map[string]string{
			"tier":     "GeneralPurpose",
			"hardware": "Gen5",
			"vcores":   "8",
			"size_gb":  "100",
		},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	want := item.RetailPrice * pluginsdk.HoursPerMonth
	if math.Abs(resp.GetCostBreakdown()["compute"]-want) > 1e-9 {
		t.Fatalf("compute = %v, want 2 vCore monthly %v", resp.GetCostBreakdown()["compute"], want)
	}
}

func TestGetProjectedCostSQLZoneRedundantFromFixture(t *testing.T) {
	t.Parallel()

	compute, storage := loadSQLFixtures(t)
	computeItem := requireSQLItem(
		t, compute.Items, sqlTestComputeProduct, "2 vCore", sqlTestMeterVCore, sqlTestUnitHour,
	)
	zoneCompute := requireSQLItem(
		t, compute.Items, sqlTestComputeProduct, "2 vCore Zone Redundancy", sqlTestMeterZone, sqlTestUnitHour,
	)
	storageItem := requireSQLItem(
		t, storage.Items, sqlTestStorageProduct, sqlTestStorageSKU, sqlTestMeterStored, sqlTestUnitGBMonth,
	)
	zoneStorage := requireSQLItem(
		t, storage.Items, sqlTestStorageProduct, sqlTestZoneStorageSKU, sqlTestMeterZoneStored, sqlTestUnitGBMonth,
	)
	if zoneCompute.RetailPrice == 0 || zoneStorage.RetailPrice == 0 {
		t.Fatal("zone retail price is zero")
	}

	want := map[string]float64{
		"compute":                 computeItem.RetailPrice * pluginsdk.HoursPerMonth,
		"storage":                 storageItem.RetailPrice * sqlTestSizeGB,
		"zone_redundancy_compute": zoneCompute.RetailPrice * pluginsdk.HoursPerMonth,
		"zone_redundancy_storage": zoneStorage.RetailPrice * sqlTestSizeGB,
	}
	calc := newSQLCalc(t, compute.Items, storage.Items)
	resp, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
		sqlTestCanonicalType,
		"eastus",
		"GP_Gen5_2",
		map[string]string{"size_gb": "100", "zone_redundant": "true"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	assertSQLBreakdown(t, resp, want, computeItem.RetailPrice)
}

func TestGetProjectedCostSQLMissingVCoreRowIsNotFound(t *testing.T) {
	t.Parallel()

	compute, storage := loadSQLFixtures(t)
	kept := withoutSQLSKU(compute.Items, "2 vCore", sqlTestMeterVCore)
	if sqlSKUPresent(kept, "2 vCore", sqlTestMeterVCore) {
		t.Fatal("2 vCore row remains")
	}
	if !sqlSKUPresent(kept, "vCore", sqlTestMeterVCore) {
		t.Fatal("unit vCore row is missing")
	}

	calc := newSQLCalc(t, kept, storage.Items)
	_, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
		sqlTestCanonicalType, "eastus", "GP_Gen5_2", map[string]string{"size_gb": "100"},
	))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (err=%v)", status.Code(err), err)
	}
}

func TestGetProjectedCostSQLMissingZoneVCoreRowIsNotFound(t *testing.T) {
	t.Parallel()

	compute, storage := loadSQLFixtures(t)
	kept := withoutSQLSKU(compute.Items, "2 vCore Zone Redundancy", sqlTestMeterZone)
	if sqlSKUPresent(kept, "2 vCore Zone Redundancy", sqlTestMeterZone) {
		t.Fatal("2 vCore zone row remains")
	}
	if !sqlSKUPresent(kept, "vCore ZR Zone Redundancy", sqlTestMeterZone) {
		t.Fatal("unit zone row is missing")
	}

	calc := newSQLCalc(t, kept, storage.Items)
	_, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
		sqlTestCanonicalType,
		"eastus",
		"GP_Gen5_2",
		map[string]string{"size_gb": "100", "zone_redundant": "true"},
	))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (err=%v)", status.Code(err), err)
	}
}

func TestGetProjectedCostSQLMissingPaidStorageIsNotFound(t *testing.T) {
	t.Parallel()

	compute, storage := loadSQLFixtures(t)
	free := requireSQLItem(
		t, storage.Items, sqlTestStorageProduct, sqlTestStorageSKU, sqlTestMeterStoredFree, sqlTestUnitGBMonth,
	)
	if free.RetailPrice != 0 {
		t.Fatal("free storage meter is not zero")
	}
	kept := withoutSQLMeter(storage.Items, sqlTestMeterStored)
	if sqlMeterPresent(kept, sqlTestMeterStored) {
		t.Fatal("paid storage row remains")
	}
	if !sqlMeterPresent(kept, sqlTestMeterStoredFree) {
		t.Fatal("free storage row is missing")
	}

	calc := newSQLCalc(t, compute.Items, kept)
	_, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
		sqlTestCanonicalType, "eastus", "GP_Gen5_2", map[string]string{"size_gb": "100"},
	))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (err=%v)", status.Code(err), err)
	}
	if strings.Contains(strings.ToLower(status.Convert(err).Message()), "free") &&
		!strings.Contains(status.Convert(err).Message(), sqlTestMeterStored) {
		t.Fatalf("message %q selected the free meter", status.Convert(err).Message())
	}
}

func TestGetProjectedCostSQLRefusesOtherModels(t *testing.T) {
	t.Parallel()

	calc := newSQLCalc(t, nil, nil)
	tests := []struct {
		name  string
		sku   string
		tags  map[string]string
		model string
	}{
		{name: "dtu", sku: "S0", model: "DTU"},
		{name: "serverless", sku: "GP_S_Gen5_2", model: "serverless"},
		{name: "business critical", sku: "BC_Gen5_2", model: "Business Critical"},
		{name: "hyperscale", sku: "HS_Gen5_2", model: "Hyperscale"},
		{name: "other hardware", sku: "GP_Fsv2_2", model: "Fsv2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tags := tt.tags
			if tags == nil {
				tags = map[string]string{"size_gb": "100"}
			}
			_, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
				sqlTestCanonicalType, "eastus", tt.sku, tags,
			))
			if status.Code(err) != codes.Unimplemented {
				t.Fatalf("code = %s, want Unimplemented (err=%v)", status.Code(err), err)
			}
			message := status.Convert(err).Message()
			if !strings.Contains(message, sqlTestTask) || !strings.Contains(message, tt.model) {
				t.Fatalf("message %q does not contain %s and %s", message, sqlTestTask, tt.model)
			}
		})
	}
}

func TestGetProjectedCostSQLMissingFields(t *testing.T) {
	t.Parallel()

	calc := newSQLCalc(t, nil, nil)
	tests := []struct {
		name   string
		region string
		sku    string
		tags   map[string]string
		field  string
	}{
		{
			name:  "region",
			sku:   "GP_Gen5_2",
			tags:  map[string]string{"size_gb": "100"},
			field: "region",
		},
		{
			name:   "size_gb",
			region: "eastus",
			sku:    "GP_Gen5_2",
			field:  "size_gb",
		},
		{
			name:   "vcores",
			region: "eastus",
			tags: map[string]string{
				"tier":     "GeneralPurpose",
				"hardware": "Gen5",
				"size_gb":  "100",
			},
			field: "vcores",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
				sqlTestCanonicalType, tt.region, tt.sku, tt.tags,
			))
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %s, want InvalidArgument (err=%v)", status.Code(err), err)
			}
			if !strings.Contains(status.Convert(err).Message(), tt.field) {
				t.Fatalf("message %q does not name %s", status.Convert(err).Message(), tt.field)
			}
		})
	}
}

func TestGetProjectedCostSQLOverGRPC(t *testing.T) {
	t.Parallel()

	compute, storage := loadSQLFixtures(t)
	computeItem := requireSQLItem(
		t, compute.Items, sqlTestComputeProduct, "2 vCore", sqlTestMeterVCore, sqlTestUnitHour,
	)
	storageItem := requireSQLItem(
		t, storage.Items, sqlTestStorageProduct, sqlTestStorageSKU, sqlTestMeterStored, sqlTestUnitGBMonth,
	)
	want := computeItem.RetailPrice*pluginsdk.HoursPerMonth + storageItem.RetailPrice*sqlTestSizeGB
	calc := newSQLCalc(t, compute.Items, storage.Items)

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
		sqlProjectedRequest(sqlTestCanonicalType, "eastus", "GP_Gen5_2", map[string]string{"size_gb": "100"}),
	)
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	if validateErr := pluginsdk.ValidateGetProjectedCostResponse(resp); validateErr != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", validateErr)
	}
	got := resp.GetCostPerMonth()
	storageTimes730 := storageItem.RetailPrice * sqlTestSizeGB * pluginsdk.HoursPerMonth
	doubled := computeItem.RetailPrice * float64(sqlTestVCores) * pluginsdk.HoursPerMonth
	if math.Abs(got-want) > 1e-9 || math.Abs(got-(doubled+storageItem.RetailPrice*sqlTestSizeGB)) <= 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v, not compute times vcores", got, want)
	}
	if math.Abs(got-(computeItem.RetailPrice*pluginsdk.HoursPerMonth+storageTimes730)) <= 1e-9 {
		t.Fatalf("cost_per_month = %v multiplied storage by 730", got)
	}
	if math.Abs(resp.GetCostBreakdown()["compute"]-computeItem.RetailPrice*pluginsdk.HoursPerMonth) > 1e-9 {
		t.Fatalf("compute = %v", resp.GetCostBreakdown()["compute"])
	}
	if math.Abs(resp.GetCostBreakdown()["storage"]-storageItem.RetailPrice*sqlTestSizeGB) > 1e-9 {
		t.Fatalf("storage = %v", resp.GetCostBreakdown()["storage"])
	}
}

func TestSQLQueryUsesProductNotArmSKU(t *testing.T) {
	t.Parallel()

	compute, storage := loadSQLFixtures(t)
	var mu sync.Mutex
	var filters []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		mu.Lock()
		filters = append(filters, filter)
		mu.Unlock()
		items := sqlItemsForFilter(filter, compute.Items, storage.Items)
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

	_, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
		sqlTestCanonicalType,
		"eastus",
		"GP_Gen5_2",
		map[string]string{"size_gb": "100", "currency": "EUR"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), filters...)
	mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("queries = %d, want one compute and one storage: %v", len(got), got)
	}
	sawCompute := false
	sawStorage := false
	for _, filter := range got {
		if strings.Contains(filter, "armSkuName") || strings.Contains(filter, "GP_Gen5_2") {
			t.Fatalf("filter includes arm sku: %s", filter)
		}
		for _, want := range []string{
			"armRegionName eq 'eastus'",
			"serviceName eq '" + sqlTestService + "'",
			"priceType eq 'Consumption'",
			"currencyCode eq 'EUR'",
		} {
			if !strings.Contains(filter, want) {
				t.Fatalf("filter %q does not contain %q", filter, want)
			}
		}
		if strings.Contains(filter, sqlTestComputeProduct) {
			sawCompute = true
		}
		if strings.Contains(filter, sqlTestStorageProduct) {
			sawStorage = true
		}
	}
	if !sawCompute || !sawStorage {
		t.Fatalf("filters = %v, want both products", got)
	}
}

func TestMapDescriptorToQuerySQLDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desc    *finfocusv1.ResourceDescriptor
		wantErr string
		errIs   error
	}{
		{
			name: "canonical sku",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: sqlTestCanonicalType,
				Region:       "eastus",
				Sku:          "GP_Gen5_2",
			},
		},
		{
			name: "pulumi",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: sqlTestPulumiType,
				Region:       "eastus",
				Sku:          "GP_Gen5_2",
			},
		},
		{
			name: "tags",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: sqlTestCanonicalType,
				Region:       "eastus",
				Tags: map[string]string{
					"tier":     "GeneralPurpose",
					"hardware": "Gen5",
					"vcores":   "2",
				},
			},
		},
		{
			name: "missing vcores",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: sqlTestCanonicalType,
				Region:       "eastus",
				Tags:         map[string]string{"tier": "GP", "hardware": "Gen5"},
			},
			wantErr: "vcores",
			errIs:   ErrMissingRequiredFields,
		},
		{
			name: "prefix is unsupported",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "sql/databaseextra",
				Region:       "eastus",
				Sku:          "GP_Gen5_2",
			},
			errIs: ErrUnsupportedResourceType,
		},
		{
			name: "mysql is not sql database",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "mysql/database",
				Region:       "eastus",
				Sku:          "GP_Gen5_2",
			},
			errIs: ErrUnsupportedResourceType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			query, err := MapDescriptorToQuery(tt.desc)
			if tt.errIs != nil {
				if !errors.Is(err, tt.errIs) {
					t.Fatalf("err = %v, want %v", err, tt.errIs)
				}
				if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not name %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MapDescriptorToQuery() failed: %v", err)
			}
			if query.ArmSkuName != "" || query.ProductName != "" {
				t.Fatalf("ArmSkuName = %q ProductName = %q, want empty", query.ArmSkuName, query.ProductName)
			}
			if query.ServiceName != sqlTestService {
				t.Fatalf("ServiceName = %q, want %s", query.ServiceName, sqlTestService)
			}
			if query.ArmRegionName != "eastus" || query.CurrencyCode != "USD" {
				t.Fatalf("region = %q currency = %q", query.ArmRegionName, query.CurrencyCode)
			}
		})
	}
}

func TestGetProjectedCostSQLPrefixIsUnsupported(t *testing.T) {
	t.Parallel()

	calc := newSQLCalc(t, nil, nil)
	_, err := calc.GetProjectedCost(context.Background(), sqlProjectedRequest(
		"sql/databaseextra", "eastus", "GP_Gen5_2", map[string]string{"size_gb": "100"},
	))
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("code = %s, want Unimplemented (err=%v)", status.Code(err), err)
	}
	if strings.Contains(status.Convert(err).Message(), sqlTestTask) {
		t.Fatalf("unknown type was treated as a refused SQL model: %v", err)
	}
}

func assertSQLBreakdown(
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
		t.Fatalf("unit_price = %v, want hourly compute retailPrice %v", resp.GetUnitPrice(), unit)
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

func loadSQLFixtures(t *testing.T) (azureclient.PriceResponse, azureclient.PriceResponse) {
	t.Helper()

	return loadRetailFixture(t, sqlTestComputeFile), loadRetailFixture(t, sqlTestStorageFile)
}

func newSQLCalc(t *testing.T, compute, storage []azureclient.PriceItem) *Calculator {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := sqlItemsForFilter(r.URL.Query().Get("$filter"), compute, storage)
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

func sqlItemsForFilter(filter string, compute, storage []azureclient.PriceItem) []azureclient.PriceItem {
	switch {
	case strings.Contains(filter, sqlTestComputeProduct):
		return compute
	case strings.Contains(filter, sqlTestStorageProduct):
		return storage
	default:
		return nil
	}
}

func sqlProjectedRequest(resourceType, region, sku string, tags map[string]string) *finfocusv1.GetProjectedCostRequest {
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

func requireSQLItem(
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
		if item.UnitOfMeasure != unit || item.Type != "Consumption" {
			continue
		}
		found = append(found, item)
	}
	if len(found) == 0 {
		t.Fatalf("fixture missing sku %q meter %q", sku, meter)
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

func withoutSQLSKU(items []azureclient.PriceItem, sku, meter string) []azureclient.PriceItem {
	var kept []azureclient.PriceItem
	for _, item := range items {
		if item.SkuName == sku && item.MeterName == meter {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func withoutSQLMeter(items []azureclient.PriceItem, meter string) []azureclient.PriceItem {
	var kept []azureclient.PriceItem
	for _, item := range items {
		if item.MeterName == meter {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func sqlSKUPresent(items []azureclient.PriceItem, sku, meter string) bool {
	for _, item := range items {
		if item.SkuName == sku && item.MeterName == meter {
			return true
		}
	}
	return false
}

func sqlMeterPresent(items []azureclient.PriceItem, meter string) bool {
	for _, item := range items {
		if item.MeterName == meter {
			return true
		}
	}
	return false
}
