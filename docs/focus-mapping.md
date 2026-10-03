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
| InvoiceIssuerName | `Microsoft` | direct-customer default |
| ServiceName | Azure service name | Retail API `serviceName` |
| ServiceCategory, ServiceSubcategory | see the table below | Services.csv |
| ListCost, BilledCost, EffectiveCost | monthly quote × hours / 730 | Retail API `retailPrice` |
| ContractedCost | same as ListCost | no negotiated discounts |
| PricingUnit, ConsumedUnit | `Hours` or `Months` | see the pricing basis below |
| PricingQuantity, ConsumedQuantity | basis quantity | see the pricing basis below |
| ListUnitPrice, ContractedUnitPrice | basis unit price | Retail API `retailPrice` |
| SkuId | quoted SKU | descriptor or Pulumi property |
| RegionId, RegionName | ARM region name | descriptor region |
| ResourceId, ResourceType | request resource id, descriptor type | caller |

## Pricing basis

When exactly one meter with a positive price priced the resource and its unit
is `1 Hour` or `1/Month`:

- The unit is `Hours` or `Months`.
- The unit price is that meter's retail price.
- The quantity is the cost divided by that price. A scale set of three VMs
  bills three instance-hours per hour, and a disk bills a fraction of a month.

Every other quote uses window hours and an average hourly unit price. That
includes several meters (AKS, SQL Database, Functions, Cosmos DB, Load
Balancer with overage), GB-month storage bands, request-unit meters, and zero
prices. In both cases unit price times quantity equals the cost.

## Service category and subcategory

The values follow microsoft/finops-toolkit `src/open-data/Services.csv`.

| Resource type | ServiceCategory | ServiceSubcategory |
| --- | --- | --- |
| Virtual machine, scale set | Compute | Virtual Machines |
| Managed disk | Compute | Virtual Machines |
| Storage account, Blob | Storage | Storage Platforms |
| App Service plan | Compute | Other (Compute) |
| Function App | Compute | Serverless Compute |
| AKS | Compute | Containers |
| SQL Database | Databases | Relational Databases |
| Cosmos DB | Databases | NoSQL Databases |
| Load Balancer | Networking | Application Networking |

Microsoft maps App Service plans to `Web` / `Application Platforms`.
`FocusServiceCategory` has no `Web` value yet (rshade/finfocus-spec#612), so
the plugin uses the valid pair `Compute` / `Other (Compute)` until then.

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
