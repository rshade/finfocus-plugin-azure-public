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

## Release Please

`.release-please-manifest.json` stays `{ ".": "0.0.0" }` until the first
release pull request merges. `release-please-config.json` sets
`initial-version` to `0.1.0`. A `0.0.0` manifest with no `initial-version`
makes the action open `1.0.0`. Writing `0.1.0` into the manifest before
that release records 0.1.0 as already shipped, so the next feature becomes
0.2.0. Do not edit `CHANGELOG.md`. Release Please owns it. Do not tag the
release from the task list. AZ-5.1 stays skipped.

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
- Empty results return `ErrNotFound` with query context, and `CachedClient` caches that answer
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
- Providers `azure` and `azure-native` are accepted. Resource type matching is case-insensitive and includes type tokens such as `azure:compute/linuxVirtualMachine:LinuxVirtualMachine`, `azure:compute/managedDisk:ManagedDisk`, `azure:storage/account:Account`, `azure:storage/blob:Blob`, `azure:appservice/servicePlan:ServicePlan`, `azure:mssql/database:Database`, `azure-native:compute:VirtualMachine`, `azure-native:compute:Disk`, `azure-native:storage:StorageAccount`, `azure-native:web:AppServicePlan`, `azure-native:web:WebApp` with `kind=FunctionApp`, `azure-native:containerservice:ManagedCluster`, `azure-native:sql:Database`, `azure-native:documentdb:DatabaseAccount`, and `azure-native:network:LoadBalancer`. Windows virtual machines and the real scale-set tokens are quoted. `EstimateCost` sees `kind` on `azure-native:web:WebApp`. A managed disk pricing spec rate uses `per_month`
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
- VM `EstimateCost` reads attribute `priority`. `Spot` uses the Linux Spot row and pricing category Dynamic. An empty priority or `Regular` stays the on-demand row and Standard. Any other value is InvalidArgument. When priority is empty, `pricing_model=spot` selects Spot and `pricing_model=consumption` stays on demand.
- `GetPluginInfo` returns `pluginsdk.SpecVersion` (`v0.7.1`). A value without the `v` prefix is rejected by the SDK
- `GetPluginInfo` sends the explicit `PluginCapabilities()` list: projected costs, actual costs, pricing spec, estimate cost, and dry run. It also sends metadata `type=public-pricing-fallback`, and the SDK adds the legacy `supports_*` keys. The list is explicit because `Calculator` embeds `UnimplementedCostSourceServiceServer`. Without the list, interface inference in the SDK also advertised batch cost, resolve resource types, recommendations, budgets, and dismiss, and core routed calls to them. `pricing.PluginInfo()` is the one source: `GetPluginInfo` serves it and `cmd/` passes it as `ServeConfig.PluginInfo`, so the name (`azure-public`), version, and capabilities agree

### Real Pulumi virtual machines

`GetProjectedCost` prices classic, native, and legacy virtual machines, plus
`linuxVirtualMachineScaleSet`, `windowsVirtualMachineScaleSet`,
`orchestratedVirtualMachineScaleSet`, and
`azure-native:compute:VirtualMachineScaleSet`.

The size is `Sku`, then tags `sku`, `vmSize`, `armSkuName`, `size`,
`hardwareProfile.vmSize`, `sku.name`, and `skuName`. A bare `hardwareProfile`
tag is the size when the core collapsed that object to one value.
`priority=Regular` is on-demand. `priority=Spot` selects the Spot meter.
When `priority` is empty, `virtualMachineProfile.priority=Spot` does too.
`Low` is `InvalidArgument`.

A Windows guest is the classic Windows token, a Windows scale-set token,
native `osProfile.windowsConfiguration` (including the collapsed `osProfile`
string), or legacy `osProfileWindowsConfig`. With no Hybrid Benefit
`licenseType`, the quote uses the product whose name contains Windows.
`Windows_Server` and `Windows_Client` price the base (Linux) rate, and
`billing_detail` says the licence is already paid. A Windows guest is not
given that Linux rate without the note.

The scale-set count is tag `instances`, then `sku.capacity`, otherwise 1.
Monthly cost is the meter times 730 times that count. A native scale set
whose core view dropped `sku.capacity` prices one instance. A dotted native
scale set with `virtualMachineProfile.priority=Spot` uses the Spot meter.
`plan-expected.json` records the on-demand meter for `azure-native/vmss`,
and that fixture has no Spot row, so the dotted quote is `NotFound`.

