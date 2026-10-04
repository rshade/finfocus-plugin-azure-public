# focus-record Specification

## Purpose

Attach a FOCUS 1.3 cost record to each `GetActualCost` result when a billing account id is
known, with Microsoft provider names, a pricing basis taken from the billed meter, and a service
classification that follows the finops-toolkit mapping.

## Requirements

### Requirement: Billing account id precedence

The record's `billing_account_id` SHALL be `GetActualCostRequest.billing_account_id` when it is
set, and otherwise the id set with `SetBillingAccountID`. A request id SHALL produce a record even
when no process id is set. When the request is a dry run, the request id SHALL be ignored and the
process id used.

Tests: `TestGetActualCostFocusRecordOverGRPC`, `TestGetActualCostRequestBillingAccountOverGRPC`,
`TestGetActualCostRequestBillingAccountWithoutProcessOverGRPC`,
`TestGetActualCostDryRunIgnoresRequestBillingAccount`

#### Scenario: Request id wins

- **WHEN** the process id is `ba-process` and the request id is `ba-request`
- **THEN** the record's billing account id is `ba-request`
- **AND** the record passes `pluginsdk.ValidateFocusRecord`

#### Scenario: Dry run uses the process id

- **WHEN** `dry_run` is true, the request id is `ba-request`, and the process id is `ba-process`
- **THEN** the record's billing account id is `ba-process`

### Requirement: No record without a billing account id

When neither the request nor the process supplies a billing account id, `GetActualCost` SHALL
still return the cost and confidence source, and SHALL leave `focus_record` nil. The record
builder SHALL refuse an empty id with an error naming `billing_account_id`, and SHALL refuse an
empty currency with an error naming `billing_currency` rather than substituting USD.

Tests: `TestGetActualCostOmitsFocusRecordWithoutAccountOverGRPC`, `TestFocusEmptyBillingAccountID`,
`TestFocusEmptyCurrency`

#### Scenario: No id configured

- **WHEN** `GetActualCost` is called with no request id and no process id
- **THEN** the cost is `0.0104 * 24` with source `azure-retail-prices[confidence:HIGH]`
- **AND** `focus_record` is nil

### Requirement: Provider names, costs, and periods

The record SHALL set `service_provider_name`, `host_provider_name`, `invoice_issuer`,
`provider_name`, and `publisher` to `Microsoft`, and `billing_account_name` to the account id.
Billed, effective, list, and contracted cost SHALL equal the actual cost, and the contracted unit
price SHALL equal the list unit price. Billing and charge periods SHALL be the request window.
Charge category SHALL be Usage, class Regular, frequency Usage-Based, and pricing category
Standard. `invoice_id` and commitment discount fields SHALL stay unset. The charge description
SHALL be the quote's billing detail, or the resource type when the detail is empty.

Tests: `TestBuildFocusRecord_FOCUS13Providers_SetsMandatoryNames`,
`TestFocusRecordMatchesActualCost`,
`TestBuildFocusRecord_CostColumns_AgreeWithUnitPrice`

#### Scenario: Virtual machine over 24 hours

- **WHEN** a record is built for a VM quote of `0.0104 * 730` per month over a 24-hour window
- **THEN** billed, effective, and list cost equal `0.0104 * 24`
- **AND** the five provider name columns are `Microsoft`

### Requirement: Pricing unit and quantity come from the billed meter

When a quote is billed by one positive meter whose monthly total is that price times its count,
the record SHALL use the meter unit: `1 Hour` and `1 Hours` map to `Hours` with quantity equal to
window hours times the count; `1/Month` maps to `Units/Month` and `1 Month` to `Months` with
quantity equal to window hours divided by 730. The list and contracted unit prices SHALL be the
meter price. Every other quote (several meters, a zero price, a total that is not the meter, a
`1 GB/Month` or `1/Hour` unit, or no meters) SHALL use window `Hours` with both unit prices unset.
Pricing and consumed quantity SHALL be equal and exact (24 hours is exactly 24).

