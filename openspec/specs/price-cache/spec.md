# price-cache Specification

## Purpose

Keep Azure Retail Prices answers in an in-process LRU+TTL cache so repeated quotes do not call
Azure, cache empty answers briefly as not found, never cache failures, and pass a caller-facing
`expires_at` hint to cost responses.

## Requirements

### Requirement: Normalized cache key

`CacheKey` SHALL build `region|armsku|skuname|product|service|currency` from the query, trimmed
and lowercased, keeping empty fields as empty segments. It SHALL append `type=<price type>` and
`api=<api version>` only when those fields are set. Queries that differ only in case, whitespace,
or field order SHALL produce the same key.

Tests: `TestCacheKey`, `TestCacheKeyEquivalentQueriesMatch`

#### Scenario: Normalization

- **WHEN** the query is region ` EastUS `, SKU ` Standard_B1s `, product
  ` Virtual Machines BS Series `, service ` Virtual Machines `, currency ` USD `
- **THEN** the key is `eastus|standard_b1s||virtual machines bs series|virtual machines|usd`

#### Scenario: Price type and API version

- **WHEN** `PriceType` is ` Reservation ` and `APIVersion` is ` 2023-01-01-preview `
- **THEN** the key ends with `|type=reservation|api=2023-01-01-preview`

### Requirement: Cache hits and statistics

`CachedClient.GetPrices` SHALL call Azure once per key while the entry is live and serve later
lookups from memory, counting each lookup in `Stats().Hits` or `Stats().Misses`. It SHALL be safe
for concurrent use: concurrent reads of a primed key make no further upstream call. Repeating one
virtual machine quote concurrently, in process and over gRPC, SHALL keep the hit rate above 0.80.

Tests: `TestCachedClientGetPrices_CacheMissThenHit`, `TestCachedClientConcurrentReads`,
`TestCachedClientConcurrentAccess`, `TestCachedClientConcurrentWrites`, `TestCacheHitRate`,
`TestCacheHitRateOverGRPC`

#### Scenario: Miss then hit

- **WHEN** the same query is sent twice
- **THEN** Azure receives one request
- **AND** stats show 1 miss and 1 hit

### Requirement: Expiry hint propagation

A cached result SHALL carry `ExpiresAt`, and a cache hit SHALL return the same `ExpiresAt` as the
miss that filled it. `GetProjectedCost` and `GetActualCost` results SHALL set `expires_at` from
it, and a repeated `GetProjectedCost` SHALL return the same `expires_at` with no new upstream
request.

Tests: `TestCachedClientGetPrices_CacheMissThenHit`, `TestGetProjectedCostSetsExpiresAtFromCache`,
`TestGetActualCostSetsExpiresAtFromCache`

#### Scenario: Repeated projected cost

- **WHEN** the same virtual machine `GetProjectedCost` request is sent twice
- **THEN** both responses have a non-nil `expires_at` with the same time
- **AND** the second request adds no upstream request

### Requirement: TTL expiry and LRU bound

An entry older than the cache TTL SHALL be a miss. The cache SHALL hold at most `MaxSize` entries
and evict the least recently used entry when a new key arrives at capacity; below capacity,
nothing SHALL be evicted.

Tests: `TestCachedClientGetPrices_TTLExpiryCausesMiss`, `TestCachedClientLRUEviction`,
`TestCachedClientNoEvictionBelowCapacity`

#### Scenario: LRU eviction

- **WHEN** `MaxSize` is 2, keys A and B are filled, A is read again, and C is added
- **THEN** a later lookup of B calls Azure again
- **AND** the cache length stays 2

### Requirement: Failed responses are not cached

The cache SHALL NOT store failures. HTTP 404 (`ErrNotFound`), 429 (`ErrRateLimited`), 503
(`ErrServiceUnavailable`), 400 and 500 (`ErrRequestFailed`), invalid bodies
(`ErrInvalidResponse`), and `ErrPaginationLimitExceeded` SHALL be requested again on the next
lookup with no cache hit. A 200 body is a price page only when it has an `Items` JSON array:
`{}`, `null`, `"Items":null`, a non-array `Items`, and an `{"Error":...}` envelope SHALL be
`ErrInvalidResponse`, not `ErrNotFound`.

