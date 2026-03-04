# Data Model: Cache Layer Completion

No new entities are introduced. This feature modifies the behavior of
existing entities only.

## Modified Entities

### CachedClient (internal/azureclient/cache.go)

**Current state**: `expirable.NewLRU` constructed with `nil` eviction
callback.

**Change**: Constructor receives a non-nil eviction callback closure
that logs evicted entries at debug level.

**New behavior**: When any entry is evicted (by TTL or LRU), the
callback emits a structured log with:

- `cache_key`: The evicted entry's normalized cache key
- `eviction_reason`: "expired" or "lru" (inferred from entry age)

### CacheConfig (internal/azureclient/cache.go)

**No structural change**. The `TTL` field already exists and is
configurable. The env var override sets this field before construction.

## Data Flow

```text
Startup:
  main.go reads FINFOCUS_CACHE_TTL env var
    → parses with time.ParseDuration
    → sets cacheConfig.TTL (or keeps default on error)
    → passes to NewCachedClient

Eviction:
  expirable.LRU evicts entry (TTL expiry or LRU overflow)
    → calls onEvict(key, value) callback
    → callback infers reason from value.CreatedAt + config.TTL
    → callback logs at debug level via zerolog
```
