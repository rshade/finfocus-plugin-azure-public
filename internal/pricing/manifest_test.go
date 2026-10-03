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

func expectedManifest() *finfocusv1.PluginManifest {
	return &finfocusv1.PluginManifest{
		Metadata: &finfocusv1.PluginMetadata{
			Name:        "azure-public",
			Version:     "0.1.0",
			Description: "Estimates Azure resource costs from the public Azure Retail Prices API",
			Author:      "Richard Shade",
			Repository:  "https://github.com/rshade/finfocus-plugin-azure-public",
			License:     "Apache-2.0",
			Keywords:    []string{"finfocus", "cost", "plugin", "azure"},
		},
		Specification: &finfocusv1.PluginSpecification{
			SpecVersion:        strings.TrimPrefix(pluginsdk.SpecVersion, "v"),
			SupportedProviders: []string{providerAzure, providerAzureNative},
			SupportedResources: map[string]*finfocusv1.ProviderResources{
				providerAzure: {
					ResourceTypes: SupportedResourceTypes(),
					BillingModes:  manifestBillingModes(),
				},
			},
			Capabilities: []string{"cost_projection", "cost_retrieval", "pricing_specs", "caching"},
			ServiceDefinition: &finfocusv1.ServiceDefinition{
				ServiceName: "CostSourceService",
				PackageName: "finfocus.v1",
				Methods:     []string{"Name", "Supports", "GetProjectedCost", "GetActualCost", "GetPricingSpec"},
			},
		},
		Installation: &finfocusv1.InstallationSpec{
			InstallationMethod: finfocusv1.InstallationMethod_INSTALLATION_METHOD_BINARY,
		},
	}
}

func manifestBillingModes() []string {
	modes := []string{
		billingModePerHour,
		billingModePerGBMonth,
		billingModePerMonth,
		billingModePerSecond,
		billingModePerRU,
	}
	sort.Strings(modes)
	return modes
}

func TestManifestFiles_PluginCatalog_MatchExpected(t *testing.T) {
	want := expectedManifest()
	for _, name := range []string{"manifest.json", "manifest.yaml"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", name)
			if updateManifestRequested() {
				writeManifest(t, path, want)
			}

			got, err := pluginsdk.LoadManifest(path)
			if err != nil {
				t.Fatalf("load %s: %v", name, err)
			}
			if !proto.Equal(got, want) {
				t.Fatalf(
					"%s is out of date with the plugin catalog\ngot:  %v\nwant: %v\n"+
						"regenerate with: go test ./internal/pricing -run TestManifestFiles -update-manifest",
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
}

// The registry validator reads snake_case keys and lowercase installation
// methods, and its provider list has no azure-native, while SaveManifest
// writes protojson camelCase and enum names. This test checks the registry
// view of the manifest, with azure-native left out on purpose (filed as
// rshade/finfocus-spec#611).
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

	spec, _ := doc["specification"].(map[string]any)
	var providers []any
	for _, p := range m.GetSpecification().GetSupportedProviders() {
		if p != providerAzureNative {
			providers = append(providers, p)
		}
	}
	spec["supported_providers"] = providers

	install, _ := doc["installation"].(map[string]any)
	method := m.GetInstallation().GetInstallationMethod().String()
	install["installation_method"] = strings.ToLower(strings.TrimPrefix(method, "INSTALLATION_METHOD_"))

	view, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal registry view: %v", err)
	}
	return view
}

func updateManifestRequested() bool {
	f := flag.Lookup("update-manifest")
	return f != nil && f.Value.String() == "true"
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
	if err = os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
