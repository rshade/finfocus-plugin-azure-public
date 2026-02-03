# Tasks: CostSourceService Method Stubs

**Input**: Design documents from `/specs/004-costsource-stubs/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: TDD required per constitution - tests written BEFORE implementation

**Organization**: Tasks grouped by user story for independent implementation/testing

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Project type**: Single Go module
- **Source**: `internal/pricing/calculator.go`
- **Tests**: `internal/pricing/calculator_test.go`

---

## Phase 1: Setup

**Purpose**: Verify existing infrastructure and imports

- [x] T001 Verify grpc/codes and grpc/status imports in internal/pricing/calculator.go
- [x] T002 Verify existing tests pass with `make test`

---

## Phase 2: Foundational

**Purpose**: No foundational work needed - Calculator struct already exists with
embedded UnimplementedCostSourceServiceServer

**Checkpoint**: Ready for user story implementation

---

## Phase 3: User Story 1 - Plugin Identity Query (Priority: P1) MVP

**Goal**: Name() RPC returns NameResponse, GetPluginInfo() returns complete metadata

**Independent Test**: Start plugin, call Name() RPC, verify response contains
plugin name

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T003 [P] [US1] Add test for Name() RPC in internal/pricing/calculator_test.go
- [x] T004 [P] [US1] Enhance GetPluginInfo() test for spec_version and providers in internal/pricing/calculator_test.go

### Implementation for User Story 1

- [x] T005 [US1] Implement Name(ctx, req) RPC method in internal/pricing/calculator.go
- [x] T006 [US1] Update GetPluginInfo() to return spec_version and providers in internal/pricing/calculator.go
- [x] T007 [US1] Add godoc comment for Name() method in internal/pricing/calculator.go

**Checkpoint**: Name() and GetPluginInfo() work correctly with full metadata

---

## Phase 4: User Story 2 - Resource Support Query (Priority: P2)

**Goal**: Supports() RPC returns supported=false with reason

**Independent Test**: Call Supports() with any resource type, verify supported=false

### Tests for User Story 2

- [x] T008 [P] [US2] Add test for Supports() returns false in internal/pricing/calculator_test.go
- [x] T009 [P] [US2] Add test for Supports() with nil request in internal/pricing/calculator_test.go

### Implementation for User Story 2

- [x] T010 [US2] Implement Supports() RPC method in internal/pricing/calculator.go
- [x] T011 [US2] Add godoc comment for Supports() method in internal/pricing/calculator.go

**Checkpoint**: Supports() responds correctly for any input

---

## Phase 5: User Story 3 - Cost Estimation Request (Priority: P3)

**Goal**: EstimateCost() returns Unimplemented status (update existing stub)

**Independent Test**: Call EstimateCost(), verify Unimplemented gRPC status

### Tests for User Story 3

- [x] T012 [US3] Update EstimateCost() test to verify Unimplemented status in internal/pricing/calculator_test.go

### Implementation for User Story 3

- [x] T013 [US3] Update EstimateCost() to return Unimplemented status in internal/pricing/calculator.go
- [x] T014 [US3] Add godoc comment for EstimateCost() explaining stub in internal/pricing/calculator.go

**Checkpoint**: EstimateCost() returns proper Unimplemented status

---

## Phase 6: User Story 4 - Plugin Stability (Priority: P4)

**Goal**: All remaining RPCs return Unimplemented without crashing

**Independent Test**: Call each of 7 remaining methods, verify Unimplemented status

### Tests for User Story 4

- [x] T015 [P] [US4] Add test for GetActualCost() returns Unimplemented in internal/pricing/calculator_test.go
- [x] T016 [P] [US4] Add test for GetProjectedCost() returns Unimplemented in internal/pricing/calculator_test.go
- [x] T017 [P] [US4] Add test for GetPricingSpec() returns Unimplemented in internal/pricing/calculator_test.go
- [x] T018 [P] [US4] Add test for GetRecommendations() returns Unimplemented in internal/pricing/calculator_test.go
- [x] T019 [P] [US4] Add test for DismissRecommendation() returns Unimplemented in internal/pricing/calculator_test.go
- [x] T020 [P] [US4] Add test for GetBudgets() returns Unimplemented in internal/pricing/calculator_test.go
- [x] T021 [P] [US4] Add test for DryRun() returns Unimplemented in internal/pricing/calculator_test.go

### Implementation for User Story 4

- [x] T022 [P] [US4] Implement GetActualCost() stub in internal/pricing/calculator.go
- [x] T023 [P] [US4] Implement GetProjectedCost() stub in internal/pricing/calculator.go
- [x] T024 [P] [US4] Implement GetPricingSpec() stub in internal/pricing/calculator.go
- [x] T025 [P] [US4] Implement GetRecommendations() stub in internal/pricing/calculator.go
- [x] T026 [P] [US4] Implement DismissRecommendation() stub in internal/pricing/calculator.go
- [x] T027 [P] [US4] Implement GetBudgets() stub in internal/pricing/calculator.go
- [x] T028 [P] [US4] Implement DryRun() stub in internal/pricing/calculator.go

**Checkpoint**: All 11 RPC methods respond without crashing

---

## Phase 7: Polish & Validation

**Purpose**: Constitution compliance and final validation

### Constitution Compliance Tasks

- [x] T029 [P] Run `make lint` and fix all linting issues
- [x] T030 [P] Run `go test -race ./...` to verify thread safety
- [x] T031 [P] Verify test coverage >=80% with `go test -cover ./internal/pricing/...`
- [x] T032 [P] Verify all godoc comments present on new methods
- [x] T033 Verify structured logging (zerolog) on all RPC handlers

### Final Validation

- [x] T034 Run full test suite with `make test`
- [x] T035 Build plugin with `make build`
- [x] T036 Manual smoke test: start plugin, verify PORT output

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **Foundational (Phase 2)**: N/A for this feature
- **User Stories (Phase 3-6)**: Each depends on Setup; can run in priority order
- **Polish (Phase 7)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: No dependencies - Identity methods foundation for all others
- **US2 (P2)**: No dependencies on US1 (independent method)
- **US3 (P3)**: No dependencies (modifying existing method)
- **US4 (P4)**: No dependencies (independent stub methods)

### Within Each User Story

1. Tests MUST be written and FAIL before implementation
2. Implementation follows test completion
3. Godoc comments after implementation verified

### Parallel Opportunities

**Phase 3 (US1)**:

- T003 and T004 can run in parallel (different test functions)

**Phase 4 (US2)**:

- T008 and T009 can run in parallel (different test cases)

**Phase 6 (US4)** - Maximum parallelism:

- All 7 test tasks (T015-T021) can run in parallel
- All 7 implementation tasks (T022-T028) can run in parallel

**Phase 7 (Polish)**:

- T029, T030, T031, T032 can all run in parallel

---

## Parallel Example: User Story 4

```bash
# Launch all US4 tests together:
Task: "Add test for GetActualCost() returns Unimplemented"
Task: "Add test for GetProjectedCost() returns Unimplemented"
Task: "Add test for GetPricingSpec() returns Unimplemented"
Task: "Add test for GetRecommendations() returns Unimplemented"
Task: "Add test for DismissRecommendation() returns Unimplemented"
Task: "Add test for GetBudgets() returns Unimplemented"
Task: "Add test for DryRun() returns Unimplemented"

