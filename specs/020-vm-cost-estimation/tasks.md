# Tasks: VM Cost Estimation (EstimateCost RPC)

<!-- markdownlint-disable MD013 -->

**Input**: Design documents from `/specs/020-vm-cost-estimation/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/

**Tests**: Included per constitution TDD requirements (Section II).

**Organization**: Tasks grouped by user story for independent
implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Exact file paths included in descriptions

## Phase 1: Setup

**Purpose**: No new project setup needed — codebase exists.
Verify existing infrastructure is ready for feature work.

- [X] T001 Verify `make build` and `make test` pass on branch `020-vm-cost-estimation`
- [X] T002 Verify existing `TestEstimateCostUsesCachedClient` passes in `internal/pricing/calculator_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Refactor `estimateQueryFromRequest` signature to return
structured errors instead of `bool`. This change is required by
both US1 (improved response) and US2 (field-specific errors).

**CRITICAL**: No user story work can begin until this phase is
complete.

- [X] T003 [P] Write test `TestEstimateQueryFromRequest_ValidInput_ReturnsQuery` in `internal/pricing/calculator_test.go` — verify `estimateQueryFromRequest` returns a valid `PriceQuery` and `nil` error for a well-formed request with location=eastus, vmSize=Standard_B1s
- [X] T004 [P] Write test `TestEstimateQueryFromRequest_MissingRegion_ReturnsError` in `internal/pricing/calculator_test.go` — verify `estimateQueryFromRequest` returns an error containing "region" when location/region is missing
- [X] T005 [P] Write test `TestEstimateQueryFromRequest_MissingBoth_ReturnsError` in `internal/pricing/calculator_test.go` — verify `estimateQueryFromRequest` returns an error containing both "region" and "sku" when both are missing
- [X] T006 Refactor `estimateQueryFromRequest` return type from `(PriceQuery, bool)` to `(PriceQuery, error)` in `internal/pricing/calculator.go` — return `fmt.Errorf("missing required field(s): ...")` listing all missing fields (region, sku) or `nil` on success
- [X] T007 Update `EstimateCost` method in `internal/pricing/calculator.go` to use new error return from `estimateQueryFromRequest` — return `status.Error(codes.InvalidArgument, err.Error())` instead of `codes.Unimplemented`
- [X] T008 Run `make test` to verify T003-T005 tests pass and no existing tests break

**Checkpoint**: `estimateQueryFromRequest` returns structured
errors. All existing tests still pass.

---

## Phase 3: User Story 1 — Estimate Linux VM Cost (P1) MVP

**Goal**: A valid EstimateCost request returns accurate monthly
cost with correct currency and `STANDARD` pricing category.

**Independent Test**: Send EstimateCost with region=eastus,
SKU=Standard_B1s. Verify response has non-zero `cost_monthly`,
currency=USD, and `pricing_category=STANDARD`.

### Tests for User Story 1

> **NOTE: Write tests FIRST, ensure they FAIL before implementation**

- [X] T009 [P] [US1] Write test `TestEstimateCost_ValidRequest_SetsPricingCategoryStandard` in `internal/pricing/calculator_test.go` — verify `resp.GetPricingCategory()` equals `FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD` for a valid VM estimate using mock HTTP server
- [X] T010 [P] [US1] Write test `TestEstimateCost_ValidRequest_ReturnsCostMonthlyEqualToHourlyTimes730` in `internal/pricing/calculator_test.go` — mock server returns RetailPrice=0.0200, verify `resp.GetCostMonthly()` equals `0.0200 * 730.0` (14.60)

### Implementation for User Story 1

- [X] T011 [US1] Add `pluginsdk.WithPricingCategory(pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD)` to `EstimateCost` response builder in `internal/pricing/calculator.go:117-119` (add `pbc` import alias if not already present)
- [X] T012 [US1] Run `make test` to verify T009-T010 pass and existing `TestEstimateCostUsesCachedClient` still passes

**Checkpoint**: EstimateCost returns correct monthly cost with
`STANDARD` pricing category. US1 is independently testable.

---

## Phase 4: User Story 2 — Clear Errors for Invalid Requests (P2)

**Goal**: Invalid or incomplete descriptors return specific gRPC
error codes with actionable messages identifying the problem.

**Independent Test**: Send malformed requests (missing region,
missing SKU, unsupported resource type, nonexistent VM) and
verify each returns the correct gRPC error code and message.

### Tests for User Story 2

