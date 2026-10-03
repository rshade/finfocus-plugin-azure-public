//go:build integration

package examples

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
	"github.com/rshade/finfocus-plugin-azure-public/internal/pricing"
)

// priceTolerance is the band for hand-recorded reference prices in
// projected_cost_integration_test.go. Virtual machine, disk, and AKS node
// tests read their reference live instead (see
// live_reference_integration_test.go).
const priceTolerance = 0.25 // ±25%

func skipIfDisabled(t *testing.T) {
	t.Helper()
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("integration tests disabled via SKIP_INTEGRATION=true")
	}
}

func newTestCalculator(t *testing.T) (*pricing.Calculator, *azureclient.CachedClient) {
	t.Helper()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	config := azureclient.DefaultConfig()
	config.Logger = logger

	client, err := azureclient.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create azure client: %v", err)
	}

	cacheConfig := azureclient.DefaultCacheConfig()
	cacheConfig.Logger = logger

	cachedClient, err := azureclient.NewCachedClient(client, cacheConfig)
	if err != nil {
		t.Fatalf("failed to create cached client: %v", err)
	}

	t.Cleanup(func() { cachedClient.Close() })

	calc := pricing.NewCalculator(logger, cachedClient)
	return calc, cachedClient
}

func assertInRange(t *testing.T, actual, reference float64) {
	t.Helper()
	low := reference * (1 - priceTolerance)
	high := reference * (1 + priceTolerance)
	if actual < low || actual > high {
		t.Errorf("value %.4f out of range [%.4f, %.4f] (reference=%.4f ±%.0f%%)",
			actual, low, high, reference, priceTolerance*100)
	}
}

// 12s delay keeps under 5 queries/minute Azure rate limit.
func rateLimitDelay() {
	time.Sleep(12 * time.Second)
}

func estimateCost(
	t *testing.T,
	calc *pricing.Calculator,
	resourceType string,
	attrs map[string]any,
) *finfocusv1.EstimateCostResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	attributes, err := structpb.NewStruct(attrs)
	if err != nil {
		t.Fatalf("failed to create attributes: %v", err)
	}

	resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
		ResourceType: resourceType,
		Attributes:   attributes,
	})
	if err != nil {
		t.Fatalf("EstimateCost failed: %v", err)
	}
	return resp
}

func assertStandardUSD(t *testing.T, resp *finfocusv1.EstimateCostResponse) {
	t.Helper()
	if resp.GetCostMonthly() <= 0 {
		t.Fatalf("expected positive monthly cost, got %.4f", resp.GetCostMonthly())
	}
	if resp.GetCurrency() != "USD" {
		t.Errorf("expected currency USD, got %q", resp.GetCurrency())
	}
	if got := resp.GetPricingCategory(); got != finfocusv1.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD {
		t.Errorf("expected pricing category STANDARD, got %s", got)
	}
}

// --- User Story 1: VM Cost Estimation (P1) ---

func TestEstimateCost_VMOnDemand_MatchesLiveReference(t *testing.T) {
	skipIfDisabled(t)

	for _, size := range []string{"Standard_B1s", "Standard_D2s_v3"} {
		t.Run(size, func(t *testing.T) {
			t.Cleanup(rateLimitDelay)
			reference := linuxOnDemandVMRate(t, "eastus", size)
			calc, _ := newTestCalculator(t)

			resp := estimateCost(t, calc, "azure:compute/virtualMachine:VirtualMachine", map[string]any{
				"location": "eastus",
				"vmSize":   size,
			})
			assertStandardUSD(t, resp)

			expectedMonthly := reference.RetailPrice * pluginsdk.HoursPerMonth
			assertMatchesLive(t, resp.GetCostMonthly(), expectedMonthly)
			t.Logf("%s eastus: $%.4f/month (live %q %g/h → $%.2f)",
				size, resp.GetCostMonthly(), reference.MeterName, reference.RetailPrice, expectedMonthly)
		})
	}
}

