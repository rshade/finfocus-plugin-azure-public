package azureclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// realEmptyPage is the body Azure returned on 2026-10-03 for the eastus
// Standard_B1s Reservation query, which has no rows.
const realEmptyPage = `{"BillingCurrency":"USD","CustomerEntityId":"Default",` +
	`"CustomerEntityType":"Retail","Items":[],"NextPageLink":null,"Count":0}`

func emptyPageQuery() PriceQuery {
	return PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
		CurrencyCode:  "USD",
		PriceType:     "Reservation",
	}
}

func TestCachedClientGetPrices_RealEmptyPage_CachedAsNotFound(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realEmptyPage))
	}))
	defer server.Close()

	cached := newTestCachedClient(t, newTestClient(t, server.URL), CacheConfig{
		MaxSize: 10, TTL: time.Hour, ExpiresAtTTL: time.Hour, Logger: zerolog.Nop(),
	})
	defer cached.Close()

	for call := 1; call <= 2; call++ {
		if _, err := cached.GetPrices(context.Background(), emptyPageQuery()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("call %d: expected ErrNotFound, got %v", call, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one upstream request for a real empty page, got %d", got)
	}
}

func TestCachedClientGetPrices_NotAPricePage_InvalidResponseNotCached(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "empty object", body: `{}`},
		{name: "null", body: `null`},
		{name: "null items", body: `{"Items":null,"Count":0}`},
		{name: "items not an array", body: `{"Items":{},"Count":0}`},
		{name: "error envelope", body: `{"Error":{"Code":"BadRequest","Message":"Invalid OData parameters supplied"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			cached := newTestCachedClient(t, newTestClient(t, server.URL), CacheConfig{
				MaxSize: 10, TTL: time.Hour, ExpiresAtTTL: time.Hour, Logger: zerolog.Nop(),
			})
			defer cached.Close()

			for call := 1; call <= 2; call++ {
				_, err := cached.GetPrices(context.Background(), emptyPageQuery())
				if !errors.Is(err, ErrInvalidResponse) {
					t.Fatalf("call %d: expected ErrInvalidResponse, got %v", call, err)
				}
				if errors.Is(err, ErrNotFound) {
					t.Fatalf("call %d: a body that is not a price page must not read as not found: %v", call, err)
				}
			}
			if got := calls.Load(); got != 2 {
				t.Fatalf("expected the invalid body to be re-requested, got %d upstream calls", got)
			}
		})
	}
}

func TestCachedClientGetPrices_PaginationLimit_NotCached(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"Items":[],"NextPageLink":"%s/?page=%d","Count":0}`, server.URL, n)
	}))
	defer server.Close()

	cached := newTestCachedClient(t, newTestClient(t, server.URL), CacheConfig{
		MaxSize: 10, TTL: time.Hour, ExpiresAtTTL: time.Hour, Logger: zerolog.Nop(),
	})
	defer cached.Close()

	for call := 1; call <= 2; call++ {
		if _, err := cached.GetPrices(
			context.Background(),
			emptyPageQuery(),
		); !errors.Is(
			err,
			ErrPaginationLimitExceeded,
		) {
			t.Fatalf("call %d: expected ErrPaginationLimitExceeded, got %v", call, err)
		}
	}
	if got, want := calls.Load(), int64(2*MaxPaginationPages); got != want {
		t.Fatalf("expected %d upstream requests (limit not cached), got %d", want, got)
	}
}

func TestCachedClientGetPrices_InternalServerError_NotCached(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	cached := newTestCachedClient(t, newTestClient(t, server.URL), CacheConfig{
		MaxSize: 10, TTL: time.Hour, ExpiresAtTTL: time.Hour, Logger: zerolog.Nop(),
	})
	defer cached.Close()

	for call := 1; call <= 2; call++ {
		if _, err := cached.GetPrices(context.Background(), emptyPageQuery()); !errors.Is(err, ErrRequestFailed) {
			t.Fatalf("call %d: expected ErrRequestFailed, got %v", call, err)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected a 500 to be re-requested, got %d upstream calls", got)
	}
}

func TestCachedClientGetPrices_EmptyPagesThroughNextLink_CachedAsNotFound(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "" {
			_, _ = fmt.Fprintf(w, `{"Items":[],"NextPageLink":"%s/?page=2","Count":0}`, server.URL)
			return
		}
		_, _ = w.Write([]byte(realEmptyPage))
	}))
	defer server.Close()

	cached := newTestCachedClient(t, newTestClient(t, server.URL), CacheConfig{
		MaxSize: 10, TTL: time.Hour, ExpiresAtTTL: time.Hour, Logger: zerolog.Nop(),
	})
	defer cached.Close()

	for call := 1; call <= 2; call++ {
		if _, err := cached.GetPrices(context.Background(), emptyPageQuery()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("call %d: expected ErrNotFound, got %v", call, err)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected two page requests on the first call and none on the second, got %d", got)
	}
}

