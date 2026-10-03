package pricing

import (
	"context"
	"math"
	"strings"
	"testing"

	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func TestPriorityRegularIsOnDemand(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		vmRow("Standard_B2s", "Virtual Machines BS Series", "B2s", 0.01),
		vmRow("Standard_B2s", "Virtual Machines BS Series", "B2s Spot", 0.048),
	})
	resp, err := calc.GetProjectedCost(context.Background(), vmRequest(
		"azure:compute/linuxVirtualMachine:LinuxVirtualMachine",
		"Standard_B2s",
		map[string]string{"priority": "Regular"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if resp.GetCostPerMonth() != 0.01*730 {
		t.Fatalf("cost = %v, want the on-demand row", resp.GetCostPerMonth())
	}
}

func TestPriorityLowStaysInvalid(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		vmRow("Standard_B2s", "Virtual Machines BS Series", "B2s", 0.048),
	})
	_, err := calc.GetProjectedCost(context.Background(), vmRequest(
		"azure:compute/linuxVirtualMachine:LinuxVirtualMachine",
		"Standard_B2s",
		map[string]string{"priority": "Low"},
	))
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}

func TestClassicVMReadsSizeTag(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		vmRow("Standard_D4s_v5", "Virtual Machines Dsv5 Series", "D4s v5 Spot", 0.042504),
	})
	resp, err := calc.GetProjectedCost(context.Background(), vmRequest(
		"azure:compute/linuxVirtualMachine:LinuxVirtualMachine",
		"",
		map[string]string{"size": "Standard_D4s_v5", "priority": "Spot"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if resp.GetCostPerMonth() != 0.042504*730 {
		t.Fatalf("cost = %v, want the size tag meter", resp.GetCostPerMonth())
	}
}

func TestNativeVMReadsHardwareProfile(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		vmRow("Standard_B2s", "Virtual Machines BS Series", "B2s", 0.048),
	})
	resp, err := calc.GetProjectedCost(context.Background(), vmRequest(
		"azure-native:compute:VirtualMachine",
		"",
		map[string]string{"hardwareProfile": "Standard_B2s"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if resp.GetCostPerMonth() != 0.048*730 {
		t.Fatalf("cost = %v, want the hardwareProfile size", resp.GetCostPerMonth())
	}
}

func TestDottedHardwareProfileVMSize(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		vmRow("Standard_D4s_v5", "Virtual Machines Dsv5 Series", "D4s v5 Spot", 0.042504),
	})
	resp, err := calc.GetProjectedCost(context.Background(), vmRequest(
		"azure-native:compute:VirtualMachine",
		"",
		map[string]string{
			"hardwareProfile.vmSize": "Standard_D4s_v5",
			"priority":               "Spot",
		},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if resp.GetCostPerMonth() != 0.042504*730 {
		t.Fatalf("cost = %v, want hardwareProfile.vmSize", resp.GetCostPerMonth())
	}
}

func TestHybridBenefitUsesLinuxRateAndNote(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, linuxAndWindowsVMRows())
	resp, err := calc.GetProjectedCost(context.Background(), vmRequest(
		"azure-native:compute:VirtualMachine",
		"Standard_D4s_v5",
		map[string]string{
			"licenseType": "Windows_Server",
			"osProfile":   "map[windowsConfiguration:map[provisionVMAgent:true]]",
		},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if resp.GetCostPerMonth() != 0.23*730 {
		t.Fatalf("cost = %v, want the Linux rate", resp.GetCostPerMonth())
	}
	const note = "licenseType Windows_Server is Azure Hybrid Benefit: the licence is already paid, " +
		"so the compute rate is the base (Linux) rate. Confidence Medium " +
		"(Azure documentation, not verified against the Calculator)"
	if !strings.Contains(resp.GetBillingDetail(), note) {
		t.Fatalf("billing_detail = %q, want the Hybrid Benefit note", resp.GetBillingDetail())
	}
}

func TestWindowsWithoutLicenseDoesNotUseLinuxMeter(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		vmRow("Standard_D4s_v5", "Virtual Machines Dsv5 Series", "D4s v5", 0.23),
	})
	_, err := calc.GetProjectedCost(context.Background(), vmRequest(
		"azure-native:compute:VirtualMachine",
		"Standard_D4s_v5",
		map[string]string{
			"osProfile.windowsConfiguration.provisionVMAgent": "true",
		},
	))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound (%v)", status.Code(err), err)
	}
}

func TestScaleSetMultipliesInstances(t *testing.T) {
	t.Parallel()

	row := vmRow("Standard_D2s_v5", "Virtual Machines Dsv5 Series", "D2s v5", 0.115)
	tests := []struct {
		name string
		typ  string
		tags map[string]string
	}{
		{
			name: "classic instances",
			typ:  "azure:compute/linuxVirtualMachineScaleSet:LinuxVirtualMachineScaleSet",
			tags: map[string]string{"instances": "3", "priority": "Regular"},
		},
		{
			name: "native capacity",
			typ:  "azure-native:compute:VirtualMachineScaleSet",
			tags: map[string]string{"sku.capacity": "3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calc := newPricingCalc(t, []azureclient.PriceItem{row})
			resp, err := calc.GetProjectedCost(context.Background(), vmRequest(
				tt.typ, "Standard_D2s_v5", tt.tags,
			))
			if err != nil {
				t.Fatalf("GetProjectedCost() error = %v", err)
			}
			want := 0.115 * 730 * 3
			if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
				t.Fatalf("cost = %v, want %v", resp.GetCostPerMonth(), want)
			}
		})
	}
}

func TestScaleSetNestedPrioritySelectsSpot(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		vmRow("Standard_D2s_v5", "Virtual Machines Dsv5 Series", "D2s v5", 0.115),
		vmRow("Standard_D2s_v5", "Virtual Machines Dsv5 Series", "D2s v5 Spot", 0.02),
	})
	resp, err := calc.GetProjectedCost(context.Background(), vmRequest(
		"azure-native:compute:VirtualMachineScaleSet",
		"Standard_D2s_v5",
		map[string]string{
			"sku.capacity":                   "3",
			"virtualMachineProfile.priority": "Spot",
		},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	want := 0.02 * 730 * 3
	if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", resp.GetCostPerMonth(), want)
	}
}

func vmRequest(resourceType, sku string, tags map[string]string) *finfocusv1.GetProjectedCostRequest {
	return &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: resourceType,
			Region:       "westeurope",
			Sku:          sku,
			Tags:         tags,
		},
	}
}

func vmRow(armSku, product, meter string, price float64) azureclient.PriceItem {
	return azureclient.PriceItem{
		ArmRegionName: "westeurope",
		ArmSkuName:    armSku,
		ProductName:   product,
		SkuName:       meter,
		MeterName:     meter,
		CurrencyCode:  "USD",
		RetailPrice:   price,
		UnitOfMeasure: "1 Hour",
		ServiceName:   "Virtual Machines",
	}
}

func linuxAndWindowsVMRows() []azureclient.PriceItem {
	return []azureclient.PriceItem{
		vmRow("Standard_D4s_v5", "Virtual Machines Dsv5 Series", "D4s v5", 0.23),
		vmRow("Standard_D4s_v5", "Virtual Machines Dsv5 Series Windows", "D4s v5", 0.414),
	}
}
