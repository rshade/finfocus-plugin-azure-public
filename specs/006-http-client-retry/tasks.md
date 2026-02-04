# Implementation Tasks: HTTP Client with Retry Logic

**Feature**: 006-http-client-retry
**Date**: 2026-02-03
**Status**: Complete

## Task Legend

- `[P]` = Parallel task (can run concurrently with other [P] tasks in same phase)
- `[US#]` = Implements User Story #
- `[FR-###]` = Implements Functional Requirement

## Phase 1: Setup

- [x] T001 [P] Add `github.com/hashicorp/go-retryablehttp` v0.7.7 dependency via `go get`
- [x] T002 [P] Create `internal/azureclient/` package directory structure

## Phase 2: Types & Contracts (TDD - Tests First)

- [x] T003 [P] [US5] Create `internal/azureclient/types.go` with Config, PriceQuery, PriceItem, priceResponse types
- [x] T004 [P] [US5] Create `internal/azureclient/errors.go` with sentinel errors (ErrTimeout, ErrRateLimited, etc.)

## Phase 3: Retry Logic (TDD - Tests First)

- [x] T005 [US1,US2,US3] [FR-001,FR-002,FR-003,FR-004,FR-010,FR-011] Write tests for retry policy in `internal/azureclient/retry_test.go`
- [x] T006 [US1,US2,US3] [FR-001,FR-002,FR-003,FR-004,FR-010,FR-011] Implement custom retry policy in `internal/azureclient/retry.go`
- [x] T007 [US2] [FR-005] Write tests for Retry-After header parsing
- [x] T008 [US2] [FR-005] Implement Retry-After header parsing in retry.go

## Phase 4: Logger Adapter (TDD - Tests First)

- [x] T009 [US4] [FR-008] Write tests for zerolog adapter in `internal/azureclient/logger_test.go`
- [x] T010 [US4] [FR-008] Implement zerolog adapter for retryablehttp.LeveledLogger in `internal/azureclient/logger.go`

## Phase 5: Client Implementation (TDD - Tests First)

- [x] T011 [US5] [FR-006,FR-007,FR-009,FR-012] Write tests for client in `internal/azureclient/client_test.go`
- [x] T012 [US5] [FR-006,FR-007,FR-009,FR-012] Implement NewClient constructor in `internal/azureclient/client.go`
- [x] T013 [US1,US5] Write tests for GetPrices with mock HTTP server
- [x] T014 [US1,US5] Implement GetPrices method with pagination handling
- [x] T015 [US5] Write tests for OData filter query building
- [x] T016 [US5] Implement OData filter query builder

## Phase 6: Integration & Validation

- [x] T017 [P] Run `go test -race ./internal/azureclient/...` to verify thread safety
- [x] T018 [P] Run `make lint` to verify code quality
- [x] T019 Create integration test in `examples/azure_client_integration_test.go` for live API

## Phase 7: Documentation & Polish

- [x] T020 [P] Add godoc comments to all exported types and functions
- [x] T021 [P] Update CLAUDE.md with Azure client patterns
- [x] T022 Verify test coverage exceeds 80% with `go test -cover`

## Execution Notes

- Phases must be completed in order (1→2→3→4→5→6→7)
- Within phases, sequential tasks must complete before the next
- [P] tasks within the same phase can run in parallel
- TDD phases: Write test → Run test (expect fail) → Implement → Run test (expect pass)

## Results

- **Test Coverage**: 91.3% (exceeds 80% requirement)
- **Lint**: 0 issues
- **Race Detector**: Pass
- **All Tests**: Pass
