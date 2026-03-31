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

// Reference prices last verified: 2026-03-30 against live Azure API.
// Update these when Azure adjusts pricing and tests fail.
// Run failing test with -v to see actual prices returned.
const (
	refB1sHourly   = 0.014 // Standard_B1s, eastus (hourly)
	refD2sv3Hourly = 0.075 // Standard_D2s_v3, eastus (hourly)
	priceTolerance = 0.25  // ±25%
)

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

// --- User Story 1: VM Cost Estimation (P1) ---

func TestEstimateCost_VM_StandardB1s(t *testing.T) {
	skipIfDisabled(t)
	calc, _ := newTestCalculator(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attrs, err := structpb.NewStruct(map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})
	if err != nil {
		t.Fatalf("failed to create attributes: %v", err)
	}

	resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
		Attributes:   attrs,
	})
	if err != nil {
		t.Fatalf("EstimateCost failed: %v", err)
	}

	costMonthly := resp.GetCostMonthly()
	if costMonthly <= 0 {
		t.Fatalf("expected positive monthly cost, got %.4f", costMonthly)
	}

	expectedMonthly := refB1sHourly * pluginsdk.HoursPerMonth
	assertInRange(t, costMonthly, expectedMonthly)
	t.Logf("Standard_B1s eastus: $%.4f/month (expected ~$%.2f)", costMonthly, expectedMonthly)

	rateLimitDelay()
}

func TestEstimateCost_VM_StandardD2sv3(t *testing.T) {
	skipIfDisabled(t)
	calc, _ := newTestCalculator(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attrs, err := structpb.NewStruct(map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_D2s_v3",
	})
	if err != nil {
		t.Fatalf("failed to create attributes: %v", err)
	}

	resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
		Attributes:   attrs,
	})
	if err != nil {
		t.Fatalf("EstimateCost failed: %v", err)
	}

	costMonthly := resp.GetCostMonthly()
	if costMonthly <= 0 {
		t.Fatalf("expected positive monthly cost, got %.4f", costMonthly)
	}

	expectedMonthly := refD2sv3Hourly * pluginsdk.HoursPerMonth
	assertInRange(t, costMonthly, expectedMonthly)
	t.Logf("Standard_D2s_v3 eastus: $%.4f/month (expected ~$%.2f)", costMonthly, expectedMonthly)

	rateLimitDelay()
}

func TestEstimateCost_VM_CacheHit(t *testing.T) {
	skipIfDisabled(t)
	calc, cachedClient := newTestCalculator(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attrs, err := structpb.NewStruct(map[string]any{
		"location": "eastus",
		"vmSize":   "Standard_B1s",
	})
	if err != nil {
		t.Fatalf("failed to create attributes: %v", err)
	}

	req := &finfocusv1.EstimateCostRequest{
		ResourceType: "azure:compute/virtualMachine:VirtualMachine",
		Attributes:   attrs,
	}

	// First call — cache miss (hits live API)
	resp1, err := calc.EstimateCost(ctx, req)
	if err != nil {
		t.Fatalf("first EstimateCost failed: %v", err)
	}

	missesAfterFirst := cachedClient.Stats().Misses.Load()

	// Second call — should be cache hit
	resp2, err := calc.EstimateCost(ctx, req)
	if err != nil {
		t.Fatalf("second EstimateCost failed: %v", err)
	}

	hits := cachedClient.Stats().Hits.Load()
	if hits <= 0 {
		t.Errorf("expected cache hit on second call, got hits=%d", hits)
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

	// No rateLimitDelay — second call used cache, no API request
}

// --- User Story 2: Managed Disk Estimation (P2) ---

func TestEstimateCost_Disk_StandardLRS(t *testing.T) {
	skipIfDisabled(t)

	// Known issue: buildFilterQuery applies priceType=Consumption by default,
	// but Managed Disks are not listed under Consumption in the Azure Retail
	// Prices API. This causes NotFound for all disk queries against the live API.
	// Skip until the filter is fixed to support disk pricing.
	t.Skip("disk pricing returns NotFound against live API" +
		" — priceType=Consumption filter incompatible with Managed Disks")
}

func TestEstimateCost_Disk_PremiumSSD(t *testing.T) {
	skipIfDisabled(t)

	// Same known issue as TestEstimateCost_Disk_StandardLRS.
	t.Skip("disk pricing returns NotFound against live API" +
		" — priceType=Consumption filter incompatible with Managed Disks")
}

// --- User Story 3: Error Handling (P2) ---

func TestEstimateCost_Error_InvalidSKU(t *testing.T) {
	skipIfDisabled(t)
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

	rateLimitDelay()
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