# Then launch all US4 implementations together:
Task: "Implement GetActualCost() stub"
Task: "Implement GetProjectedCost() stub"
Task: "Implement GetPricingSpec() stub"
Task: "Implement GetRecommendations() stub"
Task: "Implement DismissRecommendation() stub"
Task: "Implement GetBudgets() stub"
Task: "Implement DryRun() stub"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 3: User Story 1 (T003-T007)
3. **STOP and VALIDATE**: Test Name() and GetPluginInfo() independently
4. Plugin can be deployed with identity capability

### Incremental Delivery

1. US1 complete -> Plugin has identity
2. Add US2 -> Plugin reports support status
3. Add US3 -> EstimateCost returns proper error
4. Add US4 -> All methods stable
5. Each increment adds capability without breaking previous

### Full Implementation (Recommended)

Given simple stub nature, implement all stories sequentially:

1. T001-T002: Setup verification
2. T003-T007: US1 (Identity)
3. T008-T011: US2 (Supports)
4. T012-T014: US3 (EstimateCost)
5. T015-T028: US4 (Remaining stubs) - maximum parallelism here
6. T029-T036: Polish and validation

---

## Task Summary

| Phase | User Story | Task Count | Parallel Tasks |
| ----- | ---------- | ---------- | -------------- |
| 1 | Setup | 2 | 0 |
| 2 | Foundational | 0 | 0 |
| 3 | US1 (Identity) | 5 | 2 |
| 4 | US2 (Supports) | 4 | 2 |
| 5 | US3 (EstimateCost) | 3 | 0 |
| 6 | US4 (Stability) | 14 | 14 |
| 7 | Polish | 8 | 4 |
| **Total** | | **36** | **22** |

---

## Notes

- All methods edit the same two files (calculator.go, calculator_test.go)
- US4 has maximum parallelism (14 tasks across 7 methods)
- Tests use existing pluginsdk.NewTestPlugin helper where possible
- Unimplemented status uses `status.Error(codes.Unimplemented, "not yet implemented")`
- Commit after each user story phase for clean history
- Plugin name is "azure-public" (consistent with existing GetPluginInfo())

## Edge Case Coverage

Edge cases from spec.md are covered by existing tasks:

- Malformed protobuf: gRPC framework handles automatically (no task needed)
- Nil context: T009 tests Supports() with nil request
- Concurrent calls: T030 runs race detector on all code
- Startup timing: T036 smoke test verifies immediate response after PORT output
