# Feature Specification: gRPC Server with Port Discovery

**Feature Branch**: `002-grpc-server-port`
**Created**: 2026-01-22
**Status**: Draft
**Input**: GitHub Issue #4 - Implement gRPC server with port discovery

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Port Discovery for Plugin Communication (Priority: P1)

As the FinFocus Core application, I need to discover which port the plugin is listening on by reading a single line from stdout, so I can establish communication with the plugin immediately after launching it.

**Why this priority**: This is the fundamental mechanism for plugin discovery. Without port announcement, the core application cannot communicate with the plugin, making all other functionality impossible.

**Independent Test**: Can be fully tested by running the binary and parsing stdout for the PORT= line. Delivers the core value of enabling plugin-to-core communication.

**Acceptance Scenarios**:

1. **Given** the plugin binary is executed, **When** it starts successfully, **Then** it outputs exactly one line to stdout in the format `PORT=XXXXX` where XXXXX is the listening port number
2. **Given** the plugin binary is executed, **When** it outputs the port, **Then** no other content appears on stdout (all logs go to stderr)
3. **Given** the plugin is running, **When** a client connects to the announced port, **Then** the gRPC connection is established successfully

---

### User Story 2 - Configurable Port via Environment Variable (Priority: P2)

As a deployer, I want to specify a fixed port via the `FINFOCUS_PLUGIN_PORT` environment variable, so I can configure the plugin for specific network environments or avoid port conflicts.

**Why this priority**: Port configuration enables deployment flexibility. While ephemeral ports work for most cases, production deployments may require predictable ports for firewall rules or service discovery.

**Independent Test**: Can be fully tested by setting the environment variable and verifying the plugin listens on that specific port.

**Acceptance Scenarios**:

1. **Given** `FINFOCUS_PLUGIN_PORT=8080` is set, **When** the plugin starts, **Then** it listens on port 8080 and outputs `PORT=8080`
2. **Given** no `FINFOCUS_PLUGIN_PORT` environment variable is set, **When** the plugin starts, **Then** it uses an OS-assigned ephemeral port and announces that port
3. **Given** `FINFOCUS_PLUGIN_PORT` is set to an invalid value (non-numeric), **When** the plugin starts, **Then** it fails with a clear error message to stderr

---

### User Story 3 - Graceful Shutdown on Signals (Priority: P2)

As a developer or orchestrator, I want the plugin to shut down gracefully when receiving SIGTERM or SIGINT, so that in-flight requests complete and resources are released cleanly.

**Why this priority**: Graceful shutdown prevents data corruption and enables clean restarts during deployments or debugging sessions. Critical for production reliability.

**Independent Test**: Can be fully tested by starting the plugin, sending SIGTERM, and verifying the process exits cleanly without errors.

**Acceptance Scenarios**:

1. **Given** the plugin is running, **When** SIGTERM is sent, **Then** the server stops accepting new connections and waits for existing requests to complete before exiting
2. **Given** the plugin is running, **When** SIGINT (Ctrl+C) is sent, **Then** the server performs the same graceful shutdown as SIGTERM
3. **Given** the plugin is performing a graceful shutdown, **When** shutdown completes, **Then** the process exits with code 0

---

### User Story 4 - Structured Logging to stderr (Priority: P3)

As a debugger or operator, I want all plugin logs in structured JSON format on stderr, so I can parse and analyze them without interfering with the port discovery mechanism on stdout.

**Why this priority**: Clean separation of output streams is essential for reliable port discovery. Structured logging enables log aggregation and analysis in production environments.

**Independent Test**: Can be fully tested by running the plugin and verifying all log output appears on stderr in valid JSON format.

**Acceptance Scenarios**:

1. **Given** the plugin starts, **When** it initializes the logger, **Then** all log messages are written to stderr in JSON format
2. **Given** the plugin encounters an error, **When** it logs the error, **Then** the error appears on stderr with structured fields (level, message, timestamp)
3. **Given** the plugin is running normally, **When** stdout is captured, **Then** only the `PORT=XXXXX` line is present (no log contamination)

---

### Edge Cases

- What happens when the specified port is already in use? The plugin should fail fast with a clear error message indicating the port conflict.
- What happens when the plugin cannot bind to any port? The plugin should exit with a non-zero exit code and log the error to stderr.
- What happens when context is cancelled before server starts? The plugin should exit cleanly without outputting a PORT= line.
- How does the system handle rapid startup/shutdown cycles? Each cycle should cleanly release the port, allowing immediate rebind.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Plugin MUST output exactly one line to stdout in the format `PORT=XXXXX` upon successful server startup
- **FR-002**: Plugin MUST direct all log output exclusively to stderr, never to stdout
- **FR-003**: Plugin MUST read port configuration from the `FINFOCUS_PLUGIN_PORT` environment variable when set
- **FR-004**: Plugin MUST use an OS-assigned ephemeral port (port 0) when no environment variable is configured
- **FR-005**: Plugin MUST register signal handlers for SIGTERM and SIGINT to initiate graceful shutdown
- **FR-006**: Plugin MUST complete in-flight requests before terminating during graceful shutdown
- **FR-007**: Plugin MUST initialize the logger before any other operations to ensure all startup messages are captured
- **FR-008**: Plugin MUST exit with code 0 on successful shutdown and non-zero on errors

### Assumptions

- The plugin SDK (`pluginsdk`) provides `NewPluginLogger`, `Serve`, `ServeConfig`, and `PluginInfo` types as referenced in the AWS plugin
- The plugin SDK's `Serve` function handles the actual PORT= output to stdout internally
- The Azure plugin implementation (`azurePlugin`) will be created in a separate feature
- Log level configuration follows the pattern established in the AWS plugin (environment variable or default)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Plugin outputs the PORT= line within 500ms of binary execution under normal conditions
- **SC-002**: Zero log messages appear on stdout during normal operation or shutdown
- **SC-003**: Plugin responds to shutdown signals (SIGTERM/SIGINT) within 5 seconds under normal load
- **SC-004**: 100% of startup attempts (≥20 consecutive) with valid configuration successfully announce their port
- **SC-005**: Plugin can be started and stopped 100 times consecutively without port conflicts or zombie processes
- **SC-006**: All log messages are valid JSON parseable by standard tools

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs to be identified (binary execution, signal handling)
- [x] Performance test criteria specified (startup time, shutdown responsiveness)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (PORT output <1s, shutdown <5s)
- [x] Observability requirements specified (structured JSON logging to stderr)

### Documentation

- [x] README.md updates identified (not required for internal infrastructure)
- [x] API documentation needs outlined (godoc comments for main package)
- [x] Examples/quickstart guide planned (running the plugin, verifying output)

### Performance & Reliability

- [x] Performance targets specified (startup <1s, shutdown <5s)
- [x] Reliability requirements defined (signal handling, clean shutdown)
- [x] Resource constraints considered (minimal memory footprint, single port)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data
