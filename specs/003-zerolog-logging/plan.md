# Implementation Plan: Implement Zerolog Structured Logging

**Branch**: `003-zerolog-logging` | **Date**: 2026-02-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-zerolog-logging/spec.md`

## Summary

Implement structured JSON logging using zerolog with trace ID propagation for distributed tracing. The plugin already has basic logger initialization in `main.go` using `pluginsdk.NewPluginLogger`. This feature extends the implementation to:

1. Pass the logger to the `Calculator` struct for use in RPC handlers
2. Create `internal/logging` package with plugin-specific helpers that wrap SDK functions
3. Use `pluginsdk.TraceIDFromContext(ctx)` for trace ID extraction (not re-implement)
4. Provide `RequestLogger(ctx, base)` helper for consistent request-scoped logging
5. Ensure all logs go to stderr with required fields (level, message, time, plugin, version, trace_id)

### SDK Functions Used (from finfocus-spec/sdk/go/pluginsdk/logging.go)

- `pluginsdk.TraceIDMetadataKey` - constant: `x-finfocus-trace-id`
- `pluginsdk.TraceIDFromContext(ctx)` - extract trace ID from context
- `pluginsdk.ContextWithTraceID(ctx, traceID)` - store trace ID in context
- `pluginsdk.TracingUnaryServerInterceptor()` - server interceptor (optional)

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: zerolog v1.34.0, finfocus-spec v0.5.4 (pluginsdk)
**Storage**: N/A - stateless plugin
**Testing**: go test with race detector, table-driven tests
**Target Platform**: Linux server (gRPC plugin for FinFocus Core)
**Project Type**: Single project (Go plugin)
**Performance Goals**: <1ms logging overhead, <10ms cache hits, <2s p95 API calls
**Constraints**: All logs to stderr only, stdout reserved for `PORT=XXXX`
**Scale/Scope**: Single plugin, ~6 Go files affected

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes linting checks (`make lint`), error handling for invalid log levels (fallback to info), low complexity (logger injection pattern)
- [x] **Testing**: Plan includes TDD workflow, ≥80% coverage for logger helper functions, tests for trace ID extraction
- [x] **User Experience**: Plan addresses observability (this IS the observability feature), structured JSON logging to stderr, no stdout pollution
- [x] **Documentation**: Plan includes godoc for exported logger helpers, README updates for FINFOCUS_LOG_LEVEL env var
- [x] **Performance**: Zerolog is zero-allocation in hot paths (<1ms overhead), no impact on existing performance targets
- [x] **Architectural Constraints**: Plan DOES NOT violate "Hard No's":
  - ✅ No authenticated Azure APIs (logging is internal)
  - ✅ No persistent storage (logs to stderr stream)
  - ✅ No infrastructure mutation (read-only logging)
  - ✅ No bulk data embedding (no pricing data in logs)

## Project Structure

### Documentation (this feature)

```text
specs/003-zerolog-logging/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0: Research findings
├── data-model.md        # Phase 1: Log entry schema
├── quickstart.md        # Phase 1: Usage examples
└── contracts/           # Phase 1: N/A (no external API changes)
```

### Source Code (repository root)

```text
cmd/finfocus-plugin-azure-public/
├── main.go              # MODIFY: Pass logger to Calculator
└── main_test.go         # MODIFY: Add logger tests

internal/
├── logging/             # NEW: Plugin-specific logger utilities (wraps SDK)
│   ├── request.go       # RequestLogger helper using pluginsdk.TraceIDFromContext
│   └── request_test.go  # Unit tests for request logger helper
└── pricing/
    ├── calculator.go    # MODIFY: Accept logger, use in RPC handlers
    └── calculator_test.go # MODIFY: Test logging behavior
```

**Structure Decision**: Single project structure maintained. New `internal/logging` package provides plugin-specific helpers that wrap SDK functions, enabling:

- Consistent request-scoped logger creation across all RPC handlers
- Future extensibility for plugin-specific context fields (region, resource_type)
- Cleaner separation of concerns from pricing logic
- Easier unit testing of logging behavior

## Complexity Tracking

No constitution violations requiring justification. Implementation follows standard Go patterns:

- Logger injection via constructor (idiomatic Go)
- SDK-based trace ID extraction via `pluginsdk.TraceIDFromContext` (reuse, don't reimplement)
- Thin wrapper pattern for plugin-specific helpers
- Zerolog's built-in JSON formatting (no custom logic)
