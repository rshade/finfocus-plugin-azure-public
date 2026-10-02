package pricing

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// calculatorAccuracyTolerance is 5 percent of the owner-supplied monthly cost.
// A plugin total at exactly this limit is inside the band.
const calculatorAccuracyTolerance = 0.05

func TestCalculatorAccuracy(t *testing.T) {
	t.Parallel()

	rows := loadCalculatorOwnerRows(t)
	if len(rows) == 0 {
		t.Fatal("calculator-values.csv has no data rows")
	}
	byID := make(map[string]oracleCase, len(rows))
	for _, item := range loadOracleCases(t) {
		byID[item.ID] = item
	}

	for _, row := range rows {
		t.Run(row.id, func(t *testing.T) {
			t.Parallel()
			compareCalculatorOwnerRow(t, row, byID[row.id], row.id == byID[row.id].ID)
		})
	}
}

func TestParseCalculatorOwnerValues(t *testing.T) {
	t.Parallel()

	raw := "case_id,calculator_configuration,owner_monthly_usd,read_on,notes\n" +
		"filled:case,\"quoted config\",12.5,,\n" +
		"empty:case,\"quoted config\",,,\n"
	rows, err := parseCalculatorOwnerValues(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].id != "filled:case" || rows[0].owner == nil || *rows[0].owner != 12.5 {
		t.Fatalf("filled row = %+v", rows[0])
	}
	if rows[1].id != "empty:case" || rows[1].owner != nil {
		t.Fatalf("empty owner cell must stay unset, got %+v", rows[1])
	}
}

const calculatorValuesPath = "testdata/oracle/calculator-values.csv"

type calculatorOwnerRow struct {
	id    string
	owner *float64
}

func compareCalculatorOwnerRow(t *testing.T, row calculatorOwnerRow, item oracleCase, found bool) {
	t.Helper()
	if row.owner == nil {
		t.Skipf("%s: owner value not supplied: owner_monthly_usd", row.id)
	}
	if !found {
		t.Fatalf("%s: no oracle case for calculator row", row.id)
	}
	req, err := oracleProjectedRequest(item)
	if err != nil {
		t.Fatalf("%s: %v", row.id, err)
	}
	calc := newPricingCalc(t, item.Rows)
	resp, err := dialPricingClient(t, calc).GetProjectedCost(context.Background(), req)
	if err != nil {
		t.Fatalf("%s GetProjectedCost() failed: %v", row.id, err)
	}
	plugin := resp.GetCostPerMonth()
	if item.Expected != nil && math.Abs(plugin-*item.Expected) > oracleTolerance(*item.Expected) {
		t.Logf(
			"%s: calculator %v disagrees with oracle %v; calculator wins",
			row.id,
			*row.owner,
			*item.Expected,
		)
	}
	if msg := calculatorMonthlyMismatch(plugin, *row.owner); msg != "" {
		t.Fatalf("%s %s", row.id, msg)
	}
}

func loadCalculatorOwnerRows(t *testing.T) []calculatorOwnerRow {
	t.Helper()

	raw, err := os.ReadFile(calculatorValuesPath)
	if err != nil {
		t.Fatalf("read calculator values: %v", err)
	}
	rows, err := parseCalculatorOwnerValues(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse calculator values: %v", err)
	}
	return rows
}

func parseCalculatorOwnerValues(r io.Reader) ([]calculatorOwnerRow, error) {
	records, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("calculator values: empty file")
	}
	header := records[0]
	if len(header) < 3 ||
		strings.TrimSpace(header[0]) != "case_id" ||
		strings.TrimSpace(header[2]) != "owner_monthly_usd" {
		return nil, fmt.Errorf("calculator values: header %q", header)
	}
	rows := make([]calculatorOwnerRow, 0, len(records)-1)
	for i, record := range records[1:] {
		if len(record) < 3 {
			return nil, fmt.Errorf("calculator row %d has %d columns", i+1, len(record))
		}
		id := strings.TrimSpace(record[0])
		if id == "" {
			return nil, fmt.Errorf("calculator row %d has an empty case id", i+1)
		}
		raw := strings.TrimSpace(record[2])
		row := calculatorOwnerRow{id: id}
		if raw == "" {
			rows = append(rows, row)
			continue
		}
		value, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("calculator case %s: owner_monthly_usd %q: %w", id, raw, parseErr)
		}
		row.owner = &value
		rows = append(rows, row)
	}
	return rows, nil
}

func TestCalculatorAccuracyOutsideBandFails(t *testing.T) {
	t.Parallel()

	// Test-local pair. Not an owner-table row. Not an Azure Pricing Calculator value.
	// |100-110| = 10, and 5 percent of 110 is 5.5, so the case must fail.
	const pluginMonthly = 100.0
	const ownerMonthly = 110.0
	if msg := calculatorMonthlyMismatch(pluginMonthly, ownerMonthly); msg == "" {
		t.Fatal("plugin 100 owner 110 is outside plus or minus 5 percent and must fail the case")
	}
}

func TestCalculatorAccuracyInsideBandPasses(t *testing.T) {
	t.Parallel()

	// Test-local pairs. Not owner-table rows. Not Azure Pricing Calculator values.
	// |100-104| = 4, and 5 percent of 104 is 5.2, so the case passes.
	const pluginMonthly = 100.0
	const ownerMonthly = 104.0
	if msg := calculatorMonthlyMismatch(pluginMonthly, ownerMonthly); msg != "" {
		t.Fatalf("plugin 100 owner 104 is inside plus or minus 5 percent and must pass: %s", msg)
	}

	// |105-100| = 5, and 5 percent of 100 is 5. The limit is inclusive.
	if msg := calculatorMonthlyMismatch(105, 100); msg != "" {
		t.Fatalf("plugin 105 owner 100 is exactly 5 percent and must pass: %s", msg)
	}
}

func calculatorMonthlyMismatch(plugin, owner float64) string {
	limit := math.Abs(owner) * calculatorAccuracyTolerance
	delta := math.Abs(plugin - owner)
	if delta <= limit {
		return ""
	}
	return fmt.Sprintf(
		"cost_per_month = %v, owner calculator = %v, outside plus or minus 5 percent (delta %v, limit %v)",
		plugin,
		owner,
		delta,
		limit,
	)
}
