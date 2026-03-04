package estimation

import (
	"math"
	"testing"
)

func Test_roundCurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		amount float64
		want   float64
	}{
		{name: "exact value", amount: 73.00, want: 73.00},
		{name: "half-cent rounds up", amount: 0.105, want: 0.11},
		{name: "near-zero rounds to zero", amount: 0.004, want: 0.00},
		{name: "negative half-cent", amount: -0.105, want: -0.11},
		{name: "zero", amount: 0.00, want: 0.00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := roundCurrency(tt.amount)
			if got != tt.want {
				t.Errorf("roundCurrency(%v) = %v, want %v", tt.amount, got, tt.want)
			}
		})
	}
}

func TestHourlyToMonthly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		hourly float64
		want   float64
	}{
		{name: "standard rate", hourly: 0.10, want: 73.00},
		{name: "zero", hourly: 0.00, want: 0.00},
		{name: "negative", hourly: -0.10, want: -73.00},
		{name: "large rate", hourly: 10000.00, want: 7300000.00},
		{name: "small rate", hourly: 0.001, want: 0.73},
		{name: "rounding", hourly: 0.105, want: 76.65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := HourlyToMonthly(tt.hourly)
			if got != tt.want {
				t.Errorf("HourlyToMonthly(%v) = %v, want %v", tt.hourly, got, tt.want)
			}
		})
	}
}

func TestHourlyToYearly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		hourly float64
		want   float64
	}{
		{name: "standard rate", hourly: 0.10, want: 876.00},
		{name: "zero", hourly: 0.00, want: 0.00},
		{name: "negative", hourly: -0.10, want: -876.00},
		{name: "large rate", hourly: 10000.00, want: 87600000.00},
		{name: "rounding", hourly: 0.105, want: 919.80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := HourlyToYearly(tt.hourly)
			if got != tt.want {
				t.Errorf("HourlyToYearly(%v) = %v, want %v", tt.hourly, got, tt.want)
			}
		})
	}
}

func TestMonthlyToHourly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		monthly float64
		want    float64
	}{
		{name: "standard rate", monthly: 730.00, want: 1.00},
		{name: "zero", monthly: 0.00, want: 0.00},
		{name: "negative", monthly: -730.00, want: -1.00},
		{name: "fractional result", monthly: 5.00, want: 0.01},
		{name: "small rate rounds to zero", monthly: 0.73, want: 0.00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := MonthlyToHourly(tt.monthly)
			if got != tt.want {
				t.Errorf("MonthlyToHourly(%v) = %v, want %v", tt.monthly, got, tt.want)
			}
		})
	}
}

func TestRoundTripConsistency(t *testing.T) {
	t.Parallel()

	// Known inputs where MonthlyToHourly -> HourlyToMonthly should preserve value.
	knownMonthly := []float64{730.00, 365.00, 146.00, 73.00, 7.30}

	for _, monthly := range knownMonthly {
		hourly := MonthlyToHourly(monthly)
		roundTrip := HourlyToMonthly(hourly)

		if roundTrip != monthly {
			t.Errorf("round-trip failed for %v: MonthlyToHourly=%v, HourlyToMonthly=%v",
				monthly, hourly, roundTrip)
		}
	}
}

func TestAllResultsTwoDecimalPlaces(t *testing.T) {
	t.Parallel()

	// Test a range of inputs to verify all results have at most two decimal places.
	inputs := []float64{0.001, 0.01, 0.05, 0.10, 0.50, 1.00, 5.00, 10.00, 100.00, 1000.00, 10000.00}

	for _, input := range inputs {
		monthly := HourlyToMonthly(input)
		yearly := HourlyToYearly(input)
		hourly := MonthlyToHourly(input)

		for _, result := range []struct {
			name  string
			value float64
		}{
			{"HourlyToMonthly", monthly},
			{"HourlyToYearly", yearly},
			{"MonthlyToHourly", hourly},
		} {
			// Multiply by 100, check that there's no fractional part.
			scaled := result.value * 100
			if math.Abs(scaled-math.Round(scaled)) > 1e-9 {
				t.Errorf("%s(%v) = %v has more than 2 decimal places", result.name, input, result.value)
			}
		}
	}
}
