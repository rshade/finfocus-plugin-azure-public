# Tasks: Integration Tests with Live Azure Retail Prices API

**Input**: Design documents from `/specs/022-integration-tests/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/

**Tests**: This feature IS the tests — no separate test tasks needed.

**Organization**: Tasks grouped by user story for independent
implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2, US3, US4)
- All source changes are in
  `examples/estimate_cost_integration_test.go`

## Phase 1: Setup

**Purpose**: Create the test file skeleton with build tag, package,
imports, and reference price constants

- [x] T001 Create test file with build tag, package declaration,
  and imports in `examples/estimate_cost_integration_test.go`
  (use `//go:build integration`, package `examples`, imports per
  plan.md "Imports Required" section)
- [x] T002 Define reference price constants and tolerance in
  `examples/estimate_cost_integration_test.go`
  (`refB1sHourly=0.0104`, `refD2sv3Hourly=0.096`,
  `refDisk128StandardMonthly=5.89`,
  `refDisk256PremiumMonthly=38.00`, `priceTolerance=0.25`,
  with last-verified date comment)

---

## Phase 2: Foundational (Helper Functions)

**Purpose**: Shared helpers needed by ALL user stories — MUST
complete before any story

**CRITICAL**: No story tasks can begin until these are done

- [x] T003 Implement `skipIfDisabled(t *testing.T)` helper in
  `examples/estimate_cost_integration_test.go`
  (check `os.Getenv("SKIP_INTEGRATION") == "true"`, call
  `t.Skip()`)
- [x] T004 Implement `newTestCalculator(t *testing.T)` helper in
  `examples/estimate_cost_integration_test.go`
  (construct `DefaultConfig` → `NewClient` → `DefaultCacheConfig`
  → `NewCachedClient` → `NewCalculator`; return both
  `*pricing.Calculator` and `*azureclient.CachedClient`;
  use `t.Fatalf` on errors; register
  `t.Cleanup(func() { cachedClient.Close() })` to stop
  eviction goroutine; see contracts/test_helpers.go for
  exact construction chain. Each test calling this helper
  should create a `context.WithTimeout(ctx, 30*time.Second)`
  for EstimateCost calls per FR-010)
- [x] T005 Implement `assertInRange(t, actual, reference,
  tolerance float64)` helper in
  `examples/estimate_cost_integration_test.go`
  (calculate `low = reference*(1-tolerance)`,
  `high = reference*(1+tolerance)`, call `t.Helper()`,
  `t.Errorf` if out of range with actual/expected values)
- [x] T006 Implement `rateLimitDelay()` helper in
  `examples/estimate_cost_integration_test.go`
  (`time.Sleep(12 * time.Second)`)

**Checkpoint**: Foundation ready — all helpers available for story
implementation

---

## Phase 3: User Story 1 — VM Cost Estimation (P1) MVP

**Goal**: Verify EstimateCost returns accurate VM pricing within
±25% for Standard_B1s and Standard_D2s_v3, plus cache hit behavior

**Independent Test**: Run
`go test -v -tags=integration -run TestEstimateCost_VM ./examples/...`

### Implementation for User Story 1

- [x] T007 [US1] Implement `TestEstimateCost_VM_StandardB1s` in
  `examples/estimate_cost_integration_test.go`
  (call `skipIfDisabled`, create calculator via
  `newTestCalculator`, build `structpb.NewStruct` with
  `location=eastus`, `vmSize=Standard_B1s`, call
  `calc.EstimateCost` with resource type
  `azure:compute/virtualMachine:VirtualMachine`,
  assert `resp.GetCostMonthly() > 0`, assert monthly cost
  within ±25% of `refB1sHourly * 730` using `assertInRange`,
  log actual cost with `t.Logf`, call `rateLimitDelay()`)
- [x] T008 [US1] Implement `TestEstimateCost_VM_StandardD2sv3` in
  `examples/estimate_cost_integration_test.go`
  (same pattern as T007 but with `vmSize=Standard_D2s_v3`,
  assert monthly cost within ±25% of `refD2sv3Hourly * 730`,
  call `rateLimitDelay()`)
