package pricing

import (
	"context"
	"math"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// Real Pulumi tokens. Classic uses provider azure. Azure Native uses provider
// azure-native, which finfocus v0.4.0 and older copy from the token prefix;
// newer releases send azure (see provider_test.go).
func TestSupportsRealPulumiTokens(t *testing.T) {
	t.Parallel()

	calc := NewCalculator(zerolog.Nop())
	tests := []struct {
		name      string
		provider  string
		typ       string
		sku       string
		tags      map[string]string
		want      bool
		wantKind  string
		wantError codes.Code
	}{
		{
			name:     "classic linux vm",
			provider: "azure",
			typ:      "azure:compute/linuxVirtualMachine:LinuxVirtualMachine",
			sku:      "Standard_B1s",
			want:     true,
			wantKind: kindVM,
		},
		{
			name:     "azure native vm",
			provider: "azure-native",
			typ:      "azure-native:compute:VirtualMachine",
			sku:      "Standard_B1s",
			want:     true,
			wantKind: kindVM,
		},
		{
			name:     "windows virtual machine",
			provider: "azure",
			typ:      "azure:compute/windowsVirtualMachine:WindowsVirtualMachine",
			sku:      "Standard_D2s_v3",
			want:     true,
			wantKind: kindVM,
		},
		{
			name:     "classic managed disk",
			provider: "azure",
			typ:      "azure:compute/managedDisk:ManagedDisk",
			sku:      "Premium_SSD_LRS",
			want:     true,
			wantKind: kindDisk,
		},
		{
			name:     "azure native disk",
			provider: "azure-native",
			typ:      "azure-native:compute:Disk",
			sku:      "Premium_SSD_LRS",
			want:     true,
			wantKind: kindDisk,
		},
		{
			name:     "classic blob",
			provider: "azure",
			typ:      "azure:storage/blob:Blob",
			sku:      "Hot LRS",
			want:     true,
			wantKind: kindBlob,
		},
		{
			name:     "azure native blob",
			provider: "azure-native",
			typ:      "azure-native:storage:Blob",
			sku:      "Hot LRS",
			want:     true,
			wantKind: kindBlob,
		},
		{
			name:     "classic storage account",
			provider: "azure",
			typ:      "azure:storage/account:Account",
			sku:      "Hot LRS",
			want:     true,
			wantKind: kindStorageAccount,
		},
		{
			name:     "azure native storage account",
			provider: "azure-native",
			typ:      "azure-native:storage:StorageAccount",
			sku:      "Hot LRS",
			want:     true,
			wantKind: kindStorageAccount,
		},
		{
			name:     "classic app service plan",
			provider: "azure",
			typ:      "azure:appservice/servicePlan:ServicePlan",
			sku:      "P1v3",
			want:     true,
			wantKind: kindAppServicePlan,
		},
		{
			name:     "azure native app service plan",
			provider: "azure-native",
			typ:      "azure-native:web:AppServicePlan",
			sku:      "P1v3",
			want:     true,
			wantKind: kindAppServicePlan,
		},
		{
			name:     "classic function app",
			provider: "azure",
			typ:      "azure:appservice/functionApp:FunctionApp",
			want:     true,
			wantKind: kindFunctionApp,
		},
		{
			name:     "azure native function app",
			provider: "azure-native",
			typ:      "azure-native:web:WebApp",
			tags:     map[string]string{"kind": "FunctionApp"},
			want:     true,
			wantKind: kindFunctionApp,
		},
		{
			name:      "azure native web app is not a function",
			provider:  "azure-native",
			typ:       "azure-native:web:WebApp",
			want:      false,
			wantError: codes.Unimplemented,
		},
		{
			name:     "classic aks",
			provider: "azure",
			typ:      "azure:containerservice/kubernetesCluster:KubernetesCluster",
			sku:      "Standard",
			want:     true,
			wantKind: kindAKS,
		},
		{
			name:     "azure native aks",
			provider: "azure-native",
			typ:      "azure-native:containerservice:ManagedCluster",
			sku:      "Standard",
			want:     true,
			wantKind: kindAKS,
		},
		{
			name:     "classic sql",
			provider: "azure",
			typ:      "azure:mssql/database:Database",
			sku:      "GP_Gen5_2",
			want:     true,
			wantKind: kindSQLDatabase,
		},
		{
			name:     "azure native sql",
			provider: "azure-native",
			typ:      "azure-native:sql:Database",
			sku:      "GP_Gen5_2",
			want:     true,
			wantKind: kindSQLDatabase,
		},
		{
			name:     "classic cosmos",
			provider: "azure",
			typ:      "azure:cosmosdb/account:Account",
			want:     true,
			wantKind: kindCosmosDB,
		},
		{
			name:     "azure native cosmos",
			provider: "azure-native",
			typ:      "azure-native:documentdb:DatabaseAccount",
			want:     true,
			wantKind: kindCosmosDB,
		},
		{
			name:     "classic load balancer",
			provider: "azure",
			typ:      "azure:lb/loadBalancer:LoadBalancer",
			want:     true,
			wantKind: kindLoadBalancer,
		},
		{
			name:     "azure native load balancer",
			provider: "azure-native",
			typ:      "azure-native:network:LoadBalancer",
			want:     true,
			wantKind: kindLoadBalancer,
		},
		{
			name:      "aws stays rejected",
			provider:  "aws",
			typ:       "compute/VirtualMachine",
			sku:       "Standard_B1s",
			want:      false,
			wantError: codes.InvalidArgument,
		},
		{
			name:     "native scale set",
			provider: "azure-native",
			typ:      "azure-native:compute:VirtualMachineScaleSet",
			sku:      "Standard_B1s",
			want:     true,
			wantKind: kindVM,
		},
		{
			name:      "network cloud vm is not compute",
			provider:  "azure-native",
			typ:       "azure-native:networkcloud:VirtualMachine",
			sku:       "Standard_B1s",
			want:      false,
			wantError: codes.Unimplemented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			desc := &finfocusv1.ResourceDescriptor{
				Provider:     tt.provider,
				ResourceType: tt.typ,
				Region:       "eastus",
				Sku:          tt.sku,
				Tags:         tt.tags,
			}
			resp, err := calc.Supports(context.Background(), &finfocusv1.SupportsRequest{Resource: desc})
			if err != nil {
				t.Fatalf("Supports() error: %v", err)
			}
			if resp.GetSupported() != tt.want {
				t.Fatalf("supported = %v, reason = %s", resp.GetSupported(), resp.GetReason())
			}
			kind, classErr := classifyResource(desc)
			if tt.want {
				if classErr != nil {
					t.Fatalf("classifyResource() error: %v", classErr)
				}
				if kind != tt.wantKind {
					t.Fatalf("kind = %s, want %s", kind, tt.wantKind)
				}
				return
			}
			if status.Code(classErr) != tt.wantError {
				t.Fatalf("classify code = %s, want %s (%v)", status.Code(classErr), tt.wantError, classErr)
			}
		})
	}
}

