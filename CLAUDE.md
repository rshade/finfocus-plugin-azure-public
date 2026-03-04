# CLAUDE.md - Project Context

## Commands
- **Build**: `make build`
- **Test**: `make test`
- **Lint**: `make lint`
- **Clean**: `make clean`
- **Setup**: `make ensure`
- **Help**: `make help`
- **Run Plugin**: `go run cmd/finfocus-plugin-azure-public/main.go`

## Development
- **Go Version**: 1.25.7
- **Dependencies**:
  - `finfocus-spec`: Plugin SDK
  - `golang-lru/v2`: In-memory LRU+TTL cache
  - `go-retryablehttp`: HTTP Client
  - `zerolog`: Logging
  - `grpc`: RPC Framework
- **Architecture**:
  - `cmd/finfocus-plugin-azure-public`: Entry point
  - `internal/pricing`: Core logic
  - **No Auth**: Do not use Azure SDK auth libraries
  - **No DB**: Stateless operation only

## Code Style
- Use `gofmt` and `goimports`
- Errors: specific, wrapped, no silent failures
- Logging: `zerolog` (structured JSON) to stderr
- Output: `PORT=XXXX` to stdout ONLY

## Workflows
- **New Feature**: Run `.specify/scripts/bash/create-new-feature.sh`
- **Update Plan**: Run `.specify/scripts/bash/setup-plan.sh`
- **Check Status**: Check `ROADMAP.md`

## Active Technologies
- **Language**: Go 1.25.5 (002-grpc-server-port)
- **Storage**: N/A - stateless plugin (002-grpc-server-port)
- Go 1.25.7 + zerolog v1.34.0, finfocus-spec v0.5.7 (pluginsdk) (003-zerolog-logging)
- Go 1.25.7 + finfocus-spec v0.5.7 (pluginsdk), zerolog v1.34.0, google.golang.org/grpc (004-costsource-stubs)
- Go 1.25.5 (from go.mod) + golangci-lint (linting), actions/checkout@v6, actions/setup-go@v6 (005-ci-pipeline)
- N/A (CI workflow - no persistent storage) (005-ci-pipeline)
- Go 1.25.5 + `github.com/hashicorp/go-retryablehttp` (HTTP client with retry), `github.com/rs/zerolog` (structured logging) (006-http-client-retry)
- N/A - stateless plugin (in-memory only) (006-http-client-retry)
- Go 1.25.5 + `encoding/json` (stdlib), `github.com/rs/zerolog` (logging) (007-azure-price-models)
- Go 1.25.5 + `github.com/hashicorp/go-retryablehttp` (HTTP retry), (008-azure-error-handling)
- Go 1.25.5 + None new — pure Go stdlib (`fmt`, `strings`, `sort`) (009-odata-filter-builder)
- N/A — pure data transformation (string builder), no I/O (009-odata-filter-builder)
- Go 1.25.5 + None (Go stdlib `math` only) (019-cost-utilities)
- N/A — pure stateless functions (019-cost-utilities)
- In-memory only (stateless constraint) (012-memory-cache)
- N/A — stateless, in-memory only (010-pagination-handler)

## Recent Changes
- 002-grpc-server-port: Added Go 1.25.5
- 006-http-client-retry: Added Azure Retail Prices API client with retry logic
- 019-cost-utilities: Added cost conversion utilities in `internal/estimation`

## Cost Estimation (`internal/estimation`)

Pure utility functions for converting between hourly, monthly, and yearly
pricing rates. No external dependencies (Go stdlib `math` only).

```go
import "github.com/rshade/finfocus-plugin-azure-public/internal/estimation"

// Constants
estimation.HoursPerMonth // 730 (365 * 24 / 12)
estimation.HoursPerYear  // 8760 (365 * 24)

// Conversions (all results rounded to 2 decimal places)
estimation.HourlyToMonthly(0.10)  // 73.00
estimation.HourlyToYearly(0.10)   // 876.00
estimation.MonthlyToHourly(5.00)  // 0.01
```

## Azure Client (`internal/azureclient`)

HTTP client for Azure Retail Prices API (`https://prices.azure.com/api/retail/prices`):

```go
// Create client with defaults
config := azureclient.DefaultConfig()
config.Logger = logger // zerolog.Logger
client, err := azureclient.NewClient(config)

// Query pricing
query := azureclient.PriceQuery{
    ArmRegionName: "eastus",
    ArmSkuName:    "Standard_B1s",
    CurrencyCode:  "USD",
}
prices, err := client.GetPrices(ctx, query)
```

