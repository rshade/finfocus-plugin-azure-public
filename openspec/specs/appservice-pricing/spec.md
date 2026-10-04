# appservice-pricing Specification

## Purpose

Price Azure App Service plans and Function Apps from the Retail Prices API: plan SKUs per worker
hour, Functions Consumption usage after the free grant, and Functions Premium vCPU and memory.

## Requirements

### Requirement: App Service and Functions mapping

`web/AppServicePlan` and `azure:appservice/plan:Plan` SHALL map to service `Azure App Service`,
and `web/FunctionApp` (case-insensitive) and `azure:appservice/functionApp:FunctionApp` SHALL map
to service `Functions`. The query SHALL leave `ArmSkuName` and `ProductName` empty, so the price
filter carries region, service, `priceType eq 'Consumption'`, and currency but no `armSkuName`.
A plan with no SKU SHALL be `ErrMissingRequiredFields`; a Function App with no SKU is valid. A
type that only starts with the plan type SHALL be `ErrUnsupportedResourceType`. A Function App
whose SKU is a plan SKU SHALL query `Azure App Service`.

Tests: `TestMapDescriptorToQueryAppServiceAndFunctions`, `TestAppServiceQueryOmitsArmSKU`,
`TestFunctionsQueryOmitsArmSKU`

#### Scenario: Plan filter

- **WHEN** a `web/AppServicePlan` with SKU `P1v3` is priced
- **THEN** the filter contains `serviceName eq 'Azure App Service'` and does not contain
  `armSkuName` or `P1v3`

#### Scenario: Dedicated Function App

- **WHEN** a `web/FunctionApp` with SKU `P1v3` is priced
- **THEN** the filter contains `serviceName eq 'Azure App Service'`

### Requirement: Plan SKU matching and OS product

The plan SKU SHALL be matched against the price row SKU with spaces removed and case ignored. The
meter SHALL be the SKU or the SKU followed by a space and `App`, unit `1 Hour`.
Linux SHALL be the default, from a product whose name contains `Linux`; tag `os=Windows`
(any case) SHALL select a product
whose name does not contain `Linux`. The monthly cost SHALL be `retailPrice * 730` in the single
breakdown key `compute`, with `unit_price` equal to the retail price. A zero-priced row such as
`F1` SHALL price at 0.

Tests: `TestGetProjectedCostAppServicePlanFromFixture`, `TestGetProjectedCostAppServiceF1IsZeroRow`,
`TestGetProjectedCostAppServiceOverGRPC`

#### Scenario: Spaced and lower-case SKU

- **WHEN** SKU `P1 v3` or `p1v3` is priced with no `os` tag
- **THEN** the Linux `P1v3` row is used and `cost_per_month` is its retail price times 730

#### Scenario: Windows

- **WHEN** SKU `b1` has tag `os=WINDOWS`
- **THEN** the Windows `B1` row (meter `B1 App`) is used

#### Scenario: Free plan

- **WHEN** SKU `F1` is priced
- **THEN** `cost_per_month`, `unit_price`, and `compute` are 0

### Requirement: Unknown SKU, os, and ambiguous price

An unknown plan SKU, the `ASIP` SKU, an `os` tag other than `Windows` (including `Linux`), and a
SKU matching rows with different prices SHALL each return `InvalidArgument` naming the value.

Tests: `TestGetProjectedCostAppServiceUnknownSKU`, `TestGetProjectedCostAppServiceAmbiguousPrice`

#### Scenario: Explicit Linux tag

- **WHEN** SKU `P1v3` has tag `os=Linux`
- **THEN** the call fails with `InvalidArgument` naming `Linux`

#### Scenario: Two prices for one SKU

- **WHEN** the price page has two Linux `P1v3` rows with different prices
- **THEN** the call fails with `InvalidArgument` naming `P1v3`

### Requirement: Real Pulumi plan properties and worker count

