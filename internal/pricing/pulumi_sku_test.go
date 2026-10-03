package pricing

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// Property values below are copied from testdata/pulumi-real/core-view.json
// and core-view-proposed.json (genuine azure 6.40.0 and azure-native 3.28.0 previews).

func TestDiskSKU_RealPulumiProperties_ResolvesDiskType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sku  string
		tags map[string]string
		want string
	}{
		{
			name: "classic storageAccountType",
			tags: map[string]string{"storageAccountType": "Premium_LRS", "diskSizeGb": "256"},
			want: "Premium_LRS",
		},
		{
			name: "native sku from core",
			sku:  "Premium_LRS",
			tags: map[string]string{"sku": "Premium_LRS", "diskSizeGB": "256"},
			want: "Premium_LRS",
		},
		{
			name: "native dotted sku.name",
			tags: map[string]string{"sku.name": "StandardSSD_ZRS"},
			want: "StandardSSD_ZRS",
		},
		{
			name: "descriptor sku wins",
			sku:  "Standard_LRS",
			tags: map[string]string{"storageAccountType": "Premium_LRS"},
			want: "Standard_LRS",
		},
		{
			name: "classic tier is the performance tier, not the disk type",
			tags: map[string]string{"tier": "P30", "storageAccountType": "Premium_LRS"},
			want: "Premium_LRS",
		},
		{
			name: "unknown reference sentinel is ignored",
			tags: map[string]string{"storageAccountType": pulumiUnknownValue, "disk_type": "Standard_LRS"},
			want: "Standard_LRS",
		},
		{
			name: "invented disk_type still works",
			tags: map[string]string{"disk_type": "Premium_SSD_LRS"},
			want: "Premium_SSD_LRS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := diskSKU(&finfocusv1.ResourceDescriptor{Sku: tt.sku, Tags: tt.tags})
			if got != tt.want {
				t.Fatalf("diskSKU() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDiskType_ARMNames_MapToPremiumSSD(t *testing.T) {
	t.Parallel()

	info, err := normalizeDiskType("Premium_LRS")
	if err != nil {
		t.Fatalf("normalizeDiskType(Premium_LRS) error = %v", err)
	}
	if info.TierPrefix != "P" || info.Redundancy != redundancyLRS {
		t.Fatalf("Premium_LRS = %+v, want P tier, LRS", info)
	}
	for _, unsupported := range []string{"UltraSSD_LRS", "PremiumV2_LRS"} {
		if _, err := normalizeDiskType(unsupported); err == nil {
			t.Fatalf("normalizeDiskType(%s) succeeded, want unsupported", unsupported)
		}
	}
}

func TestDescriptorSizeGB_NativeDiskSizeGB_IsRead(t *testing.T) {
	t.Parallel()

	got, set, err := descriptorSizeGB(&finfocusv1.ResourceDescriptor{
		Tags: map[string]string{"diskSizeGB": "256"},
	})
	if err != nil || !set || got != 256 {
		t.Fatalf("descriptorSizeGB() = %v, %v, %v; want 256, true, nil", got, set, err)
	}
}

func TestAppServicePlanSKU_RealPulumiProperties_ResolvesShortSKU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sku  string
		tags map[string]string
		want string
	}{
		{name: "classic skuName", tags: map[string]string{"skuName": "P1v3", "osType": "Linux"}, want: "P1v3"},
		{name: "native sku from core", sku: "P1v3", tags: map[string]string{"sku": "P1v3"}, want: "P1v3"},
		{name: "native dotted sku.name", tags: map[string]string{"sku.name": "S1"}, want: "S1"},
		{name: "descriptor sku wins", sku: "B1", tags: map[string]string{"skuName": "S1"}, want: "B1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := appServicePlanSKU(&finfocusv1.ResourceDescriptor{Sku: tt.sku, Tags: tt.tags})
			if got != tt.want {
				t.Fatalf("appServicePlanSKU() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppServicePlanWorkers_RealPulumiProperties_ReadsCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tags     map[string]string
		want     int
		wantNote string
		wantErr  bool
	}{
		{
			name:     "absent is one worker with a note",
			tags:     map[string]string{},
			want:     1,
			wantNote: workerCountMissingNote,
		},
		{
			name:     "unknown at preview is one worker with a note",
			tags:     map[string]string{"workerCount": pulumiUnknownValue},
			want:     1,
			wantNote: workerCountUnknownNote,
		},
		{name: "classic workerCount", tags: map[string]string{"workerCount": "2"}, want: 2},
		{name: "native sku.capacity", tags: map[string]string{"sku.capacity": "3"}, want: 3},
		{name: "workerCount wins", tags: map[string]string{"workerCount": "2", "sku.capacity": "3"}, want: 2},
		{name: "non-numeric", tags: map[string]string{"workerCount": "two"}, wantErr: true},
		{name: "zero", tags: map[string]string{"workerCount": "0"}, wantErr: true},
		{name: "negative", tags: map[string]string{"sku.capacity": "-1"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, note, err := appServicePlanWorkers(tt.tags)
			if tt.wantErr {
				if status.Code(err) != codes.InvalidArgument {
					t.Fatalf("appServicePlanWorkers() err = %v, want InvalidArgument", err)
				}
				return
			}
			if err != nil || got != tt.want || note != tt.wantNote {
				t.Fatalf("appServicePlanWorkers() = %d, %q, %v; want %d, %q", got, note, err, tt.want, tt.wantNote)
			}
		})
	}
}

func TestDescriptorWindows_ClassicOSType_SelectsProduct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tags    map[string]string
		want    bool
		wantErr bool
	}{
		{name: "osType Windows", tags: map[string]string{"osType": "Windows"}, want: true},
		{name: "osType Linux", tags: map[string]string{"osType": "Linux"}, want: false},
		{name: "legacy lowercase osType", tags: map[string]string{"osType": "linux"}, want: false},
		{name: "os tag wins", tags: map[string]string{"os": "Windows", "osType": "Linux"}, want: true},
		{name: "WindowsContainer is not priced", tags: map[string]string{"osType": "WindowsContainer"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := descriptorWindows(&finfocusv1.ResourceDescriptor{Tags: tt.tags})
			if tt.wantErr {
				if status.Code(err) != codes.InvalidArgument {
					t.Fatalf("descriptorWindows() err = %v, want InvalidArgument", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("descriptorWindows() = %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

func TestGetProjectedCost_ClassicServicePlan_PricesWorkersOnOSProduct(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, "testdata/retail/appservice/eastus_consumption.json")
	calc := newPricingCalc(t, loaded.Items)

	tests := []struct {
		name   string
		tags   map[string]string
		hourly float64
		count  float64
		detail string
	}{
		{
			name:   "linux P1v3 two workers",
			tags:   map[string]string{"skuName": "P1v3", "osType": "Linux", "workerCount": "2"},
			hourly: 0.155,
			count:  2,
			detail: "2 workers",
		},
		{
			name:   "windows S1 default one worker",
			tags:   map[string]string{"skuName": "S1", "osType": "Windows"},
			hourly: 0.1,
			count:  1,
			detail: workerCountMissingNote,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
				Resource: &finfocusv1.ResourceDescriptor{
					Provider:     "azure",
					ResourceType: "azure:appservice/servicePlan:ServicePlan",
					Region:       "eastus",
					Tags:         tt.tags,
				},
			})
			if err != nil {
				t.Fatalf("GetProjectedCost() error = %v", err)
			}
			want := tt.hourly * pluginsdk.HoursPerMonth * tt.count
			if math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
				t.Fatalf("cost = %v, want %v", resp.GetCostPerMonth(), want)
			}
			if !strings.Contains(resp.GetBillingDetail(), tt.detail) {
				t.Fatalf("billing_detail = %q, want %q", resp.GetBillingDetail(), tt.detail)
			}
		})
	}
}

func TestAKSControlPlane_RealPulumiProperties_ReadsTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sku       string
		tags      map[string]string
		wantMeter string
		wantErr   codes.Code
	}{
		{
			name:      "classic skuTier",
			tags:      map[string]string{"skuTier": "Standard", "supportPlan": "KubernetesOfficial"},
			wantMeter: aksMeterStandard,
		},
		{
			name:      "native sku.tier beats sku.name in Sku",
			sku:       "Base",
			tags:      map[string]string{"sku": "Base", "sku.name": "Base", "sku.tier": "Standard"},
			wantMeter: aksMeterStandard,
		},
		{
			name:    "classic supportPlan long term support on Free",
			tags:    map[string]string{"skuTier": "Free", "supportPlan": "AKSLongTermSupport"},
			wantErr: codes.InvalidArgument,
		},
		{
			name:      "classic default supportPlan on Free",
			tags:      map[string]string{"skuTier": "Free", "supportPlan": "KubernetesOfficial"},
			wantMeter: aksMeterFree,
		},
		{
			name:    "native supportPlan long term support on Standard is refused like Azure",
			tags:    map[string]string{"sku.tier": "Standard", "supportPlan": "AKSLongTermSupport"},
			wantErr: codes.InvalidArgument,
		},
		{
			name:      "native Premium with supportPlan long term support",
			sku:       "Base",
			tags:      map[string]string{"sku.tier": "Premium", "supportPlan": "AKSLongTermSupport"},
			wantMeter: aksMeterLTS,
		},
		{
			name:      "classic Premium without supportPlan still bills long term support",
			tags:      map[string]string{"skuTier": "Premium"},
			wantMeter: aksMeterLTS,
		},
		{
			name:      "plugin support=lts on Standard keeps long term support",
			sku:       "Standard",
			tags:      map[string]string{"support": "lts"},
			wantMeter: aksMeterLTS,
		},
		{
			name:    "native Automatic with sku.tier Standard is refused",
			sku:     "Automatic",
			tags:    map[string]string{"sku": "Automatic", "sku.name": "Automatic", "sku.tier": "Standard"},
			wantErr: codes.InvalidArgument,
		},
		{
			name:    "Automatic without a tier is refused",
			sku:     "Automatic",
			wantErr: codes.InvalidArgument,
		},
		{
			name:      "Base with the generic tier tag",
			sku:       "Base",
			tags:      map[string]string{"tier": "Standard"},
			wantMeter: aksMeterStandard,
		},
		{
			name:      "unknown Sku falls through to sku.tier",
			sku:       pulumiUnknownValue,
			tags:      map[string]string{"sku.tier": "Free"},
			wantMeter: aksMeterFree,
		},
		{
			name:    "native Base without sku.tier stays an error",
			sku:     "Base",
			tags:    map[string]string{"sku": "Base"},
			wantErr: codes.InvalidArgument,
		},
		{
			name:      "per-type tier beats generic tier tag",
			tags:      map[string]string{"skuTier": "Standard", "tier": "Free"},
			wantMeter: aksMeterStandard,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			meter, _, err := aksControlPlane(&finfocusv1.ResourceDescriptor{Sku: tt.sku, Tags: tt.tags})
			if tt.wantErr != codes.OK {
				if status.Code(err) != tt.wantErr {
					t.Fatalf("aksControlPlane() err = %v, want %s", err, tt.wantErr)
				}
				return
			}
			if err != nil || meter != tt.wantMeter {
				t.Fatalf("aksControlPlane() = %q, %v; want %q", meter, err, tt.wantMeter)
			}
		})
	}
}

func TestGetProjectedCost_AKSWithoutNodePools_SaysPoolsNotIncluded(t *testing.T) {
	t.Parallel()

	loaded := loadRetailFixture(t, aksFixturePath)
	calc, _ := newAKSPricingCalc(t, loaded.Items, nil)
	resp, err := calc.GetProjectedCost(context.Background(), aksRequest(
		"azure:containerservice/kubernetesCluster:KubernetesCluster",
		"",
		map[string]string{"skuTier": "Standard", "defaultNodePool": "system"},
	))
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if !strings.Contains(resp.GetBillingDetail(), "node pools not included") {
		t.Fatalf("billing_detail = %q, want the node pool note", resp.GetBillingDetail())
	}
}

func TestSQLDatabaseSKU_RealPulumiProperties_ComposesVCoreSKU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sku     string
		tags    map[string]string
		want    string
		wantErr bool
	}{
		{name: "classic skuName", tags: map[string]string{"skuName": "GP_Gen5_4"}, want: "GP_Gen5_4"},
		{
			name: "native core sku plus dotted capacity",
			sku:  "GP_Gen5",
			tags: map[string]string{"sku": "GP_Gen5", "sku.name": "GP_Gen5", "sku.capacity": "4"},
			want: "GP_Gen5_4",
		},
		{name: "native without capacity", sku: "GP_Gen5", tags: map[string]string{"sku": "GP_Gen5"}, wantErr: true},
		{
			name: "DTU name keeps no capacity suffix",
			sku:  "S0",
			tags: map[string]string{"sku.capacity": "10"},
			want: "S0",
		},
		{name: "negative capacity", sku: "GP_Gen5", tags: map[string]string{"sku.capacity": "-2"}, wantErr: true},
		{
			name: "dotted sku.name only",
			tags: map[string]string{"sku.name": "GP_Gen5", "sku.capacity": "2"},
			want: "GP_Gen5_2",
		},
		{
			name: "capacity already in the name",
			sku:  "GP_Gen5_8",
			tags: map[string]string{"sku.capacity": "8"},
			want: "GP_Gen5_8",
		},
		{
			name: "serverless keeps its mark",
			tags: map[string]string{"sku.name": "GP_S_Gen5", "sku.capacity": "2"},
			want: "GP_S_Gen5_2",
		},
		{name: "non-numeric capacity", sku: "GP_Gen5", tags: map[string]string{"sku.capacity": "four"}, wantErr: true},
		{name: "missing", tags: map[string]string{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := sqlDatabaseSKU(&finfocusv1.ResourceDescriptor{Sku: tt.sku, Tags: tt.tags})
			if tt.wantErr {
				if status.Code(err) != codes.InvalidArgument {
					t.Fatalf("sqlDatabaseSKU() err = %v, want InvalidArgument", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("sqlDatabaseSKU() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestGetProjectedCost_NativeSQLWithCapacity_PricesVCores(t *testing.T) {
	t.Parallel()

	compute := loadRetailFixture(t, "testdata/retail/sqldb/gp_gen5_compute_eastus.json")
	storage := loadRetailFixture(t, "testdata/retail/sqldb/gp_storage_eastus.json")
	calc := newPricingCalc(t, append(append([]azureclient.PriceItem{}, compute.Items...), storage.Items...))
	resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure-native",
			ResourceType: "azure-native:sql:Database",
			Region:       "eastus",
			Sku:          "GP_Gen5",
			Tags:         map[string]string{"sku": "GP_Gen5", "sku.capacity": "4", "size_gb": "32"},
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if !strings.Contains(resp.GetBillingDetail(), "GP_Gen5_4") {
		t.Fatalf("billing_detail = %q, want the 4 vCore SKU", resp.GetBillingDetail())
	}
}

func TestStorageAccountSKU_RealPulumiProperties_ResolvesTierAndRedundancy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sku     string
		tags    map[string]string
		want    string
		wantErr bool
	}{
		{
			name: "classic account properties",
			tags: map[string]string{"accountTier": "Standard", "accountReplicationType": "GRS", "accessTier": "Cool"},
			want: "Cool GRS",
		},
		{
			name: "classic default access tier is Hot",
			tags: map[string]string{"accountTier": "Standard", "accountReplicationType": "RAGRS"},
			want: "Hot RA-GRS",
		},
		{
			name: "native ARM sku plus accessTier",
			sku:  "Standard_GRS",
			tags: map[string]string{"sku": "Standard_GRS", "accessTier": "Cool"},
			want: "Cool GRS",
		},
		{
			name: "native ARM sku default Hot",
			sku:  "Standard_RAGZRS",
			want: "Hot RA-GZRS",
		},
		{
			name: "plugin form still works",
			sku:  "Hot LRS",
			want: "Hot LRS",
		},
		{
			name: "classic properties beat plugin tier tags",
			tags: map[string]string{
				"accountTier": "Standard", "accountReplicationType": "LRS", "accessTier": "Hot",
				"tier": "Cool", "redundancy": "GRS",
			},
			want: "Hot LRS",
		},
		{
			name: "native ARM sku honours the plugin access_tier alias",
			sku:  "Standard_GRS",
			tags: map[string]string{"access_tier": "Cool"},
			want: "Cool GRS",
		},
		{
			name: "native StorageV2 kind is priced",
			sku:  "Standard_LRS",
			tags: map[string]string{"kind": "StorageV2"},
			want: "Hot LRS",
		},
		{
			name:    "native BlobStorage kind is not General Block Blob v2",
			sku:     "Standard_LRS",
			tags:    map[string]string{"kind": "BlobStorage"},
			wantErr: true,
		},
		{
			name: "classic general purpose v1 kind is not General Block Blob v2",
			tags: map[string]string{
				"accountKind":            "Storage",
				"accountTier":            "Standard",
				"accountReplicationType": "LRS",
			},
			wantErr: true,
		},
		{
			name:    "premium performance is not General Block Blob v2",
			sku:     "Premium_LRS",
			wantErr: true,
		},
		{
			name:    "classic premium performance",
			tags:    map[string]string{"accountTier": "Premium", "accountReplicationType": "LRS"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := storageAccountSKU(&finfocusv1.ResourceDescriptor{Sku: tt.sku, Tags: tt.tags})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("storageAccountSKU() = %q, want an error", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("storageAccountSKU() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestMapDescriptorToQuery_RealPulumiProperties_AcceptsPerTypeSKU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		provider     string
		resourceType string
		sku          string
		tags         map[string]string
	}{
		{
			name: "classic managed disk", provider: "azure",
			resourceType: "azure:compute/managedDisk:ManagedDisk",
			tags:         map[string]string{"storageAccountType": "Premium_LRS", "location": "westeurope"},
		},
		{
			name: "classic service plan", provider: "azure",
			resourceType: "azure:appservice/servicePlan:ServicePlan",
			tags:         map[string]string{"skuName": "P1v3"},
		},
		{
			name: "classic aks", provider: "azure",
			resourceType: "azure:containerservice/kubernetesCluster:KubernetesCluster",
			tags:         map[string]string{"skuTier": "Standard"},
		},
		{
			name: "native aks", provider: "azure-native",
			resourceType: "azure-native:containerservice:ManagedCluster",
			sku:          "Base",
			tags:         map[string]string{"sku.tier": "Standard"},
		},
		{
			name: "classic sql", provider: "azure",
			resourceType: "azure:mssql/database:Database",
			tags:         map[string]string{"skuName": "GP_Gen5_4"},
		},
		{
			name: "classic storage", provider: "azure",
			resourceType: "azure:storage/account:Account",
			tags:         map[string]string{"accountTier": "Standard", "accountReplicationType": "GRS"},
		},
		{
			name: "native storage", provider: "azure-native",
			resourceType: "azure-native:storage:StorageAccount",
			sku:          "Standard_GRS",
			tags:         map[string]string{"accessTier": "Cool"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := MapDescriptorToQuery(&finfocusv1.ResourceDescriptor{
				Provider:     tt.provider,
				ResourceType: tt.resourceType,
				Region:       "westeurope",
				Sku:          tt.sku,
				Tags:         tt.tags,
			})
			if err != nil {
				t.Fatalf("MapDescriptorToQuery() error = %v", err)
			}
		})
	}
}

func TestDiskBillingTier_PerformanceTier_BillsTheHigherTier(t *testing.T) {
	t.Parallel()

	premium, err := normalizeDiskType("Premium_LRS")
	if err != nil {
		t.Fatalf("normalizeDiskType() error = %v", err)
	}
	standardSSD, err := normalizeDiskType("StandardSSD_LRS")
	if err != nil {
		t.Fatalf("normalizeDiskType() error = %v", err)
	}
	tests := []struct {
		name     string
		info     diskTypeInfo
		sizeTier string
		tags     map[string]string
		want     string
		wantErr  bool
	}{
		{name: "no performance tier", info: premium, sizeTier: "P15", want: "P15"},
		{
			name:     "higher performance tier wins",
			info:     premium,
			sizeTier: "P15",
			tags:     map[string]string{"tier": "P30"},
			want:     "P30",
		},
		{
			name:     "lower performance tier is ignored",
			info:     premium,
			sizeTier: "P15",
			tags:     map[string]string{"tier": "p10"},
			want:     "P15",
		},
		{
			name:     "unknown at preview is ignored",
			info:     premium,
			sizeTier: "P15",
			tags:     map[string]string{"tier": pulumiUnknownValue},
			want:     "P15",
		},
		{
			name:     "not a premium tier name",
			info:     premium,
			sizeTier: "P15",
			tags:     map[string]string{"tier": "Fast"},
			wantErr:  true,
		},
		{
			name:     "premium tier number that does not exist",
			info:     premium,
			sizeTier: "P15",
			tags:     map[string]string{"tier": "P31"},
			wantErr:  true,
		},
		{
			name:     "standard SSD has no performance tier",
			info:     standardSSD,
			sizeTier: "E15",
			tags:     map[string]string{"tier": "P30"},
			want:     "E15",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := diskBillingTier(tt.info, tt.sizeTier, tt.tags)
			if tt.wantErr {
				if status.Code(err) != codes.InvalidArgument {
					t.Fatalf("diskBillingTier() err = %v, want InvalidArgument", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("diskBillingTier() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestGetProjectedCost_ClassicDiskWithPerformanceTier_PricesThatTier(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, []azureclient.PriceItem{
		{
			ProductName: "Premium SSD Managed Disks", SkuName: "P15 LRS", MeterName: "P15 LRS Disk",
			RetailPrice: 38.01, CurrencyCode: "USD", UnitOfMeasure: "1/Month",
		},
		{
			ProductName: "Premium SSD Managed Disks", SkuName: "P30 LRS", MeterName: "P30 LRS Disk",
			RetailPrice: 135.17, CurrencyCode: "USD", UnitOfMeasure: "1/Month",
		},
	})
	resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "azure:compute/managedDisk:ManagedDisk",
			Region:       "eastus",
			Tags:         map[string]string{"storageAccountType": "Premium_LRS", "diskSizeGb": "256", "tier": "P30"},
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if resp.GetCostPerMonth() != 135.17 || !strings.Contains(resp.GetBillingDetail(), "P30") {
		t.Fatalf("cost = %v, detail = %q; want the P30 price", resp.GetCostPerMonth(), resp.GetBillingDetail())
	}
}

func TestGetProjectedCost_NativeSQLWithoutCapacity_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, nil)
	_, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure-native",
			ResourceType: "azure-native:sql:Database",
			Region:       "eastus",
			Sku:          "GP_Gen5",
			Tags:         map[string]string{"sku": "GP_Gen5", "size_gb": "32"},
		},
	})
	if status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), skuCapacityTag) {
		t.Fatalf("GetProjectedCost() err = %v, want InvalidArgument naming %s", err, skuCapacityTag)
	}
}

func TestSQLRequestFrom_PulumiZoneRedundant_EnablesZoneRedundancy(t *testing.T) {
	t.Parallel()

	spec, err := sqlRequestFrom(&finfocusv1.ResourceDescriptor{
		Region: "eastus",
		Tags:   map[string]string{"skuName": "GP_Gen5_2", "zoneRedundant": "true", "size_gb": "32"},
	})
	if err != nil || !spec.zone {
		t.Fatalf("sqlRequestFrom() = %+v, %v; want zone redundancy", spec, err)
	}
}

func TestMapDescriptorToQuery_UnpriceablePulumiInput_ReturnsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		provider     string
		resourceType string
		sku          string
		tags         map[string]string
	}{
		{
			name: "native aks Base without a tier", provider: "azure-native",
			resourceType: "azure-native:containerservice:ManagedCluster",
			sku:          "Base",
			tags:         map[string]string{"sku": "Base"},
		},
		{
			name: "native aks Automatic", provider: "azure-native",
			resourceType: "azure-native:containerservice:ManagedCluster",
			sku:          "Automatic",
			tags:         map[string]string{"sku.tier": "Standard"},
		},
		{
			name: "native sql without vCore count", provider: "azure-native",
			resourceType: "azure-native:sql:Database",
			sku:          "GP_Gen5",
		},
		{
			name: "classic plan for Windows containers", provider: "azure",
			resourceType: "azure:appservice/servicePlan:ServicePlan",
			tags:         map[string]string{"skuName": "P1v3", "osType": "WindowsContainer"},
		},
		{
			name: "classic plan with zero workers", provider: "azure",
			resourceType: "azure:appservice/servicePlan:ServicePlan",
			tags:         map[string]string{"skuName": "P1v3", "workerCount": "0"},
		},
		{
			name: "native BlobStorage account", provider: "azure-native",
			resourceType: "azure-native:storage:StorageAccount",
			sku:          "Standard_LRS",
			tags:         map[string]string{"kind": "BlobStorage"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := MapDescriptorToQuery(&finfocusv1.ResourceDescriptor{
				Provider:     tt.provider,
				ResourceType: tt.resourceType,
				Region:       "westeurope",
				Sku:          tt.sku,
				Tags:         tt.tags,
			})
			if err == nil {
				t.Fatal("MapDescriptorToQuery() succeeded, want an error")
			}
		})
	}
}
