//go:build integration

package examples

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/pricing"
)

// Reference prices last verified: 2026-10-02 against live Azure API, eastus, USD.
// Update these when Azure adjusts pricing and tests fail.
const (
	refAppServiceB1LinuxHourly   = 0.017    // Basic Plan - Linux, meter B1
	refAppServiceP1v3LinuxHourly = 0.155    // Premium v3 Plan - Linux, meter P1 v3 App
	refFunctionsPer10Executions  = 0.000002 // Standard Total Executions, unit 10
	refFunctionsPerGBSecond      = 0.000016 // Standard Execution Time, unit 1 GB Second
	refFunctionsPremiumVCPUHour  = 0.173    // Premium vCPU Duration, unit 1 Hour
	refFunctionsPremiumGiBHour   = 0.0123   // Premium Memory Duration, unit 1 GiB Hour
	refAKSStandardHourly         = 0.10     // Standard Uptime SLA
	refAKSNodeD2sv3Hourly        = 0.096    // Standard_D2s_v3 Linux on-demand node

	functionsFreeExecutions = 1_000_000
	functionsFreeGBSeconds  = 400_000
)

// --- #48: App Service Plan and Function App ---

func TestGetProjectedCost_AppServicePlanLinuxSKUs_MatchesReference(t *testing.T) {
	skipIfDisabled(t)
	t.Cleanup(rateLimitDelay)
	calc, _ := newTestCalculator(t)

	tests := []struct {
		name   string
		sku    string
		hourly float64
	}{
		{name: "B1", sku: "B1", hourly: refAppServiceB1LinuxHourly},
		{name: "P1v3", sku: "P1v3", hourly: refAppServiceP1v3LinuxHourly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := projectedCost(t, calc, &finfocusv1.ResourceDescriptor{
				Provider:     "azure",
				ResourceType: "azure:appservice/servicePlan:ServicePlan",
				Region:       "eastus",
				Sku:          tt.sku,
			})
			assertBreakdownKeys(t, resp, "compute")
			assertInRange(t, resp.GetCostPerMonth(), tt.hourly*pluginsdk.HoursPerMonth)
			t.Logf("App Service %s Linux eastus: $%.4f/month", tt.sku, resp.GetCostPerMonth())
		})
	}
}

func TestGetProjectedCost_FunctionAppConsumptionAboveGrant_BillsOverage(t *testing.T) {
	skipIfDisabled(t)
	t.Cleanup(rateLimitDelay)
	calc, _ := newTestCalculator(t)

	const (
		billableExecutions = 2_000_000
		billableGBSeconds  = 1_000_000
	)
	resp := projectedCost(t, calc, &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "azure:appservice/functionApp:FunctionApp",
		Region:       "eastus",
		Sku:          "Y1",
		Tags: map[string]string{
			"executions": strconv.Itoa(functionsFreeExecutions + billableExecutions),
			"gb_seconds": strconv.Itoa(functionsFreeGBSeconds + billableGBSeconds),
		},
	})
	assertBreakdownKeys(t, resp, "executions", "gb_seconds")

	breakdown := resp.GetCostBreakdown()
	assertInRange(t, breakdown["executions"], billableExecutions/10*refFunctionsPer10Executions)
	assertInRange(t, breakdown["gb_seconds"], billableGBSeconds*refFunctionsPerGBSecond)
	t.Logf("Functions Consumption eastus: $%.4f/month (executions=%.4f gb_seconds=%.4f)",
		resp.GetCostPerMonth(), breakdown["executions"], breakdown["gb_seconds"])
}

func TestGetProjectedCost_FunctionAppPremium_MatchesReference(t *testing.T) {
	skipIfDisabled(t)
	t.Cleanup(rateLimitDelay)
	calc, _ := newTestCalculator(t)

	const (
		vcpuCount = 1.0
		memoryGiB = 3.5
	)
	resp := projectedCost(t, calc, &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "web/FunctionApp",
		Region:       "eastus",
		Tags: map[string]string{
			"pricing_model": "premium",
			"vcpu_count":    strconv.FormatFloat(vcpuCount, 'f', -1, 64),
			"memory_gib":    strconv.FormatFloat(memoryGiB, 'f', -1, 64),
		},
	})
	assertBreakdownKeys(t, resp, "memory", "vcpu")

	breakdown := resp.GetCostBreakdown()
	assertInRange(t, breakdown["vcpu"], vcpuCount*refFunctionsPremiumVCPUHour*pluginsdk.HoursPerMonth)
	assertInRange(t, breakdown["memory"], memoryGiB*refFunctionsPremiumGiBHour*pluginsdk.HoursPerMonth)
	t.Logf("Functions Premium eastus (%.0f vCPU, %.1f GiB): $%.4f/month",
		vcpuCount, memoryGiB, resp.GetCostPerMonth())
}

