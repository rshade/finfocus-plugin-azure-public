# Tasks: Thread-Safe In-Memory Cache

**Input**: Design documents from `/specs/012-memory-cache/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/cache-api.md, quickstart.md

**Tests**: Included (spec mandates TDD workflow and >=80% coverage)

**Organization**: Tasks grouped by user story for independent
implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)
- Exact file paths included in descriptions

## Phase 1: Setup

**Purpose**: Add new dependencies and bump existing ones

- [X] T001 Add hashicorp/golang-lru/v2 v2.0.7 dependency via go get
- [X] T002 Bump finfocus-spec from v0.5.4 to v0.5.7 in go.mod

**Checkpoint**: Dependencies available for import

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and cache key normalization that ALL user
stories depend on

**CRITICAL**: No user story work can begin until this phase is
complete

- [X] T003 Create CachedResult struct (Items, CreatedAt, ExpiresAt) in internal/azureclient/cache.go
- [X] T004 Create CacheConfig struct and DefaultCacheConfig function (MaxSize=1000, TTL=24h, ExpiresAtTTL=4h) in internal/azureclient/cache.go
- [X] T005 Create CacheStats struct with atomic.Int64 Hits/Misses counters in internal/azureclient/cache.go
- [X] T006 [P] Implement CacheKey normalization function (lowercase, pipe-delimited, fixed field order) in internal/azureclient/cachekey.go
- [X] T007 Write CacheKey unit tests (case normalization, empty fields, whitespace trimming, canonical order) in internal/azureclient/cachekey_test.go

**Checkpoint**: Foundation ready — types compile, CacheKey tested

---

## Phase 3: User Story 1 — Cache Pricing Lookups (Priority: P1) MVP

**Goal**: Eliminate redundant Azure API calls by caching pricing
results keyed by normalized query dimensions

**Independent Test**: Issue the same PriceQuery twice; verify the
second call returns data without an API request (mock client
records call count)

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before
> implementation**

- [X] T008 [US1] Write CachedClient unit tests (cache miss delegates to client, cache hit returns stored result, TTL expiry causes miss, errors are never cached) in internal/azureclient/cache_test.go

### Implementation for User Story 1

- [X] T009 [US1] Implement NewCachedClient constructor with config validation and expirable.LRU initialization in internal/azureclient/cache.go
- [X] T010 [US1] Implement CachedClient.GetPrices with cache key lookup, miss delegation to underlying Client, CachedResult wrapping, and cache storage in internal/azureclient/cache.go
- [X] T011 [P] [US1] Implement CachedClient.Stats, Len, and Close methods in internal/azureclient/cache.go
- [X] T012 [US1] Add cache observability logging (debug per-request hit/miss, info-level hit/miss ratio every 1000 requests or 5 minutes) in internal/azureclient/cache.go

**Checkpoint**: CachedClient passes all US1 tests — repeated
queries served from cache

---

## Phase 4: User Story 2 — Concurrent Access Safety (Priority: P1)

**Goal**: Ensure the cache handles concurrent gRPC requests
without data races, deadlocks, or panics

**Independent Test**: Launch 100+ goroutines doing simultaneous
reads and writes; run with `go test -race`

- [X] T013 [US2] Write concurrent access race detector tests (100 goroutines reading, 100 writing, mixed readers/writers) in internal/azureclient/cache_test.go

**Checkpoint**: `go test -race` passes with concurrent load

---

## Phase 5: User Story 3 — Bounded Memory Usage (Priority: P2)

**Goal**: Enforce maximum cache size with LRU eviction to keep
memory consumption predictable

**Independent Test**: Fill cache beyond MaxSize; verify
least-recently-used entries are evicted while recently accessed
entries survive

- [X] T014 [US3] Write LRU eviction tests (eviction at max capacity, read-promotion prevents eviction, no eviction below capacity) in internal/azureclient/cache_test.go

**Checkpoint**: Cache never exceeds MaxSize entries

---

## Phase 6: User Story 4 — Normalized Cache Keys (Priority: P2)

**Goal**: Ensure identical pricing data produces the same cache
key regardless of query origin (single-resource or batch)

**Independent Test**: Store a result via single-resource query key;
look it up using equivalent batch-derived key dimensions

- [X] T015 [US4] Write cross-query key normalization tests (single-resource vs batch field equivalence, different field ordering produces same key) in internal/azureclient/cachekey_test.go

**Checkpoint**: Single and batch queries share cache entries

---

## Phase 7: User Story 5 — Freshness Signaling via expires_at (Priority: P2)

**Goal**: Set `expires_at` on all gRPC cost responses so callers
can manage their own L2 caches

**Independent Test**: Call EstimateCost; verify response contains
`expires_at` approximately now + 4h for fresh results; verify
cache hits return the original (declining) `expires_at`

### Tests for User Story 5

- [X] T016 [US5] Write Calculator integration tests (accepts CachedClient, sets expires_at on responses, cache hit returns original expires_at) in internal/pricing/calculator_test.go

### Implementation for User Story 5

- [X] T017 [US5] Modify Calculator to accept *CachedClient in constructor and store it as a field in internal/pricing/calculator.go
- [X] T018 [US5] Set expires_at on implemented cost responses using CachedResult.ExpiresAt in internal/pricing/calculator.go (GetActualCost results + GetProjectedCost response; EstimateCost proto in finfocus-spec v0.5.7 has no expires_at field, BatchCost deferred)
- [X] T019 [US5] Wire CachedClient construction (DefaultCacheConfig + NewCachedClient wrapping Client) into startup flow in cmd/finfocus-plugin-azure-public/main.go
- [X] T020 [US5] Update main.go to pass CachedClient to NewCalculator in cmd/finfocus-plugin-azure-public/main.go

**Checkpoint**: All gRPC cost responses include expires_at; main.go
constructs the full cache → client → calculator chain

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Quality gates and documentation

### Constitution Compliance Tasks

- [X] T021 [P] Run `make lint` and fix all linting issues
- [X] T022 [P] Run `go test -race ./...` to verify thread safety across all packages
- [X] T023 [P] Verify test coverage >=80% for internal/azureclient and internal/pricing (`go test -cover`)
- [X] T024 [P] Verify godoc comments on all exported functions/types (docstring coverage >=80%)
- [X] T025 [P] Update CLAUDE.md with cache configuration patterns and CachedClient usage

### Additional Polish

- [X] T026 Run quickstart.md validation (verify code examples match implementation)
- [X] T027 [P] Write BenchmarkCachedClientGetPrices benchmark test validating cache hits complete under 1ms in internal/azureclient/cache_test.go

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (dependencies
  must be available for import)
- **US1 (Phase 3)**: Depends on Phase 2 (types and CacheKey
  function must exist)
- **US2 (Phase 4)**: Depends on Phase 3 (CachedClient must exist
  to test concurrency)
- **US3 (Phase 5)**: Depends on Phase 3 (CachedClient must exist
  to test eviction)
- **US4 (Phase 6)**: Depends on Phase 2 (CacheKey function must
  exist)
- **US5 (Phase 7)**: Depends on Phase 3 (CachedClient must exist
  for Calculator integration)
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational — no dependency on other
  stories
- **US2 (P1)**: Depends on US1 (needs CachedClient to test
  concurrency)
- **US3 (P2)**: Depends on US1 (needs CachedClient to test
  eviction)
- **US4 (P2)**: Depends only on Foundational (tests CacheKey
  normalization)
- **US5 (P2)**: Depends on US1 (needs CachedClient for Calculator
  integration)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Types/models before logic
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- T003-T005 sequential (same file), T006 parallel with them
  (different file)
- T011 can run in parallel with T010 (independent methods)
- US4 (Phase 6) can run in parallel with US2 or US3 (independent
  test files)
- T021-T025 can all run in parallel in Phase 8

---

## Parallel Example: Phase 2 (Foundational)

```text
# Foundational types sequentially (same file: cache.go):
T003: Create CachedResult struct in internal/azureclient/cache.go
T004: Create CacheConfig in internal/azureclient/cache.go
T005: Create CacheStats in internal/azureclient/cache.go

