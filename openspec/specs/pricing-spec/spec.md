# pricing-spec Specification

## Purpose

Answer `GetPricingSpec` with one `PricingSpec` for the requested resource, built from the same
quote as `GetProjectedCost`, with a billing mode and unit that FinFocus core either reads
correctly or skips.

## Requirements

### Requirement: One spec per resource from the projected quote

For every supported resource type, `GetPricingSpec` SHALL return one spec that passes
`ValidatePricingSpec`, with provider `azure`, the descriptor's resource type and region, currency
`USD`, source `azure-retail-prices`, and a non-empty SKU. `rate_per_unit` SHALL be the selected
meter's retail price, not a monthly total (not the price times 730) and not another meter's price.

Tests: `TestGetPricingSpecEverySupportedType`, `TestGetPricingSpecPremiumFunctions`

#### Scenario: Virtual machine rate

- **WHEN** `GetPricingSpec` is called for `compute/VirtualMachine` with SKU `Standard_B1s` in
  `eastus` and the Linux meter retail price is 0.0104
- **THEN** `rate_per_unit` is 0.0104 and `billing_mode` is `per_hour` with unit `hour`

### Requirement: Billing mode and unit per rate

The spec SHALL use these billing modes and units:

- Virtual machine, App Service plan, AKS control plane, SQL compute, and the Load Balancer
  included-rules meter: `per_hour`, unit `hour`.
- Managed disk: `per_month`, unit `Month`.
- Blob storage and storage account: `per_gb_month`, unit `GB-month`.
- Functions Consumption: `per_second`, unit `GB-second`, rate from `Standard Execution Time`.
- Functions Premium: `per_vcpu_hour`, unit `vCPU-hour`, rate from `Premium vCPU Duration`.
- Cosmos DB provisioned: `per_ru`, unit `100 RU/s per hour`; serverless: `per_ru`, unit `1M RU`.
- Load Balancer with `rule_count=0` and `data_processed_gb`: `per_data_transfer_gb`, unit
  `GB processed`, rate from the data processed meter.
- A meter whose unit is not recognised: `not_implemented`.

Tests: `TestGetPricingSpecEverySupportedType`, `TestGetPricingSpecPremiumFunctions`,
`TestGetPricingSpec_UsageNotSupplied_ReturnsUnitRate`,
`TestGetPricingSpec_LoadBalancerNoRulesWithData_ReportsProcessedDataRate`,
`TestSpecRate_EveryBranch_ReturnsListedBillingMode`

#### Scenario: Load Balancer with no rules and processed data

- **WHEN** a Standard Load Balancer has `rule_count=0` and `data_processed_gb=100`
- **THEN** `GetProjectedCost` is 100 times the data processed price with only the data component
- **AND** the spec is `per_data_transfer_gb` with unit `GB processed` at the data processed price
- **AND** no assumption mentions the included rules meter

### Requirement: Modes core cannot read never use units core reads

Every spec's `billing_mode` SHALL be a valid SDK billing mode. When core cannot turn the mode into
a monthly total, the unit SHALL NOT be one of core's fallback units (for example `hour`, `gb`,
`month`, `gb-month`), so core skips the spec instead of mispricing it. `assumptions` SHALL NOT be
empty, and a `per_hour` spec's assumptions SHALL state `730 hours`.

Tests: `TestGetPricingSpec_EverySupportedType_ModeIsValidAndReadableOrSkipped`,
`TestGetPricingSpec_LoadBalancerNoRulesWithData_ReportsProcessedDataRate`

#### Scenario: Hourly spec assumptions

- **WHEN** a spec has mode `per_hour`
- **THEN** one assumption contains `730 hours`

### Requirement: Metric hints name meters and usage inputs

`metric_hints` SHALL list each selected meter by its breakdown key with the meter's unit of
measure, then the usage inputs the quote reads, each once: `size_gb` (`GB`) for disks, storage,
SQL, and Cosmos; `workerCount` (`count`) for App Service plans; `rule_count` (`count`) and
`data_processed_gb` (`GB`) for Load Balancer; `ru_per_second` (`RU/s`) for Cosmos;
`vcpu_count` (`vCPU`) and `memory_gib` (`GiB`) for Premium Functions; and `node_pool_N_count`
(`count`) for AKS.

Tests: `TestGetPricingSpecEverySupportedType`, `TestGetPricingSpecPremiumFunctions`

#### Scenario: AKS hints

- **WHEN** `GetPricingSpec` is called for an AKS cluster with two node pools
- **THEN** the hints are `control_plane`, `node_pool_pool_1`, `node_pool_pool_2` with the meter
  units, and `node_pool_N_count` with unit `count`

### Requirement: Missing rate-neutral usage returns the unit rate

When the only missing inputs are usage values that do not change the rate and the resulting mode
is one core does not multiply, `GetPricingSpec` SHALL return the unit rate with an assumption
naming each missing input and a metric hint for it. This SHALL apply to Cosmos `ru_per_second`
and `request_units` and to the Functions usage inputs (`executions`, `gb_seconds`, `vcpu_count`,
`memory_gib`). `GetProjectedCost` SHALL still return `InvalidArgument` for the same descriptor.

Tests: `TestGetPricingSpec_UsageNotSupplied_ReturnsUnitRate`,
`TestGetProjectedCost_UsageNotSupplied_StillInvalidArgument`

#### Scenario: Cosmos without RU/s

- **WHEN** `GetPricingSpec` is called for `cosmosdb/Account` without `ru_per_second`
- **THEN** the spec is `per_ru` with unit `100 RU/s per hour` at the RU meter price
- **AND** an assumption names `ru_per_second`
- **AND** `GetProjectedCost` for the same descriptor fails with `InvalidArgument`

### Requirement: Missing identity or core-computed usage is InvalidArgument

`GetPricingSpec` SHALL return `InvalidArgument` naming the field when a region or SKU is
missing, when a managed disk has no `size_gb` (it picks the tier), and when `size_gb` is missing
for blob storage, storage accounts, and SQL Database, whose modes core would multiply.

Tests: `TestGetPricingSpecMissingFieldNamesTheField`,
`TestGetPricingSpec_IdentityMissing_ReturnsInvalidArgument`,
`TestGetPricingSpec_CoreComputedModeUsageMissing_ReturnsInvalidArgument`

#### Scenario: Storage account without size

- **WHEN** `GetPricingSpec` is called for `storage/StorageAccount` without `size_gb`
- **THEN** the call fails with `InvalidArgument` and the message names `size_gb`

### Requirement: Spot assumptions name Spot

For a virtual machine with `priority=Spot` or `virtualMachineProfile.priority=Spot`, the
assumptions SHALL name `Spot` and SHALL NOT claim an `on-demand` instance.

Tests: `TestGetPricingSpec_SpotVM_AssumptionNamesSpot`

#### Scenario: Spot priority tag

- **WHEN** `GetPricingSpec` is called for `Standard_D2s_v3` with `priority=Spot`
- **THEN** an assumption contains `Spot` and none contains `on-demand`

### Requirement: Unsupported type and missing client

`GetPricingSpec` SHALL return `Unimplemented` with a message naming the type for an unsupported
resource type, and `Unimplemented` naming `AZ-2.11` on a calculator without a price client.

Tests: `TestGetPricingSpecUnsupportedType`, `TestGetPricingSpecNilClientNamesTask`

#### Scenario: Unsupported type

- **WHEN** `GetPricingSpec` is called for `compute/VirtualMachineScaleSet`
- **THEN** the code is `Unimplemented` and the message names `compute/VirtualMachineScaleSet`
