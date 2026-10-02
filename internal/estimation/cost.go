// Package estimation provides cost conversion utilities for transforming
// between hourly, monthly, and yearly pricing rates. All conversions use
// industry-standard multipliers aligned with Azure, AWS, and GCP pricing
// conventions. Results are rounded to two decimal places for currency precision.
package estimation

import (
	"fmt"
	"math"
	"strings"
)

// HoursPerMonth is the industry-standard average number of hours in a month
// (365 * 24 / 12 = 730), used by Azure, AWS, and GCP for pricing calculations.
const HoursPerMonth = 730

// HoursPerYear is the number of hours in a non-leap year (365 * 24 = 8760),
// used for annualizing hourly cloud pricing rates.
const HoursPerYear = 8760

// threeYearReservationHours is HoursPerYear times 3. A three-year
// Reservation retailPrice is a term total over that many hours.
const threeYearReservationHours = HoursPerYear * 3

// centsFactor is the multiplier for converting dollars to cents (two decimal
// places) used by roundCurrency.
const centsFactor = 100

// roundCurrency rounds a float64 to exactly two decimal places using standard
// arithmetic rounding (round half up), matching cloud billing display conventions.
func roundCurrency(amount float64) float64 {
	return math.Round(amount*centsFactor) / centsFactor
}

// HourlyToMonthly converts an hourly rate to a monthly cost estimate using the
// industry-standard 730 hours per month (365 * 24 / 12).
// HourlyToMonthly rounds that monthly cost to two decimal places.
func HourlyToMonthly(hourly float64) float64 {
	return roundCurrency(hourly * HoursPerMonth)
}

// HourlyToYearly converts an hourly rate to a yearly cost estimate using
// 8760 hours per year (365 * 24).
// HourlyToYearly rounds that yearly cost to two decimal places.
func HourlyToYearly(hourly float64) float64 {
	return roundCurrency(hourly * HoursPerYear)
}

// MonthlyToHourly converts a monthly rate to an hourly rate by dividing by the
// industry-standard 730 hours per month (365 * 24 / 12).
// MonthlyToHourly rounds that hourly rate to two decimal places.
func MonthlyToHourly(monthly float64) float64 {
	return roundCurrency(monthly / HoursPerMonth)
}

// SavingsFraction returns (onDemand - other) / onDemand without rounding.
// onDemand must be greater than zero. other must not be negative.
// SavingsFraction returns 1 when other is zero and is negative when other is greater.
func SavingsFraction(onDemand, other float64) (float64, error) {
	if onDemand <= 0 {
		return 0, fmt.Errorf("onDemand must be > 0, got %v", onDemand)
	}
	if other < 0 {
		return 0, fmt.Errorf("other must be >= 0, got %v", other)
	}
	return (onDemand - other) / onDemand, nil
}

// ReservationHourly converts a Reservation retailPrice into an hourly rate.
// The row's unitOfMeasure says 1 Hour, and the price is the term total.
// 1 Year divides by 8760. 3 Years divides by 26280.
func ReservationHourly(retail float64, term string) (float64, error) {
	if retail < 0 || math.IsNaN(retail) || math.IsInf(retail, 0) {
		return 0, fmt.Errorf("retail price must be >= 0, got %v", retail)
	}
	var hours float64
	switch strings.TrimSpace(term) {
	case "1 Year":
		hours = HoursPerYear
	case "3 Years":
		hours = threeYearReservationHours
	default:
		return 0, fmt.Errorf("unsupported reservation term %q", term)
	}
	return retail / hours, nil
}
