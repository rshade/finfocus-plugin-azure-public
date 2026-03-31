# finfocus-plugin-azure-public

[![Test](https://github.com/rshade/finfocus-plugin-azure-public/actions/workflows/test.yml/badge.svg)](https://github.com/rshade/finfocus-plugin-azure-public/actions/workflows/test.yml)

A Live/Runtime gRPC plugin for FinFocus that estimates Azure infrastructure
costs by querying the Azure Retail Prices API.

## Purpose

This plugin enables FinFocus to provide accurate, on-demand pricing for Azure
resources without requiring Azure credentials. It operates by fetching public
pricing data from the Azure Retail Prices API and caching it for performance.

## Getting Started

### Prerequisites

- Go 1.25.5 or higher
- Internet connection (to fetch pricing data)

### Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/rshade/finfocus-plugin-azure-public.git
   cd finfocus-plugin-azure-public
   ```

2. Install tools and build the plugin:

   ```bash
   make ensure
   make build
   ```

### Usage

Run the binary directly. It starts a gRPC server and outputs the port to stdout:

```bash
./bin/finfocus-plugin-azure-public
# Output: PORT=12345
```

## Environment Variables

<!-- markdownlint-disable MD013 -->

| Variable | Default | Description |
| --- | --- | --- |
| `FINFOCUS_PLUGIN_PORT` | Ephemeral | Fixed port number for the gRPC server |
| `FINFOCUS_LOG_LEVEL` | info | Log level: trace, debug, info, warn, error |
| `FINFOCUS_CACHE_TTL` | 24h | Cache TTL (e.g., "10s", "1h", "0s" to disable) |

<!-- markdownlint-enable MD013 -->

### Examples

**Run with default settings (ephemeral port):**

```bash
./bin/finfocus-plugin-azure-public
# stdout: PORT=54321
# stderr: {"level":"info","plugin":"azure-public",...}
```

**Run with a specific port:**

```bash
FINFOCUS_PLUGIN_PORT=8080 ./bin/finfocus-plugin-azure-public
# stdout: PORT=8080
```

**Run with debug logging:**

```bash
FINFOCUS_LOG_LEVEL=debug ./bin/finfocus-plugin-azure-public
```

### Output Separation

- **stdout**: Contains only the `PORT=XXXXX` line for discovery
- **stderr**: Contains JSON-formatted structured logs

### Log Format

All logs are written to stderr in JSON format with the following fields:

```json
{
  "level": "info",
  "plugin_name": "azure-public",
  "plugin_version": "1.0.0",
  "time": "2026-02-02T10:00:00Z",
  "message": "plugin started",
  "trace_id": "abc-123"
}
```

| Field            | Description                                                    |
|------------------|----------------------------------------------------------------|
| `level`          | Log severity: trace, debug, info, warn, error, fatal           |
| `plugin_name`    | Always "azure-public"                                          |
| `plugin_version` | Plugin version (or "dev" for development builds)               |
| `time`           | RFC3339 timestamp                                              |
| `message`        | Log message                                                    |
| `trace_id`       | Request trace ID (only present when provided by FinFocus Core) |

### Parsing Logs

```bash
# Parse with jq
./bin/finfocus-plugin-azure-public 2>&1 | jq '.'

# Filter by level
./bin/finfocus-plugin-azure-public 2>&1 | jq 'select(.level == "error")'

# Filter by trace ID
./bin/finfocus-plugin-azure-public 2>&1 | jq 'select(.trace_id == "abc-123")'
```

### Graceful Shutdown

The plugin responds to SIGTERM and SIGINT signals for graceful shutdown:

```bash
./bin/finfocus-plugin-azure-public &
PID=$!
kill -SIGTERM $PID  # Graceful shutdown, exit code 0
```

## Available Commands

| Command        | Description                          |
|----------------|--------------------------------------|
| `make build`   | Compile binary with version info     |
| `make test`    | Run unit tests with race detection   |
| `make lint`    | Run code quality checks              |
| `make clean`   | Remove build artifacts               |
| `make ensure`  | Install development dependencies     |
| `make help`    | Show available targets               |

## Supported Azure Resource Types

| Resource Type | Azure Service Name | Example SKU |
|---|---|---|
| `compute/VirtualMachine` | Virtual Machines | `Standard_B1s` |
| `storage/ManagedDisk` | Managed Disks | `Premium_LRS` |
| `storage/BlobStorage` | Storage | `Standard_LRS` |

Resource type matching is case-insensitive. Additional resource types will be
added in future releases.

## Integration Tests

Integration tests query the live Azure Retail Prices API to validate the
full EstimateCost pipeline end-to-end:

```bash
go test -v -tags=integration -timeout=5m ./examples/...
```

Tests include rate-limiting delays (12s between API calls) and use ±25%
tolerance on reference prices to absorb Azure pricing changes. If tests
fail due to price drift, update the reference constants in
`examples/estimate_cost_integration_test.go` (run with `-v` to see actual
prices).

To skip integration tests (e.g., in offline environments):

```bash
SKIP_INTEGRATION=true go test -tags=integration ./examples/...
```

## Development

See [CLAUDE.md](CLAUDE.md) for development commands and guidelines.
