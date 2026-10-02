# Savings Plans

Research spike for the public Retail Prices API. No estimator uses these
files. Prices below are copied from the saved JSON. They are not taken from
documentation.

The endpoint is `https://prices.azure.com/api/retail/prices`. Responses are
under `internal/pricing/testdata/retail/savingsplan/`. Each file is the raw
body plus one trailing newline. `curl` lines that reproduce them are in
`internal/pricing/testdata/retail/README.md`.

## Scope

Issue #57 names `Standard_D2s_v3` in `eastus`. That is the comparison SKU.
`Standard_B1s` in `eastus` is the other SKU already used in this repo. Both
have a savings-plan array and no Reservation rows.

`Standard_D2als_v7` in `eastus` is a third query. It is used only because
those two SKUs cannot show a Reservation row. The SKU is one that the
truncated Reservation page already returned. It is not a guessed price.

Successful SKU-scoped responses have `NextPageLink` null. Nothing was
followed. HTTP 400 bodies have no `NextPageLink`.

`pricetype_reservation_page1.json` and
`pricetype_reservation_preview_page1.json` are the first page of
`priceType eq 'Reservation'` only. Each page has `Count` 1000 and a
`NextPageLink`. Those two catalogs were not downloaded. The pages are
truncated. The links are:

- `https://prices.azure.com:443/api/retail/prices?$filter=priceType%20eq%20%27Reservation%27&$skip=1000`
- `https://prices.azure.com:443/api/retail/prices?api-version=2023-01-01-preview&$filter=priceType%20eq%20%27Reservation%27&$skip=1000`

The 1000 items on those two pages are equal. The preview page does not add
a `savingsPlan` key to them.

`pricetype_devtestconsumption_page1.json` is the first page of
`priceType eq 'DevTestConsumption'` with no other filter. HTTP 200,
`Count` 1000, and every saved item has `type` `DevTestConsumption`. No
item on that page has a `savingsPlan` key. `NextPageLink` is set. Later
pages were not downloaded. The page is truncated. The link is
`https://prices.azure.com:443/api/retail/prices?$filter=priceType%20eq%20%27DevTestConsumption%27&$skip=1000`.

## Do Savings Plan rows exist

Yes. They are not separate `Items`, and `type` is not `SavingsPlan`.

`eastus_standard_d2s_v3_pricetype_consumption_preview.json` is HTTP 200,
`Count` 6, `NextPageLink` null. Query:

`api-version=2023-01-01-preview` and
`armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Consumption'`.

One row has a `savingsPlan` array. The other five omit the key. The row is
`skuName` `D2s v3`, `meterName` `D2s v3`, `productName`
`Virtual Machines DSv3 Series`, `type` `Consumption`, `unitOfMeasure`
`1 Hour`, `retailPrice` 0.096, `unitPrice` 0.096, `armSkuName`
`Standard_D2s_v3`, `isPrimaryMeterRegion` false. The array is:

- `term` `3 Years`, `retailPrice` 0.04512, `unitPrice` 0.04512
- `term` `1 Year`, `retailPrice` 0.06624, `unitPrice` 0.06624

Each object has only those three fields.

The same SKU without `api-version`
(`eastus_standard_d2s_v3_pricetype_consumption.json`, HTTP 200, `Count` 6)
has the same six meters and no `savingsPlan` key.
`api-version=2021-10-01-preview`
(`eastus_standard_d2s_v3_pricetype_consumption_apiver_2021-10-01-preview.json`)
also has no `savingsPlan` key.
`api-version=2023-01-01`
(`eastus_standard_d2s_v3_pricetype_consumption_apiver_2023-01-01.json`) is
HTTP 400: `Unsupported API version`.

`priceType eq 'SavingsPlan'` does not return these rates. With no other
filter, `pricetype_savingsplan.json` and `pricetype_savingsplan_preview.json`
are HTTP 200, `Count` 0, `NextPageLink` null. The same filter on this SKU
is also `Count` 0 (`eastus_standard_d2s_v3_pricetype_savingsplan.json` and
`eastus_standard_d2s_v3_pricetype_savingsplan_preview.json`). Lower case
`savingsplan` is `Count` 0 on both API versions
(`eastus_standard_d2s_v3_pricetype_savingsplan_lower.json`,
`eastus_standard_d2s_v3_pricetype_savingsplan_lower_preview.json`).

