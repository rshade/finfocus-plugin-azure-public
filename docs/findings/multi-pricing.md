# Multi-pricing comparison

AZ-2.13 records Consumption, Savings Plan, and Reservation prices for one SKU,
and the fraction between two prices that share a unit. `SavingsFraction` in
`internal/estimation` returns `(onDemand - other) / onDemand`. It does not
round. A virtual machine quote calls it for each `price_options` row. The
selected monthly cost stays the row the caller asked for.

## Standard_D2s_v3 Consumption and Savings Plan

Fixture:
`internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption_preview.json`

This is the preview Consumption response `docs/findings/savings-plans.md` cites
for `Standard_D2s_v3` in `eastus`: `api-version=2023-01-01-preview` and
`priceType eq 'Consumption'`. `Count` is 6.

The Linux meter is `meterName` `D2s v3`, product
`Virtual Machines DSv3 Series`, `type` `Consumption`, `unitOfMeasure`
`1 Hour`, `retailPrice` 0.096. Its `savingsPlan` array is:

- `term` `1 Year`, `retailPrice` 0.06624. `SavingsFraction(0.096, 0.06624)` is 0.31.
- `term` `3 Years`, `retailPrice` 0.04512. `SavingsFraction(0.096, 0.04512)` is 0.53.

Each nested object has `term`, `retailPrice`, and `unitPrice` only. It does
not repeat `unitOfMeasure`. The fraction assumes the parent meter's `1 Hour`.

## Standard_D2s_v3 has no Reservation rows

Fixture:
`internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_reservation.json`

`Count` is 0 and `Items` is empty. The preview file
`internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_reservation_preview.json`
is also `Count` 0. There is no Reservation `retailPrice` for this SKU to set
beside the Savings Plan prices in the preceding section.

## Reservation prices for Standard_D2als_v7

Fixture:
`internal/pricing/testdata/retail/savingsplan/eastus_standard_d2als_v7_pricetype_reservation.json`

Both rows are `type` `Reservation` and `unitOfMeasure` `1 Hour`:

- `reservationTerm` `1 Year`, `retailPrice` 416
- `reservationTerm` `3 Years`, `retailPrice` 803

The hourly Consumption price beside those rows is 0.0804, also
`unitOfMeasure` `1 Hour`, on the Linux meter `D2als v7` in `eastus` in
`internal/pricing/testdata/retail/savingsplan/eastus_standard_d2als_v7_nofilter_preview.json`.

Those Reservation rows are labeled `1 Hour`, and the response has no second
field that says whether 416 and 803 are an hourly rate or a term total. The
plugin treats them as term totals. `ReservationHourly` divides 416 by 8760
and 803 by 26280, then `SavingsFraction` compares each hourly rate with
the Linux on-demand rate 0.0804. A virtual machine quote returns those
rows on `price_options`. The selected monthly cost stays 0.0804 times 730.

## Savings Plans are not a price type

Savings Plans are selected by the nested `savingsPlan` array on preview
Consumption meters, not by `priceType eq 'SavingsPlan'`.

These files are `Count` 0:

- `internal/pricing/testdata/retail/savingsplan/pricetype_savingsplan_preview.json`
- `internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_savingsplan_preview.json`

The 0.06624 and 0.04512 prices are read from the preview Consumption fixture
in the first section, not from a `SavingsPlan` price type.

## GetProjectedCost returns price options

A virtual machine `GetProjectedCost` and `EstimateCost` return
`price_options`. The selected monthly cost is unchanged. The list is
advisory. Consumption, Spot, Savings Plan, and Reservation rows are
included when the retail page has them. A SKU with no reservation rows
omits Reservation. Windows, Low Priority, and DevTest rows are not
options.

`price_options` is the repeated list from
[spec issue 588](https://github.com/rshade/finfocus-spec/issues/588).
`region_prices` is the per-region list from
[spec issue 589](https://github.com/rshade/finfocus-spec/issues/589).

The notes below record why `metadata` and `cost_breakdown` were the
wrong shape in spec v0.7.0, before those fields existed. They are still
the wrong shape. The line numbers are that v0.7.0 reading.

`metadata` (lines 1615-1635) is the wrong shape. It is `map[string]string`
for plugin hints. The comment names keys such as `defaults_applied`. Values
are strings, not prices. A consumer that does not recognize a key must ignore
it.

`cost_breakdown` (lines 1636-1669) is the wrong shape. It is
`map[string]float64` for the components of that one monthly cost. When the
map is non-empty, the values sum to `cost_per_month` (lines 1644-1645).
Values are non-negative (line 1652). Discounts and credits are already
applied to the component they reduce, and there are no negative entries
(lines 1646-1647). A second retail price, or a savings fraction, is not a
component of `cost_per_month`.

`prediction_interval_lower` and `prediction_interval_upper` (lines 1561-1585)
bound that same `cost_per_month`. They are not another price.
`impact_metrics` (lines 1495-1496) is sustainability metrics, not prices.

`CommitmentAction` is not a home for a second price. In
`../finfocus-spec/proto/finfocus/v1/costsource.proto` it is field 7 of
`action_detail` on `Recommendation` (lines 1510-1528). The message is
lines 1622-1634. Commitment fields in
`../finfocus-spec/proto/finfocus/v1/focus.proto` are FOCUS columns on
`FocusCostRecord` (lines 347-371) and `ContractCommitment` rows. The discount
percentage there is one optional fraction (lines 621-625), not a list of
alternative retail prices on the projected-cost response.
