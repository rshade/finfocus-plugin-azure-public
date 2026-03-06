# Quickstart: Managed Disk Cost Estimation

**Feature Branch**: `021-disk-cost-estimation`

## Estimate a Premium SSD Disk Cost

```go
import (
    "context"
    finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
    "google.golang.org/protobuf/types/known/structpb"
)

attrs, err := structpb.NewStruct(map[string]any{
    "location":  "eastus",
    "disk_type": "Premium_SSD_LRS",
    "size_gb":   128,
})
if err != nil {
    return err
}

resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
    ResourceType: "azure:storage/managedDisk:ManagedDisk",
    Attributes:   attrs,
})
if err != nil {
    return err // gRPC codes: InvalidArgument, NotFound, Unimplemented, etc.
}

// Key response fields
resp.GetCurrency()        // "USD"
resp.GetCostMonthly()     // e.g., 19.71 (monthly cost for P10 tier)
resp.GetPricingCategory() // FOCUS_PRICING_CATEGORY_STANDARD
```

## Estimate a Standard HDD Disk Cost

```go
attrs, _ := structpb.NewStruct(map[string]any{
    "location":  "westus2",
    "disk_type": "Standard_LRS",
    "size_gb":   256,
})

resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
    ResourceType: "azure:storage/managedDisk:ManagedDisk",
    Attributes:   attrs,
})
// resp.GetCostMonthly() returns S15 tier price (256 GB, ceiling match)
```

## Supported Disk Types

| disk_type         | Description                  |
| ----------------- | ---------------------------- |
| Standard_LRS      | Standard HDD (LRS)           |
| StandardSSD_LRS   | Standard SSD (LRS)           |
| Premium_SSD_LRS   | Premium SSD (LRS)            |
| Standard_ZRS      | Standard HDD (ZRS)           |
| StandardSSD_ZRS   | Standard SSD (ZRS)           |
| Premium_ZRS       | Premium SSD (ZRS)            |

## Size-to-Tier Mapping

The system uses ceiling-match: your `size_gb` maps to the smallest tier >= that size.

| Request size_gb | Matched Tier | Billed Capacity |
| --------------- | ------------ | --------------- |
| 32              | x4           | 32 GiB          |
| 100             | x10          | 128 GiB         |
| 128             | x10          | 128 GiB         |
| 200             | x20          | 512 GiB         |
| 1024            | x30          | 1024 GiB        |

(Where `x` = S, E, or P depending on disk type)

## Error Handling

```go
import "google.golang.org/grpc/status"

resp, err := calc.EstimateCost(ctx, req)
if err != nil {
    st, _ := status.FromError(err)
    switch st.Code() {
    case codes.InvalidArgument:
        // Missing fields, unsupported disk type, or invalid size_gb
    case codes.NotFound:
        // No pricing found for region/type/size combination
    case codes.Unimplemented:
        // Unsupported resource type or no client configured
    }
}
```
