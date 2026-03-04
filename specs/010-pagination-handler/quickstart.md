# Quickstart: Pagination Handler

**Feature**: `010-pagination-handler`

## Usage

Pagination is fully automatic. No caller changes are needed.

```go
config := azureclient.DefaultConfig()
config.Logger = logger // Enable to see pagination progress at debug
client, err := azureclient.NewClient(config)
if err != nil {
    log.Fatal(err)
}

// Queries that return multiple pages are handled automatically.
// Up to 10 pages (~1,000 items) are aggregated into a single slice.
prices, err := client.GetPrices(ctx, azureclient.PriceQuery{
    ServiceName:  "Virtual Machines",
    ArmSkuName:   "Standard_D2s_v3",
    CurrencyCode: "USD",
})
if err != nil {
    if errors.Is(err, azureclient.ErrPaginationLimitExceeded) {
        // Query returned too many results — narrow your filter
        log.Printf("too many results, refine query: %v", err)
    }
    return err
}

fmt.Printf("Found %d prices across all pages\n", len(prices))
```

## Pagination Behavior

| Scenario                      | Result                              |
|-------------------------------|-------------------------------------|
| Query matches <100 items      | Single page, no pagination          |
| Query matches 100-1000 items  | Multi-page, all items returned      |
| Query matches >1000 items     | Error: `ErrPaginationLimitExceeded` |

## Debug Logging

Enable debug-level logging to see pagination progress:

```text
{"level":"debug","page":1,"items_this_page":100,"total_items":200,
 "message":"pagination progress"}
{"level":"debug","page":2,"items_this_page":100,"total_items":300,
 "message":"pagination progress"}
```

## Testing

```bash
# Unit tests (includes pagination tests)
make test

# Integration tests (queries live Azure API)
go test -tags=integration ./examples/...
```