func TestCachedClientGetPrices_NegativeEntryAfterNegativeTTL_IsAMiss(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realEmptyPage))
	}))
	defer server.Close()

	cached := newTestCachedClient(t, newTestClient(t, server.URL), CacheConfig{
		MaxSize:      10,
		TTL:          time.Hour,
		NegativeTTL:  50 * time.Millisecond,
		ExpiresAtTTL: time.Hour,
		Logger:       zerolog.Nop(),
	})
	defer cached.Close()

	ctx := context.Background()
	for call := 1; call <= 2; call++ {
		if _, err := cached.GetPrices(ctx, emptyPageQuery()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("call %d: expected ErrNotFound, got %v", call, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected the second call inside the negative TTL to be a hit, got %d upstream calls", got)
	}

	time.Sleep(80 * time.Millisecond)

	if _, err := cached.GetPrices(ctx, emptyPageQuery()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after negative TTL: expected ErrNotFound, got %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected an expired negative entry to be re-requested, got %d upstream calls", got)
	}
}

func TestEffectiveNegativeTTL_Config_CappedAtTTL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ttl      time.Duration
		negative time.Duration
		want     time.Duration
	}{
		{name: "default is one hour", ttl: 24 * time.Hour, negative: 0, want: time.Hour},
		{name: "default capped at a shorter ttl", ttl: 10 * time.Minute, negative: 0, want: 10 * time.Minute},
		{name: "explicit value kept", ttl: 24 * time.Hour, negative: 5 * time.Minute, want: 5 * time.Minute},
		{name: "explicit value capped at ttl", ttl: 10 * time.Minute, negative: 2 * time.Hour, want: 10 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := effectiveNegativeTTL(tt.ttl, tt.negative); got != tt.want {
				t.Fatalf("effectiveNegativeTTL(%v, %v) = %v, want %v", tt.ttl, tt.negative, got, tt.want)
			}
		})
	}
}

func TestNewCachedClient_NegativeNegativeTTL_ReturnsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := NewCachedClient(newTestClient(t, "http://127.0.0.1:1"), CacheConfig{
		MaxSize: 10, TTL: time.Hour, NegativeTTL: -time.Second, Logger: zerolog.Nop(),
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestCachedClientGetPrices_DisabledCacheEmptyPage_RequestsEveryTime(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realEmptyPage))
	}))
	defer server.Close()

	cached := newTestCachedClient(t, newTestClient(t, server.URL), CacheConfig{
		MaxSize: 10, TTL: 0, ExpiresAtTTL: time.Hour, Logger: zerolog.Nop(),
	})
	defer cached.Close()

	for call := 1; call <= 2; call++ {
		if _, err := cached.GetPrices(context.Background(), emptyPageQuery()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("call %d: expected ErrNotFound, got %v", call, err)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected a disabled cache to request every time, got %d upstream calls", got)
	}
}

// syncBuffer guards a log buffer that the cache's eviction callback can write
// from another goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestCachedClientEvictionLogging_NegativeEntryLRU_LogsLRUAndNegative(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realEmptyPage))
	}))
	defer server.Close()

	var logs syncBuffer
	cached := newTestCachedClient(t, newTestClient(t, server.URL), CacheConfig{
		MaxSize:      1,
		TTL:          time.Hour,
		ExpiresAtTTL: time.Hour,
		Logger:       zerolog.New(&logs).Level(zerolog.DebugLevel),
	})
	defer cached.Close()

	ctx := context.Background()
	queryA := emptyPageQuery()
	queryB := emptyPageQuery()
	queryB.ArmSkuName = "Standard_D2s_v3"

	if _, err := cached.GetPrices(ctx, queryA); !errors.Is(err, ErrNotFound) {
		t.Fatalf("queryA: expected ErrNotFound, got %v", err)
	}
	if _, err := cached.GetPrices(ctx, queryA); !errors.Is(err, ErrNotFound) {
		t.Fatalf("queryA hit: expected ErrNotFound, got %v", err)
	}
	if _, err := cached.GetPrices(ctx, queryB); !errors.Is(err, ErrNotFound) {
		t.Fatalf("queryB: expected ErrNotFound, got %v", err)
	}

	var sawEviction, sawNegativeHit bool
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		switch entry["message"] {
		case "cache entry evicted":
			sawEviction = true
			if entry["eviction_reason"] != "lru" {
				t.Errorf("expected eviction_reason=lru for a fresh negative entry, got %v", entry["eviction_reason"])
			}
			if entry["negative"] != true {
				t.Errorf("expected negative=true on the eviction log, got %v", entry["negative"])
			}
		case "cache hit":
			if entry["negative"] == true {
				sawNegativeHit = true
			}
		}
	}
	if !sawEviction {
		t.Errorf("expected an eviction log entry, got:\n%s", logs.String())
	}
	if !sawNegativeHit {
		t.Errorf("expected a cache hit log with negative=true, got:\n%s", logs.String())
	}
}
