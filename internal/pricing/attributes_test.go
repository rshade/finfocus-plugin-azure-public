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
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func mustAttributes(t *testing.T, fields map[string]any) *structpb.Struct {
	t.Helper()
	attrs, err := structpb.NewStruct(fields)
	if err != nil {
		t.Fatalf("structpb.NewStruct() error = %v", err)
	}
	return attrs
}

func TestAttributeTags_Flatten_MatchesCoreTagFormat(t *testing.T) {
	t.Parallel()

	attrs := mustAttributes(t, map[string]any{
		"sku":             map[string]any{"name": "GP_Gen5", "capacity": 4},
		"hardwareProfile": map[string]any{"vmSize": "Standard_B2s"},
		"zoneRedundant":   true,
		"ratio":           0.5,
		"zones":           []any{"1", "2"},
		"deep": map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{
			"d": map[string]any{"e": map[string]any{"f": map[string]any{"g": "leaf"}}},
		}}}},
		"__internal":  map[string]any{"x": "hidden"},
		"placeholder": map[string]any{"v": pulumiUnknownValue},
	})
	got := attributeTags(attrs)

	want := map[string]string{
		"sku":                    "GP_Gen5",
		"sku.name":               "GP_Gen5",
		"sku.capacity":           "4",
		"hardwareProfile":        "Standard_B2s",
		"hardwareProfile.vmSize": "Standard_B2s",
		"zoneRedundant":          "true",
		"ratio":                  "0.5",
		"zones":                  "1,2",
		"zones.0":                "1",
		"zones.1":                "2",
		"deep.a.b.c.d.e.f.g":     "leaf",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("tag %q = %q, want %q", key, got[key], value)
		}
	}
	for _, key := range []string{"__internal.x", "placeholder.v"} {
		if _, ok := got[key]; ok {
			t.Errorf("tag %q is present, want it skipped", key)
		}
	}
}

func TestWithAttributeTags_NoAttributes_ReturnsSameDescriptor(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{Tags: map[string]string{"sku.capacity": "3"}}
	got, err := withAttributeTags(desc)
	if err != nil {
		t.Fatalf("withAttributeTags() error = %v", err)
	}
	if got != desc {
		t.Fatal("withAttributeTags() copied a descriptor that has no attributes")
	}
}

func TestWithAttributeTags_Conflict_AttributesWin(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Tags:       map[string]string{"sku.capacity": "2", "size_gb": "32"},
		Attributes: mustAttributes(t, map[string]any{"sku": map[string]any{"capacity": 5}}),
	}
	got, err := withAttributeTags(desc)
	if err != nil {
		t.Fatalf("withAttributeTags() error = %v", err)
	}
	if got.GetTags()["sku.capacity"] != "5" || got.GetTags()["size_gb"] != "32" {
		t.Fatalf("tags = %v, want attributes over tags and tag-only keys kept", got.GetTags())
	}
	if desc.GetTags()["sku.capacity"] != "2" {
		t.Fatal("withAttributeTags() modified the request descriptor")
	}
}

func TestWithAttributeTags_OverSizeLimit_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Attributes: mustAttributes(t, map[string]any{"blob": strings.Repeat("x", pluginsdk.MaxAttributesBytes)}),
	}
	_, err := withAttributeTags(desc)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}

func TestGetProjectedCost_NativeScaleSetCapacityInAttributes_PricesInstances(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags map[string]string
		want float64
	}{
		{name: "attributes only", want: 3},
		{name: "attributes over a stale tag", tags: map[string]string{"sku.capacity": "2"}, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calc := newPricingCalc(t, []azureclient.PriceItem{
				vmRow("Standard_D2s_v5", "Virtual Machines Dsv5 Series", "D2s v5", 0.115),
			})
			req := vmRequest("azure-native:compute:VirtualMachineScaleSet", "Standard_D2s_v5", tt.tags)
			req.Resource.Attributes = mustAttributes(t, map[string]any{
				"sku": map[string]any{"name": "Standard_D2s_v5", "capacity": 3},
			})
			resp, err := calc.GetProjectedCost(context.Background(), req)
			if err != nil {
				t.Fatalf("GetProjectedCost() error = %v", err)
			}
			if want := 0.115 * 730 * tt.want; math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
				t.Fatalf("cost = %v, want %v", resp.GetCostPerMonth(), want)
			}
		})
	}
}

