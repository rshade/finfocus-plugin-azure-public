# Quickstart: Zerolog Structured Logging

**Feature**: 003-zerolog-logging
**Date**: 2026-02-02

## Overview

This feature implements structured JSON logging for the Azure public pricing plugin. All logs are written to stderr in JSON format, with support for trace ID propagation from FinFocus Core.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FINFOCUS_LOG_LEVEL` | `info` | Log level (trace, debug, info, warn, error, fatal, panic) |
| `LOG_LEVEL` | `info` | Fallback if `FINFOCUS_LOG_LEVEL` not set |

### Log Level Priority

1. `FINFOCUS_LOG_LEVEL` (highest priority)
2. `LOG_LEVEL` (fallback)
3. `info` (default)

## Usage Examples

### Example 1: Default Logging (Info Level)

```bash
# Run plugin with default settings
./finfocus-plugin-azure-public

# stderr output:
{"level":"info","plugin":"azure-public","version":"1.0.0","time":"2026-02-02T10:00:00Z","message":"plugin started"}
{"level":"info","plugin":"azure-public","version":"1.0.0","time":"2026-02-02T10:00:00Z","message":"server listening","port":50051}
```

### Example 2: Debug Logging

```bash
# Enable debug logging
FINFOCUS_LOG_LEVEL=debug ./finfocus-plugin-azure-public

# stderr output includes debug messages:
{"level":"debug","plugin":"azure-public","version":"1.0.0","time":"2026-02-02T10:00:00Z","message":"using ephemeral port"}
{"level":"info","plugin":"azure-public","version":"1.0.0","time":"2026-02-02T10:00:00Z","message":"plugin started"}
{"level":"debug","plugin":"azure-public","version":"1.0.0","time":"2026-02-02T10:00:00Z","message":"health check endpoint ready"}
```

### Example 3: Error-Only Logging (Production)

```bash
# Suppress info/debug, show only errors
FINFOCUS_LOG_LEVEL=error ./finfocus-plugin-azure-public

# stderr output (quiet unless errors occur):
# (no output during normal operation)

# On error:
{"level":"error","plugin":"azure-public","version":"1.0.0","time":"2026-02-02T10:00:05Z","error":"connection refused","message":"azure api request failed"}
```

### Example 4: Trace ID Propagation

When FinFocus Core sends requests with trace IDs, all related logs include the trace ID:

```bash
# FinFocus Core sends gRPC request with metadata:
# x-finfocus-trace-id: req-abc-123

# stderr output shows trace_id in all request logs:
{"level":"info","plugin":"azure-public","version":"1.0.0","trace_id":"req-abc-123","time":"2026-02-02T10:00:01Z","message":"processing estimate cost request"}
{"level":"debug","plugin":"azure-public","version":"1.0.0","trace_id":"req-abc-123","time":"2026-02-02T10:00:02Z","message":"fetching pricing data","resource_type":"Microsoft.Compute/virtualMachines"}
{"level":"info","plugin":"azure-public","version":"1.0.0","trace_id":"req-abc-123","time":"2026-02-02T10:00:03Z","message":"estimate cost request completed"}
```

### Example 5: Parsing Logs with jq

```bash
# Run plugin and parse JSON logs
./finfocus-plugin-azure-public 2>&1 | jq '.'

# Filter by log level
./finfocus-plugin-azure-public 2>&1 | jq 'select(.level == "error")'

# Filter by trace ID
./finfocus-plugin-azure-public 2>&1 | jq 'select(.trace_id == "req-abc-123")'

# Extract only messages
./finfocus-plugin-azure-public 2>&1 | jq -r '.message'
```

## Log Entry Schema

Every log entry contains:

```json
{
  "level": "info",           // Required: trace|debug|info|warn|error|fatal|panic
  "message": "...",          // Required: Human-readable message
  "time": "2026-...",        // Required: RFC3339 timestamp
  "plugin": "azure-public",  // Required: Plugin identifier
  "version": "1.0.0",        // Required: Plugin version
  "trace_id": "...",         // Conditional: Present only if trace ID in request
  "error": "...",            // Optional: Error details
  // ... additional context fields
}
```

## Stdout vs Stderr

| Stream | Content |
|--------|---------|
| stdout | `PORT=XXXXX` only (for FinFocus Core discovery) |
| stderr | All JSON log messages |

**Important**: Never mix logs with stdout output. FinFocus Core parses stdout to discover the plugin port.

## Testing Log Output

```bash
# Verify JSON format
./finfocus-plugin-azure-public 2>&1 | head -1 | jq . && echo "Valid JSON"

# Verify required fields
./finfocus-plugin-azure-public 2>&1 | head -1 | jq 'has("level", "message", "time", "plugin", "version")'
# Output: true

# Verify no stdout pollution (should only see PORT=)
./finfocus-plugin-azure-public 2>/dev/null
# Output: PORT=50051
```
