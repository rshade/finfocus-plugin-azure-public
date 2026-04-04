# CLAUDE.md - Project Context

## Commands
- **Build**: `make build`
- **Test**: `make test`
- **Lint**: `make lint`
- **Clean**: `make clean`
- **Setup**: `make ensure`
- **Help**: `make help`
- **Run Plugin**: `go run cmd/finfocus-plugin-azure-public/main.go`
- **Integration Tests**: `go test -v -tags=integration -timeout=5m ./examples/...`

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
- N/A — stateless, in-memory only (010-pagination-handler)
- In-memory only (stateless constraint) (012-memory-cache)
- Go 1.25.7 + `github.com/hashicorp/golang-lru/v2/expirable`, (015-cache-completion)
- N/A — in-memory only (stateless constraint) (015-cache-completion)
- Go 1.25.5 + finfocus-spec v0.5.4 (`finfocusv1.ResourceDescriptor`), internal `azureclient` (PriceQuery, FilterBuilder) (016-descriptor-filter-mapping)
- N/A — pure data transformation, no I/O (016-descriptor-filter-mapping)
- Go 1.25.5 + None (Go stdlib `math` only) (019-cost-utilities)
- N/A — pure stateless functions (019-cost-utilities)
- N/A — in-memory LRU+TTL cache only (stateless constraint) (020-vm-cost-estimation)
- Go 1.25.7 + finfocus-spec v0.5.7 (pluginsdk), zerolog v1.34.0, google.golang.org/grpc, golang-lru/v2 (cache) (021-disk-cost-estimation)
- N/A — stateless plugin (in-memory LRU+TTL cache only) (021-disk-cost-estimation)
- Go 1.25.7 (from `go.mod`) + `azureclient` (HTTP client with retry), (022-integration-tests)
- N/A — stateless, in-memory LRU+TTL cache only (022-integration-tests)

## Recent Changes
- 002-grpc-server-port: Added Go 1.25.5
- 006-http-client-retry: Added Azure Retail Prices API client with retry logic
- 016-descriptor-filter-mapping: Added ResourceDescriptor to PriceQuery mapper
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

## Resource Descriptor Mapper (`internal/pricing/mapper.go`)

Maps `finfocusv1.ResourceDescriptor` to `azureclient.PriceQuery`:

```go
query, err := pricing.MapDescriptorToQuery(desc)
// query.ArmRegionName, query.ArmSkuName, query.ServiceName, query.CurrencyCode
```

**Supported Resource Types**:

| Resource Type | Azure Service Name |
| --- | --- |
| `compute/VirtualMachine` | Virtual Machines |
| `storage/ManagedDisk` | Managed Disks |
| `storage/BlobStorage` | Storage |

**Behavior**:
- Case-insensitive provider and resource type matching
- Tag fallback: `Tags["region"]` and `Tags["sku"]` when primary fields empty
- Primary fields always take precedence over tags
- Default currency: USD
- Multi-field validation: reports all missing fields in single error

**Error Sentinels**:
- `ErrUnsupportedResourceType` -> gRPC `Unimplemented`
- `ErrMissingRequiredFields` -> gRPC `InvalidArgument`

**Integration**: `Calculator.Supports()` uses `MapDescriptorToQuery` to validate resources

## EstimateCost RPC (`internal/pricing/calculator.go`)

Estimate monthly VM pricing from Azure Retail Prices API with cache support:

```go
attrs, err := structpb.NewStruct(map[string]any{
    "location": "eastus",
    "vmSize":   "Standard_B1s",
})
if err != nil {
    return err
}

resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
    ResourceType: "azure:compute/virtualMachine:VirtualMachine",
    Attributes:   attrs,
})
if err != nil {
    return err // gRPC code includes InvalidArgument, NotFound, etc.
}

// Key response fields
resp.GetCurrency()        // e.g., "USD"
resp.GetCostMonthly()     // hourly price * 730
resp.GetPricingCategory() // FOCUS_PRICING_CATEGORY_STANDARD
```

