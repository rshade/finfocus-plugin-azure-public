# Azure SQL Database and Cosmos DB

Retail prices are read from the fixture files. This note lists meters from
those files. It does not copy retail prices as the source of truth.

## SQL Database

Both queries are `eastus`, service `SQL Database`, `priceType` Consumption.
`NextPageLink` was JSON null on each response, so nothing was followed. Each
file is that single page.

- Compute: `internal/pricing/testdata/retail/sqldb/gp_gen5_compute_eastus.json`
  (62 items).
- Storage: `internal/pricing/testdata/retail/sqldb/gp_storage_eastus.json`
  (5 items).

The request SKU `GP_Gen5_2` is not an `armSkuName`. Compute rows carry ARM SKUs
such as `SQLDB_GP_Compute_Gen5_2` (skuName `2 vCore`) and
`SQLDB_GP_Compute_Gen5` (skuName `vCore`). Storage rows leave `armSkuName`
empty. Queries filter region, service, product, and currency only.

### Models this task prices

General Purpose Gen5 provisioned vCore only. The SKU is `GP_Gen5_{n}`, or tags
`tier` (`GeneralPurpose` or `GP`), `hardware` `Gen5`, and `vcores` `{n}`.
`size_gb` is required. Monthly cost is compute plus storage.

`zone_redundant=true` adds zone-redundancy compute and zone-redundancy
storage. Any other value omits those components.

### Models this task refuses

These return `Unimplemented`. The message names the model and `AZ-2.7`. They
do not return a zero cost.

- DTU, for example `S0`
- Serverless, for example `GP_S_Gen5_2`
- Business Critical, for example `BC_Gen5_2`
- Hyperscale, for example `HS_Gen5_2`
- Any other hardware family, for example `GP_Fsv2_2`

Cosmos DB is not priced here.

### Compute product

Product `SQL Database Single/Elastic Pool General Purpose - Compute Gen5`.
Unit of measure is `1 Hour`. Type is `Consumption`.

skuName `{n} vCore` with meter `vCore` is already the price for n vCores. Do
not multiply by n again. skuName `vCore` with meter `vCore` is the one-vCore
unit row. Do not use it when `{n} vCore` exists. If `{n} vCore` is missing,
the result is `NotFound` even though the unit row remains.

Zone redundancy uses skuName `{n} vCore Zone Redundancy` and meter
`Zone Redundancy vCore`. skuName `vCore ZR Zone Redundancy` is the one-vCore
zone unit row and is not used.

Rows that share a skuName and meterName do not differ by retail price. Most
pairs differ by `skuId` only. `1 vCore` and `1 vCore Zone Redundancy` also
differ by `effectiveStartDate` and `isPrimaryMeterRegion`. Selection keeps one
of those rows. It does not average prices. Monthly compute is that row's
`retailPrice` times 730.

