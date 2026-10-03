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

// calculatorMinComparedRows is the number of rows the offline run compares today:
// 23 filled rows, less the two known failures and the two live-only SQL rows, with
// AKS Free and the 1000 GB SQL row in both groups. Set to the current count rather
// than a low floor, so a refresh that empties even one row fails the suite instead
// of quietly turning it into a skip. Raise it when rows are added.
const calculatorMinComparedRows = 20

// calculatorStaleAfter is when a calculator value is logged as due for a re-read.
const calculatorStaleAfter = 90 * 24 * time.Hour

// calculatorFutureSlack allows read_on to be the next calendar day, because the
// script stamps the UTC date and the test may run in a timezone behind UTC.
const calculatorFutureSlack = 24 * time.Hour

const calculatorReadOnLayout = "2006-01-02"

// calculatorMinColumns covers case_id, calculator_configuration, owner_monthly_usd and read_on.
const calculatorMinColumns = 4

const (
	calculatorAKSFreeRow        = "aks_control_plane_free:eastus"
	calculatorSQLZoneRow        = "sql_gp_gen5:2vcore:100gb:zr:eastus"
	calculatorSQLZoneStorageRow = "sql_gp_gen5:2vcore:1000gb:zr:eastus"
)

// calculatorTestNow is the fixed clock for the parser tests.
func calculatorTestNow() time.Time {
	return time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)
}

// TestCalculatorAccuracy compares the plugin with Azure Pricing Calculator values.
// Offline it serves each case's oracle rows from a fake Retail Prices server.
// With ORACLE_LIVE=1 it queries the real Retail Prices API instead.
func TestCalculatorAccuracy(t *testing.T) {
	t.Parallel()

	rows := loadCalculatorOwnerRows(t)
	if len(rows) == 0 {
		t.Fatal("calculator-values.csv has no data rows")
	}
	live := os.Getenv("ORACLE_LIVE") == "1"
	if err := checkFilledCalculatorRows(rows, live); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]oracleCase, len(rows))
	for _, item := range loadOracleCases(t) {
		byID[item.ID] = item
	}

	var shared *Calculator
	if live {
		shared = newLivePricingCalc(t)
	}

	for _, row := range rows {
		t.Run(row.id, func(t *testing.T) {
			t.Parallel()
			item, found := calculatorOracleCase(row.id, byID)
			compareCalculatorOwnerRow(t, row, item, found, shared)
		})
	}
}

func TestParseCalculatorOwnerValues_FilledAndEmptyRows_KeepsEmptyUnset(t *testing.T) {
	t.Parallel()

	raw := "case_id,calculator_configuration,owner_monthly_usd,read_on,notes\n" +
		"filled:case,\"quoted config\",12.5,2026-10-03,\n" +
		"empty:case,\"quoted config\",,,\n"
	rows, err := parseCalculatorOwnerValuesAt(strings.NewReader(raw), calculatorTestNow())
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
		{name: "date one day ahead", row: "a:case,\"c\",12.5,2026-10-04,\n", wantOn: "2026-10-04"},
		{name: "date in the future", row: "a:case,\"c\",12.5,2062-10-03,\n", wantErr: "future"},
		{
			name:    "duplicate case id",
			row:     "a:case,\"c\",12.5,2026-10-03,\na:case,\"c\",13,2026-10-03,\n",
			wantErr: "duplicate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows, err := parseCalculatorOwnerValuesAt(strings.NewReader(header+tt.row), calculatorTestNow())
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

func TestCountComparedCalculatorRows_KnownFailuresAndLiveOnly_NotCounted(t *testing.T) {
	t.Parallel()

	value := 1.0
	rows := []calculatorOwnerRow{
		{id: "vm_ondemand_linux:Standard_D2s_v3:eastus", owner: &value, readOn: "2026-10-03"},
		{id: "managed_disk:P10 LRS:eastus", owner: &value, readOn: "2026-10-03"},
		{id: calculatorAKSFreeRow, owner: &value, readOn: "2026-10-03"},
		{id: calculatorSQLZoneRow, owner: &value, readOn: "2026-10-03"},
		{id: calculatorSQLZoneStorageRow, owner: &value, readOn: "2026-10-03"},
		{id: "empty"},
	}
	if got := countComparedCalculatorRows(rows, false); got != 2 {
		t.Fatalf("offline compared rows = %d, want 2 (known failures, live-only and empty rows excluded)", got)
	}
	if got := countComparedCalculatorRows(rows, true); got != 3 {
		t.Fatalf("live compared rows = %d, want 3 (known failures and empty rows excluded)", got)
	}
}

func TestCheckFilledCalculatorRows_FewerThanMinimum_ReturnsError(t *testing.T) {
	t.Parallel()

	value := 1.0
	rows := make([]calculatorOwnerRow, 0, calculatorMinComparedRows)
	for i := range calculatorMinComparedRows {
		rows = append(rows, calculatorOwnerRow{id: fmt.Sprintf("row:%d", i), owner: &value, readOn: "2026-10-03"})
	}
	if err := checkFilledCalculatorRows(rows, false); err != nil {
		t.Fatalf("%d compared rows must pass the guard: %v", calculatorMinComparedRows, err)
	}
	rows[0].owner = nil
	if err := checkFilledCalculatorRows(rows, false); err == nil {
		t.Fatalf("%d compared rows must fail the guard", calculatorMinComparedRows-1)
	}
}

func TestCalculatorLiveOnlyReason_RowsWithoutOfflineRows_NameTheGap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       string
		wantGap  bool
		wantText string
	}{
		{id: calculatorSQLZoneRow, wantGap: true, wantText: "ORACLE_LIVE=1"},
		{id: calculatorSQLZoneStorageRow, wantGap: true, wantText: "ORACLE_LIVE=1"},
		{id: calculatorAKSFreeRow},
		{id: "vm_ondemand_linux:Standard_D2s_v3:eastus"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			t.Parallel()

			reason := calculatorLiveOnlyReason(tt.id)
			if (reason != "") != tt.wantGap || !strings.Contains(reason, tt.wantText) {
				t.Fatalf("live-only reason = %q, want gap %v naming %q", reason, tt.wantGap, tt.wantText)
			}
		})
	}
}

