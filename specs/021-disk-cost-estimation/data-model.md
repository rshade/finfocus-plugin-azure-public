# Data Model: Managed Disk Cost Estimation

**Feature Branch**: `021-disk-cost-estimation`
**Date**: 2026-03-05

## Entities

### DiskType (value object)

Represents a supported Azure Managed Disk redundancy/performance tier.

| Attribute    | Type   | Description                                        |
| ------------ | ------ | -------------------------------------------------- |
| UserName     | string | User-facing name (e.g., `Premium_SSD_LRS`)         |
| ArmSkuName   | string | Azure API armSkuName (e.g., `Premium_LRS`)         |
| TierPrefix   | string | Meter name prefix (e.g., `P` for Premium)          |
| Redundancy   | string | `LRS` or `ZRS`                                     |

**Supported values**:

| UserName          | ArmSkuName        | TierPrefix | Redundancy |
| ----------------- | ----------------- | ---------- | ---------- |
| Standard_LRS      | Standard_LRS      | S          | LRS        |
| StandardSSD_LRS   | StandardSSD_LRS   | E          | LRS        |
| Premium_SSD_LRS   | Premium_LRS       | P          | LRS        |
| Standard_ZRS      | Standard_ZRS      | S          | ZRS        |
| StandardSSD_ZRS   | StandardSSD_ZRS   | E          | ZRS        |
| Premium_ZRS       | Premium_ZRS       | P          | ZRS        |

**Validation rules**:
- UserName is case-insensitive on input, normalized to canonical form
- `Premium_SSD_LRS` normalizes to `Premium_LRS` for Azure API queries

### DiskTier (value object)

Represents a specific disk size tier within a disk type.

| Attribute  | Type    | Description                                      |
| ---------- | ------- | ------------------------------------------------ |
| Name       | string  | Tier identifier (e.g., `P10`, `S30`, `E6`)       |
| CapacityGB | int     | Provisioned capacity in GiB                      |

**Static mapping** (shared across all disk types — tier numbers are consistent):

| Tier Number | Capacity (GiB) |
| ----------- | -------------- |
| 1           | 4              |
| 2           | 8              |
| 3           | 16             |
| 4           | 32             |
| 6           | 64             |
| 10          | 128            |
| 15          | 256            |
| 20          | 512            |
| 30          | 1024           |
| 40          | 2048           |
| 50          | 4096           |
| 60          | 8192           |
| 70          | 16384          |
| 80          | 32767          |

**Note**: Standard HDD (S-series) starts at tier 4 (32 GiB). Standard SSD and Premium start at tier 1 (4 GiB).

### DiskCostRequest (input)

Attributes extracted from `EstimateCostRequest.Attributes`:

| Attribute   | Type    | Required | Aliases                         | Default |
| ----------- | ------- | -------- | ------------------------------- | ------- |
| region      | string  | yes      | `location`, `region`            | -       |
| disk_type   | string  | yes      | `diskType`, `sku`               | -       |
| size_gb     | float64 | yes      | `sizeGb`, `diskSizeGb`          | -       |
| currency    | string  | no       | `currencyCode`, `currency`      | USD     |

**Validation rules**:
- All required fields must be present; report all missing in single error
- `size_gb` must be > 0 (zero or negative → InvalidArgument)
- `size_gb` is rounded up to nearest integer before tier matching
- `disk_type` must match a supported DiskType (case-insensitive)

### DiskCostResponse (output)

Maps to `EstimateCostResponse` via pluginsdk builder:

| Field            | Source                                  |
| ---------------- | --------------------------------------- |
| cost_monthly     | Selected tier's `retailPrice` (direct)  |
| currency         | From PriceItem.CurrencyCode             |
| pricing_category | FOCUS_PRICING_CATEGORY_STANDARD         |

## Relationships

```text
EstimateCostRequest
  └── attributes (Struct)
        ├── region ─────────────→ PriceQuery.ArmRegionName
        ├── disk_type ──────────→ DiskType.UserName
        │                           └── DiskType.ArmSkuName → PriceQuery.ArmSkuName
        ├── size_gb ────────────→ ceiling match against DiskTier.CapacityGB
        │                           └── DiskTier.Name → filter PriceItem.MeterName
        └── currency ───────────→ PriceQuery.CurrencyCode

PriceQuery ──→ CachedClient.GetPrices() ──→ []PriceItem
                                                └── filter by MeterName == DiskTier.Name
                                                      └── RetailPrice → cost_monthly
```

## State Transitions

None — this is a stateless request/response flow. Cache state is managed by the existing `CachedClient` layer.
