# Implementation Plan: Cost Calculation Utilities

**Branch**: `019-cost-utilities` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/019-cost-utilities/spec.md`

## Summary

Implement pure utility functions for converting between hourly, monthly,
and yearly cost rates in a new `internal/estimation` package. Functions
use industry-standard multipliers (730 hrs/month, 8760 hrs/year) and
round all results to two decimal places for currency precision. No
external dependencies — only Go stdlib `math` package.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: None (Go stdlib `math` only)
**Storage**: N/A — pure stateless functions
**Testing**: `go test` with table-driven tests, race detector
**Target Platform**: Linux server (gRPC plugin process)
**Project Type**: Single Go module
**Performance Goals**: Sub-microsecond — trivial arithmetic, no I/O
**Constraints**: Zero allocations, no side effects, deterministic
**Scale/Scope**: 3 exported functions + 2 exported constants + 1 helper

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Named constants (no magic numbers), golangci-lint
  compliance, godoc on all exports. Cyclomatic complexity ~1 per function.
- [x] **Testing**: TDD workflow — write tests first, ≥80% coverage
  target. No concurrency → race detector pass is trivial.
- [x] **User Experience**: N/A for internal library. No gRPC surface
  changes. No logging needed (pure functions).
- [x] **Documentation**: Godoc comments on package, all exported
  functions and constants. CLAUDE.md updated with new package.
  Docstring coverage 100% (all symbols exported and documented).
- [x] **Performance**: Sub-microsecond arithmetic. No I/O, no
  allocations, no retries. Deterministic output.
- [x] **Architectural Constraints**: No authenticated APIs, no storage,
  no infrastructure mutation, no bulk data embedding. Pure math only.

## Project Structure

### Documentation (this feature)

```text
specs/019-cost-utilities/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── estimation/          # NEW — cost conversion utilities
│   ├── cost.go          # Exported functions + constants
│   └── cost_test.go     # Table-driven unit tests
├── azureclient/         # Existing — Azure API client
├── pricing/             # Existing — gRPC service (future consumer)
├── client/              # Existing — plugin SDK wrapper
└── logging/             # Existing — request logging
```

**Structure Decision**: New `internal/estimation` package created
alongside existing packages. Chosen over placing in `internal/pricing`
because:

1. Separation of concerns — cost math is independent of gRPC layer
2. Reusable by future consumers (cache layer, batch RPC)
3. The `component/estimation` label on the issue signals a dedicated
   domain
4. Keeps `internal/pricing` focused on gRPC service implementation
