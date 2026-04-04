# Data Model: GetProjectedCost RPC

**Branch**: `023-projected-cost-rpc` | **Date**: 2026-04-04

## Entities

### GetProjectedCostRequest (Input — proto-defined)

| Field                    | Type               | Required | Description                         |
| ------------------------ | ------------------ | -------- | ----------------------------------- |
| resource                 | ResourceDescriptor | Yes      | Target Azure resource to price      |
| utilization_percentage   | double             | No       | Utilization factor (future use)     |
| growth_type              | GrowthType         | No       | Growth projection model (future)    |
| growth_rate              | double (optional)  | No       | Growth rate percentage (future)     |
| dry_run                  | bool               | No       | Dry run mode flag (future)          |
| usage_profile            | UsageProfile       | No       | Usage profile override (future)     |

### ResourceDescriptor (Input �� proto-defined)

| Field         | Type              | Required | Validation Rule                               |
| ------------- | ----------------- | -------- | --------------------------------------------- |
| provider      | string            | Yes      | Must be "azure" (case-insensitive)            |
| resource_type | string            | Yes      | Must be in supported set (see below)          |
| region        | string            | Yes      | Falls back to Tags["region"] if empty         |
| sku           | string            | Yes      | Falls back to Tags["sku"] if empty            |
| tags          | map<string,string> | No      | Fallback for region, sku, currency, service   |

**Supported resource types**:

- `compute/VirtualMachine` → full cost computation (this feature)
- `storage/ManagedDisk` → validates but returns Unimplemented (future)
- `storage/BlobStorage` → validates but returns Unimplemented (future)

### PriceQuery (Internal — maps from ResourceDescriptor)

| Field         | Source                              | Default           |
| ------------- | ----------------------------------- | ----------------- |
| ArmRegionName | resource.Region or Tags["region"]   | (required)        |
| ArmSkuName    | resource.Sku or Tags["sku"]         | (required)        |
| ServiceName   | resourceTypeToService lookup        | "Virtual Machines"|
| CurrencyCode  | Tags["currency"] or Tags["currencyCode"] | "USD"        |

### GetProjectedCostResponse (Output — proto-defined)

| Field              | Type                  | Set By        | Description                                      |
| ------------------ | --------------------- | ------------- | ------------------------------------------------ |
| unit_price         | double                | This feature  | Hourly retail price from Azure API               |
| currency           | string                | This feature  | Currency code (e.g., "USD")                      |
| cost_per_month     | double                | This feature  | unit_price × 730                                 |
| billing_detail     | string                | This feature  | Human-readable pricing explanation               |
| pricing_category   | FocusPricingCategory  | This feature  | FOCUS_PRICING_CATEGORY_STANDARD                  |
| expires_at         | Timestamp             | Already done  | Cache expiry hint from CachedResult.ExpiresAt    |
| impact_metrics     | ImpactMetric[]        | Future        | Not set in this feature                          |
| growth_type        | GrowthType            | Future        | Not set in this feature                          |
| dry_run_result     | DryRunResponse        | Future        | Not set in this feature                          |
| spot_interruption  | double                | Future        | Not set in this feature                          |
| prediction_interval| double (optional pair)| Future        | Not set in this feature                          |
| confidence_level   | double (optional)     | Future        | Not set in this feature                          |

## Relationships

```text
GetProjectedCostRequest
  └── ResourceDescriptor
        │
        ├── MapDescriptorToQuery() ──→ PriceQuery
        │                                  │
        │                                  ▼
        │                           CachedClient.GetPrices()
        │                                  │
        │                                  ▼
        │                           CachedResult { Items, ExpiresAt }
        │                                  │
        │                                  ▼
        │                           unitPriceAndCurrency()
        │                                  │
        └──────────────────────────────────▼
                                    GetProjectedCostResponse
```

## Error Flow

```text
nil request/descriptor ──→ ErrMissingRequiredFields ──→ InvalidArgument
wrong provider ──────────→ ErrUnsupportedResourceType ─→ Unimplemented (via MapToGRPCStatus)
unknown resource type ───→ ErrUnsupportedResourceType ─→ Unimplemented
missing region/sku ──��───→ ErrMissingRequiredFields ───→ InvalidArgument
nil cachedClient ────────→ (guard check) ──────────────→ Unimplemented
API failure ─────��───────→ azureclient errors ─────────→ MapToGRPCStatus()
empty price list ────────→ ErrNotFound ────────────────→ NotFound
```

## State Transitions

N/A — stateless request-response. No entity lifecycle.
