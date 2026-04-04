# Tasks: GetProjectedCost RPC

**Input**: Design documents from `/specs/023-projected-cost-rpc/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Included per constitution (TDD requirement — tests MUST be written BEFORE implementation).

**Organization**: Tasks grouped by user story. US1 (VM Cost Lookup) and US2 (Validation Errors) are combined because they share the same implementation function (`GetProjectedCost()`).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Verify existing infrastructure supports the feature

- [x] T001 Verify `MapDescriptorToQuery()` returns sentinel errors for all validation cases by reading `internal/pricing/mapper.go` and `internal/pricing/mapper_test.go`
- [x] T002 Verify `MapToGRPCStatus()` maps `ErrUnsupportedResourceType` and `ErrMissingRequiredFields` to correct gRPC codes by reading `internal/pricing/errors.go` and `internal/pricing/errors_test.go`
- [x] T003 Verify pluginsdk helpers `WithProjectedCostDetails()`, `WithProjectedCostPricingCategory()`, `WithProjectedCostExpiresAt()` exist and accept expected parameters

**Checkpoint**: All existing infrastructure confirmed — no new dependencies needed

---

## Phase 2: US1+US2 — VM Cost Lookup + Validation Error Feedback (Priority: P1) MVP

**Goal**: Refactor `GetProjectedCost` to use `MapDescriptorToQuery()` for validation, return complete responses with `billing_detail` and `pricing_category`, and provide specific gRPC error codes for all invalid inputs.

**Independent Test**: Send a valid VM request and verify all 5 response fields; send invalid requests and verify specific gRPC codes with field names in error messages.

### Tests for US1+US2 (TDD Red — write first, verify they fail)

- [x] T004 [US1] [US2] Write table-driven `TestGetProjectedCost_Validation` test with cases for nil request, nil descriptor, wrong provider, unsupported resource type, missing region, missing SKU, missing both, nil cachedClient in `internal/pricing/calculator_test.go`
- [x] T005 [US1] Write `TestGetProjectedCost_Success_VMResponse` test asserting unit_price, currency, cost_per_month, billing_detail, pricing_category, and expires_at are all set correctly in `internal/pricing/calculator_test.go`
- [x] T006 [US1] Write `TestGetProjectedCost_Success_BillingDetail` test asserting billing_detail format matches `"Azure Retail Prices API: {SKU} in {region} at ${price}/hr * 730 hrs/mo"` in `internal/pricing/calculator_test.go`
- [x] T007 [US1] Write `TestGetProjectedCost_APIError` test asserting Azure API errors are mapped through `MapToGRPCStatus()` to correct gRPC codes in `internal/pricing/calculator_test.go`

### Implementation for US1+US2 (TDD Green)

- [x] T008 [US1] [US2] Refactor `GetProjectedCost()` in `internal/pricing/calculator.go`: replace `projectedQueryFromRequest()` call with `MapDescriptorToQuery(req.GetResource())`, add nil request/descriptor guard, map errors via `MapToGRPCStatus()`
- [x] T009 [US1] [US2] Add resource-type routing after `MapDescriptorToQuery()` succeeds: VM service name → compute cost path, Managed Disks/Storage → return `codes.Unimplemented` with descriptive message in `internal/pricing/calculator.go`
- [x] T010 [US1] Build response with `WithProjectedCostDetails(unitPrice, currency, costMonthly, billingDetail)`, `WithProjectedCostPricingCategory(STANDARD)`, `WithProjectedCostExpiresAt()` in `internal/pricing/calculator.go`
- [x] T011 [US1] Update `TestGetProjectedCostSetsExpiresAtFromCache` to use resource type `"compute/VirtualMachine"` instead of `"azure:compute/virtualMachine:VirtualMachine"` in `internal/pricing/calculator_test.go`
- [x] T012 [US1] Rewrite `TestProjectedCostSupported` to use Azure provider/resource type (`provider="azure"`, `resource_type="compute/VirtualMachine"`, `region="eastus"`, `sku="Standard_B1s"`) replacing the AWS placeholder data, remove `t.Skip()`, and verify it passes in `internal/pricing/calculator_test.go`
- [x] T013 [US1] [US2] Remove now-unused `projectedQueryFromRequest()` function from `internal/pricing/calculator.go` after verifying no other callers via grep
- [x] T014 Run `make test` to verify all tests pass including existing `TestGetProjectedCostReturnsUnimplemented` (US4 backward compat)

**Checkpoint**: US1+US2 complete — VM projected cost returns all fields, all validation errors return specific gRPC codes. US4 (nil cachedClient) also verified.

---

## Phase 3: US3 — Structured Observability (Priority: P2)

**Goal**: Add zerolog structured logging at all decision points in `GetProjectedCost`, matching the `EstimateCost` pattern.

**Independent Test**: Capture log output during request processing and verify structured fields appear at correct log levels.

### Tests for US3 (TDD Red — write first, verify they fail)

- [x] T015a [US3] Write `TestGetProjectedCost_Logging` table-driven test capturing zerolog output (via `zerolog.New(&buf)`) and asserting: (a) success path emits Info with `cost_monthly`, `currency`, `unit_price`, `result_status=success`; (b) validation failure emits Warn with `result_status=error`; (c) API failure emits Error with `result_status=error`; (d) nil cachedClient emits Warn with `result_status=error` in `internal/pricing/calculator_test.go`

### Implementation for US3

- [x] T015 [US3] Add request entry log (Info level) with `region`, `sku`, `resource_type`, `provider` fields after `MapDescriptorToQuery()` succeeds in `internal/pricing/calculator.go`
- [x] T016 [US3] Add validation failure log (Warn level) with `result_status=error` for nil descriptor, mapper errors, and unsupported resource types in `internal/pricing/calculator.go`
- [x] T017 [US3] Add API/cache failure log (Error level) with `region`, `sku`, `resource_type`, `result_status=error` for `GetPrices()` and `unitPriceAndCurrency()` failures in `internal/pricing/calculator.go`
- [x] T018 [US3] Add success log (Info level) with `region`, `sku`, `resource_type`, `cost_monthly`, `currency`, `unit_price`, `result_status=success` in `internal/pricing/calculator.go`
- [x] T019 [US3] Add cachedClient nil guard log (Warn level) with `result_status=error` in `internal/pricing/calculator.go`
- [x] T020 Run `make test` to verify logging additions don't break existing tests

**Checkpoint**: All `GetProjectedCost` code paths produce structured log entries at correct severity levels.

---

## Phase 4: US4 — Graceful Degradation Without Cache (Priority: P3)

**Goal**: Verify the nil cachedClient guard returns `Unimplemented` with a descriptive message. This behavior already exists from the current implementation; this phase confirms it survives refactoring.

**Independent Test**: Create Calculator without cached client, send valid request, assert `Unimplemented`.

- [x] T021 [US4] Verify `TestGetProjectedCostReturnsUnimplemented` still passes after all refactoring by running `go test -run TestGetProjectedCostReturnsUnimplemented -v ./internal/pricing/`

**Checkpoint**: US4 confirmed — nil cachedClient returns Unimplemented with logging.

---

## Phase 5: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, quality gates, and final validation

### Constitution Compliance Tasks

- [x] T022 [P] Update godoc comment on `GetProjectedCost()` method to document supported resource types, error codes, and response fields in `internal/pricing/calculator.go`
- [x] T023 [P] Update CLAUDE.md with `GetProjectedCost` section documenting response fields, supported resource types, error codes, and attribute aliases
- [x] T024 [P] Run `make lint` and fix all linting issues (use extended timeout >5min)
- [x] T025 [P] Run `make test` with race detector to verify thread safety
- [x] T026 Verify docstring coverage ≥80% for `internal/pricing` package

### Quality Gates

- [x] T027 Run `make build` and verify success
- [x] T028 Run full `make test` and verify all tests pass
- [x] T029 Verify no new files need to be added to `.gitignore`

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — verification only
- **Phase 2 (US1+US2)**: Depends on Phase 1 confirmation — BLOCKS all subsequent phases
- **Phase 3 (US3)**: Depends on Phase 2 (logging attaches to refactored code paths)
- **Phase 4 (US4)**: Depends on Phase 2 (verifies refactoring preserved guard)
- **Phase 5 (Polish)**: Depends on Phases 2, 3, 4

### User Story Dependencies

- **US1+US2 (P1)**: Can start after Phase 1 — no dependencies on other stories
- **US3 (P2)**: Depends on US1+US2 (logging hooks into the refactored code)
- **US4 (P3)**: Depends on US1+US2 (verifies guard survives refactoring)
- US3 and US4 can run in parallel after US1+US2 completes

### Within Phase 2 (US1+US2)

- T004-T007 (tests) MUST be written first and FAIL before T008
- T008-T010 (implementation) are sequential within the same function
- T011-T012 (fixture fixes) depend on T008 (refactored code)
- T013 (dead code removal) depends on T011-T012 (all callers migrated)
- T014 (full test run) depends on all above

### Parallel Opportunities

```text
Phase 2 tests (can run in parallel since they're in the same file
but test different scenarios — write all before implementing):
  T004 (validation table) || T005 (success) || T006 (billing_detail) || T007 (API error)

