package pricing

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus-spec/sdk/go/pricing"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// coreUnitFallbacks are the PricingSpec units finfocus core turns into a
// billing mode when it does not recognise billing_mode. Copied from
// rshade/finfocus commit 706c5f3, internal/engine/pricing_spec.go:293-324
// (normalizeBilling); re-check it when core changes that function.
// A spec whose mode core cannot read must not use one of these units, or core
// would price a GB-second or request-unit rate as an hourly or GB-month rate.
func coreUnitFallbacks() []string {
	return []string{
		"hour", "hours", "hr", "day", "days", "gb-month", "gb_month", "gb",
		"request", "requests", "month", "monthly", "cpu-hour", "cpu_hour",
	}
}

// coreBillingModes are the billing_mode values core's normalizeBilling reads
// and turns into a monthly total (same source as coreUnitFallbacks).
func coreBillingModes() []string {
	return []string{
		"per_hour", "hourly", "hour", "per_day", "daily", "day",
		"per_gb_month", "gb-month", "gb_month", "per_gb",
		"per_request", "request", "requests", "flat", "monthly", "per_month",
		"per_cpu_hour", "cpu-hour", "cpu_hour",
	}
}

func wantCapabilities() []finfocusv1.PluginCapability {
	return []finfocusv1.PluginCapability{
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS,
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_PRICING_SPEC,
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_ESTIMATE_COST,
		finfocusv1.PluginCapability_PLUGIN_CAPABILITY_DRY_RUN,
	}
}

func TestGetPluginInfo_Direct_ListsImplementedCapabilities(t *testing.T) {
	t.Parallel()

	resp, err := NewCalculator(zerolog.Nop()).GetPluginInfo(context.Background(), &finfocusv1.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo() error = %v", err)
	}
	if !slices.Equal(resp.GetCapabilities(), wantCapabilities()) {
		t.Fatalf("capabilities = %v, want %v", resp.GetCapabilities(), wantCapabilities())
	}
	if got := resp.GetMetadata()[pluginMetadataType]; got != pluginTypePublicPricing {
		t.Fatalf("metadata[%s] = %q, want %q", pluginMetadataType, got, pluginTypePublicPricing)
	}
	if _, ok := any(NewCalculator(zerolog.Nop())).(pluginsdk.DryRunHandler); !ok {
		t.Fatal("DRY_RUN is advertised but Calculator does not implement pluginsdk.DryRunHandler")
	}
}

func TestGetPluginInfo_OverGRPC_SendsExplicitCapabilitiesAndType(t *testing.T) {
	t.Parallel()

	client := newDiscoveryClient(t, NewCalculator(zerolog.Nop()))
	resp, err := client.GetPluginInfo(context.Background(), &finfocusv1.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo() error = %v", err)
	}
	if !slices.Equal(resp.GetCapabilities(), wantCapabilities()) {
		t.Fatalf("capabilities = %v, want %v", resp.GetCapabilities(), wantCapabilities())
	}
	if got := resp.GetMetadata()[pluginMetadataType]; got != pluginTypePublicPricing {
		t.Fatalf("metadata[%s] = %q, want %q", pluginMetadataType, got, pluginTypePublicPricing)
	}
	// The SDK derives the legacy supports_* keys from the explicit list, so an
	// unserved RPC such as BatchCost must not appear there either.
	metadata := resp.GetMetadata()
	if metadata["supports_dry_run"] != "true" {
		t.Fatalf("metadata supports_dry_run = %q, want true (metadata=%v)", metadata["supports_dry_run"], metadata)
	}
	for _, key := range []string{"supports_batch_cost", "max_batch_size", "supports_recommendations"} {
		if value, ok := metadata[key]; ok {
			t.Fatalf("metadata %s = %q is advertised for an RPC the plugin does not serve", key, value)
		}
	}
}

func TestMissingFieldsError_Wrapped_KeepsStatusAndFields(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("quote: %w", missingFieldsError([]string{"region", "sku"}))
	if got := status.Convert(missingFieldsError([]string{"region", "sku"})).Message(); got !=
		"missing required field(s): region, sku" {
		t.Fatalf("status message = %q", got)
	}
	if status.Code(missingFieldsError([]string{"region"})) != codes.InvalidArgument {
		t.Fatal("missingFieldsError is not InvalidArgument")
	}
	var missing *requiredFieldsError
	if !errors.As(err, &missing) || !slices.Equal(missing.fields, []string{"region", "sku"}) {
		t.Fatalf("errors.As(%v) did not recover the field list", err)
	}
}

