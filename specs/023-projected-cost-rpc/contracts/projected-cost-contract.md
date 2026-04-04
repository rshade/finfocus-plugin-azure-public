# API Contract: GetProjectedCost RPC

**Branch**: `023-projected-cost-rpc` | **Date**: 2026-04-04

## Proto Definition (finfocus-spec v0.5.7)

The proto is defined in the `finfocus-spec` repository. This document
captures the behavioral contract for the Azure plugin implementation.

## Request Contract

```text
GetProjectedCost(GetProjectedCostRequest) → GetProjectedCostResponse

Required input (via ResourceDescriptor):
  - provider:      "azure" (case-insensitive, required)
  - resource_type: "compute/VirtualMachine" (case-insensitive, required)
  - region:        Azure region name (e.g., "eastus", required)
  - sku:           Azure SKU name (e.g., "Standard_B1s", required)

Optional input (via ResourceDescriptor.tags):
  - currency/currencyCode: Currency code (default: "USD")
  - service/serviceName:   Azure service name (default: per resource type)
  - product/productName:   Azure product name (optional filter)
```

## Response Contract

### Success (VM resource type)

```text
unit_price:       Hourly retail price from Azure API (float64)
currency:         Currency code from API response (string, e.g., "USD")
cost_per_month:   unit_price × 730 (float64)
billing_detail:   "Azure Retail Prices API: {SKU} in {region} at
                   ${unit_price}/hr * 730 hrs/mo" (string)
pricing_category: FOCUS_PRICING_CATEGORY_STANDARD (enum)
expires_at:       Cache expiry timestamp from CachedResult (Timestamp)
```

### Error Responses

| Condition                 | gRPC Code        | Error Message Pattern                         |
| ------------------------- | ---------------- | --------------------------------------------- |
| nil resource descriptor   | InvalidArgument  | "missing required fields: descriptor is nil"  |
| provider != "azure"       | Unimplemented    | "unsupported provider: {provider}: ..."       |
| unknown resource type     | Unimplemented    | "unsupported resource type: {type}: ..."      |
| missing region            | InvalidArgument  | "missing required fields: region"             |
| missing sku               | InvalidArgument  | "missing required fields: sku"                |
| missing region + sku      | InvalidArgument  | "missing required fields: region, sku"        |
| nil cachedClient          | Unimplemented    | "not yet implemented"                         |
| Azure API not found       | NotFound         | Query context + "not found"                   |
| Azure API rate limited    | ResourceExhausted| Query context + rate limit details            |
| Azure API unavailable     | Unavailable      | Query context + service unavailable           |
| Azure API other error     | Internal         | Query context + error details                 |
| context canceled          | Canceled         | Context cancellation message                  |
| context deadline exceeded | DeadlineExceeded | Deadline exceeded message                     |

## Logging Contract

| Event              | Level | Required Fields                                              |
| ------------------ | ----- | ------------------------------------------------------------ |
| Request entry      | Info  | region, sku, resource_type, provider                         |
| Success            | Info  | region, sku, resource_type, cost_monthly, currency, unit_price, result_status=success |
| Validation failure | Warn  | resource_type (if available), result_status=error, error     |
| API/cache failure  | Error | region, sku, resource_type, result_status=error, error       |
| Cache unavailable  | Warn  | result_status=error, error                                   |

## Performance Contract

| Scenario       | Target         |
| -------------- | -------------- |
| Cache hit      | < 10ms (p99)   |
| Cache miss     | < 2s (p95)     |
| Cache miss     | < 5s (p99)     |
