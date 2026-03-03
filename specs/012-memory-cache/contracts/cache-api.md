# Cache API Contract

**Feature**: `012-memory-cache`
**Package**: `internal/azureclient`

## Types

### CachedResult

```go
// CachedResult wraps a pricing query result with expiry
// metadata for cache storage and expires_at response
// production.
type CachedResult struct {
    // Items contains the pricing data from the Azure API.
    Items []PriceItem

    // CreatedAt records when this result was cached.
    CreatedAt time.Time

    // ExpiresAt is the expires_at value for gRPC
    // responses (L2 callers). Computed as
    // CreatedAt + ExpiresAtTTL (default 4h).
    // Returned as-is on cache hits so callers see
    // the remaining L2 TTL.
    ExpiresAt time.Time
}
```

### CacheConfig

```go
// CacheConfig configures the in-memory pricing cache.
type CacheConfig struct {
    // MaxSize is the maximum number of cache entries.
    // Default: 1000.
    MaxSize int

    // TTL is the L1 internal cache time-to-live.
    // Default: 24 hours (per constitution Section V).
    // Controls when entries are evicted from the
    // in-memory LRU cache.
    TTL time.Duration

    // ExpiresAtTTL is the TTL used for expires_at
    // timestamps on gRPC responses (L2 hint).
    // Default: 4 hours. Must be <= TTL.
    // Controls how often callers re-fetch data.
    ExpiresAtTTL time.Duration

    // Logger is the structured logger for cache
    // observability. Required.
    Logger zerolog.Logger
}

// DefaultCacheConfig returns a CacheConfig with default
// values (1000 entries, 24h L1 TTL, 4h expires_at TTL).
func DefaultCacheConfig() CacheConfig
```

### CachedClient

```go
// CachedClient wraps an azureclient.Client with an
// in-memory LRU cache. It provides the same GetPrices
// signature but returns cached results when available.
type CachedClient struct {
    // unexported fields:
    // client  *Client
    // cache   *expirable.LRU[string, CachedResult]
    // config  CacheConfig
    // stats   CacheStats
}

// NewCachedClient creates a CachedClient wrapping the
// given Client with the specified cache configuration.
// Returns error if config is invalid (e.g., MaxSize < 0).
func NewCachedClient(
    client *Client,
    config CacheConfig,
) (*CachedClient, error)
```

## Operations

### GetPrices (cache-aware)

```go
// GetPrices queries pricing data, checking the cache
// first. On cache hit, returns the cached result with
// its original ExpiresAt. On cache miss, delegates to
// the underlying Client, caches the result, and returns
// it with a fresh ExpiresAt.
//
// Errors from the Azure API are never cached (FR-015).
// The next call for the same key retries the API.
//
// Returns CachedResult instead of raw []PriceItem so
// the caller can read ExpiresAt for gRPC responses.
func (cc *CachedClient) GetPrices(
    ctx context.Context,
    query PriceQuery,
) (CachedResult, error)
```

**Behavior**:

1. Compute normalized cache key from `PriceQuery`
2. Check LRU cache for key
3. **Cache hit**: Log at debug level, increment hit counter,
   return `CachedResult` with stored `ExpiresAt`
4. **Cache miss**: Log at debug level, increment miss
   counter, call `Client.GetPrices`, wrap in
   `CachedResult{Items, CreatedAt: now,
   ExpiresAt: now+ExpiresAtTTL}`, store in cache, return
5. **API error**: Do not cache. Return error directly.
6. **Stats check**: Every 1000 requests or 5 minutes, log
   hit/miss ratio at info level

### CacheKey (normalization)

```go
// CacheKey produces a normalized cache key from a
// PriceQuery. The key is a pipe-delimited, lowercased
// string in fixed order:
// {region}|{sku}|{product}|{service}|{currency}
//
// Empty fields produce empty segments. The same key is
// produced regardless of whether the query comes from a
// single-resource or batch operation.
func CacheKey(query PriceQuery) string
```

### Stats

```go
// Stats returns the current cache hit/miss counters.
func (cc *CachedClient) Stats() CacheStats

// Len returns the current number of entries in the cache.
func (cc *CachedClient) Len() int
```

### Close

```go
// Close releases cache resources and the underlying
// Client's connection pool.
func (cc *CachedClient) Close()
```

## Integration with Calculator

The `pricing.Calculator` is modified to:

1. Accept `*CachedClient` instead of constructing its own
   `*Client`
2. Call `CachedClient.GetPrices` which returns
   `CachedResult`
3. Read `CachedResult.ExpiresAt` and set it on gRPC
   response messages via SDK helpers:
   - `pluginsdk.WithProjectedCostExpiresAt` for projected
   - Direct field assignment for `ActualCostResult`
4. Map errors via existing `MapToGRPCStatus`

## Integration with main.go

```text
main.go construction flow:

1. Create azureclient.Client (existing)
2. Create azureclient.CachedClient wrapping Client
3. Create pricing.Calculator with CachedClient + logger
4. Pass Calculator to pluginsdk.Serve
```
