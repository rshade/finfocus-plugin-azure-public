# estimate-cost Specification

## Purpose

Price a resource from a resource type and a free-form attribute map, for callers that have no
`ResourceDescriptor`, using the same Azure Retail Prices quotes as projected cost.

## Requirements

### Requirement: Virtual machine estimate

`EstimateCost` SHALL price a virtual machine as the on-demand Linux hourly `retailPrice` times 730,
with pricing category `FOCUS_PRICING_CATEGORY_STANDARD` and the price currency. An empty
`resource_type` SHALL be routed to the virtual machine path. When the price page holds Windows or
Low Priority rows first, the on-demand Linux row SHALL be selected.

Tests: `TestEstimateCost_ValidRequest_ReturnsCostMonthlyEqualToHourlyTimes730`,
`TestEstimateCost_ValidRequest_SetsPricingCategoryStandard`,
`TestEstimateCost_EmptyResourceType_Succeeds`,
`TestEstimateCost_Disk_ResourceTypeRouting`, `TestEstimateCostUsesCachedClient`,
`TestEstimateCostD2sV3UsesOnDemandLinux`

#### Scenario: Hourly to monthly

- **WHEN** `EstimateCost` receives type `azure:compute/virtualMachine:VirtualMachine`, `location`
  `eastus`, `vmSize` `Standard_B1s`, and the price is 0.02 per hour
- **THEN** `cost_monthly` is 14.6 and the pricing category is Standard

#### Scenario: Empty resource type

- **WHEN** `resource_type` is empty and the attributes name a region and VM size
- **THEN** the virtual machine price is returned

#### Scenario: On-demand Linux row

- **WHEN** the eastus `Standard_D2s_v3` page starts with a Windows Low Priority row at 0.075
- **THEN** `cost_monthly` is 0.096 times 730

### Requirement: Virtual machine attribute aliases

The region SHALL be read from `location` or `region`, the SKU from `vmSize`, `sku`, or
`armSkuName`, and the currency from `currency` or `currencyCode`, defaulting to `USD`. The service
SHALL be `Virtual Machines`. Missing fields SHALL be reported together as
`missing required field(s): region`, `... sku`, or `... region, sku`.

Tests: `TestEstimateQueryFromRequest_AliasHandling`,
`TestEstimateQueryFromRequest_ValidInput_ReturnsQuery`,
`TestEstimateQueryFromRequest_MissingRegion_ReturnsError`,
`TestEstimateQueryFromRequest_MissingBoth_ReturnsError`

#### Scenario: armSkuName alias

- **WHEN** the attributes are `location=eastus` and `armSkuName=Standard_B4ms`
- **THEN** the query SKU is `Standard_B4ms` and the currency is `USD`

### Requirement: Missing VM fields are InvalidArgument

A virtual machine request missing the region, the SKU, or both SHALL return `InvalidArgument`
naming each missing field.

Tests: `TestEstimateCost_MissingRegion_ReturnsInvalidArgument`,
`TestEstimateCost_MissingSKU_ReturnsInvalidArgument`,
`TestEstimateCost_MissingBothFields_ReturnsInvalidArgument`

#### Scenario: No attributes

- **WHEN** `EstimateCost` receives a VM type and no attributes
- **THEN** the status is `InvalidArgument` and the message names `region` and `sku`

### Requirement: Unsupported types are Unimplemented

A non-empty `resource_type` that is not a supported type, such as `custom/Widget` or
`azure:compute/virtualMachineScaleSet:VirtualMachineScaleSet`, SHALL return `Unimplemented` with
a message containing `unsupported resource type`. A calculator created without a pricing client
SHALL return `Unimplemented`.

Tests: `TestEstimateCost_UnsupportedResourceType_ReturnsUnimplemented`,
`TestEstimateCost_VirtualMachineScaleSet_ReturnsUnimplemented`,
`TestEstimateCostReturnsUnimplemented`

#### Scenario: Unknown type

- **WHEN** `resource_type` is `custom/Widget`
- **THEN** the status is `Unimplemented` and the message contains `unsupported resource type`

