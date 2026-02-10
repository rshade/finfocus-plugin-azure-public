# Quickstart: Error Handling for Azure API Failures

**Feature Branch**: `008-azure-error-handling`

## Overview

This feature enhances the Azure client with comprehensive error handling.
After implementation, all errors from `GetPrices()` include query context
and can be classified programmatically.

## Error Classification

```go
import (
    "errors"
    "github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

prices, err := client.GetPrices(ctx, query)
if err != nil {
    switch {
    case errors.Is(err, azureclient.ErrNotFound):
        // No pricing data for this SKU/region
        // Safe to return "not found" to caller
    case errors.Is(err, azureclient.ErrRateLimited):
        // Azure API rate limited us (after retries exhausted)
        // Consider backing off at application level
    case errors.Is(err, azureclient.ErrServiceUnavailable):
        // Azure API temporarily down (after retries exhausted)
        // Retry later
    case errors.Is(err, azureclient.ErrRequestFailed):
        // Generic failure (network, unexpected HTTP status)
        // Log and return internal error
    case errors.Is(err, azureclient.ErrInvalidResponse):
        // Malformed JSON from Azure API
        // Log and return internal error
    default:
        // Unknown error
    }
}
```

## gRPC Status Mapping

```go
import (
    "github.com/rshade/finfocus-plugin-azure-public/internal/pricing"
)

prices, err := client.GetPrices(ctx, query)
if err != nil {
    // Convert to gRPC status for returning to FinFocus Core
    grpcStatus := pricing.MapToGRPCStatus(err)
    return nil, grpcStatus.Err()
}
```

## Structured Logging

Errors are automatically logged with structured fields when `GetPrices`
encounters a failure. Ensure the client is configured with a logger:

```go
config := azureclient.DefaultConfig()
config.Logger = logger // zerolog.Logger

client, err := azureclient.NewClient(config)
```

Log output example (JSON):

```json
{
  "level": "warn",
  "region": "eastus",
  "sku": "Standard_B1s",
  "service": "Virtual Machines",
  "error_category": "not_found",
  "message": "pricing query returned no results"
}
```

## Error Message Format

All errors include query context:

```text
query [region=eastus sku=Standard_B1s service=Virtual Machines]: not found: no pricing data
query [region=eastus sku=Standard_B1s]: request failed: status 400: invalid filter
query [region=eastus] page 3: rate limited: status 429: too many requests
```