| skuName | meterName | unitOfMeasure |
| --- | --- | --- |
| `1 vCore` | `vCore` | `1 Hour` |
| `1 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `2 vCore` | `vCore` | `1 Hour` |
| `2 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `4 vCore` | `vCore` | `1 Hour` |
| `4 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `6 vCore` | `vCore` | `1 Hour` |
| `6 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `8 vCore` | `vCore` | `1 Hour` |
| `8 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `10 vCore` | `vCore` | `1 Hour` |
| `10 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `12 vCore` | `vCore` | `1 Hour` |
| `12 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `14 vCore` | `vCore` | `1 Hour` |
| `14 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `16 vCore` | `vCore` | `1 Hour` |
| `16 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `18 vCore` | `vCore` | `1 Hour` |
| `18 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `20 vCore` | `vCore` | `1 Hour` |
| `20 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `24 vCore` | `vCore` | `1 Hour` |
| `24 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `32 vCore` | `vCore` | `1 Hour` |
| `32 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `40 vCore` | `vCore` | `1 Hour` |
| `40 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `80 vCore` | `vCore` | `1 Hour` |
| `80 vCore Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |
| `vCore` | `vCore` | `1 Hour` |
| `vCore ZR Zone Redundancy` | `Zone Redundancy vCore` | `1 Hour` |

### Storage product

Product `SQL Database Single/Elastic Pool General Purpose - Storage`. Type is
`Consumption`.

The paid capacity meter is `General Purpose Data Stored`, unit `1 GB/Month`.
Monthly storage is that row's `retailPrice` times `size_gb`. It is not
multiplied by 730.

`General Purpose Data Stored - Free` is the included quantity. Its retail
price in the fixture is 0. Do not select it when the paid meter exists. If
the paid meter is missing, the result is `NotFound`, not the free row.

Zone capacity is meter `General Purpose Zone Redundancy Data Stored`, unit
`1 GB/Month`. It is added only when `zone_redundant=true`. IO rate meters are
not capacity and are not selected.

| skuName | meterName | unitOfMeasure |
| --- | --- | --- |
| `General Purpose` | `General Purpose Data Stored` | `1 GB/Month` |
| `General Purpose` | `General Purpose Data Stored - Free` | `1 GB/Month` |
| `General Purpose` | `General Purpose IO Rate Operations` | `1M` |
| `General Purpose Zone Redundancy` | `General Purpose Zone Redundancy Data Stored` | `1 GB/Month` |
| `General Purpose Zone Redundancy` | `General Purpose Zone Redundancy IO Rate Operations` | `1M` |

## Cosmos DB

`internal/pricing/testdata/retail/cosmosdb/eastus_consumption.json` is the
`eastus` Consumption page for service `Azure Cosmos DB`. `NextPageLink` was
JSON null, so nothing was followed. The file is that single page (111 items).

The query filters region, service, and currency only. `armSkuName` and
`productName` stay empty. Provisioned, multi-master, autoscale, and serverless
rows are different products on that one page. Selection is local.

Other products on the page are not account throughput. They include
`Azure DocumentDB`, dedicated gateway, Garnet cache, materialized views,
analytics storage, snapshot, and PITR. They are not selected.

### Cosmos models priced

The default is manual provisioned throughput. Tags `ru_per_second` (or `rus`)
and `size_gb` (or `sizeGb`) are required. `ru_per_second` wins when both RU
tags are set. Monthly cost is RU plus storage. Components are `ru` and
`storage`. They sum to the monthly cost.

`multi_master=true` selects sku `mRUs` for the RU meter and for
`Data Stored`. It applies only when `pricing_model` is empty. Any other
value, including `false`, stays on sku `RUs`.

`pricing_model=autoscale` still requires `ru_per_second` and `size_gb`.
`pricing_model=serverless` requires `request_units` and does not require
`size_gb`. Serverless has no `storage` component, even if `size_gb` is set.
Any other `pricing_model`, including `manual`, is `InvalidArgument` and names
that value. A missing meter is `NotFound`, not a zero cost.

### Meters this task does not select

These rows are on product `Azure Cosmos DB`. They are not a paid account
price. Free and Free Tier retail prices in the fixture are 0. Do not subtract
a free-tier grant. sku `RUm` and meter `1000 RU/m` are a per-minute unit, not
a block of RU/s.

| skuName | meterName | unitOfMeasure |
| --- | --- | --- |
| `Free` | `Free 100 Multi-master RU/s` | `1/Hour` |
| `Free Tier` | `100 RU/s` | `1/Hour` |
| `Free Tier` | `100 Multi-master RU/s` | `1/Hour` |
| `Free Tier` | `Data Stored` | `1 GB/Month` |
| `RUm` | `1000 RU/m` | `1/Hour` |
| `RUm` | `Data Stored` | `1 GB/Month` |

`S1`, `S2`, and `S3` use unit `1 Hour`, not `1/Hour`. They are not RU blocks.

### Manual provisioned product

Product `Azure Cosmos DB`. Type is `Consumption`.

The RU meter is sku `RUs`, meter `100 RU/s`, unit `1/Hour`. The leading
integer in the meter name is the RU block. Monthly RU cost is
`(ru_per_second / that integer) * retailPrice * 730`. The `730` is
`pluginsdk.HoursPerMonth`. If that meter name does not start with an integer,
the result is `InvalidArgument` and names the meter.

Storage is sku `RUs`, meter `Data Stored`, unit `1 GB/Month`. Monthly storage
is `retailPrice * size_gb`. It is not multiplied by 730.

Multi-master uses sku `mRUs`, meter `100 Multi-master RU/s`, unit `1/Hour`,
and sku `mRUs` meter `Data Stored`. The same leading-integer rule applies.
The multi-master RU meter in this file is not the same price as `100 RU/s`.

| skuName | meterName | unitOfMeasure |
| --- | --- | --- |
| `RUs` | `100 RU/s` | `1/Hour` |
| `RUs` | `Data Stored` | `1 GB/Month` |
| `mRUs` | `100 Multi-master RU/s` | `1/Hour` |
| `mRUs` | `Data Stored` | `1 GB/Month` |

### Serverless product

Product `Azure Cosmos DB serverless`. sku `RUs`, meter `1M RUs`, unit `1M`,
type `Consumption`. `1M` means per million request units. Monthly cost is
`(request_units / 1000000) * retailPrice`. It is not multiplied by 730.

If the unit is not `1M`, the result is `InvalidArgument` and names the unit.
The saved page has no serverless storage meter. A `Data Stored` row on this
product is still omitted. That product has no `size_gb` component. The quote
note says that product publishes no storage meter.

A live query on 2026-10-01 returned one row and no next page. The filter was
service `Azure Cosmos DB`, the same region as the fixture, and
`contains(productName, 'serverless')`. The only meter was `1M RUs` at
0.25 USD per `1M`. No storage meter was present.

| skuName | meterName | unitOfMeasure |
| --- | --- | --- |
| `RUs` | `1M RUs` | `1M` |

### Autoscale product

Product `Azure Cosmos DB autoscale`. Type is `Consumption`. Unit is `1/Hour`.

The RU meters are `AP1 100 RUs`, `AP2 100 RUs`, `AP3 100 RUs`, and
`AP4 100 RUs`. Match a meter that ends with `100 RUs`. These meters do not
start with an integer. That is not an error. Their retail prices in this file
do not differ. The block size is the `100` in that suffix. Monthly RU cost
uses the same `/ 100 * 730` rule as manual throughput. Do not also multiply
by 1.5. If those prices disagree, the result is `InvalidArgument` and names
the meter.

`AP1 Entry Price` through `AP4 Entry Price` are not the RU price.

The saved page has no autoscale storage meter. Storage stays the provisioned
`Data Stored` meter on sku `RUs`. A Consumption row on this product with unit
`1 GB/Month` and a meter name containing `Data Stored`, other than a free
row, is used instead when it is present.

| skuName | meterName | unitOfMeasure |
| --- | --- | --- |
| `AP1` | `AP1 100 RUs` | `1/Hour` |
| `AP1` | `AP1 Entry Price` | `1/Hour` |
| `AP2` | `AP2 100 RUs` | `1/Hour` |
| `AP2` | `AP2 Entry Price` | `1/Hour` |
| `AP3` | `AP3 100 RUs` | `1/Hour` |
| `AP3` | `AP3 Entry Price` | `1/Hour` |
| `AP4` | `AP4 100 RUs` | `1/Hour` |
| `AP4` | `AP4 Entry Price` | `1/Hour` |
