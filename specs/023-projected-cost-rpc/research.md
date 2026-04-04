# Research: GetProjectedCost RPC Implementation

**Branch**: `023-projected-cost-rpc` | **Date**: 2026-04-04

## R1: Refactoring projectedQueryFromRequest to use MapDescriptorToQuery

**Decision**: Replace the boolean-return `projectedQueryFromRequest()` with
a call to `MapDescriptorToQuery()` which returns sentinel errors.

**Rationale**: `MapDescriptorToQuery()` already validates provider,
resource type, region, and SKU with proper sentinel errors
(`ErrUnsupportedResourceType`, `ErrMissingRequiredFields`) that map to gRPC
codes via `MapToGRPCStatus()`. The current `projectedQueryFromRequest()`
duplicates validation logic and discards error context by returning a boolean.

**Alternatives considered**:

- Keep `projectedQueryFromRequest()` but add error returns: Rejected because
  it would duplicate validation logic already in `MapDescriptorToQuery()`.
- Call `MapDescriptorToQuery()` inside `projectedQueryFromRequest()`: Viable
  but adds unnecessary indirection. Better to call the mapper directly in
  `GetProjectedCost()`.

## R2: Resource Type URN Format in Existing Test

**Decision**: Update the existing test fixture
`TestGetProjectedCostSetsExpiresAtFromCache` to use short-form resource type
`"compute/VirtualMachine"` instead of full URN
`"azure:compute/virtualMachine:VirtualMachine"`.

**Rationale**: The mapper performs `strings.ToLower(desc.GetResourceType())`
and does a direct lookup against keys like `"compute/virtualmachine"`. The
full URN format does not match. The existing test only passes because the
current code ignores the resource type field entirely. This is a test data
fix, not a behavioral change.

**Alternatives considered**:

- Add URN parsing to the mapper: Rejected because `ResourceDescriptor.ResourceType`
  is defined as the type segment, not the full Pulumi URN. Changing the
  mapper's contract for a test fixture issue is overengineering.
- Add URN stripping in `GetProjectedCost()`: Rejected — same reason as above.

## R3: Resource Type Routing (VM vs Disk/Blob)

**Decision**: For this feature, only implement the full projected cost path
for VMs (`compute/VirtualMachine`). Disk and Blob Storage pass mapper
validation but return `codes.Unimplemented` with a descriptive message at
the resource-type routing level.

**Rationale**: VMs use hourly pricing (unit_price × 730 = monthly cost).
Disks use monthly pricing directly. Blob Storage has a different pricing
model entirely. Implementing all three would significantly expand scope
beyond issue #59. The mapper validates the type is "known" but the cost
computation path is VM-specific.

**Alternatives considered**:

- Implement all three in this feature: Rejected — scope creep beyond issue #59.
- Block disk/blob at the mapper level: Rejected — the mapper correctly
  identifies them as "known" types. The routing decision belongs in the RPC
  handler.

## R4: billing_detail Format

**Decision**: Use format
`"Azure Retail Prices API: {SKU} in {region} at ${unit_price}/hr * 730 hrs/mo"`
matching the `EstimateCost` pattern.

**Rationale**: The `WithProjectedCostDetails()` pluginsdk helper accepts a
`billingDetail string` parameter. The format should be human-readable and
include enough context for debugging. The issue example shows this exact
pattern.

**Alternatives considered**:

- Machine-parseable JSON: Rejected — `billing_detail` is documented as
  informational; structured data belongs in dedicated response fields.
- Include cache status: Rejected — cache is transparent to consumers.

## R5: pluginsdk Response Option Composition

**Decision**: Use `WithProjectedCostDetails()` for core fields,
`WithProjectedCostPricingCategory()` for pricing category, and
`WithProjectedCostExpiresAt()` for cache expiry.

**Rationale**: The pluginsdk provides functional option helpers that compose
cleanly. `WithProjectedCostDetails()` sets unit_price, currency,
cost_per_month, and billing_detail in a single call.
`WithProjectedCostPricingCategory()` is a separate option since it's an enum
value, not a string/float. This matches how `EstimateCost` uses
`WithEstimateCost()` + `WithPricingCategory()`.

**Alternatives considered**:

- Set response fields directly without helpers: Rejected — pluginsdk helpers
  are the established pattern and include validation.
