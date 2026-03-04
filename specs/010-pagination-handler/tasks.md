# Tasks: Pagination Handler for Azure API Responses

**Input**: Design documents from `/specs/010-pagination-handler/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Included per spec SC-005 (>=80% coverage for pagination logic).

**Organization**: Tasks grouped by user story. US1 and US2 share P1 priority and the
same code path — listed as separate phases but can be implemented in a single pass.
All changes modify existing files in `internal/azureclient/`; no new files are created.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No setup needed — enhancement to existing package `internal/azureclient/`.

_No tasks in this phase._

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Change the pagination constant and update documentation to reflect the
new 10-page limit. All subsequent tests depend on this value.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T001 [P] Update `MaxPaginationPages` constant from 1000 to 10 in `internal/azureclient/errors.go`
- [X] T002 [P] Update `GetPrices()` docstring to reflect 10-page limit instead of 1000 in `internal/azureclient/client.go`

**Checkpoint**: Constant and docstring reflect the new 10-page limit. Run `make test` — all existing tests still pass.

---

## Phase 3: User Story 1 — Complete Result Retrieval (Priority: P1) MVP

**Goal**: Verify that queries returning multiple pages aggregate all items into a single result set.

**Independent Test**: Issue a query matching 250 items across 3 pages and verify all 250 items appear in the result.

### Tests for User Story 1

- [X] T003 [US1] Add test for 3-page query (250 items across 3 pages) returning all 250 items in `internal/azureclient/client_test.go`
- [X] T004 [US1] Add test for single-page query (50 items) verifying no additional page requests via call count in `internal/azureclient/client_test.go`

**Checkpoint**: Multi-page aggregation verified. No implementation changes needed — existing pagination loop handles this correctly.

---

## Phase 4: User Story 2 — Automatic Page Following (Priority: P1)

**Goal**: Verify that the client automatically follows NextPageLink and handles edge cases correctly.

**Independent Test**: Verify that when a page has a NextPageLink, the client fetches the next page automatically, including edge cases like empty pages and mid-pagination cancellation.

### Tests for User Story 2

- [X] T005 [US2] Add test for empty items list with NextPageLink — client follows link and returns items from subsequent page in `internal/azureclient/client_test.go`
- [X] T006 [US2] Add test for context cancellation mid-pagination (cancel after first page succeeds) in `internal/azureclient/client_test.go`

**Checkpoint**: Page following and cancellation edge cases verified. Existing `TestGetPrices_MidPaginationErrorIncludesPage` already covers mid-pagination HTTP errors.

---

## Phase 5: User Story 3 — Pagination Safety Limit (Priority: P2)

**Goal**: Enforce a 10-page maximum and return `ErrPaginationLimitExceeded` when exceeded.

**Independent Test**: Simulate a response chain exceeding 10 pages and verify the client stops with the correct error. Also verify exactly 10 pages succeeds.

### Tests for User Story 3

- [X] T007 [P] [US3] Add test for query exceeding 10-page limit returning `ErrPaginationLimitExceeded` in `internal/azureclient/client_test.go`
- [X] T008 [P] [US3] Add test for query returning exactly 10 pages succeeding without error in `internal/azureclient/client_test.go`

**Checkpoint**: Safety limit enforcement verified at boundary (10 pages) and beyond (11+ pages).

---

## Phase 6: User Story 4 — Pagination Observability (Priority: P3)

**Goal**: Add structured debug-level logging for pagination progress with page number and cumulative item count.

**Independent Test**: Execute a multi-page query and verify structured log entries include `page`, `items_this_page`, and `total_items` fields.

### Implementation for User Story 4

- [X] T009 [US4] Add pagination progress logging (debug level, message `"pagination progress"`) to `GetPrices()` loop in `internal/azureclient/client.go` with fields: `page` (int), `items_this_page` (int), `total_items` (int) — emit for each page after the first

### Tests for User Story 4

- [X] T010 [US4] Add test for multi-page query emitting structured log entries with `page`, `items_this_page`, `total_items` fields in `internal/azureclient/client_test.go`
- [X] T011 [US4] Add test for single-page query emitting no pagination progress logs in `internal/azureclient/client_test.go`

**Checkpoint**: Pagination progress is observable via structured debug logs. Single-page queries produce no noise.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Validation, quality gates, and documentation updates.

### Constitution Compliance Tasks

- [X] T012 [P] Run `make lint` and fix all linting issues
- [X] T013 [P] Run `make test` with `-race` flag to verify thread safety (`go test -race ./internal/azureclient/...`)
- [X] T014 [P] Verify test coverage >=80% for pagination logic (`go test -cover ./internal/azureclient/...`)
- [X] T015 [P] Verify godoc comments on all exported symbols meet >=80% docstring coverage
- [X] T016 [P] Update CLAUDE.md with pagination handler documentation if patterns changed

### Additional Validation

- [X] T017 Run quickstart.md validation — verify code examples compile and match implementation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — start immediately. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Phase 2 completion.
- **US2 (Phase 4)**: Depends on Phase 2 completion. Independent of US1.
- **US3 (Phase 5)**: Depends on Phase 2 completion. Independent of US1/US2.
- **US4 (Phase 6)**: Depends on Phase 2 completion. Independent of US1/US2/US3.
- **Polish (Phase 7)**: Depends on all user story phases (3–6) being complete.

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 — no dependencies on other stories
- **US2 (P1)**: Can start after Phase 2 — independent of US1 (tests different behavior)
- **US3 (P2)**: Can start after Phase 2 — independent of US1/US2
- **US4 (P3)**: Can start after Phase 2 — has implementation (T009) before tests (T010, T011)

### Within Each User Story

- Tests written FIRST, verified to FAIL before implementation (where applicable)
- US1/US2/US3: Test-only phases (implementation is existing code or Phase 2 constant change)
- US4: Implementation (T009) MUST precede test tasks (T010, T011)

### Parallel Opportunities

- T001 and T002 can run in parallel (different files: `errors.go` vs `client.go`)
- T007 and T008 are independent test cases (marked [P])
- T012–T016 can all run in parallel (independent validation tasks)
- All user story phases (3–6) could run in parallel if staffed independently

---

## Parallel Example: Foundational Phase

```text
# These edits are in different files (marked [P]):
Task T001: "Update MaxPaginationPages in errors.go"
Task T002: "Update GetPrices docstring in client.go"
```

## Parallel Example: User Story 3

```text
# These tests are independent (marked [P]):
Task T007: "Test exceeding 10-page limit returns ErrPaginationLimitExceeded"
Task T008: "Test exactly 10 pages succeeds without error"
```

## Parallel Example: Polish Phase

```text
# These validation tasks are independent (marked [P]):
Task T012: "Run make lint"
Task T013: "Run make test -race"
Task T014: "Verify test coverage"
Task T015: "Verify docstring coverage"
Task T016: "Update CLAUDE.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (change constant, update docstring)
2. Complete Phase 3: US1 (verify multi-page aggregation)
3. **STOP and VALIDATE**: Run `make test` — all tests pass

### Incremental Delivery

1. Phase 2 → Foundation ready (constant + docstring)
2. Phase 3: US1 → Multi-page aggregation verified (MVP!)
3. Phase 4: US2 → Edge cases verified
4. Phase 5: US3 → Safety limit enforced at boundary
5. Phase 6: US4 → Observability added with structured logging
6. Phase 7 → Quality gates passed, docs updated

### Single Developer (Recommended)

All changes are in 3 files (`errors.go`, `client.go`, `client_test.go`).
Execute phases sequentially: 2 → 3 → 4 → 5 → 6 → 7.

---

## Notes

- All changes modify existing files — no new files created
- Source files: `internal/azureclient/errors.go`, `internal/azureclient/client.go`, `internal/azureclient/client_test.go`
- Tests use `httptest.NewServer` with controlled handlers for deterministic behavior
- The pagination loop already works correctly — this feature reduces the limit and adds observability
- Existing test `TestGetPrices_MidPaginationErrorIncludesPage` already covers mid-pagination HTTP failure edge case
- Existing test `TestClient_GetPrices_Pagination` covers basic 2-page following
- [P] tasks = different files or independent test cases, no dependencies
- [Story] label maps task to specific user story for traceability
