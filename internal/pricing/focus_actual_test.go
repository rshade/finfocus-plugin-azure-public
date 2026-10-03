package pricing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	focusActualHours   = 24.0
	focusD2sv3Linux    = 0.096
	focusD2sv3Windows  = 0.188
	focusD2sv3Spot     = 0.018816
	focusAKSStandard   = 0.1
	focusDiskP10Month  = 19.71
	focusDiskMonthUnit = "1/Month"
)

// focusActualWant is the FOCUS pricing basis one GetActualCost record must carry.
// A zero unitPrice means the quote mixes meters, so both unit prices stay unset.
type focusActualWant struct {
	service   string
	unit      string
	quantity  float64
	unitPrice float64
}

func TestGetActualCost_RealQuotes_SetFocusPricingBasis(t *testing.T) {
	t.Parallel()

	hourly := func(service string, count, price float64) focusActualWant {
		return focusActualWant{
			service:   service,
			unit:      focusUnitHours,
			quantity:  focusActualHours * count,
			unitPrice: price,
		}
	}
	mixed := func(service string) focusActualWant {
		return focusActualWant{service: service, unit: focusUnitHours, quantity: focusActualHours}
	}
	tests := []struct {
		name string
		tags map[string]string
		want focusActualWant
	}{
		{
			name: "linux vm",
			tags: map[string]string{"sku": "Standard_D2s_v3"},
			want: hourly("Virtual Machines", 1, focusD2sv3Linux),
		},
		{
			name: "spot vm",
			tags: map[string]string{"sku": "Standard_D2s_v3", "priority": "Spot"},
			want: hourly("Virtual Machines", 1, focusD2sv3Spot),
		},
		{
			name: "classic scale set of three",
			tags: map[string]string{
				"resource_type": "azure:compute/linuxVirtualMachineScaleSet:LinuxVirtualMachineScaleSet",
				"sku":           "Standard_D2s_v3",
				"instances":     "3",
			},
			want: hourly("Virtual Machine Scale Sets", 3, focusD2sv3Linux),
		},
		{
			name: "classic windows vm with size",
			tags: map[string]string{
				"resource_type": "azure:compute/windowsVirtualMachine:WindowsVirtualMachine",
				"size":          "Standard_D2s_v3",
			},
			want: hourly("Virtual Machines", 1, focusD2sv3Windows),
		},
		{
			name: "managed disk",
			tags: map[string]string{
				"resource_type": "storage/ManagedDisk",
				"disk_type":     "Premium_SSD_LRS",
				"size_gb":       "128",
			},
			want: focusActualWant{
				service:   "Virtual Machines",
				unit:      focusUnitUnitsPerMonth,
				quantity:  focusActualHours / pluginsdk.HoursPerMonth,
				unitPrice: focusDiskP10Month,
			},
		},
		{
			name: "aks without node pools",
			tags: map[string]string{"resource_type": aksCanonicalType, "sku": "Standard"},
			want: hourly("Azure Kubernetes Service", 1, focusAKSStandard),
		},
		{
			name: "aks with a node pool",
			tags: map[string]string{
				"resource_type":     aksCanonicalType,
				"sku":               "Standard",
				"node_pool_1_sku":   "Standard_D2s_v3",
				"node_pool_1_count": "2",
			},
			want: mixed("Azure Kubernetes Service"),
		},
		{
			name: "sql database",
			tags: map[string]string{"resource_type": "sql/Database", "sku": "GP_Gen5_2", "size_gb": "100"},
			want: mixed("Azure SQL Database"),
		},
		{
			name: "functions consumption",
			tags: map[string]string{
				"resource_type": "web/FunctionApp",
				"executions":    "3000000",
				"gb_seconds":    "600000",
			},
			want: mixed("Functions"),
		},
		{
			name: "load balancer with no rules",
			tags: map[string]string{"resource_type": "network/LoadBalancer", "sku": "Standard", "rule_count": "0"},
			want: mixed("Load Balancer"),
		},
	}

	client := dialPricingClient(t, newFocusActualCalc(t))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			record := focusActualRecord(t, client, tt.tags)
			assertFocusPricingBasis(t, record, tt.want)
		})
	}
}

func TestGetActualCost_VMSizeKeys_BuildsFocusRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tags    map[string]string
		service string
		count   float64
	}{
		{
			name: "native hardwareProfile.vmSize",
			tags: map[string]string{
				"resource_type":          "azure-native:compute:VirtualMachine",
				"hardwareProfile.vmSize": "Standard_D2s_v3",
			},
			service: "Virtual Machines",
			count:   1,
		},
		{
			name: "native collapsed hardwareProfile",
			tags: map[string]string{
				"resource_type":   "azure-native:compute:VirtualMachine",
				"hardwareProfile": "Standard_D2s_v3",
			},
			service: "Virtual Machines",
			count:   1,
		},
		{
			name: "classic size",
			tags: map[string]string{
				"resource_type": "azure:compute/linuxVirtualMachine:LinuxVirtualMachine",
				"size":          "Standard_D2s_v3",
			},
			service: "Virtual Machines",
			count:   1,
		},
		{
			name:    "skuName",
			tags:    map[string]string{"resource_type": "compute/VirtualMachine", "skuName": "Standard_D2s_v3"},
			service: "Virtual Machines",
			count:   1,
		},
		{
			name: "native scale set sku.name and capacity",
			tags: map[string]string{
				"resource_type": "azure-native:compute:VirtualMachineScaleSet",
				"sku.name":      "Standard_D2s_v3",
				"sku.capacity":  "3",
			},
			service: "Virtual Machine Scale Sets",
			count:   3,
		},
	}

	client := dialPricingClient(t, newFocusActualCalc(t))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			record := focusActualRecord(t, client, tt.tags)
			assertFocusPricingBasis(t, record, focusActualWant{
				service:   tt.service,
				unit:      focusUnitHours,
				quantity:  focusActualHours * tt.count,
				unitPrice: focusD2sv3Linux,
			})
		})
	}
}

