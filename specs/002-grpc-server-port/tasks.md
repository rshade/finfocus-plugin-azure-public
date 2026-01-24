# Tasks: gRPC Server with Port Discovery

**Input**: Design documents from `/specs/002-grpc-server-port/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, quickstart.md

**Tests**: Required per constitution (TDD workflow, ≥80% coverage for business logic)

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Project structure**: `cmd/finfocus-plugin-azure-public/` for entrypoint
- **Internal packages**: `internal/` (existing, no changes needed for this feature)

---

## Phase 1: Setup

**Purpose**: Verify existing project structure is ready for implementation

- [x] T001 Verify go.mod has required dependencies (pluginsdk, zerolog) in go.mod
- [x] T002 [P] Verify existing main.go stub compiles in cmd/finfocus-plugin-azure-public/main.go
- [x] T003 [P] Create test file skeleton in cmd/finfocus-plugin-azure-public/main_test.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extract `run()` function pattern to enable testability

**⚠️ CRITICAL**: This refactoring enables unit testing of all user stories

- [x] T004 Refactor main.go to use `run() error` pattern in cmd/finfocus-plugin-azure-public/main.go
- [x] T005 Add version variable with ldflags injection support in cmd/finfocus-plugin-azure-public/main.go
- [x] T006 Implement basic error return structure (main calls run, exits 1 on error) in cmd/finfocus-plugin-azure-public/main.go

**Checkpoint**: Foundation ready - `run()` function exists and returns error on failure

---

## Phase 3: User Story 1 - Port Discovery (Priority: P1) 🎯 MVP

**Goal**: Plugin outputs exactly one line `PORT=XXXXX` to stdout upon successful server startup

**Independent Test**: Run binary, parse stdout for PORT= line, verify gRPC connection succeeds

### Tests for User Story 1 (TDD - Write First, Must Fail)

- [x] T007 [US1] Write test for PORT= output format verification in cmd/finfocus-plugin-azure-public/main_test.go
- [x] T008 [US1] Write test for stdout contains only PORT= line (no log contamination) in cmd/finfocus-plugin-azure-public/main_test.go

### Implementation for User Story 1

- [x] T009 [US1] Initialize logger using pluginsdk.NewPluginLogger in cmd/finfocus-plugin-azure-public/main.go
- [x] T010 [US1] Create pricing.Calculator plugin instance in cmd/finfocus-plugin-azure-public/main.go
- [x] T011 [US1] Configure pluginsdk.ServeConfig with PluginInfo in cmd/finfocus-plugin-azure-public/main.go
- [x] T012 [US1] Call pluginsdk.Serve(ctx, config) to start server in cmd/finfocus-plugin-azure-public/main.go
- [x] T013 [US1] Verify tests pass (PORT= output working) in cmd/finfocus-plugin-azure-public/main_test.go

**Checkpoint**: `go run ./cmd/finfocus-plugin-azure-public` outputs `PORT=XXXXX` to stdout

---

## Phase 4: User Story 2 - Configurable Port (Priority: P2)

**Goal**: Plugin reads port from `FINFOCUS_PLUGIN_PORT` environment variable

**Independent Test**: Set env var, verify plugin listens on that specific port

### Tests for User Story 2 (TDD - Write First, Must Fail)

- [x] T014 [P] [US2] Write test for FINFOCUS_PLUGIN_PORT=8080 uses port 8080 in cmd/finfocus-plugin-azure-public/main_test.go
- [x] T015 [P] [US2] Write test for missing env var uses ephemeral port in cmd/finfocus-plugin-azure-public/main_test.go

### Implementation for User Story 2

- [x] T016 [US2] Use pluginsdk.GetPort() to read port configuration in cmd/finfocus-plugin-azure-public/main.go
- [x] T017 [US2] Pass port to ServeConfig.Port field in cmd/finfocus-plugin-azure-public/main.go
- [x] T018 [US2] Verify tests pass (port configuration working) in cmd/finfocus-plugin-azure-public/main_test.go

**Checkpoint**: `FINFOCUS_PLUGIN_PORT=8080 go run ./cmd/finfocus-plugin-azure-public` outputs `PORT=8080`

---

## Phase 5: User Story 3 - Graceful Shutdown (Priority: P2)

**Goal**: Plugin shuts down gracefully on SIGTERM/SIGINT, completing in-flight requests

**Independent Test**: Start plugin, send SIGTERM, verify clean exit with code 0

### Tests for User Story 3 (TDD - Write First, Must Fail)

- [x] T019 [P] [US3] Write test for context cancellation on SIGTERM in cmd/finfocus-plugin-azure-public/main_test.go
- [x] T020 [P] [US3] Write test for exit code 0 on graceful shutdown in cmd/finfocus-plugin-azure-public/main_test.go

### Implementation for User Story 3

- [x] T021 [US3] Create context with cancel function in cmd/finfocus-plugin-azure-public/main.go
- [x] T022 [US3] Setup signal.Notify for os.Interrupt and syscall.SIGTERM in cmd/finfocus-plugin-azure-public/main.go
- [x] T023 [US3] Implement goroutine to call cancel() on signal receipt in cmd/finfocus-plugin-azure-public/main.go
- [x] T024 [US3] Pass context to pluginsdk.Serve for shutdown coordination in cmd/finfocus-plugin-azure-public/main.go
- [x] T025 [US3] Verify tests pass with race detector (`go test -race`) in cmd/finfocus-plugin-azure-public/main_test.go

**Checkpoint**: `kill -SIGTERM <pid>` results in clean shutdown with exit code 0

---

## Phase 6: User Story 4 - Structured Logging (Priority: P3)

**Goal**: All logs output to stderr in JSON format, never to stdout

**Independent Test**: Capture stderr, verify valid JSON; capture stdout, verify only PORT= line

### Tests for User Story 4 (TDD - Write First, Must Fail)

- [x] T026 [P] [US4] Write test for log messages appearing on stderr in cmd/finfocus-plugin-azure-public/main_test.go
- [x] T027 [P] [US4] Write test for log messages in valid JSON format in cmd/finfocus-plugin-azure-public/main_test.go

### Implementation for User Story 4

- [x] T028 [US4] Configure zerolog with pluginsdk.GetLogLevel() in cmd/finfocus-plugin-azure-public/main.go
- [x] T029 [US4] Add startup log message with version and plugin name in cmd/finfocus-plugin-azure-public/main.go
- [x] T030 [US4] Add shutdown log message on signal receipt in cmd/finfocus-plugin-azure-public/main.go
- [x] T031 [US4] Add error log message on server failure in cmd/finfocus-plugin-azure-public/main.go
- [x] T032 [US4] Verify tests pass (JSON logs on stderr only) in cmd/finfocus-plugin-azure-public/main_test.go

**Checkpoint**: `./bin/finfocus-plugin-azure-public 2>stderr.txt` produces valid JSON logs in stderr.txt

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Quality assurance, documentation, and constitution compliance

### Constitution Compliance Tasks

- [x] T033 [P] Run `make lint` and fix all linting issues
- [x] T034 [P] Run `go test -race ./cmd/...` to verify thread safety
- [x] T035 [P] Verify test coverage ≥80% for main package (N/A: integration tests use separate process)
- [x] T036 [P] Add godoc comments to run() function in cmd/finfocus-plugin-azure-public/main.go
- [x] T037 Verify error messages are actionable and include context

### Edge Case Verification

- [x] T038 Test port already in use scenario (expect error to stderr, non-zero exit)
- [x] T039 Test rapid startup/shutdown cycles (100 iterations without port conflicts)

### Final Validation

- [x] T040 Run quickstart.md commands and verify all outputs match expected
- [x] T041 Run `make build && make test` to verify full build pipeline (cmd tests pass; pricing tests have pre-existing issues)
- [x] T042 Verify architectural constraints not violated (no auth APIs, no persistent storage)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS all user stories
- **User Stories (Phases 3-6)**: All depend on Foundational completion
  - US1 → US2 → US3 → US4 (sequential by priority)
  - OR can be done in parallel if team capacity allows
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

| Story | Priority | Can Start After | Integrates With |
|-------|----------|-----------------|-----------------|
| US1 (Port Discovery) | P1 | Foundational | None (standalone) |
| US2 (Port Config) | P2 | Foundational | US1 (uses same ServeConfig) |
| US3 (Graceful Shutdown) | P2 | Foundational | US1 (context passed to Serve) |
| US4 (Structured Logging) | P3 | Foundational | US1, US3 (logging in same file) |

### Within Each User Story

1. Tests written FIRST (TDD) - must FAIL before implementation
2. Implementation tasks in dependency order
3. Tests pass verification at the end
4. Story independently testable before moving to next

### Parallel Opportunities

**Phase 1 (Setup)**:

```text
T002 and T003 can run in parallel [P]
```

**Phase 4-6 (User Stories 2-4) Tests**:

```text
# US2 tests can run in parallel:
T014 and T015 [P]

