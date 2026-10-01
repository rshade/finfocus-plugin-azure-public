package estimation

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
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

func TestSavingsFraction(t *testing.T) {
	t.Parallel()

	const tol = 1e-9

	tests := []struct {
		name      string
		onDemand  float64
		other     float64
		want      float64
		errSubstr string
	}{
		{name: "partial savings", onDemand: 0.10, other: 0.06, want: 0.4},
		{name: "onDemand zero", onDemand: 0, other: 0.06, errSubstr: "onDemand"},
		{name: "onDemand negative", onDemand: -0.10, other: 0.06, errSubstr: "onDemand"},
		{name: "other negative", onDemand: 0.10, other: -0.01, errSubstr: "other"},
		{name: "other above onDemand", onDemand: 0.10, other: 0.15, want: -0.5},
		{name: "other zero", onDemand: 0.10, other: 0, want: 1},
		{name: "not rounded", onDemand: 3, other: 1, want: 2.0 / 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := SavingsFraction(tt.onDemand, tt.other)
			if tt.errSubstr != "" {
				if err == nil {
					t.Fatalf(
						"SavingsFraction(%v, %v) error = nil, want substring %q",
						tt.onDemand,
						tt.other,
						tt.errSubstr,
					)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf(
						"SavingsFraction(%v, %v) error = %q, want substring %q",
						tt.onDemand,
						tt.other,
						err.Error(),
						tt.errSubstr,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("SavingsFraction(%v, %v) unexpected error: %v", tt.onDemand, tt.other, err)
			}
			if math.Abs(got-tt.want) > tol {
				t.Errorf("SavingsFraction(%v, %v) = %v, want %v", tt.onDemand, tt.other, got, tt.want)
			}
		})
	}
}

// d2sV3PreviewFixturePath is the Task 13 preview Consumption response for
// Standard_D2s_v3 in eastus (api-version=2023-01-01-preview).
func d2sV3PreviewFixturePath() string {
	return filepath.Join(
		"..",
		"pricing",
		"testdata",
		"retail",
		"savingsplan",
		"eastus_standard_d2s_v3_pricetype_consumption_preview.json",
	)
}

func TestSavingsFractionD2sV3Fixture(t *testing.T) {
	t.Parallel()

	const tol = 1e-9

	onDemand, oneYear, threeYear := readD2sV3LinuxSavingsPrices(t)

	gotYear, err := SavingsFraction(onDemand, oneYear)
	if err != nil {
		t.Fatalf("SavingsFraction(onDemand, 1 Year) error: %v", err)
	}
	if math.Abs(gotYear-0.31) > tol {
		t.Errorf("SavingsFraction(onDemand, 1 Year) = %v, want 0.31", gotYear)
	}

	gotYears, err := SavingsFraction(onDemand, threeYear)
	if err != nil {
		t.Fatalf("SavingsFraction(onDemand, 3 Years) error: %v", err)
	}
	if math.Abs(gotYears-0.53) > tol {
		t.Errorf("SavingsFraction(onDemand, 3 Years) = %v, want 0.53", gotYears)
	}
}

func readD2sV3LinuxSavingsPrices(t *testing.T) (float64, float64, float64) {
	t.Helper()

	var onDemand, oneYear, threeYear float64

	body, err := os.ReadFile(filepath.Clean(d2sV3PreviewFixturePath()))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var payload struct {
		Items []struct {
			ArmSkuName    string            `json:"armSkuName"`
			MeterName     string            `json:"meterName"`
			ProductName   string            `json:"productName"`
			Type          string            `json:"type"`
			UnitOfMeasure string            `json:"unitOfMeasure"`
			RetailPrice   float64           `json:"retailPrice"`
			SavingsPlan   []json.RawMessage `json:"savingsPlan"`
		} `json:"Items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	const (
		wantOnDemand = 0.096
		wantOneYear  = 0.06624
		wantThree    = 0.04512
		priceTol     = 1e-9
	)

	found := false
	for _, item := range payload.Items {
		if item.ArmSkuName != "Standard_D2s_v3" ||
			item.MeterName != "D2s v3" ||
			item.ProductName != "Virtual Machines DSv3 Series" ||
			item.Type != "Consumption" ||
			item.UnitOfMeasure != "1 Hour" {
			continue
		}
		found = true
		onDemand = item.RetailPrice
		if len(item.SavingsPlan) == 0 {
			t.Fatal("fixture savingsPlan prices are absent")
		}
		for _, raw := range item.SavingsPlan {
			var keys map[string]json.RawMessage
			if err := json.Unmarshal(raw, &keys); err != nil {
				t.Fatalf("unmarshal savingsPlan object: %v", err)
			}
			if _, ok := keys["unitOfMeasure"]; ok {
				t.Fatal("savingsPlan repeats unitOfMeasure; fraction must not assume a nested unit")
			}
			var term struct {
				Term        string  `json:"term"`
				RetailPrice float64 `json:"retailPrice"`
			}
			if err := json.Unmarshal(raw, &term); err != nil {
				t.Fatalf("unmarshal savingsPlan term: %v", err)
			}
			switch term.Term {
			case "1 Year":
				oneYear = term.RetailPrice
			case "3 Years":
				threeYear = term.RetailPrice
			default:
				t.Fatalf("unexpected savingsPlan term %q", term.Term)
			}
		}
		break
	}
	if !found {
		t.Fatal("Linux D2s v3 meter is absent from fixture")
	}
	if math.Abs(onDemand-wantOnDemand) > priceTol ||
		math.Abs(oneYear-wantOneYear) > priceTol ||
		math.Abs(threeYear-wantThree) > priceTol {
		t.Fatalf("fixture prices absent: onDemand=%v 1 Year=%v 3 Years=%v", onDemand, oneYear, threeYear)
	}
	return onDemand, oneYear, threeYear
}