// TestHandleDryRun_PluginInspectRequest_ReturnsFieldMappings sends the request
// `finfocus plugin inspect <plugin> <type>` builds: a resource type and
// nothing else (rshade/finfocus internal/cli/plugin_inspect.go).
func TestHandleDryRun_PluginInspectRequest_ReturnsFieldMappings(t *testing.T) {
	t.Parallel()

	client := newDiscoveryClient(t, newDryRunCalc(t))
	for _, resourceType := range []string{
		"compute/VirtualMachine",
		"azure:compute/linuxVirtualMachine:LinuxVirtualMachine",
		"azure-native:compute:VirtualMachine",
		"azure:storage/managedDisk:ManagedDisk",
		"azure-native:documentdb:DatabaseAccount",
	} {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()

			resp, err := client.DryRun(context.Background(), &finfocusv1.DryRunRequest{
				Resource: &finfocusv1.ResourceDescriptor{ResourceType: resourceType},
			})
			if err != nil {
				t.Fatalf("DryRun() error = %v", err)
			}
			if !resp.GetResourceTypeSupported() {
				t.Fatalf("resource_type_supported = false for %s", resourceType)
			}
			assertProjectedFieldMappings(t, resp.GetFieldMappings())
			if resp.GetConfigurationValid() {
				t.Fatal("configuration_valid = true for a descriptor with no region")
			}
			if !strings.Contains(strings.Join(resp.GetConfigurationErrors(), " "), "region") {
				t.Fatalf("configuration_errors = %v, want the missing region named", resp.GetConfigurationErrors())
			}
		})
	}
}

func TestHandleDryRun_EmptyProviderOtherCloud_IsUnsupported(t *testing.T) {
	t.Parallel()

	calc := newDryRunCalc(t)
	for _, resourceType := range []string{"aws:ec2/instance:Instance", "gcp:compute/instance:Instance"} {
		resp, err := calc.HandleDryRun(context.Background(), &finfocusv1.DryRunRequest{
			Resource: &finfocusv1.ResourceDescriptor{ResourceType: resourceType},
		})
		if err != nil {
			t.Fatalf("HandleDryRun(%s) error = %v", resourceType, err)
		}
		if resp.GetResourceTypeSupported() {
			t.Fatalf("resource_type_supported = true for %s", resourceType)
		}
	}
}

// Core's Supports call always carries the provider (rshade/finfocus
// internal/engine/engine.go checkPluginSupports), so Supports keeps requiring it.
func TestSupports_EmptyProvider_StaysUnsupported(t *testing.T) {
	t.Parallel()

	resp, err := NewCalculator(zerolog.Nop()).Supports(context.Background(), &finfocusv1.SupportsRequest{
		Resource: &finfocusv1.ResourceDescriptor{
			ResourceType: "compute/VirtualMachine",
			Region:       "eastus",
			Sku:          "Standard_B1s",
		},
	})
	if err != nil {
		t.Fatalf("Supports() error = %v", err)
	}
	if resp.GetSupported() {
		t.Fatal("Supports() accepted a descriptor with no provider")
	}
}

func TestGetPricingSpec_EverySupportedType_ModeIsValidAndReadableOrSkipped(t *testing.T) {
	t.Parallel()

	fx := loadPricingSpecFX(t)
	calc := newPricingSpecCalc(t, fx)
	for resourceType, desc := range dryRunDescriptors() {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{Resource: desc})
			if err != nil {
				t.Fatalf("GetPricingSpec() error = %v", err)
			}
			spec := resp.GetSpec()
			assertHonestBilling(t, spec)
			if len(spec.GetAssumptions()) == 0 {
				t.Fatal("assumptions are empty")
			}
			if spec.GetBillingMode() == billingModePerHour && !containsText(spec.GetAssumptions(), "730 hours") {
				t.Fatalf("hourly spec assumptions %v do not state 730 hours", spec.GetAssumptions())
			}
		})
	}
}

