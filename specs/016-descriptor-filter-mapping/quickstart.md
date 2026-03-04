# Quickstart: ResourceDescriptor to Azure Filter Mapping

## Overview

The mapper translates a `finfocusv1.ResourceDescriptor`
(from gRPC requests) into an `azureclient.PriceQuery`
(for Azure Retail Prices API lookups).

## Basic Usage

```go
import (
    finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/
proto/finfocus/v1"
    "github.com/rshade/finfocus-plugin-azure-public/
internal/pricing"
)

// Create a resource descriptor (from a gRPC request)
desc := &finfocusv1.ResourceDescriptor{
    Provider:     "azure",
    ResourceType: "compute/VirtualMachine",
    Sku:          "Standard_B1s",
    Region:       "eastus",
}

// Map to a pricing query
query, err := pricing.MapDescriptorToQuery(desc)
if err != nil {
    // handle error
}

// query.ArmRegionName == "eastus"
// query.ArmSkuName    == "Standard_B1s"
// query.ServiceName   == "Virtual Machines"
// query.CurrencyCode  == "USD"
```

## Tag Fallback

When primary fields are empty, tags are used as fallback:

```go
desc := &finfocusv1.ResourceDescriptor{
    Provider:     "azure",
    ResourceType: "storage/ManagedDisk",
    // Sku and Region empty — fall back to tags
    Tags: map[string]string{
        "region": "westus2",
        "sku":    "Premium_LRS",
    },
}

query, err := pricing.MapDescriptorToQuery(desc)
// query.ArmRegionName == "westus2"     (from tag)
// query.ArmSkuName    == "Premium_LRS" (from tag)
// query.ServiceName   == "Managed Disks"
```

## Supported Resource Types

```go
types := pricing.SupportedResourceTypes()
// ["compute/VirtualMachine",
//  "storage/BlobStorage",
//  "storage/ManagedDisk"]
```

| Resource Type | Azure Service Name |
|---|---|
| `compute/VirtualMachine` | Virtual Machines |
| `storage/ManagedDisk` | Managed Disks |
| `storage/BlobStorage` | Storage |

Resource type matching is case-insensitive.

## Error Handling

### Unsupported resource type

```go
desc := &finfocusv1.ResourceDescriptor{
    Provider:     "azure",
    ResourceType: "network/LoadBalancer",
}

_, err := pricing.MapDescriptorToQuery(desc)
// errors.Is(err, pricing.ErrUnsupportedResourceType)
// err.Error():
//   "unsupported resource type: network/LoadBalancer"
```

### Non-azure provider

```go
desc := &finfocusv1.ResourceDescriptor{
    Provider:     "aws",
    ResourceType: "compute/VirtualMachine",
}

_, err := pricing.MapDescriptorToQuery(desc)
// errors.Is(err, pricing.ErrUnsupportedResourceType)
// err.Error(): "unsupported provider: aws"
```

### Missing required fields

```go
desc := &finfocusv1.ResourceDescriptor{
    Provider:     "azure",
    ResourceType: "compute/VirtualMachine",
    // Region and Sku both missing
}

_, err := pricing.MapDescriptorToQuery(desc)
// errors.Is(err, pricing.ErrMissingRequiredFields)
// err.Error(): "missing required fields: region, sku"
```

## Integration with Calculator

The `Supports()` gRPC method uses the mapper to check if
a resource can be priced:

```go
func (c *Calculator) Supports(
    ctx context.Context,
    req *finfocusv1.SupportsRequest,
) (*finfocusv1.SupportsResponse, error) {
    _, err := pricing.MapDescriptorToQuery(
        req.GetResource(),
    )
    if err != nil {
        return &finfocusv1.SupportsResponse{
            Supported: false,
            Reason:    err.Error(),
        }, nil
    }
    return &finfocusv1.SupportsResponse{
        Supported: true,
    }, nil
}
```

## gRPC Error Mapping

Mapper errors integrate with `MapToGRPCStatus`:

| Mapper Error | gRPC Code |
|---|---|
| `ErrUnsupportedResourceType` | `codes.Unimplemented` |
| `ErrMissingRequiredFields` | `codes.InvalidArgument` |
