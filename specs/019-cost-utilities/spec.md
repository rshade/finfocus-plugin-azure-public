# Feature Specification: Cost Calculation Utilities

**Feature Branch**: `019-cost-utilities`
**Created**: 2026-03-03
**Status**: Draft
**Input**: GitHub Issue #19 — Create cost calculation utilities (hourly to monthly conversion)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Convert Hourly VM Rates to Monthly and Yearly Estimates (Priority: P1)

Azure pricing returns hourly rates for compute resources (e.g., Virtual Machines). The cost estimation
logic needs to present users with monthly and yearly cost projections derived from these hourly rates
so they can plan budgets and compare options at familiar time horizons.

**Why this priority**: Hourly-to-monthly conversion is the most common path — VM pricing is always
hourly, and monthly cost is the primary reporting period for cloud budgets. Without this, no cost
estimates can be generated.

**Independent Test**: Can be fully tested by providing a known hourly rate (e.g., $0.10/hr) and
verifying the monthly and yearly outputs match expected values ($73.00/mo, $876.00/yr).

**Acceptance Scenarios**:

1. **Given** an hourly rate of $0.10, **When** converting to monthly, **Then** the result is $73.00
2. **Given** an hourly rate of $0.10, **When** converting to yearly, **Then** the result is $876.00
3. **Given** an hourly rate of $0.00, **When** converting to monthly or yearly, **Then** the result is $0.00
4. **Given** an hourly rate with many decimal places (e.g., $0.105), **When** converting, **Then** the result is rounded to two decimal places (currency precision)

---

### User Story 2 - Convert Monthly Rates Back to Hourly (Priority: P2)

Some Azure resources (e.g., managed disks) report pricing as a flat monthly rate. The estimation logic
needs to normalize these back to hourly rates so that all cost comparisons use a common base unit
before computing final monthly or yearly totals.

**Why this priority**: Enables uniform cost normalization. Less common than hourly-to-monthly, but
required for disk and storage pricing which uses monthly rates.

**Independent Test**: Can be fully tested by providing a known monthly rate (e.g., $730.00/mo) and
verifying the hourly output matches the expected value ($1.00/hr).

**Acceptance Scenarios**:

1. **Given** a monthly rate of $730.00, **When** converting to hourly, **Then** the result is $1.00
2. **Given** a monthly rate of $0.00, **When** converting to hourly, **Then** the result is $0.00
3. **Given** a monthly rate that produces a fractional hourly value, **When** converting, **Then** the result is rounded to two decimal places

---

### User Story 3 - Consistent Rounding for Currency Precision (Priority: P2)

All cost conversions must produce values rounded to exactly two decimal places (cents). This prevents
floating-point artifacts from propagating through cost reports and ensures displayed values match what
users expect from currency amounts.

**Why this priority**: Financial data must be precise. Floating-point drift across multiple
calculations can lead to user-visible inconsistencies in cost reports.

**Independent Test**: Can be fully tested by providing inputs known to produce half-cent boundaries
(e.g., $0.105) and verifying correct banker-adjacent rounding behavior ($0.11).

**Acceptance Scenarios**:

1. **Given** a conversion that produces $0.105, **When** rounding, **Then** the result is $0.11
2. **Given** a conversion that produces $73.00 exactly, **When** rounding, **Then** the result remains $73.00
3. **Given** a conversion that produces $0.004, **When** rounding, **Then** the result is $0.00

---

### Edge Cases

- What happens when a negative rate is provided? Negative inputs should pass through the conversion
  and return a negative result (preserving sign), since credit adjustments may use negative values.
- What happens with extremely large hourly rates? The conversion must not overflow or lose precision
  for rates up to $10,000/hr (monthly = $7,300,000).
- What happens with extremely small hourly rates (e.g., $0.001)? The result should round correctly
  to two decimal places even when the output is near zero.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST convert an hourly cost to a monthly cost using 730 hours per month (365 days x 24 hours / 12 months)
- **FR-002**: System MUST convert an hourly cost to a yearly cost using 8,760 hours per year (365 days x 24 hours)
- **FR-003**: System MUST convert a monthly cost to an hourly cost by dividing by 730
- **FR-004**: System MUST round all conversion results to exactly two decimal places (currency precision)
- **FR-005**: System MUST return zero when given a zero input (no division-by-zero errors)
- **FR-006**: System MUST preserve the sign of the input (negative inputs produce negative outputs) to support credit adjustments
- **FR-007**: All conversion functions MUST be pure — producing the same output for the same input with no side effects

### Key Entities

- **Cost Rate**: A monetary value associated with a time period (hourly, monthly, or yearly). Represented as a decimal number with two-place precision.
- **Time Period Multiplier**: A constant that defines the relationship between time periods. Hours-per-month (730) and hours-per-year (8,760) are industry-standard values derived from a 365-day year.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All three conversion directions (hourly-to-monthly, hourly-to-yearly, monthly-to-hourly) produce correct results for known reference values
- **SC-002**: No conversion result ever has more than two decimal places, verified across a range of inputs including boundary values
- **SC-003**: Conversion functions handle the full practical range of Azure pricing ($0.00 to $10,000/hr) without precision loss or overflow
- **SC-004**: Zero and negative inputs are handled gracefully with no errors or panics
- **SC-005**: Test coverage for cost conversion logic is at or above 80%

## Assumptions

- **730 hours/month**: The standard industry average (365 x 24 / 12) is used rather than calendar-specific month lengths. This matches Azure's own pricing methodology.
- **No currency conversion**: All values are assumed to be in a single currency. Multi-currency support is out of scope.
- **Rounding strategy**: Standard arithmetic rounding (round half up) is used rather than banker's rounding. This is consistent with typical cloud billing display conventions.
- **No quantity multiplication**: These utilities convert between time periods only. Multiplying by resource count or quantity is handled elsewhere.

## Scope Boundaries

### In Scope

- Hourly-to-monthly conversion
- Hourly-to-yearly conversion
- Monthly-to-hourly conversion
- Two-decimal-place rounding for all outputs
- Pure, stateless functions

### Out of Scope

- Monthly-to-yearly or yearly-to-monthly conversions (can be composed from existing functions)
- Currency conversion or multi-currency support
- Quantity or resource count multiplication
- Tax or discount calculations
- Per-second or per-minute granularity

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [ ] Integration test needs identified (external API interactions) — N/A: pure functions with no external dependencies
- [ ] Performance test criteria specified (if applicable) — N/A: trivial arithmetic operations

### User Experience

- [ ] Error messages are user-friendly and actionable — N/A: library functions, not user-facing
- [ ] Response time expectations defined — N/A: sub-microsecond arithmetic
- [x] Observability requirements specified (logging, metrics) — No logging needed for pure math functions

### Documentation

- [ ] README.md updates identified (if user-facing changes) — N/A: internal library
- [x] API documentation needs outlined (godoc comments, contracts)
- [x] Docstring coverage ≥80% maintained (all exported symbols documented)
- [ ] Examples/quickstart guide planned (if new capability) — N/A: straightforward API

### Performance & Reliability

- [x] Performance targets specified (response times, throughput) — sub-microsecond, no I/O
- [x] Reliability requirements defined (retry logic, error handling) — pure functions, deterministic
- [x] Resource constraints considered (memory, connections, cache TTL) — zero allocations expected

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data
