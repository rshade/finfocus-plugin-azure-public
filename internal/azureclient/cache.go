package azureclient

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/rs/zerolog"
)

const (
	defaultCacheMaxSize = 1000
	defaultCacheTTL     = 24 * time.Hour
	defaultExpiresAtTTL = 4 * time.Hour
	defaultNegativeTTL  = time.Hour

	statsRequestInterval = 1000
	statsTimeInterval    = 5 * time.Minute
)

// CachedResult wraps a cached pricing result with timestamps used by callers.
type CachedResult struct {
	Items     []PriceItem
	CreatedAt time.Time
	ExpiresAt time.Time

	noData bool
}

// CacheConfig configures CachedClient behavior.
//
// NegativeTTL bounds how long an empty price page (an HTTP 200 with zero
// rows) is remembered. Zero means one hour. The value is capped at TTL.
type CacheConfig struct {
	MaxSize      int
	TTL          time.Duration
	NegativeTTL  time.Duration
	ExpiresAtTTL time.Duration
	Logger       zerolog.Logger
}

// DefaultCacheConfig returns cache defaults suitable for production usage.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxSize:      defaultCacheMaxSize,
		TTL:          defaultCacheTTL,
		NegativeTTL:  defaultNegativeTTL,
		ExpiresAtTTL: defaultExpiresAtTTL,
		Logger:       zerolog.Nop(),
	}
}

// CacheStats tracks cache hits and misses.
type CacheStats struct {
	Hits   atomic.Int64
	Misses atomic.Int64
}

// CachedClient wraps Client with an in-memory thread-safe LRU cache.
type CachedClient struct {
	client      *Client
	cache       *expirable.LRU[string, CachedResult]
	config      CacheConfig
	negativeTTL time.Duration
	logger      zerolog.Logger
	stats       CacheStats

	requests    atomic.Int64
	lastStatsNS atomic.Int64
	disabled    bool
}

// NewCachedClient creates a cache wrapper around a pricing client.
func NewCachedClient(client *Client, config CacheConfig) (*CachedClient, error) {
	if client == nil {
		return nil, fmt.Errorf("%w: client is required", ErrInvalidConfig)
	}
	if config.MaxSize < 0 {
		return nil, fmt.Errorf("%w: MaxSize must be >= 0", ErrInvalidConfig)
	}
	if config.TTL < 0 {
		return nil, fmt.Errorf("%w: TTL must be >= 0", ErrInvalidConfig)
	}
	if config.ExpiresAtTTL < 0 {
		return nil, fmt.Errorf("%w: ExpiresAtTTL must be >= 0", ErrInvalidConfig)
	}
	if config.NegativeTTL < 0 {
		return nil, fmt.Errorf("%w: NegativeTTL must be >= 0", ErrInvalidConfig)
	}

	cc := &CachedClient{
		client:      client,
		config:      config,
		negativeTTL: effectiveNegativeTTL(config.TTL, config.NegativeTTL),
		logger:      config.Logger,
		disabled:    config.MaxSize == 0 || config.TTL == 0,
	}
	cc.lastStatsNS.Store(time.Now().UnixNano())

	if !cc.disabled {
		onEvict := func(key string, value CachedResult) {
			reason := "lru"
			if time.Since(value.CreatedAt) >= cc.entryTTL(value) {
				reason = "expired"
			}
			cc.logger.Debug().
				Str("cache_key", key).
				Str("eviction_reason", reason).
				Bool("negative", value.noData).
				Msg("cache entry evicted")
		}
		cc.cache = expirable.NewLRU[string, CachedResult](config.MaxSize, onEvict, config.TTL)
	}

	return cc, nil
}

