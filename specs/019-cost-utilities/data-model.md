# Data Model: Cost Calculation Utilities

**Branch**: `019-cost-utilities` | **Date**: 2026-03-03

## Entities

### Constants

| Name          | Value | Derivation    | Purpose                                |
|---------------|-------|---------------|----------------------------------------|
| HoursPerMonth | 730   | 365 * 24 / 12 | Hourly ↔ monthly conversion multiplier |
| HoursPerYear  | 8760  | 365 * 24      | Hourly → yearly conversion multiplier  |

### Functions

| Function        | Input           | Output  | Formula                |
|-----------------|-----------------|---------|------------------------|
| HourlyToMonthly | hourly float64  | float64 | round(hourly * 730, 2) |
| HourlyToYearly  | hourly float64  | float64 | round(hourly * 8760, 2)|
| MonthlyToHourly | monthly float64 | float64 | round(monthly / 730, 2)|

### Internal Helper

| Function      | Input          | Output  | Formula                        |
|---------------|----------------|---------|--------------------------------|
| roundCurrency | amount float64 | float64 | math.Round(amount * 100) / 100 |

## Relationships

```text
azureclient.PriceItem.RetailPrice (float64, hourly)
    │
    ▼
estimation.HourlyToMonthly() ──► monthly cost (float64)
estimation.HourlyToYearly()  ──► yearly cost (float64)

azureclient.PriceItem.RetailPrice (float64, monthly for disks)
    │
    ▼
estimation.MonthlyToHourly() ──► hourly cost (float64)
    │
    ▼
estimation.HourlyToMonthly() ──► normalized monthly (float64)
estimation.HourlyToYearly()  ──► normalized yearly (float64)
```

## Validation Rules

- All inputs are `float64` — no validation needed (any float64 is valid)
- All outputs are rounded to exactly 2 decimal places
- Sign is preserved (negative inputs → negative outputs)
- Zero input → zero output (no special casing needed, math handles it)

## State Transitions

N/A — all functions are pure and stateless. No lifecycle management.
