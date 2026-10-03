package pricing

import (
	"context"
	"slices"
	"testing"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// finfocus core normalizes ResourceDescriptor.provider to the cloud
// (rshade/finfocus#1645), so the plugin advertises azure alone. The Pulumi
// package is read from the resource_type prefix.
func TestGetPluginInfo_OverGRPC_AdvertisesOnlyAzure(t *testing.T) {
	t.Parallel()

	client := newDiscoveryClient(t, NewCalculator(zerolog.Nop()))
	resp, err := client.GetPluginInfo(context.Background(), &finfocusv1.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo() error = %v", err)
	}
	if want := []string{providerAzure}; !slices.Equal(resp.GetProviders(), want) {
		t.Fatalf("providers = %v, want %v", resp.GetProviders(), want)
	}
}

func TestExpectedManifest_SupportedProviders_OnlyAzure(t *testing.T) {
	t.Parallel()

	got := expectedManifest().GetSpecification().GetSupportedProviders()
	if want := []string{providerAzure}; !slices.Equal(got, want) {
		t.Fatalf("supported_providers = %v, want %v", got, want)
	}
}

func TestDryRunDescriptor_EmptyProvider_InfersAzureForEveryPackage(t *testing.T) {
	t.Parallel()

	for _, resourceType := range []string{
		"azure-native:compute:VirtualMachine",
		"azure:compute/linuxVirtualMachine:LinuxVirtualMachine",
		"compute/VirtualMachine",
	} {
		got := dryRunDescriptor(&finfocusv1.ResourceDescriptor{ResourceType: resourceType}).GetProvider()
		if got != providerAzure {
			t.Fatalf("inferred provider for %s = %q, want %q", resourceType, got, providerAzure)
		}
	}
}

// TestCostRPCs_AzureNativeTokens_SameForBothProviders sends every Azure Native
// resource from the real Pulumi plan twice: once as a host with
// rshade/finfocus#1645 sends it (provider azure) and once as an older host
// sends it (provider azure-native). Both must get the same answer.
func TestCostRPCs_AzureNativeTokens_SameForBothProviders(t *testing.T) {
	t.Parallel()

	plan := loadRealPlan(t)
	views := loadCoreViews(t)
	previews := loadPreviewInputs(t)

	checked := 0
	for _, tc := range plan.Cases {
		view, ok := views[tc.ID]
		if !ok || view.Provider != providerAzureNative {
			continue
		}
		dotted, err := dottedRequest(view, previews, tc)
		if err != nil {
			t.Fatalf("case %s dotted inputs: %v", tc.ID, err)
		}
		for input, req := range map[string]*finfocusv1.GetProjectedCostRequest{
			"core":   requestFromView(view),
			"dotted": dotted,
		} {
			t.Run(tc.ID+"/"+input, func(t *testing.T) {
				t.Parallel()
				assertSameForBothProviders(t, tc.Expect.Rows, req.GetResource())
			})
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no azure-native cases found in the real plan")
	}
}

func assertSameForBothProviders(t *testing.T, rows []azureclient.PriceItem, resource *finfocusv1.ResourceDescriptor) {
	t.Helper()

	asProvider := func(provider string) *finfocusv1.ResourceDescriptor {
		desc, ok := proto.Clone(resource).(*finfocusv1.ResourceDescriptor)
		if !ok {
			t.Fatal("clone descriptor")
		}
		desc.Provider = provider
		return desc
	}
	ctx := context.Background()
	calls := map[string]func(finfocusv1.CostSourceServiceClient, *finfocusv1.ResourceDescriptor) (proto.Message, error){
		"GetProjectedCost": func(c finfocusv1.CostSourceServiceClient, d *finfocusv1.ResourceDescriptor) (proto.Message, error) {
			return c.GetProjectedCost(ctx, &finfocusv1.GetProjectedCostRequest{Resource: d})
		},
		"Supports": func(c finfocusv1.CostSourceServiceClient, d *finfocusv1.ResourceDescriptor) (proto.Message, error) {
			return c.Supports(ctx, &finfocusv1.SupportsRequest{Resource: d})
		},
		"DryRun": func(c finfocusv1.CostSourceServiceClient, d *finfocusv1.ResourceDescriptor) (proto.Message, error) {
			return c.DryRun(ctx, &finfocusv1.DryRunRequest{Resource: d})
		},
		"GetPricingSpec": func(c finfocusv1.CostSourceServiceClient, d *finfocusv1.ResourceDescriptor) (proto.Message, error) {
			return c.GetPricingSpec(ctx, &finfocusv1.GetPricingSpecRequest{Resource: d})
		},
	}
	for name, call := range calls {
		normalized, normalizedErr := call(newDiscoveryClient(t, newPricingCalc(t, rows)), asProvider(providerAzure))
		older, olderErr := call(newDiscoveryClient(t, newPricingCalc(t, rows)), asProvider(providerAzureNative))
		if status.Code(normalizedErr) != status.Code(olderErr) ||
			status.Convert(normalizedErr).Message() != status.Convert(olderErr).Message() {
			t.Fatalf("%s: provider azure error = %v, provider azure-native error = %v", name, normalizedErr, olderErr)
		}
		if normalizedErr != nil {
			continue
		}
		clearExpiresAt(normalized)
		clearExpiresAt(older)
		if !proto.Equal(normalized, older) {
			t.Fatalf("%s differs by provider:\nazure:        %v\nazure-native: %v", name, normalized, older)
		}
	}
}

// clearExpiresAt drops the cache hint, which is derived from the clock.
func clearExpiresAt(m proto.Message) {
	msg := m.ProtoReflect()
	if field := msg.Descriptor().Fields().ByName("expires_at"); field != nil {
		msg.Clear(field)
	}
}
