# FOCUS Column Mapping

`GetActualCost` attaches a FOCUS 1.3 `FocusCostRecord` to each result when a
billing account id is available (request `billing_account_id`, or
`FINFOCUS_BILLING_ACCOUNT_ID`). The record is built in
`internal/pricing/focus.go` and passes
`pluginsdk.ValidateFocusRecordWithOptions`.

Projected and estimate responses are not FOCUS rows. They carry the FOCUS
`pricing_category` enum only.

## Columns set

| FOCUS column | Value | Source |
| --- | --- | --- |
| BillingAccountId | request id, else `FINFOCUS_BILLING_ACCOUNT_ID` | caller |
| BillingAccountName | same as BillingAccountId | no display name is known |
| BillingCurrency | quote currency | Retail API `currencyCode` |
| BillingPeriodStart, BillingPeriodEnd | request window | request `start`, `end` |
| ChargePeriodStart, ChargePeriodEnd | request window | request `start`, `end` |
| ChargeCategory | `Usage` | fixed |
| ChargeClass | `Regular` | fixed |
| ChargeFrequency | `Usage-Based` | fixed |
| ChargeDescription | quote billing detail, else resource type | plugin |
| PricingCategory | `Dynamic` for Spot, else `Standard` | descriptor priority |
| ServiceProviderName | `Microsoft` | finops-toolkit Services.csv |
| HostProviderName | `Microsoft` | equals ServiceProviderName |
| ProviderName (deprecated) | `Microsoft` | still mandatory in FOCUS 1.3 |
| PublisherName (deprecated) | `Microsoft` | still mandatory in FOCUS 1.3 |
| InvoiceIssuerName | `Microsoft` | correct for direct EA and MCA customers; a CSP partner is the issuer for CSP customers |
| ServiceName | Azure service name, per resource type | Services.csv `ServiceName` |
| ServiceCategory, ServiceSubcategory | see the table below | Services.csv |
| ListCost, BilledCost, EffectiveCost | monthly quote × hours / 730 | Retail API `retailPrice` |
| ContractedCost | same as ListCost | no negotiated discounts |
| PricingUnit, ConsumedUnit | `Hours`, `Units/Month`, or `Months` | PricingUnits.csv; see the pricing basis below |
| PricingQuantity, ConsumedQuantity | basis quantity | see the pricing basis below |
| ListUnitPrice, ContractedUnitPrice | meter retail price, or unset | Retail API `retailPrice`; unset when several meters priced the resource |
| SkuId | quoted SKU | descriptor or Pulumi property |
| RegionId, RegionName | ARM region name | descriptor region |
| ResourceId, ResourceType | request resource id, descriptor type | caller |

## Pricing basis

A quote billed by exactly one meter with a positive price, whose monthly total
is that price times its count, is expressed in the meter's unit. The units
follow microsoft/finops-toolkit `src/open-data/PricingUnits.csv` (read
2026-10-02):

| Retail `unitOfMeasure` | PricingUnit | Quantity |
| --- | --- | --- |
| `1 Hour`, `1 Hours` (any case) | `Hours` | window hours × instance count |
| `1/Month` | `Units/Month` | window hours / 730 × count |
| `1 Month` | `Months` | window hours / 730 × count |

The quantity is counted, not divided back out of the cost, so a 24-hour window
is exactly 24 and a scale set of three is exactly 72. The unit price is the
meter's retail price, and ListUnitPrice × PricingQuantity equals ListCost to
within float noise.

Every other quote uses window hours with ListUnitPrice and ContractedUnitPrice
unset (0). That includes several meters (AKS with node pools, SQL Database,
Functions, Cosmos DB, Load Balancer with overage or data), GB-month storage
bands, request-unit meters, and zero prices. A blended hourly rate is not a
published price, so the record does not claim one. `ValidateFocusRecord`
compares ContractedCost with unit price × quantity only when both are non-zero,
and still requires ConsumedQuantity > 0.

`ActualCostResult.usage_amount` stays the window hours for every resource,
including scale sets. The FOCUS ConsumedQuantity counts instance-hours, so a
scale set of three over 24 hours reports 24 there and 72 in the record.

