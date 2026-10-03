# Implementation Plan: FOCUS 1.3 Cost Record Alignment

**Branch**: `023-focus-13-record` | **Date**: 2026-10-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/023-focus-13-record/spec.md`

## Summary

This plan corrects the `FocusCostRecord` that `GetActualCost` builds in
`internal/pricing/focus.go`. Five things change:

- Providers move to the FOCUS 1.3 columns (`Microsoft`).
- The mandatory `ContractedCost` is now set, along with `ContractedUnitPrice`.
- The pricing basis comes from the single selected meter when there is one,
  with units in FOCUS format.
- `ServiceSubcategory` is set from Microsoft's own FOCUS service mapping.
- The column mapping is documented.

Projected, estimate, and DryRun responses do not change. The `Web` category
waits on finfocus-spec#612.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: finfocus-spec v0.7.1 (`pluginsdk.FocusRecordBuilder`,
`ValidateFocusRecordWithOptions`)
**Storage**: N/A (stateless)
**Testing**: `go test -race`, table-driven, no network
**Target Platform**: Linux gRPC plugin
**Project Type**: single
**Performance Goals**: no new I/O; mapping only
**Constraints**: no new proto fields (spec-owned); keep the change in
`focus.go` and `focus_test.go` to limit conflicts with open PRs #70–#73
**Scale/Scope**: 10 supported resource types

## Constitution Check

- [x] **Code Quality**: golangci-lint via `make lint`. The basis selection is a
  small pure function. Errors stay wrapped; unsupported types keep
  `ErrUnsupportedResourceType`.
- [x] **Testing**: TDD. Failing tests in `focus_test.go` first, then
  implementation. Table-driven tests named
  `Test<Function>_<Scenario>_<ExpectedOutcome>`, run with `-race` through
  `make test`.
- [x] **User Experience**: no lifecycle or logging change.
- [x] **Documentation**: godoc on new helpers, `docs/focus-mapping.md`, and the
  CLAUDE.md FOCUS note.
- [x] **Performance**: no new calls; constant-time mapping.
- [x] **Architectural Constraints**: no auth, no storage, no mutation. The
  category table is classification for ten types, not pricing data.

## Design

### Providers (FR-001)

`WithIdentity("Microsoft", id, id)`, `WithPublisher("Microsoft")`,
`WithServiceProvider("Microsoft")`, `WithHostProvider("Microsoft")`, and
`WithInvoice("", "Microsoft")`.

- FOCUS 1.3 marks `ProviderName` and `PublisherName` as deprecated, but they
  are still mandatory and must not be null. `InvoiceIssuerName` and
  `BillingAccountName` are mandatory too.
- `Microsoft` is the publisher name in microsoft/finops-toolkit `Services.csv`
  for every supported type.
- The invoice issuer is Microsoft for direct customers. The plugin cannot see
  a reseller, which the docs note.
- The account name equals the id, matching the sibling AWS Cost Explorer
  plugin.
- `provider_name` changes from `azure` to `Microsoft`. finfocus core reads only
  `billing_currency` and `pricing_currency` from the record
  (`internal/proto/adapter.go`), so no current reader is affected.
- With both the deprecated and the new provider fields set, the SDK logs one
  deprecation warning per process. FOCUS 1.3 requires both, so the warning is
  expected.

### Pricing basis (FR-002–FR-005)

`focusPricingBasis(quote, cost, hours) (quantity, unit, unitPrice)`:

| Quote meters | Unit, unit price, quantity |
| --- | --- |
| one meter, price > 0, hourly | `Hours`, price, cost / price |
| one meter, price > 0, monthly | `Months`, price, cost / price |
| anything else | `Hours`, cost / hours, hours |

- **ContractedUnitPrice:** equals unitPrice.
- **ContractedCost, ListCost, BilledCost, EffectiveCost:** all equal the window
  cost.
- **ConsumedQuantity and ConsumedUnit:** equal the pricing pair.
  `ConsumedQuantity > 0` holds for usage records whenever hours > 0. With a
  positive single meter it equals cost / price, which is also > 0 unless cost
  is 0. A zero cost falls back to hours.
- **Storage GB-month meters:** use the window basis. Tiered bands make
  cost / first-band price an inaccurate GB-month count.

### Service subcategory (FR-006)

`focusServiceClass(resourceType, tags) (category, subcategory, error)`
replaces `focusServiceCategory`. The categories come from Services.csv:

| Type | Category | Subcategory |
| --- | --- | --- |
| Virtual machine, scale sets | Compute | Virtual Machines |
| Managed disk | Compute | Virtual Machines |
| Storage account, Blob | Storage | Storage Platforms |
| App Service plan | Compute | Other (Compute), see note |
| Function App | Compute | Serverless Compute |
| AKS | Compute | Containers |
| SQL Database | Databases | Relational Databases |
| Cosmos DB | Databases | NoSQL Databases |
| Load Balancer | Networking | Application Networking |

App Service plans move to Web / Application Platforms once
finfocus-spec#612 adds `Web`.

Managed disks move from Storage to Compute, matching Microsoft's FOCUS data
for `microsoft.compute/disks`. This is a visible value change, and the PR
records it.

### Deferred, with reasons (documented)

- `RegionName` keeps the region id. A display name needs the price row
  `location` plumbed through every quote builder, and those builders are being
  changed by open PRs. FOCUS also requires the host-provider display name
  (`East US`), while the retail `location` field is `US East`.
- `SkuPriceId` and `SkuMeter`: `quoteMeter` does not carry `meterId` or
  `meterName`. The same plumbing applies.
- Commitment, capacity reservation, invoice, and allocation columns do not
  apply to list-price projections.
- `Tags`: descriptor tags are flattened Pulumi inputs, not resource tags
  (finfocus-spec#609).

## Project Structure

### Documentation (this feature)

```text
specs/023-focus-13-record/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/pricing/
├── focus.go          # record builder, pricing basis, service class
└── focus_test.go     # TDD tests
docs/
└── focus-mapping.md  # FOCUS column → Azure source
CLAUDE.md             # FOCUS note under "Other cost RPCs"
```

**Structure Decision**: single project; change confined to `internal/pricing`
FOCUS code and docs.

## Complexity Tracking

None.
