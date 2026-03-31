# Data Model: Integration Tests

**Branch**: `022-integration-tests` | **Date**: 2026-03-12

## Entities

### Test Reference Prices

Constant values used for ±25% range assertions. These represent expected
hourly/monthly prices for known SKUs and must be updated periodically.

| Field          | Type    | Description                          |
| -------------- | ------- | ------------------------------------ |
| SKU Name       | string  | Azure ARM SKU identifier             |
| Region         | string  | Azure region name                    |
| Resource Type  | string  | Plugin resource type string          |
| Reference Rate | float64 | Expected hourly (VM) or monthly cost |
| Tolerance      | float64 | Allowed deviation (0.25 = ±25%)      |

**Instances**:

- Standard_B1s / eastus / VM: ~$0.0104/hr (hourly)
- Standard_D2s_v3 / eastus / VM: ~$0.096/hr (hourly)
- 128GB Standard_LRS / eastus / Disk: ~$5.89/mo (monthly)
- 256GB Premium_SSD_LRS / eastus / Disk: ~$38.00/mo (monthly)

### Test Fixture Configuration

Configuration for each integration test case.

| Field         | Type              | Description                       |
| ------------- | ----------------- | --------------------------------- |
| Name          | string            | Test case name                    |
| ResourceType  | string            | Plugin resource type              |
| Attributes    | map[string]any    | EstimateCost request attributes   |
| ExpectError   | bool              | Whether error is expected         |
| ExpectedCode  | codes.Code        | Expected gRPC status code         |
| ReferenceRate | float64           | Reference price for range check   |
| Tolerance     | float64           | Allowed deviation fraction        |

## Relationships

- Test fixtures reference Calculator via `EstimateCost` RPC
- Calculator uses CachedClient which wraps Client
- Client queries live Azure Retail Prices API
- CachedClient exposes Stats for cache hit/miss verification
