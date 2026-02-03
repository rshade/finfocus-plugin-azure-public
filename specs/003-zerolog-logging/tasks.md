# Tasks: Implement Zerolog Structured Logging

**Input**: Design documents from `/specs/003-zerolog-logging/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: Required per constitution (TDD workflow - tests before implementation)

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)
- Include exact file paths in descriptions

## Path Conventions

Based on plan.md structure:

```text
cmd/finfocus-plugin-azure-public/
├── main.go              # Entry point with logger initialization
└── main_test.go         # Main tests

internal/
├── logging/             # NEW: Plugin-specific logger utilities (wraps SDK)
│   ├── request.go       # RequestLogger helper using pluginsdk.TraceIDFromContext
│   └── request_test.go  # Unit tests for request logger helper
└── pricing/
    ├── calculator.go    # RPC handler with logger
    └── calculator_test.go
```

## SDK Functions Used (DO NOT REIMPLEMENT)

From `finfocus-spec/sdk/go/pluginsdk/logging.go`:

- `pluginsdk.TraceIDMetadataKey` - constant: `x-finfocus-trace-id`
- `pluginsdk.TraceIDFromContext(ctx)` - extract trace ID from context
- `pluginsdk.ContextWithTraceID(ctx, traceID)` - store trace ID in context

---

## Phase 1: Setup

**Purpose**: Create new package structure for logging utilities

- [x] T001 Create internal/logging/ directory structure
- [x] T002 [P] Verify finfocus-spec dependency has logging utilities in go.mod

---

## Phase 2: Foundational (RequestLogger Helper)

**Purpose**: Plugin-specific helper that wraps SDK functions for consistent usage

**Note**: Uses `pluginsdk.TraceIDFromContext` - does NOT reimplement extraction

### Tests First (TDD)

- [x] T003 [P] Write unit tests for RequestLogger in internal/logging/request_test.go
  - Test: Returns logger with trace_id field when trace ID in context
  - Test: Returns base logger unchanged when no trace ID in context
  - Test: Preserves all existing logger fields (plugin, version)
  - Test: Does not add empty trace_id field when trace ID is empty string

### Implementation

- [x] T004 Implement RequestLogger function in internal/logging/request.go
- [x] T005 Add godoc comment explaining RequestLogger wraps pluginsdk.TraceIDFromContext
- [x] T006 Run tests to verify T003 passes: `go test ./internal/logging/...`

**Checkpoint**: RequestLogger helper ready for use in RPC handlers

---

## Phase 3: User Story 1+3+4 - JSON Logging with Structured Fields (Priority: P1)

**Goal**: All logs output as valid JSON to stderr with plugin, version fields

**Independent Test**: Run plugin, capture stderr, validate JSON with jq

**Note**: US1 (JSON format), US3 (stderr only), US4 (structured fields) are satisfied by existing pluginsdk.NewPluginLogger. This phase verifies and documents.

### Tests First (TDD)

- [x] T007 [P] [US1] Write test in cmd/finfocus-plugin-azure-public/main_test.go
  - Test: Verify log output is valid JSON
  - Test: Verify log contains level, message, time fields
  - Test: Verify log contains plugin_name="azure-public" field
  - Test: Verify log contains plugin_version field

- [x] T008 [P] [US3] Write test in cmd/finfocus-plugin-azure-public/main_test.go
  - Test: Verify all log output goes to stderr
  - Test: Verify stdout contains only PORT=XXXXX format

### Implementation

- [x] T009 [US1] Verify pluginsdk.NewPluginLogger outputs JSON format (existing code)
- [x] T010 [US3] Verify logger writes to stderr only (existing code)
- [x] T011 [US4] Verify logger includes plugin_name and plugin_version fields (existing code)
- [x] T012 Run tests to verify T007, T008 pass: `go test ./cmd/...`

**Checkpoint**: Basic JSON logging to stderr verified and tested

---

## Phase 4: User Story 2 - Debug Mode (Priority: P2)

**Goal**: Log level controllable via FINFOCUS_LOG_LEVEL environment variable

**Independent Test**: Set env var, verify debug logs appear/suppressed

**Note**: Already implemented in main.go using pluginsdk.GetLogLevel(). This phase adds tests.

### Tests First (TDD)

- [x] T013 [P] [US2] Write test in cmd/finfocus-plugin-azure-public/main_test.go
  - Test: FINFOCUS_LOG_LEVEL=debug shows debug messages
  - Test: FINFOCUS_LOG_LEVEL=error suppresses info messages
  - Test: Default level is info when no env var set
  - Test: FINFOCUS_LOG_LEVEL takes precedence over LOG_LEVEL
  - Test: Invalid FINFOCUS_LOG_LEVEL logs warning and falls back to info

### Implementation

- [x] T014 [US2] Verify existing log level parsing in main.go handles all cases
- [x] T015 [US2] Add warning log for invalid log level values (fallback to info)
- [x] T016 Run tests to verify T013 passes: `go test ./cmd/...`

**Checkpoint**: Log level control verified and tested

---

## Phase 5: User Story 5 - Trace ID Propagation (Priority: P1)

**Goal**: Trace IDs from gRPC metadata appear in all request logs

**Independent Test**: Send RPC with x-finfocus-trace-id, verify logs contain trace_id field

### Tests First (TDD)

- [x] T017 [P] [US5] Write test in internal/pricing/calculator_test.go
  - Test: Log includes trace_id when present in context (via pluginsdk.ContextWithTraceID)
  - Test: Log omits trace_id field when not in context (no empty string)
  - Test: Multiple concurrent requests maintain separate trace IDs

### Implementation

- [x] T018 [US5] Modify Calculator struct in internal/pricing/calculator.go to accept logger
- [x] T019 [US5] Update NewCalculator constructor to take zerolog.Logger parameter
- [x] T020 [US5] Modify main.go to pass logger to NewCalculator
- [x] T021 [US5] Use logging.RequestLogger in GetPluginInfo handler
- [x] T022 [US5] Use logging.RequestLogger in EstimateCost handler
- [x] T023 [US5] Add import for internal/logging package in calculator.go
- [x] T024 Run tests to verify T017 passes: `go test ./internal/pricing/...`

**Checkpoint**: Trace ID propagation complete - all P1 stories done

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, documentation, and quality checks

### Constitution Compliance Tasks

- [x] T025 [P] Run `make lint` and fix all linting issues
- [x] T026 [P] Run `go test -race ./...` to verify thread safety
- [x] T027 [P] Verify test coverage ≥80% for internal/logging: `go test -cover ./internal/logging/...`
- [x] T028 [P] Verify godoc comments on RequestLogger function
- [x] T029 [P] Update README.md with FINFOCUS_LOG_LEVEL documentation
- [x] T030 [P] Update README.md with log format examples (from quickstart.md)

### Final Validation

- [x] T031 Run full test suite: `make test`
- [x] T032 Run integration test: start plugin, parse stderr with jq
- [x] T033 Verify quickstart.md examples work as documented

---

## Dependencies & Execution Order

### Phase Dependencies

```text
Phase 1: Setup ──────────────────────────────► (no dependencies)
                    │
                    ▼