func TestGetProjectedCost_NativeScaleSetUnknownCapacity_FallsBackToTag(t *testing.T) {
	t.Parallel()

	calc, filters := newCapturingPricingCalc(t, []azureclient.PriceItem{
		vmRow("Standard_D2s_v5", "Virtual Machines Dsv5 Series", "D2s v5", 0.115),
	})
	req := vmRequest("azure-native:compute:VirtualMachineScaleSet", "",
		map[string]string{"sku": "Standard_D2s_v5", "sku.capacity": "2"})
	req.Resource.Attributes = mustAttributes(t, map[string]any{
		"sku": map[string]any{"capacity": pulumiUnknownValue},
	})
	resp, err := calc.GetProjectedCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if want := 0.115 * 730 * 2; math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", resp.GetCostPerMonth(), want)
	}
	for _, filter := range filters() {
		if strings.Contains(filter, pulumiUnknownValue) || !strings.Contains(filter, "Standard_D2s_v5") {
			t.Fatalf("filter %q, want the sku tag Standard_D2s_v5", filter)
		}
	}
}

func TestGetProjectedCost_NativeSQLSkuInAttributes_PricesVCores(t *testing.T) {
	t.Parallel()

	compute := loadRetailFixture(t, "testdata/retail/sqldb/gp_gen5_compute_eastus.json")
	storage := loadRetailFixture(t, "testdata/retail/sqldb/gp_storage_eastus.json")
	calc := newPricingCalc(t, append(append([]azureclient.PriceItem{}, compute.Items...), storage.Items...))
	resp, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "azure-native:sql:Database",
			Region:       "eastus",
			Tags:         map[string]string{"size_gb": "32"},
			Attributes: mustAttributes(t, map[string]any{
				"sku": map[string]any{"name": "GP_Gen5", "capacity": 4},
			}),
		},
	})
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if !strings.Contains(resp.GetBillingDetail(), "GP_Gen5_4") {
		t.Fatalf("billing_detail = %q, want the 4 vCore SKU", resp.GetBillingDetail())
	}
}

func TestGetProjectedCost_NativeVMProfilesInAttributes_SelectWindowsSize(t *testing.T) {
	t.Parallel()

	calc := newPricingCalc(t, linuxAndWindowsVMRows())
	req := vmRequest("azure-native:compute:VirtualMachine", "", nil)
	req.Resource.Attributes = mustAttributes(t, map[string]any{
		"hardwareProfile": map[string]any{"vmSize": "Standard_D4s_v5"},
		"osProfile": map[string]any{
			"computerName":         "vm1",
			"windowsConfiguration": map[string]any{"provisionVMAgent": true},
		},
	})
	resp, err := calc.GetProjectedCost(context.Background(), req)
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	if want := 0.414 * 730; math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
		t.Fatalf("cost = %v, want the Windows D4s v5 rate %v", resp.GetCostPerMonth(), want)
	}
}

func TestSupportsAndDryRun_SkuOnlyInAttributes_AreSupported(t *testing.T) {
	t.Parallel()

	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "azure-native:sql:Database",
		Region:       "eastus",
		Attributes: mustAttributes(t, map[string]any{
			"sku": map[string]any{"name": "GP_Gen5", "capacity": 4},
		}),
	}
	calc := newPricingCalc(t, nil)

	supports, err := calc.Supports(context.Background(), &finfocusv1.SupportsRequest{Resource: desc})
	if err != nil || !supports.GetSupported() {
		t.Fatalf("Supports() = %v, %v; want supported", supports, err)
	}
	dry, err := calc.DryRun(context.Background(), &finfocusv1.DryRunRequest{Resource: desc})
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}
	if !dry.GetConfigurationValid() {
		t.Fatalf("DryRun() configuration_errors = %v, want valid", dry.GetConfigurationErrors())
	}
}

