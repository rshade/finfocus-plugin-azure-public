# vm-pricing Specification

## Purpose

Price Azure virtual machines and virtual machine scale sets from the Azure Retail Prices API:
pick the right hourly meter, turn it into a monthly cost, and report the Spot, Savings Plan,
Reservation, and other-region alternatives beside the selected price.

## Requirements

### Requirement: On-demand Linux meter priced at 730 hours

`GetProjectedCost` for a virtual machine SHALL select the Linux on-demand row (a product whose
name does not contain Windows, and whose SKU or meter name does not contain the whole word
`Spot`) and SHALL return `cost_per_month` equal to `retailPrice * 730`. The response SHALL put
that amount under the `compute` breakdown key, SHALL use pricing category
`FOCUS_PRICING_CATEGORY_STANDARD`, and SHALL carry a non-empty `billing_detail`. Classic
(`azure:compute/virtualMachine:VirtualMachine`) and native (`azure-native:compute:VirtualMachine`)
tokens SHALL produce the same quote. A row with an empty product name SHALL still be priceable.

Tests: `TestGetProjectedCostVMIncludesBreakdownAndCategory`, `TestAzureNativeVMQuoteMatchesAzure`,
`TestSelectVMItemSkipsWindowsAndEmbeddedSpot`, `TestSelectVMItemEmptyProductNamePricesItem`

#### Scenario: Standard_B1s in eastus

- **WHEN** `GetProjectedCost` is called for `Standard_B1s` in `eastus` and the row costs 0.0104
- **THEN** `cost_per_month` is `0.0104 * 730`
- **AND** `cost_breakdown["compute"]` equals `cost_per_month` and the category is Standard

#### Scenario: Spot word inside another word

- **WHEN** the candidate rows are a `Spotlight` row and a real `Spot` row and Spot is requested
- **THEN** the real `Spot` row is selected

### Requirement: Priority selects on-demand or Spot

The VM quote SHALL read tag `priority` case-insensitively. An empty value or `Regular` SHALL
price the on-demand row with category Standard and a `billing_detail` that says `On-demand`.
`Spot` SHALL price the Linux Spot row with category `FOCUS_PRICING_CATEGORY_DYNAMIC` and a
`billing_detail` that says `Spot`. Any other value (`Low`, `LowPriority`) SHALL return
`InvalidArgument`. A Spot request with no Spot row SHALL return `NotFound`. `EstimateCost` SHALL
apply the same rule to attribute `priority`.

Tests: `TestGetProjectedCostSpotVMFromFixture`, `TestPriorityRegularIsOnDemand`,
`TestPriorityLowStaysInvalid`, `TestGetProjectedCostSpotMissingIsNotFound`,
`TestGetProjectedCostSpotOverGRPC`, `TestEstimateCostSpotD2sV3Eastus`

#### Scenario: Spot priority

- **WHEN** `GetProjectedCost` is called for `Standard_D2s_v3` in `eastus` with `priority=spot`
- **THEN** `cost_per_month` is the Spot row's `retailPrice * 730`
- **AND** the pricing category is Dynamic

#### Scenario: Low priority is rejected

- **WHEN** `priority` is `LowPriority`
- **THEN** the call fails with `InvalidArgument` and the message names `LowPriority`

#### Scenario: Spot meter missing

- **WHEN** `priority=Spot` and the price page has no Spot row
- **THEN** the call fails with `NotFound`

### Requirement: pricing_model is a Spot alias when priority is empty

When `priority` is empty, tag `pricing_model=spot` SHALL select the Spot row and
`pricing_model=consumption` SHALL stay on demand. A non-empty `priority` SHALL win over
`pricing_model`. Any other `pricing_model` value SHALL return `InvalidArgument` naming the value.

Tests: `TestPricingModelSpotAlias`

#### Scenario: priority wins

- **WHEN** the tags are `priority=Spot` and `pricing_model=consumption`
- **THEN** the Spot row is priced

#### Scenario: Unknown pricing model

- **WHEN** the tag is `pricing_model=reserved`
- **THEN** the call fails with `InvalidArgument` and the message names `reserved`

### Requirement: Size sources for real Pulumi inputs

When the descriptor `Sku` is empty, the VM size SHALL be read from tag `size`, from
`hardwareProfile.vmSize`, or from a bare `hardwareProfile` tag that holds the collapsed size
string. The real Pulumi VM and scale-set cases that the ratchet in `TestRealPulumiPlan` marks
as must-pass (for example `azure/linuxVm`, `azure/windowsVm`, `azure/vmss`, `azure/legacyVm`,
`azure-native/linuxVm`, `azure-native/windowsVm`) SHALL match `plan-expected.json` for the core,
dotted, and attributes input forms.

Tests: `TestClassicVMReadsSizeTag`, `TestNativeVMReadsHardwareProfile`,
`TestDottedHardwareProfileVMSize`, `TestRealPulumiPlan`

#### Scenario: Collapsed hardwareProfile

- **WHEN** a native `azure-native:compute:VirtualMachine` has no `Sku` and tag
  `hardwareProfile=Standard_B2s`
