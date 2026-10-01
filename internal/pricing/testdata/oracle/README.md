# Price oracle data

Owner-owned, read-only for run agents. Produced from the public Azure Retail Prices
API by an independent script that does not read this plugin's code. **Do not edit,
regenerate or "fix" these files.** A case that fails is a finding: fix the plugin, or
list it in the "Not delivered" register with the reason.

## Files

| File | What it is |
| --- | --- |
| `expected.json` | About 50 cases with an expected monthly cost, the formula, and the full live API rows each came from, including rows deliberately excluded |
| `calculator-values.csv` | Values the owner reads from the Azure Pricing Calculator. The owner fills `owner_monthly_usd`. Empty means not read yet |

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
   says so. The rows with no value are skipped with the reason "owner value not supplied".
6. **Write the results** (case id, plugin value, expected, difference, verdict) to
   `.superpowers/oracle-results.md` and paste the table in the run report.

Known traps the oracle encodes: Linux only, Windows and Low Priority rows are not Spot or on-demand
Linux; reservation `retailPrice` is a term total; blob storage is billed in volume bands; Disk
"Mount" and operations meters are not part of the disk price; the SQL zone-redundancy meter may be a
surcharge, not a replacement rate.
