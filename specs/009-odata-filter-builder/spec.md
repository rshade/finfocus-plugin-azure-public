# Feature Specification: OData Filter Query Builder

**Feature Branch**: `009-odata-filter-builder`
**Created**: 2026-02-28
**Status**: Draft
**Input**: GitHub Issue #9 - Implement OData filter query builder

## Clarifications

### Session 2026-02-28

- Q: Should the builder support OR logic in addition to AND? → A: Yes,
  support both AND and OR with explicit method selection.
- Q: Should the builder include ProductName and CurrencyCode fields
  (from existing PriceQuery) beyond the 4 in the issue? → A: Yes,
  include all 6 named methods (Region, Service, SKU, Type, ProductName,
  CurrencyCode) plus a generic Field(name, value) for arbitrary OData
  fields.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Build Simple Price Filters (Priority: P1)

As cost estimation logic, I need to construct filter expressions that
target specific cloud resources by region, service, SKU, product name,
currency code, and pricing type so that the pricing API returns only
relevant results. The builder should provide named methods for common
fields plus a generic method for arbitrary OData fields, offering a
clear, composable interface without requiring knowledge of the
underlying query syntax.

**Why this priority**: This is the core use case. Without the ability to filter by region, service, and SKU, the system cannot retrieve targeted pricing data, making cost estimation impossible.

**Independent Test**: Can be fully tested by constructing a filter with region and SKU criteria and verifying the output matches the expected query syntax. Delivers the ability to query for specific VM pricing in a specific region.

**Acceptance Scenarios**:

1. **Given** a new filter builder, **When** region "eastus" and SKU "Standard_B1s" are specified, **Then** the output contains both criteria plus the default Consumption pricing type, all joined with AND logic
2. **Given** a new filter builder, **When** only region "westus2" is specified, **Then** the output contains only the region criterion plus the default pricing type filter
3. **Given** a new filter builder, **When** region, service "Virtual
   Machines", and SKU "B1s" are all specified, **Then** the output
   contains all three criteria plus the default Consumption pricing
   type, combined correctly with AND logic
4. **Given** a new filter builder, **When** a generic field
   "meterName" with value "B1s Low Priority" is specified alongside
   region "eastus", **Then** the output includes both the named region
   criterion and the arbitrary field criterion

---

### User Story 2 - Default Pay-As-You-Go Pricing Type (Priority: P1)

As the HTTP client, I need every filter to include a default pricing type of "Consumption" (pay-as-you-go) unless explicitly overridden, so that queries return the most commonly needed pricing tier without requiring callers to remember this filter.

**Why this priority**: Without a default pricing type, queries may return mixed results including reservation and dev/test pricing, leading to incorrect cost estimates. This is equally critical as basic filtering.

**Independent Test**: Can be tested by building a filter with no explicit type and verifying the output includes the Consumption type criterion. Also testable by overriding the type and confirming the override takes effect.

**Acceptance Scenarios**:

1. **Given** a new filter builder with region "eastus", **When** no pricing type is explicitly set, **Then** the output includes both the region and the default Consumption type filter
2. **Given** a new filter builder, **When** pricing type is explicitly set to "Reservation", **Then** the output uses "Reservation" instead of the default "Consumption"
3. **Given** a new filter builder with no criteria set, **When** the filter is built, **Then** the output is a minimal valid filter containing only the default Consumption type

---

### User Story 3 - Fluent API for Complex Filters (Priority: P2)

As a developer, I want a chainable, fluent interface for building
filters with explicit AND/OR logic selection so that complex
multi-criteria queries are readable and easy to construct without
manual string concatenation.

**Why this priority**: Developer ergonomics improve adoption and reduce
bugs. While the system works with a basic interface, a fluent API with
both AND and OR support reduces mistakes from manual filter assembly
and enables more expressive queries (e.g., querying multiple regions
or multiple SKUs in a single request).

**Independent Test**: Can be tested by chaining multiple filter methods
with both AND and OR operators in a single expression and verifying
the resulting output is correct and readable.

**Acceptance Scenarios**:

1. **Given** a new filter builder, **When** Region, Service, SKU, and
   Type methods are chained together with AND logic, **Then** a
   correctly formatted filter string is produced with all four
   criteria joined by AND
2. **Given** a new filter builder, **When** methods are called in any
   order, **Then** the output is deterministic and independent of call
   order
3. **Given** a new filter builder, **When** two region criteria are
   combined with OR logic, **Then** the output produces a filter with
   the two regions joined by OR and properly grouped
4. **Given** a new filter builder, **When** AND and OR logic are mixed,
   **Then** OR-grouped criteria are parenthesized to preserve correct
   operator precedence

---

### User Story 4 - Safe Value Handling (Priority: P2)

As a test writer and security-conscious developer, I want filter values to be properly escaped so that special characters in input (such as quotes or apostrophes) do not corrupt the query syntax or cause unexpected behavior.

**Why this priority**: Malformed queries could return wrong data or cause API errors. Proper escaping ensures reliability and prevents query injection.

**Independent Test**: Can be tested by constructing filters with values containing special characters (single quotes, spaces, Unicode) and verifying the output is syntactically valid.

**Acceptance Scenarios**:

