# Tasks: OData Filter Query Builder

**Input**: Design documents from `/specs/009-odata-filter-builder/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/filter_builder.go

**Tests**: Included per constitution (TDD is NON-NEGOTIABLE).
Tests MUST be written FIRST and FAIL before implementation.

**Organization**: Tasks grouped by user story. US1+US2 are
combined (both P1, tightly coupled — US1 acceptance scenarios
reference default type from US2).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no deps)
- **[Story]**: User story label (US1, US2, US3, US4)
- All paths relative to repository root

---

## Phase 1: Setup

**Purpose**: Create new files for the FilterBuilder

- [X] T001 Create `internal/azureclient/filter.go` with
  package `azureclient` declaration, package-level doc comment,
  and stdlib imports (`fmt`, `sort`, `strings`)
- [X] T002 Create `internal/azureclient/filter_test.go` with
  package `azureclient` declaration and `testing` import

**Checkpoint**: Two empty files exist, `make build` passes

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and helpers needed by ALL user stories

**CRITICAL**: No user story work can begin until this phase
is complete

### Tests for Foundational

> **NOTE: Write these tests FIRST, ensure they FAIL**

- [X] T003 [P] Write table-driven tests for `escapeODataValue`
  in `internal/azureclient/filter_test.go` — cases: empty
  string, no quotes, single quote, multiple quotes, consecutive
  quotes, value with spaces
- [X] T004 [P] Write table-driven tests for `isBlank` in
  `internal/azureclient/filter_test.go` — cases: empty string,
  spaces only, tabs only, mixed whitespace, non-blank value,
  value with leading/trailing spaces
- [X] T005 [P] Write table-driven tests for package-level
  constructors (`Region`, `Service`, `SKU`, `PriceType`,
  `ProductName`, `CurrencyCode`, `Condition`) in
  `internal/azureclient/filter_test.go` — verify each returns
  correct `FilterCondition` with expected Field and Value

### Implementation for Foundational

- [X] T006 Implement `FilterCondition` exported struct type
  with `Field` and `Value` string fields, and all 7
  package-level constructor functions (`Region`, `Service`,
  `SKU`, `PriceType`, `ProductName`, `CurrencyCode`, `Condition`) in
  `internal/azureclient/filter.go`
- [X] T007 Implement `escapeODataValue` (doubles single quotes)
  and `isBlank` (empty or whitespace-only check) unexported
  helpers in `internal/azureclient/filter.go`
- [X] T008 Verify foundational tests pass with `make test`

**Checkpoint**: FilterCondition type, constructors, and helpers
all tested and working

---

## Phase 3: US1 + US2 — Simple Filters + Default Type (P1)

**Goal**: Construct AND-joined OData filter expressions from
named fields and generic fields, with automatic default
`priceType eq 'Consumption'` included unless overridden

**Independent Test**: Build a filter with Region("eastus") and
SKU("Standard_B1s"), verify output is
`armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s'
and priceType eq 'Consumption'`

### Tests for US1 + US2

> **NOTE: Write these tests FIRST, ensure they FAIL**

- [X] T009 [P] [US1] Write table-driven tests for single-field
  AND filters in `internal/azureclient/filter_test.go` — test
  each named method individually (Region, Service, SKU,
  ProductName, CurrencyCode) produces correct output with
  default type appended
- [X] T010 [P] [US1] Write table-driven tests for multi-field
  AND filters in `internal/azureclient/filter_test.go` — test
  combinations of 2, 3, and all 5 named fields; verify all
  criteria present, AND-joined, alphabetically sorted
- [X] T011 [P] [US2] Write table-driven tests for default
  Consumption type in `internal/azureclient/filter_test.go` —
  test: no type set produces default, explicit Type("Reservation")
  overrides, Type("") preserves default, Type("  ") preserves
  default
- [X] T012 [P] [US1] Write table-driven tests for generic
  Field() method in `internal/azureclient/filter_test.go` —
  test: Field("meterName","B1s") alongside Region, Field with
  empty name omitted, Field with empty value omitted, multiple
  Field calls for same name both included
- [X] T013 [P] [US1] Write test for empty/minimal build in
  `internal/azureclient/filter_test.go` — test:
  NewFilterBuilder().Build() returns
  `priceType eq 'Consumption'`
- [X] T014 [P] [US1] Write test for last-write-wins behavior
  in `internal/azureclient/filter_test.go` — test:
  Region("eastus").Region("westus2") produces only westus2

### Implementation for US1 + US2

- [X] T015 [US1] Implement `FilterBuilder` struct with
  unexported fields (`andConditions`, `orGroups`, `typeValue`,
  `genericFields`) and `NewFilterBuilder()` constructor in
  `internal/azureclient/filter.go`
- [X] T016 [US1] Implement named chainable methods (`Region`,
  `Service`, `SKU`, `ProductName`, `CurrencyCode`) that store
  AND conditions with last-write-wins in
  `internal/azureclient/filter.go`
- [X] T017 [US2] Implement `Type()` method that sets
  `typeValue` override (skip if blank) in
  `internal/azureclient/filter.go`
- [X] T018 [US1] Implement `Field()` method that appends to
  `genericFields` slice (skip if name or value blank) in
  `internal/azureclient/filter.go`
- [X] T019 [US1] Implement `Build()` method for AND-only logic:
  collect andConditions + genericFields + default/override
  priceType, escape values, sort alphabetically by field name,
  join with ` and ` in `internal/azureclient/filter.go`
- [X] T020 [US1] Verify all US1+US2 tests pass with `make test`

**Checkpoint**: Simple AND filters with default type work.
All 4 US1 and 3 US2 acceptance scenarios pass.

---

## Phase 4: US3 — Fluent API with OR Logic (P2)

**Goal**: Support OR-grouped conditions with automatic
parenthesization, chainable in a single expression, with
deterministic output regardless of call order

**Independent Test**: Build
`Or(Region("eastus"),Region("westus2")).Service("VMs").Build()`
and verify output is
`(armRegionName eq 'eastus' or armRegionName eq 'westus2')
and priceType eq 'Consumption' and serviceName eq 'VMs'`

### Tests for US3

> **NOTE: Write these tests FIRST, ensure they FAIL**

- [X] T021 [P] [US3] Write table-driven tests for `Or()` groups
  in `internal/azureclient/filter_test.go` — test: two regions
  OR-joined, OR group parenthesized, single-condition OR (no
  parens needed), empty conditions in OR omitted, all-empty OR
  group omitted entirely
- [X] T022 [P] [US3] Write table-driven tests for mixed AND/OR
  in `internal/azureclient/filter_test.go` — test: OR group
  with AND conditions, multiple OR groups, OR groups
  parenthesized correctly when mixed with AND
- [X] T023 [P] [US3] Write tests for deterministic ordering in
  `internal/azureclient/filter_test.go` — test: same methods
  called in different orders produce identical output, OR group
  sorted by first condition's field name
- [X] T024 [P] [US3] Write tests for fluent method chaining in
  `internal/azureclient/filter_test.go` — test: full chain
  `NewFilterBuilder().Region().Service().SKU().Type().Build()`
  in single expression, verify return type is `*FilterBuilder`
  for all methods

### Implementation for US3

- [X] T025 [US3] Implement `Or()` method that appends valid
  (non-blank) conditions to `orGroups` slice in
  `internal/azureclient/filter.go`
- [X] T026 [US3] Update `Build()` to render OR groups as
  parenthesized expressions joined by ` or `, include in
  top-level parts sorted by primary field name, and join all
  parts with ` and ` in `internal/azureclient/filter.go`
- [X] T027 [US3] Verify all US3 tests pass with `make test`

**Checkpoint**: Fluent chaining with AND/OR works. All 4 US3
acceptance scenarios pass.

---

## Phase 5: US4 — Safe Value Handling (P2)

**Goal**: Ensure special characters, empty values, and edge
cases are handled correctly in filter output

**Independent Test**: Build a filter with value `O'Brien` and
verify output contains `'O''Brien'` (doubled single quote)