> **NOTE: Write tests FIRST, ensure they FAIL before implementation**
>
> **NOTE**: T015-T019 intentionally test at the `EstimateCost`
> gRPC method level, complementing T003-T005 which test the
> internal `estimateQueryFromRequest` helper directly. Both layers
> are needed: helper tests validate extraction logic; method tests
> validate gRPC error code propagation.

- [X] T013 [P] [US2] Write test `TestEstimateCost_UnsupportedResourceType_ReturnsUnimplemented` in `internal/pricing/calculator_test.go` — send request with `resource_type="network/LoadBalancer"` and valid region/sku, verify `codes.Unimplemented` error
- [X] T014 [P] [US2] Write test `TestEstimateCost_EmptyResourceType_Succeeds` in `internal/pricing/calculator_test.go` — send request with empty `resource_type` and valid region/sku via mock server, verify success (backward compatibility)
- [X] T015 [P] [US2] Write test `TestEstimateCost_MissingRegion_ReturnsInvalidArgument` in `internal/pricing/calculator_test.go` — send request missing location/region, verify `codes.InvalidArgument` error containing "region"
- [X] T016 [P] [US2] Write test `TestEstimateCost_MissingSKU_ReturnsInvalidArgument` in `internal/pricing/calculator_test.go` — send request missing vmSize/sku, verify `codes.InvalidArgument` error containing "sku"
- [X] T017 [P] [US2] Write test `TestEstimateCost_MissingBothFields_ReturnsInvalidArgument` in `internal/pricing/calculator_test.go` — send request with nil attributes, verify `codes.InvalidArgument` error containing both "region" and "sku"
- [X] T018 [P] [US2] Write test `TestEstimateCost_NotFoundSKU_ReturnsNotFound` in `internal/pricing/calculator_test.go` — mock server returns empty items, verify `codes.NotFound` error
- [X] T019 [P] [US2] Write test `TestEstimateCost_MultipleItems_UsesFirstItem` in `internal/pricing/calculator_test.go` — mock server returns 3 price items with different RetailPrice values, verify `cost_monthly` uses the first item's price (edge case from spec)

### Implementation for User Story 2

- [X] T020 [US2] Add resource type validation to `EstimateCost` in `internal/pricing/calculator.go` — before `estimateQueryFromRequest`, check if `req.GetResourceType()` is non-empty and does not contain `compute/virtualMachine` (case-insensitive via `strings.Contains` + `strings.ToLower`); return `status.Errorf(codes.Unimplemented, "unsupported resource type: %s", resourceType)`. Allow empty resource type for backward compatibility
- [X] T021 [US2] Update `TestEstimateCostReturnsUnimplemented` in `internal/pricing/calculator_test.go` — existing test at line 472 uses nil cachedClient to get Unimplemented; verify it still works with the new validation logic (nil attributes should still return InvalidArgument or Unimplemented depending on path)
- [X] T022 [US2] Run `make test` to verify T013-T019 pass and all US1 tests still pass

**Checkpoint**: All invalid request scenarios return correct
error codes with specific messages. US2 is independently testable.

---

## Phase 5: User Story 3 — Cached Pricing for Repeated Queries (P3)

**Goal**: Repeated EstimateCost queries for the same VM are served
from cache with no outbound API call.

**Independent Test**: Issue the same EstimateCost request twice
via mock server. Verify second request does not hit the mock
server (request count = 1) and both responses are identical.

### Tests for User Story 3

- [X] T023 [US3] Write test `TestEstimateCost_RepeatedQuery_UsesCacheOnSecondCall` in `internal/pricing/calculator_test.go` — use `atomic.Int32` counter on mock server handler, send same request twice, assert handler called exactly once and both responses have same `cost_monthly`
- [X] T024 [US3] Write test `TestEstimateCost_CacheStats_RecordsHitAndMiss` in `internal/pricing/calculator_test.go` — after two identical requests, verify `cachedClient.Stats().Hits.Load() == 1` and `cachedClient.Stats().Misses.Load() == 1`

### Implementation for User Story 3

- [X] T025 [US3] Verify existing cache integration works — no code changes expected. Run T023-T024 tests. If tests pass, cache integration is confirmed complete via existing `CachedClient` wrapper

**Checkpoint**: Cache integration verified. Repeated queries use
cache. US3 is independently testable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Structured logging, documentation, linting, and
final validation.

### Structured Logging (FR-012)

> **NOTE**: Cache hit/miss logging is handled at the `CachedClient`
> layer (debug level). These tasks add request-level logging only
> (success/error), not cache-hit distinction.

