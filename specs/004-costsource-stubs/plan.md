# Implementation Plan: CostSourceService Method Stubs

**Branch**: `004-costsource-stubs` | **Date**: 2026-02-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-costsource-stubs/spec.md`

## Summary

Implement stub methods for all 11 CostSourceService RPC methods to satisfy the
interface contract. The existing `Calculator` type already embeds
`UnimplementedCostSourceServiceServer` and has partial implementations:

- `Name() string` exists but need `Name(ctx, req) (*NameResponse, error)` RPC method
- `GetPluginInfo()` exists but needs spec_version and providers fields
- `EstimateCost()` exists but returns empty response (needs Unimplemented status)

This feature adds the Name() RPC method, enhances GetPluginInfo(), updates
EstimateCost() to return Unimplemented, and adds 8 remaining stub methods.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: finfocus-spec v0.5.4 (pluginsdk), zerolog v1.34.0
**Storage**: N/A - stateless plugin
**Testing**: go test with table-driven tests, pluginsdk.NewTestPlugin helper
**Target Platform**: Linux server (gRPC plugin for FinFocus Core)
**Project Type**: Single Go module
**Performance Goals**: <10ms response time for all stub methods
**Constraints**: No authenticated Azure APIs, no persistent storage
**Scale/Scope**: 11 RPC methods, ~100 lines of new code, ~200 lines of tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes linting checks (`make lint`), error
  handling strategy (return Unimplemented status), no complexity violations
- [x] **Testing**: Plan includes TDD workflow (write tests first), 80% coverage
  target for new methods, race detector for concurrent code
- [x] **User Experience**: Plan addresses plugin lifecycle (existing - no
  changes), observability (Info-level logging per RPC), error handling (gRPC
  status codes)
- [x] **Documentation**: Plan includes godoc comments for new methods, no README
  updates needed for internal stubs
- [x] **Performance**: All stub methods are stateless, synchronous, <10ms
  response time
- [x] **Architectural Constraints**: Plan DOES NOT violate "Hard No's":
  - No authenticated Azure APIs (stubs return immediately)
  - No persistent storage (stateless)
  - No infrastructure mutation (read-only stubs)
  - No bulk data embedding (no data)

## Project Structure

### Documentation (this feature)

```text
specs/004-costsource-stubs/
├── plan.md              # This file
├── research.md          # Phase 0 output (minimal - no unknowns)
├── data-model.md        # Phase 1 output (N/A - no new entities)
├── quickstart.md        # Phase 1 output (N/A - internal feature)
├── contracts/           # Phase 1 output (N/A - existing gRPC contract)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
cmd/finfocus-plugin-azure-public/
└── main.go              # Entry point (no changes needed)

internal/
├── client/
│   └── client.go        # HTTP client (no changes)
├── logging/
│   ├── request.go       # Request logging helper (no changes)
│   └── request_test.go
└── pricing/
    ├── calculator.go    # ADD: 8 new stub methods
    ├── calculator_test.go # ADD: tests for new methods
    └── data.go          # No changes
```

**Structure Decision**: Existing single-project Go module structure. All new
code goes in `internal/pricing/calculator.go` where the `Calculator` type
already lives.

## Complexity Tracking

No complexity violations. All stubs are trivial implementations:

- Each method: 5-10 lines
- Total cyclomatic complexity: 1 per method
- No new dependencies required
