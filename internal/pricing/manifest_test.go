package pricing

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	sdkpricing "github.com/rshade/finfocus-spec/sdk/go/pricing"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus-spec/sdk/go/registry"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// manifestSchemaMaxResourceTypeLength is the maxLength the finfocus-spec
// plugin_manifest.schema.json puts on supported_resources resource_types.
const manifestSchemaMaxResourceTypeLength = 50

//nolint:gochecknoglobals // Test flag; registered once at package init.
var updateManifest = flag.Bool(
	"update-manifest",
	false,
	"rewrite manifest.json and manifest.yaml from the plugin's resource types",
)

// manifestSchemaCapabilities is the capabilities enum in the finfocus-spec
// v0.7.1 plugin_manifest.schema.json. ValidatePluginManifest does not check it.
func manifestSchemaCapabilities() map[string]bool {
	return map[string]bool{
		"cost_retrieval": true, "cost_projection": true, "pricing_specs": true,
		"historical_data": true, "real_time_data": true, "batch_processing": true,
		"rate_limiting": true, "caching": true, "encryption": true,
		"compression": true, "filtering": true, "aggregation": true,
		"multi_tenancy": true, "audit_logging": true,
	}
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
			// cost_retrieval matches the ACTUAL_COSTS capability GetPluginInfo
			// advertises; GetActualCost is a list-price projection, not billed spend.
			Capabilities: []string{"cost_projection", "cost_retrieval", "pricing_specs", "caching"},
			ServiceDefinition: &finfocusv1.ServiceDefinition{
				ServiceName: "CostSourceService",
				PackageName: "finfocus.v1",
				// The schema enum allows only these five; EstimateCost, DryRun and
				// GetPluginInfo are served but cannot be listed (finfocus-spec#611).
				Methods: []string{"Name", "Supports", "GetProjectedCost", "GetActualCost", "GetPricingSpec"},
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
				writeManifest(t, path, want)
			}

			got, err := pluginsdk.LoadManifest(path)
			if err != nil {
				t.Fatalf("load %s: %v", name, err)
			}
			if !proto.Equal(got, want) {
				t.Fatalf(
					"%s is out of date with the plugin catalog\ngot:  %v\nwant: %v\n"+
						"regenerate with: go test ./internal/pricing -run TestExpectedManifest_CommittedFiles -update-manifest",
					name, got, want,
				)
			}
		})
	}
}

func TestExpectedManifest_ResourceTypesAndModes_FitSchemaLimits(t *testing.T) {
	resources := expectedManifest().GetSpecification().GetSupportedResources()[providerAzure]
	if len(resources.GetResourceTypes()) == 0 {
		t.Fatal("no resource types for provider azure")
	}
	for _, rt := range resources.GetResourceTypes() {
		if len(rt) > manifestSchemaMaxResourceTypeLength {
			t.Errorf("resource type %q is longer than %d characters", rt, manifestSchemaMaxResourceTypeLength)
		}
	}
	for _, mode := range resources.GetBillingModes() {
		if !sdkpricing.ValidBillingMode(mode) {
			t.Errorf("billing mode %q is not a finfocus-spec billing mode", mode)
		}
	}
	allowed := manifestSchemaCapabilities()
	for _, capability := range expectedManifest().GetSpecification().GetCapabilities() {
		if !allowed[capability] {
			t.Errorf("capability %q is not in the manifest schema enum", capability)
		}
	}
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

// The committed files use the SaveManifest format (protojson camelCase JSON,
// lowercased-key YAML, the installation method as an enum), so
// registry.ValidatePluginManifest rejects them at the first required key. This
// test validates a converted in-memory view instead: snake_case keys and a
// lowercase installation method (rshade/finfocus-spec#611).
func TestExpectedManifest_RegistryView_PassesRegistryValidation(t *testing.T) {
	view := registryManifestView(t, expectedManifest())
	if err := registry.ValidatePluginManifest(view); err != nil {
		t.Fatalf("registry validation: %v\nmanifest: %s", err, view)
	}
}

func registryManifestView(t *testing.T, m *finfocusv1.PluginManifest) []byte {
	t.Helper()
	raw, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	install, ok := doc["installation"].(map[string]any)
	if !ok {
		t.Fatal("manifest has no installation object")
	}
	method := m.GetInstallation().GetInstallationMethod().String()
	install["installation_method"] = strings.ToLower(strings.TrimPrefix(method, "INSTALLATION_METHOD_"))

	view, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal registry view: %v", err)
	}
	return view
}

func writeManifest(t *testing.T, path string, m *finfocusv1.PluginManifest) {
	t.Helper()
	if err := pluginsdk.SaveManifest(path, m); err != nil {
		t.Fatalf("save %s: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	// protojson randomizes its whitespace between builds; re-indent so the
	// committed file only changes when the manifest does.
	if filepath.Ext(path) == ".json" {
		var buf bytes.Buffer
		if err = json.Indent(&buf, data, "", "  "); err != nil {
			t.Fatalf("indent %s: %v", path, err)
		}
		data = buf.Bytes()
	}
	if !strings.HasSuffix(string(data), "\n") {
		data = append(data, '\n')
	}
	// SaveManifest created the file 0o600; WriteFile keeps that mode.
	if err = os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