# In parallel with T003-T005 (different file):
T006: Implement CacheKey in internal/azureclient/cachekey.go

# Then sequentially:
T007: Write CacheKey tests in internal/azureclient/cachekey_test.go
```

## Parallel Example: User Stories After Phase 3

```text
# After US1 is complete, these can run in parallel:
US2 (T013): Concurrent access tests
US3 (T014): LRU eviction tests
US5 (T016-T020): Calculator integration + main.go wiring

# US4 (T015) can run in parallel with any post-Phase 2 work
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (add dependencies)
2. Complete Phase 2: Foundational (types + CacheKey)
3. Complete Phase 3: User Story 1 (CachedClient core)
4. **STOP and VALIDATE**: Run `make test` and `make lint`
5. Cache eliminates redundant API calls — core value delivered

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1 -> Test independently -> MVP delivered
3. Add US2 -> Verify concurrency safety with race detector
4. Add US3 + US4 -> Verify eviction and key normalization
5. Add US5 -> expires_at on gRPC responses, full integration
6. Polish -> Lint, coverage, docs

### Suggested MVP Scope

**Phase 1 + Phase 2 + Phase 3 (US1)** = 12 tasks

This delivers the core caching value: repeated pricing queries
served from memory without Azure API calls. The remaining stories
add safety verification (US2), memory bounds (US3), key
deduplication (US4), and caller-side caching support (US5).

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- T003-T005 share `cache.go` and should be done sequentially
  (same file, independent types)
- US2/US3 phases are test-only (golang-lru provides thread safety
  and LRU eviction; tests verify the guarantees hold)
- US5 is the largest post-MVP phase (4 implementation tasks +
  1 test task) touching 3 packages