// GetPrices returns cached pricing data when available and fresh.
//
// An empty price page (HTTP 200, zero rows) is cached as a negative entry for
// the negative TTL and returned as ErrNotFound. Failed requests are never
// cached.
func (cc *CachedClient) GetPrices(ctx context.Context, query PriceQuery) (CachedResult, error) {
	key := CacheKey(query)

	if !cc.disabled {
		if cached, ok := cc.lookup(key); ok {
			cc.recordHit(key, cached.noData)
			if cached.noData {
				return CachedResult{}, fmt.Errorf("%s: %w", formatQueryContext(query), errNoPricingData)
			}
			return cloneCachedResult(cached), nil
		}
	}
	cc.recordMiss(key)

	items, err := cc.client.GetPrices(ctx, query)
	if err != nil {
		if !cc.disabled && errors.Is(err, errNoPricingData) {
			cc.cache.Add(key, CachedResult{noData: true, CreatedAt: time.Now()})
		}
		return CachedResult{}, err
	}

	now := time.Now()
	result := CachedResult{
		Items:     clonePriceItems(items),
		CreatedAt: now,
		ExpiresAt: now.Add(cc.config.ExpiresAtTTL),
	}

	if !cc.disabled {
		// Ensure caller-facing expiry never exceeds L1 cache lifetime.
		if cc.config.TTL > 0 {
			internalExpiry := now.Add(cc.config.TTL)
			if result.ExpiresAt.After(internalExpiry) {
				result.ExpiresAt = internalExpiry
			}
		}
		cc.cache.Add(key, cloneCachedResult(result))
	}

	return result, nil
}

// lookup returns a fresh cache entry. A negative entry older than the
// negative TTL is removed and reported as absent.
func (cc *CachedClient) lookup(key string) (CachedResult, bool) {
	cached, ok := cc.cache.Get(key)
	if !ok {
		return CachedResult{}, false
	}
	if cached.noData && time.Since(cached.CreatedAt) >= cc.negativeTTL {
		cc.cache.Remove(key)
		return CachedResult{}, false
	}
	return cached, true
}

func (cc *CachedClient) entryTTL(value CachedResult) time.Duration {
	if value.noData {
		return cc.negativeTTL
	}
	return cc.config.TTL
}

// effectiveNegativeTTL applies the one-hour default and caps the result at ttl.
func effectiveNegativeTTL(ttl, negative time.Duration) time.Duration {
	if negative <= 0 {
		negative = defaultNegativeTTL
	}
	return min(negative, ttl)
}

// Stats returns cache hit/miss counters.
func (cc *CachedClient) Stats() *CacheStats {
	return &cc.stats
}

// Len returns the number of items currently stored in cache.
func (cc *CachedClient) Len() int {
	if cc.disabled || cc.cache == nil {
		return 0
	}
	return cc.cache.Len()
}

// Close releases the cache and underlying HTTP client resources.
func (cc *CachedClient) Close() {
	if cc.cache != nil {
		cc.cache.Purge()
	}
	cc.client.Close()
}

func (cc *CachedClient) recordHit(key string, negative bool) {
	hits := cc.stats.Hits.Add(1)
	total := cc.requests.Add(1)

	cc.logger.Debug().
		Str("cache_key", key).
		Bool("negative", negative).
		Msg("cache hit")

	cc.maybeLogStats(total, hits)
}

func (cc *CachedClient) recordMiss(key string) {
	hits := cc.stats.Hits.Load()
	total := cc.requests.Add(1)
	cc.stats.Misses.Add(1)

	cc.logger.Debug().
		Str("cache_key", key).
		Msg("cache miss")

	cc.maybeLogStats(total, hits)
}

func (cc *CachedClient) maybeLogStats(total, hits int64) {
	nowNS := time.Now().UnixNano()
	lastNS := cc.lastStatsNS.Load()

	logForTime := nowNS-lastNS >= statsTimeInterval.Nanoseconds()
	logForCount := total%statsRequestInterval == 0
	if !logForTime && !logForCount {
		return
	}

	if logForTime {
		if !cc.lastStatsNS.CompareAndSwap(lastNS, nowNS) {
			return
		}
	} else {
		cc.lastStatsNS.Store(nowNS)
	}

	misses := cc.stats.Misses.Load()
	denom := hits + misses

	var ratio float64
	if denom > 0 {
		ratio = float64(hits) / float64(denom)
	}

	cc.logger.Info().
		Int64("cache_hits", hits).
		Int64("cache_misses", misses).
		Float64("cache_hit_ratio", ratio).
		Int("cache_size", cc.Len()).
		Msg("cache stats")
}

func cloneCachedResult(in CachedResult) CachedResult {
	in.Items = clonePriceItems(in.Items)
	return in
}

func clonePriceItems(items []PriceItem) []PriceItem {
	if len(items) == 0 {
		return []PriceItem{}
	}
	cloned := make([]PriceItem, len(items))
	copy(cloned, items)
	return cloned
}
