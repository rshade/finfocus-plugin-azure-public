# Tasks: Cost Calculation Utilities

**Input**: Design documents from `/specs/019-cost-utilities/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: Included — constitution mandates TDD (tests before
implementation) with ≥80% coverage for business logic.

**Organization**: Tasks grouped by user story for independent
implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create the `internal/estimation` package structure

- [X] T001 Create package directory `internal/estimation/`
- [X] T002 Create `internal/estimation/cost.go` with package declaration
  and godoc package comment describing cost conversion utilities

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Constants and rounding helper that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational

> **NOTE: Write these tests FIRST, ensure they FAIL before
> implementation**

- [X] T003 Create `internal/estimation/cost_test.go` with table-driven
  tests for `roundCurrency`: cases for exact values (73.00), half-cent
  boundary (0.105 → 0.11), near-zero (0.004 → 0.00), negative
  (-0.105 → -0.11), and zero (0.00 → 0.00)

### Implementation for Foundational

- [X] T004 Define exported constants `HoursPerMonth = 730` and
  `HoursPerYear = 8760` with godoc comments in
  `internal/estimation/cost.go`
- [X] T005 Implement unexported `roundCurrency(amount float64) float64`
  helper using `math.Round(amount * 100) / 100` in
  `internal/estimation/cost.go`
- [X] T006 Run `go test ./internal/estimation/...` and verify T003 tests
  pass

**Checkpoint**: Constants and rounding helper verified — user story
implementation can begin

---

## Phase 3: User Story 1 - Hourly to Monthly/Yearly (Priority: P1) MVP

**Goal**: Convert hourly VM rates to monthly and yearly cost estimates

**Independent Test**: Provide hourly rate $0.10 → verify $73.00/mo and
$876.00/yr

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before
> implementation**

- [X] T007 [US1] Add table-driven tests for `HourlyToMonthly` in
  `internal/estimation/cost_test.go`: cases for standard rate
  (0.10 → 73.00), zero (0.00 → 0.00), negative (-0.10 → -73.00),
  large rate (10000.00 → 7300000.00), small rate (0.001 → 0.73),
  and rounding (0.105 → 76.65)
- [X] T008 [P] [US1] Add table-driven tests for `HourlyToYearly` in
  `internal/estimation/cost_test.go`: cases for standard rate
  (0.10 → 876.00), zero (0.00 → 0.00), negative (-0.10 → -876.00),
  large rate (10000.00 → 87600000.00), and rounding

### Implementation for User Story 1

- [X] T009 [US1] Implement `HourlyToMonthly(hourly float64) float64`
  with godoc comment in `internal/estimation/cost.go`
- [X] T010 [US1] Implement `HourlyToYearly(hourly float64) float64`
  with godoc comment in `internal/estimation/cost.go`
- [X] T011 [US1] Run `go test ./internal/estimation/...` and verify
  T007 and T008 tests pass

**Checkpoint**: Hourly-to-monthly and hourly-to-yearly conversions
work independently

---

## Phase 4: User Story 2 - Monthly to Hourly (Priority: P2)

**Goal**: Convert monthly disk/storage rates to hourly for normalization

**Independent Test**: Provide monthly rate $730.00 → verify $1.00/hr

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before
> implementation**

- [X] T012 [US2] Add table-driven tests for `MonthlyToHourly` in
  `internal/estimation/cost_test.go`: cases for standard rate
  (730.00 → 1.00), zero (0.00 → 0.00), negative (-730.00 → -1.00),
  fractional result (5.00 → 0.01), and small rate (0.73 → 0.00)

### Implementation for User Story 2

- [X] T013 [US2] Implement `MonthlyToHourly(monthly float64) float64`
  with godoc comment in `internal/estimation/cost.go`
- [X] T014 [US2] Run `go test ./internal/estimation/...` and verify
  T012 tests pass

**Checkpoint**: All three conversion directions work independently

---

## Phase 5: User Story 3 - Rounding Verification (Priority: P2)

**Goal**: Verify all conversions produce exactly two decimal places
across boundary inputs

**Independent Test**: Provide inputs at half-cent boundaries and verify
rounding consistency

### Tests for User Story 3

- [X] T015 [US3] Add cross-function rounding verification tests in
  `internal/estimation/cost_test.go`: verify round-trip consistency
  (MonthlyToHourly then HourlyToMonthly preserves value for known
  inputs), verify no result exceeds two decimal places across a range
  of inputs (0.001 to 10000.0 in increments)
- [X] T016 [US3] Run `go test ./internal/estimation/...` and verify
  all tests pass including T015

**Checkpoint**: All user stories verified — rounding is consistent
across all conversion functions

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Quality gates, documentation, and compliance

### Constitution Compliance Tasks

- [X] T017 [P] Run `make lint` and fix any linting issues
- [X] T018 [P] Run `go test -race ./internal/estimation/...` to verify
  thread safety
- [X] T019 [P] Verify test coverage ≥80% with
  `go test -cover ./internal/estimation/...`
- [X] T020 [P] Verify all exported symbols in
  `internal/estimation/cost.go` have godoc comments (docstring
  coverage ≥80%)
- [X] T021 Update CLAUDE.md with `internal/estimation` package
  documentation

### Final Validation

- [X] T022 Run `make test` to verify full test suite passes
- [X] T023 Run `make build` to verify clean build

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all stories
- **US1 (Phase 3)**: Depends on Phase 2
- **US2 (Phase 4)**: Depends on Phase 2 (independent of US1)
- **US3 (Phase 5)**: Depends on Phase 3 and Phase 4 (needs all
  functions implemented for cross-function tests)
- **Polish (Phase 6)**: Depends on Phase 5

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational only — no other story
  dependencies
- **US2 (P2)**: Depends on Foundational only — can run in parallel
  with US1
- **US3 (P2)**: Depends on US1 and US2 — cross-function verification

### Parallel Opportunities

- T007 and T008 can run in parallel (different test functions, same
  file — but no conflicts since both are additive)
- T017, T018, T019, T020 can all run in parallel (different tools)
- US1 and US2 could be implemented in parallel since they are
  independent (but share a file, so sequential is simpler)

---

## Parallel Example: User Story 1

```text
# Write tests first (T007 and T008 can be written in parallel):
Task: "Add HourlyToMonthly tests in internal/estimation/cost_test.go"
Task: "Add HourlyToYearly tests in internal/estimation/cost_test.go"

# Then implement (T009 and T010 are sequential in same file):
Task: "Implement HourlyToMonthly in internal/estimation/cost.go"
Task: "Implement HourlyToYearly in internal/estimation/cost.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T006)
3. Complete Phase 3: User Story 1 (T007-T011)
4. **STOP and VALIDATE**: `go test ./internal/estimation/...` — hourly
   conversions work
5. Usable immediately by `internal/pricing.Calculator.EstimateCost()`

### Incremental Delivery

1. Setup + Foundational → constants and rounding ready
2. Add US1 → hourly conversions work → MVP
3. Add US2 → monthly normalization works → full feature
4. Add US3 → rounding verified across all paths → confidence
5. Polish → lint, coverage, docs → merge-ready

---

## Notes

- All functions live in a single file (`cost.go`) — parallelization
  is limited within implementation tasks but tests can be written
  concurrently
- TDD is required by constitution — tests MUST fail before
  implementation
- No external dependencies — only `math` stdlib
- Commit after each phase checkpoint