func TestAzureNativeVMQuoteMatchesAzure(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
		ServiceName:   "Virtual Machines",
		ProductName:   "Virtual Machines BS Series",
		SkuName:       "B1s",
		MeterName:     "B1s",
		CurrencyCode:  "USD",
		RetailPrice:   0.0104,
		UnitOfMeasure: "1 Hour",
	}})
	resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure-native",
			ResourceType: "azure-native:compute:VirtualMachine",
			Region:       "eastus",
			Sku:          "Standard_B1s",
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() failed: %v", err)
	}
	wantMonthly := 0.0104 * pluginsdk.HoursPerMonth
	if math.Abs(resp.GetCostPerMonth()-wantMonthly) > 1e-9 {
		t.Fatalf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), wantMonthly)
	}
}

func TestWindowsVMQuoteIsNotLinuxPrice(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_D2s_v3",
			ProductName:   "Virtual Machines Dsv3 Series",
			SkuName:       "D2s v3",
			MeterName:     "D2s v3",
			CurrencyCode:  "USD",
			RetailPrice:   0.096,
			UnitOfMeasure: "1 Hour",
		},
		{
			ArmRegionName: "eastus",
			ArmSkuName:    "Standard_D2s_v3",
			ProductName:   "Virtual Machines Dsv3 Series Windows",
			SkuName:       "D2s v3",
			MeterName:     "D2s v3",
			CurrencyCode:  "USD",
			RetailPrice:   0.188,
			UnitOfMeasure: "1 Hour",
		},
	})
	resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "azure:compute/windowsVirtualMachine:WindowsVirtualMachine",
			Region:       "eastus",
			Sku:          "Standard_D2s_v3",
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if resp.GetCostPerMonth() != 0.188*730 {
		t.Fatalf("cost = %v, want the Windows meter", resp.GetCostPerMonth())
	}
	estimated, err := calc.EstimateCost(context.Background(), newEstimateCostRequest(t,
		"azure:compute/windowsVirtualMachine:WindowsVirtualMachine",
		map[string]any{
			"location": "eastus",
			"vmSize":   "Standard_D2s_v3",
		},
	))
	if err != nil {
		t.Fatalf("EstimateCost() error = %v", err)
	}
	if estimated.GetCostMonthly() != 0.188*730 {
		t.Fatalf("EstimateCost cost = %v, want the Windows meter", estimated.GetCostMonthly())
	}
}

func TestEstimateCostNativeFunctionWebApp(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, functionsFixturePath)
	execItem := fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Total Executions", "10")
	gbItem := fixturePositiveMeter(t, loaded.Items, "Functions", "Standard Execution Time", "1 GB Second")
	executions := testFreeExecutions + testExecutionsPerPrice
	gbSeconds := testFreeGBSeconds + 1
	want := (float64(executions-testFreeExecutions) / testExecutionsPerPrice * execItem.RetailPrice) +
		float64(gbSeconds-testFreeGBSeconds)*gbItem.RetailPrice
	calc := newPricingCalc(t, loaded.Items)
	resp, err := calc.EstimateCost(context.Background(), newEstimateCostRequest(t,
		"azure-native:web:WebApp",
		map[string]any{
			"location":   "eastus",
			"kind":       "FunctionApp",
			"executions": strconv.Itoa(executions),
			"gb_seconds": strconv.Itoa(gbSeconds),
		},
	))
	if err != nil {
		t.Fatalf("EstimateCost() failed: %v", err)
	}
	if math.Abs(resp.GetCostMonthly()-want) > 1e-9 {
		t.Fatalf("cost_monthly = %v, want %v", resp.GetCostMonthly(), want)
	}
}