### Tests for US4

> **NOTE: Write these tests FIRST, ensure they FAIL**

- [X] T028 [P] [US4] Write table-driven tests for single quote
  escaping in full filter output in
  `internal/azureclient/filter_test.go` — test:
  Region("O'Brien") produces `armRegionName eq 'O''Brien'`,
  multiple quotes, value with both quotes and spaces
- [X] T029 [P] [US4] Write table-driven tests for edge cases
  in `internal/azureclient/filter_test.go` — test:
  whitespace-only Region("  ") omitted, empty string
  Field("","val") omitted, all fields empty returns minimal
  filter, same field set twice last-write-wins, Build()
  idempotent (call twice, same result)

### Implementation for US4

- [X] T030 [US4] Review and verify that `Build()` in
  `internal/azureclient/filter.go` calls `escapeODataValue`
  on all values and `isBlank` to omit empty/whitespace — add
  any missing edge case handling
- [X] T031 [US4] Verify all US4 tests pass with `make test`

**Checkpoint**: All edge cases handled. All 3 US4 acceptance
scenarios pass.

---

## Phase 6: Client Integration

**Purpose**: Refactor existing HTTP client to use FilterBuilder

- [X] T032 Refactor `buildFilterQuery()` in
  `internal/azureclient/client.go` to use `NewFilterBuilder()`
  with `Region()`, `Service()`, `SKU()`, `ProductName()`,
  `CurrencyCode()` from the `PriceQuery` struct fields, then
  call `Build()` — delete the old manual filter construction
  and the now-redundant local `escapeODataString` function