func TestGetPricingSpec_NativeSQLSkuInAttributes_ReturnsSpec(t *testing.T) {
	t.Parallel()

	compute := loadRetailFixture(t, "testdata/retail/sqldb/gp_gen5_compute_eastus.json")
	storage := loadRetailFixture(t, "testdata/retail/sqldb/gp_storage_eastus.json")
	calc := newPricingCalc(t, append(append([]azureclient.PriceItem{}, compute.Items...), storage.Items...))
	resp, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "azure-native:sql:Database",
			Region:       "eastus",
			Tags:         map[string]string{"size_gb": "32"},
			Attributes: mustAttributes(t, map[string]any{
				"sku": map[string]any{"name": "GP_Gen5", "capacity": 4},
			}),
		},
	})
	if err != nil {
		t.Fatalf("GetPricingSpec() error = %v", err)
	}
	if !strings.Contains(resp.GetSpec().GetDescription(), "GP_Gen5_4") {
		t.Fatalf("spec description = %q, want the 4 vCore SKU", resp.GetSpec().GetDescription())
	}
}

func TestAttributeTags_UnknownPlaceholder_SkipsEveryValueContainingIt(t *testing.T) {
	t.Parallel()

	got := attributeTags(mustAttributes(t, map[string]any{
		"location":        pulumiUnknownValue,
		"hardwareProfile": map[string]any{"vmSize": pulumiUnknownValue},
		"zones":           []any{"1", pulumiUnknownValue},
		"note":            "prefix-" + pulumiUnknownValue,
	}))
	for _, key := range []string{"location", "hardwareProfile", "hardwareProfile.vmSize", "zones", "zones.1", "note"} {
		if value, ok := got[key]; ok {
			t.Errorf("tag %q = %q, want it skipped", key, value)
		}
	}
	if got["zones.0"] != "1" {
		t.Errorf("tag zones.0 = %q, want 1", got["zones.0"])
	}
}

func TestAttributeTags_ValueShapes_MatchCoreConvertValueToString(t *testing.T) {
	t.Parallel()

	got := attributeTags(mustAttributes(t, map[string]any{
		"null":    nil,
		"empty":   map[string]any{},
		"none":    []any{},
		"single":  []any{"a"},
		"objects": []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}},
		"big":     1e21,
	}))
	want := map[string]string{
		"null":           "",
		"empty":          "map[]",
		"none":           "",
		"single":         "a",
		"single.0":       "a",
		"objects":        "a,b",
		"objects.0.name": "a",
		"objects.1.name": "b",
		"big":            "1000000000000000000000",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("tag %q = %q, want %q", key, got[key], value)
		}
	}
}

func TestAttributeTags_DeepAndWide_StaysBounded(t *testing.T) {
	t.Parallel()

	deep := map[string]any{"leaf": "x"}
	for range 200 {
		deep = map[string]any{"n": deep}
	}
	wide := make([]any, 5000)
	for i := range wide {
		wide[i] = "v"
	}
	got := attributeTags(mustAttributes(t, map[string]any{
		"deep": deep,
		"sku":  map[string]any{"capacity": 3},
		"wide": wide,
	}))
	if len(got) > maxAttributeTags {
		t.Fatalf("got %d tags, want at most %d", len(got), maxAttributeTags)
	}
	for key := range got {
		if segments := strings.Count(key, ".") + 1; segments > maxAttributeDepth {
			t.Fatalf("key with %d segments, want at most %d", segments, maxAttributeDepth)
		}
	}
	if got["sku.capacity"] != "3" {
		t.Fatalf("sku.capacity = %q, want the shallow key kept", got["sku.capacity"])
	}
}

