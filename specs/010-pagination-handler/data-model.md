# Data Model: Pagination Handler

**Feature**: `010-pagination-handler`
**Date**: 2026-03-01

## Entities

### Existing Entities (no changes)

#### PriceResponse

Envelope for a single API page. Already contains all required fields.

| Field | Type | Description |
| --- | --- | --- |
| BillingCurrency | string | Currency code (e.g., "USD") |
| CustomerEntityID | string | Customer entity identifier |
| CustomerEntityType | string | Pricing type ("Retail") |
| Items | []PriceItem | Price entries for this page |
| NextPageLink | string | URL for next page, empty if none |
| Count | int | Number of items in this page |

#### PriceItem

Individual price entry. No changes needed.

#### PriceQuery

Filter parameters. No changes needed.

#### Config

Client configuration. No changes needed — page limit is a constant,
not configurable.

### Modified Constants

| Constant | Current | New | Location |
| --- | --- | --- | --- |
| MaxPaginationPages | 1000 | 10 | errors.go |

### No New Entities

No new types, structs, or interfaces are introduced. The pagination
handler operates on existing data structures. The only data flow
change is adding structured log events during the pagination loop.

## State Transitions

```text
GetPrices() pagination flow:

  [Start] → Build initial URL from PriceQuery
    │
    ▼
  [Fetch Page N] → fetchPage(ctx, url)
    │
    ├── Error → Return (nil, wrapped error with page N)
    │
    ├── Success + NextPageLink present + page < limit
    │     │
    │     ├── Log progress (debug): page N, items, total
    │     │
    │     └── Append items → [Fetch Page N+1]
    │
    ├── Success + NextPageLink empty
    │     │
    │     └── Append items → [Check Results]
    │
    └── Success + page >= limit
          │
          └── Return (nil, ErrPaginationLimitExceeded)

  [Check Results]
    │
    ├── len(allItems) == 0 → Return (nil, ErrNotFound)
    │
    └── len(allItems) > 0 → Return (allItems, nil)
```
