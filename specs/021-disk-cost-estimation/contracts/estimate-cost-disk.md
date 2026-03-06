# Contract: EstimateCost — Managed Disk

**RPC**: `finfocus.v1.CostSourceService/EstimateCost`
**Feature Branch**: `021-disk-cost-estimation`

## Request

Uses existing `EstimateCostRequest` proto message. No schema changes required.

### Required Attributes

```json
{
  "resource_type": "azure:storage/managedDisk:ManagedDisk",
  "attributes": {
    "location": "eastus",
    "disk_type": "Premium_SSD_LRS",
    "size_gb": 128
  }
}
```

### Attribute Aliases

| Canonical Key | Aliases                    |
| ------------- | -------------------------- |
| location      | `region`                   |
| disk_type     | `diskType`, `sku`          |
| size_gb       | `sizeGb`, `diskSizeGb`     |
| currencyCode  | `currency`                 |

### Supported resource_type Values

| Pattern                                   | Match   |
| ----------------------------------------- | ------- |
| `azure:storage/managedDisk:ManagedDisk`   | yes     |
| `storage/managedDisk`                     | yes     |
| `storage/ManagedDisk`                     | yes     |
| (empty string)                            | yes (backward compat — falls through to VM path) |
| `azure:compute/virtualMachine:...`        | no (VM path) |

### Supported disk_type Values

| disk_type         | Azure armSkuName  |
| ----------------- | ----------------- |
| Standard_LRS      | Standard_LRS      |
| StandardSSD_LRS   | StandardSSD_LRS   |
| Premium_SSD_LRS   | Premium_LRS       |
| Standard_ZRS      | Standard_ZRS      |
| StandardSSD_ZRS   | StandardSSD_ZRS   |
| Premium_ZRS       | Premium_ZRS       |

## Response

### Success (200 OK / gRPC OK)

```json
{
  "currency": "USD",
  "cost_monthly": 19.71,
  "pricing_category": "FOCUS_PRICING_CATEGORY_STANDARD"
}
```

- `cost_monthly`: The monthly price for the matched disk tier (direct from Azure API, no hourly conversion)
- `currency`: From Azure pricing response (default USD)

### Errors

| Condition                        | gRPC Code         | Message Example                                              |
| -------------------------------- | ----------------- | ------------------------------------------------------------ |
| Missing required fields          | InvalidArgument   | `missing required field(s): region, disk_type, size_gb`      |
| Unsupported disk type            | InvalidArgument   | `unsupported disk type: UltraSSD_LRS`                        |
| size_gb <= 0                     | InvalidArgument   | `size_gb must be greater than 0`                             |
| Unsupported resource type        | Unimplemented     | `unsupported resource type: compute/ScaleSet`                |
| No pricing found                 | NotFound          | `query [region=eastus sku=Premium_LRS service=Managed Disks]: not found` |
| No tier matches size             | NotFound          | `no disk tier found for 99999 GB with type Premium_SSD_LRS`  |
| Azure API rate limited           | ResourceExhausted | `query ... : rate limited`                                   |
| Azure API unavailable            | Unavailable       | `query ... : service unavailable`                            |
| No cached client configured      | Unimplemented     | `not yet implemented`                                        |

## Internal Flow

```text
EstimateCost(req)
  ├── Extract resource_type
  ├── Route: isManagedDiskResourceType(resource_type)?
  │     ├── yes → estimateDiskCost(req)
  │     │           ├── Extract region, disk_type, size_gb from attributes
  │     │           ├── Validate required fields (report all missing)
  │     │           ├── Validate disk_type is supported
  │     │           ├── Validate size_gb > 0
  │     │           ├── Normalize disk_type → armSkuName
  │     │           ├── Build PriceQuery(region, armSkuName, "Managed Disks", currency)
  │     │           ├── cachedClient.GetPrices(ctx, query) → []PriceItem
  │     │           ├── Ceiling-match size_gb → tier name (e.g., P10)
  │     │           ├── Filter items by meterName == tier name
  │     │           ├── Return retailPrice as cost_monthly
  │     │           └── Error handling → MapToGRPCStatus
  │     └── no → isVirtualMachineResourceType(resource_type)?
  │               ├── yes → existing VM flow (unchanged)
  │               └── no → Unimplemented error
  └── Return EstimateCostResponse
```