Behavior notes:
- Empty `resource_type` is accepted for backward compatibility (routes to VM path)
- Unsupported non-empty `resource_type` returns `codes.Unimplemented`
- Missing `location/region` or `vmSize/sku` returns `codes.InvalidArgument`
- Cache hits are served from `CachedClient` with no outbound API request

### Managed Disk Cost Estimation

Estimate monthly Managed Disk pricing. Disk prices are monthly (not hourly
like VMs), so `retailPrice` is returned directly as `cost_monthly`.

```go
attrs, err := structpb.NewStruct(map[string]any{
    "location":  "eastus",
    "disk_type": "Premium_SSD_LRS",
    "size_gb":   128,
})
if err != nil {
    return err
}

resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
    ResourceType: "azure:storage/managedDisk:ManagedDisk",
    Attributes:   attrs,
})
// resp.GetCostMonthly() → e.g., 19.71 (P10 tier monthly price)
```

**Supported disk types**: `Standard_LRS`, `StandardSSD_LRS`, `Premium_SSD_LRS`,
`Standard_ZRS`, `StandardSSD_ZRS`, `Premium_ZRS`

**Attribute aliases**:
- region: `location`, `region`
- disk_type: `diskType`, `disk_type`, `sku`
- size_gb: `sizeGb`, `size_gb`, `diskSizeGb`
- currency: `currencyCode`, `currency` (default: USD)

**Size-to-tier mapping**: Ceiling match — `size_gb` maps to smallest tier >= that
size (e.g., 100 GB → P10/128 GiB tier). 14 tiers from 4 GiB to 32767 GiB.

**ZRS pricing**: ZRS disk types use meter names with " ZRS" suffix (e.g., "P10 ZRS").

## GetProjectedCost RPC (`internal/pricing/calculator.go`)

Returns projected monthly cost for Azure resources via `ResourceDescriptor`:

```go
req := &finfocusv1.GetProjectedCostRequest{
    Resource: &finfocusv1.ResourceDescriptor{
        Provider:     "azure",
        ResourceType: "compute/VirtualMachine",
        Region:       "eastus",
        Sku:          "Standard_B1s",
    },
}

resp, err := calc.GetProjectedCost(ctx, req)
// resp.GetUnitPrice()      → 0.0104 (hourly)
// resp.GetCurrency()       → "USD"
// resp.GetCostPerMonth()   → 7.592 (0.0104 × 730)
// resp.GetBillingDetail()  → "Azure Retail Prices API: Standard_B1s in ..."
// resp.GetPricingCategory() → FOCUS_PRICING_CATEGORY_STANDARD
// resp.GetExpiresAt()      → cache expiry timestamp
```

**Supported resource types**:

| Resource Type | Status | Pricing Model |
| --- | --- | --- |
| `compute/VirtualMachine` | Supported | Hourly × 730 hrs/mo |
| `storage/ManagedDisk` | Validates, returns Unimplemented | Monthly retail price |
| `storage/BlobStorage` | Validates, returns Unimplemented | Per-GB monthly price |

**Error codes**:

- `InvalidArgument`: nil descriptor, missing region/sku
- `Unimplemented`: unsupported provider/resource type, nil cachedClient, non-VM
- `NotFound`: no pricing data for region/SKU
- `ResourceExhausted`, `Unavailable`, `Internal`: Azure API errors

**Validation**: Uses `MapDescriptorToQuery()` with sentinel errors mapped via
`MapToGRPCStatus()`. Tag fallback: `Tags["region"]` and `Tags["sku"]` when
primary fields empty. Default currency: USD.

**Logging**: Structured zerolog at all decision points — Info for request
entry and success (with `cost_monthly`, `currency`, `unit_price`,
`result_status=success`), Warn for validation failures and nil cachedClient,
Error for API/cache failures.

## Environment Variables

<!-- markdownlint-disable MD013 -->

| Variable | Default | Description |
| --- | --- | --- |
| `FINFOCUS_PLUGIN_PORT` | 0 (ephemeral) | gRPC listen port |
| `FINFOCUS_LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `FINFOCUS_CACHE_TTL` | 24h | Cache TTL duration (e.g., "10s", "1h", "0s" to disable) |
| `SKIP_INTEGRATION` | (unset) | Set to "true" to skip integration tests |

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
