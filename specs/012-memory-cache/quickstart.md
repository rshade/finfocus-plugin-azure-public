# Quickstart: Thread-Safe In-Memory Cache

**Feature**: `012-memory-cache`
**Date**: 2026-03-02

## Overview

The in-memory cache wraps the Azure pricing client to avoid
redundant API calls. Cached results include `ExpiresAt`
metadata used to set `expires_at` on gRPC responses.

## Basic Usage

### Create a Cached Client

```go
package main

import (
    "time"

    "github.com/rs/zerolog"
    "github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func main() {
    logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

    // Create the underlying Azure API client
    clientConfig := azureclient.DefaultConfig()
    clientConfig.Logger = logger
    client, err := azureclient.NewClient(clientConfig)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to create client")
    }

    // Wrap with cache (1000 entries, 24h L1, 4h expires_at)
    cacheConfig := azureclient.DefaultCacheConfig()
    cacheConfig.Logger = logger
    cachedClient, err := azureclient.NewCachedClient(
        client, cacheConfig,
    )
    if err != nil {
        log.Fatal().Err(err).Msg("failed to create cache")
    }
    defer cachedClient.Close()
}
```

### Query with Cache

```go
query := azureclient.PriceQuery{
    ArmRegionName: "eastus",
    ArmSkuName:    "Standard_B1s",
    ServiceName:   "Virtual Machines",
    CurrencyCode:  "USD",
}

// First call: cache miss → Azure API call
result, err := cachedClient.GetPrices(ctx, query)
// result.Items = []PriceItem{...}
// result.ExpiresAt = ~4 hours from now (L2 hint)
// (L1 internal expiry = ~24 hours from now)

// Second call: cache hit → instant return
result2, err := cachedClient.GetPrices(ctx, query)
// result2.Items = same data
// result2.ExpiresAt = same original value (remaining L2 TTL)
```

### Set expires_at on gRPC Responses

```go
// In Calculator.GetProjectedCost:
result, err := c.cachedClient.GetPrices(ctx, query)
if err != nil {
    return nil, pricing.MapToGRPCStatus(err).Err()
}

resp := pluginsdk.NewGetProjectedCostResponse(
    pluginsdk.WithProjectedCostDetails(unitPrice, "USD", unitPrice*730, "azure-retail-prices"),
    pluginsdk.WithProjectedCostExpiresAt(result.ExpiresAt),
)
```

### Custom Configuration

```go
// Custom TTL and size
config := azureclient.CacheConfig{
    MaxSize:      500,
    TTL:          12 * time.Hour,   // L1 internal
    ExpiresAtTTL: 2 * time.Hour,    // L2 hint
    Logger:       logger,
}
cachedClient, err := azureclient.NewCachedClient(
    client, config,
)
```

## Cache Key Normalization

Cache keys are automatically normalized from PriceQuery
dimensions:

```go
// These produce the same cache key:
query1 := PriceQuery{ArmRegionName: "EastUS", ArmSkuName: "Standard_B1s"}
query2 := PriceQuery{ArmRegionName: "eastus", ArmSkuName: "standard_b1s"}

key := azureclient.CacheKey(query1)
// "eastus|standard_b1s|||"
```

## Observability

Cache statistics are logged automatically:

- **Debug level**: Individual cache hit/miss per query
- **Info level**: Hit/miss ratio summary every 1000 requests
  or 5 minutes

```json
{
  "level": "info",
  "message": "cache stats",
  "cache_hits": 950,
  "cache_misses": 50,
  "cache_hit_ratio": 0.95,
  "cache_size": 50,
  "time": "2026-03-02T10:00:00Z"
}
```

## Architecture

```text
GetProjectedCost(req)
  → Calculator.GetProjectedCost(query)
    → CachedClient.GetPrices(query)
      → CacheKey(query)           // normalize
      → cache.Get(key)            // check LRU
      → [hit]  return CachedResult
      → [miss] client.GetPrices() // Azure API
               cache.Add(key, result)
               return CachedResult
  → set response.expires_at = result.ExpiresAt
  → return resp
```
