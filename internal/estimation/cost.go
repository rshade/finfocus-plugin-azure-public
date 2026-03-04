// Package estimation provides cost conversion utilities for transforming
// between hourly, monthly, and yearly pricing rates. All conversions use
// industry-standard multipliers aligned with Azure, AWS, and GCP pricing
// conventions. Results are rounded to two decimal places for currency precision.
package estimation

import "math"

// HoursPerMonth is the industry-standard average number of hours in a month
// (365 * 24 / 12 = 730), used by Azure, AWS, and GCP for pricing calculations.
const HoursPerMonth = 730

// HoursPerYear is the number of hours in a non-leap year (365 * 24 = 8760),
// used for annualizing hourly cloud pricing rates.
const HoursPerYear = 8760

// centsFactor is the multiplier for converting dollars to cents (two decimal
// places) used by roundCurrency.
const centsFactor = 100

// roundCurrency rounds a float64 to exactly two decimal places using standard
// arithmetic rounding (round half up), matching cloud billing display conventions.
func roundCurrency(amount float64) float64 {
	return math.Round(amount*centsFactor) / centsFactor
}

// HourlyToMonthly converts an hourly rate to a monthly cost estimate using the
// industry-standard 730 hours per month (365 * 24 / 12). The result is rounded
// to two decimal places.
func HourlyToMonthly(hourly float64) float64 {
	return roundCurrency(hourly * HoursPerMonth)
}

// HourlyToYearly converts an hourly rate to a yearly cost estimate using
// 8760 hours per year (365 * 24). The result is rounded to two decimal places.
func HourlyToYearly(hourly float64) float64 {
	return roundCurrency(hourly * HoursPerYear)
}

// MonthlyToHourly converts a monthly rate to an hourly rate by dividing by the
// industry-standard 730 hours per month (365 * 24 / 12). The result is rounded
// to two decimal places.
func MonthlyToHourly(monthly float64) float64 {
	return roundCurrency(monthly / HoursPerMonth)
}