## Which filter selects them

There is no `priceType` for Savings Plans in the rows above. The selector
is `api-version=2023-01-01-preview` plus a Consumption query for the meter.
The array is read from that item. It is not a filter field.
`priceType eq 'SavingsPlan'` is accepted and matches nothing, so it does
not select these rows.

`savingsPlan/any(t: t/term eq '1 Year')` on this SKU
(`savingsplan_term_filter.json`) is HTTP 400: `Invalid parameters supplied`.
Rows that carry a savings plan cannot be selected with that OData clause.

`priceType` strings from the client comment, plus the strings tried after
the error text, behave like this. The error text does not name the allowed
values. It says `Invalid OData parameters supplied`.

| Filter `priceType` | File | Result |
| --- | --- | --- |
| `Consumption` | `eastus_standard_d2s_v3_pricetype_consumption.json` | HTTP 200, 6 rows |
| `Reservation` | `eastus_standard_d2s_v3_pricetype_reservation.json` | HTTP 200, `Count` 0 |
| `DevTestConsumption` | `eastus_standard_d2s_v3_pricetype_devtestconsumption.json` | HTTP 200, 3 rows |
| `DevTestConsumption` (no other filter) | `pricetype_devtestconsumption_page1.json` | HTTP 200, first page `Count` 1000, truncated |
| `SavingsPlan` | `pricetype_savingsplan.json` | HTTP 200, `Count` 0 for the catalog |
| `savingsplan` | `eastus_standard_d2s_v3_pricetype_savingsplan_lower.json` | HTTP 200, `Count` 0 |
| `NotAType` | `eastus_standard_d2s_v3_pricetype_notatype.json` | HTTP 400 |
| `OnDemand` | `eastus_standard_d2s_v3_pricetype_ondemand.json` | HTTP 400 |
| `Spot` | `eastus_standard_d2s_v3_pricetype_spot.json` | HTTP 400 |
| `Reserved` | `pricetype_reserved.json` | HTTP 400 |
| `ReservedInstance` | `pricetype_reservedinstance.json` | HTTP 400 |
| `ComputeSavingsPlan` | `pricetype_computesavingsplan.json` | HTTP 400 |
| `Savings Plan` | `pricetype_savings_plan_spaced.json` | HTTP 400 |

`Reservation` is a real type. The unfiltered first page
(`pricetype_reservation_page1.json`) is HTTP 200 and is truncated, as
recorded above. `DevTestConsumption` for this SKU is three Windows meters
and has no `savingsPlan` key, including the preview file
`eastus_standard_d2s_v3_pricetype_devtestconsumption_preview.json`.

`meterRegion=primary` is not how savings plans are selected. On this SKU
it returns only the two Spot rows
(`eastus_standard_d2s_v3_pricetype_consumption_preview_primary.json`,
`Count` 2). The Linux meter that carries `savingsPlan` has
`isPrimaryMeterRegion` false, so this filter drops it.

## Per SKU or per commitment

Per SKU meter, not per spend commitment.

The array hangs on `armSkuName` `Standard_D2s_v3`, meter `D2s v3`. It does
not name a dollar-per-hour commitment. No saved row is a commitment product.

These queries are HTTP 200, `Count` 0, `NextPageLink` null:

- `eastus_servicename_savings_plan.json`,
  `serviceName eq 'Savings Plan'` in `eastus`
- `eastus_vm_contains_product_savings_plan.json`,
  Virtual Machines in `eastus` and `contains(productName, 'Savings Plan')`
- `eastus_contains_product_savings_plan_preview.json`, the same `contains`
  on the preview API with no service filter

The commitment size the customer buys is not answerable from the public
API. The queries above are what was tried.

