# Data Model: Thread-Safe In-Memory Cache

**Feature**: `012-memory-cache`
**Date**: 2026-03-02

## Entities

### CachedResult

Wraps a pricing query result with expiry metadata. This is the
value type stored in the LRU cache.

| Field     | Type        | Description                  |
| --------- | ----------- | ---------------------------- |
| Items     | []PriceItem | Pricing data from Azure API  |
| CreatedAt | time.Time   | When this result was cached  |
| ExpiresAt | time.Time   | expires_at value for L2      |

**Lifecycle**: Created on cache miss (fresh API result) ->
Served on cache hit -> Evicted by LRU (24h) or TTL expiry.

**Invariants**:

- `Items` is never nil (may be empty slice for valid
  empty results)
- `CreatedAt` is always `time.Now()` at creation
- `ExpiresAt` is `CreatedAt + expiresAtTTL` (4h default)
- `ExpiresAt` is returned as-is on cache hits (remaining
  L2 TTL, not refreshed)
- `ExpiresAt` is always <= the L1 cache entry's internal
  expiry time

### CacheKey

A normalized string derived from `PriceQuery` dimensions.

**Format**: `{region}|{sku}|{product}|{service}|{currency}`

**Normalization rules**:

1. All values lowercased via `strings.ToLower`
2. Fields joined in fixed canonical order with `|` separator
3. Empty fields produce empty segments (e.g.,
   `eastus|standard_b1s||virtual machines|usd`)
4. Leading/trailing whitespace trimmed from each field

**Examples**:

```text
Query: Region=eastus, SKU=Standard_B1s, Svc=VM, USD
Key:   eastus|standard_b1s||virtual machines|usd

Query: Region=WestUS2, Service=Storage
Key:   westus2|||storage|

Query: All fields populated
Key:   eastus|standard_b1s|product|vm|usd
```

### CacheConfig

Configuration for cache construction.

| Field        | Type           | Default | Description         |
| ------------ | -------------- | ------- | ------------------- |
| MaxSize      | int            | 1000    | Max entries         |
| TTL          | time.Duration  | 24h     | L1 internal expiry  |
| ExpiresAtTTL | time.Duration  | 4h      | L2 response hint    |
| Logger       | zerolog.Logger | —       | Structured logger   |

### CacheStats

Atomic counters for observability.

| Field  | Type         | Description                    |
| ------ | ------------ | ------------------------------ |
| Hits   | atomic.Int64 | Total cache hit count          |
| Misses | atomic.Int64 | Total cache miss count         |

**Reporting**: Logged every 1000 requests or 5 minutes
(whichever comes first), per constitution Section III.

## Relationships

```text
Calculator (pricing)
  ├── uses CachedClient (azureclient)
  │     ├── has LRU[string, CachedResult]
  │     ├── has CacheConfig
  │     ├── has CacheStats
  │     └── delegates to Client (azureclient)
  │           └── calls Azure Retail Prices API
  └── sets expires_at on gRPC responses from
      CachedResult.ExpiresAt
```

## State Transitions

```text
Cache Entry Lifecycle:

  [miss] ──→ API call ──→ [stored]
                              │
                    ┌─────────┼──────────┐
                    │         │          │
                 [hit]    [expired]   [evicted]
                    │         │          │
                 promote   remove     remove
                 in LRU   on access   by LRU
```