func TestBuildFocusRecord_ProductionCostPath_QuantityIsExact(t *testing.T) {
	t.Parallel()

	for _, count := range []float64{1, 3} {
		quote := focusQuote(0.0104*pluginsdk.HoursPerMonth*count, "USD", "vm detail")
		quote.meters = []quoteMeter{{key: breakdownCompute, price: 0.0104, unit: "1 Hour", count: count}}
		record, err := buildFocusRecord(
			focusVMDescriptor(nil),
			quote,
			focusWindow(focusActualHours),
			"ba-focus-test",
			"vm-1",
		)
		if err != nil {
			t.Fatalf("buildFocusRecord() failed: %v", err)
		}
		want := focusActualHours * count
		if record.GetPricingQuantity() != want || record.GetConsumedQuantity() != want {
			t.Errorf("count %v: quantities = %v and %v, want exactly %v",
				count, record.GetPricingQuantity(), record.GetConsumedQuantity(), want)
		}
	}
}

func focusActualRecord(
	t *testing.T,
	client finfocusv1.CostSourceServiceClient,
	tags map[string]string,
) *finfocusv1.FocusCostRecord {
	t.Helper()

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	all := map[string]string{"region": "eastus"}
	for key, value := range tags {
		all[key] = value
	}
	resp, err := client.GetActualCost(context.Background(), &finfocusv1.GetActualCostRequest{
		ResourceId:       "res-1",
		Tags:             all,
		Start:            timestamppb.New(start),
		End:              timestamppb.New(start.Add(time.Duration(focusActualHours) * time.Hour)),
		BillingAccountId: "ba-focus-test",
	})
	if err != nil {
		t.Fatalf("GetActualCost() failed: %v", err)
	}
	record := resp.GetResults()[0].GetFocusRecord()
	if record == nil {
		t.Fatal("FocusRecord = nil, want a record")
	}
	if err := pluginsdk.ValidateFocusRecord(record); err != nil {
		t.Fatalf("ValidateFocusRecord() = %v", err)
	}
	return record
}

func assertFocusPricingBasis(t *testing.T, record *finfocusv1.FocusCostRecord, want focusActualWant) {
	t.Helper()

	if record.GetServiceName() != want.service {
		t.Errorf("service name = %q, want %q", record.GetServiceName(), want.service)
	}
	if record.GetPricingUnit() != want.unit || record.GetConsumedUnit() != want.unit {
		t.Errorf("units = %q and %q, want %q", record.GetPricingUnit(), record.GetConsumedUnit(), want.unit)
	}
	if record.GetPricingQuantity() != want.quantity || record.GetConsumedQuantity() != want.quantity {
		t.Errorf("quantities = %v and %v, want exactly %v",
			record.GetPricingQuantity(), record.GetConsumedQuantity(), want.quantity)
	}
	if record.GetListUnitPrice() != want.unitPrice || record.GetContractedUnitPrice() != want.unitPrice {
		t.Errorf("unit prices list=%v contracted=%v, want %v",
			record.GetListUnitPrice(), record.GetContractedUnitPrice(), want.unitPrice)
	}
}

// newFocusActualCalc serves each price query the rows its service filter asks
// for, so every resource type below prices from realistic retail rows.
func newFocusActualCalc(t *testing.T) *Calculator {
	t.Helper()

	vm := loadRetailFixture(t, vmFixturePath).Items
	aks := loadRetailFixture(t, aksFixturePath).Items
	sqlCompute, sqlStorage := loadSQLFixtures(t)
	functions := loadRetailFixture(t, "testdata/retail/functions/eastus_consumption.json").Items
	loadBalancer := loadRetailFixture(t, "testdata/retail/loadbalancer/global_standard.json").Items
	disk := []azureclient.PriceItem{{
		CurrencyCode:  "USD",
		RetailPrice:   focusDiskP10Month,
		MeterName:     "P10 LRS Disk",
		UnitOfMeasure: focusDiskMonthUnit,
	}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		items := []azureclient.PriceItem{}
		switch {
		case strings.Contains(filter, "Azure Kubernetes Service"):
			items = aks
		case strings.Contains(filter, "Virtual Machines"):
			items = vm
		case strings.Contains(filter, "Managed Disks"):
			items = disk
		case strings.Contains(filter, "SQL Database"):
			items = sqlItemsForFilter(filter, sqlCompute.Items, sqlStorage.Items)
		case strings.Contains(filter, "Functions"):
			items = functions
		case strings.Contains(filter, "Load Balancer") && strings.Contains(filter, "Global"):
			items = loadBalancer
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
