# Research: Managed Disk Cost Estimation

**Feature Branch**: `021-disk-cost-estimation`
**Date**: 2026-03-05

## R1: Azure Retail Prices API — Managed Disk Query Patterns

### Decision
Query using `serviceName eq 'Managed Disks'` with `armSkuName` set to the disk type (e.g., `Premium_LRS`). The API returns **multiple items** — one per size tier (P10, P20, P30, etc.) — each with a `meterName` identifying the tier and `retailPrice` as the monthly cost.

### Rationale
- The mapper already has `storage/manageddisk` -> `"Managed Disks"` (mapper.go:22)
- Azure API returns all tiers for a given armSkuName in a single query, allowing client-side tier selection
- `UnitOfMeasure` for disks is `"1/Month"` or `"1 GB/Month"`, confirming monthly pricing (no hourly conversion needed)
- `priceType eq 'Consumption'` filter is applied automatically by FilterBuilder

### Alternatives Considered
- **Querying per-tier with meterName filter**: Would require knowing the exact tier name upfront (P10, S30, etc.), adding complexity. Rejected because a single query returns all tiers.
- **Using productName filter**: More specific (e.g., `"Premium SSD Managed Disks"`) but armSkuName is sufficient and consistent with VM pattern.

## R2: Disk Type to Azure SKU Mapping

### Decision
Map user-facing disk type names directly to `armSkuName` values:

| User Disk Type    | armSkuName        | Azure Product Family            | Tier Prefix |
| ----------------- | ----------------- | ------------------------------- | ----------- |
| Standard_LRS      | Standard_LRS      | Standard HDD Managed Disks      | S           |
| StandardSSD_LRS   | StandardSSD_LRS   | Standard SSD Managed Disks      | E           |
| Premium_SSD_LRS   | Premium_LRS       | Premium SSD Managed Disks       | P           |
| Standard_ZRS      | Standard_ZRS      | Standard HDD Managed Disks ZRS  | S (ZRS)     |
| StandardSSD_ZRS   | StandardSSD_ZRS   | Standard SSD Managed Disks ZRS  | E (ZRS)     |
| Premium_ZRS       | Premium_ZRS       | Premium SSD Managed Disks ZRS   | P (ZRS)     |

### Rationale
- Azure API uses `armSkuName` values that match the user-facing redundancy suffix (LRS/ZRS)
- `Premium_SSD_LRS` in user input maps to `Premium_LRS` in Azure API (the "SSD" is implicit in Premium)
- Tier prefixes (S, E, P) are embedded in `meterName` responses (e.g., "P10", "E30", "S4")

### Alternatives Considered
- **Passing user input directly as armSkuName**: Would fail for `Premium_SSD_LRS` since Azure uses `Premium_LRS`. Need a normalization step.

## R3: Size-to-Tier Ceiling Match Strategy

### Decision
Build a static lookup table mapping tier names to their capacity in GiB. When processing a request:
1. Query Azure API for all tiers of the given disk type + region
2. Filter response items to `isPrimaryMeterRegion == true` and `type == "Consumption"`
3. Parse `meterName` to identify tier (e.g., "P10", "S30")
4. Look up tier capacity from static table
5. Select the smallest tier whose capacity >= requested `size_gb`
6. Return that tier's `retailPrice` as monthly cost

### Standard Tier Capacity Table (GiB)

| Tier | S (HDD)  | E (SSD)  | P (Premium) |
| ---- | -------- | -------- | ----------- |
| 1    | -        | 4        | 4           |
| 2    | -        | 8        | 8           |
| 3    | -        | 16       | 16          |
| 4    | 32       | 32       | 32          |
| 6    | 64       | 64       | 64          |
| 10   | 128      | 128      | 128         |
| 15   | 256      | 256      | 256         |
| 20   | 512      | 512      | 512         |
| 30   | 1024     | 1024     | 1024        |
| 40   | 2048     | 2048     | 2048        |
| 50   | 4096     | 4096     | 4096        |
| 60   | 8192     | 8192     | 8192        |
| 70   | 16384    | 16384    | 16384       |
| 80   | 32767    | 32767    | 32767       |

### Rationale
- Ceiling match mirrors Azure's own provisioning behavior (you always get billed at the tier that fits your requested size)
- Static capacity table is stable (tier sizes haven't changed since inception) and avoids embedding bulk pricing data (only sizes, not prices)
- Filtering `isPrimaryMeterRegion` avoids duplicate entries for secondary regions

### Alternatives Considered
- **Dynamic tier discovery from API response**: Would not know capacities without a lookup table since `meterName` only contains the tier name (e.g., "P10"), not the size
- **Requiring exact tier name input**: Too cumbersome for users; they think in GB, not tier names

## R4: Existing Calculator Extension Points

### Decision
Extend the existing `EstimateCost` method in `calculator.go` with a resource-type routing pattern:
1. Replace `isVirtualMachineResourceType()` gate with a broader resource type check
2. Add `isManagedDiskResourceType()` helper
3. Route to a disk-specific estimation function that handles tier selection
4. Keep VM path unchanged for backward compatibility

### Rationale
- Minimal change to existing code — only the routing logic in `EstimateCost` changes
- Disk-specific logic is isolated in a new function, keeping complexity low
- Reuses existing `cachedClient.GetPrices()`, `MapToGRPCStatus()`, and error handling patterns

### Alternatives Considered
- **Separate gRPC method for disks**: Rejected — `EstimateCost` is the canonical RPC; resource_type routing is the intended pattern
- **Strategy pattern with interface**: Over-engineering for 2 resource types; simple function dispatch is sufficient

## R5: Response Mapping — Disk vs VM

### Decision
For disk estimation, return the tier's `retailPrice` directly as `cost_monthly` (no `HoursPerMonth` multiplication). Use the same `pluginsdk.NewEstimateCostResponse` builder.

### Rationale
- VM pricing: `retailPrice` is hourly → multiply by `HoursPerMonth` (730) for monthly
- Disk pricing: `retailPrice` is already monthly → use directly
- The `EstimateCostResponse` message is agnostic to pricing model; it just carries `cost_monthly`

### Alternatives Considered
- **Always normalizing to hourly then back to monthly**: Adds unnecessary precision loss for disks and creates misleading intermediate values