### Real Pulumi SKU properties

`GetProjectedCost`, `Supports`, and `DryRun` read the SKU from the real
Pulumi property for each type. For most types the descriptor `Sku` wins,
then the property below, then the generic tags (`sku`, `vmSize`,
`armSkuName`, `disk_type`, `diskType`). AKS and storage accounts differ, as
the table says. The Pulumi unknown placeholder
`04da6b54-80e4-46f7-96ec-b56ff0331ba9` is skipped everywhere, including in
`Sku`. `Supports` and `DryRun` refuse the inputs the quote refuses below.

| Type | SKU source |
| --- | --- |
| Managed disk | classic `storageAccountType`, native `sku.name` (`Premium_LRS` is accepted). Size also reads native `diskSizeGB`. Pulumi `tier` is the performance tier: a higher Premium SSD tier (`P30` on a 256 GB disk) is billed instead of the size tier, a lower one is ignored, and a value that is not a P tier is `InvalidArgument` |
| Storage account | `Sku`, then tag `sku`, before the classic properties. Native `sku.name` such as `Standard_GRS`, or classic `accountTier` plus `accountReplicationType`. The access tier is `accessTier`, `access_tier`, or `tier`, default Hot. `Premium` performance is `InvalidArgument`. Native `kind` or classic `accountKind` other than `StorageV2` (for example `Storage` or `BlobStorage`) is `InvalidArgument` |
| App Service plan | classic `skuName`, native `sku.name`. Classic `osType` (any case) is Linux or Windows when `os` is empty. `WindowsContainer` is `InvalidArgument`. Monthly cost is multiplied by `workerCount`, or by `sku.capacity` when `workerCount` is absent. With neither, or an unknown value, one worker is priced and `billing_detail` says the count was not provided |
| AKS | `Sku` (or tag `sku`) wins unless it is native `sku.name` `Base`. Then native `sku.tier`, classic `skuTier`, or tag `tier`. `Automatic` in `Sku` is `InvalidArgument` even with `sku.tier`. `Premium` bills `Standard Long Term Support`. `supportPlan=AKSLongTermSupport` needs `Premium` and is `InvalidArgument` on `Standard` or `Free`. With no node pool tags the note says node pools are not included |
| SQL Database | classic `skuName` (`GP_Gen5_4`), native `sku.name` plus `sku.capacity` (`GP_Gen5` and 4 is `GP_Gen5_4`). Capacity is appended only to `GP_`, `BC_`, and `HS_` names. A vCore name with no count is a missing `sku.capacity` (`InvalidArgument`). A non-numeric or non-positive capacity is `InvalidArgument`. Pulumi `zoneRedundant` is read like `zone_redundant` |

A native plan with no `kind` and no `reserved` is still priced as Linux, and
AKS `defaultNodePool` and `agentPoolProfiles` are not priced (AZ-7.5, AZ-7.6).

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

`storage/BlobStorage` queries the same `General Block Blob v2` product with
skuName `{Tier} {Redundancy}` and uses the same marginal bands on meters
whose name contains `Data Stored`. A size that stays inside the first band
is `retailPrice * size_gb`. When `General Block Blob v2` has no Data Stored
row for that skuName in the region, the quote reads the legacy `Blob Storage`
product instead, and `billing_detail` says so. A SKU that neither product
sells is `NotFound` and names the SKU. The SKU is passed through as given:
it is not normalized, and the `tier`/`redundancy` tags are not read.

Product coverage, Retail Prices API, 2026-10-03:

- In `eastus`, the legacy product sells only LRS, GRS, and RA-GRS, each at the
  same price as `General Block Blob v2`. Only `General Block Blob v2` sells
  ZRS, GZRS, and RA-GZRS.
- In `israelcentral`, `General Block Blob v2` has no Hot or Cold GRS or RA-GRS
  rows. Those are priced from the legacy product (Hot GRS 100 GB is
  6.5494).
- In `southeastasia3`, `General Block Blob v2` charges about 25% more for Cold
  GRS and Cold RA-GRS than the legacy product (Cold GRS 0.010125 against
  0.0081 per GB). The quote uses the `General Block Blob v2` price, because
  that is what a general-purpose v2 account pays.

