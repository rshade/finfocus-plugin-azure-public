# actual-cost Specification

## Purpose

Answer `GetActualCost` with a list-price projection: the projected monthly cost scaled to the
requested window, with a confidence level that says how much of the window the caller supplied.

## Requirements

### Requirement: Cost scales the monthly quote by the window

`GetActualCost` SHALL return one result whose cost is the projected monthly cost times
`hours / 730`, where `hours` is the length of the request window. The result SHALL report
`usage_amount` equal to the window hours and `usage_unit` `hours`. A monthly-priced resource
(managed disk, blob storage) SHALL scale the same way, so a 730-hour window costs exactly the
monthly quote.

Tests: `TestGetActualCostScalesByRuntimeAndConfidence`, `TestGetActualCostDiskAndBlob`,
`TestDescriptorLimits_CoreShapedTags_Price`

#### Scenario: One day of a B1s virtual machine

- **WHEN** `GetActualCost` is called with tags `region=eastus`, `sku=Standard_B1s`, a 24-hour
  window, and the meter retail price is 0.0104
- **THEN** the cost is `0.0104 * 24`
- **AND** `usage_amount` is 24 and `usage_unit` is `hours`

#### Scenario: Monthly disk price over a full month

- **WHEN** a `storage/ManagedDisk` with `sku=Premium_SSD_LRS` and `size_gb=128` is priced over a
  730-hour window and the P10 LRS meter is 19.71
- **THEN** the cost is 19.71

### Requirement: Confidence and default window

The result `source` SHALL be `azure-retail-prices[confidence:HIGH]` when both start and end are
set, `azure-retail-prices[confidence:MEDIUM]` when only start is set, and
`azure-retail-prices[confidence:LOW]` when neither is set. With no window the cost SHALL assume
730 hours (`pluginsdk.HoursPerMonth`).

Tests: `TestGetActualCostScalesByRuntimeAndConfidence`, `TestGetActualCostConfidenceLevels`

#### Scenario: No window

- **WHEN** `GetActualCost` is called with no start and no end and the hourly price is 0.02
- **THEN** the source is `azure-retail-prices[confidence:LOW]`
- **AND** the cost is `0.02 * 730`

#### Scenario: Start only

- **WHEN** only `start` is set
- **THEN** the source is `azure-retail-prices[confidence:MEDIUM]`

### Requirement: Inverted window is rejected

`GetActualCost` SHALL return `InvalidArgument` when the window end is before its start.

Tests: `TestGetActualCostRejectsInvertedWindowAndMissingSKU`

#### Scenario: End before start

- **WHEN** start is 2026-09-02 and end is 2026-09-01
- **THEN** the call fails with `InvalidArgument`

### Requirement: Request resource descriptor is priced when set

When `GetActualCostRequest.resource` is set, `GetActualCost` SHALL price that descriptor,
including its structured attributes (for example native `sku.capacity` and `diskSizeGB`) and its
own tags. The request's cloud tags SHALL then be labels only: a cloud tag named `region`, `sku`,
`provider`, `resource_type`, or `instances` SHALL NOT reach the Azure query or change the cost.
When `resource` is not set, the request tags are the pricing inputs.

Tests: `TestGetActualCost_ScaleSetCapacityInResource_PricesInstances`,
`TestGetActualCost_ResourceTagsWithoutAttributes_AreRead`,
`TestGetActualCost_DiskSizeInResource_PicksTier`,
`TestGetActualCost_ResourceSet_CloudTagsAreNotPricingInputs`

#### Scenario: Native scale set capacity in attributes

- **WHEN** `resource` is `azure-native:compute:VirtualMachineScaleSet` with attributes
  `sku.name=Standard_D2s_v5` and `sku.capacity=3` over a 730-hour window
- **THEN** the cost is `0.115 * 730 * 3`

#### Scenario: Disk size from attributes picks the tier

- **WHEN** `resource` is `azure-native:compute:Disk` with `sku.name=Premium_LRS` and
  `diskSizeGB=200`
- **THEN** the cost is the P15 price 38.01

#### Scenario: Cloud tags are labels

- **WHEN** `resource` is set and the cloud tags carry `region=prod-eu` and `sku=gold`
- **THEN** no Azure `$filter` contains `prod-eu` or `gold`
- **AND** the cost is the descriptor's VM price times the window hours

### Requirement: Invalid descriptors fail like projected cost

`GetActualCost` SHALL reject a descriptor with the same `InvalidArgument` code and message as
`GetProjectedCost`, without querying Azure, when a required field holds the Pulumi unknown
placeholder (`missing required field(s): region`) or the attributes exceed 65536 bytes.

Tests: `TestGetActualCost_ResourceUnknownOrOversize_MatchesProjected`

#### Scenario: Unknown location

- **WHEN** `resource` has `location` set to the Pulumi unknown placeholder
- **THEN** both RPCs fail with `InvalidArgument` and the same message naming `region`
- **AND** no price request is sent

### Requirement: SDK descriptor limits apply on every descriptor RPC

`GetProjectedCost`, `GetActualCost`, and `GetPricingSpec` SHALL return `InvalidArgument` for a
descriptor over the SDK limits (more than 256 tags, or a tag value longer than 2048 bytes).
`Supports` SHALL answer unsupported with a reason naming the limit, and `DryRun` SHALL report a
supported type with an invalid configuration. No Azure request SHALL be sent. A descriptor with
49 tags of 2048-byte values SHALL still price.

Tests: `TestDescriptorLimits_EachRPC_RejectOverLimitTags`,
`TestDescriptorLimits_CoreShapedTags_Price`

#### Scenario: 300 tags

- **WHEN** a descriptor carries 300 tags
- **THEN** each cost RPC fails with `InvalidArgument` containing `tag count 300 exceeds maximum 256`
- **AND** Azure is not queried

### Requirement: No price client names the task

`GetActualCost` on a calculator built without a price client SHALL return `Unimplemented` with a
message naming `AZ-2.2`.

Tests: `TestGetActualCostNilClientNamesTask`

#### Scenario: Calculator without a client

- **WHEN** `GetActualCost` is called on `NewCalculator` with no cached client
- **THEN** the code is `Unimplemented` and the message contains `AZ-2.2`