func TestJudgeCalculatorRow_KnownFailureStates_SkipUntilGapCloses(t *testing.T) {
	t.Parallel()

	const outside = "outside plus or minus 5 percent"
	tests := []struct {
		name     string
		id       string
		mismatch string
		wantSkip string
		wantFail string
	}{
		{name: "known failure still failing", id: calculatorAKSFreeRow, mismatch: outside, wantSkip: "PR #70"},
		{name: "known failure now passing", id: calculatorAKSFreeRow, wantFail: "gap closed"},
		{
			name:     "storage-heavy SQL row still failing",
			id:       calculatorSQLZoneStorageRow,
			mismatch: outside,
			wantSkip: "zone storage",
		},
		{name: "storage-heavy SQL row now passing", id: calculatorSQLZoneStorageRow, wantFail: "gap closed"},
		{
			name:     "ordinary row failing",
			id:       "vm_ondemand_linux:Standard_D2s_v3:eastus",
			mismatch: outside,
			wantFail: outside,
		},
		{name: "ordinary row passing", id: "vm_ondemand_linux:Standard_D2s_v3:eastus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			skip, fail := judgeCalculatorRow(tt.id, tt.mismatch)
			if (skip != "") != (tt.wantSkip != "") || !strings.Contains(skip, tt.wantSkip) {
				t.Fatalf("skip = %q, want it to contain %q", skip, tt.wantSkip)
			}
			if (fail != "") != (tt.wantFail != "") || !strings.Contains(fail, tt.wantFail) {
				t.Fatalf("fail = %q, want it to contain %q", fail, tt.wantFail)
			}
		})
	}
}

func TestCalculatorOracleCase_StorageHeavySQLRow_DerivesFromBaseCase(t *testing.T) {
	t.Parallel()

	base := oracleCase{
		ID:     calculatorSQLZoneRow,
		Kind:   "sql_database",
		Region: "eastus",
		Params: map[string]any{"vcores": 2.0, "storage_gb": 100.0, "zone_redundant": true},
	}
	byID := map[string]oracleCase{base.ID: base}

	item, found := calculatorOracleCase(calculatorSQLZoneStorageRow, byID)
	if !found {
		t.Fatal("storage-heavy SQL row must derive a case from the 100 GB zone redundant case")
	}
	if item.ID != calculatorSQLZoneStorageRow || item.Params["storage_gb"] != 1000.0 || item.Expected != nil {
		t.Fatalf(
			"derived case = %+v, want id %s, storage_gb 1000, no oracle expectation",
			item,
			calculatorSQLZoneStorageRow,
		)
	}
	if base.Params["storage_gb"] != 100.0 {
		t.Fatalf("deriving must not change the base case params, got %v", base.Params["storage_gb"])
	}
	if _, found := calculatorOracleCase("unknown:case", byID); found {
		t.Fatal("a row with no oracle case and no derivation must not be found")
	}
}

const calculatorValuesPath = "testdata/oracle/calculator-values.csv"

type calculatorOwnerRow struct {
	id     string
	owner  *float64
	readOn string
}

// calculatorKnownFailure names a row whose plugin quote is known to miss the
// calculator. The row is still compared on every run: while it misses, it skips
// with this reason; once it is inside the band, the test fails so the entry is
// removed. Each entry is also listed in the oracle README's "Not delivered" register.
func calculatorKnownFailure(id string) string {
	switch id {
	case calculatorAKSFreeRow:
		return "known failure, pending PR #70 (AKS Free = $0): main still quotes the " +
			"FreeTierInfrastructureCost meter at 0.05 USD/hour"
	case calculatorSQLZoneStorageRow:
		return "known failure: the plugin bills local storage plus zone storage, while the " +
			"calculator bills zone storage in place of local storage"
	}
	return ""
}