1. **Given** a filter builder, **When** a value containing a single quote (e.g., "O'Brien") is provided, **Then** the quote is properly escaped in the output
2. **Given** a filter builder, **When** a value containing spaces (e.g., "Virtual Machines") is provided, **Then** the value is preserved correctly in the output
3. **Given** a filter builder, **When** an empty string is provided for a field, **Then** that field is omitted from the filter rather than producing an invalid empty clause

---

### Edge Cases

- What happens when all filter fields are empty? The builder returns the minimal valid filter (default Consumption type only).
- What happens when a value contains single quotes? Single quotes are escaped per OData conventions (doubled: `''`).
- What happens when the same field is set multiple times? The last value wins (override behavior).
- What happens when a value contains only whitespace? Whitespace-only values are treated as empty and the field is omitted.
- What happens when AND and OR are mixed without explicit grouping? OR-joined criteria are automatically parenthesized to ensure correct OData operator precedence (AND binds tighter than OR in OData, but explicit grouping avoids ambiguity).
- What happens when a generic field name is empty? The field is omitted from the filter (same as empty value behavior).
- What happens when a generic field duplicates a named field (e.g., Field("armRegionName", "eastus") alongside Region("westus2"))? Both are included; the builder does not deduplicate across named and generic fields. The caller is responsible for avoiding conflicts.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a filter builder that constructs OData-compatible filter expressions from individual search criteria
- **FR-002**: System MUST support filtering by region name, service
  name, SKU name, product name, and currency code as individual
  named composable AND criteria with last-write-wins semantics.
  Pricing type filtering is governed separately by FR-003 and FR-004
  due to its default-value behavior.
- **FR-002a**: System MUST provide a generic field method that accepts
  an arbitrary field name and value, enabling filters on any OData
  field not covered by the named methods
- **FR-003**: System MUST include a default pricing type filter of "Consumption" (pay-as-you-go) when no explicit type is specified
- **FR-004**: System MUST allow the default pricing type to be overridden by explicitly setting a different type value
- **FR-005**: System MUST support combining criteria using both AND and OR logic with explicit method selection for each operator
- **FR-005a**: System MUST default to AND logic when combining filter criteria
- **FR-005b**: System MUST support OR logic for combining alternative values (e.g., region "eastus" OR region "westus2")
- **FR-005c**: System MUST automatically parenthesize OR-grouped criteria when mixed with AND to preserve correct operator precedence
- **FR-006**: System MUST properly escape special characters in filter values (specifically single quotes, per OData v4 conventions)
- **FR-007**: System MUST return a minimal valid filter (containing only the default type) when no other criteria are specified
- **FR-008**: System MUST provide a fluent/chainable interface where each filter method returns the builder for method chaining
- **FR-009**: System MUST omit fields with empty or whitespace-only values from the filter expression rather than producing invalid clauses
- **FR-010**: System MUST produce deterministic output regardless of the order in which filter methods are called

### Key Entities

- **FilterBuilder**: A composable query constructor that accumulates
  filter criteria with explicit AND/OR logic and produces a formatted
  filter expression. Named attributes: region, service, SKU, pricing
  type, product name, and currency code. Also supports arbitrary
  field/value pairs via a generic method. Tracks the logical operator
  (AND/OR) binding each group.
- **Filter Expression**: The string output of the builder, formatted as
  an OData-compatible query filter with properly escaped values,
  AND/OR-joined criteria, and parenthesized grouping where needed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All filter construction operations complete in under 1 millisecond for up to 10 combined criteria
- **SC-002**: 100% of filter expressions produced by the builder are syntactically valid OData filter strings
- **SC-003**: Test coverage for the filter builder reaches at least
  80%, covering single-field, multi-field, empty, special-character,
  AND-only, OR-only, and mixed AND/OR scenarios
- **SC-004**: Developers can construct a complete multi-criteria filter in a single chained expression without intermediate variables
- **SC-005**: Zero query syntax errors caused by unescaped special characters in filter values

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (>=80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (external API interactions)
- [x] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (e.g., cache hits <10ms, API calls <2s p95)
- [x] Observability requirements specified (logging, metrics)

### Documentation

- [x] README.md updates identified (if user-facing changes)
- [x] API documentation needs outlined (godoc comments, contracts)
- [x] Docstring coverage >=80% maintained (all exported symbols documented)
- [x] Examples/quickstart guide planned (if new capability)

### Performance & Reliability

- [x] Performance targets specified (response times, throughput)
- [x] Reliability requirements defined (retry logic, error handling)
- [x] Resource constraints considered (memory, connections, cache TTL)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data

## Assumptions

- The Azure Retail Prices API uses OData v4 filter syntax where values are enclosed in single quotes and criteria are joined with ` and `.
- "Consumption" is the correct type value for pay-as-you-go pricing in the Azure Retail Prices API.
- The existing HTTP client will be updated to use the new filter builder instead of manually constructing filter strings.
- The filter builder is a pure data transformation (input criteria to output string) with no I/O, network, or side effects.
- Filter field ordering in the output does not affect API query results (field order is deterministic for testability, not for API correctness).
- **Behavioral change**: After integration (Phase 6), all queries via `GetPrices()` will include `priceType eq 'Consumption'` by default. The existing `buildFilterQuery()` does not include any pricing type filter. This is an intentional improvement — without a type filter, queries return mixed Consumption/Reservation/DevTest results, which can produce incorrect cost estimates. Existing tests must be updated to expect the new default filter.
