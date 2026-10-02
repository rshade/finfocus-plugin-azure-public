package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

const (
	vmGoldenFixture = "testdata/retail/spot/standard_d2s_v3_eastus.json"
	vmGoldenCost    = "testdata/golden/compute_virtual_machine.txt"

	cacheHitWorkers = 50
	cacheHitRepeats = 8
	cacheHitRateMin = 0.80
)

// TestCacheHitRate repeats one on-demand VM quote concurrently against an
// in-process fixture. The ratio is hits / (hits + misses), including warm-up.
// It is not a hardcoded fraction. The run fails at or below 0.80, and when
// the cache records nothing.
func TestCacheHitRate(t *testing.T) {
	calc, cached, req, want := newVMGoldenQuote(t)

	warm, err := calc.GetProjectedCost(context.Background(), req)
	if err != nil {
		t.Fatalf("warm-up GetProjectedCost() failed: %v", err)
	}
	if math.Abs(warm.GetCostPerMonth()-want) > goldenCostTolerance {
		t.Fatalf("warm-up cost_per_month = %v, want %v", warm.GetCostPerMonth(), want)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var callErr error
	record := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if callErr == nil {
			callErr = err
		}
	}

	for range cacheHitWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range cacheHitRepeats {
				resp, quoteErr := calc.GetProjectedCost(context.Background(), req)
				if quoteErr != nil {
					record(quoteErr)
					return
				}
				if math.Abs(resp.GetCostPerMonth()-want) > goldenCostTolerance {
					record(fmt.Errorf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), want))
					return
				}
			}
		}()
	}
	wg.Wait()
	if callErr != nil {
		t.Fatalf("concurrent GetProjectedCost() failed: %v", callErr)
	}

	hits := cached.Stats().Hits.Load()
	misses := cached.Stats().Misses.Load()
	total := hits + misses
	if total == 0 {
		t.Fatal("cache recorded no hits and no misses")
	}
	ratio := float64(hits) / float64(total)
	t.Logf("cache hits=%d misses=%d total=%d ratio=%.6f", hits, misses, total, ratio)
	if ratio <= cacheHitRateMin {
		t.Fatalf("cache hit rate = %.6f, want > %.2f (hits=%d misses=%d)", ratio, cacheHitRateMin, hits, misses)
	}
}

// TestCacheHitRateOverGRPC drives the same quote through a real gRPC server.
// Disabling the cache drops the hit rate to zero and fails this test.
func TestCacheHitRateOverGRPC(t *testing.T) {
	t.Parallel()

	calc, cached, req, want := newVMGoldenQuote(t)
	client := dialPricingClient(t, calc)

	warm, err := client.GetProjectedCost(context.Background(), req)
	if err != nil {
		t.Fatalf("warm-up GetProjectedCost() failed: %v", err)
	}
	if math.Abs(warm.GetCostPerMonth()-want) > goldenCostTolerance {
		t.Fatalf("warm-up cost_per_month = %v, want %v", warm.GetCostPerMonth(), want)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var callErr error
	record := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if callErr == nil {
			callErr = err
		}
	}

	for range cacheHitWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range cacheHitRepeats {
				resp, quoteErr := client.GetProjectedCost(context.Background(), req)
				if quoteErr != nil {
					record(quoteErr)
					return
				}
				if math.Abs(resp.GetCostPerMonth()-want) > goldenCostTolerance {
					record(fmt.Errorf("cost_per_month = %v, want %v", resp.GetCostPerMonth(), want))
					return
				}
			}
		}()
	}
	wg.Wait()
	if callErr != nil {
		t.Fatalf("concurrent GetProjectedCost() failed: %v", callErr)
	}

	hits := cached.Stats().Hits.Load()
	misses := cached.Stats().Misses.Load()
	total := hits + misses
	if total == 0 {
		t.Fatal("cache recorded no hits and no misses")
	}
	ratio := float64(hits) / float64(total)
	t.Logf("grpc cache hits=%d misses=%d total=%d ratio=%.6f", hits, misses, total, ratio)
	if ratio <= cacheHitRateMin {
		t.Fatalf("cache hit rate = %.6f, want > %.2f (hits=%d misses=%d)", ratio, cacheHitRateMin, hits, misses)
	}
}

// BenchmarkGetProjectedCostCacheHit times the same in-process on-demand VM
// quote as TestCacheHitRate. The timer starts after one prime, so the loop
// is cache hits. The number is a local baseline, not a gate.
func BenchmarkGetProjectedCostCacheHit(b *testing.B) {
	calc, _, req, _ := newVMGoldenQuote(b)

	if _, err := calc.GetProjectedCost(context.Background(), req); err != nil {
		b.Fatalf("prime GetProjectedCost() failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := calc.GetProjectedCost(context.Background(), req); err != nil {
			b.Fatalf("GetProjectedCost() failed: %v", err)
		}
	}
}

