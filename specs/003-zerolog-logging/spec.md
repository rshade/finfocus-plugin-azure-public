# Feature Specification: Implement Zerolog Structured Logging

**Feature Branch**: `003-zerolog-logging`
**Created**: 2026-02-02
**Status**: Draft
**Input**: GitHub Issue #6 - Configure zerolog for structured JSON logging to stderr with log level control via environment variables

## Clarifications

### Session 2026-02-02

- Q: What is the gRPC metadata key for trace ID propagation? → A: `x-finfocus-trace-id`

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Log Aggregation for Operators (Priority: P1)

As an operator running FinFocus plugins, I want all plugin logs in JSON format so that my log aggregator (ELK, Splunk, Datadog) can parse and index them automatically.

**Why this priority**: JSON logging is the core deliverable - without it, the entire feature fails to meet its primary purpose.

**Independent Test**: Can be fully tested by running the plugin and capturing stderr output, then validating it parses as valid JSON with expected fields.

**Acceptance Scenarios**:

1. **Given** the plugin is running, **When** it emits a log message, **Then** the output is valid JSON with `level`, `message`, and `time` fields
2. **Given** the plugin is running, **When** multiple log messages are emitted, **Then** each message appears on a separate line (newline-delimited JSON)
3. **Given** the plugin is running, **When** it logs, **Then** all output goes to stderr (stdout remains clean for port discovery)

---

### User Story 2 - Debug Mode for Developers (Priority: P2)

As a developer debugging plugin issues, I want to set `FINFOCUS_LOG_LEVEL=debug` to see verbose output including internal state and detailed error context.

**Why this priority**: Debug capability is essential for troubleshooting but only needed during development/investigation, not normal operation.

**Independent Test**: Can be fully tested by setting the environment variable and verifying debug-level messages appear that would not appear at default (info) level.

**Acceptance Scenarios**:

1. **Given** `FINFOCUS_LOG_LEVEL=debug` is set, **When** the plugin starts, **Then** debug-level messages appear in stderr
2. **Given** `FINFOCUS_LOG_LEVEL=error` is set, **When** the plugin runs normally, **Then** info and debug messages are suppressed
3. **Given** no log level environment variable is set, **When** the plugin runs, **Then** the default level is `info`

---

### User Story 3 - Stdout/Stderr Separation for FinFocus Core (Priority: P1)

As FinFocus Core, I need plugin logs completely isolated from stdout so that port discovery (the only stdout output) works reliably.

**Why this priority**: This is a critical architectural constraint - mixing logs with port discovery breaks the plugin contract.

**Independent Test**: Can be fully tested by capturing stdout and stderr separately, verifying stdout contains only `PORT=XXXX` and stderr contains all log output.

**Acceptance Scenarios**:

1. **Given** the plugin starts successfully, **When** I capture stdout, **Then** it contains only `PORT=XXXX` format output
2. **Given** the plugin logs at any level, **When** I capture stderr, **Then** all log messages appear there
3. **Given** the plugin encounters an error, **When** it logs the error, **Then** the error appears on stderr (not stdout)

---

### User Story 4 - Structured Fields for Monitoring (Priority: P2)

As a monitoring system, I want consistent structured fields (plugin name, version) in every log entry so I can filter and search logs across multiple plugins.

**Why this priority**: Structured fields enable observability but the logging system works without them - they enhance rather than enable functionality.

**Independent Test**: Can be fully tested by parsing any log entry and verifying the `plugin` and `version` fields are present with correct values.

**Acceptance Scenarios**:

1. **Given** the plugin emits any log message, **When** I parse the JSON, **Then** the `plugin` field equals `azure-public`
2. **Given** the plugin emits any log message, **When** I parse the JSON, **Then** the `version` field contains the build version (e.g., `1.0.0`)
3. **Given** the plugin logs from different components, **When** I filter by `plugin` field, **Then** all azure-public logs are captured

---

### User Story 5 - Trace ID Propagation for Distributed Tracing (Priority: P1)

As an operator debugging distributed systems, I want trace IDs from incoming requests to appear in all related log entries so I can correlate logs across FinFocus Core and all plugins.

**Why this priority**: Trace ID propagation is essential for debugging in distributed systems - without it, correlating logs across service boundaries is nearly impossible.

**Independent Test**: Can be fully tested by sending an RPC request with a trace ID in the context/metadata and verifying all resulting log entries include that trace ID.

**Acceptance Scenarios**:

1. **Given** an RPC request arrives with a trace ID in metadata, **When** the plugin logs during request processing, **Then** all log entries include the `trace_id` field with the incoming value
2. **Given** an RPC request arrives without a trace ID, **When** the plugin logs during request processing, **Then** log entries omit the `trace_id` field (no empty or placeholder values)
3. **Given** multiple concurrent requests with different trace IDs, **When** I filter logs by `trace_id`, **Then** each trace ID shows only logs from its specific request

---

### Edge Cases

