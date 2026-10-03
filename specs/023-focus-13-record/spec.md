# Feature Specification: FOCUS 1.3 Cost Record Alignment

**Feature Branch**: `023-focus-13-record`
**Created**: 2026-10-03
**Status**: Draft
**Input**: Issue #46 ("Align response fields with FOCUS 1.3 specification"),
narrowed to what finfocus-spec v0.7.1 already defines.

## Scope

finfocus-spec v0.7.1 already carries a FOCUS 1.3 and 1.4 `FocusCostRecord`
(finfocus-spec #183, #540, #541, #542). The proto extension that issue #46
sketches is therefore already in place. `GetActualCost` attaches that record
when a billing account id is set. This feature fixes the columns the plugin
fills with deprecated, missing, or non-conformant values.

Projected and estimate responses are not FOCUS rows. FOCUS describes billed
cost and usage, and those responses already carry the FOCUS
`pricing_category` enum. They stay unchanged.

One gap needs a spec change. `FocusServiceCategory` lacks nine FOCUS 1.3
categories, including `Web`. That is tracked in rshade/finfocus-spec#612 and is
out of scope here.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - FOCUS 1.3 provider columns (Priority: P1)

A FinOps tool that reads FOCUS 1.3 rows finds the provider in
`ServiceProviderName` and `HostProviderName`. FOCUS 1.3 deprecates
`ProviderName` and `PublisherName`, but still lists both as mandatory and not
null. FOCUS 1.4 removes them. `InvoiceIssuerName` and `BillingAccountName` are
also mandatory.

**Why this priority**: These are mandatory FOCUS 1.3 columns. Today the plugin
fills only the deprecated column.

**Independent Test**: Request an actual cost with a billing account id and read
`focus_record`.

**Acceptance Scenarios**:

1. **Given** an actual-cost request for any supported type with a billing
   account id, **When** the record is built, **Then**
   `service_provider_name`, `host_provider_name`, `invoice_issuer`, and the
   deprecated but still mandatory `provider_name` and `publisher` are all
   `Microsoft`, and `billing_account_name` equals the billing account id.

---

### User Story 2 - Cost and pricing columns that agree (Priority: P1)

A FinOps tool can recompute `ListCost` as `ListUnitPrice × PricingQuantity`,
and `ContractedCost` as `ContractedUnitPrice × PricingQuantity`, with
`PricingUnit` in FOCUS unit format and expressed in the meter's own unit where
one meter prices the resource.

**Why this priority**: `ContractedCost` is mandatory and is missing today. The
pricing unit is `hour`, which breaks FOCUS unit-format capitalization and is
wrong for monthly meters such as managed disks.

**Independent Test**: Build records for a VM priced by one hourly meter, a scale set,
a monthly disk, a multi-meter AKS cluster, and a zero-price plan, then check
unit, quantity, unit price, and both cost identities.

**Acceptance Scenarios**:

1. **Given** a quote with exactly one hourly meter, **When** the record is
   built, **Then** `pricing_unit` is `Hours`, `list_unit_price` is the meter
   price, and `pricing_quantity` is cost divided by that price (instance-hours
   for a scale set).
2. **Given** a quote with exactly one monthly meter, **When** the record is
   built, **Then** `pricing_unit` is `Months` and `pricing_quantity` is the
   fraction of a month in the window.
3. **Given** a quote with several meters, a zero price, or any other unit,
   **When** the record is built, **Then** the basis is the window in `Hours`
   and `list_unit_price` is cost divided by window hours.
4. **Given** any quote, **When** the record is built, **Then**
   `contracted_unit_price` equals `list_unit_price`, and `contracted_cost`,
   `list_cost`, `billed_cost`, and `effective_cost` equal the window cost.

---

### User Story 3 - Service category and subcategory (Priority: P2)

A FinOps tool groups Azure rows from this plugin with the same category and
subcategory that Microsoft's own FOCUS data uses for those resource types.

**Why this priority**: `ServiceSubcategory` is recommended, not mandatory, but
it is the main dimension for workload-type analysis.

**Independent Test**: Table test over every supported resource type.

**Acceptance Scenarios**:

1. **Given** each supported type, **When** the record is built, **Then**
   category and subcategory follow microsoft/finops-toolkit `Services.csv`,
   except App Service plans, which use `Compute` / `Other (Compute)` until the
   spec adds `Web` (finfocus-spec#612).
2. **Given** any supported type, **When** the record is built, **Then** the
   subcategory is a FOCUS 1.3 allowed value whose parent is the record's
   category.

---

### User Story 4 - Column mapping documentation (Priority: P3)

A maintainer can see which Azure source fills each FOCUS column, and which
columns stay empty and why.

**Independent Test**: The document lists every column the record sets and
every column it leaves empty on purpose.

### Edge Cases

- Window hours of 0: no pricing columns are set, and the record is unchanged
  from today's behavior.
- Meter price of 0 with a single meter (for example App Service `F1`): use the
  window basis, so no division by zero.
- Spot VM: single Spot meter, `pricing_category` Dynamic, hourly basis.
- Unknown resource type: the existing unsupported-type error is unchanged.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The record MUST set `service_provider_name`,
  `host_provider_name`, `invoice_issuer`, `provider_name`, and `publisher` to
  `Microsoft`, and `billing_account_name` to the billing account id, because
  the plugin knows no account display name.
- **FR-002**: The record MUST set `contracted_cost` and
  `contracted_unit_price`. With no negotiated discounts in public prices,
  contracted equals list.
- **FR-003**: `pricing_unit` and `consumed_unit` MUST use FOCUS unit format
  (`Hours`, `Months`).
- **FR-004**: With exactly one meter that has a positive price and an hourly
  or monthly unit, the pricing basis MUST use that meter's unit and price, with
  quantity equal to cost divided by price. Every other quote MUST use the
  window-hours basis.
- **FR-005**: `list_cost` MUST equal `list_unit_price × pricing_quantity`, and
  `contracted_cost` MUST equal `contracted_unit_price × pricing_quantity`,
  within the SDK's validation tolerance.
- **FR-006**: The record MUST set `service_subcategory` to a FOCUS 1.3 allowed
  value whose parent is the record's service category, using the mapping in
  User Story 3.
- **FR-007**: Every record MUST pass
  `pluginsdk.ValidateFocusRecordWithOptions` in aggregate mode with no errors.
- **FR-008**: Projected cost, estimate cost, and DryRun responses MUST NOT
  change.
- **FR-009**: Documentation MUST map each FOCUS column the record sets to its
  Azure source, and list deliberately empty columns with the reason.

### Key Entities

- **FOCUS cost record**: one `FocusCostRecord` per actual-cost result.
- **Pricing basis**: unit, quantity, and unit price derived from the quote's
  meters, or from the window when no single meter applies.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For all ten supported resource types, the built record passes
  aggregate FOCUS validation with zero errors.
- **SC-002**: Every FOCUS 1.3 mandatory Cost and Usage column that the plugin
  can know is set, and the rest are listed in the documentation with a reason.
- **SC-003**: No change in projected or estimate responses. The existing
  golden and gRPC tests pass unchanged.

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80%)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified: none, because no Azure query changes
- [x] Performance test criteria specified: not applicable (mapping only)

### User Experience

- [x] Error messages are user-friendly and actionable (unchanged errors)
- [x] Response time expectations defined (no new I/O)
- [x] Observability requirements specified (no new logs)

### Documentation

- [x] README.md updates identified (none; the column mapping goes in `docs/`)
- [x] API documentation needs outlined (CLAUDE.md FOCUS note, `docs/focus-mapping.md`)
- [x] Docstring coverage ≥80% maintained
- [x] Examples/quickstart guide planned (quickstart.md)

### Performance & Reliability

- [x] Performance targets specified (no new I/O)
- [x] Reliability requirements defined (division by zero avoided)
- [x] Resource constraints considered (none)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data (the category table is ten rows of
  classification, not prices)