- [x] T009 [US1] Implement `TestEstimateCost_VM_CacheHit` in
  `examples/estimate_cost_integration_test.go`
  (call `skipIfDisabled`, create calculator, call EstimateCost
  for Standard_B1s eastus (cache miss), record
  `cachedClient.Stats().Misses.Load()`, call EstimateCost
  again with same params, assert
  `cachedClient.Stats().Hits.Load() > 0`, assert no new
  misses, assert both responses have identical
  `GetCostMonthly()` — NO `rateLimitDelay` needed since
  second call uses cache)

**Checkpoint**: VM estimation pipeline validated end-to-end.
Run: `go test -v -tags=integration -run TestEstimateCost_VM ./examples/...`

---

## Phase 4: User Story 2 — Managed Disk Estimation (P2)

**Goal**: Verify EstimateCost returns positive pricing for Standard
and Premium managed disks via size-to-tier mapping

**Independent Test**: Run
`go test -v -tags=integration -run TestEstimateCost_Disk ./examples/...`

### Implementation for User Story 2

- [x] T010 [US2] Implement `TestEstimateCost_Disk_StandardLRS` in
  `examples/estimate_cost_integration_test.go`
  (call `skipIfDisabled`, create calculator, build
  `structpb.NewStruct` with `location=eastus`,
  `disk_type=Standard_LRS`, `size_gb=128`, call
  `calc.EstimateCost` with resource type
  `azure:storage/managedDisk:ManagedDisk`, assert
  `resp.GetCostMonthly() > 0`, assert within ±25% of
  `refDisk128StandardMonthly`, log actual cost,
  call `rateLimitDelay()`)
- [x] T011 [US2] Implement `TestEstimateCost_Disk_PremiumSSD` in
  `examples/estimate_cost_integration_test.go`
  (same pattern with `disk_type=Premium_SSD_LRS`,
  `size_gb=256`, assert `resp.GetCostMonthly() > 0`,
  assert within ±25% of `refDisk256PremiumMonthly`,
  call `rateLimitDelay()`)

**Checkpoint**: Disk estimation pipeline validated. Run:
`go test -v -tags=integration -run TestEstimateCost_Disk ./examples/...`

---

## Phase 5: User Story 3 — Error Handling (P2)

**Goal**: Verify invalid inputs produce correct gRPC error codes
(NotFound, InvalidArgument) instead of silent failures

**Independent Test**: Run
`go test -v -tags=integration -run TestEstimateCost_Error ./examples/...`

### Implementation for User Story 3

- [x] T012 [US3] Implement `TestEstimateCost_Error_InvalidSKU` in
  `examples/estimate_cost_integration_test.go`
  (call `skipIfDisabled`, create calculator, build attrs with
  `location=eastus`, `vmSize=Nonexistent_ZZZZZ_Invalid`,
  call `calc.EstimateCost` with VM resource type, assert
  `err != nil`, extract gRPC status with
  `status.FromError(err)`, assert
  `st.Code() == codes.NotFound`, call `rateLimitDelay()`)
- [x] T013 [US3] Implement
  `TestEstimateCost_Error_MissingAttributes` in
  `examples/estimate_cost_integration_test.go`
  (call `skipIfDisabled`, create calculator, build empty
  attrs via `structpb.NewStruct(map[string]any{})`, call
  `calc.EstimateCost` with VM resource type, assert
  `err != nil`, extract gRPC status, assert
  `st.Code() == codes.InvalidArgument` — NO
  `rateLimitDelay` needed since request fails before API call)

**Checkpoint**: Error paths validated. Run:
`go test -v -tags=integration -run TestEstimateCost_Error ./examples/...`

---

## Phase 6: User Story 4 — CI Pipeline Integration (P3)

**Goal**: Verify tests can be skipped via env var and CI workflow
compatibility

**Independent Test**: Run
`SKIP_INTEGRATION=true go test -v -tags=integration -run TestEstimateCost_Skip ./examples/...`

### Implementation for User Story 4

- [x] T014 [US4] Implement `TestEstimateCost_SkipIntegration` in
  `examples/estimate_cost_integration_test.go`
  (set `t.Setenv("SKIP_INTEGRATION", "true")`, call
  `skipIfDisabled(t)`, assert test was skipped — if execution
  reaches past `skipIfDisabled` call, `t.Fatal("expected skip")`)
