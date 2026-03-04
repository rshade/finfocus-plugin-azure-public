# Data Model: VM Cost Estimation

**Feature**: 020-vm-cost-estimation
**Date**: 2026-03-04

## Entities

### EstimateCostRequest (proto — owned by finfocus-spec)

<!-- markdownlint-disable MD013 -->

| Field | Type | Source | Notes |
| --- | --- | --- | --- |
| `resource_type` | string | Caller | e.g., `azure:compute/virtualMachine:VirtualMachine` |
| `attributes` | structpb.Struct | Caller | Contains `location`, `vmSize`, optional `currency` |

<!-- markdownlint-enable MD013 -->

**Attribute keys** (extracted by `estimateQueryFromRequest`):

| Key | Aliases | Required | Default |
| --- | --- | --- | --- |
| `location` | `region` | Yes | — |
| `vmSize` | `sku`, `armSkuName` | Yes | — |
| `serviceName` | — | No | `"Virtual Machines"` |
| `productName` | — | No | — |
| `currencyCode` | `currency` | No | `"USD"` |

### EstimateCostResponse (proto — owned by finfocus-spec)

<!-- markdownlint-disable MD013 -->

| Field | Type | Populated By | Notes |
| --- | --- | --- | --- |
| `currency` | string | Plugin | ISO 4217 (default: "USD") |
| `cost_monthly` | double | Plugin | `hourly_price * 730` |
| `pricing_category` | FocusPricingCategory | Plugin | `STANDARD` for Consumption |
| `spot_interruption_risk_score` | double | Plugin | `0.0` (not applicable) |

<!-- markdownlint-enable MD013 -->

### PriceQuery (internal — azureclient)

Maps 1:1 from EstimateCostRequest attributes. See `estimateQueryFromRequest()`.

### PriceItem (internal — azureclient)

Azure API response item. Key fields used:

- `RetailPrice` — primary price (fallback: `UnitPrice`)
- `CurrencyCode` — currency from Azure
- `Type` — filtered to "Consumption" by FilterBuilder

### CachedResult (internal — azureclient)

Wraps `[]PriceItem` with cache metadata:

- `Items` — price items from API or cache
- `CreatedAt` — when cached
- `ExpiresAt` — caller-facing cache hint (default 4h)

## Data Flow

```text
EstimateCostRequest
  |
  ├── resource_type ──> validate (must contain "compute/virtualMachine" or empty)
  |
  └── attributes ──> estimateQueryFromRequest() ──> PriceQuery
                                                       |
                                                       v
                                               CachedClient.GetPrices()
                                                       |
                                                       v
                                                  CachedResult
                                                       |
                                                       v
                                               unitPriceAndCurrency()
                                                       |
                                                       v
                                            (unitPrice, currency, err)
                                                       |
                                                       v
                                          EstimateCostResponse {
                                            currency: currency,
                                            cost_monthly: unitPrice * 730,
                                            pricing_category: STANDARD,
                                          }
```

## Validation Rules

<!-- markdownlint-disable MD013 -->

| Rule | Error Code | Message |
| --- | --- | --- |
| `resource_type` non-empty, not VM | Unimplemented | `unsupported resource type: {type}` |
| `location`/`region` missing | InvalidArgument | `missing required field(s): region` |
| `vmSize`/`sku` missing | InvalidArgument | `missing required field(s): sku` |
| Both missing | InvalidArgument | `missing required field(s): region, sku` |
| No pricing data found | NotFound | (from azureclient.ErrNotFound) |
| Azure API unavailable | Unavailable | (from azureclient.ErrServiceUnavailable) |
| Rate limited | ResourceExhausted | (from azureclient.ErrRateLimited) |

<!-- markdownlint-enable MD013 -->

## State Transitions

N/A — stateless request/response. Cache state is managed by `CachedClient`.
