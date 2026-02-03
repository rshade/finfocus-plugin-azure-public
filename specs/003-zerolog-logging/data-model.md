# Data Model: Zerolog Structured Logging

**Feature**: 003-zerolog-logging
**Date**: 2026-02-02

## Entities

### Log Entry

A single JSON log record emitted to stderr.

**Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `level` | string | Yes | Log severity: trace, debug, info, warn, error, fatal, panic |
| `message` | string | Yes | Human-readable log message |
| `time` | string | Yes | ISO 8601 timestamp (RFC3339 format) |
| `plugin` | string | Yes | Plugin identifier: `azure-public` |
| `version` | string | Yes | Plugin version (e.g., `1.0.0`, `dev`) |
| `trace_id` | string | Conditional | Request trace ID from gRPC metadata (omitted if not present) |
| `error` | string | No | Error message when logging errors |
| `*` | any | No | Additional context fields (e.g., `port`, `region`, `resource_type`) |

**Example Output**:

```json
{"level":"info","plugin":"azure-public","version":"1.0.0","time":"2026-02-02T10:30:00Z","message":"plugin started"}
{"level":"info","plugin":"azure-public","version":"1.0.0","trace_id":"abc123","time":"2026-02-02T10:30:01Z","message":"processing estimate cost request"}
{"level":"error","plugin":"azure-public","version":"1.0.0","trace_id":"abc123","time":"2026-02-02T10:30:02Z","error":"azure api timeout","message":"failed to fetch pricing"}
```

**Validation Rules**:

- `level` must be one of: trace, debug, info, warn, error, fatal, panic
- `time` must be valid RFC3339 timestamp
- `plugin` must always be `azure-public`
- `trace_id` must not be empty string or null (omit field entirely if no trace)

### Log Level

Enumeration controlling log visibility.

**Values** (ordered by severity, lowest to highest):

| Level | Numeric | Use Case |
|-------|---------|----------|
| trace | -1 | Fine-grained debugging (rarely used) |
| debug | 0 | Development debugging, internal state |
| info | 1 | Normal operation events (default) |
| warn | 2 | Recoverable issues, deprecation warnings |
| error | 3 | Errors that don't crash the plugin |
| fatal | 4 | Unrecoverable errors (causes exit) |
| panic | 5 | Programming errors (causes panic) |

**Behavior**: Setting level to X suppresses all messages with severity < X.

### Trace ID

Unique identifier for request correlation across services.

**Attributes**:

| Attribute | Value |
|-----------|-------|
| Source | gRPC incoming metadata |
| Key | `pluginsdk.TraceIDMetadataKey` (`x-finfocus-trace-id`) |
| Extraction | `pluginsdk.TraceIDFromContext(ctx)` (SDK function) |
| Format | Opaque string (no validation) |
| Lifecycle | Extracted per-request via SDK, attached to request-scoped logger |

**Propagation Flow**:

```text
FinFocus Core                    Plugin
     |                              |
     |-- gRPC request ------------->|
     |   metadata:                  |
     |   x-finfocus-trace-id: abc   |
     |                              |
     |                    pluginsdk.TraceIDFromContext(ctx)
     |                    logging.RequestLogger(ctx, base)
     |                    All logs include trace_id: "abc"
     |                              |
     |<-- gRPC response ------------|
```

## State Transitions

### Logger Lifecycle

```text
[Startup]
    |
    v
[Base Logger Created]  -- pluginsdk.NewPluginLogger()
    |                     Fields: plugin, version
    |
    v
[Logger Injected]      -- NewCalculator(logger)
    |                     Calculator stores logger
    |
    +-- [Per Request] --+
    |                   |
    |                   v
    |           [Request Logger]    -- logging.RequestLogger(ctx, base)
    |                   |              Uses: pluginsdk.TraceIDFromContext(ctx)
    |                   |              Fields: plugin, version, trace_id (if present)
    |                   |
    |                   v
    |           [Log Events]        -- log.Info().Msg(...)
    |                   |
    +-------------------+
    |
    v
[Shutdown]             -- logger.Info().Msg("shutdown complete")
```

## Relationships

```text
┌─────────────┐     creates      ┌─────────────┐
│   main()    │ ───────────────► │ Base Logger │
└─────────────┘                  └─────────────┘
                                       │
                                       │ injected into
                                       ▼
                                 ┌─────────────┐
                                 │ Calculator  │
                                 └─────────────┘
                                       │
                                       │ per-request via
                                       │ logging.RequestLogger()
                                       ▼
                                 ┌─────────────┐     SDK extracts  ┌──────────┐
                                 │Request      │ ◄──────────────── │ Trace ID │
                                 │Logger       │   pluginsdk.      └──────────┘
                                 └─────────────┘   TraceIDFromContext()
                                       │
                                       │ emits
                                       ▼
                                 ┌─────────────┐
                                 │ Log Entry   │ ──► stderr (JSON)
                                 └─────────────┘
```
