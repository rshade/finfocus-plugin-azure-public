# Data Model: Azure Retail Prices API

**Feature**: 007-azure-price-models
**Date**: 2026-02-03
**Package**: `internal/azureclient`

## Overview

This document defines the Go struct definitions for the Azure Retail Prices API response format. These models enable type-safe JSON parsing of API responses.

## Entity Relationship

```text
┌─────────────────────────────────────────────────────────────┐
│                      PriceResponse                          │
│  (API response envelope)                                    │
├─────────────────────────────────────────────────────────────┤
│  BillingCurrency    : string                                │
│  CustomerEntityID   : string                                │
│  CustomerEntityType : string                                │
│  Items              : []PriceItem  ─────────┐               │
│  NextPageLink       : string                │               │
│  Count              : int                   │               │
└─────────────────────────────────────────────│───────────────┘
                                              │
                                              ▼
┌─────────────────────────────────────────────────────────────┐
│                        PriceItem                            │
│  (Individual pricing record)                                │
├─────────────────────────────────────────────────────────────┤
│  Pricing:                                                   │
│    RetailPrice      : float64                               │
│    UnitPrice        : float64                               │
│    TierMinimumUnits : float64                               │
│    CurrencyCode     : string                                │
│                                                             │
│  Resource:                                                  │
│    ArmRegionName    : string                                │
│    Location         : string                                │
│    ArmSkuName       : string                                │
│    SkuName          : string                                │
│    SkuID            : string                                │
│                                                             │
│  Service:                                                   │
│    ServiceName      : string                                │
│    ServiceID        : string                                │
│    ServiceFamily    : string                                │
│    ProductName      : string                                │
│    ProductID        : string                                │
│                                                             │
│  Meter:                                                     │
│    MeterID          : string                                │
│    MeterName        : string                                │
│    UnitOfMeasure    : string                                │
│    IsPrimaryMeterRegion : bool                              │
│                                                             │
│  Temporal:                                                  │
│    EffectiveStartDate : string (ISO8601)                    │
│                                                             │
│  Type:                                                      │
│    Type             : string                                │
│    ReservationTerm  : string (optional)                     │
│                                                             │
│  Optional:                                                  │
│    AvailabilityID   : string (often empty)                  │
└─────────────────────────────────────────────────────────────┘
```

## Struct Definitions

### PriceResponse

```go
// PriceResponse represents the envelope returned by the Azure Retail Prices API.
// It contains a paginated list of price items along with metadata.
//
// The API returns up to 1000 items per request. When NextPageLink is non-empty,
// additional pages are available.
//
// Example response:
//
//	{
//	  "BillingCurrency": "USD",
//	  "CustomerEntityId": "Default",
//	  "CustomerEntityType": "Retail",
//	  "Items": [...],
//	  "NextPageLink": "https://prices.azure.com/api/retail/prices?$skip=1000",
//	  "Count": 1000
//	}
type PriceResponse struct {
    // BillingCurrency is the currency code for all prices in the response.
    // Typically "USD" unless a different currency was requested.
    BillingCurrency string `json:"BillingCurrency"`

    // CustomerEntityID identifies the customer entity for pricing.
    // Default value is "Default" for retail pricing.
    CustomerEntityID string `json:"CustomerEntityId"`

    // CustomerEntityType indicates the type of pricing returned.
    // Value is "Retail" for the public retail prices API.
    CustomerEntityType string `json:"CustomerEntityType"`

    // Items contains the price entries for this page of results.
    // Maximum 1000 items per response.
    Items []PriceItem `json:"Items"`

    // NextPageLink is the URL to fetch the next page of results.
    // Empty string indicates no more pages are available.
    NextPageLink string `json:"NextPageLink"`

    // Count is the number of items in this page of results.
    Count int `json:"Count"`
}
```

### PriceItem