// calculatorLiveOnlyReason names a row the offline run cannot compare, because its
// oracle case has no base rows to serve. ORACLE_LIVE=1 compares it.
func calculatorLiveOnlyReason(id string) string {
	switch id {
	case calculatorSQLZoneRow, calculatorSQLZoneStorageRow:
		return "oracle rows hold only the zone redundancy meters, not the base compute " +
			"and storage rows; run with ORACLE_LIVE=1"
	}
	return ""
}

// calculatorOracleCase finds the oracle case a calculator row is priced from. A
// row the oracle does not cover is derived from a base case with its params
// overridden; a derived case carries no oracle expectation.
func calculatorOracleCase(id string, byID map[string]oracleCase) (oracleCase, bool) {
	if item, found := byID[id]; found {
		return item, true
	}
	if id != calculatorSQLZoneStorageRow {
		return oracleCase{}, false
	}
	base, found := byID[calculatorSQLZoneRow]
	if !found {
		return oracleCase{}, false
	}
	params := make(map[string]any, len(base.Params))
	for key, value := range base.Params {
		params[key] = value
	}
	params["storage_gb"] = 1000.0
	base.ID = id
	base.Params = params
	base.Expected = nil
	return base, true
}

// judgeCalculatorRow turns a row's mismatch into a skip or a failure.
func judgeCalculatorRow(id, mismatch string) (string, string) {
	known := calculatorKnownFailure(id)
	switch {
	case known != "" && mismatch != "":
		return known, ""
	case known != "":
		return "", "gap closed: the plugin is now within 5 percent of the calculator; " +
			"remove the known-failure entry for " + id + " and its Not delivered register line"
	case mismatch != "":
		return "", mismatch
	}
	return "", ""
}

func countComparedCalculatorRows(rows []calculatorOwnerRow, live bool) int {
	compared := 0
	for _, row := range rows {
		if row.owner == nil || calculatorKnownFailure(row.id) != "" {
			continue
		}
		if !live && calculatorLiveOnlyReason(row.id) != "" {
			continue
		}
		compared++
	}
	return compared
}

func checkFilledCalculatorRows(rows []calculatorOwnerRow, live bool) error {
	compared := countComparedCalculatorRows(rows, live)
	if compared < calculatorMinComparedRows {
		return fmt.Errorf(
			"calculator-values.csv gives %d compared rows, want at least %d; "+
				"run scripts/calculator-values.py --write and check its KEPT and EMPTY lines",
			compared,
			calculatorMinComparedRows,
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
	if reason := calculatorLiveOnlyReason(row.id); shared == nil && reason != "" {
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
	skip, fail := judgeCalculatorRow(row.id, calculatorMonthlyMismatch(plugin, *row.owner))
	if fail != "" {
		t.Fatalf("%s %s", row.id, fail)
	}
	if skip != "" {
		t.Skipf("%s: %s (plugin %.2f, calculator %.2f)", row.id, skip, plugin, *row.owner)
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
	return parseCalculatorOwnerValuesAt(r, time.Now())
}

func parseCalculatorOwnerValuesAt(r io.Reader, now time.Time) ([]calculatorOwnerRow, error) {
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
	seen := make(map[string]bool, len(records)-1)
	for i, record := range records[1:] {
		if len(record) < calculatorMinColumns {
			return nil, fmt.Errorf("calculator row %d has %d columns", i+1, len(record))
		}
		id := strings.TrimSpace(record[0])
		if id == "" {
			return nil, fmt.Errorf("calculator row %d has an empty case id", i+1)
		}
		if seen[id] {
			return nil, fmt.Errorf("calculator row %d: duplicate case id %s", i+1, id)
		}
		seen[id] = true
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
		readDate, dateErr := time.Parse(calculatorReadOnLayout, readOn)
		if dateErr != nil {
			return nil, fmt.Errorf("calculator case %s: read_on %q must be a YYYY-MM-DD date: %w", id, readOn, dateErr)
		}
		if readDate.After(now.Add(calculatorFutureSlack)) {
			return nil, fmt.Errorf("calculator case %s: read_on %s is in the future", id, readOn)
		}
		row.owner = &value
		rows = append(rows, row)
	}
	return rows, nil
}

func TestCalculatorMonthlyMismatch_OutsideBand_ReturnsMismatch(t *testing.T) {
	t.Parallel()

	// Test-local pair. Not an owner-table row. Not an Azure Pricing Calculator value.
	// |100-110| = 10, and 5 percent of 110 is 5.5, so the case must fail.
	const pluginMonthly = 100.0
	const ownerMonthly = 110.0
	if msg := calculatorMonthlyMismatch(pluginMonthly, ownerMonthly); msg == "" {
		t.Fatal("plugin 100 owner 110 is outside plus or minus 5 percent and must fail the case")
	}
}

func TestCalculatorMonthlyMismatch_InsideOrAtBand_ReturnsEmpty(t *testing.T) {
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