The plan SKU SHALL be the descriptor `Sku`, then classic `skuName` or native `sku.name`. Classic
`osType` SHALL select the OS when `os` is empty, with `os` winning, and `WindowsContainer` SHALL
be `InvalidArgument`. The monthly cost SHALL be multiplied by `workerCount`, or by `sku.capacity`
when `workerCount` is absent. With neither, or with the Pulumi unknown placeholder, one worker
SHALL be priced with a note that the count was missing or unknown. A non-numeric, zero, or
negative count SHALL be `InvalidArgument`.

Tests: `TestAppServicePlanSKU_RealPulumiProperties_ResolvesShortSKU`,
`TestAppServicePlanWorkers_RealPulumiProperties_ReadsCount`,
`TestDescriptorWindows_ClassicOSType_SelectsProduct`,
`TestGetProjectedCost_ClassicServicePlan_PricesWorkersOnOSProduct`

#### Scenario: Two Linux workers

- **WHEN** `azure:appservice/servicePlan:ServicePlan` has `skuName=P1v3`, `osType=Linux`, and
  `workerCount=2`
- **THEN** `cost_per_month` is the Linux `P1v3` hourly price times 730 times 2
- **AND** `billing_detail` contains `2 workers`

#### Scenario: Windows with no count

- **WHEN** `skuName=S1` and `osType=Windows` are set without a worker count
- **THEN** one worker is priced on the Windows product and `billing_detail` carries the
  missing-count note

### Requirement: Functions Consumption

A Function App with SKU `Standard`, `Y1`, `Dynamic`, `Consumption`, an empty SKU, or
`pricing_model=consumption` SHALL be priced from `Standard Total Executions` (unit `10`) and
`Standard Execution Time` (unit `1 GB Second`), ignoring zero-priced sibling rows. The free grant
of 1,000,000 executions and 400,000 GB-seconds SHALL be subtracted first, billable executions
SHALL be divided by 10, and the result SHALL NOT be multiplied by 730. Components SHALL be
`executions` and `gb_seconds`, with no `compute` key. A missing positive execution row SHALL be
`NotFound`, and an execution row whose unit is not `10` SHALL be `InvalidArgument`.

Tests: `TestGetProjectedCostFunctionsConsumptionOverage`,
`TestGetProjectedCostFunctionsConsumptionInsideGrant`, `TestGetProjectedCostFunctionsOverGRPC`,
`TestGetProjectedCostFunctionsMissingNonZeroExecutionIsNotFound`,
`TestGetProjectedCostFunctionsRejectsExecutionUnit`

#### Scenario: Just over the grant

- **WHEN** `executions=1000010` and `gb_seconds=400001`
- **THEN** `executions` is (10 / 10) times the execution price and `gb_seconds` is 1 times the
  GB-second price

#### Scenario: Inside the grant

- **WHEN** usage is at or below both grants
- **THEN** both components are 0

### Requirement: Functions Premium and refused plans

SKU `Premium` or `pricing_model=premium` (any case) SHALL price `Premium vCPU Duration`
(unit `1 Hour`) times `vcpu_count` times 730 as `vcpu` and `Premium Memory Duration`
(unit `1 GiB Hour`) times `memory_gib` times 730 as `memory`. A Function App SKU that is a plan
SKU SHALL use the plan quote with key `compute`. SKU `EP1` and `pricing_model=flex` SHALL be
`InvalidArgument`, and a missing `executions`, `gb_seconds`, `vcpu_count`, or `memory_gib` for
its model SHALL be `InvalidArgument` naming that tag.

Tests: `TestGetProjectedCostFunctionsPremiumComponents`,
`TestGetProjectedCostFunctionDedicatedUsesPlan`, `TestGetProjectedCostFunctionsRejectsUnknown`

#### Scenario: Premium two vCPU

- **WHEN** `pricing_model=premium`, `vcpu_count=2`, and `memory_gib=4`
- **THEN** `vcpu` is 2 times the vCPU price times 730 and `memory` is 4 times the memory price
  times 730
- **AND** `cost_per_month` is their sum

#### Scenario: EP1

- **WHEN** a Function App has SKU `EP1`
- **THEN** the call fails with `InvalidArgument` naming `EP1`
