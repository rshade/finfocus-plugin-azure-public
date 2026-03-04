# Tasks: Cache Layer Completion (v0.3.0 Gaps)

**Input**: Design documents from `/specs/015-cache-completion/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
quickstart.md

**Tests**: Included per constitution (TDD required for all new
features).

**Organization**: Tasks grouped by user story for independent
implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2)
- Exact file paths included in all task descriptions

## Path Conventions

- **Entry point**: `cmd/finfocus-plugin-azure-public/main.go`
- **Cache library**: `internal/azureclient/cache.go`
- **Cache tests**: `internal/azureclient/cache_test.go`

---

## Phase 1: Setup

**Purpose**: No new project setup needed. Both changes modify existing
files in an established codebase. This phase is a no-op.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: No foundational infrastructure needed. Both user stories
operate on existing code with no shared new dependencies.

---

## Phase 3: User Story 1 - Environment Variable TTL Override (P1)

**Goal**: Allow operators to override the default 24-hour cache TTL
via `FINFOCUS_CACHE_TTL` environment variable.

**Independent Test**: Set `FINFOCUS_CACHE_TTL=1s`, start plugin,
perform a pricing lookup, wait 2 seconds, perform another lookup,
verify second lookup triggers a fresh API call.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before
> implementation**

- [x] T001 [US1] Write test for valid `FINFOCUS_CACHE_TTL` parsing
  (e.g., "10s", "1h") in
  `cmd/finfocus-plugin-azure-public/main_test.go`
- [x] T002 [US1] Write test for invalid `FINFOCUS_CACHE_TTL` parsing
  (e.g., "banana") falling back to default in
  `cmd/finfocus-plugin-azure-public/main_test.go`
- [x] T003 [US1] Write test for unset `FINFOCUS_CACHE_TTL` using
  default 24h TTL in
  `cmd/finfocus-plugin-azure-public/main_test.go`
- [x] T004 [US1] Write test for `FINFOCUS_CACHE_TTL=0s` disabling
  cache in `cmd/finfocus-plugin-azure-public/main_test.go`
- [x] T005 [US1] Write test for negative duration (e.g., "-5s")
  falling back to default in
  `cmd/finfocus-plugin-azure-public/main_test.go`

### Implementation for User Story 1

- [x] T006 [US1] Extract a `parseCacheTTL` helper function that reads
  `FINFOCUS_CACHE_TTL` env var, parses with `time.ParseDuration`,
  logs warning on invalid value, returns default on error/unset, in
  `cmd/finfocus-plugin-azure-public/main.go`
- [x] T007 [US1] Call `parseCacheTTL` and set `cacheConfig.TTL` before
  `NewCachedClient` call (after line 91) in
  `cmd/finfocus-plugin-azure-public/main.go`
- [x] T008 [US1] Log effective cache TTL at info level after cache
  config is resolved in
  `cmd/finfocus-plugin-azure-public/main.go`

**Checkpoint**: `FINFOCUS_CACHE_TTL` env var overrides default TTL.
All T001-T005 tests pass. Plugin logs effective TTL at startup.

---

## Phase 4: User Story 2 - Eviction Event Logging (P2)

**Goal**: Emit debug-level structured logs when cache entries are
evicted by TTL expiry or LRU overflow.

**Independent Test**: Configure a cache with MaxSize=2, insert 3
entries, verify a debug log is emitted for the evicted entry with
the correct key and reason.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before
> implementation**

- [x] T009 [US2] Write test for LRU eviction callback logging with
  reason "lru" when cache exceeds capacity in
  `internal/azureclient/cache_test.go`
- [x] T010 [US2] Write test for TTL eviction callback logging with
  reason "expired" when entry ages past TTL in
  `internal/azureclient/cache_test.go`
- [x] T011 [US2] Write test verifying eviction callback is safe under
  concurrent access (`go test -race`) in
  `internal/azureclient/cache_test.go`

### Implementation for User Story 2

- [x] T012 [US2] Replace `nil` eviction callback in
  `expirable.NewLRU` (cache.go:90) with a closure that logs
  eviction events at debug level in
  `internal/azureclient/cache.go`
- [x] T013 [US2] Implement eviction reason inference in the callback:
  compare `CachedResult.CreatedAt + config.TTL` vs `time.Now()` to
  classify as "expired" or "lru" in
  `internal/azureclient/cache.go`
- [x] T014 [US2] Include structured log fields: `cache_key` and
  `eviction_reason` in the eviction callback debug log in
  `internal/azureclient/cache.go`

**Checkpoint**: Eviction events produce structured debug logs with
key and reason. All T009-T011 tests pass. Race detector passes.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Validation, documentation, and compliance

### Constitution Compliance Tasks

- [x] T015 [P] Run `make lint` and fix all linting issues
- [x] T016 [P] Run `go test -race ./...` to verify thread safety
- [x] T017 [P] Verify test coverage >=80% for modified files
  (`go test -cover ./internal/azureclient/...
  ./cmd/finfocus-plugin-azure-public/...`)
- [x] T018 [P] Add godoc comments to `parseCacheTTL` and any new
  exported functions
- [x] T019 [P] Update CLAUDE.md with `FINFOCUS_CACHE_TTL` env var
  documentation
- [x] T020 Verify all existing cache tests still pass (no regressions)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Skipped — no new setup needed
- **Foundational (Phase 2)**: Skipped — no blocking prerequisites
- **US1 (Phase 3)**: Can start immediately — modifies `main.go` only
- **US2 (Phase 4)**: Can start immediately — modifies `cache.go` only
- **Polish (Phase 5)**: Depends on both US1 and US2 completion

### User Story Dependencies

- **US1 (P1)**: No dependencies on US2. Modifies `main.go` only.
- **US2 (P2)**: No dependencies on US1. Modifies `cache.go` and
  `cache_test.go` only.
- **US1 and US2 can run in parallel** — they touch different files.

### Within Each User Story

- Tests MUST be written first and FAIL before implementation
- Implementation tasks are sequential within each story
- Story complete before moving to Polish phase

### Parallel Opportunities

- US1 (T001-T008) and US2 (T009-T014) can execute in parallel
  (different files)
- All T001-T005 tests can be written in parallel
- T009, T010, T011 tests can be written in parallel
- All T015-T020 polish tasks marked [P] can run in parallel

---

## Parallel Example: Full Feature

```text
# Both user stories can run in parallel:

# Stream A (US1 - main.go):
T001-T005: Write env var tests (parallel)
T006-T008: Implement env var parsing (sequential)

# Stream B (US2 - cache.go):
T009-T011: Write eviction callback tests (parallel)
T012-T014: Implement eviction callback (sequential)

# After both streams complete:
T015-T020: Polish tasks (parallel where marked [P])
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 3: US1 (env var TTL override)
2. **STOP and VALIDATE**: Test with `FINFOCUS_CACHE_TTL=1s`
3. This alone closes issue #13

### Incremental Delivery

1. US1 (env var) -> Test independently -> Closes #13
2. US2 (eviction logging) -> Test independently -> Closes #15
3. Polish -> Closes v0.3.0 milestone

### Parallel Strategy

Both stories touch different files and can execute simultaneously:

- Stream A: `main.go` + `main_test.go` (US1)
- Stream B: `cache.go` + `cache_test.go` (US2)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 and US2 are fully independent — no cross-story dependencies
- Commit after each story checkpoint
- Run `make test` after each story to catch regressions early