The same SKU still has several meters. Only one has the array. From
`eastus_standard_d2s_v3_pricetype_consumption_preview.json`:

| skuName | productName | retailPrice | savingsPlan |
| --- | --- | --- | --- |
| `D2s v3 Low Priority` | `Virtual Machines DSv3 Series Windows` | 0.075 | absent |
| `D2s v3 Low Priority` | `Virtual Machines DSv3 Series` | 0.019 | absent |
| `D2s v3 Spot` | `Virtual Machines DSv3 Series` | 0.018816 | absent |
| `D2s v3 Spot` | `Virtual Machines DSv3 Series Windows` | 0.036848 | absent |
| `D2s v3` | `Virtual Machines DSv3 Series` | 0.096 | 1 Year 0.06624, 3 Years 0.04512 |
| `D2s v3` | `Virtual Machines DSv3 Series Windows` | 0.188 | absent |

`Standard_B1s` is the same shape. Preview Consumption
(`eastus_standard_b1s_pricetype_consumption_preview.json`, `Count` 2) puts
the array only on `Virtual Machines BS Series`, meter `B1s`, `retailPrice`
0.0104: 1 Year 0.0070044 and 3 Years 0.00468624. The Windows meter,
`retailPrice` 0.014, has no array. Both rows are `isPrimaryMeterRegion`
true, so "primary" does not mean "no savings plan". That was only true of
the D2s v3 Linux row above.

## How they differ from Reservation

For `Standard_D2s_v3` the Reservation query returns no rows. Savings-plan
prices and Reservation prices are not both present for this machine.

HTTP 200, `Count` 0, `NextPageLink` null:

- `eastus_standard_d2s_v3_pricetype_reservation.json` and
  `eastus_standard_d2s_v3_pricetype_reservation_preview.json`,
  `priceType eq 'Reservation'` plus region and `armSkuName`
- `standard_d2s_v3_pricetype_reservation_noregion.json`, same SKU, no region
- `eastus_skuname_d2s_v3_pricetype_reservation.json`, `skuName eq 'D2s v3'`
- `eastus_vm_meter_contains_d2s_v3_pricetype_reservation.json` and
  `contains_meter_d2s_v3_pricetype_reservation.json`,
  `contains(meterName, 'D2s v3')`
- `eastus_contains_armsku_d2s_v3_pricetype_reservation.json` and
  `contains_armsku_d2s_v3_pricetype_reservation.json`,
  `contains(armSkuName, 'D2s_v3')`
- `contains_skuname_d2s_v3_pricetype_reservation.json`,
  `contains(skuName, 'D2s v3')`
- `eastus_vm_product_dsv3_series_pricetype_reservation.json` and
  `product_vm_dsv3_series_pricetype_reservation.json`,
  `productName eq 'Virtual Machines DSv3 Series'`
- `eastus_vm_product_dsv3_series_windows_pricetype_reservation.json`,
  the Windows product in `eastus`
- `eastus_standard_d2s_v3_reservationterm_1_year.json`,
  `reservationTerm eq '1 Year'` on this SKU
- `eastus_standard_b1s_pricetype_reservation.json`, `Standard_B1s` in `eastus`

`contains(productName, 'DSv3')` with `priceType eq 'Reservation'`
(`contains_product_dsv3_pricetype_reservation.json`) is HTTP 200, `Count`
244, `NextPageLink` null. Every row is product
`DSv3 Series Dedicated Host`, not `Virtual Machines DSv3 Series`. The
`eastus` ARM SKUs in that file are `Dsv3_Type3` and `Dsv3_Type4`. They are
not `Standard_D2s_v3`. `eastus` prices in that file: `Dsv3_Type3` 1 Year
21996 and 3 Years 42292; `Dsv3_Type4` 1 Year 27495 and 3 Years 52866.
`unitOfMeasure` is `1 Hour`, `type` is `Reservation`, and there is no
`savingsPlan` key.

Where a VM SKU has both, the shapes differ. This is `Standard_D2als_v7` in
`eastus`, not the comparison SKU above.
`eastus_standard_d2als_v7_nofilter_preview.json` is one preview response
with no `priceType` filter. `Count` 8, `NextPageLink` null.

