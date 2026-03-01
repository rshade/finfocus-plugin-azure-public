# Research: OData Filter Query Builder

**Feature Branch**: `009-odata-filter-builder`
**Date**: 2026-02-28

## Research Tasks

### RT-001: OData v4 Filter Syntax Conventions

**Decision**: Use standard OData v4 `$filter` expression syntax with lowercase
operators (`and`, `or`, `not`) and single-quoted string values.

**Rationale**: The Azure Retail Prices API uses OData v4, which is the standard
for Microsoft REST APIs. The syntax is well-documented and widely supported.

**Key Rules**:

- String values are delimited by single quotes: `field eq 'value'`
- Single quotes within values are escaped by doubling: `O'Brien` →
  `'O''Brien'`
- Logical operators are lowercase: `and`, `or`, `not`
- Comparison operators: `eq`, `ne`, `gt`, `lt`, `ge`, `le`
- Operator precedence (highest to lowest): `not` > comparisons > `and` > `or`
- Parentheses override precedence: `(a or b) and c`
- Field names are unquoted and case-sensitive (API version 2023-01-01-preview+)

**Alternatives Considered**:

- Custom query syntax → Rejected: OData is the API standard; inventing syntax
  adds complexity without benefit
- URL-encoded key-value pairs → Rejected: Doesn't support complex boolean logic

### RT-002: Azure Retail Prices API Filter Fields

**Decision**: Support all documented `$filter` fields plus `currencyCode` for
backward compatibility.

**Rationale**: The API documents specific fields as filterable. The builder
should provide named methods for the most common ones and a generic `Field()`
method for the rest.

**Supported `$filter` Fields** (from Microsoft documentation):

| Field            | Named Method       | Notes                |
|------------------|--------------------|----------------------|
| `armRegionName`  | `Region()`         | ARM region ID        |
| `serviceName`    | `Service()`        | Service name         |
| `armSkuName`     | `SKU()`            | ARM SKU name         |
| `productName`    | `ProductName()`    | Full product name    |
| `priceType`      | `Type()`           | Pricing type         |
| `currencyCode`   | `CurrencyCode()`   | See note below       |
| Any other        | `Field(name, val)` | Generic method       |

**CurrencyCode Note**: The Azure documentation specifies `currencyCode` as a URL
query parameter (`?currencyCode=EUR`), not a `$filter` field. However, the
existing codebase uses it within the filter expression, and integration tests
pass. The builder will support it as a filter criterion for backward
compatibility. The HTTP client layer can optionally extract it to a query
parameter in a future iteration.

**Additional filterable fields** (available via `Field()` method): `Location`,
`meterId`, `meterName`, `productid`, `skuId`, `skuName`, `serviceId`,
`serviceFamily`.

**Alternatives Considered**:

- Only support the 4 fields from the original issue → Rejected: spec clarified
  all 6 named methods plus generic Field()
- Remove currencyCode from filter → Rejected: breaks backward compatibility with
  existing `PriceQuery` behavior

### RT-003: Default Pricing Type ("Consumption")

**Decision**: The builder will always include `priceType eq 'Consumption'` unless
explicitly overridden via `Type()`.

**Rationale**: Without a default pricing type, queries return mixed results
including reservation and dev/test pricing, leading to incorrect cost estimates.
"Consumption" represents standard pay-as-you-go pricing, which is the most
commonly needed tier.

**Valid priceType values**:

- `Consumption` - pay-as-you-go (default)
- `Reservation` - reserved instances
- `DevTestConsumption` - dev/test pricing

**Behavioral Change**: The existing `buildFilterQuery()` does NOT add a default
type filter. Adding it changes the result set returned by `GetPrices()`. This is
intentional per FR-003 and narrows results to only pay-as-you-go pricing.

**Alternatives Considered**:

- No default type → Rejected: spec requires it (FR-003)
- Default to empty (no type filter) → Rejected: mixed results cause incorrect
  cost estimates

### RT-004: AND/OR Logic and Grouping Design

**Decision**: Use a dual-API approach: named methods add individual AND
conditions; `Or()` method creates parenthesized OR groups from `FilterCondition`
values.

**Rationale**: The most common case (AND-only) should be simple. OR is less
common and benefits from explicit grouping semantics. Package-level constructor
functions (`Region()`, `Service()`, etc.) return `FilterCondition` values for use
with `Or()`.

