# Research: FOCUS 1.3 Cost Record Alignment

Sources read on 2026-10-02.

## R1: Does finfocus-spec already define FOCUS 1.3 columns?

**Decision**: Yes. Use the existing `FocusCostRecord`.

**Evidence**:

- finfocus-spec v0.7.1 `proto/finfocus/v1/focus.proto` has
  `service_provider_name` (59), `host_provider_name` (60), `contracted_cost`
  (41), `contracted_unit_price` (50), `service_subcategory` (56), the
  allocation columns, and the FOCUS 1.4 columns.
- finfocus-spec #183 (migrate to FOCUS 1.3) and #540–#542 (FOCUS 1.4) are
  closed.

**Alternative rejected**: the issue's own `FOCUSCostRecord` proto sketch. It
duplicates the existing message.

## R2: Which FOCUS 1.3 columns are wrong or missing today?

`internal/pricing/focus.go` on `a28c4ea` has these gaps:

- It sets only the deprecated `provider_name`, through `WithIdentity`.
- It never sets `contracted_cost`, which is mandatory in FOCUS 1.3
  (`contractedcost.md`, "MUST be present").
- `pricing_unit` and `consumed_unit` are `hour`. The FOCUS Unit Format lists
  the time unit as `Hour` and wants plural units (`Hours`).
- The unit is always hours, even for monthly meters such as disks.
- `service_subcategory` is not set. It is recommended
  (`servicesubcategory.md`).

## R3: Provider name value

**Decision**: `Microsoft` for both service and host provider.

**Evidence**:

- microsoft/finops-toolkit `src/open-data/Services.csv` gives `PublisherName`
  `Microsoft` for every supported resource type.
- FOCUS 1.3 `hostprovidername.md` says HostProviderName MUST equal
  ServiceProviderName when the service provider does not expose a separate
  host.

## R4: Service category and subcategory mapping

**Decision**: Follow `Services.csv`.

| ResourceType | ServiceCategory | ServiceSubcategory |
| --- | --- | --- |
| microsoft.compute/virtualmachines | Compute | Virtual Machines |
| microsoft.compute/disks | Compute | Virtual Machines |
| microsoft.storage/storageaccounts | Storage | Storage Platforms |
| microsoft.web/serverfarms | Web | Application Platforms |
| functions (microsoft.web/sites) | Compute | Serverless Compute |
| microsoft.containerservice/managedclusters | Compute | Containers |
| microsoft.sql/servers/databases | Databases | Relational Databases |
| microsoft.documentdb/databaseaccounts | Databases | NoSQL Databases |
| microsoft.network/loadbalancers | Networking | Application Networking |

**Gap**: The `Web` category is missing from `FocusServiceCategory`, which has
10 values plus `UNSPECIFIED` against 19 in FOCUS 1.3, so nine are missing.
The finfocus-spec rule for this case (`specs/009-focus-1-2-integration`
spec.md, edge cases) is to send `Other` and keep the raw value in
`extended_columns`. FOCUS 1.3 `ServiceSubcategory` allows only
`Other (Other)` under `Other` (FOCUS_Spec v1.3
`specification/datasets/cost_and_usage/columns/servicesubcategory.md`, read
2026-10-03). App Service plans therefore send `Other` / `Other (Other)` with
`x_ServiceCategory=Web` and `x_ServiceSubcategory=Application Platforms`.
finfocus core never reads `service_category`, so nothing downstream depends
on the enum value. rshade/finfocus-spec#612 tracks adding the values.

## R5: Who reads the record today?

finfocus core's `internal/proto/adapter.go` reads only `billing_currency` and
`pricing_currency` from `focus_record`, so the provider change has no runtime
consumer impact.

## R6: Pricing basis

**Decision**:

- With one positive meter whose unit is hourly or monthly, use that meter's
  unit and price, with quantity = cost / price.
- Otherwise, use window hours.

**Rationale**:

- FOCUS `PricingUnit` is the provider's unit for the price.
- `ListCost = ListUnitPrice × PricingQuantity` holds by construction.
- Multi-meter quotes (AKS, SQL, Functions, Cosmos, Load Balancer overage) have
  no single unit.
- Tiered GB-month storage would misstate GB-months with a single price.

## R7: Mandatory columns (analysis pass)

The FOCUS 1.3 Cost and Usage columns marked Mandatory are:

- BilledCost
- BillingAccountId
- BillingAccountName
- BillingCurrency
- BillingPeriodStart and BillingPeriodEnd
- ChargeCategory
- ChargeClass
- ChargeDescription
- ChargePeriodStart and ChargePeriodEnd
- ContractedCost
- EffectiveCost
- HostProviderName
- InvoiceIssuerName
- ListCost
- PricingQuantity
- PricingUnit
- ProviderName (deprecated)
- PublisherName (deprecated)
- ServiceCategory
- ServiceName
- ServiceProviderName

The first draft dropped ProviderName and missed PublisherName,
InvoiceIssuerName, and BillingAccountName. The spec now sets all of them.
