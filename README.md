# finfocus-plugin-azure-public

[![Test](https://github.com/rshade/finfocus-plugin-azure-public/actions/workflows/test.yml/badge.svg)](https://github.com/rshade/finfocus-plugin-azure-public/actions/workflows/test.yml)

A Live/Runtime gRPC plugin for FinFocus that estimates Azure infrastructure
costs by querying the Azure Retail Prices API.

## Purpose

This plugin enables FinFocus to provide accurate, on-demand pricing for Azure
resources without requiring Azure credentials. It operates by fetching public
pricing data from the Azure Retail Prices API and caching it for performance.

`GetProjectedCost` returns the monthly retail quote. `GetActualCost` scales
that quote by the requested hours over 730. The default window is 730 hours,
so the default result matches the monthly quote. Both calls read the public
Retail Prices API. Neither call reads billed spend, and neither call needs
credentials. Confidence is recorded in `Source`. A FOCUS record is attached
when the request sets `billing_account_id`, or, when that field is empty,
when `FINFOCUS_BILLING_ACCOUNT_ID` is set. Dry run ignores the request id.
An empty id leaves the record unset.

A virtual machine quote also returns `price_options` and `region_prices`.
`price_options` lists Consumption, Spot, Savings Plan, and Reservation
rates. `region_prices` lists the same SKU in other regions. Both lists are
advisory and are not added to the monthly cost.

The provider is the cloud, `azure`. Classic `azure` and `azure-native` are
Pulumi packages, read from the resource type prefix. The plugin also
advertises and accepts the provider `azure-native`, because hosts before
rshade/finfocus#1645 send the package prefix as the provider. Issue #84 drops
it from what the plugin advertises once that ships. A plan fixture with one
resource for each priced type is in `testdata/pulumi/azure-plan.json`.

## Getting Started

### Prerequisites

- Go 1.27.1 or higher
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
| `FINFOCUS_BILLING_ACCOUNT_ID` | empty | FOCUS billing account id used when the request id is empty. Empty leaves the record unset. |

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

| Field | Description |
| --- | --- |
| `level` | Severity: trace, debug, info, warn, error, fatal |
| `plugin_name` | Always "azure-public" |
| `plugin_version` | Plugin version, or "dev" for a dev build |
| `time` | RFC3339 timestamp |
| `message` | Log message |
| `trace_id` | Trace ID when FinFocus Core sends one |

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
| --- | --- | --- |
| `compute/VirtualMachine` | Virtual Machines | `Standard_B1s` |
| `storage/ManagedDisk` | Managed Disks | `Premium_LRS` |
| `storage/BlobStorage` | Storage | `Hot LRS` |
| `storage/StorageAccount` | Storage | `Hot LRS` |
| `web/AppServicePlan` | Azure App Service | `P1v3` |
| `web/FunctionApp` | Functions | `Y1` |
| `containerservice/KubernetesCluster` | Azure Kubernetes Service | `Standard` |
| `sql/Database` | SQL Database | `GP_Gen5_2` |
| `cosmosdb/Account` | Azure Cosmos DB | `400 RU` |
| `network/LoadBalancer` | Load Balancer | `Standard` |

Resource type matching is case-insensitive. Spot is a Virtual Machine with
tag `priority=Spot`, or the same `priority` attribute on `EstimateCost`.
When priority is empty, `pricing_model=spot` selects the same row and
`pricing_model=consumption` stays on demand. The response category is
Dynamic. `priority=Regular` is on-demand. Any other priority or
pricing_model value is rejected. A storage
account accepts `capacity_gb` as an alias of `size_gb`. The quote is
capacity only. Transaction meters are not included.
Standard Load Balancer bills the included rules meter. A regional page with
no rows is read again at price region `Global`. Omitted `rule_count` uses
that meter once. Gateway and cross-region meters are not quoted. NAT
Gateway, Cache for Redis, and database servers for PostgreSQL and MySQL are
not priced. Virtual machine scale sets use the VM meter times `instances`
or `sku.capacity`. `priority=Regular` is on-demand. A Windows VM with
`licenseType` `Windows_Server` or `Windows_Client` uses the base rate and
says so. Any other Windows VM uses the Windows meter.

## Integration Tests

Integration tests query the live Azure Retail Prices API to validate the
full EstimateCost pipeline end-to-end:

```bash
go test -v -tags=integration -timeout=5m ./examples/...
```

Refresh the four live golden snapshots (two virtual machines and two
managed disks) and review the diff before committing it:

```bash
go test -tags=integration -update-golden -count=1 -timeout 8m ./... -run TestUpdateGoldenFromLiveAPI
```

`TestGoldenLiveSnapshots` replays those snapshots. A monthly cost that
moves by more than 0.01 fails the test. The procedure is also in
`internal/pricing/testdata/golden/README.md`.

Each live test waits 12 seconds when it finishes, to stay under the Retail
Prices API rate limit. One test can make several API calls. The virtual
machine and managed disk tests in `examples/estimate_cost_integration_test.go`
read their reference price from the Retail Prices API at test time, with
selection rules written in the test, so Azure price changes do not fail
them. The reference request retries HTTP 429 and 503 up to four times and
honours `Retry-After`. Tests that still use recorded reference constants
allow ±25%: the App Service, Functions, and AKS control-plane checks in
`examples/projected_cost_integration_test.go`. The AKS node pool checks read
the virtual machine price live. If a constant-based check fails due to price
drift, update its constant. Run with `-v` to see the actual prices.

To skip integration tests (e.g., in offline environments):

```bash
SKIP_INTEGRATION=true go test -tags=integration ./examples/...
```

Live meter checks are opt-in and are not part of CI:

```bash
./scripts/live-check.sh
```

`TestCalculatorAccuracy` checks the plugin against Azure Pricing Calculator
values, within 5 percent. The values come from the calculator's own backend,
not from the Retail Prices API. To refresh them, see "Updating calculator
values" in `internal/pricing/testdata/oracle/README.md`.

## Development

See [CLAUDE.md](CLAUDE.md) for development commands and guidelines.
