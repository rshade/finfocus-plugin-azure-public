# Quickstart: Cost Calculation Utilities

**Branch**: `019-cost-utilities` | **Date**: 2026-03-03

## Usage

### Import

```go
import "github.com/rshade/finfocus-plugin-azure-public/internal/estimation"
```

### Convert Hourly Rates

```go
// VM at $0.10/hr
monthly := estimation.HourlyToMonthly(0.10)  // 73.00
yearly := estimation.HourlyToYearly(0.10)    // 876.00
```

### Convert Monthly Rates

```go
// Managed disk at $5.00/mo → normalize to hourly first
hourly := estimation.MonthlyToHourly(5.00)    // 0.01
monthly := estimation.HourlyToMonthly(hourly) // 5.00 (round-trip)
```

### Use Constants

```go
// Access the multipliers directly if needed
fmt.Println(estimation.HoursPerMonth) // 730
fmt.Println(estimation.HoursPerYear)  // 8760
```

## Constants Reference

| Constant        | Value | Meaning                              |
|-----------------|-------|--------------------------------------|
| `HoursPerMonth` | 730   | Average hours in a month (365*24/12) |
| `HoursPerYear`  | 8760  | Hours in a year (365*24)             |

## Integration with Azure Client

```go
// Fetch pricing and convert
prices, err := client.GetPrices(ctx, query)
if err != nil {
    return err
}

for _, item := range prices {
    monthly := estimation.HourlyToMonthly(item.RetailPrice)
    yearly := estimation.HourlyToYearly(item.RetailPrice)
    fmt.Printf("%s: $%.2f/hr, $%.2f/mo, $%.2f/yr\n",
        item.SkuName, item.RetailPrice, monthly, yearly)
}
```