// BenchmarkEstimateCostCold times EstimateCost with the cache disabled.
// Every loop calls the in-process fixture. The number is a local baseline.
func BenchmarkEstimateCostCold(b *testing.B) {
	page := readRetailPage(b, vmGoldenFixture)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := azureclient.PriceResponse{Items: page.Items, Count: len(page.Items)}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	b.Cleanup(server.Close)

	clientConfig := azureclient.DefaultConfig()
	clientConfig.BaseURL = server.URL
	clientConfig.RetryMax = 0
	clientConfig.Timeout = 3 * time.Second
	clientConfig.Logger = zerolog.Nop()
	client, err := azureclient.NewClient(clientConfig)
	if err != nil {
		b.Fatalf("NewClient() failed: %v", err)
	}
	b.Cleanup(client.Close)

	cacheConfig := azureclient.DefaultCacheConfig()
	cacheConfig.TTL = 0
	cacheConfig.Logger = zerolog.Nop()
	cached, err := azureclient.NewCachedClient(client, cacheConfig)
	if err != nil {
		b.Fatalf("NewCachedClient() failed: %v", err)
	}
	b.Cleanup(func() { cached.Close() })

	calc := NewCalculator(zerolog.Nop(), cached)
	req := estimateVMRequest(b, "")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := calc.EstimateCost(context.Background(), req); err != nil {
			b.Fatalf("EstimateCost() failed: %v", err)
		}
	}
}

// BenchmarkMapDescriptorToQuery times descriptor mapping with no I/O.
func BenchmarkMapDescriptorToQuery(b *testing.B) {
	desc := &finfocusv1.ResourceDescriptor{
		Provider:     "azure",
		ResourceType: "compute/VirtualMachine",
		Region:       "eastus",
		Sku:          "Standard_B1s",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := MapDescriptorToQuery(desc); err != nil {
			b.Fatal(err)
		}
	}
}

func newVMGoldenQuote(tb testing.TB) (
	*Calculator,
	*azureclient.CachedClient,
	*finfocusv1.GetProjectedCostRequest,
	float64,
) {
	tb.Helper()

	page := readRetailPage(tb, vmGoldenFixture)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := azureclient.PriceResponse{Items: page.Items, Count: len(page.Items)}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	tb.Cleanup(server.Close)

	cached := newLoadCachedClient(tb, server.URL)
	tb.Cleanup(func() { cached.Close() })

	req := vmProjectedRequest(page.Items[0].ArmRegionName, page.Items[0].ArmSkuName, "")
	return NewCalculator(zerolog.Nop(), cached), cached, req, readGoldenNumber(tb, vmGoldenCost)
}

func newLoadCachedClient(tb testing.TB, baseURL string) *azureclient.CachedClient {
	tb.Helper()

	clientConfig := azureclient.DefaultConfig()
	clientConfig.BaseURL = baseURL
	clientConfig.RetryMax = 0
	clientConfig.RetryWaitMin = time.Millisecond
	clientConfig.RetryWaitMax = 5 * time.Millisecond
	clientConfig.Timeout = 3 * time.Second
	clientConfig.Logger = zerolog.Nop()

	client, err := azureclient.NewClient(clientConfig)
	if err != nil {
		tb.Fatalf("NewClient() failed: %v", err)
	}

	cacheConfig := azureclient.DefaultCacheConfig()
	cacheConfig.MaxSize = 100
	cacheConfig.TTL = time.Hour
	cacheConfig.ExpiresAtTTL = 4 * time.Hour
	cacheConfig.Logger = zerolog.Nop()

	cached, err := azureclient.NewCachedClient(client, cacheConfig)
	if err != nil {
		tb.Fatalf("NewCachedClient() failed: %v", err)
	}
	return cached
}

func readRetailPage(tb testing.TB, path string) azureclient.PriceResponse {
	tb.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read fixture: %v", err)
	}
	var resp azureclient.PriceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		tb.Fatalf("unmarshal fixture: %v", err)
	}
	if len(resp.Items) == 0 {
		tb.Fatal("fixture has no items")
	}
	return resp
}

func readGoldenNumber(tb testing.TB, path string) float64 {
	tb.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read golden cost: %v", err)
	}
	text := strings.TrimSpace(string(data))
	cost, err := strconv.ParseFloat(text, 64)
	if err != nil {
		tb.Fatalf("parse golden cost %q: %v", text, err)
	}
	return cost
}
