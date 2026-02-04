# Tasks: Azure Retail Prices API Data Models

**Input**: Design documents from `/specs/007-azure-price-models/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, quickstart.md ✅

**Tests**: TDD workflow specified in constitution - tests FIRST, ≥80% coverage target.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Project structure**: `internal/azureclient/` (existing package)
- Files to modify: `types.go`, `client.go`
- Files to create: `types_test.go`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: No setup required - modifying existing codebase

*This feature enhances existing code. No project initialization needed.*

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Export PriceResponse struct - blocks all user story tests

**⚠️ CRITICAL**: User story tests cannot use `PriceResponse` until it's exported

- [x] T001 Export `priceResponse` as `PriceResponse` in internal/azureclient/types.go
- [x] T002 Update client.go to use exported `PriceResponse` in internal/azureclient/client.go

**Checkpoint**: PriceResponse is now exported and usable by tests

---

## Phase 3: User Story 1 - HTTP Client Response Parsing (Priority: P1) 🎯 MVP

**Goal**: Unmarshal JSON responses from Azure Retail Prices API into strongly-typed Go structs

**Independent Test**: Provide sample JSON from Azure API and verify all fields are correctly populated

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T003 [P] [US1] Create test file with TestPriceResponse_UnmarshalJSON in internal/azureclient/types_test.go
- [x] T004 [P] [US1] Add TestPriceItem_UnmarshalJSON_AllFields to verify all 21 fields unmarshal correctly in internal/azureclient/types_test.go
- [x] T005 [P] [US1] Add TestPriceItem_UnmarshalJSON_MissingOptionalFields for graceful null/missing handling in internal/azureclient/types_test.go (Note: Go's encoding/json handles unknown fields by ignoring them and null numerics as zero values by default - document this behavior in test comments)

### Implementation for User Story 1

- [x] T006 [US1] Add `Location` field to PriceItem struct in internal/azureclient/types.go
- [x] T007 [US1] Add `ReservationTerm` field with omitempty tag to PriceItem struct in internal/azureclient/types.go
- [x] T008 [US1] Add `AvailabilityID` field with omitempty tag to PriceItem struct in internal/azureclient/types.go
- [x] T009 [US1] Verify all tests pass with `go test ./internal/azureclient/... -run TestPrice`

**Checkpoint**: JSON unmarshaling works for all Azure API response formats

---

## Phase 4: User Story 2 - Type-Safe Field Access (Priority: P1)

**Goal**: Ensure correct Go types for all pricing fields to enable calculations without type assertions

**Independent Test**: Write code accessing all fields and verify compile-time type checking

### Tests for User Story 2

- [x] T010 [P] [US2] Add TestPriceItem_TypeSafety to verify float64/string/bool types in internal/azureclient/types_test.go

### Verification for User Story 2 (No Code Changes - Checkpoint Only)

- [x] T011 [US2] **CHECKPOINT**: Confirm RetailPrice, UnitPrice, TierMinimumUnits are float64 in internal/azureclient/types.go
- [x] T012 [US2] **CHECKPOINT**: Confirm MeterID, ProductID, SkuID, ServiceID are string in internal/azureclient/types.go
- [x] T013 [US2] **CHECKPOINT**: Confirm IsPrimaryMeterRegion is bool in internal/azureclient/types.go

**Checkpoint**: All field types are correct for calculations and IDE support

---

## Phase 5: User Story 3 - Mock Data Construction (Priority: P2)

**Goal**: Enable test writers to construct mock pricing data and marshal to JSON

**Independent Test**: Create PriceItem structs and marshal to JSON, verify output format

### Tests for User Story 3

- [x] T014 [P] [US3] Add TestPriceItem_MarshalJSON to verify JSON output format in internal/azureclient/types_test.go
- [x] T015 [P] [US3] Add TestPriceItem_RoundTrip to verify unmarshal→marshal→unmarshal produces identical values in internal/azureclient/types_test.go
- [x] T016 [P] [US3] Add TestPriceResponse_MarshalJSON to verify envelope format in internal/azureclient/types_test.go

### Implementation for User Story 3

- [x] T017 [US3] Verify json tags on all PriceItem fields match Azure API camelCase format in internal/azureclient/types.go
- [x] T018 [US3] Verify json tags on all PriceResponse fields match Azure API PascalCase format in internal/azureclient/types.go
- [x] T019 [US3] Run round-trip tests and fix any JSON tag mismatches

**Checkpoint**: Structs can be marshaled to JSON matching Azure API format

---

## Phase 6: User Story 4 - Field Documentation (Priority: P3)

**Goal**: Add godoc comments to all struct fields explaining their purpose

**Independent Test**: Run `go doc` and verify each field has descriptive documentation

### Tests for User Story 4

*No automated tests - manual godoc review*

### Implementation for User Story 4

- [x] T020 [P] [US4] Add godoc comment to PriceResponse struct with example JSON in internal/azureclient/types.go
- [x] T021 [P] [US4] Add godoc comments to all PriceResponse fields in internal/azureclient/types.go
- [x] T022 [P] [US4] Add godoc comment to PriceItem struct with example JSON in internal/azureclient/types.go
- [x] T023 [P] [US4] Add godoc comments to all PriceItem pricing fields (RetailPrice, UnitPrice, etc.) in internal/azureclient/types.go
- [x] T024 [P] [US4] Add godoc comments to all PriceItem resource fields (ArmRegionName, Location, etc.) in internal/azureclient/types.go
- [x] T025 [P] [US4] Add godoc comments to all PriceItem service fields (ServiceName, ServiceFamily, etc.) in internal/azureclient/types.go
- [x] T026 [P] [US4] Add godoc comments to all PriceItem meter fields (MeterID, MeterName, etc.) in internal/azureclient/types.go
- [x] T027 [P] [US4] Add godoc comments to PriceItem optional fields (ReservationTerm, AvailabilityID) in internal/azureclient/types.go

**Checkpoint**: All exported types and fields have descriptive godoc comments

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and constitution compliance

### Constitution Compliance Tasks

- [x] T028 [P] Run `make lint` and fix all linting issues
- [x] T029 [P] Verify test coverage ≥80% with `go test -cover ./internal/azureclient/...`
- [x] T030 [P] Verify `go doc github.com/rshade/finfocus-plugin-azure-public/internal/azureclient` output is complete
- [x] T031 [P] Run existing client tests to ensure no regressions: `go test ./internal/azureclient/...`

### Final Validation

- [x] T032 Run full test suite with `make test`
- [x] T033 Update CLAUDE.md with new model documentation if needed

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: N/A - no setup needed
- **Phase 2 (Foundational)**: No dependencies - MUST complete before user stories
- **Phase 3 (US1)**: Depends on Phase 2 (needs exported PriceResponse for tests)
- **Phase 4 (US2)**: Depends on Phase 3 (type verification after fields added)
- **Phase 5 (US3)**: Depends on Phase 3 (marshal tests need complete fields)
- **Phase 6 (US4)**: Depends on Phase 3 (document all fields after they exist)
- **Phase 7 (Polish)**: Depends on all user stories complete

### User Story Dependencies

- **User Story 1 (P1)**: Foundational only - core parsing functionality
- **User Story 2 (P1)**: Can run parallel with US1 after foundational
- **User Story 3 (P2)**: Depends on US1 fields being complete
- **User Story 4 (P3)**: Can run parallel with US3 after US1

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation tasks can often run in parallel within a story
- Verification task runs last to confirm tests pass

### Parallel Opportunities

**Phase 2**:
- T001, T002 are sequential (T002 depends on T001)

**Phase 3 (US1)**:
- T003, T004, T005 can run in parallel (all create tests)
- T006, T007, T008 can run in parallel (different fields)

**Phase 5 (US3)**:
- T014, T015, T016 can run in parallel (all test tasks)

**Phase 6 (US4)**:
- T020-T027 can ALL run in parallel (different comment sections)

**Phase 7**:
- T028, T029, T030, T031 can all run in parallel

---

## Parallel Example: User Story 1 Tests

```bash
# Launch all US1 test tasks together:
Task: "Create test file with TestPriceResponse_UnmarshalJSON in internal/azureclient/types_test.go"
Task: "Add TestPriceItem_UnmarshalJSON_AllFields in internal/azureclient/types_test.go"
Task: "Add TestPriceItem_UnmarshalJSON_MissingOptionalFields in internal/azureclient/types_test.go"