Tests: `TestFocusPricingBasis_Quotes_ReturnMeterOrWindowBasis`,
`TestBuildFocusRecord_ProductionCostPath_QuantityIsExact`,
`TestGetActualCost_RealQuotes_SetFocusPricingBasis`,
`TestGetActualCost_VMSizeKeys_BuildsFocusRecord`,
`TestGetActualCost_ScaleSetCapacityInResource_PricesInstances`

#### Scenario: Scale set of three

- **WHEN** a classic `linuxVirtualMachineScaleSet` with `instances=3` is priced over 24 hours
- **THEN** the pricing unit is `Hours` and the pricing and consumed quantity are 72
- **AND** the list unit price is the hourly meter price

#### Scenario: Mixed-meter quote

- **WHEN** an AKS cluster with a node pool, a SQL database, or a Functions Consumption app is
  priced over 24 hours
- **THEN** the unit is `Hours`, the quantity is 24, and both unit prices are 0

### Requirement: Service name and category follow the Microsoft mapping

`ServiceName`, category, and subcategory SHALL follow finops-toolkit Services.csv by resource
type: virtual machines and managed disks are `Virtual Machines` / Compute / `Virtual Machines`;
scale sets are `Virtual Machine Scale Sets` / Compute / `Virtual Machines`; storage accounts and
blob storage are `Storage Accounts` / Storage / `Storage Platforms`; Function Apps on Consumption
are `Functions` / Compute / `Serverless Compute`; AKS is `Azure Kubernetes Service` / Compute /
`Containers`; SQL is `Azure SQL Database` / Database / `Relational Databases`; Cosmos DB is
`Cosmos DB` / Database / `NoSQL Databases`; Load Balancer is `Load Balancer` / Network /
`Application Networking`. A native `azure-native:web:WebApp` SHALL be a Function App only with
`kind=FunctionApp`; without it, and for an unknown type, the builder SHALL return an error naming
the type and SHALL NOT default to Other. Every supported type's record SHALL pass aggregate FOCUS
validation with a non-empty subcategory.

Tests: `TestFocusServiceClass_EverySupportedType_MatchesMicrosoftMapping`,
`TestFocusServiceCategory`,
`TestFocusNativeFunctionWebApp`, `TestFocusUnknownType`,
`TestBuildFocusRecord_EverySupportedType_PassesAggregateValidation`

#### Scenario: Managed disk

- **WHEN** a record is built for `storage/ManagedDisk`
- **THEN** the service is `Virtual Machines` in category Compute

#### Scenario: Unknown type

- **WHEN** a record is built for `compute/VirtualMachineScaleSet`
- **THEN** the builder returns an error naming that type

### Requirement: Web services are Other with FOCUS extended columns

App Service plans, and Function Apps whose SKU is an App Service plan SKU, SHALL be
`Azure App Service` with category Other and subcategory `Other (Other)`, and SHALL carry extended
columns `x_ServiceCategory=Web` and `x_ServiceSubcategory=Application Platforms`. Types inside the
category enum SHALL NOT carry those columns.

Tests: `TestBuildFocusRecord_CategoryOutsideEnum_CarriesFocusValuesInExtendedColumns`,
`TestFocusServiceClass_EverySupportedType_MatchesMicrosoftMapping`, `TestFocusServiceCategory`

#### Scenario: Function App on a plan SKU

- **WHEN** a record is built for `web/FunctionApp` with SKU `B1`
- **THEN** the category is Other with subcategory `Other (Other)`
- **AND** `x_ServiceCategory` is `Web` and `x_ServiceSubcategory` is `Application Platforms`

#### Scenario: Virtual machine has no extended category

- **WHEN** a record is built for `compute/VirtualMachine`
- **THEN** the category is Compute and both `x_` columns are empty

### Requirement: Spot is a dynamic pricing category

A virtual machine with tag `priority=Spot` SHALL have pricing category Dynamic, and the costs
SHALL equal those of the same quote at pricing category Standard.

Tests: `TestFocusSpotPricingCategory`

#### Scenario: Spot VM record

- **WHEN** a record is built for a VM descriptor tagged `priority=Spot`
- **THEN** the pricing category is Dynamic and billed cost is unchanged
