# Data Model: HTTP Client with Retry Logic

**Feature**: 006-http-client-retry
**Date**: 2026-02-03

## Entities

### Client

The main HTTP client wrapper for Azure Retail Prices API.

| Field | Type | Description |
|-------|------|-------------|
| `httpClient` | `*retryablehttp.Client` | Underlying retryable HTTP client |
| `baseURL` | `string` | Base URL for Azure API (default: `https://prices.azure.com/api/retail/prices`) |
| `userAgent` | `string` | User-Agent header value |
| `logger` | `zerolog.Logger` | Structured logger for retry events |

**Validation Rules**:
- `baseURL` must be a valid URL (validated at construction)
- `userAgent` must be non-empty

**Lifecycle**:
- Created once at plugin startup via `NewClient()`
- Reused for all pricing requests (thread-safe)
- No explicit shutdown required (uses connection pooling)

### ClientConfig

Configuration options for creating a Client.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `BaseURL` | `string` | `https://prices.azure.com/api/retail/prices` | API base URL |
| `RetryMax` | `int` | `3` | Maximum retry attempts |
| `RetryWaitMin` | `time.Duration` | `1s` | Minimum backoff duration |
| `RetryWaitMax` | `time.Duration` | `30s` | Maximum backoff duration |
| `Timeout` | `time.Duration` | `60s` | Request timeout |
| `Logger` | `zerolog.Logger` | `zerolog.Nop()` | Logger instance |

**Validation Rules**:
- `RetryMax` must be >= 0
- `RetryWaitMin` must be > 0 and <= `RetryWaitMax`
- `Timeout` must be > 0

### PriceResponse

Azure Retail Prices API response envelope.

| Field | Type | Description |
|-------|------|-------------|
| `BillingCurrency` | `string` | Currency code (e.g., "USD") |
| `CustomerEntityId` | `string` | Customer entity identifier |
| `CustomerEntityType` | `string` | Customer entity type |
| `Items` | `[]PriceItem` | Array of price items |
| `NextPageLink` | `string` | URL for next page (empty if last page) |
| `Count` | `int` | Number of items in this response |

**State Transitions**:
- N/A (immutable response from API)

### PriceItem

Individual pricing item from Azure API.

| Field | Type | Description |
|-------|------|-------------|
| `ArmRegionName` | `string` | Azure region (e.g., "eastus") |
| `ArmSkuName` | `string` | SKU name (e.g., "Standard_B1s") |
| `CurrencyCode` | `string` | Price currency (e.g., "USD") |
| `EffectiveStartDate` | `string` | Price effective date (ISO 8601) |
| `IsPrimaryMeterRegion` | `bool` | Primary meter region flag |
| `MeterId` | `string` | Meter identifier (GUID) |
| `MeterName` | `string` | Human-readable meter name |
| `ProductId` | `string` | Product identifier |
| `ProductName` | `string` | Human-readable product name |
| `RetailPrice` | `float64` | Retail price per unit |
| `ServiceFamily` | `string` | Service family (e.g., "Compute") |
| `ServiceId` | `string` | Service identifier |
| `ServiceName` | `string` | Human-readable service name |
| `SkuId` | `string` | SKU identifier |
| `SkuName` | `string` | Human-readable SKU name |
| `TierMinimumUnits` | `float64` | Minimum units for tier pricing |
| `Type` | `string` | Price type (e.g., "Consumption") |
| `UnitOfMeasure` | `string` | Unit (e.g., "1 Hour") |
| `UnitPrice` | `float64` | Unit price |

**Validation Rules**:
- `RetailPrice` and `UnitPrice` must be >= 0
- JSON field names use PascalCase (Azure API convention)

### PriceQuery

Query parameters for filtering prices.

| Field | Type | Description |
|-------|------|-------------|
| `ArmRegionName` | `string` | Filter by region |
| `ArmSkuName` | `string` | Filter by SKU |
| `ServiceName` | `string` | Filter by service |
| `ProductName` | `string` | Filter by product |
| `CurrencyCode` | `string` | Filter by currency (default: "USD") |

**Validation Rules**:
- At least one filter field should be provided (empty query returns all prices, very large)
- Field values are case-sensitive per Azure API behavior

## Relationships

```text
Client --uses--> ClientConfig (at construction)
Client --returns--> PriceResponse (from API calls)
Client --accepts--> PriceQuery (for filtering)
PriceResponse --contains--> []PriceItem
```

## Error Types

### ClientError

Errors returned by the client.

| Error | Description | Retryable |
|-------|-------------|-----------|
| `ErrTimeout` | Request exceeded timeout | No (context deadline) |
| `ErrRateLimited` | Exceeded retry attempts on 429 | No (retries exhausted) |
| `ErrServiceUnavailable` | Exceeded retry attempts on 503 | No (retries exhausted) |
| `ErrInvalidResponse` | JSON parsing failed | No (server bug) |
| `ErrContextCancelled` | Context was cancelled | No (caller cancelled) |

All errors wrap underlying errors for context using `fmt.Errorf("...: %w", err)`.
