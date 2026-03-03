# Pagination Contract

**Feature**: `010-pagination-handler`

## API Surface

No new public API surface is introduced. The pagination handler is
an internal behavior change to the existing `GetPrices()` method.

### Existing Method (unchanged signature)

```go
func (c *Client) GetPrices(
    ctx context.Context,
    query PriceQuery,
) ([]PriceItem, error)
```

### Behavioral Contract Changes

#### Page Limit

- **Before**: Max 1000 pages per query
- **After**: Max 10 pages per query (~1,000 items)
- **Error**: Returns `ErrPaginationLimitExceeded` when limit exceeded
- **Detection**: `errors.Is(err, azureclient.ErrPaginationLimitExceeded)`

#### Pagination Progress Logging

- **Level**: Debug
- **Message**: `"pagination progress"`
- **Fields**:
  - `page` (int): Current page number (1-indexed in logs)
  - `items_this_page` (int): Items returned in current page
  - `total_items` (int): Cumulative items across all pages so far
- **Condition**: Emitted for each page after the first (page >= 1)

### Existing Sentinel Errors (unchanged)

```go
var ErrPaginationLimitExceeded  // pagination limit exceeded (10 pages)
var ErrNotFound                 // not found (zero results)
var ErrRateLimited              // rate limited (HTTP 429)
var ErrServiceUnavailable       // service unavailable (HTTP 503)
var ErrRequestFailed            // request failed (other errors)
var ErrInvalidResponse          // invalid API response
```

### Existing Constant (value change only)

```go
const MaxPaginationPages = 10  // was 1000
```
