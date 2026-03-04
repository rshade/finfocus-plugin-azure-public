# Research: Cache Layer Completion

## R1: golang-lru/v2 Eviction Callback API

**Decision**: Use the `onEvict func(key K, value V)` parameter in
`expirable.NewLRU` constructor.

**Rationale**: The constructor signature is
`NewLRU[K, V](size int, onEvict func(K, V), ttl time.Duration)`.
Currently `nil` is passed (cache.go:90). Passing a non-nil function
enables eviction logging with zero changes to the library.

**Alternatives considered**:

- Custom wrapper around the LRU: Unnecessary complexity; the callback
  API is sufficient.
- Polling for evictions: Not supported by the library; callbacks are
  the intended mechanism.

**Key finding**: The callback fires for both LRU eviction and TTL
expiry. The library does not pass a reason parameter. Reason must be
inferred from the entry's age.

## R2: Eviction Reason Inference Accuracy

**Decision**: Compare `CachedResult.CreatedAt + config.TTL` against
`time.Now()` to classify eviction as "expired" or "lru".

**Rationale**: The `expirable.LRU` library processes TTL evictions
before LRU evictions during its internal cleanup cycle. When an
entry is evicted and its age exceeds the TTL, it was a TTL eviction.
When evicted before TTL expiry, it was displaced by a newer entry
(LRU).

**Alternatives considered**:

- Always log "evicted" without reason: Loses diagnostic value.
- Track eviction reason in a separate map: Over-engineering for a
  debug log.

**Edge case**: An entry could be evicted by LRU at the exact moment
its TTL expires. In this race, classifying it as "expired" is
acceptable since the entry would have been evicted by TTL moments
later anyway.

## R3: Go Duration Parsing for Env Var

**Decision**: Use `time.ParseDuration` from the Go standard library.

**Rationale**: `time.ParseDuration` supports all needed formats
("10s", "1h", "24h", "500ms") and returns a clear error for invalid
inputs. This is the idiomatic Go approach.

**Alternatives considered**:

- Custom parser with unit suffixes: Unnecessary when stdlib handles it.
- Integer-only (seconds): Less flexible; "24h" is more readable than
  "86400".

**Key finding**: `time.ParseDuration("0s")` returns 0, which triggers
the existing `disabled` flag in `NewCachedClient` (cache.go:85).
No special handling needed for the zero-TTL disable case.

## R4: Env Var Naming Convention

**Decision**: Use `FINFOCUS_CACHE_TTL` as specified in issue #13.

**Rationale**: Follows the existing `FINFOCUS_` prefix convention
established by `FINFOCUS_PLUGIN_PORT` and `FINFOCUS_LOG_LEVEL`.
The `CACHE_TTL` suffix clearly describes the purpose.

**Alternatives considered**:

- `FINFOCUS_CACHE_EXPIRES_AT_TTL` for the caller-facing TTL: Deferred.
  Only the internal L1 TTL is user-configurable for now. The L2
  ExpiresAtTTL is an internal implementation detail.
