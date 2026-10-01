# Regional price comparison

AZ-2.15 sorts `Standard_B1s` Linux on-demand prices across regions.
`SortRegionPrices` in `internal/pricing/regions.go` calls
`selectVMItem(items, false)` on each saved page. `GetProjectedCost` does
not call it. Quote selection, DryRun, and GetPricingSpec are unchanged.

## RPC

The RPC exposure is `BLOCKED`.

`GetProjectedCostResponse` is
`../finfocus-spec/sdk/go/proto/finfocus/v1/costsource.pb.go` lines 1485-1672.
It has one `unit_price` (line 1488) and one `cost_per_month` (line 1492).
It has no repeated region list. The only repeated message field is
`impact_metrics` (line 1496), which is sustainability metrics, not prices
by region. `metadata` (line 1635) is a string hint map. `cost_breakdown`
(line 1669) is the components of that one monthly cost. A region ranking is
not written into either map.

## Selected rows

Each file is one Retail Prices response for `armSkuName` `Standard_B1s`,
service `Virtual Machines`, and `priceType` `Consumption`. `BillingCurrency`
is `USD`. `NextPageLink` was JSON null, so nothing was followed. Each file
is that response plus one trailing newline.

`selectVMItem(items, false)` keeps the non-Windows row. On these pages that
row is product `Virtual Machines BS Series`, meter `B1s`, `unitOfMeasure`
`1 Hour`. The Windows row on the same page is not selected. These pages
have no Spot row and no Low Priority row.

| Region | Fixture | Selected |
| --- | --- | --- |
| `eastus` | `internal/pricing/testdata/retail/regions/standard_b1s_eastus.json` | `retailPrice` 0.0104, `unitOfMeasure` `1 Hour` |
| `westus2` | `internal/pricing/testdata/retail/regions/standard_b1s_westus2.json` | `retailPrice` 0.0104, `unitOfMeasure` `1 Hour` |
| `northeurope` | `internal/pricing/testdata/retail/regions/standard_b1s_northeurope.json` | `retailPrice` 0.0113, `unitOfMeasure` `1 Hour` |
| `not-a-region` | `internal/pricing/testdata/retail/regions/standard_b1s_not-a-region.json` | missing |

`eastus` and `westus2` tie at 0.0104, so region name puts `eastus` first.
`northeurope` follows at 0.0113. `not-a-region` has `Count` 0 and an empty
`Items` array. It is missing, not priced at zero, and sorts last.