### App Service Plan Cost Estimation

`GetProjectedCost` prices `web/AppServicePlan`, including Pulumi
`azure:appservice/plan:Plan`. The query is the region plus service
`Azure App Service`, and `ArmSkuName` stays empty. The short SKU is matched
locally with spaces removed, case-insensitive. Linux is the default, from a
`productName` that contains Linux. Tag `os=Windows` selects a product whose
name does not contain Linux. Any other non-empty `os` value, including the
string Linux, is `InvalidArgument`. That rule is for the plugin tag `os`.
When `os` is empty, Pulumi `osType` `Linux` or `Windows` is accepted in any
case (see Real Pulumi SKU properties). The meter is `meterName` equal to the
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
`Standard Long Term Support` instead. Tier `Premium` always uses
`Standard Long Term Support`, because long term support needs Premium. A
live query on 2026-10-03 returned that meter at 0.60 USD per hour. The Free
control plane is 0 and the AKS price page is not queried for it. A live query
on 2026-10-02 returned an open `FreeTierInfrastructureCost Uptime SLA` row at
0.05 USD per hour, effective 2026-10-01. The published AKS pricing page and the
Pricing Calculator still show no Free-tier charge, so that meter is not billed,
and `billing_detail` says so. Node pools on a Free cluster are still priced.
Tier `Automatic` is `InvalidArgument`, including a native `sku.name` of
`Automatic` with `sku.tier` set.

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

Tag `zone_redundant=true` (or Pulumi `zoneRedundant=true`) adds meter
`Zone Redundancy vCore` to compute and bills storage on meter
`General Purpose Zone Redundancy Data Stored` instead of
`General Purpose Data Stored`. Zone storage replaces local storage; it is
not added on top (issue #77). Components are `compute`, `storage`, and
`zone_redundancy_compute` when zone redundancy was requested.

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
successful `GetProjectedCost` fills is marked supported. An empty provider is
taken from the type token: `azure-native:` is azure-native, and `azure:` or a
bare canonical type such as `compute/VirtualMachine` is azure. Another cloud's
token stays unsupported. `finfocus plugin inspect <plugin> <type>` sends only
the type, so it now gets the field mappings and `configuration_errors` naming
the required fields (for example `missing required fields: region, sku`).
DryRun reports identity fields only (region, SKU, tier). It does not name
usage inputs such as `size_gb` or `ru_per_second`; `GetPricingSpec`
`metric_hints` list those. `Supports` and the cost calls still require the
provider, which core always sends there.

`GetPricingSpec` calls the same quote as `GetProjectedCost` and returns one
`PricingSpec` for that resource. It is not a catalog list. FinFocus core shows
it in the `cost estimate` view and uses it for `--pricing-spec-fallback`.
Core's `--pricing-spec-fallback` turns billing modes `per_hour` (rate times
730), `per_gb_month` (rate times GB), and `per_month` (flat) into a monthly
total. The rates below have no core equivalent, so each uses a precise SDK
mode and a unit core does not read. The fallback skips such a spec rather than
price it wrongly; the `cost estimate` view still shows its rate and mode:

| Rate | Mode | Unit |
| --- | --- | --- |
| Cosmos provisioned or autoscale block | `per_ru` | `100 RU/s per hour` |
| Cosmos serverless | `per_ru` | `1M RU` |
| Functions Consumption | `per_second` | `GB-second` |
| Functions Premium | `per_vcpu_hour` | `vCPU-hour` |
| Load Balancer with `rule_count=0` and data processed | `per_data_transfer_gb` | `GB processed` |
| Any other meter | `not_implemented` | the meter unit |

`assumptions` say what the rate covers (730 hours, one on-demand or Spot
instance or worker, the first storage band, the Functions free grant, and for a
Load Balancer with `rule_count=0` that there is no hourly rules charge).
`metric_hints` list the selected meters, then the usage inputs the quote reads:
`size_gb`, `ru_per_second`, `request_units`, `executions`, `gb_seconds`,
`vcpu_count`, `memory_gib`, `workerCount`, `instances`, `rule_count`,
`data_processed_gb`, and `node_pool_N_count`.

