# Quickstart: OData Filter Query Builder

## Import

```go
import (
    "github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)
```

## Basic Usage — AND Filters (Most Common)

Build a filter for Virtual Machines pricing in East US:

```go
filter := azureclient.NewFilterBuilder().
    Region("eastus").
    Service("Virtual Machines").
    SKU("Standard_B1s").
    Build()
// Result (fields sorted alphabetically, default type included):
// armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s'
//   and priceType eq 'Consumption'
//   and serviceName eq 'Virtual Machines'
```

## Override Default Pricing Type

Query reservation pricing instead of pay-as-you-go:

```go
filter := azureclient.NewFilterBuilder().
    Region("eastus").
    Service("Virtual Machines").
    Type("Reservation").
    Build()
// Result:
// armRegionName eq 'eastus'
//   and priceType eq 'Reservation'
//   and serviceName eq 'Virtual Machines'
```

## OR Groups — Multiple Regions

Query pricing across multiple regions:

```go
filter := azureclient.NewFilterBuilder().
    Or(
        azureclient.Region("eastus"),
        azureclient.Region("westus2"),
    ).
    Service("Virtual Machines").
    Build()
// Result:
// (armRegionName eq 'eastus' or armRegionName eq 'westus2')
//   and priceType eq 'Consumption'
//   and serviceName eq 'Virtual Machines'
```

## Generic Fields

Filter on any OData field not covered by named methods:

```go
filter := azureclient.NewFilterBuilder().
    Region("eastus").
    Field("meterName", "B1s Low Priority").
    Build()
// Result:
// armRegionName eq 'eastus'
//   and meterName eq 'B1s Low Priority'
//   and priceType eq 'Consumption'
```

## Special Characters — Automatic Escaping

Values with single quotes are escaped per OData conventions:

```go
filter := azureclient.NewFilterBuilder().
    Field("productName", "SQL Server O'Brien Edition").
    Build()
// Result:
// priceType eq 'Consumption'
//   and productName eq 'SQL Server O''Brien Edition'
// Single quotes are doubled: O'Brien → O''Brien
```

## Minimal Filter — No Criteria

When no criteria are set, returns only the default pricing type:

```go
filter := azureclient.NewFilterBuilder().Build()
// Result: "priceType eq 'Consumption'"
```

## Package-Level Constructors

Use package-level functions to create `FilterCondition` values
for `Or()`:

| Function          | OData Field     | Example                   |
|-------------------|-----------------|---------------------------|
| `Region(v)`       | `armRegionName` | `Region("eastus")`        |
| `Service(v)`      | `serviceName`   | `Service("VMs")`          |
| `SKU(v)`          | `armSkuName`    | `SKU("Standard_B1s")`     |
| `PriceType(v)`    | `priceType`     | `PriceType("Res.")`       |
| `ProductName(v)`  | `productName`   | `ProductName("VMs BS")`   |
| `CurrencyCode(v)` | `currencyCode`  | `CurrencyCode("USD")`     |
| `Condition(n,v)`  | any             | `Condition("m","B")`      |

## Integration with Existing HTTP Client

The existing `GetPrices(PriceQuery)` method uses the builder
internally. No changes needed for current callers:

```go
query := azureclient.PriceQuery{
    ArmRegionName: "eastus",
    ArmSkuName:    "Standard_B1s",
}
prices, err := client.GetPrices(ctx, query)
// Internally uses FilterBuilder now.
// Includes default priceType eq 'Consumption' filter.
```
