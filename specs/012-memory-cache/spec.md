# Feature Specification: Thread-Safe In-Memory Cache

**Feature Branch**: `012-memory-cache`
**Created**: 2026-03-02
**Status**: Draft
**Input**: GitHub Issue #12 — Thread-safe in-memory cache for
Azure pricing data

## Architecture Context

This cache operates as **Layer 1 (L1)** in a two-layer caching
architecture:

```text
Caller (CLI / third-party gRPC client)
  L2: caller-side cache (BoltDB, custom, etc.)
      ↕ expires_at on gRPC responses bridges L1 ↔ L2
  gRPC boundary
  L1: plugin in-memory cache (this feature)
      ↕ avoids redundant HTTP calls
  Azure Retail Prices API
```

- **L1 (this feature)**: Ephemeral, in-process. Eliminates
  redundant Azure API calls within the plugin's lifetime.
  Uses `hashicorp/golang-lru/v2/expirable` for thread-safe
  LRU with TTL.
- **L2 (caller-side)**: Persistent or ephemeral, owned by the
  caller. The finfocus CLI uses BoltDB; third-party callers
  may use anything. L2 relies on `expires_at` timestamps in
  gRPC responses to know when cached data is stale.
- **Bridge**: The plugin sets `expires_at` on every cost
  response using a separate, shorter TTL than the L1 cache.
  L1 uses a 24-hour TTL (matching Azure's daily data
  refresh). `expires_at` uses a 4-hour TTL, prompting
  callers to re-fetch sooner. L2 misses that re-enter the
  plugin are served from L1 without hitting Azure. Worst
  case staleness: ~28 hours (24h L1 + 4h L2).

Requires finfocus-spec >= v0.5.7 for `expires_at` fields and
`BatchCost` RPC support.

## Clarifications

### Session 2026-03-02

- Q: When the Azure API fails during a cache miss, what
  should the cache do? → A: Never cache errors; propagate
  directly to the caller and let the next request retry.
- Q: When serving a cache hit, what `expires_at` value
  should the response carry? → A: The entry's original
  expiry time (remaining TTL), not a fresh full TTL.
  Prevents L2 callers from holding data longer than L1.