- [X] T033 Update existing filter tests in
  `internal/azureclient/client_test.go` to account for the new
  default `priceType eq 'Consumption'` that is now included in
  all filter output — update expected strings and assertions
- [X] T034 Run full test suite with `make test` to verify no
  regressions in client, retry, or integration tests

**Checkpoint**: Existing `GetPrices()` works with FilterBuilder
internally. No API changes, no regressions.

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Constitution compliance and documentation

### Constitution Compliance Tasks

- [X] T035 [P] Run `make lint` and fix all linting issues in
  `internal/azureclient/filter.go` and
  `internal/azureclient/filter_test.go`
- [X] T036 [P] Run `go test -race ./internal/azureclient/...`
  to verify thread safety
- [X] T037 [P] Verify test coverage is at least 80% for
  `internal/azureclient/filter.go` with
  `go test -cover ./internal/azureclient/...`
- [X] T038 [P] Verify all exported types, functions, and
  methods in `internal/azureclient/filter.go` have godoc
  comments — ensure docstring coverage is at least 80%
- [X] T039 [P] Verify architectural constraints: no
  authenticated Azure APIs, no persistent storage, no
  infrastructure mutation, no bulk data embedding
- [X] T042 [P] Write a `BenchmarkBuild` function in
  `internal/azureclient/filter_test.go` that exercises
  `NewFilterBuilder()` with 10 criteria (5 named + 3 generic +
  1 Or group + 1 Type override), calls `Build()`, and validates
  SC-001 performance target (<1ms) using `b.Elapsed()`

### Documentation Tasks

- [X] T040 Update CLAUDE.md `## Azure Client` section with
  FilterBuilder usage examples showing `NewFilterBuilder()`,
  named methods, `Or()`, `Field()`, and `Build()`
- [X] T041 Validate `specs/009-odata-filter-builder/quickstart.md`
  examples match actual `Build()` output — run each example
  mentally or as a test

---

## Dependencies and Execution Order

### Phase Dependencies

