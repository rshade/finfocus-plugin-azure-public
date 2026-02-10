# Tasks: Comprehensive Error Handling for Azure API Failures

**Input**: Design documents from `/specs/008-azure-error-handling/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/
**Tests**: Required (constitution mandates TDD - tests before implementation)
**Organization**: Tasks grouped by user story for independent implementation

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Go module**: `internal/azureclient/`, `internal/pricing/` at repository root
- Tests co-located with source per Go convention (`*_test.go`)

---

## Phase 1: Setup

**Purpose**: No new project initialization needed. Existing Go module structure
is used. This phase is a no-op.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core error infrastructure that MUST be complete before ANY user
story can be implemented. Adds new sentinel errors and logger field.

- [x] T001 Add `ErrNotFound` sentinel error to `internal/azureclient/errors.go`
- [x] T002 Move `ErrPaginationLimitExceeded` from `internal/azureclient/client.go` to `internal/azureclient/errors.go` (consolidate all sentinels in one file)
- [x] T003 Add `logger zerolog.Logger` field to `Client` struct and store `config.Logger` in `NewClient()` in `internal/azureclient/client.go`
- [x] T004 Run `make test` to verify no regressions from foundational changes

**Checkpoint**: Foundation ready - ErrNotFound exists, logger accessible on
Client, all existing tests pass. User story implementation can begin.

---

## Phase 3: User Story 1 - Contextual Error Messages (Priority: P1) MVP

**Goal**: All errors from `GetPrices()` include query context (region, SKU,
service) so operators can identify which resource lookup failed.

**Independent Test**: Issue a pricing query against a mock server returning
HTTP 400. Verify the returned error string contains `region=`, `sku=`, and
the original error cause is extractable via `errors.Is()`.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T005 [US1] Write table-driven test `TestGetPrices_ErrorIncludesQueryContext` with cases for HTTP errors, network errors, and 404s, verifying error messages contain region, SKU, and service fields in `internal/azureclient/client_test.go`
- [x] T006 [US1] Write test `TestGetPrices_ErrorPreservesRootCause` verifying `errors.Is()` still works through context wrapping in `internal/azureclient/client_test.go`
- [x] T007 [US1] Write test `TestGetPrices_MidPaginationErrorIncludesPage` verifying mid-pagination errors include page number in `internal/azureclient/client_test.go`

### Implementation for User Story 1

- [x] T008 [US1] Add helper function `formatQueryContext(query PriceQuery) string` that formats query fields as `query [region=X sku=Y service=Z]` in `internal/azureclient/client.go`
- [x] T009 [US1] Wrap all errors returned from the pagination loop in `GetPrices()` with query context and page number using `fmt.Errorf` in `internal/azureclient/client.go`
- [x] T010 [US1] Wrap `ErrPaginationLimitExceeded` error with query context in `GetPrices()` in `internal/azureclient/client.go`
- [x] T011 [US1] Run tests T005-T007 and verify they pass

**Checkpoint**: All errors from `GetPrices()` include query context. Existing
tests updated to account for wrapped error messages. MVP is functional.

---

## Phase 4: User Story 2 - Error Classification for Retry Decisions (Priority: P2)

**Goal**: Callers can programmatically distinguish between error categories
(not found, rate limited, unavailable, internal) to make retry decisions.
Includes HTTP 404 handling and gRPC status code mapping.

**Independent Test**: Mock HTTP 404 response, verify `errors.Is(err,
ErrNotFound)` returns true. Call `MapToGRPCStatus()` with each sentinel error,
verify correct gRPC code returned.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T012 [P] [US2] Write test `TestFetchPage_HTTP404_ReturnsErrNotFound` verifying HTTP 404 returns `ErrNotFound` sentinel in `internal/azureclient/client_test.go`
- [x] T013 [P] [US2] Write table-driven test `TestMapToGRPCStatus` covering all sentinel-to-gRPC-code mappings (10 cases from contracts/error-contract.go) in `internal/pricing/errors_test.go`

### Implementation for User Story 2

- [x] T014 [US2] Add `http.StatusNotFound` case to the status code switch in `fetchPage()` returning `ErrNotFound` with status and body in `internal/azureclient/client.go`
- [x] T015 [US2] Create `internal/pricing/errors.go` with `MapToGRPCStatus(err error) *status.Status` function implementing the mapping from contracts/error-contract.go
- [x] T016 [US2] Run tests T012-T013 and verify they pass

**Checkpoint**: HTTP 404 produces `ErrNotFound`. All sentinel errors map to
correct gRPC codes. Callers can classify errors programmatically.

---

## Phase 5: User Story 4 - Empty Result Set Handling (Priority: P2)

**Goal**: Queries returning zero results produce a specific `ErrNotFound`
error with query context instead of returning an empty slice.

**Independent Test**: Mock a server returning `{"Items": [], "Count": 0}`.
Call `GetPrices()` and verify `errors.Is(err, ErrNotFound)` is true and the
error message contains the queried region and SKU.

### Tests for User Story 4

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T017 [US4] Write test `TestGetPrices_EmptyResults_ReturnsErrNotFound` verifying empty result set returns `ErrNotFound` with query context in `internal/azureclient/client_test.go`
- [x] T018 [US4] Write test `TestGetPrices_NonEmptyResults_ReturnsSuccess` verifying non-empty results still return successfully (regression guard) in `internal/azureclient/client_test.go`

### Implementation for User Story 4

- [x] T019 [US4] Add empty result detection after pagination loop in `GetPrices()`: if `len(allItems) == 0`, return `ErrNotFound` wrapped with query context and "no pricing data" message in `internal/azureclient/client.go`
- [x] T020 [US4] Update existing tests that return empty results to account for new `ErrNotFound` behavior: `TestClient_GetPrices_UserAgent` (L322) and `TestClient_GetPrices_Logging` (L352) in `internal/azureclient/client_test.go` must provide non-empty responses or assert `ErrNotFound`
- [x] T021 [US4] Run tests T017-T018 and all existing tests to verify no regressions

**Checkpoint**: Empty results are detected and reported as `ErrNotFound`.
Non-empty results still work correctly.

---

## Phase 6: User Story 3 - Structured Error Logging (Priority: P3)

**Goal**: All pricing query errors are logged with structured fields (region,
SKU, service, error category, HTTP status) at differentiated severity levels.
JSON parse errors include a truncated response body snippet.

**Independent Test**: Trigger HTTP 400, 500, and invalid JSON errors. Capture
zerolog output and verify: (a) correct severity level, (b) structured fields
present, (c) JSON errors include response snippet.

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T022 [US3] Write table-driven test `TestGetPrices_LogsErrorWithStructuredFields` with cases for each error type, verifying log entries contain region, sku, service, url, and error_category fields in `internal/azureclient/client_test.go`
- [x] T023 [US3] Write table-driven test `TestGetPrices_LogSeverityDifferentiation` with cases for 4xx (warn), 5xx (error), network (debug), verifying correct severity level in `internal/azureclient/client_test.go`
- [x] T024 [US3] Write test `TestFetchPage_InvalidJSON_IncludesResponseSnippet` verifying JSON parse errors include truncated response body (up to 256 bytes) in `internal/azureclient/client_test.go`
- [x] T025 [US3] Write test `TestFetchPage_LargeResponseBody_TruncatedAt256Bytes` verifying response snippets are capped at 256 bytes via `io.LimitReader` in `internal/azureclient/client_test.go`

### Implementation for User Story 3

- [x] T026 [US3] Add helper function `errorCategory(err error) string` that maps sentinel errors to category strings (e.g., "not_found", "rate_limited") in `internal/azureclient/client.go`
- [x] T027 [US3] Modify `fetchPage()` to read response body via `io.LimitReader` (256+1 bytes) for JSON parse error snippets and include snippet in `ErrInvalidResponse` wrapping in `internal/azureclient/client.go`
- [x] T028 [US3] Add structured error logging in `GetPrices()` before each error return: log with fields `region`, `sku`, `service`, `url`, `error_category`, `http_status` at differentiated severity levels (debug for network/empty, warn for 4xx, error for 5xx/parse) in `internal/azureclient/client.go`
- [x] T029 [US3] Run tests T022-T025 and verify they pass

**Checkpoint**: All error paths produce structured log entries with correct
severity. JSON errors include response snippets capped at 256 bytes.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Validation, documentation, and quality assurance across all stories

### Constitution Compliance Tasks

- [x] T030 [P] Run `make lint` and fix all linting issues
- [x] T031 [P] Run `go test -race ./internal/azureclient/... ./internal/pricing/...` to verify thread safety
- [x] T032 [P] Verify test coverage >=80% for `internal/azureclient/` and `internal/pricing/` via `go test -cover`
- [x] T033 [P] Add godoc comments to all new exported symbols (`ErrNotFound`, `MapToGRPCStatus`) and verify docstring coverage >=80%
- [x] T034 [P] Update CLAUDE.md with error handling patterns (sentinel errors, gRPC mapping usage)

### Final Validation

- [x] T035 Run `make build` to verify project compiles cleanly
- [x] T036 Run `make test` to verify all tests pass end-to-end
- [x] T037 Run `make lint` final pass (extended timeout)
- [x] T038 Verify no sensitive information in error messages or logs (FR-009 compliance spot check)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No-op for this feature
- **Foundational (Phase 2)**: No dependencies - starts immediately. BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 completion
- **US2 (Phase 4)**: Depends on Phase 2 completion. Can run in parallel with US1
- **US4 (Phase 5)**: Depends on Phase 2 completion. Must run after US1 (same file: GetPrices in client.go)
- **US3 (Phase 6)**: Depends on Phase 2 completion. Must run after US1 and US4 (same file: GetPrices in client.go)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on foundational only. Modifies `GetPrices()` in `client.go`
- **US2 (P2)**: Depends on foundational only. Modifies `fetchPage()` in `client.go` + creates new `pricing/errors.go`. Can run in parallel with US1 (different functions)
- **US4 (P2)**: Depends on US1 completion (both modify `GetPrices()`). Adds empty result check after US1's context wrapping
- **US3 (P3)**: Depends on US1 + US4 completion (all modify `GetPrices()`). Adds logging as final layer

### File Conflict Matrix

| File                         | US1       | US2       | US3                   | US4       |
| ---------------------------- | --------- | --------- | --------------------- | --------- |
| azureclient/errors.go        | -         | -         | -                     | -         |
| azureclient/client.go        | GetPrices | fetchPage | GetPrices + fetchPage | GetPrices |
| azureclient/client\_test.go  | write     | write     | write                 | write     |
| pricing/errors.go            | -         | create    | -                     | -         |
| pricing/errors\_test.go      | -         | create    | -                     | -         |

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation tasks within a story are sequential (same file)
- Verification task confirms all story tests pass

### Parallel Opportunities

- **US1 and US2 can run in parallel** (US1 modifies `GetPrices`, US2 modifies `fetchPage` + creates new files)
- **T012 and T013 can run in parallel** (different files: client_test.go vs errors_test.go)
- **T030, T031, T032, T033, T034 can all run in parallel** (different concerns)

---

## Parallel Example: US1 + US2

```text
# These two story phases can execute simultaneously:

