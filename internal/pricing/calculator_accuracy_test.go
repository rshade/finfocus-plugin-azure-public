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
	"time"
)

// calculatorAccuracyTolerance is 5 percent of the owner-supplied monthly cost.
// A plugin total at exactly this limit is inside the band.
const calculatorAccuracyTolerance = 0.05

// calculatorMinFilledRows keeps an emptied CSV from turning the suite into all skips.
const calculatorMinFilledRows = 4

// calculatorStaleAfter is when a calculator value is logged as due for a re-read.
const calculatorStaleAfter = 90 * 24 * time.Hour

const calculatorReadOnLayout = "2006-01-02"

// calculatorMinColumns covers case_id, calculator_configuration, owner_monthly_usd and read_on.
const calculatorMinColumns = 4

// TestCalculatorAccuracy compares the plugin with Azure Pricing Calculator values.
// Offline it serves each case's oracle rows from a fake Retail Prices server.
// With ORACLE_LIVE=1 it queries the real Retail Prices API instead.
func TestCalculatorAccuracy(t *testing.T) {
	t.Parallel()

	rows := loadCalculatorOwnerRows(t)
	if len(rows) == 0 {
		t.Fatal("calculator-values.csv has no data rows")
	}
	if err := checkFilledCalculatorRows(rows); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]oracleCase, len(rows))
	for _, item := range loadOracleCases(t) {
		byID[item.ID] = item
	}

	live := os.Getenv("ORACLE_LIVE") == "1"
	var shared *Calculator
	if live {
		shared = newLivePricingCalc(t)
	}

	for _, row := range rows {
		t.Run(row.id, func(t *testing.T) {
			t.Parallel()
			item, found := byID[row.id]
			compareCalculatorOwnerRow(t, row, item, found, shared)
		})
	}
}

func TestParseCalculatorOwnerValues(t *testing.T) {
	t.Parallel()

	raw := "case_id,calculator_configuration,owner_monthly_usd,read_on,notes\n" +
		"filled:case,\"quoted config\",12.5,2026-10-03,\n" +
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

func TestParseCalculatorOwnerValues_ReadOn_ValidatesDate(t *testing.T) {
	t.Parallel()

	const header = "case_id,calculator_configuration,owner_monthly_usd,read_on,notes\n"
	tests := []struct {
		name    string
		row     string
		wantErr string
		wantOn  string
	}{
		{name: "filled value with date", row: "a:case,\"c\",12.5,2026-10-03,\n", wantOn: "2026-10-03"},
		{name: "filled value without date", row: "a:case,\"c\",12.5,,\n", wantErr: "read_on"},
		{name: "filled value with malformed date", row: "a:case,\"c\",12.5,03/10/2026,\n", wantErr: "read_on"},
		{name: "date without value", row: "a:case,\"c\",,2026-10-03,\n", wantErr: "owner_monthly_usd"},
		{name: "empty row", row: "a:case,\"c\",,,\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows, err := parseCalculatorOwnerValues(strings.NewReader(header + tt.row))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to name %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := rows[0].readOn; got != tt.wantOn {
				t.Fatalf("read_on = %q, want %q", got, tt.wantOn)
			}
		})
	}
}

func TestCheckFilledCalculatorRows_FewerThanMinimum_ReturnsError(t *testing.T) {
	t.Parallel()

	value := 1.0
	filled := calculatorOwnerRow{id: "filled", owner: &value, readOn: "2026-10-03"}
	empty := calculatorOwnerRow{id: "empty"}

	enough := []calculatorOwnerRow{filled, filled, filled, filled, empty}
	if err := checkFilledCalculatorRows(enough); err != nil {
		t.Fatalf("four filled rows must pass the guard: %v", err)
	}
	tooFew := []calculatorOwnerRow{filled, filled, filled, empty, empty}
	if err := checkFilledCalculatorRows(tooFew); err == nil {
		t.Fatal("three filled rows must fail the guard")
	}
}

func TestCalculatorRowSkipReason_KnownGaps_SkipByMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       string
		live     bool
		wantSkip bool
	}{
		{name: "pending plugin fix offline", id: "aks_control_plane_free:eastus", wantSkip: true},
		{name: "pending plugin fix live", id: "aks_control_plane_free:eastus", live: true, wantSkip: true},
		{name: "oracle rows gap offline", id: "sql_gp_gen5:2vcore:100gb:zr:eastus", wantSkip: true},
		{name: "oracle rows gap live", id: "sql_gp_gen5:2vcore:100gb:zr:eastus", live: true},
		{name: "ordinary row", id: "vm_ondemand_linux:Standard_D2s_v3:eastus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reason := calculatorRowSkipReason(tt.id, tt.live)
			if (reason != "") != tt.wantSkip {
				t.Fatalf("skip reason = %q, want skip %v", reason, tt.wantSkip)
			}
		})
	}
}