Phase 3 logging (test first, then sequential implementation):
  T015a (test) → T015 → T016 → T017 → T018 → T019

Phase 5 polish (all parallel — different files/concerns):
  T022 || T023 || T024 || T025 || T026
```

---

## Parallel Example: Phase 2 Tests

```text
# Write all test functions in parallel (same file, different test functions):
T004: "Table-driven validation test in internal/pricing/calculator_test.go"
T005: "Success response test in internal/pricing/calculator_test.go"
T006: "billing_detail format test in internal/pricing/calculator_test.go"
T007: "API error propagation test in internal/pricing/calculator_test.go"
```

---

## Implementation Strategy

### MVP First (Phase 2: US1+US2 Only)

1. Complete Phase 1: Verify infrastructure (read-only)
2. Complete Phase 2: US1+US2 — TDD tests then implementation
3. **STOP and VALIDATE**: `make test` passes, VM projected cost works
4. This alone delivers the core value: production-ready GetProjectedCost

### Incremental Delivery

1. Phase 2 (US1+US2) → Core functionality (MVP)
2. Phase 3 (US3) → Observability (production readiness)
3. Phase 4 (US4) → Guard verification (confidence)
4. Phase 5 (Polish) → Documentation and quality gates

---

## Notes

- US1 and US2 are combined because they share `GetProjectedCost()` — validation errors and success path are branches of the same function
- US4 is minimal because the nil cachedClient guard already exists; this phase just confirms it survives refactoring
- All test tasks follow TDD: write tests first, verify they fail, then implement
- The `projectedQueryFromRequest()` removal (T013) is the final step after all consumers are migrated
- Logging (Phase 3) is separated from implementation (Phase 2) to keep each phase focused and reviewable