func TestGetPricingSpec_UsageNotSupplied_ReturnsUnitRate(t *testing.T) {
	t.Parallel()

	fx := loadPricingSpecFX(t)
	calc := newPricingSpecCalc(t, fx)
	cosmosServerless := requireCosmosItem(t, fx.cosmos, "Azure Cosmos DB serverless", cosmosTestSKURU, "1M RUs", "1M")
	tests := []struct {
		name     string
		desc     *finfocusv1.ResourceDescriptor
		missing  []string
		mode     string
		unit     string
		rate     float64
		hintName string
	}{
		{
			name:     "cosmos without request units per second",
			desc:     descriptorWithoutTags(dryRunDescriptors()["cosmosdb/Account"], "ru_per_second", "size_gb"),
			missing:  []string{"ru_per_second"},
			mode:     billingModePerRU,
			unit:     "100 RU/s per hour",
			rate:     fx.cosmosRU.RetailPrice,
			hintName: "ru_per_second",
		},
		{
			name: "cosmos serverless without request units",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "cosmosdb/Account",
				Region:       "eastus",
				Tags:         map[string]string{"pricing_model": "serverless"},
			},
			missing:  []string{"request_units"},
			mode:     billingModePerRU,
			unit:     "1M RU",
			rate:     cosmosServerless.RetailPrice,
			hintName: "request_units",
		},
		{
			name:     "consumption function without usage",
			desc:     descriptorWithoutTags(dryRunDescriptors()["web/FunctionApp"], "executions", "gb_seconds"),
			missing:  []string{"executions", "gb_seconds"},
			mode:     billingModePerSecond,
			unit:     specUnitGBSecond,
			rate:     fx.gbItem.RetailPrice,
			hintName: "gb_seconds",
		},
		{
			name: "premium function without size",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "web/FunctionApp",
				Region:       "eastus",
				Sku:          "Premium",
			},
			missing:  []string{"vcpu_count", "memory_gib"},
			mode:     billingModePerVCPUHour,
			unit:     specUnitVCPUHour,
			rate:     fx.vcpuItem.RetailPrice,
			hintName: "vcpu_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{Resource: tt.desc})
			if err != nil {
				t.Fatalf("GetPricingSpec() error = %v", err)
			}
			spec := resp.GetSpec()
			if spec.GetBillingMode() != tt.mode || spec.GetUnit() != tt.unit {
				t.Fatalf("mode/unit = %s/%s, want %s/%s", spec.GetBillingMode(), spec.GetUnit(), tt.mode, tt.unit)
			}
			if spec.GetRatePerUnit() != tt.rate {
				t.Fatalf("rate_per_unit = %v, want %v", spec.GetRatePerUnit(), tt.rate)
			}
			if slices.Contains(coreBillingModes(), spec.GetBillingMode()) {
				t.Fatalf("billing_mode %q is one core multiplies, but usage %v was not supplied",
					spec.GetBillingMode(), tt.missing)
			}
			for _, field := range tt.missing {
				if !containsText(spec.GetAssumptions(), field) {
					t.Fatalf("assumptions %v do not name the missing %s", spec.GetAssumptions(), field)
				}
			}
			if !hasHint(spec.GetMetricHints(), tt.hintName) {
				t.Fatalf("metric_hints %v do not name %s", spec.GetMetricHints(), tt.hintName)
			}
		})
	}
}

// A spec core would multiply into a monthly total must not be built from a
// placeholder quantity, so a missing size keeps the InvalidArgument error for
// kinds whose rate is per hour, per GB-month, or per month.
func TestGetPricingSpec_CoreComputedModeUsageMissing_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	calc := newPricingSpecCalc(t, loadPricingSpecFX(t))
	for _, resourceType := range []string{"storage/BlobStorage", "storage/StorageAccount", "sql/Database"} {
		t.Run(resourceType, func(t *testing.T) {
			t.Parallel()

			_, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{
				Resource: descriptorWithoutTags(dryRunDescriptors()[resourceType], "size_gb"),
			})
			assertInvalidArgument(t, err, "size_gb")
		})
	}
}

func TestGetPricingSpec_LoadBalancerNoRulesWithData_ReportsProcessedDataRate(t *testing.T) {
	t.Parallel()

	fx := loadPricingSpecFX(t)
	calc := newPricingSpecCalc(t, fx)
	data := requireLoadBalancerMeter(t, fx.loadBalancer, loadBalancerMeterData, loadBalancerUnitData)
	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "network/LoadBalancer",
		Region:       "eastus",
		Sku:          "Standard",
		Tags:         map[string]string{"rule_count": "0", "data_processed_gb": "100"},
	}

	projected, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{Resource: desc})
	if err != nil {
		t.Fatalf("GetProjectedCost() error = %v", err)
	}
	wantMonthly := 100 * data.RetailPrice
	if projected.GetCostPerMonth() != wantMonthly {
		t.Fatalf("cost_per_month = %v, want %v", projected.GetCostPerMonth(), wantMonthly)
	}
	breakdown := projected.GetCostBreakdown()
	if len(breakdown) != 1 || breakdown[loadBalancerComponentData] != wantMonthly {
		t.Fatalf("cost_breakdown = %v, want only %s=%v", breakdown, loadBalancerComponentData, wantMonthly)
	}

	resp, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{Resource: desc})
	if err != nil {
		t.Fatalf("GetPricingSpec() error = %v", err)
	}
	spec := resp.GetSpec()
	if spec.GetBillingMode() != billingModePerDataGB || spec.GetUnit() != specUnitDataGB {
		t.Fatalf(
			"mode/unit = %s/%s, want %s/%s",
			spec.GetBillingMode(),
			spec.GetUnit(),
			billingModePerDataGB,
			specUnitDataGB,
		)
	}
	if spec.GetRatePerUnit() != data.RetailPrice {
		t.Fatalf("rate_per_unit = %v, want %v", spec.GetRatePerUnit(), data.RetailPrice)
	}
	if containsText(spec.GetAssumptions(), "included rules meter") {
		t.Fatalf(
			"assumptions %v describe the included rules meter, which rule_count=0 does not bill",
			spec.GetAssumptions(),
		)
	}
	assertHonestBilling(t, spec)
}

