# Implementation Plan: gRPC Server with Port Discovery

**Branch**: `002-grpc-server-port` | **Date**: 2026-01-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-grpc-server-port/spec.md`

## Summary

Implement the main.go entrypoint for the Azure public pricing plugin that starts a gRPC server using `pluginsdk.Serve` and announces the listening port to stdout. The implementation follows the established pattern from the AWS plugin, using the finfocus-spec SDK for server lifecycle management.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**:

- `github.com/rshade/finfocus-spec/sdk/go/pluginsdk` - Plugin SDK for server lifecycle
- `github.com/rs/zerolog` - Structured JSON logging
- `github.com/hashicorp/go-retryablehttp` - HTTP client (already in go.mod)

**Storage**: N/A (stateless plugin)
**Testing**: `go test` with race detector (`-race`)
**Target Platform**: Linux/macOS/Windows (Go cross-platform)
**Project Type**: Single project (Go CLI plugin)
**Performance Goals**: Startup <1s, shutdown <5s (per spec SC-001, SC-003)
**Constraints**: <100ms health check response (per constitution III)
**Scale/Scope**: Single-instance plugin, no horizontal scaling in scope

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes linting checks (`make lint`), error handling strategy (all errors logged to stderr, non-zero exit on failure), complexity managed (simple main.go pattern)
- [x] **Testing**: Plan includes TDD workflow (tests before implementation), ≥80% coverage target for main package logic, race detector for signal handling
- [x] **User Experience**: Plan addresses plugin lifecycle (port announcement via SDK, graceful shutdown on SIGTERM/SIGINT), observability (structured JSON logging to stderr), error handling (clear error messages to stderr)
- [x] **Documentation**: Plan includes godoc comments for exported functions (none in main package), README updates (not required per spec), CLAUDE.md (no workflow changes)
- [x] **Performance**: Plan includes performance targets (startup <1s, shutdown <5s), reliability guarantees (signal handling), resource constraints (ephemeral port, minimal memory)
- [x] **Architectural Constraints**: Plan DOES NOT violate "Hard No's":
  - ✅ No authenticated Azure APIs (main.go only initializes server)
  - ✅ No persistent storage (stateless operation)
  - ✅ No infrastructure mutation (read-only plugin)
  - ✅ No bulk data embedding (dynamic pricing fetch in separate feature)

## Project Structure

### Documentation (this feature)

```text
specs/002-grpc-server-port/
├── plan.md              # This file
├── research.md          # Phase 0: SDK patterns research
├── data-model.md        # Phase 1: N/A (no data model for transport layer)
├── quickstart.md        # Phase 1: How to run and verify the plugin
├── contracts/           # Phase 1: N/A (no API contracts for main.go)
└── tasks.md             # Phase 2: Implementation tasks
```

### Source Code (repository root)

```text
cmd/
└── finfocus-plugin-azure-public/
    ├── main.go          # Entry point (UPDATE: implement pluginsdk.Serve)
    └── main_test.go     # NEW: Unit tests for configuration logic

internal/
├── client/
│   └── client.go        # Azure API client (existing, no changes)
└── pricing/
    ├── calculator.go    # Pricing calculator (existing, no changes)
    ├── calculator_test.go
    └── data.go          # Data types (existing, no changes)
```

**Structure Decision**: Single project structure using existing `cmd/` and `internal/` layout. No new directories needed - only updating existing `main.go` and adding `main_test.go`.

## Complexity Tracking

> **No violations to justify** - Plan follows constitution guidelines:
>
> - Simple main.go pattern matches AWS plugin reference
> - No new dependencies beyond existing go.mod
> - No additional abstractions needed