## Service name, category, and subcategory

The values follow microsoft/finops-toolkit `src/open-data/Services.csv` (read
2026-10-02), keyed by Azure resource type, so rows line up with Azure's own
FOCUS exports. The Retail API `serviceName` (for example `Storage` for a disk)
is not the FOCUS ServiceName. A VM `service` tag changes which retail rows are
queried, not the FOCUS service.

| Resource type | ServiceName | ServiceCategory | ServiceSubcategory |
| --- | --- | --- | --- |
| Virtual machine | Virtual Machines | Compute | Virtual Machines |
| Scale set | Virtual Machine Scale Sets | Compute | Virtual Machines |
| Managed disk | Virtual Machines | Compute | Virtual Machines |
| Storage account, Blob | Storage Accounts | Storage | Storage Platforms |
| App Service plan | Azure App Service | Other (see below) | Other (Other) |
| Function App on a plan SKU | Azure App Service | Other (see below) | Other (Other) |
| Function App (Consumption, Premium) | Functions | Compute | Serverless Compute |
| AKS | Azure Kubernetes Service | Compute | Containers |
| SQL Database | Azure SQL Database | Databases | Relational Databases |
| Cosmos DB | Cosmos DB | Databases | NoSQL Databases |
| Load Balancer | Load Balancer | Networking | Application Networking |

Microsoft maps App Service (`microsoft.web/serverfarms` and `sites`) to `Web` /
`Application Platforms`. `FocusServiceCategory` has no `Web` value. The
finfocus-spec rule for a category missing from the enum
(`specs/009-focus-1-2-integration`) is to send `Other` and keep the raw value
in extended columns. The plugin therefore sends:

| Column | Value |
| --- | --- |
| ServiceCategory | `FOCUS_SERVICE_CATEGORY_OTHER` |
| ServiceSubcategory | `Other (Other)`, the only FOCUS 1.3 child of `Other` |
| `extended_columns["x_ServiceCategory"]` | `Web` |
| `extended_columns["x_ServiceSubcategory"]` | `Application Platforms` |

FOCUS 1.3 requires each subcategory to have one parent category, so
`Compute` / `Other (Compute)` would misstate the service, and
`Other` / `Application Platforms` would break the parent rule. The `x_` prefix
is the FOCUS custom-column convention. finfocus core does not read the
service category today.

Until rshade/finfocus-spec#612 adds the missing values, `x_ServiceCategory`
and `x_ServiceSubcategory` are the authoritative FOCUS values for these rows.
A consumer that flattens extended columns sees `ServiceCategory=Other` next to
`x_ServiceCategory=Web`, and should trust the `x_` column. When a spec release
includes `Web`, the plugin will send it directly and keep both `x_` columns
for one more release. Removing them after that is a visible output change for
anyone who reads them.

## Expected SDK warnings

FOCUS 1.3 deprecates ProviderName and PublisherName but still lists them as
mandatory, so the record sets both. The SDK builder then logs two warnings,
`provider_name is deprecated, using service_provider_name` and the matching
publisher warning. Each is guarded by a `sync.Once`, so each prints at most
once per process. They go through the global `zerolog` logger in the SDK to `stderr`, so
`FINFOCUS_LOG_LEVEL` does not filter them.

## Columns left empty

| FOCUS column | Reason |
| --- | --- |
| RegionName as a display name | The quote does not carry the price row location. The column holds the ARM region name. |
| SkuPriceId, SkuMeter | The quote does not carry `meterId` or `meterName` |
| InvoiceId, InvoiceDetailId | No invoice exists for a list-price projection |
| Commitment discount columns | Quotes are on-demand or Spot list prices |
| Capacity reservation columns | Not applicable to list prices |
| Allocation columns | No split cost allocation |
| SubAccountId, SubAccountName | No subscription is known |
| AvailabilityZone | Not part of the descriptor |
| Tags | Descriptor tags are flattened Pulumi inputs, not resource tags (finfocus-spec#609) |
