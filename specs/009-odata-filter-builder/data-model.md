# Data Model: OData Filter Query Builder

**Feature Branch**: `009-odata-filter-builder`
**Date**: 2026-02-28

## Entities

### FilterCondition

A single OData equality condition pairing a field name with a value.

| Field   | Type     | Description          | Validation           |
|---------|----------|----------------------|----------------------|
| `Field` | `string` | OData field name     | Non-empty, non-ws    |
| `Value` | `string` | Filter value         | Non-empty, non-ws    |

**Purpose**: Immutable value type used as input to `Or()` groups
and internally by named convenience methods. Created via
package-level constructor functions.

**Constructors**:

| Function          | Field Mapping   | Example                    |
|-------------------|-----------------|----------------------------|
| `Region(v)`       | `armRegionName` | `Region("eastus")`         |
| `Service(v)`      | `serviceName`   | `Service("VMs")`           |
| `SKU(v)`          | `armSkuName`    | `SKU("Standard_B1s")`      |
| `PriceType(v)`    | `priceType`     | `PriceType("Consump.")`    |
| `ProductName(v)`  | `productName`   | `ProductName("VMs BS")`    |
| `CurrencyCode(v)` | `currencyCode`  | `CurrencyCode("USD")`      |
| `Condition(n,v)`  | any             | `Condition("m","B1s")`     |

### FilterBuilder

The composable query constructor. Accumulates filter criteria
and produces a formatted OData `$filter` expression.

| Field            | Type                  | Default |
|------------------|-----------------------|---------|
| `andConditions`  | `map[string]string`   | empty   |
| `orGroups`       | `[][]FilterCondition` | empty   |
| `typeValue`      | `*string`             | `nil`   |
| `genericFields`  | `[]FilterCondition`   | empty   |

**Field Descriptions**:

- `andConditions`: Named AND conditions (field to value,
  last-write-wins)
- `orGroups`: OR-grouped condition sets
- `typeValue`: Pricing type override (nil = default)
- `genericFields`: Arbitrary field conditions

**State Transitions**:

```text
                ┌──────────────┐
                │ Empty Builder │
                │ (default type │
                │  "Consumption"│
                │  pending)     │
                └──────┬───────┘
                       │
        ┌──────────────┼──────────────┐
        │              │              │
        ▼              ▼              ▼
 Named Method     Or() Method    Field() Method
 (AND condition)  (OR group)     (generic AND)
        │              │              │
        ▼              ▼              ▼
 ┌──────────────────────────────────────┐
 │         Accumulating Builder          │
 │  (more methods can be chained)        │
 └──────────────────┬───────────────────┘
                    │
                    ▼ Build()
 ┌──────────────────────────────────────┐
 │         Filter String Output          │
 │  (immutable, deterministic)           │
 └──────────────────────────────────────┘
```

**Invariants**:

- `Build()` is idempotent: calling it multiple times produces
  the same string
- Named method calls for the same field overwrite
  (last-write-wins)
- OR groups are accumulated, never overwritten
- Empty/whitespace values are silently omitted at `Build()` time
- Default `priceType eq 'Consumption'` is appended if
  `typeValue` is nil

### Filter Expression (Output)

The string output of `Build()`. Not a stored entity — it is a
computed value.

**Format**: OData-compatible `$filter` string.

**Structure**:

```text
<part1> and <part2> and ... and <partN>
```

Where each part is either:

- A single AND condition: `fieldName eq 'value'`
- A parenthesized OR group:
  `(fieldName eq 'v1' or fieldName eq 'v2')`

**Ordering**: All parts sorted alphabetically by primary field
name (first condition's field name for OR groups). This ensures
FR-010 deterministic output.

## Relationships

```text
FilterCondition ──▶ FilterBuilder.Or()
FilterCondition ──▶ Region(), Service(), SKU(), etc.
FilterBuilder   ──▶ Filter Expression (string)
FilterBuilder   ──▶ buildFilterQuery() in client.go
PriceQuery      ──▶ FilterBuilder calls
```

## Validation Rules

| Rule                  | Applies To        | Behavior             |
|-----------------------|-------------------|----------------------|
| Empty string value    | All methods       | Silently omitted     |
| Whitespace-only value | All methods       | Silently omitted     |
| Empty field name      | `Field()`         | Silently omitted     |
| Single quote in value | All methods       | Escaped to `''`      |
| Duplicate named field | Named methods     | Last value wins      |
| Duplicate generic     | `Field()`         | Both included        |
| `Type("")`            | `Type()`          | Default still applies|
| No criteria at all    | `Build()`         | Returns default type |