The Linux Consumption meter `D2als v7`, product
`Virtual Machines Dalsv7 Series`, `unitOfMeasure` `1 Hour`, `retailPrice`
0.0804, carries:

- `term` `3 Years`, `retailPrice` 0.035376
- `term` `1 Year`, `retailPrice` 0.053868

The Reservation rows in that same file, same `skuName`, `meterName`,
`productName`, and `unitOfMeasure` `1 Hour`, are separate items:

- `type` `Reservation`, `reservationTerm` `1 Year`, `retailPrice` 416
- `type` `Reservation`, `reservationTerm` `3 Years`, `retailPrice` 803

Those Reservation items have no `savingsPlan` key. The savings-plan objects
have no `reservationTerm`. `unitPrice` equals `retailPrice` on these rows.

`reservationTerm` is a response field and a working filter on this SKU.
`eastus_standard_d2als_v7_reservationterm_1_year.json` is HTTP 200, `Count`
1: the 416 row only.

The Reservation `retailPrice` values 416 and 803 are not the same magnitude
as the hourly Consumption price 0.0804 or the savings-plan prices on that
meter. The response still labels Reservation `unitOfMeasure` as `1 Hour`.
It has no second field that says whether 416 is an hourly rate or a term
total. That meaning is not answerable from the public API. The queries are
the D2als files named in this section.

## Spend commitment and per-resource estimation

Savings Plans are described as a spend commitment. The public API does not
return the commitment amount. See the empty service and product queries in
the per-SKU section. Not answerable from the public API.

What it does return is a second retail price on the SKU meter. The nested
objects do not repeat `unitOfMeasure`. The parent meter says `1 Hour`, and
the nested prices are the same order of magnitude as that meter's
`retailPrice`. A per-resource estimate can use the nested `retailPrice` as
an hourly rate for that meter only. It cannot price the commitment itself,
and it cannot apply one array to Spot, Low Priority, or the Windows meter
when those rows omit `savingsPlan`.

## Savings percent for one SKU

Yes, for a meter that has both prices. No, for a meter that omits the array.
Reservation percent is not answerable for `Standard_D2s_v3` because that
Reservation query is empty. The public rows for `Standard_D2als_v7` still do
not label whether `retailPrice` is hourly or a term total. The plugin reading
below supplies that rule.

From `eastus_standard_d2s_v3_pricetype_consumption_preview.json`, Linux
`D2s v3`, on-demand `retailPrice` 0.096:

- 1 Year: `(0.096 - 0.06624) / 0.096 = 0.31`
- 3 Years: `(0.096 - 0.04512) / 0.096 = 0.53`

The Windows meter at 0.188 has no savings-plan price, so no percent.

From `eastus_standard_b1s_pricetype_consumption_preview.json`, Linux `B1s`,
on-demand 0.0104:

- 1 Year: `(0.0104 - 0.0070044) / 0.0104 = 0.3265`
- 3 Years: `(0.0104 - 0.00468624) / 0.0104 = 0.5494`

The percent is per meter. It is not a constant of the API.

## One-year and three-year terms

Yes. On every savings-plan array saved here the terms are `1 Year` and
`3 Years`. No other `term` appears. The strings match Reservation
`reservationTerm` on the D2als rows, including the plural `3 Years`.

The truncated Reservation page also contains `reservationTerm` `5 Years`
and `10 Years`. Those values were not on a savings-plan array. They are not
Savings Plan terms in the files saved here.

## Fold into issue 45 or keep separate

Fold the per-SKU rates into the multi-price comparison. Do not add a second
data source, and do not select them with `priceType eq 'SavingsPlan'`.

Issue #45 says the API exposes Consumption, Reservation, and Savings Plans
through `priceType` and `reservationTerm`. These responses contradict that
for Savings Plans. Consumption and Reservation are `priceType` values.
Savings Plans are the `savingsPlan` array on a preview Consumption item.
`reservationTerm` selects Reservation rows. `term` inside the array selects
the savings-plan length.

