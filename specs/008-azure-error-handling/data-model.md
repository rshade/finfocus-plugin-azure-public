# Data Model: Comprehensive Error Handling for Azure API Failures

**Feature Branch**: `008-azure-error-handling`
**Date**: 2026-02-09

## Entity Overview

This feature does not introduce new persistent data entities. It extends the
existing error handling model with additional sentinel errors and a mapping
function.

## Sentinel Errors (extended)

### Existing (no change)

| Sentinel                 | Package       | Purpose                            |
| ------------------------ | ------------- | ---------------------------------- |
| `ErrInvalidConfig`       | `azureclient` | Configuration validation failure   |
| `ErrInvalidResponse`     | `azureclient` | JSON parse failure                 |
| `ErrRateLimited`         | `azureclient` | HTTP 429 (retries disabled/direct) |
| `ErrServiceUnavailable`  | `azureclient` | HTTP 503 (retries disabled/direct) |
| `ErrRequestFailed`       | `azureclient` | Generic HTTP/network failure       |

### New

| Sentinel                     | Package       | Purpose                                            |
| ---------------------------- | ------------- | -------------------------------------------------- |
| `ErrNotFound`                | `azureclient` | HTTP 404 or empty result set                       |
| `ErrPaginationLimitExceeded` | `azureclient` | Moved from client.go to errors.go (consolidation)  |

## Client Struct Extension

### Current

```text
Client {
  httpClient *retryablehttp.Client
  baseURL    string
  userAgent  string
}
```

### After

```text
Client {
  httpClient *retryablehttp.Client
  baseURL    string
  userAgent  string
  logger     zerolog.Logger          # NEW: application-level logging
}
```

## Error Wrapping Pattern

Errors flow through two layers with progressive context enrichment:

```text
Layer 1: fetchPage (HTTP mechanics)
  - Produces: sentinel error + HTTP status + response body snippet
  - Example: "rate limited: status 429: {\"error\":\"too many requests\"}"

Layer 2: GetPrices (business logic)
  - Wraps Layer 1 errors with query context
  - Example: "query [region=eastus sku=Standard_B1s]: rate limited: status 429"
  - Also handles: empty results, pagination limit, structured logging
```

## gRPC Mapping Function

```text
MapToGRPCStatus(err error) -> (codes.Code, string)

Input:  Any error from azureclient
Output: gRPC status code + human-readable message

Mapping rules (evaluated in order):
  1. context.Canceled      -> codes.Canceled
  2. context.DeadlineExceeded -> codes.DeadlineExceeded
  3. ErrNotFound           -> codes.NotFound
  4. ErrRateLimited        -> codes.ResourceExhausted
  5. ErrServiceUnavailable -> codes.Unavailable
  6. ErrInvalidResponse    -> codes.Internal
  7. ErrRequestFailed      -> codes.Internal
  8. ErrInvalidConfig      -> codes.Internal
  9. ErrPaginationLimitExceeded -> codes.Internal
  10. default (unknown)    -> codes.Internal
```

## Structured Log Fields

All error log entries from `GetPrices` include these structured fields:

| Field              | Type   | Source                           | Example                               |
| ------------------ | ------ | -------------------------------- | ------------------------------------- |
| `region`           | string | PriceQuery                       | `"eastus"`                            |
| `sku`              | string | PriceQuery                       | `"Standard_B1s"`                      |
| `service`          | string | PriceQuery                       | `"Virtual Machines"`                  |
| `url`              | string | request URL                      | `"https://prices.azure.com/api/"`     |
| `error_category`   | string | sentinel error                   | `"rate_limited"`                      |
| `http_status`      | int    | HTTP response                    | `429`                                 |
| `page`             | int    | pagination counter               | `3`                                   |
| `response_snippet` | string | response body (JSON errors only) | `"<!DOCTYPE..."`                      |
