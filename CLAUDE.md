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
| `storage/StorageAccount` | Storage |
| `web/AppServicePlan` | Azure App Service |
| `web/FunctionApp` | Functions |
| `containerservice/KubernetesCluster` | Azure Kubernetes Service |
| `sql/Database` | SQL Database |
| `cosmosdb/Account` | Azure Cosmos DB |
| `network/LoadBalancer` | Load Balancer |

**Behavior**:
- Providers `azure` and `azure-native` are accepted. Resource type matching is case-insensitive and includes type tokens such as `azure:compute/linuxVirtualMachine:LinuxVirtualMachine`, `azure:compute/managedDisk:ManagedDisk`, `azure:storage/account:Account`, `azure:storage/blob:Blob`, `azure:appservice/servicePlan:ServicePlan`, `azure:mssql/database:Database`, `azure-native:compute:VirtualMachine`, `azure-native:compute:Disk`, `azure-native:storage:StorageAccount`, `azure-native:web:AppServicePlan`, `azure-native:web:WebApp` with `kind=FunctionApp`, `azure-native:containerservice:ManagedCluster`, `azure-native:sql:Database`, `azure-native:documentdb:DatabaseAccount`, and `azure-native:network:LoadBalancer`. `azure:compute/windowsVirtualMachine:WindowsVirtualMachine` is not quoted. `EstimateCost` sees `kind` on `azure-native:web:WebApp`. A managed disk pricing spec rate uses `per_month`
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
- Mapped types other than virtual machines and managed disks use the same quote as `GetProjectedCost`
- Missing `location/region` or `vmSize/sku` returns `codes.InvalidArgument`
- Cache hits are served from `CachedClient` with no outbound API request
- VM `EstimateCost` reads attribute `priority`. `Spot` uses the Linux Spot row and pricing category Dynamic. An empty priority stays the on-demand row and Standard. Any other value is InvalidArgument. When priority is empty, `pricing_model=spot` selects Spot and `pricing_model=consumption` stays on demand.
- `GetPluginInfo` returns `pluginsdk.SpecVersion` (`v0.7.0`). A value without the `v` prefix is rejected by the SDK

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

**Live meter**: service `Storage`, product `Premium SSD Managed Disks`,
`Standard SSD Managed Disks`, or `Standard HDD Managed Disks`. The price row
is skuName `{tier} {LRS|ZRS}` and meterName `{tier} {LRS|ZRS} Disk`
(for example, `P10 LRS` / `P10 LRS Disk` at 19.71 USD per month). Disk Mount
and Disk Operations are separate meters. `armSkuName` is not the filter.

### Storage Account Cost Estimation

`GetProjectedCost` prices `storage/StorageAccount` (including Pulumi
`azure:storage/storageAccount:StorageAccount`) from product
`General Block Blob v2`. The SKU is `{Tier} {Redundancy}` (`Hot LRS`), or
tags `tier` / `access_tier` plus `redundancy`. The meter is that SKU, a
space, then `Data Stored`, unit `1 GB/Month`. Monthly cost is
`retailPrice * size_gb` (`capacity_gb` is an alias)
across marginal `tierMinimumUnits` bands. Transaction meters are not included. Each GB uses the band it falls
in. The first band is the list rate. It is not multiplied by 730. The query
leaves `ArmSkuName` empty. A missing meter is `NotFound` and names the tier
and redundancy.

`storage/BlobStorage` uses the same marginal bands on meters whose name
contains `Data Stored`. A size that stays inside the first band is
`retailPrice * size_gb`.

### App Service Plan Cost Estimation

`GetProjectedCost` prices `web/AppServicePlan`, including Pulumi
`azure:appservice/plan:Plan`. The query is the region plus service
`Azure App Service`, and `ArmSkuName` stays empty. The short SKU is matched
locally with spaces removed, case-insensitive. Linux is the default, from a
`productName` that contains Linux. Tag `os=Windows` selects a product whose
name does not contain Linux. Any other non-empty `os` value, including the
string Linux, is `InvalidArgument`. The meter is `meterName` equal to the
SKU, or the SKU followed by a space and `App`, unit `1 Hour`. Stamp, SSL, Domain, and ASIP
meters are skipped. Monthly cost is `retailPrice * 730`. The breakdown key
is `compute`.

