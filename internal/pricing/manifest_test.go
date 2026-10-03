package pricing

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus-spec/sdk/go/registry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//nolint:gochecknoglobals // Test flag; registered once at package init.
var updateManifest = flag.Bool(
	"update-manifest",
	false,
	"rewrite manifest.json and manifest.yaml from the plugin's resource types",
)

// servedMethods are the CostSourceService RPCs Calculator implements. Every
// other RPC falls through to UnimplementedCostSourceServiceServer.
func servedMethods() []string {
	return []string{
		"Name", "Supports", "GetProjectedCost", "GetActualCost", "GetPricingSpec",
		"EstimateCost", "GetPluginInfo", "DryRun",
	}
}

// manifestCapabilities are the capabilities GetPluginInfo serves, under their
// manifest names, plus caching, which has no protocol capability.
func manifestCapabilities() []string {
	caps := PluginInfo().Capabilities
	names := make([]string, 0, len(caps)+1)
	for _, c := range caps {
		names = append(names, registry.ManifestCapabilityName(c))
	}
	return append(names, registry.PluginCapabilityCaching.String())
}

func expectedManifest() *finfocusv1.PluginManifest {
	return &finfocusv1.PluginManifest{
		Metadata: &finfocusv1.PluginMetadata{
			Name:        "azure-public",
			Version:     pluginVersion,
			Description: "Estimates Azure resource costs from the public Azure Retail Prices API",
			Author:      "Richard Shade",
			Repository:  "https://github.com/rshade/finfocus-plugin-azure-public",
			License:     "Apache-2.0",
			Keywords:    []string{"finfocus", "cost", "plugin", "azure"},
		},
		Specification: &finfocusv1.PluginSpecification{
			SpecVersion:        strings.TrimPrefix(pluginsdk.SpecVersion, "v"),
			SupportedProviders: []string{providerAzure},
			SupportedResources: map[string]*finfocusv1.ProviderResources{
				providerAzure: {
					ResourceTypes: SupportedResourceTypes(),
					BillingModes:  specBillingModes(),
				},
			},
			Capabilities: manifestCapabilities(),
			ServiceDefinition: &finfocusv1.ServiceDefinition{
				ServiceName: "CostSourceService",
				PackageName: "finfocus.v1",
				Methods:     servedMethods(),
			},
		},
		Installation: &finfocusv1.InstallationSpec{
			InstallationMethod: finfocusv1.InstallationMethod_INSTALLATION_METHOD_BINARY,
		},
	}
}

func TestExpectedManifest_CommittedFiles_MatchExpected(t *testing.T) {
	want := expectedManifest()
	for _, name := range []string{"manifest.json", "manifest.yaml"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", name)
			if *updateManifest {
				if err := pluginsdk.SaveManifest(path, want); err != nil {
					t.Fatalf("save %s: %v", name, err)
				}
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if wantBytes := marshalManifest(t, name, want); !bytes.Equal(got, wantBytes) {
				t.Fatalf(
					"%s is out of date with the plugin catalog\ngot:\n%s\nwant:\n%s\n"+
						"regenerate with: go test ./internal/pricing -run TestExpectedManifest_CommittedFiles -update-manifest",
					name, got, wantBytes,
				)
			}
		})
	}
}

// ValidatePluginManifest reads JSON only. The YAML file is validated through
// the canonical JSON of what LoadManifest reads from it, as the SDK documents.
func TestExpectedManifest_CommittedFiles_PassRegistryValidation(t *testing.T) {
	for _, name := range []string{"manifest.json", "manifest.yaml"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if filepath.Ext(path) == ".yaml" {
				loaded, loadErr := pluginsdk.LoadManifest(path)
				if loadErr != nil {
					t.Fatalf("load %s: %v", name, loadErr)
				}
				data = marshalManifest(t, "manifest.json", loaded)
			}
			if err = registry.ValidatePluginManifest(data); err != nil {
				t.Fatalf("registry validation of %s: %v", name, err)
			}
		})
	}
}

