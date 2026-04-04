# Quickstart: GetProjectedCost RPC

**Branch**: `023-projected-cost-rpc` | **Date**: 2026-04-04

## Overview

`GetProjectedCost` returns the projected monthly cost for an Azure resource
based on current retail pricing. It uses the `ResourceDescriptor` proto
message to identify the resource and queries the Azure Retail Prices API
(via a cached client) for live pricing data.

## Usage (Go client)

```go
import (
    finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// Build the request
req := &finfocusv1.GetProjectedCostRequest{
    Resource: &finfocusv1.ResourceDescriptor{
        Provider:     "azure",
        ResourceType: "compute/VirtualMachine",
        Region:       "eastus",
        Sku:          "Standard_B1s",
    },
}

// Call the RPC
resp, err := client.GetProjectedCost(ctx, req)
if err != nil {
    // Handle gRPC status error
    st, _ := status.FromError(err)
    switch st.Code() {
    case codes.InvalidArgument:
        // Missing or invalid fields — check st.Message() for details
    case codes.Unimplemented:
        // Resource type not supported yet
    case codes.NotFound:
        // No pricing data for this region/SKU
    default:
        // Other error (rate limited, unavailable, etc.)
    }
    return err
}

// Access response fields
fmt.Printf("Monthly cost: %s %.2f\n", resp.GetCurrency(), resp.GetCostPerMonth())
fmt.Printf("Unit price:   %s %.4f/hr\n", resp.GetCurrency(), resp.GetUnitPrice())
fmt.Printf("Detail:       %s\n", resp.GetBillingDetail())
fmt.Printf("Category:     %s\n", resp.GetPricingCategory())
fmt.Printf("Expires at:   %s\n", resp.GetExpiresAt().AsTime())
```

## Supported Resource Types

| Resource Type             | Status        | Pricing Model         |
| ------------------------- | ------------- | --------------------- |
| compute/VirtualMachine    | Supported     | Hourly × 730 hrs/mo  |
| storage/ManagedDisk       | Planned       | Monthly retail price  |
| storage/BlobStorage       | Planned       | Per-GB monthly price  |

## Error Handling

All errors are returned as gRPC status codes with descriptive messages.
Missing fields are reported in a single error (e.g., "missing required
fields: region, sku").

## Caching

Responses include an `expires_at` timestamp indicating when the cached
pricing data will be refreshed. Cache TTL defaults to 24 hours and can
be configured via the `FINFOCUS_CACHE_TTL` environment variable.