- What happens when `FINFOCUS_LOG_LEVEL` is set to an invalid value (e.g., "verbose", "3", "")?
  - System falls back to `info` level and logs a warning about the invalid value
- What happens when both `FINFOCUS_LOG_LEVEL` and `LOG_LEVEL` are set?
  - `FINFOCUS_LOG_LEVEL` takes precedence
- What happens when log messages contain special characters (newlines, quotes, unicode)?
  - JSON encoding handles escaping automatically via zerolog; message integrity is preserved (no explicit test required - zerolog library behavior)
- What happens when a trace ID is malformed or excessively long?
  - System accepts and logs the trace ID as-is (validation is caller's responsibility)
- What happens when trace ID is passed but logger context is lost mid-request?
  - Logger with trace ID context must be passed through all function calls in request path

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST initialize logger via `pluginsdk.NewPluginLogger("azure-public", version, level, nil)`
- **FR-002**: System MUST write all log output to stderr, never to stdout
- **FR-003**: System MUST format logs as JSON with one complete object per line (newline-delimited JSON)
- **FR-004**: System MUST include `level`, `message`, and `time` fields in every log entry
- **FR-005**: System MUST include `plugin: "azure-public"` field in every log entry
- **FR-006**: System MUST include `version` field with the current build version in every log entry
- **FR-007**: System MUST read log level from `FINFOCUS_LOG_LEVEL` environment variable using `pluginsdk.GetLogLevel()`
- **FR-008**: System MUST fall back to `LOG_LEVEL` environment variable if `FINFOCUS_LOG_LEVEL` is not set
- **FR-009**: System MUST default to `info` level if no log level environment variable is set
- **FR-010**: System MUST pass the initialized logger to the plugin constructor for use in RPC handlers
- **FR-011**: System MUST support standard log levels: trace, debug, info, warn, error, fatal, panic
- **FR-012**: System MUST extract trace ID from request context using `pluginsdk.TraceIDFromContext(ctx)` (SDK handles gRPC metadata key `x-finfocus-trace-id`)
- **FR-013**: System MUST include `trace_id` field in all log entries when a trace ID is present in the request context
- **FR-014**: System MUST propagate the logger with trace ID context through all function calls within a request
- **FR-015**: System MUST omit `trace_id` field from log entries when no trace ID is present (no empty or null values)

### Key Entities

- **Logger**: Zerolog logger instance configured with plugin metadata and stderr output
- **Log Entry**: JSON object containing level, message, time, plugin, version, trace_id (when present), and optional context fields
- **Log Level**: Enumeration controlling message visibility (trace < debug < info < warn < error < fatal < panic)
- **Trace ID**: Unique identifier passed via gRPC metadata that correlates all logs from a single request across service boundaries

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of log messages parse as valid JSON
- **SC-002**: 0% of log messages appear on stdout (all on stderr)
- **SC-003**: 100% of log entries contain required fields (level, message, time, plugin, version)
- **SC-004**: Setting `FINFOCUS_LOG_LEVEL=error` suppresses all info-level messages
- **SC-005**: Setting `FINFOCUS_LOG_LEVEL=debug` enables debug-level messages
- **SC-006**: Log aggregators can parse plugin logs without custom parsing rules
- **SC-007**: 100% of logs emitted during a traced request include the correct `trace_id` field
- **SC-008**: Filtering logs by `trace_id` returns all and only logs from that specific request

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80% for business logic)
- [x] Error handling strategy is defined (invalid log levels fall back to info with warning)
- [x] Code complexity is considered (logger initialization is straightforward single function)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (run binary and parse stderr JSON output)
- [x] Performance test criteria specified (N/A - logging overhead is negligible)

### User Experience

- [x] Error messages are user-friendly and actionable (invalid log level warning includes valid options)
- [x] Response time expectations defined (N/A - logging is synchronous and fast)
- [x] Observability requirements specified (this feature IS the observability implementation)

### Documentation

- [x] README.md updates identified (document FINFOCUS_LOG_LEVEL environment variable)
- [x] API documentation needs outlined (godoc for any exported logger helpers)
- [x] Examples/quickstart guide planned (example log output format in README)

### Performance & Reliability

- [x] Performance targets specified (logging adds <1ms p99 overhead per call)
- [x] Reliability requirements defined (logger initialization cannot fail - falls back to defaults)
- [x] Resource constraints considered (zerolog is zero-allocation in hot paths)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data

## Assumptions

- The `pluginsdk.NewPluginLogger` and `pluginsdk.GetLogLevel` functions exist and work as documented
- The plugin version is available at initialization time (injected at build time via ldflags)
- Zerolog's default JSON formatting meets the requirements (no custom formatting needed)
- The existing `main.go` structure allows logger injection into the plugin constructor
- FinFocus Core passes trace IDs via gRPC metadata using key `x-finfocus-trace-id`
- The pluginsdk provides utilities for extracting trace ID from gRPC context/metadata
