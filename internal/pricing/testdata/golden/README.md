# Golden pricing snapshots

`TestGolden` compares one monthly cost per supported resource type with the
numbers in this directory. Those numbers come from the recorded retail
fixtures under `../retail/`.

`live/` holds four more snapshots taken from the public Retail Prices API:
two virtual machines and two managed disks. `capture.json` is the set of
HTTP responses the quote made. `expected.txt` is the monthly cost.
`TestGoldenLiveSnapshots` replays `capture.json` and fails when
`GetProjectedCost` or `EstimateCost` moves by more than 0.01.

Refresh the live snapshots from the repository root:

```bash
go test -tags=integration -update-golden -count=1 -timeout 8m ./... -run TestUpdateGoldenFromLiveAPI
```

`-tags=integration` keeps the live call out of the default test run.
`-update-golden` is what writes the files. Without that flag the same test
skips. `SKIP_INTEGRATION=true` skips it as well. Review the diff before
committing a refresh. The command does not edit `../retail/` or
`calculator-values.csv`.