### Function App Cost Estimation

`GetProjectedCost` prices `web/FunctionApp`, including Pulumi
`azure:appservice/functionApp:FunctionApp`. The service name is `Functions`.
Classic Consumption (`Standard`, `Y1`, `Dynamic`, `Consumption`, an empty
SKU, or `pricing_model=consumption`) uses `Standard Execution Time`
(unit `1 GB Second`) and `Standard Total Executions` (unit `10`, so billable
executions are divided by 10). When a positive sibling exists, the zero row
is ignored. The free grant of 1,000,000 executions and 400,000 GB-seconds
is applied to this one resource before that divide. Consumption is a usage
total, so it is not multiplied by 730. Components are `executions` and
`gb_seconds`.

Premium uses `Premium vCPU Duration` and `Premium Memory Duration`. Each
component is count times `retailPrice` times 730, once. Keys are `vcpu` and
`memory`. A Function App SKU that matches an App Service plan SKU uses the
plan quote. `EP1` and Flex are `InvalidArgument`.

### AKS Cost Estimation

`GetProjectedCost` prices `containerservice/KubernetesCluster`, including
Pulumi `azure:containerservice/kubernetesCluster:KubernetesCluster`. The
query is the region plus service `Azure Kubernetes Service`. `ArmSkuName`
and `ProductName` stay empty.

Standard control plane uses meter `Standard Uptime SLA`, unit `1 Hour`,
and monthly `retailPrice * 730`. Tag `support=lts` uses
`Standard Long Term Support` instead. Free uses meter
`FreeTierInfrastructureCost Uptime SLA` and keeps the row whose
`effectiveEndDate` is empty. A live query on 2026-10-01 returned one open
row at 0.05 USD per hour, so the control plane month is 36.50. Tier
`Automatic` is `InvalidArgument`.

Node pools use tags `node_pool_1_sku` and `node_pool_1_count`, with optional
`node_pool_1_name` (default `pool_1`), and the same pair for pool 2. Each
pool is one on-demand VM quote multiplied by the count. A cluster Spot tag
is not copied onto the node. Components `control_plane` and
`node_pool_<name>` sum to the monthly cost.

### SQL Database Cost Estimation

`GetProjectedCost` prices `sql/Database`, including Pulumi
`azure:sql/database:Database`, for General Purpose Gen5 provisioned compute
and storage. The query is the region, service `SQL Database`, and the
product name. `ArmSkuName` stays empty. SKU `GP_Gen5_{n}` wins over tags.
`size_gb` is required, and `sizeGb` is accepted.

Compute uses sku `{n} vCore`, meter `vCore`, unit `1 Hour`, on product
`SQL Database Single/Elastic Pool General Purpose - Compute Gen5`. That
retail price already covers n vCores. Monthly compute is `retailPrice * 730`.
The sku named `vCore` is the 1-vCore unit row and is not the match.

Storage uses meter `General Purpose Data Stored`, unit `1 GB/Month`, on
product `SQL Database Single/Elastic Pool General Purpose - Storage`.
Monthly storage is `retailPrice * size_gb`.
`General Purpose Data Stored - Free` is not the overage.

Tag `zone_redundant=true` adds meter `Zone Redundancy vCore` and meter
`General Purpose Zone Redundancy Data Stored`. Components are `compute`,
`storage`, and those two zone keys when zone redundancy was requested.

DTU, serverless, Business Critical, Hyperscale, and other hardware return
`Unimplemented`. The message names the model and `AZ-2.7`.

### Cosmos DB Cost Estimation

`GetProjectedCost` prices `cosmosdb/Account`, including Pulumi
`azure:cosmosdb/account:Account`. The query is the region plus service
`Azure Cosmos DB`. `ArmSkuName` stays empty. Manual provisioned is the
default. Tags `ru_per_second` (or `rus`) are required. `size_gb` is
optional: when it is omitted there is no storage component and no storage
meter is required. A present `size_gb` must be greater than 0 and uses the
storage meter.

Manual RU uses sku `RUs`, meter `100 RU/s`, unit `1/Hour`. Monthly RU cost
is `(ru_per_second / the leading integer in the meter name) * retailPrice * 730`.
Storage uses meter `Data Stored`, unit `1 GB/Month`, and monthly
`retailPrice * size_gb`. Tag `multi_master=true` uses sku `mRUs` for both
meters.

