# Quickstart: HTTP Client with Retry Logic

**Feature**: 006-http-client-retry
**Date**: 2026-02-03

## Overview

This feature adds an HTTP client for querying the Azure Retail Prices API with automatic retry logic for transient failures.

## Prerequisites

1. Go 1.25.5 or later
2. Project dependencies installed (`make ensure`)

## Installation

Add the dependency (if not already present):

```bash
go get github.com/hashicorp/go-retryablehttp@v0.7.7
```

## Basic Usage

### Creating a Client

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/rs/zerolog"
    "github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func main() {
    // Create a logger
    logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

    // Create client with default configuration
    config := azureclient.DefaultConfig()
    config.Logger = logger

    client, err := azureclient.NewClient(config)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    // Query pricing
    ctx := context.Background()
    query := azureclient.PriceQuery{
        ArmRegionName: "eastus",
        ArmSkuName:    "Standard_B1s",
        CurrencyCode:  "USD",
    }

    prices, err := client.GetPrices(ctx, query)
    if err != nil {
        log.Fatalf("Failed to get prices: %v", err)
    }

    for _, price := range prices {
        log.Printf("SKU: %s, Price: %.4f %s/%s",
            price.SkuName, price.RetailPrice, price.CurrencyCode, price.UnitOfMeasure)
    }
}
```

### Custom Configuration

```go
config := azureclient.Config{
    BaseURL:      "https://prices.azure.com/api/retail/prices",
    RetryMax:     5,                    // More retries for unreliable networks
    RetryWaitMin: 2 * time.Second,      // Start backoff at 2s
    RetryWaitMax: 60 * time.Second,     // Allow longer backoff
    Timeout:      120 * time.Second,    // Longer timeout for slow networks
    Logger:       logger,
}

client, err := azureclient.NewClient(config)
```

### Handling Errors

```go
prices, err := client.GetPrices(ctx, query)
if err != nil {
    switch {
    case errors.Is(err, context.Canceled):
        log.Println("Request was cancelled")
    case errors.Is(err, context.DeadlineExceeded):
        log.Println("Request timed out")
    default:
        log.Printf("Request failed: %v", err)
    }
    return
}
```

## Testing

### Unit Tests

Run unit tests with race detector:

```bash
go test -race ./internal/azureclient/...
```

### Integration Tests

Run integration tests against live Azure API:

```bash
go test -tags=integration ./examples/...
```

## Common Queries

### Get VM Pricing for a Region

```go
query := azureclient.PriceQuery{
    ArmRegionName: "eastus",
    ServiceName:   "Virtual Machines",
    CurrencyCode:  "USD",
}
```

### Get Specific SKU Pricing

```go
query := azureclient.PriceQuery{
    ArmSkuName:   "Standard_D2s_v3",
    CurrencyCode: "USD",
}
```

### Get Storage Pricing

```go
query := azureclient.PriceQuery{
    ServiceName:  "Storage",
    ArmRegionName: "westus2",
    CurrencyCode: "USD",
}
```

## Retry Behavior

The client automatically retries on:
- HTTP 429 (Too Many Requests) - respects Retry-After header
- HTTP 503 (Service Unavailable)
- Network errors (connection refused, timeout, DNS failure)

The client does NOT retry on:
- HTTP 4xx errors (except 429) - client errors
- HTTP 5xx errors (except 503) - persistent server errors
- Context cancellation

## Logging

Retry events are logged with structured JSON via zerolog:

```json
{
  "level": "warn",
  "time": "2026-02-03T10:15:30Z",
  "message": "retrying request",
  "attempt": 2,
  "max_attempts": 4,
  "status_code": 429,
  "retry_after": "5s",
  "url": "https://prices.azure.com/api/retail/prices?$filter=..."
}
```

## Troubleshooting

### "context deadline exceeded"

The request took longer than the configured timeout. Options:
1. Increase `Config.Timeout`
2. Use a more specific query (fewer results = faster)
3. Check network connectivity to Azure

### "exhausted retries"

All retry attempts failed. Possible causes:
1. Azure API is experiencing extended outage
2. Rate limiting is too aggressive - wait and retry later
3. Network issues between your server and Azure

### Empty results

The query returned no matching prices. Check:
1. Region name is valid (use Azure region names like "eastus", not display names)
2. SKU name is exact match (case-sensitive)
3. The resource type is available in the selected region