**FilterBuilder (OData `$filter`)**:

```go
// Basic AND filter (default priceType=Consumption is always included)
filter := azureclient.NewFilterBuilder().
    Region("eastus").
    Service("Virtual Machines").
    SKU("Standard_B1s").
    Build()
// armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s'
//   and priceType eq 'Consumption' and serviceName eq 'Virtual Machines'

// OR grouping + generic fields + explicit type override
filter = azureclient.NewFilterBuilder().
    Or(
        azureclient.Region("eastus"),
        azureclient.Region("westus2"),
    ).
    Field("meterName", "B1s").
    Type("Reservation").
    Build()
// (armRegionName eq 'eastus' or armRegionName eq 'westus2')
//   and meterName eq 'B1s' and priceType eq 'Reservation'
```

**Retry Policy**:
- Retries on HTTP 429 (rate limit), 503 (service unavailable), network errors
- Does NOT retry on 4xx (except 429) or 5xx (except 503)
- Exponential backoff: 1s min, 30s max
- Respects `Retry-After` header
- Max 3 retries (4 total attempts)

**Error Handling**:

- All errors from `GetPrices()` include query context: `query [region=X sku=Y service=Z] page N: ...`
- Empty results return `ErrNotFound` with query context
- HTTP 404 returns `ErrNotFound` sentinel
- Use `errors.Is(err, azureclient.ErrNotFound)` etc. for programmatic classification
- Sentinel errors: `ErrNotFound`, `ErrRateLimited`, `ErrServiceUnavailable`, `ErrRequestFailed`, `ErrInvalidResponse`, `ErrPaginationLimitExceeded`
- gRPC mapping: `pricing.MapToGRPCStatus(err)` converts any azureclient error to `*status.Status`
- Structured logging: errors logged with `region`, `sku`, `service`, `url`, `error_category` fields at differentiated severity levels (debug/warn/error)

**Integration Tests**: `go test -tags=integration ./examples/...`

## Environment Variables

<!-- markdownlint-disable MD013 -->

| Variable | Default | Description |
| --- | --- | --- |
| `FINFOCUS_PLUGIN_PORT` | 0 (ephemeral) | gRPC listen port |
| `FINFOCUS_LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `FINFOCUS_CACHE_TTL` | 24h | Cache TTL duration (e.g., "10s", "1h", "0s" to disable) |

<!-- markdownlint-enable MD013 -->

## Cached Azure Client (`internal/azureclient/cache.go`)

`CachedClient` wraps `azureclient.Client` with a thread-safe in-memory cache:

```go
cacheConfig := azureclient.DefaultCacheConfig()
cacheConfig.MaxSize = 1000
cacheConfig.TTL = 24 * time.Hour
cacheConfig.ExpiresAtTTL = 4 * time.Hour
cacheConfig.Logger = logger

cachedClient, err := azureclient.NewCachedClient(client, cacheConfig)
if err != nil {
    return err
}
defer cachedClient.Close()

result, err := cachedClient.GetPrices(ctx, query)
// result.Items: Azure price rows
// result.ExpiresAt: caller-facing cache hint for projected/actual cost responses
```

Cache behavior:
- Key normalization: `CacheKey(query)` => `region|sku|product|service|currency` (lowercase, trimmed)
- L1 cache: in-process LRU+TTL (default 1000 entries, 24h TTL)
- L2 hint: `CachedResult.ExpiresAt` (default 4h) propagated to gRPC projected/actual cost responses
- TTL override: `FINFOCUS_CACHE_TTL` env var parsed in `main.go` (e.g., "10s", "1h", "0s" to disable)
- Eviction logging: debug-level structured logs with `cache_key` and `eviction_reason` ("lru" or "expired")
- Errors are never cached
- Stats: `cachedClient.Stats().Hits.Load()` / `cachedClient.Stats().Misses.Load()`

## Zerolog

 The constant already exists in finfocus-spec at sdk/go/pluginsdk/logging.go:24-25:

  // TraceIDMetadataKey is the gRPC metadata header for trace ID propagation.
  const TraceIDMetadataKey = "x-finfocus-trace-id"

  Along with:
  - TracingUnaryServerInterceptor() - server-side interceptor
  - TraceIDFromContext(ctx) - context extraction
  - ContextWithTraceID(ctx, traceID) - context storage