**API Design**:

```go
// Simple AND (most common):
filter := azureclient.NewFilterBuilder().
    Region("eastus").
    Service("Virtual Machines").
    Build()

// OR group with AND:
filter := azureclient.NewFilterBuilder().
    Or(azureclient.Region("eastus"), azureclient.Region("westus2")).
    Service("Virtual Machines").
    Build()
// → "(armRegionName eq 'eastus' or armRegionName eq 'westus2') and ..."
```

**Precedence Handling**: Since `and` binds tighter than `or` in OData, OR groups
are always parenthesized when the final expression contains both AND and OR. This
prevents ambiguity regardless of the number of conditions.

**Alternatives Considered**:

- Single `Add(operator, conditions...)` method → Rejected: less readable,
  violates fluent API requirement
- Separate `AndBuilder`/`OrBuilder` types → Rejected: over-engineered for the
  scope
- Callback-based grouping (`OrGroup(func(b))`) → Rejected: more complex API,
  harder to compose

### RT-005: Deterministic Output Ordering

**Decision**: Sort all top-level filter parts alphabetically by their primary
field name. For OR groups, the primary field is the first condition's field name.

**Rationale**: Alphabetical sorting by field name is simple, predictable, and
independent of call order. Named Azure fields sort naturally:
`armRegionName` < `armSkuName` < `currencyCode` < `priceType` < `productName` <
`serviceName`.

**Alternatives Considered**:

- Canonical fixed order (region, service, sku, ...) → Rejected: harder to extend
  when generic fields are added
- Insertion order → Rejected: violates FR-010 (must be independent of call order)

### RT-006: Empty and Whitespace Value Handling

**Decision**: Empty strings and whitespace-only strings are silently omitted from
the filter. No error is returned.

**Rationale**: Per FR-009, empty/whitespace values should not produce invalid
clauses. Silent omission is the least surprising behavior for a builder pattern.

**Edge Cases**:

- `Region("")` → omitted
- `Region("   ")` → omitted (whitespace-only)
- `Type("")` → omitted, default "Consumption" still applies
- `Type("   ")` → omitted, default "Consumption" still applies
- `Field("", "value")` → omitted (empty field name)
- `Field("name", "")` → omitted (empty value)

### RT-007: Same-Field Override Behavior

**Decision**: For named methods (AND conditions), the last value wins. For OR
groups, all conditions are preserved.

**Rationale**: Per the spec edge cases: "The last value wins (override
behavior)." This applies to named method calls. OR groups represent explicit
multi-value intent and should not be deduplicated.

**Example**:

```go
// Last value wins for named methods:
builder.Region("eastus").Region("westus2").Build()
// → "armRegionName eq 'westus2' and priceType eq 'Consumption'"

// OR groups are preserved:
builder.Or(Region("eastus"), Region("westus2")).Build()
// → "(armRegionName eq 'eastus' or armRegionName eq 'westus2')
//     and priceType eq 'Consumption'"
```

**No deduplication** between named conditions and OR groups (per spec: "The
caller is responsible for avoiding conflicts").

### RT-008: Integration with Existing HTTP Client

**Decision**: Refactor `buildFilterQuery()` in `client.go` to use
`FilterBuilder` internally. Keep `GetPrices(PriceQuery)` signature unchanged.

**Rationale**: This provides backward compatibility while adopting the new
builder. The `FilterBuilder` is also exported for direct use by callers who need
more control (OR logic, generic fields).

**Refactored `buildFilterQuery`**:

```go
func buildFilterQuery(query PriceQuery) string {
    return NewFilterBuilder().
        Region(query.ArmRegionName).
        Service(query.ServiceName).
        SKU(query.ArmSkuName).
        ProductName(query.ProductName).
        CurrencyCode(query.CurrencyCode).
        Build()
}
```

**Note**: This adds the default `priceType eq 'Consumption'` to all queries made
via `GetPrices()`, which is the intended behavioral change per FR-003.

**Alternatives Considered**:

- New `GetPricesWithFilter(*FilterBuilder)` method → Deferred: can be added in a
  future feature without changing the builder
- Replace `PriceQuery` with `FilterBuilder` in `GetPrices` → Rejected: breaking
  API change

## Unresolved Items

None. All NEEDS CLARIFICATION items have been resolved through spec analysis
and API documentation review.