// Every CostSourceService RPC the manifest leaves out must answer
// Unimplemented, and every one it lists must not.
func TestExpectedManifest_Methods_MatchServedRPCs(t *testing.T) {
	listed := make(map[string]bool)
	for _, m := range expectedManifest().GetSpecification().GetServiceDefinition().GetMethods() {
		listed[m] = true
	}
	calc := reflect.ValueOf(NewCalculator(zerolog.Nop()))
	for _, name := range registry.AllServiceMethods() {
		t.Run(name, func(t *testing.T) {
			method := calc.MethodByName(name)
			if !method.IsValid() {
				t.Fatalf("Calculator has no %s method", name)
			}
			if method.Type().NumIn() != 2 {
				// Name() string shadows the RPC; the SDK serves it.
				if !listed[name] {
					t.Fatalf("%s is implemented but not listed", name)
				}
				return
			}
			req := reflect.New(method.Type().In(1).Elem())
			out := method.Call([]reflect.Value{reflect.ValueOf(context.Background()), req})
			var err error
			if e, ok := out[len(out)-1].Interface().(error); ok {
				err = e
			}
			unimplemented := status.Code(err) == codes.Unimplemented
			if listed[name] && unimplemented {
				t.Fatalf("%s is listed but answers Unimplemented: %v", name, err)
			}
			if !listed[name] && !unimplemented {
				t.Fatalf("%s is served (err %v) but not listed", name, err)
			}
		})
	}
}

func marshalManifest(t *testing.T, name string, m *finfocusv1.PluginManifest) []byte {
	t.Helper()
	marshal := pluginsdk.MarshalManifestJSON
	if filepath.Ext(name) == ".yaml" {
		marshal = pluginsdk.MarshalManifestYAML
	}
	data, err := marshal(m)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	return data
}

func TestSpecRate_EveryBranch_ReturnsListedBillingMode(t *testing.T) {
	modes := specBillingModes()
	if !sort.StringsAreSorted(modes) {
		t.Fatalf("specBillingModes is not sorted: %v", modes)
	}
	listed := make(map[string]bool, len(modes))
	for _, mode := range modes {
		listed[mode] = true
	}

	tests := []struct {
		name   string
		meters []quoteMeter
		want   string
	}{
		{
			name: "consumption gb-second",
			meters: []quoteMeter{
				{key: breakdownExecutions, unit: "10"},
				{key: breakdownGBSeconds, unit: "1 GB Second"},
			},
			want: billingModePerSecond,
		},
		{
			name:   "cosmos request units",
			meters: []quoteMeter{{key: cosmosComponentRU, unit: cosmosUnitPerHour, specUnit: "100 RU/s per hour"}},
			want:   billingModePerRU,
		},
		{
			name:   "premium vcpu",
			meters: []quoteMeter{{key: breakdownVCPU, unit: appServiceUnitHour}, {key: breakdownMemory}},
			want:   billingModePerVCPUHour,
		},
		{name: "hourly", meters: []quoteMeter{{unit: appServiceUnitHour}}, want: billingModePerHour},
		{name: "storage", meters: []quoteMeter{{unit: storageUnitGBMonth}}, want: billingModePerGBMonth},
		{name: "monthly", meters: []quoteMeter{{unit: "1/Month"}}, want: billingModePerMonth},
		{name: "processed data", meters: []quoteMeter{{unit: loadBalancerUnitData}}, want: billingModePerDataGB},
		{name: "unrecognised unit", meters: []quoteMeter{{unit: "1M"}}, want: billingModeNone},
		{name: "no meters", meters: nil, want: billingModePerHour},
	}
	seen := make(map[string]bool, len(modes))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _ := specRate(tt.meters)
			if got != tt.want {
				t.Fatalf("specRate mode = %q, want %q", got, tt.want)
			}
			if got == billingModeNone {
				if listed[got] {
					t.Fatalf("specBillingModes lists the %q fallback", got)
				}
				return
			}
			if !listed[got] {
				t.Fatalf("specRate returned %q, which specBillingModes does not list", got)
			}
		})
		seen[tt.want] = true
	}
	for _, mode := range modes {
		if !seen[mode] {
			t.Errorf("specBillingModes lists %q, which no specRate branch returns", mode)
		}
	}
}