func TestGetPricingSpec_SpotVM_AssumptionNamesSpot(t *testing.T) {
	t.Parallel()

	calc := newPricingSpecCalc(t, loadPricingSpecFX(t))
	for name, tags := range map[string]map[string]string{
		"priority":                       {"priority": "Spot"},
		"virtualMachineProfile.priority": {"virtualMachineProfile.priority": "Spot"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resp, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{
				Resource: &finfocusv1.ResourceDescriptor{
					Provider:     "azure",
					ResourceType: "compute/VirtualMachine",
					Region:       "eastus",
					Sku:          "Standard_D2s_v3",
					Tags:         tags,
				},
			})
			if err != nil {
				t.Fatalf("GetPricingSpec() error = %v", err)
			}
			assumptions := resp.GetSpec().GetAssumptions()
			if !containsText(assumptions, "Spot") || containsText(assumptions, "on-demand") {
				t.Fatalf("assumptions %v, want the Spot instance named and no on-demand claim", assumptions)
			}
		})
	}
}

func TestGetPricingSpec_IdentityMissing_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	calc := newPricingSpecCalc(t, loadPricingSpecFX(t))
	tests := []struct {
		name  string
		desc  *finfocusv1.ResourceDescriptor
		field string
	}{
		{
			name:  "disk size picks the tier",
			desc:  descriptorWithoutTags(dryRunDescriptors()["storage/ManagedDisk"], "size_gb"),
			field: "size_gb",
		},
		{
			name: "cosmos region",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "cosmosdb/Account",
				Tags:         map[string]string{"ru_per_second": "400"},
			},
			field: "region",
		},
		{
			name: "storage sku and size",
			desc: &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "storage/StorageAccount",
				Region:       "eastus",
			},
			field: "sku",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := calc.GetPricingSpec(context.Background(), &finfocusv1.GetPricingSpecRequest{Resource: tt.desc})
			assertInvalidArgument(t, err, tt.field)
		})
	}
}

func TestGetProjectedCost_UsageNotSupplied_StillInvalidArgument(t *testing.T) {
	t.Parallel()

	calc := newPricingSpecCalc(t, loadPricingSpecFX(t))
	_, err := calc.GetProjectedCost(context.Background(), &finfocusv1.GetProjectedCostRequest{
		Resource: descriptorWithoutTags(dryRunDescriptors()["cosmosdb/Account"], "ru_per_second"),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}

func assertHonestBilling(t *testing.T, spec *finfocusv1.PricingSpec) {
	t.Helper()

	mode := spec.GetBillingMode()
	if !pricing.ValidBillingMode(mode) {
		t.Fatalf("billing_mode %q is not a pluginsdk pricing.BillingMode", mode)
	}
	if slices.Contains(coreBillingModes(), mode) {
		return
	}
	if slices.Contains(coreUnitFallbacks(), strings.ToLower(strings.TrimSpace(spec.GetUnit()))) {
		t.Fatalf("billing_mode %q is not read by core but unit %q is, so core would misprice it", mode, spec.GetUnit())
	}
}

func descriptorWithoutTags(desc *finfocusv1.ResourceDescriptor, drop ...string) *finfocusv1.ResourceDescriptor {
	tags := make(map[string]string, len(desc.GetTags()))
	for key, value := range desc.GetTags() {
		if !slices.Contains(drop, key) {
			tags[key] = value
		}
	}
	return &finfocusv1.ResourceDescriptor{
		Provider:     desc.GetProvider(),
		ResourceType: desc.GetResourceType(),
		Region:       desc.GetRegion(),
		Sku:          desc.GetSku(),
		Tags:         tags,
	}
}

func containsText(values []string, text string) bool {
	for _, value := range values {
		if strings.Contains(value, text) {
			return true
		}
	}
	return false
}

func hasHint(hints []*finfocusv1.UsageMetricHint, metric string) bool {
	for _, hint := range hints {
		if hint.GetMetric() == metric {
			return true
		}
	}
	return false
}

func newDiscoveryClient(t *testing.T, calc *Calculator) finfocusv1.CostSourceServiceClient {
	t.Helper()

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
	return finfocusv1.NewCostSourceServiceClient(conn)
}