```go
// PriceItem represents a single price entry from the Azure Retail Prices API.
// Each item corresponds to a specific SKU/meter combination in a region.
//
// Field names match Azure API JSON field names using json struct tags.
// All fields use Go types appropriate for their JSON counterparts.
//
// Example:
//
//	{
//	  "currencyCode": "USD",
//	  "retailPrice": 0.0104,
//	  "armRegionName": "eastus",
//	  "skuName": "B1s",
//	  "serviceName": "Virtual Machines",
//	  "type": "Consumption"
//	}
type PriceItem struct {
    // === Pricing Information ===

    // RetailPrice is the Microsoft retail price without any discount.
    // This is the list price for the resource.
    RetailPrice float64 `json:"retailPrice"`

    // UnitPrice is the price per unit of measure.
    // For most resources, this equals RetailPrice.
    UnitPrice float64 `json:"unitPrice"`

    // TierMinimumUnits is the minimum units of consumption for this price tier.
    // Value is 0 for non-tiered pricing.
    TierMinimumUnits float64 `json:"tierMinimumUnits"`

    // CurrencyCode is the ISO 4217 currency code for the price (e.g., "USD").
    CurrencyCode string `json:"currencyCode"`

    // === Resource Identification ===

    // ArmRegionName is the Azure Resource Manager region identifier (e.g., "eastus").
    // This is the programmatic region name used in ARM templates.
    ArmRegionName string `json:"armRegionName"`

    // Location is the human-readable Azure datacenter location (e.g., "US East").
    // This is the display name shown in the Azure portal.
    Location string `json:"location"`

    // ArmSkuName is the SKU identifier used in Azure Resource Manager (e.g., "Standard_B1s").
    // This is the value used when provisioning resources via ARM.
    ArmSkuName string `json:"armSkuName"`

    // SkuName is the display name for the SKU (e.g., "B1s").
    // This is a human-friendly version of the SKU identifier.
    SkuName string `json:"skuName"`

    // SkuID is the unique identifier for this SKU (e.g., "DZH318Z0BQPS/00TG").
    SkuID string `json:"skuId"`

    // === Service Classification ===

    // ServiceName is the name of the Azure service (e.g., "Virtual Machines").
    ServiceName string `json:"serviceName"`

    // ServiceID is the unique identifier for the Azure service.
    ServiceID string `json:"serviceId"`

    // ServiceFamily is the service category (e.g., "Compute", "Storage", "Networking").
    ServiceFamily string `json:"serviceFamily"`

    // ProductName is the full product name (e.g., "Virtual Machines BS Series").
    ProductName string `json:"productName"`

    // ProductID is the unique identifier for the product.
    ProductID string `json:"productId"`

    // === Meter Information ===

    // MeterID is the unique identifier for the billing meter (UUID format).
    // This is used in Azure cost management and billing.
    MeterID string `json:"meterId"`

    // MeterName is the human-readable name for the meter (e.g., "B1s").
    MeterName string `json:"meterName"`

    // UnitOfMeasure describes the billing unit (e.g., "1 Hour", "1 GB", "10K Transactions").
    UnitOfMeasure string `json:"unitOfMeasure"`

    // IsPrimaryMeterRegion indicates whether this is the primary region for the meter.
    // When true, this region is used for billing purposes.
    IsPrimaryMeterRegion bool `json:"isPrimaryMeterRegion"`

    // === Temporal Data ===

    // EffectiveStartDate is when this price became effective (ISO 8601 format).
    // Example: "2020-08-01T00:00:00Z"
    EffectiveStartDate string `json:"effectiveStartDate"`

    // === Pricing Type ===

    // Type indicates the pricing model for this item.
    // Values: "Consumption", "Reservation", "DevTestConsumption"
    Type string `json:"type"`

    // ReservationTerm is the commitment period for reservation pricing.
    // Only present when Type is "Reservation".
    // Values: "1 Year", "3 Years"
    ReservationTerm string `json:"reservationTerm,omitempty"`

    // === Optional Fields ===

    // AvailabilityID is the availability identifier.
    // Often empty or null in API responses.
    AvailabilityID string `json:"availabilityId,omitempty"`
}
```

### PriceQuery (existing, no changes)

```go
// PriceQuery contains filter parameters for querying prices.
// All fields are optional; empty values are not included in the filter.
type PriceQuery struct {
    // ArmRegionName filters by Azure region (e.g., "eastus").
    ArmRegionName string

    // ArmSkuName filters by SKU name (e.g., "Standard_B1s").
    ArmSkuName string

    // ServiceName filters by service (e.g., "Virtual Machines").
    ServiceName string

    // ProductName filters by product name.
    ProductName string

    // CurrencyCode filters by currency (default: "USD").
    CurrencyCode string
}
```

## Field Type Decisions

| JSON Type | Go Type | Rationale |
|-----------|---------|-----------|
| number (decimal) | `float64` | Prices require decimal precision |
| number (integer) | `int` | Count is always whole numbers |
| string | `string` | Most fields are strings |
| boolean | `bool` | Direct mapping |
| null | `omitempty` tag | Allows graceful handling of missing/null values |
| ISO 8601 date | `string` | Preserves format, avoids timezone parsing |

## Validation Rules

### PriceResponse

- `Items` may be empty but not nil after unmarshaling
- `NextPageLink` is empty string (not nil) when no more pages
- `Count` should equal `len(Items)` but this is not enforced

### PriceItem

- `RetailPrice` and `UnitPrice` are non-negative
- `CurrencyCode` is 3-letter ISO code
- `Type` is one of: "Consumption", "Reservation", "DevTestConsumption"
- `ReservationTerm` only populated when `Type == "Reservation"`
- `EffectiveStartDate` is ISO 8601 format

## JSON Mapping Verification

| Go Field | JSON Field | Direction |
|----------|------------|-----------|
| `BillingCurrency` | `BillingCurrency` | ↔ |
| `CustomerEntityID` | `CustomerEntityId` | ↔ |
| `CustomerEntityType` | `CustomerEntityType` | ↔ |
| `Items` | `Items` | ↔ |
| `NextPageLink` | `NextPageLink` | ↔ |
| `Count` | `Count` | ↔ |
| `RetailPrice` | `retailPrice` | ↔ |
| `UnitPrice` | `unitPrice` | ↔ |
| `TierMinimumUnits` | `tierMinimumUnits` | ↔ |
| `CurrencyCode` | `currencyCode` | ↔ |
| `ArmRegionName` | `armRegionName` | ↔ |
| `Location` | `location` | ↔ |
| `ArmSkuName` | `armSkuName` | ↔ |
| `SkuName` | `skuName` | ↔ |
| `SkuID` | `skuId` | ↔ |
| `ServiceName` | `serviceName` | ↔ |
| `ServiceID` | `serviceId` | ↔ |
| `ServiceFamily` | `serviceFamily` | ↔ |
| `ProductName` | `productName` | ↔ |
| `ProductID` | `productId` | ↔ |
| `MeterID` | `meterId` | ↔ |
| `MeterName` | `meterName` | ↔ |
| `UnitOfMeasure` | `unitOfMeasure` | ↔ |
| `IsPrimaryMeterRegion` | `isPrimaryMeterRegion` | ↔ |
| `EffectiveStartDate` | `effectiveStartDate` | ↔ |
| `Type` | `type` | ↔ |
| `ReservationTerm` | `reservationTerm` | ↔ |
| `AvailabilityID` | `availabilityId` | ↔ |
