# Research: Zerolog Structured Logging

**Feature**: 003-zerolog-logging
**Date**: 2026-02-02

## Research Tasks

### 1. Zerolog Best Practices for gRPC Services

**Decision**: Use zerolog's context-based logging with `logger.With()` for request-scoped fields.

**Rationale**:

- Zerolog's `With()` method creates a child logger with additional fields without allocation
- Each RPC handler receives a logger with trace ID pre-populated
- Child loggers inherit all parent fields (plugin, version) automatically

**Alternatives Considered**:

- Global logger with manual field addition per log call: Rejected - verbose, error-prone
- Context value for logger: Considered but unnecessary given struct injection pattern

### 2. Trace ID Extraction from gRPC Metadata

**Decision**: Use existing `pluginsdk.TraceIDFromContext(ctx)` from finfocus-spec SDK - do not reimplement.

**Rationale**:

- SDK already provides complete implementation in `sdk/go/pluginsdk/logging.go`
- Consistent with other FinFocus plugins
- Reduces code duplication and maintenance burden
- SDK also provides `TracingUnaryServerInterceptor()` for automatic extraction

**SDK Functions Available** (from `finfocus-spec/sdk/go/pluginsdk/logging.go:24-25`):

```go
// Already implemented in SDK - DO NOT REIMPLEMENT
const TraceIDMetadataKey = "x-finfocus-trace-id"

func TraceIDFromContext(ctx context.Context) string      // Extract trace ID
func ContextWithTraceID(ctx, traceID) context.Context    // Store trace ID
func TracingUnaryServerInterceptor() grpc.UnaryServerInterceptor  // Auto-extract
```

**Plugin-Specific Helper** (wraps SDK):

```go
// internal/logging/request.go - thin wrapper for plugin use
package logging

import (
    "context"
    "github.com/rs/zerolog"
    "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// RequestLogger creates a request-scoped logger with trace ID from context.
func RequestLogger(ctx context.Context, base zerolog.Logger) zerolog.Logger {
    traceID := pluginsdk.TraceIDFromContext(ctx)
    if traceID == "" {
        return base
    }
    return base.With().Str("trace_id", traceID).Logger()
}
```

**Alternatives Considered**:

- Reimplement extraction: Rejected - SDK already has it, violates DRY
- Use SDK directly in handlers: Works but less readable/extensible
- OpenTelemetry trace context: Overkill for current requirements

### 3. Logger Injection Pattern

**Decision**: Inject logger into `Calculator` struct via constructor, create request-scoped loggers in RPC handlers.

**Rationale**:

- Constructor injection is idiomatic Go
- Allows testing with mock/buffer loggers
- Request-scoped loggers ensure trace ID isolation

**Implementation Pattern**:

```go
type Calculator struct {
    logger zerolog.Logger
    // ... other fields
}

func NewCalculator(logger zerolog.Logger) *Calculator {
    return &Calculator{logger: logger}
}

func (c *Calculator) EstimateCost(ctx context.Context, req *Request) (*Response, error) {
    // Create request-scoped logger with trace ID using helper
    log := logging.RequestLogger(ctx, c.logger)
    log.Info().Msg("processing estimate cost request")
    // ...
}
```

**Alternatives Considered**:

- Global logger: Rejected - hard to test, no request isolation
- Logger in context: Unnecessary given struct injection works well

### 4. Log Level Environment Variable Handling

**Decision**: Use existing `pluginsdk.GetLogLevel()` which handles `FINFOCUS_LOG_LEVEL` > `LOG_LEVEL` > default `info`.

**Rationale**:

- SDK already implements the env var priority logic
- Consistent with other FinFocus plugins (reference AWS plugin)
- No additional code needed in this plugin

**Verification**: Current `main.go` already uses this pattern correctly.

### 5. Stderr-Only Output Guarantee

**Decision**: Zerolog configured to write to `os.Stderr` by default via `pluginsdk.NewPluginLogger`.

**Rationale**:

- SDK function handles stderr configuration
- No risk of stdout pollution
- PORT announcement remains the only stdout output

**Verification**: `pluginsdk.NewPluginLogger` returns logger configured with `zerolog.New(os.Stderr)`.

## Resolved Clarifications

| Unknown | Resolution | Source |
|---------|------------|--------|
| Trace ID metadata key | `x-finfocus-trace-id` | User clarification (2026-02-02) |
| Trace ID extraction | Use `pluginsdk.TraceIDFromContext(ctx)` | SDK: logging.go:24-25 |
| Log level priority | `FINFOCUS_LOG_LEVEL` > `LOG_LEVEL` > `info` | pluginsdk implementation |
| Logger injection point | `Calculator` constructor | Existing codebase pattern |

## No Further Research Needed

All technical unknowns have been resolved. SDK provides core trace ID functionality - plugin adds thin wrapper for consistent usage pattern. Ready for Phase 1 design.