// --- #49: AKS cluster ---

func TestGetProjectedCost_AKSStandardOneNodePool_SumsControlPlaneAndNodes(t *testing.T) {
	skipIfDisabled(t)
	t.Cleanup(rateLimitDelay)
	calc, _ := newTestCalculator(t)

	const nodeCount = 2
	resp := projectedCost(t, calc, &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "azure:containerservice/kubernetesCluster:KubernetesCluster",
		Region:       "eastus",
		Sku:          "Standard",
		Tags: map[string]string{
			"node_pool_1_name":  "system",
			"node_pool_1_sku":   "Standard_D2s_v3",
			"node_pool_1_count": strconv.Itoa(nodeCount),
		},
	})
	assertBreakdownKeys(t, resp, "control_plane", "node_pool_system")

	breakdown := resp.GetCostBreakdown()
	assertInRange(t, breakdown["control_plane"], refAKSStandardHourly*pluginsdk.HoursPerMonth)
	assertInRange(t, breakdown["node_pool_system"], nodeCount*refAKSNodeD2sv3Hourly*pluginsdk.HoursPerMonth)
	t.Logf("AKS Standard eastus, 2x Standard_D2s_v3: $%.4f/month (control_plane=%.4f nodes=%.4f)",
		resp.GetCostPerMonth(), breakdown["control_plane"], breakdown["node_pool_system"])
}

func TestGetProjectedCost_AKSFreeTierOneNodePool_ControlPlaneIsZero(t *testing.T) {
	skipIfDisabled(t)
	t.Cleanup(rateLimitDelay)
	calc, _ := newTestCalculator(t)

	resp := projectedCost(t, calc, &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "containerservice/KubernetesCluster",
		Region:       "eastus",
		Sku:          "Free",
		Tags: map[string]string{
			"node_pool_1_name":  "system",
			"node_pool_1_sku":   "Standard_D2s_v3",
			"node_pool_1_count": "1",
		},
	})
	assertBreakdownKeys(t, resp, "control_plane", "node_pool_system")

	breakdown := resp.GetCostBreakdown()
	if breakdown["control_plane"] != 0 {
		t.Fatalf("free control_plane = %.4f, want 0", breakdown["control_plane"])
	}
	assertInRange(t, breakdown["node_pool_system"], refAKSNodeD2sv3Hourly*pluginsdk.HoursPerMonth)
	if !strings.Contains(resp.GetBillingDetail(), "FreeTierInfrastructureCost") {
		t.Fatalf("billing_detail = %q, want the unbilled Free meter note", resp.GetBillingDetail())
	}
	t.Logf("AKS Free eastus, 1x Standard_D2s_v3: $%.4f/month (control_plane=0)", resp.GetCostPerMonth())
}

// projectedCost calls GetProjectedCost against the live API and checks the
// response is valid, positive, and that its breakdown sums to the month.
func projectedCost(
	t *testing.T,
	calc *pricing.Calculator,
	desc *finfocusv1.ResourceDescriptor,
) *finfocusv1.GetProjectedCostResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := calc.GetProjectedCost(ctx, &finfocusv1.GetProjectedCostRequest{Resource: desc})
	if err != nil {
		t.Fatalf("GetProjectedCost(%s) failed: %v", desc.GetResourceType(), err)
	}
	if err := pluginsdk.ValidateGetProjectedCostResponse(resp); err != nil {
		t.Fatalf("ValidateGetProjectedCostResponse() failed: %v", err)
	}
	if resp.GetCostPerMonth() <= 0 {
		t.Fatalf("expected positive monthly cost, got %.4f", resp.GetCostPerMonth())
	}

	var sum float64
	for _, v := range resp.GetCostBreakdown() {
		sum += v
	}
	if math.Abs(sum-resp.GetCostPerMonth()) > 0.01 {
		t.Errorf("breakdown sum %.4f != cost_per_month %.4f (%v)",
			sum, resp.GetCostPerMonth(), resp.GetCostBreakdown())
	}
	return resp
}

func assertBreakdownKeys(t *testing.T, resp *finfocusv1.GetProjectedCostResponse, want ...string) {
	t.Helper()
	got := make([]string, 0, len(resp.GetCostBreakdown()))
	for k := range resp.GetCostBreakdown() {
		got = append(got, k)
	}
	sort.Strings(got)
	want = append([]string(nil), want...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("breakdown keys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("breakdown keys = %v, want %v", got, want)
		}
	}
}