### Requirement: Every mapped type is served

`EstimateCost` SHALL serve every type in `SupportedResourceTypes` over gRPC: none SHALL return
`Unimplemented`. A type that is not a VM or managed disk SHALL use the projected-cost quote, so an
`azure-native:web:WebApp` with attribute `kind=FunctionApp` SHALL be priced as a Consumption
Function App from `executions` and `gb_seconds` after the free grant.

Tests: `TestEstimateCostEveryMappedTypeOverGRPC`, `TestEstimateCostNativeFunctionWebApp`

#### Scenario: Native Function App

- **WHEN** `EstimateCost` receives `azure-native:web:WebApp` with `kind=FunctionApp`,
  `executions`, and `gb_seconds`
- **THEN** `cost_monthly` is the billable executions divided by 10 times the execution price plus
  the billable GB-seconds times the execution-time price

### Requirement: VM priority selects Spot or on-demand

Attribute `priority` `Spot` SHALL select the Linux Spot row and pricing category Dynamic. An empty
priority SHALL stay on-demand with category Standard. Any other value, such as `LowPriority`,
SHALL return `InvalidArgument` naming the value.

Tests: `TestEstimateCostSpotD2sV3Eastus`

#### Scenario: Spot

- **WHEN** `EstimateCost` receives `Standard_D2s_v3` in `eastus` with `priority=Spot`
- **THEN** `cost_monthly` is the Spot hourly price times 730 (about 13.74)
- **AND** the pricing category is `FOCUS_PRICING_CATEGORY_DYNAMIC`

### Requirement: Not found and caching

An empty price page SHALL return `NotFound`. A repeated identical request SHALL be served from the
cache: the first VM quote sends four upstream requests (selected page, preview, reservation, other
regions), and the second sends none and returns the same cost.

Tests: `TestEstimateCost_NotFoundSKU_ReturnsNotFound`,
`TestEstimateCost_RepeatedQuery_UsesCacheOnSecondCall`,
`TestEstimateCost_CacheStats_RecordsHitAndMiss`

#### Scenario: Second call is a cache hit

- **WHEN** the same VM estimate is requested twice
- **THEN** the cache records four misses and four hits
- **AND** both responses have the same `cost_monthly`

### Requirement: Managed disk estimate

A managed disk type SHALL read `location`, `disk_type`, and `size_gb`, and SHALL return
`InvalidArgument` naming every missing one (`region, disk_type, size_gb`), for `size_gb` of 0 or
less (`size_gb must be greater than 0`), and for a disk type outside `Standard_LRS`,
`StandardSSD_LRS`, `Premium_SSD_LRS`, `Standard_ZRS`, `StandardSSD_ZRS`, and `Premium_ZRS`
(`unsupported disk type`, for example `UltraSSD_LRS` or `PremiumV2_LRS`). The size SHALL map to
the smallest tier at least that size (100 GB is P10, 128.5 GB is P15, 0.5 GB is P1, 32767 GB is
the largest tier). A size above the largest tier SHALL return `NotFound`. The tier meter
`{tier} {LRS|ZRS} Disk` `retailPrice` SHALL be returned as `cost_monthly` without multiplying by
730, with category Standard.

Tests: `TestEstimateCost_Disk_Success`, `TestEstimateCost_Disk_ValidationErrors`,
`TestEstimateCost_Disk_AllTypes`, `TestEstimateCost_Disk_UnsupportedTypes`,
`TestEstimateCost_Disk_SizeScaling`, `TestEstimateCost_Disk_SizeEdgeCases`, `TestTierForSize`

#### Scenario: Ceiling tier

- **WHEN** `EstimateCost` receives `azure:storage/managedDisk:ManagedDisk` with
  `disk_type=Premium_SSD_LRS` and `size_gb=100`
- **THEN** `cost_monthly` is the `P10 LRS Disk` price (19.71)

#### Scenario: Too large

- **WHEN** `size_gb` is 99999
- **THEN** the status is `NotFound`