This repository's client calls the base URL with no `api-version`. A stable
response for this SKU omits the array, as
`eastus_standard_d2s_v3_pricetype_consumption.json` shows. `PriceItem` keeps
the array when a preview body includes it. Issue #45 was not edited.

Where the two price points go in the gRPC response is the spec question
already on that task. This spike does not choose a field.

## One query for Consumption, Reservation, and Savings Plans

Yes, when the SKU has Reservation rows and the call uses the preview
version.

`eastus_standard_d2als_v7_pricetype_consumption_or_reservation_preview.json`
is HTTP 200, `Count` 6, `NextPageLink` null. Query:

`api-version=2023-01-01-preview` and
`armRegionName eq 'eastus' and armSkuName eq 'Standard_D2als_v7' and (priceType eq 'Consumption' or priceType eq 'Reservation')`.

That response has four Consumption rows and two Reservation rows. The Linux
Consumption row still has the `savingsPlan` array. DevTest rows are not in
it. The no-filter preview file for this SKU has those plus two
DevTestConsumption rows (`Count` 8).

The same OR on `Standard_D2s_v3`
(`eastus_standard_d2s_v3_pricetype_consumption_or_reservation_preview.json`,
`Count` 6) returns only Consumption rows, one of them with `savingsPlan`.
It cannot return Reservation rows because this SKU has none. Leaving
`priceType` off
(`eastus_standard_d2s_v3_nofilter_preview.json`, `Count` 9) adds
DevTestConsumption and still no Reservation.

`priceType eq 'Reservation'` alone does not return the array. The D2als
Reservation file has two rows and no `savingsPlan` key.

## How a response should tell them apart

Use the fields the items already have. Do not collapse them into one
discount.

- Reservation: `type` is `Reservation`, term is `reservationTerm`, price is
  the item `retailPrice`. There is no `savingsPlan` key on the Reservation
  rows saved here.
- Savings plan: `type` stays `Consumption`, term is `savingsPlan[].term`,
  price is `savingsPlan[].retailPrice`. The item `retailPrice` remains the
  on-demand price. There is no `reservationTerm` on that row.

A single savings percentage would hide the term and which model produced
it. The D2s v3 Linux meter is 0.31 at 1 Year and 0.53 at 3 Years against
on-demand. Those are not Reservation savings. Reservation savings for that
SKU are not in the API.

## Can Savings Plans be shown per SKU

Answer: yes.

Show them from the preview Consumption meter for that `armSkuName`, not
from `priceType eq 'SavingsPlan'` and not from a commitment product. For
`Standard_D2s_v3` in `eastus`, the only meter with the array is Linux
`D2s v3` at on-demand 0.096, with 1 Year 0.06624 and 3 Years 0.04512.
Spot, Low Priority, and Windows on that SKU have no array.
`meterRegion=primary`
drops the one row that has it.

## How the code reads the rows

`PriceItem.SavingsPlan` stores the nested array. A test on
`eastus_standard_d2s_v3_pricetype_consumption_preview.json` fails if that
array is ignored. `SavingsFraction` on the Linux meter is 0.31 for one year
and 0.53 for three years. The production client does not request
`api-version=2023-01-01-preview`, so a default quote still has no array.

`ReservationHourly` treats Reservation `retailPrice` as the term total even
though `unitOfMeasure` says `1 Hour`. One year divides by 8760. Three years
divides by 26280. For `Standard_D2als_v7` that is 416 / 8760 and 803 / 26280.
`SavingsFraction` can then compare those hourly rates with the Linux
on-demand rate 0.0804. The public response still does not label the unit.
This is the rule the plugin applies.

`GetProjectedCost` still returns one price. Nothing in the RPC returns the
savings-plan terms, the reservation hourly rate, or the fraction. Putting
those values in `metadata` or `cost_breakdown` would be the wrong shape.
The missing repeated alternative-price list is
[spec issue 588](https://github.com/rshade/finfocus-spec/issues/588).