# Developer A: US1 (GetPrices context wrapping)
T005: Test error includes query context     -> client_test.go
T006: Test error preserves root cause       -> client_test.go
T007: Test mid-pagination error             -> client_test.go
T008: formatQueryContext helper             -> client.go (new function)
T009: Wrap errors in GetPrices              -> client.go (GetPrices)
T010: Wrap pagination limit error           -> client.go (GetPrices)
T011: Verify US1 tests pass

# Developer B: US2 (Error classification + gRPC mapping)
T012: Test HTTP 404 -> ErrNotFound          -> client_test.go
T013: Test MapToGRPCStatus                  -> pricing/errors_test.go (NEW)
T014: Add 404 case in fetchPage             -> client.go (fetchPage)
T015: Create MapToGRPCStatus                -> pricing/errors.go (NEW)
T016: Verify US2 tests pass
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (T001-T004)
2. Complete Phase 3: User Story 1 (T005-T011)
3. **STOP and VALIDATE**: All errors include query context, `errors.Is()` works
4. This alone delivers significant debugging improvement

### Incremental Delivery

1. Phase 2: Foundational -> Error infrastructure ready
2. Phase 3: US1 -> Contextual errors (MVP!)
3. Phase 4: US2 -> Error classification + gRPC mapping
4. Phase 5: US4 -> Empty result detection
5. Phase 6: US3 -> Structured logging
6. Phase 7: Polish -> Lint, coverage, docs
7. Each phase adds value without breaking previous phases

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Constitution mandates TDD: all test tasks must run and FAIL before implementation
- `client.go` is the primary contention point - US1/US4/US3 modify `GetPrices()` sequentially
- US2 can safely run in parallel with US1 since it only modifies `fetchPage()` and creates new files
- Total: 38 tasks (4 foundational, 7 US1, 5 US2, 5 US4, 8 US3, 9 polish)
