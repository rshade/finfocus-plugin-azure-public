package azureclient

import (
	"context"
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

	statsRequestInterval = 1000
	statsTimeInterval    = 5 * time.Minute
)

// CachedResult wraps a cached pricing result with timestamps used by callers.
type CachedResult struct {
	Items     []PriceItem
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CacheConfig configures CachedClient behavior.
type CacheConfig struct {
	MaxSize      int
	TTL          time.Duration
	ExpiresAtTTL time.Duration
	Logger       zerolog.Logger
}

// DefaultCacheConfig returns cache defaults suitable for production usage.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxSize:      defaultCacheMaxSize,
		TTL:          defaultCacheTTL,
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
	client *Client
	cache  *expirable.LRU[string, CachedResult]
	config CacheConfig
	logger zerolog.Logger
	stats  CacheStats

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

	cc := &CachedClient{
		client:   client,
		config:   config,
		logger:   config.Logger,
		disabled: config.MaxSize == 0 || config.TTL == 0,
	}
	cc.lastStatsNS.Store(time.Now().UnixNano())

	if !cc.disabled {
		cc.cache = expirable.NewLRU[string, CachedResult](config.MaxSize, nil, config.TTL)
	}

	return cc, nil
}

// GetPrices returns cached pricing data when available and fresh.
func (cc *CachedClient) GetPrices(ctx context.Context, query PriceQuery) (CachedResult, error) {
	key := CacheKey(query)

	if !cc.disabled {
		if cached, ok := cc.cache.Get(key); ok {
			cc.recordHit(key)
			return cloneCachedResult(cached), nil
		}
	}
	cc.recordMiss(key)

	items, err := cc.client.GetPrices(ctx, query)
	if err != nil {
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

func (cc *CachedClient) recordHit(key string) {
	hits := cc.stats.Hits.Add(1)
	total := cc.requests.Add(1)

	cc.logger.Debug().
		Str("cache_key", key).
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
