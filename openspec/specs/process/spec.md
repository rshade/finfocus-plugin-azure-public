# process Specification

## Purpose

Run the plugin as a child process of FinFocus core: announce the listen port on stdout, write
structured JSON logs to stderr, honour the port and log level environment variables, and shut
down on SIGTERM.

## Requirements

### Requirement: Port announcement on stdout only

On startup the process SHALL write exactly one non-empty stdout line, `PORT=<n>` with n in 1 to
65535, and nothing else before it. Repeated start and SIGTERM cycles SHALL each announce a port.

Tests: `TestPortOutputFormat`, `TestStdoutContainsOnlyPortLine`, `TestRapidStartupShutdown`

#### Scenario: Port line

- **WHEN** the plugin binary starts with default settings
- **THEN** the first stdout line matches `^PORT=\d+$`
- **AND** no other non-empty line precedes it

### Requirement: Listen port configuration

The process SHALL listen on `FINFOCUS_PLUGIN_PORT` when it is set and an ephemeral port when it
is unset. A non-numeric value SHALL exit non-zero, write `FINFOCUS_PLUGIN_PORT must be numeric`
to stderr, and print no `PORT=` line. A port already in use SHALL exit non-zero with a message
on stderr.

Tests: `TestConfiguredPortUsed`, `TestEphemeralPortWhenNotConfigured`, `TestInvalidPortNonNumeric`,
`TestPortAlreadyInUse`

#### Scenario: Configured port

- **WHEN** `FINFOCUS_PLUGIN_PORT` is set to a free port P
- **THEN** stdout is `PORT=` followed by P

#### Scenario: Non-numeric port

- **WHEN** `FINFOCUS_PLUGIN_PORT=invalid`
- **THEN** the process exits non-zero with `FINFOCUS_PLUGIN_PORT must be numeric` on stderr

### Requirement: Shutdown on SIGTERM

After announcing its port, the process SHALL exit within 5 seconds of receiving SIGTERM.

Tests: `TestGracefulShutdownOnSIGTERM`, `TestExitCodeZeroOnGracefulShutdown`

#### Scenario: SIGTERM

- **WHEN** SIGTERM is sent after the `PORT=` line
- **THEN** the process exits within 5 seconds

### Requirement: JSON logs on stderr

The process SHALL write logs to stderr, one JSON object per line, each with `level`, `message`,
an RFC 3339 `time`, `plugin_name` equal to `azure-public`, and a non-empty `plugin_version`.

Tests: `TestLogsAppearOnStderr`, `TestLogsAreValidJSON`, `TestLogsContainPluginAndVersionFields`,
`TestLogsContainTimeField`

#### Scenario: Startup log line

- **WHEN** the plugin starts and runs briefly
- **THEN** stderr is not empty and every line parses as JSON with `plugin_name=azure-public`

### Requirement: Log level selection

The log level SHALL default to `info` (info logged, debug suppressed). `FINFOCUS_LOG_LEVEL=debug`
SHALL emit debug entries, `FINFOCUS_LOG_LEVEL=error` SHALL suppress info, debug, and trace, and
`FINFOCUS_LOG_LEVEL` SHALL take precedence over `LOG_LEVEL`. An invalid value SHALL fall back to
info.

Tests: `TestLogLevelDebugShowsDebugMessages`, `TestLogLevelErrorSuppressesInfoMessages`,
`TestLogLevelDefaultIsInfo`, `TestLogLevelFinfocusTakesPrecedenceOverLogLevel`,
`TestLogLevelInvalidFallsBackToInfo`

#### Scenario: Precedence

- **WHEN** `FINFOCUS_LOG_LEVEL=debug` and `LOG_LEVEL=error`
- **THEN** debug entries appear on stderr

### Requirement: Trace id on request logs

`RequestLogger` SHALL add `trace_id` from the request context to the base logger, keeping the
base fields such as `plugin` and `version`. An absent or empty trace id SHALL add no field, and a
trace id longer than 128 characters SHALL be truncated to 128.

Tests: `TestRequestLogger_WithTraceID`, `TestRequestLogger_WithoutTraceID`,
`TestRequestLogger_PreservesExistingFields`, `TestRequestLogger_EmptyTraceIDNotAdded`,
`TestRequestLogger_TruncatesLongTraceID`

#### Scenario: Long trace id

- **WHEN** the context trace id is 200 characters
- **THEN** the logged `trace_id` is 128 characters
