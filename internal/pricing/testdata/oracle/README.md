# Price oracle data

Owner-owned, read-only for run agents. `expected.json` is produced from the public
Azure Retail Prices API by an independent script that does not read this plugin's code.
**Do not edit, regenerate or "fix" it.** `calculator-values.csv` is written only by
`scripts/calculator-values.py`, as described below. A case that fails is a finding: fix
the plugin, or list it in the "Not delivered" register with the reason.

## Files

| File | What it is |
| --- | --- |
| `expected.json` | About 50 cases with an expected monthly cost, the formula, and the full live API rows each came from, including rows deliberately excluded |
| `calculator-values.csv` | Azure Pricing Calculator values, one per case: `owner_monthly_usd`, the `read_on` date, and in `notes` the calculator offers and formula used. Empty means not read yet |

## Rules for the comparison test

1. **Statuses.**
   - `ok`: the plugin's monthly cost must be within `max(0.01, 0.5%)` of `expected_monthly`.
   - `not_available`: the plugin must return a gRPC error, never a zero cost.
   - `ambiguous`: the plugin must make a documented choice. The test records the choice and the
     candidates in `candidates` or `notes`, and fails only on a zero cost.
2. **Two modes.** Offline (default): a local fake Retail Prices server returns the case's `rows`
   (including the `excluded` ones) for any query, so the plugin's own selection logic is what is
   tested. Live (`ORACLE_LIVE=1`, opt-in, never in CI): the plugin's client queries the real API.
3. **Go through the real RPCs.** Build the request from `params` and `request_hint`, start a real
   gRPC server, call `GetProjectedCost` (and `EstimateCost` where the type supports it).
4. **A case the plugin cannot express** (no way to put `params` in a request) is a finding named in
   the report, not a skip.
5. **Calculator values win.** For every row of `calculator-values.csv` with a value, the plugin must
   be within 5% of it. When the calculator and the oracle disagree, the test logs both and the report
   says so. The rows with no value are skipped with the reason `owner value not supplied`. A
   filled value needs a `read_on` date that is not in the future, and case ids are unique.
   `calculatorMinComparedRows` is the number of rows the offline run compares today, so losing
   any row fails the suite.
6. **Write the results** (case id, plugin value, expected, difference, verdict) to
   `.superpowers/oracle-results.md` and paste the table in the run report.

Known traps the oracle encodes: Linux only, Windows and Low Priority rows are not Spot or on-demand
Linux; reservation `retailPrice` is a term total; blob storage is billed in volume bands; Disk
"Mount" and operations meters are not part of the disk price; the SQL zone-redundancy meter may be a
surcharge, not a replacement rate.

## Updating calculator values

The values come from the JSON the [Azure Pricing Calculator](https://azure.microsoft.com/pricing/calculator/)
page itself loads, at `https://azure.microsoft.com/api/{v2,v3}/pricing/<service>/calculator/`. They are
not a manual read of the page, and never come from the Retail Prices API, which is the plugin's
own source and would make the comparison circular.

1. Print the values without changing anything:

   ```bash
   python3 scripts/calculator-values.py
   ```

2. Write them into `calculator-values.csv`. Each filled row gets today's UTC date in `read_on`
   (`YYYY-MM-DD`) and the offers and formula in `notes`:

   ```bash
   python3 scripts/calculator-values.py --write
   ```

   The script prints `FILLED`, `EMPTY`, `KEPT` and `WARN` lines. `KEPT` means a row that held a
   value can no longer be read from the calculator. The old value stays and the script exits 1;
   find out what changed in the calculator data before committing. `WARN` means the row's
   `calculator_configuration` does not mention something the script read from the case id.
   Text in `notes` after an `owner:` marker is kept when the script rewrites the generated part.

3. Compare. Offline uses each case's oracle rows; `ORACLE_LIVE=1` queries the real API:

   ```bash
   go test ./internal/pricing/ -run TestCalculatorAccuracy -count=1 -v
   ORACLE_LIVE=1 go test ./internal/pricing/ -run TestCalculatorAccuracy -count=1 -v
   ```

4. Review the CSV diff before committing it.

The script computes each row the way the calculator does: 730 hours per month, graduated
blob bands, and the Functions free grant bands. A row it cannot express without guessing is
left empty, and the script prints why. A price of 0 is a calculator placeholder for a size or
offer not sold in that region, so it is refused rather than written as a free row. The one
intended zero is AKS Free, and only while the calculator has no priced Free or infrastructure
offer.

Zone redundant SQL follows the calculator's single database module: compute is the zone offer
plus the local offer, and storage is the zone storage rate in place of the local rate.

Re-read the values every 90 days. The test logs a row whose `read_on` is older than that.

When the plugin and the calculator disagree by more than 5 percent, find out which side is
wrong before changing anything. A plugin bug is fixed in the plugin. A row whose
`calculator_configuration` does not match what the plugin is asked to price is fixed in the
script. Never edit a value by hand and never widen the tolerance.

A known plugin gap is listed in `calculatorKnownFailure` in `calculator_accuracy_test.go` and in
the Not delivered register. The row is still compared on every run and skips while it misses.
When it comes inside the band the test fails with `gap closed`; remove the entry and the register
line then. A row the offline oracle cannot serve is listed in `calculatorLiveOnlyReason` and is
compared only with `ORACLE_LIVE=1`.