When the only missing fields are usage inputs that do not change the rate,
`GetPricingSpec` quotes with a neutral value and returns the unit rate with an
assumption naming them. This applies only where the resulting mode is one core
does not multiply: Cosmos `ru_per_second` and `request_units`, and the
Functions usage tags. Storage, blob, and SQL rates are `per_gb_month` or
`per_hour`, which the fallback would turn into a monthly total, so a missing
`size_gb` stays `InvalidArgument` for them. A missing region, SKU, or tier is
still `InvalidArgument`, and so is a managed disk's `size_gb`, which picks the
tier. `GetProjectedCost` is unchanged.

Issue #44 asked for a type catalog, field requirements, auto-complete, and
marketplace data. FinFocus core reads none of those, so no spec change is
needed. The discovery core does read is the capability list, `plugin inspect`
through DryRun, and the per-resource `GetPricingSpec` described in this section.

`GetActualCost` is the projected monthly cost times `hours / 730`. The
default window is 730 hours. The source string carries
`azure-retail-prices[confidence:HIGH|MEDIUM|LOW]`. A FOCUS record is built
when a billing account id is available. `GetActualCostRequest.billing_account_id`
wins. An empty request id uses `SetBillingAccountID`, which the process sets
from `FINFOCUS_BILLING_ACCOUNT_ID`. Dry run ignores the request id. An empty
id logs the validation error and leaves `FocusRecord` nil. No id is invented.

The record follows FOCUS 1.3. The service, host, provider, publisher, and
invoice issuer names are `Microsoft`, and `billing_account_name` is the id.
`Microsoft` is the right invoice issuer for direct EA and MCA customers; a CSP
customer's issuer is the partner. Contracted cost equals list cost.

A quote billed by one positive meter, whose monthly total is that price times
its count, uses the meter's unit from finops-toolkit PricingUnits.csv:
`1 Hour` and `1 Hours` are `Hours`, `1/Month` is `Units/Month`, and `1 Month`
is `Months`. The quantity is counted (window hours times instances, or window
months), not divided out of the cost, so 24 hours is exactly 24. Every other
quote uses window `Hours` with both unit prices unset. `usage_amount` stays
window hours, while a scale set's FOCUS consumed quantity is instance-hours.

ServiceName, category, and subcategory follow finops-toolkit Services.csv by
resource type, not the Retail API service: disks are Virtual Machines /
Compute / Virtual Machines, scale sets are Virtual Machine Scale Sets, storage
is Storage Accounts, and SQL is Azure SQL Database. App Service plans, and a
Function App on a plan SKU, are Azure App Service / Compute / Other (Compute)
until finfocus-spec#612 adds Web. The SDK logs two one-time deprecation
warnings for the provider and publisher columns. `docs/focus-mapping.md` lists
each column.

`estimation.SavingsFraction` returns `(onDemand-other)/onDemand` with no
rounding. `PriceItem.SavingsPlan` keeps the nested `savingsPlan` array when
a preview Consumption body includes it. `priceType eq 'SavingsPlan'` does
not return those rows. `ReservationHourly` treats a Reservation
`retailPrice` as the term total and divides by 8760 or 26280. A VM
`GetProjectedCost` or `EstimateCost` adds Consumption, Spot, Savings Plan,
and Reservation rows to `price_options`. The selected monthly cost does not
include them. Projected `savings_fraction` compares unit prices. Estimate
compares monthly costs. The fraction is 0 when the selected price is 0.
The preview query sends `api-version=2023-01-01-preview`. Reservation sends
`priceType eq 'Reservation'`. A failed extra query is omitted.

`SortRegionPrices` orders Linux on-demand VM rows. A miss is `Found: false`
and a zero price that is not a cost. A VM quote calls it on the regionless
Consumption page and returns the other regions on `region_prices`. The
requested region stays the parent price. A region with no selected row is
omitted. A Spot quote uses the Linux Spot row for those other regions.

Carbon is findings only, in `docs/findings/carbon.md`. No estimator is wired.

### Manifest files

`manifest.json` and `manifest.yaml` are generated. Their
`supported_resources["azure"]` lists `SupportedResourceTypes()` and
`specBillingModes()`, the modes `specRate` returns. The version is
`pluginVersion`, the same constant `GetPluginInfo` reports.
`TestExpectedManifest_CommittedFiles_MatchExpected` fails when a type is added
without regenerating:

```bash
go test ./internal/pricing -run TestExpectedManifest_CommittedFiles -update-manifest
```

Pass the flag to `./internal/pricing` only.

The files use the `pluginsdk.SaveManifest` format: `protojson` JSON keys in
camel case, YAML keys in lower case, and the installation method as an
enumeration value. `registry.ValidatePluginManifest` reads snake case keys, so
it rejects the files on disk at the first required key. The test validates a
converted in-memory view instead.

Known gaps against the spec schema, filed as FinFocus spec #611:

- `azure-native` stays a provider even though the schema rejects it.
- The `methods` list in the schema allows five names, so `EstimateCost`,
  `DryRun`, and `GetPluginInfo` are served but not listed.
- Per-type input fields have no manifest field, and core reads none. DryRun
  `configuration_errors` name the identity fields, and `GetPricingSpec`
  `metric_hints` name the usage inputs.

`cost_retrieval` matches the `ACTUAL_COSTS` capability that `GetPluginInfo`
advertises. `GetActualCost` is a list-price projection times `hours / 730`, not
billed spend.

Core reads neither file today. The `goreleaser` archive does not include them,
and core's flat `registry.Manifest` struct cannot parse the old layout or the
new one.

## Environment Variables

<!-- markdownlint-disable MD013 -->

| Variable | Default | Description |
| --- | --- | --- |
| `FINFOCUS_PLUGIN_PORT` | 0 (ephemeral) | gRPC listen port |
| `FINFOCUS_LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `FINFOCUS_CACHE_TTL` | 24h | Cache TTL duration (e.g., "10s", "1h", "0s" to disable). Empty price pages are cached for at most one hour, or this TTL when shorter |
| `SKIP_INTEGRATION` | (unset) | Set to "true" to skip integration tests |
| `FINFOCUS_BILLING_ACCOUNT_ID` | (unset) | FOCUS billing account id used when the request id is empty. Empty leaves the actual-cost record unset |

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
- Key normalization: `CacheKey(query)` => `region|armsku|skuname|product|service|currency` (lowercase, trimmed). `type=` and `api=` are appended only when `PriceType` or `APIVersion` is set.
- L1 cache: in-process LRU+TTL (default 1000 entries, 24h TTL)
- L2 hint: `CachedResult.ExpiresAt` (default 4h) propagated to gRPC projected/actual cost responses
- TTL override: `FINFOCUS_CACHE_TTL` env var parsed in `main.go` (e.g., "10s", "1h", "0s" to disable)
- Eviction logging: debug-level structured logs with `cache_key`,
  `eviction_reason` ("lru" or "expired"), and `negative`. Cache hit logs
  also carry `negative`
- Failed responses are never cached: HTTP 404, 429, 5xx, request failures,
  invalid bodies, and the pagination limit are re-requested
- A 200 body is a price page only when it has an `Items` JSON array. Azure's
  empty answer is `{"Items":[],"NextPageLink":null,"Count":0,...}`. A body
  without that array (`{}`, `null`, `"Items":null`, or an `{"Error":...}`
  envelope) is `ErrInvalidResponse` and is not cached
- An HTTP 200 price page with zero rows is cached as a negative entry. A
  later lookup counts as a hit and returns `ErrNotFound` with query context,
  without calling Azure. A VM quote's Reservation query is empty for sizes
  such as `Standard_B1s`, so this saves one request per quote
- Negative entries expire after `CacheConfig.NegativeTTL` (default one
  hour, capped at the L1 TTL), not the 24-hour TTL. A SKU that Azure
  publishes after an empty answer stays invisible until that entry expires
  or the process restarts. Negative entries share the LRU slots with real
  price pages
- Stats: `cachedClient.Stats().Hits.Load()` / `cachedClient.Stats().Misses.Load()`

## Zerolog

 The constant already exists in finfocus-spec at sdk/go/pluginsdk/logging.go:24-25:

  // TraceIDMetadataKey is the gRPC metadata header for trace ID propagation.
  const TraceIDMetadataKey = "x-finfocus-trace-id"

  Along with:
  - TracingUnaryServerInterceptor() - server-side interceptor
  - TraceIDFromContext(ctx) - context extraction
  - ContextWithTraceID(ctx, traceID) - context storage