- **THEN** the quote is the `Standard_B2s` row times 730

### Requirement: Windows guests and Hybrid Benefit

A Windows guest (for example the classic `windowsVirtualMachine` token, or native
`osProfile.windowsConfiguration`) with no Hybrid Benefit `licenseType` SHALL be priced from the
product whose name contains Windows and SHALL NOT fall back to the Linux row; with only a Linux
row available it SHALL return `NotFound`. `licenseType=Windows_Server` SHALL price the Linux
(base) rate and SHALL add a `billing_detail` note that the licence is already paid.

Tests: `TestWindowsVMQuoteIsNotLinuxPrice`, `TestWindowsWithoutLicenseDoesNotUseLinuxMeter`,
`TestHybridBenefitUsesLinuxRateAndNote`

#### Scenario: Windows token

- **WHEN** a `azure:compute/windowsVirtualMachine:WindowsVirtualMachine` is quoted with Linux row
  0.096 and Windows row 0.188
- **THEN** `cost_per_month` is `0.188 * 730`

#### Scenario: Hybrid Benefit

- **WHEN** a native Windows VM has `licenseType=Windows_Server`
- **THEN** `cost_per_month` is the Linux rate times 730
- **AND** `billing_detail` contains the Azure Hybrid Benefit note

### Requirement: Scale sets multiply by instance count

A scale set SHALL be priced as meter times 730 times the instance count, read from tag
`instances` or native `sku.capacity`. For a native scale set, `virtualMachineProfile.priority`
of `Spot` SHALL select the Spot row when `priority` is empty.

Tests: `TestScaleSetMultipliesInstances`, `TestScaleSetNestedPrioritySelectsSpot`

#### Scenario: Three instances

- **WHEN** a `linuxVirtualMachineScaleSet` with `instances=3` uses a 0.115 row
- **THEN** `cost_per_month` is `0.115 * 730 * 3`

#### Scenario: Nested Spot priority

- **WHEN** a native scale set has `sku.capacity=3` and `virtualMachineProfile.priority=Spot`
- **THEN** `cost_per_month` is the Spot rate times 730 times 3

### Requirement: Price options list alternatives without changing the selected cost

A VM quote SHALL add `price_options` for Consumption, Spot, Savings Plan (`1 Year`,
`3 Years`), and Reservation rows, ignoring Windows and Low Priority rows. Each option's
`monthly_cost` SHALL be its unit price times 730. A Reservation `retailPrice` SHALL be treated as
the term total: the unit price is that total divided by 8760 per year of term, and the total is
the `upfront_cost`. The selected `cost_per_month` and `compute` breakdown SHALL NOT include the
options. `savings_fraction` SHALL compare unit prices for `GetProjectedCost`, monthly costs for
`EstimateCost`, and SHALL be 0 when the selected price is 0. A failed advisory query (preview,
Reservation, or regionless page) SHALL be omitted, and the quote SHALL still succeed.

Tests: `TestGetProjectedCostVMAlternativesFromFixtures`,
`TestGetProjectedCostVMReservationOptionsFromFixtures`,
`TestPriceOptionProtosUsesTheRequestedBasis`,
`TestGetProjectedCostVMZeroPrimarySavingsFraction`, `TestVMQuoteIgnoresAdvisoryFetchErrors`

#### Scenario: Standard_D2s_v3 alternatives

- **WHEN** `Standard_D2s_v3` in `eastus` is quoted at 0.096 per hour
- **THEN** `cost_per_month` is `0.096 * 730`
- **AND** the Spot option is 0.018816 with savings fraction `(0.096 - 0.018816) / 0.096`
- **AND** Savings Plan options exist for `1 Year` (0.06624) and `3 Years` (0.04512)

#### Scenario: Reservation term total

- **WHEN** the Reservation page has a `1 Year` row of 416 and a `3 Years` row of 803
- **THEN** the options have unit prices `416 / 8760` and `803 / 26280` and upfront costs 416 and 803

#### Scenario: Advisory query fails

- **WHEN** the Reservation, preview, and regionless queries return HTTP 500
- **THEN** the quote succeeds with only the Consumption option and no region prices

### Requirement: Region prices from the regionless page

A VM quote SHALL read the regionless Consumption page and return `region_prices` for the other
regions, each with the Linux on-demand unit price, `monthly_cost` of unit price times 730, and
currency. The requested region SHALL stay the parent price and SHALL NOT be listed, and a region
with no selected row SHALL be omitted. `SortRegionPrices` SHALL order found regions by ascending
price, then by region name, and SHALL place a region with no selected row last with `Found` false
and price 0.

Tests: `TestGetProjectedCostVMRegionPricesFromFixtures`, `TestSortRegionPrices`

#### Scenario: Other regions

- **WHEN** `Standard_B1s` is quoted in `eastus` and the regionless page holds eastus, westus2,
  northeurope, and a Windows-only switzerlandnorth row
- **THEN** `region_prices` is westus2 at 0.0104 then northeurope at 0.0113

#### Scenario: Equal prices

- **WHEN** westus2 and eastus have the same price
- **THEN** `SortRegionPrices` returns eastus before westus2