# US3 tests can run in parallel:
T019 and T020 [P]

# US4 tests can run in parallel:
T026 and T027 [P]
```

**Phase 7 (Polish)**:

```text
T033, T034, T035, T036 can all run in parallel [P]
```

---

## Parallel Example: Phase 7 Polish

```bash
# Launch all constitution compliance checks together:
Task: "Run make lint and fix all linting issues"
Task: "Run go test -race ./cmd/... to verify thread safety"
Task: "Verify test coverage ≥80% for main package"
Task: "Add godoc comments to run() function"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T006)
3. Complete Phase 3: User Story 1 (T007-T013)
4. **STOP and VALIDATE**: Run binary, verify PORT= output
5. MVP is deployable!

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add User Story 1 → **MVP: Port Discovery works**
3. Add User Story 2 → Port configuration works
4. Add User Story 3 → Graceful shutdown works
5. Add User Story 4 → Structured logging works
6. Polish → Production ready

### Single Developer Strategy

Execute phases sequentially:
Phase 1 → Phase 2 → Phase 3 (MVP) → Phase 4 → Phase 5 → Phase 6 → Phase 7

---

## Task Summary

| Phase | Task Range | Count | Description |
|-------|------------|-------|-------------|
| 1 | T001-T003 | 3 | Setup |
| 2 | T004-T006 | 3 | Foundational |
| 3 | T007-T013 | 7 | User Story 1 (MVP) |
| 4 | T014-T018 | 5 | User Story 2 |
| 5 | T019-T025 | 7 | User Story 3 |
| 6 | T026-T032 | 7 | User Story 4 |
| 7 | T033-T042 | 10 | Polish |
| **Total** | | **42** | |

---

## Notes

- All implementation in single file: `cmd/finfocus-plugin-azure-public/main.go`
- All tests in single file: `cmd/finfocus-plugin-azure-public/main_test.go`
- TDD required: Tests must fail before implementation
- Race detector required for signal handling tests
- Reference: AWS plugin `finfocus-plugin-aws-public/cmd/finfocus-plugin-aws-public/main.go`