- [x] T015 [US4] Verify CI workflow compatibility by confirming
  `.github/workflows/test.yml` integration job runs
  `go test -v -tags=integration -timeout=2m ./examples/...`
  (read-only check — no file changes needed per research.md R6)

**Checkpoint**: Skip mechanism validated. CI picks up new tests
automatically.

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Quality gates and documentation updates

### Constitution Compliance Tasks

- [x] T016 [P] Run `gofmt` and `goimports` on
  `examples/estimate_cost_integration_test.go`
- [x] T017 [P] Run `make lint` and fix any linting issues
  (use extended timeout — can take >5 minutes)
- [x] T018 [P] Run full integration test suite:
  `go test -v -tags=integration -timeout=5m ./examples/...`
  and verify all tests pass
- [x] T019 [P] Update CHANGELOG.md with integration test addition
  (format: Keep a Changelog v1.0.0, under `### Added`)
- [x] T020 Update CLAUDE.md with integration test documentation
  (add `go test -tags=integration ./examples/...` to Commands
  section, note `SKIP_INTEGRATION` env var)
- [x] T021 [P] Update README.md with integration test section
  (add run command `go test -tags=integration ./examples/...`,
  document `SKIP_INTEGRATION` env var, note ±25% price tolerance
  and how to update reference prices when Azure adjusts pricing)

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001-T002)
- **US1 (Phase 3)**: Depends on Phase 2 (T003-T006) — MVP
- **US2 (Phase 4)**: Depends on Phase 2 only — independent of US1
- **US3 (Phase 5)**: Depends on Phase 2 only — independent of
  US1/US2
- **US4 (Phase 6)**: Depends on Phase 2 only — independent of
  US1-US3
- **Polish (Phase 7)**: Depends on all stories being complete

### User Story Dependencies

- **US1 (P1)**: No dependencies on other stories — MVP target
- **US2 (P2)**: Independent of US1 (different resource type)
- **US3 (P2)**: Independent (error paths, not success paths)
- **US4 (P3)**: Independent (tests skip mechanism only)

### Within Each User Story

Since all code lives in a single file, stories are implemented
sequentially in priority order. However, each story can be tested
independently via `-run` regex filters.

### Parallel Opportunities

- T003-T006 (foundational helpers): Independent functions, can be
  written in parallel
- T016-T019 (polish): Independent checks, can run in parallel
- Stories are independent but share a single file — implement
  sequentially by priority

---

## Parallel Example: Foundational Helpers

```bash
# All four helpers are independent functions — write in parallel:
Task: "Implement skipIfDisabled in estimate_cost_integration_test.go"
Task: "Implement newTestCalculator in estimate_cost_integration_test.go"
Task: "Implement assertInRange in estimate_cost_integration_test.go"
Task: "Implement rateLimitDelay in estimate_cost_integration_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational helpers (T003-T006)
3. Complete Phase 3: US1 VM tests (T007-T009)
4. **STOP and VALIDATE**: Run
   `go test -v -tags=integration -run TestEstimateCost_VM ./examples/...`
5. VM pipeline confirmed working end-to-end

### Incremental Delivery

1. Setup + Foundational → file skeleton ready
2. US1 (VM tests) → validate independently → MVP done
3. US2 (Disk tests) → validate independently
4. US3 (Error tests) → validate independently
5. US4 (Skip/CI) → validate independently
6. Polish → lint, docs, full suite run

### Estimated Execution Time

- T001-T006 (setup + helpers): ~15 min implementation
- T007-T009 (US1 VM): ~10 min + ~24s rate delays
- T010-T011 (US2 Disk): ~10 min + ~24s rate delays
- T012-T013 (US3 Errors): ~10 min + ~12s rate delay
- T014-T015 (US4 CI): ~5 min
- T016-T021 (Polish): ~20 min
- **Total**: ~70 min implementation + ~60s test execution delays

---

## Notes

- All source changes target a single file:
  `examples/estimate_cost_integration_test.go`
- Reference prices will need periodic updates as Azure pricing
  changes — constants are intentionally easy to find and modify
- Rate limit delays (12s) are conservative — Azure public API
  informal limit is ~100 requests/minute, but we stay well under
  to avoid flaky CI
- The CI `integration` job timeout is 2 minutes — sufficient for
  ~90s total test execution time