```text
Phase 1 (Setup) ─────────▶ Phase 2 (Foundational)
                                    │
                                    ▼
                           Phase 3 (US1+US2)
                                    │
                            ┌───────┴───────┐
                            ▼               ▼
                     Phase 4 (US3)   Phase 5 (US4)
                            │               │
                            └───────┬───────┘
                                    ▼
                           Phase 6 (Integration)
                                    │
                                    ▼
                           Phase 7 (Polish)
```

### User Story Dependencies

- **US1+US2 (P1)**: Depends on Foundational (Phase 2).
  No dependencies on other stories.
- **US3 (P2)**: Depends on US1+US2 (needs FilterBuilder
  struct and Build() to exist). Adds Or() and updates Build().
- **US4 (P2)**: Depends on US1+US2 (needs Build() to exist).
  Can run in parallel with US3 since it validates existing
  behavior rather than adding new methods.
- **Integration (Phase 6)**: Depends on all user stories
  complete.

### Within Each User Story

1. Tests MUST be written and FAIL before implementation
2. Types/structs before methods
3. Methods before Build() logic
4. Verify tests pass after implementation

### Parallel Opportunities

- **Phase 2**: T003, T004, T005 (tests) can run in parallel
- **Phase 3**: T009-T014 (tests) can run in parallel
- **Phase 4**: T021-T024 (tests) can run in parallel
- **Phase 5**: T028, T029 (tests) can run in parallel
- **Phase 4 and Phase 5**: Can run in parallel after Phase 3
- **Phase 7**: T035-T039 and T042 can all run in parallel

---

## Parallel Example: Phase 3 (US1+US2)

```text
# Write all US1+US2 tests in parallel (same file, diff funcs):
T009: Tests for single-field AND filters
T010: Tests for multi-field AND filters
T011: Tests for default type and override
T012: Tests for generic Field() method
T013: Test for empty/minimal build
T014: Test for last-write-wins behavior

# Then implement sequentially:
T015: FilterBuilder struct + NewFilterBuilder
T016: Named methods (Region, Service, SKU, etc.)
T017: Type() method
T018: Field() method
T019: Build() for AND-only
T020: Verify all tests pass
```

---

## Implementation Strategy

### MVP First (US1+US2 Only)

1. Complete Phase 1: Setup (2 tasks)
2. Complete Phase 2: Foundational (6 tasks)
3. Complete Phase 3: US1+US2 (12 tasks)
4. **STOP and VALIDATE**: Run `make test`, verify all
   acceptance scenarios
5. This delivers core AND filtering with default type

### Incremental Delivery

1. Setup + Foundational -> Core types ready
2. US1+US2 -> AND filters work (MVP)
3. US3 -> OR logic + fluent chaining added
4. US4 -> Edge cases validated
5. Integration -> Client uses FilterBuilder
6. Polish -> Constitution compliant

### Task Summary

| Phase        | Tasks   | Story    | Parallel      |
|--------------|---------|----------|---------------|
| Setup        | T001-02 | —        | No            |
| Foundational | T003-08 | —        | T003-05       |
| US1+US2      | T009-20 | US1, US2 | T009-14       |
| US3          | T021-27 | US3      | T021-24       |
| US4          | T028-31 | US4      | T028-29       |
| Integration  | T032-34 | —        | No            |
| Polish       | T035-42 | —        | T035-39,T042  |
| **Total**    | **42**  |          |               |

### Suggested MVP Scope

Phase 1 + Phase 2 + Phase 3 (US1+US2) = 20 tasks.
Delivers core AND filtering with default pricing type,
which is the primary use case covering both P1 stories.

---

## Notes

- [P] tasks = different functions in same file, no deps
- [US*] label maps task to specific user story
- All tests are table-driven per constitution
- `make test` includes `-race` flag
- `make lint` timeout may exceed 5 minutes
- Single quotes in values: `'` becomes `''` (OData v4)
- Default type `priceType eq 'Consumption'` is a behavioral
  change from existing `buildFilterQuery()` — update tests