- [X] T026 [P] Add structured log fields to `EstimateCost` request entry in `internal/pricing/calculator.go` — add `region`, `sku`, `resource_type` fields to the `log.Info()` call at method start
- [X] T027 [P] Add structured log fields to `EstimateCost` success path in `internal/pricing/calculator.go` — log `cost_monthly`, `currency`, `result_status=success` after building response
- [X] T028 [P] Add structured log fields to `EstimateCost` error paths in `internal/pricing/calculator.go` — log `result_status=error` with error details on validation and API failures

### Documentation

- [X] T029 [P] Update godoc comment on `EstimateCost` in `internal/pricing/calculator.go` — replace "stub that returns Unimplemented status" with accurate description of behavior, input requirements, error conditions, and PricingCategory behavior
- [X] T030 [P] Update godoc comment on `estimateQueryFromRequest` in `internal/pricing/calculator.go` — document new error return type, list extracted attribute keys, and describe missing-field error format
- [X] T031 [P] Update CLAUDE.md with EstimateCost usage section — add code example showing how to call EstimateCost with expected response fields

### Constitution Compliance

- [X] T032 Run `make lint` and fix all linting issues (use extended timeout >5 minutes)
- [X] T033 Run `go test -race ./internal/pricing/...` to verify thread safety
- [X] T034 Run `go test -cover ./internal/pricing/...` and verify ≥80% coverage for business logic
- [X] T035 Verify docstring coverage ≥80% for `internal/pricing/` package — all exported functions, types, and the package itself must have godoc comments
- [X] T036 Run `make test` for final full-suite validation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all
  user stories
- **US1 (Phase 3)**: Depends on Phase 2 completion
- **US2 (Phase 4)**: Depends on Phase 2 completion (independent
  of US1)
- **US3 (Phase 5)**: Depends on Phase 2 completion (independent
  of US1 and US2)
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Requires Phase 2 refactor. No dependency on US2
  or US3
- **US2 (P2)**: Requires Phase 2 refactor. Independent of US1
  (can run in parallel)
- **US3 (P3)**: Requires Phase 2 refactor. Independent of US1
  and US2 (can run in parallel)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation then makes tests pass
- Checkpoint validation before moving to next story

### Parallel Opportunities

- T003, T004, T005 can run in parallel (different test functions)
- T009, T010 can run in parallel (different test assertions)
- T013-T019 can all run in parallel (independent test cases)
- T026, T027, T028 can run in parallel (different log locations)
- T029, T030, T031 can run in parallel (different doc targets)
- US1, US2, US3 can run in parallel after Phase 2 completes

---

## Parallel Example: User Story 2

```text
# Launch all US2 tests in parallel (different test functions):
T013: TestEstimateCost_UnsupportedResourceType_ReturnsUnimplemented
T014: TestEstimateCost_EmptyResourceType_Succeeds
T015: TestEstimateCost_MissingRegion_ReturnsInvalidArgument
T016: TestEstimateCost_MissingSKU_ReturnsInvalidArgument
T017: TestEstimateCost_MissingBothFields_ReturnsInvalidArgument
T018: TestEstimateCost_NotFoundSKU_ReturnsNotFound
T019: TestEstimateCost_MultipleItems_UsesFirstItem

# Then implement (sequential — same file):
T020: Add resource type validation
T021: Update existing Unimplemented test
T022: Run make test
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T008)
3. Complete Phase 3: User Story 1 (T009-T012)
4. **STOP and VALIDATE**: `make test` passes, PricingCategory is
   STANDARD
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1 -> PricingCategory + accurate cost (MVP!)
3. Add US2 -> Proper error codes for all invalid inputs
4. Add US3 -> Cache verification confirmed
5. Polish -> Logging, docs, lint compliance

### Single Developer Strategy

1. Complete Phases 1-2 sequentially
2. Complete US1 (P1 priority — core value)
3. Complete US2 (P2 — error handling)
4. Complete US3 (P3 — cache verification, minimal work)
5. Complete Polish (logging, docs, lint)

---

## Summary

- **Total tasks**: 36
- **US1 tasks**: 4 (T009-T012)
- **US2 tasks**: 10 (T013-T022)
- **US3 tasks**: 3 (T023-T025)
- **Setup + Foundational**: 8 (T001-T008)
- **Polish**: 11 (T026-T036)
- **Parallel opportunities**: 6 groups identified
- **MVP scope**: Phases 1-3 (T001-T012, 12 tasks)
- **All tasks follow checklist format**: Confirmed

<!-- markdownlint-enable MD013 -->