# Then launch all US1 field additions together:
Task: "Add Location field to PriceItem struct in internal/azureclient/types.go"
Task: "Add ReservationTerm field to PriceItem struct in internal/azureclient/types.go"
Task: "Add AvailabilityID field to PriceItem struct in internal/azureclient/types.go"
```

---

## Parallel Example: User Story 4 Documentation

```bash
# Launch ALL documentation tasks together (all different comment sections):
Task: "Add godoc comment to PriceResponse struct in internal/azureclient/types.go"
Task: "Add godoc comments to all PriceResponse fields in internal/azureclient/types.go"
Task: "Add godoc comment to PriceItem struct in internal/azureclient/types.go"
Task: "Add godoc comments to all PriceItem pricing fields in internal/azureclient/types.go"
Task: "Add godoc comments to all PriceItem resource fields in internal/azureclient/types.go"
Task: "Add godoc comments to all PriceItem service fields in internal/azureclient/types.go"
Task: "Add godoc comments to all PriceItem meter fields in internal/azureclient/types.go"
Task: "Add godoc comments to PriceItem optional fields in internal/azureclient/types.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 & 2 Only)

1. Complete Phase 2: Export PriceResponse
2. Complete Phase 3: US1 - JSON unmarshaling
3. Complete Phase 4: US2 - Type safety verification
4. **STOP and VALIDATE**: Run `make test` - all tests pass
5. Feature is functional for HTTP client use

### Incremental Delivery

1. Phase 2 → Foundation ready
2. + US1 → JSON parsing works (MVP!)
3. + US2 → Type safety verified
4. + US3 → Mock data construction works
5. + US4 → Full documentation
6. + Polish → Production ready

### Single Developer Strategy

Execute in order: Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6 → Phase 7

Estimated task count per phase:
- Phase 2: 2 tasks
- Phase 3: 7 tasks
- Phase 4: 4 tasks
- Phase 5: 6 tasks
- Phase 6: 8 tasks
- Phase 7: 6 tasks
- **Total: 33 tasks**

---

## Notes

- [P] tasks = different files OR different sections, no dependencies
- [Story] label maps task to specific user story for traceability
- Existing tests in `client_test.go` already cover some marshaling - don't duplicate
- Focus `types_test.go` on dedicated model testing
- Use sample JSON from `research.md` for test data
- Commit after each completed user story phase
