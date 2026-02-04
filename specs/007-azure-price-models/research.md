# Research: Azure Retail Prices API Data Models

**Feature**: 007-azure-price-models
**Date**: 2026-02-03
**Source**: [Azure Retail Prices API Documentation](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices)

## Executive Summary

The Azure Retail Prices API (`https://prices.azure.com/api/retail/prices`) returns pricing data in a well-documented JSON format. The existing implementation in `internal/azureclient/types.go` covers 18 of 22+ documented fields. This research documents all available fields and identifies gaps.

## Azure API Response Structure

### Response Envelope

| Field | Type | Description | Present in Code |
|-------|------|-------------|-----------------|
| `BillingCurrency` | string | Currency for all prices | ✅ Yes |
| `CustomerEntityId` | string | Customer entity identifier | ✅ Yes |
| `CustomerEntityType` | string | Entity type (e.g., "Retail") | ✅ Yes |
| `Items` | array | Array of PriceItem objects | ✅ Yes |
| `NextPageLink` | string | URL for next page (empty if none) | ✅ Yes |
| `Count` | int | Number of items in this page | ✅ Yes |

**Decision**: Response envelope is complete. Export as `PriceResponse` for external use.

### PriceItem Fields

#### Core Pricing Fields

| Field | Type | Example | Present | Notes |
|-------|------|---------|---------|-------|
| `retailPrice` | float64 | `0.0104` | ✅ | Retail price without discount |
| `unitPrice` | float64 | `0.0104` | ✅ | Same as retailPrice for most cases |
| `tierMinimumUnits` | float64 | `0` | ✅ | Min units for tiered pricing |
| `currencyCode` | string | `"USD"` | ✅ | ISO currency code |

#### Resource Identification

| Field | Type | Example | Present | Notes |
|-------|------|---------|---------|-------|
| `armRegionName` | string | `"eastus"` | ✅ | ARM region identifier |
| `location` | string | `"US East"` | ❌ | Human-readable location |
| `armSkuName` | string | `"Standard_B1s"` | ✅ | ARM SKU identifier |
| `skuName` | string | `"B1s"` | ✅ | SKU display name |
| `skuId` | string | `"DZH318Z0BQPS/00TG"` | ✅ | Unique SKU identifier |

#### Service Classification

| Field | Type | Example | Present | Notes |
|-------|------|---------|---------|-------|
| `serviceName` | string | `"Virtual Machines"` | ✅ | Azure service name |
| `serviceId` | string | `"DZH313Z7MMC8"` | ✅ | Unique service identifier |
| `serviceFamily` | string | `"Compute"` | ✅ | Service category |
| `productName` | string | `"Virtual Machines BS Series"` | ✅ | Product display name |
| `productId` | string | `"DZH318Z0BQPS"` | ✅ | Unique product identifier |

#### Meter Information

| Field | Type | Example | Present | Notes |
|-------|------|---------|---------|-------|
| `meterId` | string | `"000a794b-bdb0-..."` | ✅ | Unique meter identifier (UUID) |
| `meterName` | string | `"B1s"` | ✅ | Human-readable meter name |
| `unitOfMeasure` | string | `"1 Hour"` | ✅ | Billing unit description |
| `isPrimaryMeterRegion` | bool | `true` | ✅ | Primary region for billing |

#### Temporal Data

| Field | Type | Example | Present | Notes |
|-------|------|---------|---------|-------|
| `effectiveStartDate` | string | `"2020-08-01T00:00:00Z"` | ✅ | ISO8601 timestamp |

#### Pricing Type

| Field | Type | Example | Present | Notes |
|-------|------|---------|---------|-------|
| `type` | string | `"Consumption"` | ✅ | Pricing type |
| `reservationTerm` | string | `"1 Year"` | ❌ | Only for type="Reservation" |

#### Optional Fields

| Field | Type | Example | Present | Notes |
|-------|------|---------|---------|-------|
| `availabilityId` | string | `null` | ❌ | Often null in responses |

#### Preview API Fields (v2023-01-01-preview)

| Field | Type | Description | Present | Notes |
|-------|------|-------------|---------|-------|
| `savingsPlan` | array | Savings plan pricing options | ❌ | Only in preview API |

## Decisions

### Decision 1: Add Missing Fields

**Decision**: Add `location`, `reservationTerm`, and `availabilityId` fields.

**Rationale**:

- `location` provides human-readable region names useful for display
- `reservationTerm` is required to distinguish 1-year vs 3-year reservation pricing
- `availabilityId` is in the API response (even if often null)

**Alternatives Considered**:

- Skip `availabilityId` since it's usually null → Rejected; better to have complete model
- Add `savingsPlan` for preview API → Deferred; not needed for current use cases

### Decision 2: Export PriceResponse

**Decision**: Rename `priceResponse` to `PriceResponse` (exported).

**Rationale**:

- External packages may need to work with response envelopes
- Consistent with Go convention of exporting public API types
- Enables mocking in tests without accessing internal types

**Alternatives Considered**:

- Keep unexported, add accessor methods → Rejected; overly complex for simple data
- Create separate `models` package → Rejected; types belong with client that uses them

### Decision 3: String Type for Dates

**Decision**: Keep `effectiveStartDate` as `string`, not `time.Time`.

**Rationale**:

- Preserves exact API format for round-trip fidelity
- Avoids timezone parsing complexity
- Callers can parse as needed with `time.Parse()`

**Alternatives Considered**:

- Use `time.Time` with custom unmarshaler → Rejected; adds complexity, breaks round-trip
- Use `*time.Time` for optionality → Rejected; string handles empty case naturally

### Decision 4: Omit savingsPlan

**Decision**: Do not implement `savingsPlan` field in this iteration.

**Rationale**:

- Requires preview API version (`api-version=2023-01-01-preview`)
- Current implementation uses GA API
- Can be added later when savings plan support is needed

**Alternatives Considered**:

- Add as `json.RawMessage` for forward compatibility → Rejected; unused fields add noise
- Support both API versions → Rejected; scope creep for data model task

## Sample JSON for Testing

### Complete PriceItem Example

```json
{
  "currencyCode": "USD",
  "tierMinimumUnits": 0.0,
  "retailPrice": 0.0104,
  "unitPrice": 0.0104,
  "armRegionName": "eastus",
  "location": "US East",
  "effectiveStartDate": "2020-08-01T00:00:00Z",
  "meterId": "000a794b-bdb0-58be-a0cd-0c3a0f222923",
  "meterName": "B1s",
  "productId": "DZH318Z0BQPS",
  "skuId": "DZH318Z0BQPS/00TG",
  "availabilityId": null,
  "productName": "Virtual Machines BS Series",
  "skuName": "B1s",
  "serviceName": "Virtual Machines",
  "serviceId": "DZH313Z7MMC8",
  "serviceFamily": "Compute",
  "unitOfMeasure": "1 Hour",
  "type": "Consumption",
  "isPrimaryMeterRegion": true,
  "armSkuName": "Standard_B1s"
}
```

### Reservation PriceItem Example

```json
{
  "currencyCode": "USD",
  "tierMinimumUnits": 0.0,
  "retailPrice": 3285.0,
  "unitPrice": 3285.0,
  "armRegionName": "eastus",
  "location": "US East",
  "effectiveStartDate": "2023-01-01T00:00:00Z",
  "meterId": "abc123-reservation-meter",
  "meterName": "D2s v3 Reserved",
  "productId": "DZH318Z0BQPS",
  "skuId": "DZH318Z0BQPS/RESV",
  "productName": "Virtual Machines Dv3 Series",
  "skuName": "D2s v3",
  "serviceName": "Virtual Machines",
  "serviceId": "DZH313Z7MMC8",
  "serviceFamily": "Compute",
  "unitOfMeasure": "1 Year",
  "type": "Reservation",
  "reservationTerm": "1 Year",
  "isPrimaryMeterRegion": true,
  "armSkuName": "Standard_D2s_v3"
}
```

### Complete Response Envelope Example

```json
{
  "BillingCurrency": "USD",
  "CustomerEntityId": "Default",
  "CustomerEntityType": "Retail",
  "Items": [
    {
      "currencyCode": "USD",
      "retailPrice": 0.0104,
      "armRegionName": "eastus",
      "skuName": "B1s",
      "type": "Consumption"
    }
  ],
  "NextPageLink": "https://prices.azure.com/api/retail/prices?$skip=1000",
  "Count": 1
}
```

## References

- [Azure Retail Prices API Documentation](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices)
- [OData Query Options](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices#odata-query-options)
- [Savings Plans Preview API](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices#savings-plans)