Phase 2: Foundational (RequestLogger) ───────► (depends on Phase 1)
                    │
          ┌────────┼────────┬────────┐
          ▼        ▼        ▼        ▼
Phase 3: US1+3+4  Phase 4: US2   Phase 5: US5  (all depend on Phase 2)
(JSON/stderr)     (Log Level)   (Trace ID)
          │        │             │
          └────────┴─────────────┘
                    │
                    ▼
Phase 6: Polish ─────────────────────────────► (depends on all stories)
```

### User Story Independence

- **US1+3+4 (JSON/stderr)**: Can start after Phase 2 - No dependencies on US2 or US5
- **US2 (Log Level)**: Can start after Phase 2 - No dependencies on other stories
- **US5 (Trace ID)**: Can start after Phase 2 - Uses RequestLogger helper

### Parallel Opportunities Within Phases

**Phase 2 (Foundational)**:

```bash
# Tests can be written in parallel:
T003 [P] Write unit tests for RequestLogger
```

**Phase 3-5 (User Stories)**:

```bash
# All user story test tasks marked [P] can run in parallel:
T007 [P] [US1] Write JSON format tests
T008 [P] [US3] Write stderr tests
T013 [P] [US2] Write log level tests
T017 [P] [US5] Write trace ID propagation tests
```

**Phase 6 (Polish)**:

```bash
# All polish tasks marked [P] can run in parallel:
T025-T030 can all run concurrently
```

---

## Implementation Strategy

### MVP First (P1 Stories Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T006)
3. Complete Phase 3: US1+3+4 JSON/stderr (T007-T012)
4. Complete Phase 5: US5 Trace ID (T017-T024)
5. **STOP and VALIDATE**: All P1 stories complete and tested
6. Run `make lint && make test` to verify

### Full Implementation

1. Continue with Phase 4: US2 Log Level (T013-T016)
2. Complete Phase 6: Polish (T025-T033)
3. Final validation with full test suite

### Parallel Team Strategy

With multiple developers:

- **Developer A**: Phase 2 (Foundational) → Phase 5 (US5 Trace ID)
- **Developer B**: Phase 3 (US1+3+4) → Phase 4 (US2) → Phase 6 (Polish)

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [Story] label maps task to specific user story for traceability
- TDD required: Write tests FIRST, verify they FAIL, then implement
- **SDK reuse**: Use `pluginsdk.TraceIDFromContext` - do NOT reimplement
- `internal/logging` provides thin wrapper for plugin-specific patterns
- Constitution requires ≥80% test coverage for business logic
- All logs MUST go to stderr - stdout is reserved for PORT=XXXXX only
