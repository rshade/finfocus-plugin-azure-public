# Research: Cost Calculation Utilities

**Branch**: `019-cost-utilities` | **Date**: 2026-03-03

## R1: Hours-per-Month Constant

**Decision**: Use 730 (365 * 24 / 12)

**Rationale**: This is the industry-standard average hours per month used
by all major cloud providers (Azure, AWS, GCP) for pricing calculations.
Azure's own pricing page uses this exact multiplier for VM cost
estimates. Using calendar-specific month lengths would introduce
unnecessary complexity and inconsistency with Azure's displayed prices.

**Alternatives considered**:

- 720 (30 * 24): Simpler but inaccurate, undercounts by ~1.4%
- Calendar-specific (28-31 days): Accurate per-month but inconsistent
  with Azure pricing methodology and requires date context
- 730.5 (365.25 * 24 / 12): Accounts for leap years but no cloud
  provider uses this value

## R2: Rounding Strategy

**Decision**: Standard arithmetic rounding (round half up) via
`math.Round(x * 100) / 100`

**Rationale**: Matches cloud billing display conventions. Azure portal,
AWS cost explorer, and GCP billing all display costs rounded to two
decimal places using standard rounding. Banker's rounding would
occasionally produce results that differ from what users see on their
cloud provider dashboards.

**Alternatives considered**:

- Banker's rounding (round half to even): Reduces statistical bias in
  large datasets but doesn't match cloud billing UI conventions
- Truncation (floor): Would systematically undercount, creating
  discrepancy with provider-displayed prices
- No rounding (full precision): Produces values like $73.00000000000001
  due to floating-point representation, confusing in reports

## R3: Package Location

**Decision**: `internal/estimation/cost.go`

**Rationale**: Creates a clean domain boundary for cost estimation logic.
The `internal/pricing` package is the gRPC service layer and should not
contain utility math. A dedicated `estimation` package:

- Aligns with the `component/estimation` issue label
- Will house future estimation logic (field mapping, resource matching)
- Can be imported by `internal/pricing` without circular dependencies
- Follows the existing pattern of focused single-purpose packages
  (`azureclient`, `logging`, `client`)

**Alternatives considered**:

- `internal/pricing/cost.go`: Mixes gRPC concerns with pure math
- `internal/cost/calc.go`: Generic name, less aligned with domain
- `pkg/estimation/`: Unnecessary public exposure for internal utilities

## R4: Function Signatures

**Decision**: Package-level functions taking `float64` and returning
`float64`. No struct receiver.

**Rationale**: These are pure transformations with no state. Using
package-level functions (like `math.Round`) is idiomatic Go for stateless
operations. A struct would add unnecessary complexity without benefit.

**Alternatives considered**:

- Method on a `Converter` struct: Over-engineered for stateless math
- Accept/return `decimal.Decimal`: Adds dependency for unnecessary
  precision (two decimal places is sufficient for cost display)
- Return `(float64, error)`: No error conditions exist (division by
  constant is always valid, rounding is always valid)

## R5: Negative Input Handling

**Decision**: Pass through — negative inputs produce negative outputs

**Rationale**: Azure pricing can include credits and refunds which
appear as negative line items. Preserving sign through conversions
allows the same utility functions to handle both costs and credits
without special-casing.

**Alternatives considered**:

- Return error on negative input: Breaks credit/refund use cases
- Return absolute value: Silently discards sign information
- Clamp to zero: Same issue as absolute value