func TestGetProjectedCost_UnknownAttributes_MatchTagOnlyResults(t *testing.T) {
	t.Parallel()

	t.Run("location", func(t *testing.T) {
		t.Parallel()
		calc, filters := newCapturingPricingCalc(t, nil)
		_, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
			Resource: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "azure-native:compute:VirtualMachine",
				Attributes: mustAttributes(t, map[string]any{
					"location":        pulumiUnknownValue,
					"hardwareProfile": map[string]any{"vmSize": "Standard_B2s"},
				}),
			},
		})
		if status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), "region") {
			t.Fatalf("err = %v, want InvalidArgument naming region", err)
		}
		if got := filters(); len(got) != 0 {
			t.Fatalf("Azure was called %d times, want none", len(got))
		}
	})
	t.Run("vmSize", func(t *testing.T) {
		t.Parallel()
		calc := newPricingCalc(t, nil)
		_, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
			Resource: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "azure-native:compute:VirtualMachine",
				Attributes: mustAttributes(t, map[string]any{
					"location":        "westeurope",
					"hardwareProfile": map[string]any{"vmSize": pulumiUnknownValue},
				}),
			},
		})
		if status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), "sku") {
			t.Fatalf("err = %v, want InvalidArgument naming sku", err)
		}
	})
	t.Run("instances", func(t *testing.T) {
		t.Parallel()
		calc := newPricingCalc(t, []azureclient.PriceItem{
			vmRow("Standard_D2s_v5", "Virtual Machines Dsv5 Series", "D2s v5", 0.115),
		})
		req := vmRequest(
			"azure:compute/linuxVirtualMachineScaleSet:LinuxVirtualMachineScaleSet", "Standard_D2s_v5", nil,
		)
		req.Resource.Attributes = mustAttributes(t, map[string]any{"instances": pulumiUnknownValue})
		resp, err := calc.GetProjectedCost(context.Background(), req)
		if err != nil {
			t.Fatalf("GetProjectedCost() error = %v", err)
		}
		if want := 0.115 * 730; math.Abs(resp.GetCostPerMonth()-want) > 1e-9 {
			t.Fatalf("cost = %v, want one instance %v", resp.GetCostPerMonth(), want)
		}
	})
}

func oversizeAttributes(t *testing.T) *structpb.Struct {
	t.Helper()
	return mustAttributes(t, map[string]any{"blob": strings.Repeat("x", pluginsdk.MaxAttributesBytes)})
}

func TestOversizeAttributes_EachRPC_RejectsOrReportsUnsupported(t *testing.T) {
	t.Parallel()

	azure := func() *finfocusv1.ResourceDescriptor {
		return &finfocusv1.ResourceDescriptor{
			Provider:     "azure",
			ResourceType: "azure-native:sql:Database",
			Region:       "eastus",
			Sku:          "GP_Gen5_4",
			Attributes:   oversizeAttributes(t),
		}
	}
	calc := newPricingCalc(t, nil)

	supports, err := calc.Supports(context.Background(), &finfocusv1.SupportsRequest{Resource: azure()})
	if err != nil || supports.GetSupported() || !strings.Contains(supports.GetReason(), "byte limit") {
		t.Fatalf("Supports() = %v, %v; want unsupported naming the byte limit", supports, err)
	}

	dry, err := calc.DryRun(context.Background(), &finfocusv1.DryRunRequest{Resource: azure()})
	if err != nil || !dry.GetResourceTypeSupported() || dry.GetConfigurationValid() {
		t.Fatalf("DryRun() = %v, %v; want supported type with an invalid configuration", dry, err)
	}

	foreign := &finfocusv1.ResourceDescriptor{
		Provider:     "aws",
		ResourceType: "aws:ec2/instance:Instance",
		Attributes:   oversizeAttributes(t),
	}
	dry, err = calc.DryRun(context.Background(), &finfocusv1.DryRunRequest{Resource: foreign})
	if err != nil || dry.GetResourceTypeSupported() {
		t.Fatalf("DryRun(non-Azure) = %v, %v; want the type unsupported", dry, err)
	}

	_, err = calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{Resource: azure()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("GetPricingSpec() code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}
