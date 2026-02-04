# Quickstart: Azure Retail Prices API Data Models

**Feature**: 007-azure-price-models
**Package**: `github.com/rshade/finfocus-plugin-azure-public/internal/azureclient`

## Overview

The `azureclient` package provides Go structs for working with the Azure Retail Prices API. These models enable type-safe JSON parsing of API responses.

## Basic Usage

### Querying Prices

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

func main() {
    // Create client with default configuration
    config := azureclient.DefaultConfig()
    client, err := azureclient.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Query prices for a specific VM SKU
    query := azureclient.PriceQuery{
        ArmRegionName: "eastus",
        ArmSkuName:    "Standard_B1s",
        ServiceName:   "Virtual Machines",
        CurrencyCode:  "USD",
    }

    prices, err := client.GetPrices(context.Background(), query)
    if err != nil {
        log.Fatal(err)
    }

    // Access pricing data with type safety
    for _, item := range prices {
        fmt.Printf("SKU: %s\n", item.SkuName)
        fmt.Printf("Region: %s (%s)\n", item.Location, item.ArmRegionName)
        fmt.Printf("Price: %.4f %s per %s\n",
            item.RetailPrice,
            item.CurrencyCode,
            item.UnitOfMeasure)
        fmt.Printf("Type: %s\n", item.Type)
        fmt.Println("---")
    }
}
```

### Working with Response Envelope

```go
import (
    "encoding/json"
    "fmt"
)

// Parse a raw API response
func parseResponse(jsonData []byte) (*azureclient.PriceResponse, error) {
    var response azureclient.PriceResponse
    if err := json.Unmarshal(jsonData, &response); err != nil {
        return nil, err
    }

    fmt.Printf("Currency: %s\n", response.BillingCurrency)
    fmt.Printf("Items: %d\n", response.Count)

    if response.NextPageLink != "" {
        fmt.Printf("More pages available: %s\n", response.NextPageLink)
    }

    return &response, nil
}
```

### Handling Different Pricing Types

```go
func categorizePrices(items []azureclient.PriceItem) {
    var consumption, reservation []azureclient.PriceItem

    for _, item := range items {
        switch item.Type {
        case "Consumption":
            consumption = append(consumption, item)
        case "Reservation":
            reservation = append(reservation, item)
            // Access reservation-specific field
            fmt.Printf("Reservation term: %s\n", item.ReservationTerm)
        }
    }

    fmt.Printf("Consumption prices: %d\n", len(consumption))
    fmt.Printf("Reservation prices: %d\n", len(reservation))
}
```

### Creating Test Data

```go
func createMockPriceItem() azureclient.PriceItem {
    return azureclient.PriceItem{
        // Pricing
        RetailPrice:      0.0104,
        UnitPrice:        0.0104,
        TierMinimumUnits: 0,
        CurrencyCode:     "USD",

        // Resource
        ArmRegionName: "eastus",
        Location:      "US East",
        ArmSkuName:    "Standard_B1s",
        SkuName:       "B1s",
        SkuID:         "DZH318Z0BQPS/00TG",

        // Service
        ServiceName:   "Virtual Machines",
        ServiceID:     "DZH313Z7MMC8",
        ServiceFamily: "Compute",
        ProductName:   "Virtual Machines BS Series",
        ProductID:     "DZH318Z0BQPS",

        // Meter
        MeterID:              "000a794b-bdb0-58be-a0cd-0c3a0f222923",
        MeterName:            "B1s",
        UnitOfMeasure:        "1 Hour",
        IsPrimaryMeterRegion: true,

        // Temporal
        EffectiveStartDate: "2020-08-01T00:00:00Z",

        // Type
        Type: "Consumption",
    }
}

func createMockResponse() azureclient.PriceResponse {
    return azureclient.PriceResponse{
        BillingCurrency:    "USD",
        CustomerEntityID:   "Default",
        CustomerEntityType: "Retail",
        Items: []azureclient.PriceItem{
            createMockPriceItem(),
        },
        NextPageLink: "",
        Count:        1,
    }
}
```

### JSON Round-Trip

```go
import (
    "encoding/json"
    "fmt"
)

func roundTripDemo() {
    // Create a price item
    original := createMockPriceItem()

    // Marshal to JSON
    jsonBytes, err := json.MarshalIndent(original, "", "  ")
    if err != nil {
        panic(err)
    }
    fmt.Printf("JSON:\n%s\n", string(jsonBytes))

    // Unmarshal back to struct
    var parsed azureclient.PriceItem
    if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
        panic(err)
    }

    // Verify round-trip
    if original.RetailPrice == parsed.RetailPrice &&
       original.ArmRegionName == parsed.ArmRegionName {
        fmt.Println("Round-trip successful!")
    }
}
```

## Common Patterns

### Filter by Service Family

```go
func filterByServiceFamily(items []azureclient.PriceItem, family string) []azureclient.PriceItem {
    var filtered []azureclient.PriceItem
    for _, item := range items {
        if item.ServiceFamily == family {
            filtered = append(filtered, item)
        }
    }
    return filtered
}

// Usage
computePrices := filterByServiceFamily(prices, "Compute")
storagePrices := filterByServiceFamily(prices, "Storage")
```

### Calculate Monthly Cost

```go
func calculateMonthlyCost(item azureclient.PriceItem, hoursPerMonth float64) float64 {
    // Only for hourly pricing
    if item.UnitOfMeasure != "1 Hour" {
        return 0 // Handle other units differently
    }
    return item.RetailPrice * hoursPerMonth
}

// Usage: 730 hours in a typical month
monthlyCost := calculateMonthlyCost(vmPrice, 730)
fmt.Printf("Estimated monthly cost: $%.2f\n", monthlyCost)
```

### Find Cheapest Option

```go
func findCheapestByRegion(items []azureclient.PriceItem) map[string]azureclient.PriceItem {
    cheapest := make(map[string]azureclient.PriceItem)

    for _, item := range items {
        if item.Type != "Consumption" {
            continue // Only compare consumption pricing
        }

        existing, found := cheapest[item.ArmRegionName]
        if !found || item.RetailPrice < existing.RetailPrice {
            cheapest[item.ArmRegionName] = item
        }
    }

    return cheapest
}
```

## Field Reference

| Field | Description | Example |
|-------|-------------|---------|
| `RetailPrice` | List price per unit | `0.0104` |
| `CurrencyCode` | ISO currency | `"USD"` |
| `ArmRegionName` | Region identifier | `"eastus"` |
| `Location` | Display name | `"US East"` |
| `SkuName` | SKU display name | `"B1s"` |
| `ArmSkuName` | ARM SKU identifier | `"Standard_B1s"` |
| `ServiceName` | Azure service | `"Virtual Machines"` |
| `ServiceFamily` | Category | `"Compute"` |
| `UnitOfMeasure` | Billing unit | `"1 Hour"` |
| `Type` | Pricing model | `"Consumption"` |
| `ReservationTerm` | Commitment (if reservation) | `"1 Year"` |

## Error Handling

```go
prices, err := client.GetPrices(ctx, query)
if err != nil {
    switch {
    case errors.Is(err, azureclient.ErrRateLimited):
        // Handle rate limiting
        log.Println("Rate limited, retry later")
    case errors.Is(err, azureclient.ErrServiceUnavailable):
        // Handle service unavailability
        log.Println("Azure API unavailable")
    case errors.Is(err, azureclient.ErrInvalidResponse):
        // Handle malformed response
        log.Println("Invalid API response")
    default:
        // Handle other errors
        log.Printf("Error: %v\n", err)
    }
    return
}
```