`pricing_model=serverless` uses meter `1M RUs`, unit `1M`. Monthly cost is
`(request_units / 1000000) * retailPrice`. There is no storage component.
A live query on 2026-10-01 returned that meter only. The quote
note says that product publishes no storage meter.
`pricing_model=autoscale` matches a meter that ends with `100 RUs` on
product `Azure Cosmos DB autoscale`, then applies the same `/ 100 * 730`
rule. When `size_gb` is set, storage stays the provisioned `Data Stored`
row. sku `Free`, `Free Tier`, and `RUm` are not selected. Components are
`ru` and, when storage was requested, `storage`.

### Load Balancer Cost Estimation

`GetProjectedCost` prices `network/LoadBalancer`, including classic
`azure:lb/loadBalancer:LoadBalancer` and
`azure-native:network/loadBalancer:LoadBalancer` and
`azure-native:network:LoadBalancer`. The service is
`Load Balancer`. `ArmSkuName` stays empty. An empty name means `Standard`.
The query uses the resource region, then price region `Global` when that
page has no included-rules meter. A live query on 2026-10-01 returned no
rows for one public region and, at `Global`, meter
`Standard Included LB Rules and Outbound Rules` at 0.025 USD per hour.
Monthly cost for that meter is `retailPrice * 730`. Omitted `rule_count`
uses it once, which covers up to 5 load-balancing and outbound rules.
`rule_count` 0 has no hourly charge. A higher count adds meter
`Standard Overage LB Rules and Outbound Rules`. `data_processed_gb` adds
meter `Standard Data Processed` only when set. Inbound NAT rules are not
counted. Gateway and cross-region meters are `InvalidArgument`.

### Other cost RPCs

`DryRun` validates a descriptor and does not call Azure. A known type with
missing fields is supported and not configuration-valid. An unknown type is
not supported, and the RPC still returns a response. The OData filter is
logged, not returned. Field mappings start unsupported, and only a field a
successful `GetProjectedCost` fills is marked supported.

`GetPricingSpec` calls the same quote as `GetProjectedCost` and returns one
`PricingSpec` for that resource. It is not a catalog list.

`GetActualCost` is the projected monthly cost times `hours / 730`. The
default window is 730 hours. The source string carries
`azure-retail-prices[confidence:HIGH|MEDIUM|LOW]`. A FOCUS record is built
only when `SetBillingAccountID` has a non-empty id. The process reads
`FINFOCUS_BILLING_ACCOUNT_ID`. The request has no field for it. An empty
setting logs the validation error and leaves `FocusRecord` nil. The request
field is spec issue 590.

`estimation.SavingsFraction` returns `(onDemand-other)/onDemand` with no
rounding. `PriceItem.SavingsPlan` keeps the nested `savingsPlan` array when
a preview Consumption body includes it. `priceType eq 'SavingsPlan'` does
not return those rows. `ReservationHourly` treats a Reservation
`retailPrice` as the term total and divides by 8760 or 26280. The production
client does not request the preview API. `GetProjectedCost` still returns
one price. The extra prices are not delivered until the spec has a repeated
list. The proposals are spec issues 588 and 589.

`SortRegionPrices` orders Linux on-demand VM rows from saved pages. A miss
is `Found: false` and a zero price that is not a cost. The RPC still prices
one region. A repeated region list is spec issue 589.

Carbon is findings only, in `docs/findings/carbon.md`. No estimator is wired.

## Environment Variables

<!-- markdownlint-disable MD013 -->

| Variable | Default | Description |
| --- | --- | --- |
| `FINFOCUS_PLUGIN_PORT` | 0 (ephemeral) | gRPC listen port |
| `FINFOCUS_LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `FINFOCUS_CACHE_TTL` | 24h | Cache TTL duration (e.g., "10s", "1h", "0s" to disable) |
| `SKIP_INTEGRATION` | (unset) | Set to "true" to skip integration tests |
| `FINFOCUS_BILLING_ACCOUNT_ID` | (unset) | FOCUS billing account id. Empty leaves the actual-cost record unset |

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
- Key normalization: `CacheKey(query)` => `region|armsku|skuname|product|service|currency` (lowercase, trimmed)
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