Tests: `TestCachedClientGetPrices_ErrorsAreNotCached`,
`TestCachedClientGetPrices_FailedResponses_NotCached`,
`TestCachedClientGetPrices_NotAPricePage_InvalidResponseNotCached`,
`TestCachedClientGetPrices_PaginationLimit_NotCached`,
`TestCachedClientGetPrices_InternalServerError_NotCached`

#### Scenario: Error envelope

- **WHEN** Azure answers 200 with `{"Error":{"Code":"BadRequest",...}}` twice in a row
- **THEN** both lookups return `ErrInvalidResponse` and not `ErrNotFound`
- **AND** Azure receives two requests

### Requirement: Negative caching of empty pages

A 200 answer whose pages hold zero rows, including empty pages reached through `NextPageLink`,
SHALL be cached as a negative entry: a later lookup SHALL count as a hit and return `ErrNotFound`
with the query context, without calling Azure. Negative entries SHALL expire after the negative
TTL, which defaults to one hour, keeps an explicit value, and is capped at the cache TTL. A
negative `NegativeTTL` SHALL be `ErrInvalidConfig`. With the cache disabled (TTL 0), every lookup
SHALL call Azure.

Tests: `TestCachedClientGetPrices_EmptyPage_CachedAsNotFound`,
`TestCachedClientGetPrices_RealEmptyPage_CachedAsNotFound`,
`TestCachedClientGetPrices_EmptyPagesThroughNextLink_CachedAsNotFound`,
`TestCachedClientGetPrices_NegativeEntryAfterNegativeTTL_IsAMiss`,
`TestEffectiveNegativeTTL_Config_CappedAtTTL`,
`TestNewCachedClient_NegativeNegativeTTL_ReturnsInvalidConfig`,
`TestCachedClientGetPrices_DisabledCacheEmptyPage_RequestsEveryTime`

#### Scenario: Empty reservation page

- **WHEN** Azure answers the eastus `Standard_B1s` Reservation query with `"Items":[]` and the
  query is sent twice
- **THEN** both calls return `ErrNotFound` containing `region=eastus`
- **AND** Azure receives one request and stats show 1 hit and 1 miss

#### Scenario: Negative TTL capped

- **WHEN** the cache TTL is 10 minutes and no negative TTL is set
- **THEN** the effective negative TTL is 10 minutes

### Requirement: Eviction logging

Each eviction SHALL log a debug `cache entry evicted` entry with `cache_key` and
`eviction_reason` `lru` or `expired`, and SHALL be safe under concurrent eviction. Eviction and
`cache hit` logs for a negative entry SHALL carry `negative=true`.

Tests: `TestCachedClientEvictionLogging_LRU`, `TestCachedClientEvictionLogging_TTL`,
`TestCachedClientEvictionLogging_ConcurrentSafe`,
`TestCachedClientEvictionLogging_NegativeEntryLRU_LogsLRUAndNegative`

#### Scenario: Negative entry pushed out

- **WHEN** `MaxSize` is 1, an empty-page key is read twice, and a second empty-page key is added
- **THEN** a `cache hit` log has `negative=true`
- **AND** the eviction log has `eviction_reason=lru` and `negative=true`

### Requirement: FINFOCUS_CACHE_TTL

The process SHALL read the cache TTL from `FINFOCUS_CACHE_TTL` as a Go duration. Unset SHALL use
the default TTL, `0s` SHALL return 0 (cache disabled), and an unparsable value (`banana`, `42`,
blanks) or a negative duration SHALL log a warning and use the default TTL.

Tests: `TestParseCacheTTL_ValidDurations`, `TestParseCacheTTL_InvalidFallsBackToDefault`,
`TestParseCacheTTL_UnsetUsesDefault`, `TestParseCacheTTL_ZeroDisablesCache`,
`TestParseCacheTTL_NegativeFallsBackToDefault`

#### Scenario: Invalid value

- **WHEN** `FINFOCUS_CACHE_TTL=-5s`
- **THEN** the TTL is `DefaultCacheConfig().TTL`
- **AND** a warning is logged
