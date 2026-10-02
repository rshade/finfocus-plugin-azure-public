# Regional price comparison

AZ-2.15 sorts `Standard_B1s` Linux on-demand prices across regions.
`SortRegionPrices` in `internal/pricing/regions.go` calls
`selectVMItem(items, false)` on each saved page.

A virtual machine `GetProjectedCost` and `EstimateCost` return the other
regions on `region_prices`. The requested region stays the parent cost.
A region with no selected row is left off the list. A zero appears only
when a row was found. A Spot quote uses the Linux Spot row for those
regions. An on-demand quote uses `SortRegionPrices`. The list is
advisory. The monthly cost stays the requested region. DryRun and
GetPricingSpec do not return the list.

## RPC

`region_prices` is the repeated region list from
[spec issue 589](https://github.com/rshade/finfocus-spec/issues/589).
The parent response still has one `unit_price` and one monthly cost.
`metadata` and `cost_breakdown` stay hints and components of that one
cost. The rows below are the fixture evidence. The prices are unchanged.

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