const calculatorValuesPath = "testdata/oracle/calculator-values.csv"

type calculatorOwnerRow struct {
	id     string
	owner  *float64
	readOn string
}

// calculatorRowSkipReason names a calculator row that cannot be compared yet.
// Each entry is temporary: remove it when the named gap closes.
func calculatorRowSkipReason(id string, live bool) string {
	switch id {
	case "aks_control_plane_free:eastus":
		return "pending PR #70 (AKS Free = $0): this branch still quotes the FreeTierInfrastructureCost meter"
	case "sql_gp_gen5:2vcore:100gb:zr:eastus":
		if live {
			return ""
		}
		return "oracle rows for this case hold only the zone redundancy meters, not the base " +
			"compute and storage rows; run with ORACLE_LIVE=1"
	}
	return ""
}

func checkFilledCalculatorRows(rows []calculatorOwnerRow) error {
	filled := 0
	for _, row := range rows {
		if row.owner != nil {
			filled++
		}
	}
	if filled < calculatorMinFilledRows {
		return fmt.Errorf(
			"calculator-values.csv has %d rows with owner_monthly_usd, want at least %d; "+
				"run scripts/calculator-values.py --write",
			filled,
			calculatorMinFilledRows,
		)
	}
	return nil
}

func compareCalculatorOwnerRow(
	t *testing.T,
	row calculatorOwnerRow,
	item oracleCase,
	found bool,
	shared *Calculator,
) {
	t.Helper()
	if row.owner == nil {
		t.Skipf("%s: owner value not supplied: owner_monthly_usd", row.id)
	}
	if reason := calculatorRowSkipReason(row.id, shared != nil); reason != "" {
		t.Skipf("%s: %s", row.id, reason)
	}
	if !found {
		t.Fatalf("%s: no oracle case for calculator row", row.id)
	}
	if readOn, err := time.Parse(calculatorReadOnLayout, row.readOn); err == nil &&
		time.Since(readOn) > calculatorStaleAfter {
		t.Logf(
			"%s: calculator value read on %s is over 90 days old; re-run scripts/calculator-values.py",
			row.id,
			row.readOn,
		)
	}
	req, err := oracleProjectedRequest(item)
	if err != nil {
		t.Fatalf("%s: %v", row.id, err)
	}
	calc := shared
	if calc == nil {
		calc = newPricingCalc(t, item.Rows)
	}
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
	t.Logf("%s: plugin %.4f, calculator %.2f (read %s)", row.id, plugin, *row.owner, row.readOn)
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
	if len(header) < calculatorMinColumns ||
		strings.TrimSpace(header[0]) != "case_id" ||
		strings.TrimSpace(header[2]) != "owner_monthly_usd" ||
		strings.TrimSpace(header[3]) != "read_on" {
		return nil, fmt.Errorf("calculator values: header %q", header)
	}
	rows := make([]calculatorOwnerRow, 0, len(records)-1)
	for i, record := range records[1:] {
		if len(record) < calculatorMinColumns {
			return nil, fmt.Errorf("calculator row %d has %d columns", i+1, len(record))
		}
		id := strings.TrimSpace(record[0])
		if id == "" {
			return nil, fmt.Errorf("calculator row %d has an empty case id", i+1)
		}
		raw := strings.TrimSpace(record[2])
		readOn := strings.TrimSpace(record[3])
		row := calculatorOwnerRow{id: id, readOn: readOn}
		if raw == "" {
			if readOn != "" {
				return nil, fmt.Errorf("calculator case %s: read_on %q set without owner_monthly_usd", id, readOn)
			}
			rows = append(rows, row)
			continue
		}
		value, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("calculator case %s: owner_monthly_usd %q: %w", id, raw, parseErr)
		}
		if _, dateErr := time.Parse(calculatorReadOnLayout, readOn); dateErr != nil {
			return nil, fmt.Errorf("calculator case %s: read_on %q must be a YYYY-MM-DD date: %w", id, readOn, dateErr)
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