- Q: What should the default cache TTL be? → A: Two-TTL
  model. L1 internal cache = 24 hours (matches
  constitution and Azure's daily data refresh). L2
  `expires_at` on responses = 4 hours (prompts callers
  to re-fetch periodically). Azure pricing data refreshes
  daily with changes typically on the 1st of the month.
  Worst case staleness ~28 hours.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Cache Pricing Lookups (Priority: P1)

As the Azure pricing client, I want to check if pricing data
for a given query is already cached so that redundant API calls
are eliminated and response times improve for repeated queries.

**Why this priority**: This is the core value proposition.
Without caching, every cost query triggers an HTTP round-trip to
the Azure Retail Prices API, adding latency and risking
rate-limit errors.

**Independent Test**: Can be fully tested by issuing the same
pricing query twice and verifying the second call returns data
without an API request.

**Acceptance Scenarios**:

1. **Given** a cache with no entry for query Q, **When** the
   caller stores result R for query Q, **Then** a subsequent
   lookup for Q returns R.
2. **Given** a cache with an entry for query Q, **When** the
   caller looks up query Q, **Then** the cached result is
   returned immediately without external I/O.
3. **Given** a cache with an entry whose TTL has elapsed,
   **When** the caller looks up that entry, **Then** the cache
   reports a miss (entry not found).

---

### User Story 2 — Concurrent Access Safety (Priority: P1)

As concurrent gRPC handlers, I want safe simultaneous cache
reads and writes so that the system never panics, corrupts data,
or deadlocks under load.

**Why this priority**: The gRPC server handles multiple requests
in parallel. A data race in the cache would cause undefined
behavior or crashes in production.

**Independent Test**: Can be tested by launching 100+ goroutines
performing simultaneous reads and writes and running with the Go
race detector enabled.

**Acceptance Scenarios**:

1. **Given** 100 goroutines reading cached entries
   simultaneously, **When** all goroutines complete, **Then** no
   data race is detected and all returned values are consistent.
2. **Given** 100 goroutines writing different entries
   simultaneously, **When** all goroutines complete, **Then** no
   data race is detected and all entries are stored correctly.
3. **Given** mixed concurrent readers and writers, **When** all
   goroutines complete, **Then** no deadlock occurs and the race
   detector reports no issues.

---

### User Story 3 — Bounded Memory Usage (Priority: P2)

As an operator, I want the cache to enforce a maximum number of
entries so that memory consumption stays predictable and the
process does not grow unbounded over time.

**Why this priority**: Important for production reliability but
secondary to correctness and safety. An unbounded cache on a
long-running server could exhaust memory.

**Independent Test**: Can be tested by filling the cache beyond
its maximum size and verifying the least-recently-used entries
are evicted.

**Acceptance Scenarios**:

1. **Given** a cache at maximum capacity, **When** a new entry
   is stored, **Then** the least-recently-used entry is evicted
   to make room.
2. **Given** a cache at maximum capacity, **When** an existing
   entry is accessed (read), **Then** that entry is promoted as
   recently used and will not be the next eviction candidate.
3. **Given** a cache below maximum capacity, **When** a new
   entry is stored, **Then** no eviction occurs and all existing
   entries remain available.

---

### User Story 4 — Normalized Cache Keys (Priority: P2)

As a developer, I want cache keys to be normalized from query
dimensions so that the same pricing data is matched regardless
of whether it was populated by a single-resource query or a
batch query.

**Why this priority**: Batch and single queries hit the same
Azure data. Without normalized keys, the cache would store
duplicates and batch queries would never benefit from prior
single-query cache hits (or vice versa).

**Independent Test**: Can be tested by storing a result via a
single-resource query key and then looking it up using the
equivalent batch-derived key.

**Acceptance Scenarios**:

1. **Given** a result cached from a single EstimateCost call
   for region R and SKU S, **When** a BatchCost call includes
   the same resource, **Then** the cache returns the previously
   stored result. *(Deferred — validates after BatchCost RPC
   is implemented)*
2. **Given** two queries with the same dimensions but different
   field ordering, **When** both are normalized, **Then** they
   produce the same cache key.

---

### User Story 5 — Freshness Signaling via expires_at (Priority: P2)

As any gRPC caller (finfocus CLI or third-party), I want cost
responses to include an `expires_at` timestamp so that I can
manage my own caller-side cache without guessing when data
becomes stale.

**Why this priority**: Without `expires_at`, callers must either
re-fetch on every call (wasteful) or guess at freshness
(risky). This field enables a clean contract between plugin and
caller regardless of the caller's caching technology.

**Independent Test**: Can be tested by calling EstimateCost and
verifying the response contains an `expires_at` timestamp set
to approximately now + configured TTL.

**Acceptance Scenarios**:

1. **Given** an `expires_at` TTL of 4 hours, **When** the
   plugin returns a fresh (uncached) cost response, **Then**
   the response's `expires_at` is set to approximately now
   plus 4 hours.
2. **Given** a cached entry created at time C with
   `expires_at` TTL of 4h and L1 TTL of 24h, **When** the
   plugin serves that entry at time C+2h, **Then** the
   response's `expires_at` equals C+4h (2 hours remaining),
   not now+4h.
3. **Given** a caller that caches responses using `expires_at`,
   **When** the `expires_at` time passes, **Then** the caller
   knows to re-fetch and receives fresh data from the plugin.
   *(Informational — validates L2 contract, not testable
   within L1 plugin scope)*
4. **Given** a third-party gRPC client with no knowledge of
   finfocus internals, **When** it reads `expires_at` from the
   response proto, **Then** it has enough information to
   implement its own caching strategy.
   *(Informational — validates L2 contract, not testable
   within L1 plugin scope)*

---

### Edge Cases

- What happens when Get is called with an empty string key?
  The cache treats it as a valid key (no special handling).
- What happens when Add is called with the same key twice?
  The second call overwrites the first entry and resets its LRU
  position. The TTL remains the global cache TTL.
- What happens when all entries have the same last-access time?
  The eviction policy selects one deterministically (insertion
  order breaks ties).
- What happens when the cache maximum size is set to zero?
  The cache stores no entries (effectively disabled). Queries
  always fall through to the Azure API.
- What happens when L1 TTL is set to zero?
  Entries expire immediately. Every query hits the Azure API.
  The `expires_at` field still uses the `expires_at` TTL.
- What happens when `expires_at` TTL is set to zero?
  Responses carry an `expires_at` of approximately now,
  telling callers to always re-fetch. L1 cache still
  operates normally with its own TTL.
- What happens during a BatchCost call with mixed cache
  hits and misses? *(Deferred — BatchCost RPC not yet
  implemented)*
  Cached results are returned for hits; only the missing
  resources trigger Azure API calls. Results are merged before
  returning to the caller.
- What happens when the Azure API fails during a cache miss?
  The error propagates directly to the caller. Errors are
  never cached. The next request for the same key retries the
  Azure API. The existing retry logic in the HTTP client
  handles transient failures before the cache layer sees them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST store pricing results keyed by a
  normalized string derived from query dimensions (region,
  service, SKU, product, currency).
- **FR-002**: System MUST return cached results under
  1 millisecond when a cache hit occurs (no external I/O).
- **FR-003**: System MUST support two configurable TTL
  values: (a) an internal L1 cache TTL with a default of
  24 hours (matching Azure's daily data refresh and the
  constitution), and (b) an `expires_at` TTL for gRPC
  responses with a default of 4 hours (prompting callers
  to re-fetch periodically).
- **FR-004**: System MUST treat entries whose TTL has elapsed
  as cache misses.
- **FR-005**: System MUST enforce a configurable maximum number
  of cached entries (default: 1000).
- **FR-006**: System MUST evict the least-recently-used entry
  when the cache is full and a new entry is added.
- **FR-007**: System MUST support fully concurrent read and
  write access without data races, deadlocks, or panics.
- **FR-008**: System MUST pass the Go race detector
  (`go test -race`) under concurrent load.
- **FR-009**: System MUST provide a Get operation that returns
  the cached value. Cache hit/miss is tracked via atomic
  counters (CacheStats) and logged per constitution
  Section III, not exposed as a public API boolean.
- **FR-010**: System MUST provide an Add operation that stores
  a key-value pair using the global TTL.
- **FR-011**: System MUST produce the same normalized cache key
  for equivalent queries regardless of whether they originate
  from single-resource or batch operations.
- **FR-012**: System MUST promote an entry's LRU position when
  it is accessed via Get (read promotes freshness).
- **FR-013**: System MUST set the `expires_at` field on all
  implemented cost responses (EstimateCost, GetActualCost,
  GetProjectedCost) using the configured `expires_at` TTL
  (default: 4 hours), which is separate from the L1 cache
  TTL. BatchCost integration deferred until the BatchCost
  RPC handler is implemented (separate feature).
- **FR-014**: The `expires_at` timestamp MUST be computed
  relative to the entry's creation time, not the response
  time. For a fresh API result: `expires_at = now +
  expiresAtTTL`. For a cache hit: `expires_at =
  min(createdAt + expiresAtTTL, internalExpiresAt)`,
  ensuring L2 callers never hold data longer than L1.
- **FR-015**: System MUST NOT cache error responses. Only
  successful pricing results are stored. API failures
  propagate to the caller and the next request retries.

### Key Entities

- **Cache**: The top-level container holding all cached pricing
  entries. Configured with a maximum size, L1 TTL (24h), and
  `expires_at` TTL (4h) at creation. Provides Get and Add
  operations. Backed by `hashicorp/golang-lru/v2/expirable`.
- **Cache Entry**: A single cached result comprising the
  pricing data, creation timestamp, and LRU tracking
  metadata. Internal expiry is managed by the L1 TTL.
  The `expires_at` value for responses is computed from
  the creation time plus the `expires_at` TTL.
- **Cache Key**: A normalized string derived from the pricing
  query dimensions (region, SKU, service, product, currency)
  that uniquely identifies a cached result. The same key
  is produced regardless of whether the query comes from a
  single-resource or batch operation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Repeated pricing lookups for the same criteria
  complete in under 1 millisecond (compared to seconds for an
  API call).
- **SC-002**: The cache handles 100 concurrent readers and 100
  concurrent writers without data races, deadlocks, or panics.
- **SC-003**: Memory usage remains bounded — the cache never
  stores more than the configured maximum number of entries.
- **SC-004**: All unit tests achieve at least 80% code coverage
  of the cache package.
- **SC-005**: Expired entries are never returned to callers;
  the false-hit rate is zero.
- **SC-006**: Every cost response includes an `expires_at`
  timestamp that any gRPC caller (finfocus CLI or third-party)
  can use to manage its own cache.
- **SC-007**: *(Deferred — depends on BatchCost RPC
  implementation)* A BatchCost call for N resources where
  M are already cached results in only N-M Azure API calls.

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations
  (>=80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered
  (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories
  (Given/When/Then format)
- [x] Integration test needs identified
  (external API interactions)
- [x] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (cache hits <1ms)
- [x] Observability requirements specified (logging, metrics)

### Documentation

- [x] README.md updates identified (if user-facing changes)
- [x] API documentation needs outlined
  (godoc comments, contracts)
- [x] Docstring coverage >=80% maintained
  (all exported symbols documented)
- [x] Examples/quickstart guide planned (if new capability)

### Performance & Reliability

- [x] Performance targets specified (response times, throughput)
- [x] Reliability requirements defined
  (retry logic, error handling)
- [x] Resource constraints considered
  (memory, connections, cache TTL)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data

## Assumptions

- The default maximum cache size of 1000 entries is sufficient
  for typical workloads. Operators can tune this at cache
  creation time.
- A two-TTL model is used: L1 internal cache TTL (24 hours)
  controls when cached data is evicted; `expires_at` TTL
  (4 hours) controls when callers are told to re-fetch.
  This decoupling is deliberate — L2 misses re-enter the
  plugin and are served from L1 without hitting Azure,
  reducing API pressure. Azure pricing data refreshes
  daily with changes typically effective on the 1st of
  the month.
- The cache stores pricing result slices, not individual
  price items. One cache entry corresponds to one complete
  API query result.
- No persistence is required — the cache is ephemeral and
  rebuilt on process restart. This aligns with the project's
  stateless-operation constraint.
- `hashicorp/golang-lru/v2` (MPL 2.0) is license-compatible
  with this project (Apache 2.0). The project already depends
  on `hashicorp/go-retryablehttp` (MPL 2.0) as precedent.
- finfocus-spec will be bumped from v0.5.4 to >= v0.5.7 to
  gain `expires_at` field support and `BatchCost` RPC.
- Third-party gRPC callers that do not use the finfocus CLI
  can still benefit from `expires_at` — the field is
  self-describing in the protobuf schema and requires no
  finfocus-specific knowledge to consume.