func TestEstimateCost_VMCacheHit_NoNewMisses(t *testing.T) {
	skipIfDisabled(t)
	t.Cleanup(rateLimitDelay)
	calc, cachedClient := newTestCalculator(t)

	attrs := map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	}

	resp1 := estimateCost(t, calc, "azure:compute/virtualMachine:VirtualMachine", attrs)
	missesAfterFirst := cachedClient.Stats().Misses.Load()
	hitsAfterFirst := cachedClient.Stats().Hits.Load()

	resp2 := estimateCost(t, calc, "azure:compute/virtualMachine:VirtualMachine", attrs)

	hits := cachedClient.Stats().Hits.Load()
	if hits <= hitsAfterFirst {
		t.Errorf("expected cache hits on second call, got hits before=%d after=%d", hitsAfterFirst, hits)
	}

	missesAfterSecond := cachedClient.Stats().Misses.Load()
	if missesAfterSecond != missesAfterFirst {
		t.Errorf("expected no new cache misses, got misses before=%d after=%d",
			missesAfterFirst, missesAfterSecond)
	}

	if resp1.GetCostMonthly() != resp2.GetCostMonthly() {
		t.Errorf("cached response cost mismatch: first=%.4f second=%.4f",
			resp1.GetCostMonthly(), resp2.GetCostMonthly())
	}

	t.Logf("cache hit verified: hits=%d, misses=%d, cost=$%.4f",
		hits, missesAfterSecond, resp2.GetCostMonthly())
}

// --- User Story 2: Managed Disk Estimation (P2) ---

func TestEstimateCost_ManagedDisk_MatchesLiveReference(t *testing.T) {
	skipIfDisabled(t)

	tests := []struct {
		diskType string
		sizeGB   int
		product  string
		tierSKU  string
	}{
		{diskType: "Standard_LRS", sizeGB: 128, product: "Standard HDD Managed Disks", tierSKU: "S10 LRS"},
		{diskType: "Premium_SSD_LRS", sizeGB: 128, product: "Premium SSD Managed Disks", tierSKU: "P10 LRS"},
	}

	for _, tt := range tests {
		t.Run(tt.diskType, func(t *testing.T) {
			t.Cleanup(rateLimitDelay)
			reference := managedDiskRate(t, "eastus", tt.product, tt.tierSKU)
			calc, _ := newTestCalculator(t)

			resp := estimateCost(t, calc, "azure:storage/managedDisk:ManagedDisk", map[string]any{
				"location":  "eastus",
				"disk_type": tt.diskType,
				"size_gb":   tt.sizeGB,
			})
			// Disks are list-price Consumption rows, so they share the VM
			// on-demand STANDARD category check.
			assertStandardUSD(t, resp)

			assertMatchesLive(t, resp.GetCostMonthly(), reference.RetailPrice)
			t.Logf("%s %d GB eastus: $%.4f/month (live %q $%g/month)",
				tt.diskType, tt.sizeGB, resp.GetCostMonthly(), reference.MeterName, reference.RetailPrice)
		})
	}
}

// --- User Story 3: Error Handling (P2) ---

func TestEstimateCost_Error_InvalidSKU(t *testing.T) {
	skipIfDisabled(t)
	t.Cleanup(rateLimitDelay)
	calc, _ := newTestCalculator(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attrs, err := structpb.NewStruct(map[string]any{
		"location": "eastus",
		"vmSize":   "Nonexistent_ZZZZZ_Invalid",
	})
	if err != nil {
		t.Fatalf("failed to create attributes: %v", err)
	}

	_, err = calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
		Attributes:   attrs,
	})
	if err == nil {
		t.Fatal("expected error for invalid SKU, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %s: %s", st.Code(), st.Message())
	}

	t.Logf("invalid SKU correctly returned %s: %s", st.Code(), st.Message())
}

func TestEstimateCost_Error_MissingAttributes(t *testing.T) {
	skipIfDisabled(t)

	// No client needed — request fails at attribute validation before reaching API.
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	calc := pricing.NewCalculator(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attrs, err := structpb.NewStruct(map[string]any{})
	if err != nil {
		t.Fatalf("failed to create attributes: %v", err)
	}

	_, err = calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
		Attributes:   attrs,
	})
	if err == nil {
		t.Fatal("expected error for missing attributes, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s: %s", st.Code(), st.Message())
	}

	t.Logf("missing attrs correctly returned %s: %s", st.Code(), st.Message())

	// No rateLimitDelay — fails before reaching API
}

// --- User Story 4: CI Pipeline Integration (P3) ---

func TestEstimateCost_SkipIntegration(t *testing.T) {
	t.Setenv("SKIP_INTEGRATION", "true")
	skipIfDisabled(t)
	t.Fatal("expected test to be skipped but execution continued")
}
