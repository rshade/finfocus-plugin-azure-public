package azureclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestCachedClientGetPrices_CacheMissThenHit(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		resp := PriceResponse{
			Items: []PriceItem{
				{
					ArmRegionName: "eastus",
					ArmSkuName:    "Standard_B1s",
					ServiceName:   "Virtual Machines",
					CurrencyCode:  "USD",
					RetailPrice:   0.0104,
				},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	baseClient := newTestClient(t, server.URL)
	cached := newTestCachedClient(t, baseClient, CacheConfig{
		MaxSize:      100,
		TTL:          time.Hour,
		ExpiresAtTTL: 4 * time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	query := PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
		ServiceName:   "Virtual Machines",
		CurrencyCode:  "USD",
	}

	first, err := cached.GetPrices(context.Background(), query)
	if err != nil {
		t.Fatalf("first GetPrices() unexpected error: %v", err)
	}
	second, err := cached.GetPrices(context.Background(), query)
	if err != nil {
		t.Fatalf("second GetPrices() unexpected error: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly one upstream API call, got %d", got)
	}
	if len(first.Items) != 1 || len(second.Items) != 1 {
		t.Fatalf("expected cached result with 1 item, got %d and %d", len(first.Items), len(second.Items))
	}
	if !first.ExpiresAt.Equal(second.ExpiresAt) {
		t.Fatalf("cache hit should preserve original ExpiresAt: first=%s second=%s", first.ExpiresAt, second.ExpiresAt)
	}

	stats := cached.Stats()
	if got := stats.Misses.Load(); got != 1 {
		t.Fatalf("expected 1 cache miss, got %d", got)
	}
	if got := stats.Hits.Load(); got != 1 {
		t.Fatalf("expected 1 cache hit, got %d", got)
	}
}

func TestCachedClientGetPrices_TTLExpiryCausesMiss(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		resp := PriceResponse{
			Items: []PriceItem{
				{ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", CurrencyCode: "USD", RetailPrice: 0.0104},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	baseClient := newTestClient(t, server.URL)
	cached := newTestCachedClient(t, baseClient, CacheConfig{
		MaxSize:      100,
		TTL:          50 * time.Millisecond,
		ExpiresAtTTL: 4 * time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	query := PriceQuery{ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", CurrencyCode: "USD"}
	if _, err := cached.GetPrices(context.Background(), query); err != nil {
		t.Fatalf("first GetPrices() unexpected error: %v", err)
	}

	time.Sleep(70 * time.Millisecond)

	if _, err := cached.GetPrices(context.Background(), query); err != nil {
		t.Fatalf("second GetPrices() unexpected error: %v", err)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected two upstream calls after TTL expiry, got %d", got)
	}
}

func TestCachedClientGetPrices_ErrorsAreNotCached(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("upstream failure"))
			return
		}

		resp := PriceResponse{
			Items: []PriceItem{
				{ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", CurrencyCode: "USD", RetailPrice: 0.0104},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	baseClient := newTestClient(t, server.URL)
	cached := newTestCachedClient(t, baseClient, CacheConfig{
		MaxSize:      100,
		TTL:          time.Hour,
		ExpiresAtTTL: 4 * time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	query := PriceQuery{ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", CurrencyCode: "USD"}

	_, err := cached.GetPrices(context.Background(), query)
	if err == nil {
		t.Fatal("expected first call to return upstream error")
	}
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("expected ErrRequestFailed, got %v", err)
	}

	second, err := cached.GetPrices(context.Background(), query)
	if err != nil {
		t.Fatalf("expected second call to succeed, got %v", err)
	}
	if len(second.Items) != 1 {
		t.Fatalf("expected 1 item on second call, got %d", len(second.Items))
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected second call to hit upstream (error not cached), got %d calls", got)
	}
}

func TestCachedClientConcurrentAccess(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		resp := PriceResponse{
			Items: []PriceItem{
				{ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", CurrencyCode: "USD", RetailPrice: 0.0104},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	baseClient := newTestClient(t, server.URL)
	cached := newTestCachedClient(t, baseClient, CacheConfig{
		MaxSize:      1000,
		TTL:          time.Hour,
		ExpiresAtTTL: 4 * time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	ctx := context.Background()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			query := PriceQuery{
				ArmRegionName: "eastus",
				ArmSkuName:    "standard_b1s",
				ServiceName:   "vm",
				ProductName:   "product",
				CurrencyCode:  "usd",
			}
			if i%2 == 0 {
				query.ArmSkuName = "standard_b2s"
			}
			_, err := cached.GetPrices(ctx, query)
			if err != nil {
				t.Errorf("GetPrices() error: %v", err)
			}
		}(i)
	}
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			query := PriceQuery{
				ArmRegionName: "westus2",
				ArmSkuName:    "standard_f2s_v2",
				ServiceName:   "vm",
				ProductName:   "product",
				CurrencyCode:  "usd",
			}
			if i%2 == 0 {
				query.ArmRegionName = "eastus"
			}
			_, err := cached.GetPrices(ctx, query)
			if err != nil {
				t.Errorf("GetPrices() error: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

func TestCachedClientConcurrentReads(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		resp := PriceResponse{
			Items: []PriceItem{
				{ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", CurrencyCode: "USD", RetailPrice: 0.0104},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	baseClient := newTestClient(t, server.URL)
	cached := newTestCachedClient(t, baseClient, CacheConfig{
		MaxSize:      1000,
		TTL:          time.Hour,
		ExpiresAtTTL: 4 * time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	query := PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "standard_b1s",
		ServiceName:   "virtual machines",
		CurrencyCode:  "usd",
	}

	if _, err := cached.GetPrices(context.Background(), query); err != nil {
		t.Fatalf("prime cache failed: %v", err)
	}

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cached.GetPrices(context.Background(), query); err != nil {
				t.Errorf("GetPrices() error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected only one upstream call for concurrent reads, got %d", got)
	}
}

func TestCachedClientConcurrentWrites(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		resp := PriceResponse{
			Items: []PriceItem{
				{ArmRegionName: "eastus", ArmSkuName: "sku", CurrencyCode: "USD", RetailPrice: 0.0104},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	baseClient := newTestClient(t, server.URL)
	cached := newTestCachedClient(t, baseClient, CacheConfig{
		MaxSize:      1000,
		TTL:          time.Hour,
		ExpiresAtTTL: 4 * time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			query := PriceQuery{
				ArmRegionName: "eastus",
				ArmSkuName:    "standard_b1s-" + string(rune('a'+(i%26))),
				ServiceName:   "virtual machines",
				CurrencyCode:  "usd",
			}
			if _, err := cached.GetPrices(context.Background(), query); err != nil {
				t.Errorf("GetPrices() error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if cached.Len() == 0 {
		t.Fatal("expected cache to contain entries after concurrent writes")
	}
}

func TestCachedClientLRUEviction(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		resp := PriceResponse{
			Items: []PriceItem{
				{
					ArmRegionName: "eastus",
					ArmSkuName:    r.URL.Query().Get("$filter"),
					CurrencyCode:  "USD",
					RetailPrice:   0.0104,
				},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	baseClient := newTestClient(t, server.URL)
	cached := newTestCachedClient(t, baseClient, CacheConfig{
		MaxSize:      2,
		TTL:          time.Hour,
		ExpiresAtTTL: time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	queryA := PriceQuery{ArmRegionName: "eastus", ArmSkuName: "A", CurrencyCode: "USD"}
	queryB := PriceQuery{ArmRegionName: "eastus", ArmSkuName: "B", CurrencyCode: "USD"}
	queryC := PriceQuery{ArmRegionName: "eastus", ArmSkuName: "C", CurrencyCode: "USD"}

	if _, err := cached.GetPrices(context.Background(), queryA); err != nil {
		t.Fatalf("queryA failed: %v", err)
	}
	if _, err := cached.GetPrices(context.Background(), queryB); err != nil {
		t.Fatalf("queryB failed: %v", err)
	}
	if cached.Len() != 2 {
		t.Fatalf("expected cache len 2, got %d", cached.Len())
	}

	// Promote A to make B the LRU entry.
	if _, err := cached.GetPrices(context.Background(), queryA); err != nil {
		t.Fatalf("queryA hit failed: %v", err)
	}
	if _, err := cached.GetPrices(context.Background(), queryC); err != nil {
		t.Fatalf("queryC failed: %v", err)
	}

	// B should be evicted, forcing another upstream call.
	before := calls.Load()
	if _, err := cached.GetPrices(context.Background(), queryB); err != nil {
		t.Fatalf("queryB after eviction failed: %v", err)
	}
	after := calls.Load()
	if after != before+1 {
		t.Fatalf("expected queryB miss after eviction; upstream calls before=%d after=%d", before, after)
	}

	if cached.Len() != 2 {
		t.Fatalf("expected bounded cache len 2, got %d", cached.Len())
	}
}

func TestCachedClientNoEvictionBelowCapacity(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		resp := PriceResponse{
			Items: []PriceItem{
				{ArmRegionName: "eastus", ArmSkuName: "sku", CurrencyCode: "USD", RetailPrice: 0.0104},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	baseClient := newTestClient(t, server.URL)
	cached := newTestCachedClient(t, baseClient, CacheConfig{
		MaxSize:      3,
		TTL:          time.Hour,
		ExpiresAtTTL: 4 * time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	queryA := PriceQuery{ArmRegionName: "eastus", ArmSkuName: "A", CurrencyCode: "USD"}
	queryB := PriceQuery{ArmRegionName: "eastus", ArmSkuName: "B", CurrencyCode: "USD"}

	if _, err := cached.GetPrices(context.Background(), queryA); err != nil {
		t.Fatalf("queryA failed: %v", err)
	}
	if _, err := cached.GetPrices(context.Background(), queryB); err != nil {
		t.Fatalf("queryB failed: %v", err)
	}
	if cached.Len() != 2 {
		t.Fatalf("expected len 2 below capacity, got %d", cached.Len())
	}

	before := calls.Load()
	if _, err := cached.GetPrices(context.Background(), queryA); err != nil {
		t.Fatalf("queryA repeat failed: %v", err)
	}
	after := calls.Load()
	if after != before {
		t.Fatalf("expected queryA repeat to hit cache below capacity: before=%d after=%d", before, after)
	}
}

func BenchmarkCachedClientGetPrices(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := PriceResponse{
			Items: []PriceItem{
				{ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", CurrencyCode: "USD", RetailPrice: 0.0104},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	baseClient := benchmarkClient(b, server.URL)
	cached, err := NewCachedClient(baseClient, CacheConfig{
		MaxSize:      1000,
		TTL:          time.Hour,
		ExpiresAtTTL: 4 * time.Hour,
		Logger:       zerolog.Nop(),
	})
	if err != nil {
		b.Fatalf("NewCachedClient() error: %v", err)
	}
	defer cached.Close()

	query := PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "standard_b1s",
		ServiceName:   "virtual machines",
		CurrencyCode:  "usd",
	}

	// Prime cache before timing to measure hit performance.
	if _, err := cached.GetPrices(context.Background(), query); err != nil {
		b.Fatalf("cache prime failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cached.GetPrices(context.Background(), query); err != nil {
			b.Fatalf("GetPrices() error: %v", err)
		}
	}
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()

	cfg := DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.RetryMax = 0
	cfg.RetryWaitMin = time.Millisecond
	cfg.RetryWaitMax = 5 * time.Millisecond
	cfg.Timeout = 3 * time.Second
	cfg.Logger = zerolog.Nop()

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	return client
}

func newTestCachedClient(t *testing.T, client *Client, cfg CacheConfig) *CachedClient {
	t.Helper()

	cached, err := NewCachedClient(client, cfg)
	if err != nil {
		t.Fatalf("NewCachedClient() error: %v", err)
	}
	return cached
}

func benchmarkClient(b *testing.B, baseURL string) *Client {
	b.Helper()

	cfg := DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.RetryMax = 0
	cfg.RetryWaitMin = time.Millisecond
	cfg.RetryWaitMax = 5 * time.Millisecond
	cfg.Timeout = 3 * time.Second
	cfg.Logger = zerolog.Nop()

	client, err := NewClient(cfg)
	if err != nil {
		b.Fatalf("NewClient() error: %v", err)
	}
	return client
}
