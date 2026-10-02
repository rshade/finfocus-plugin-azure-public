# Retail Prices fixtures

Raw responses from the Azure Retail Prices API
(`https://prices.azure.com/api/retail/prices`). Tests read these files.
Do not edit the JSON by hand.

## Spot virtual machines

`spot/standard_d2s_v3_eastus.json` is the Consumption query for
`Standard_D2s_v3` in `eastus`.

Rows share `armSkuName` and `priceType`. They differ by `skuName`,
`meterName`, and `productName`:

- On-demand rows have neither `Spot` nor `Low Priority` in the name.
- A Spot row has the word `Spot` in `skuName` or `meterName`.
- A Low Priority row is neither on-demand nor Spot.
- A `productName` containing `Windows` is not used. An empty `productName`
  is kept, so a one-row unit test still prices.

`GetProjectedCost` reads the descriptor tag `priority`:

- `priority=Spot` (any capitalization) uses the non-Windows Spot row.
- An empty `priority` uses the non-Windows on-demand row.
- Any other non-empty `priority` is rejected.

`EstimateCost` reads the same values from the attribute `priority`. A Spot
quote sets pricing category Dynamic. The interruption score stays 0.

Monthly cost is that row's `retailPrice` times 730 hours. The resource type
stays `compute/VirtualMachine`.

## Storage accounts

`storageaccount/general_block_blob_v2_eastus.json` is the Consumption page for
product `General Block Blob v2` in `eastus`. `NextPageLink` was empty, so this
file is that single response. The query does not filter `armSkuName`. A capacity
meter is `skuName`, a space, then `Data Stored`, unit `1 GB/Month`. Hot meters
repeat for `tierMinimumUnits` bands. Monthly cost walks those bands: each GB
uses the rate of the band it falls in. The first band starts at `0`. It is
not multiplied by 730.

```bash
mkdir -p internal/pricing/testdata/retail/storageaccount
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Storage' and productName eq 'General Block Blob v2' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/storageaccount/general_block_blob_v2_eastus.json
```

## App Service plans

`appservice/eastus_consumption.json` is the Consumption page for service
`Azure App Service` in `eastus`. `NextPageLink` was null, so this file is that
single response (130 items). The query does not filter `armSkuName`. The short
plan SKU, such as `P1v3`, is not an ARM SKU and returns no rows when used as
`armSkuName`.

Plan rows are selected locally. Spaces in the SKU are ignored, so `P1v3`
matches skuName `P1 v3`. The meter name is the skuName, or the skuName
followed by a space and `App`. Unit is `1 Hour` and type is `Consumption`.
Stamp, SSL, Domain, and ASIP rows are not
plan prices. The default product contains `Linux`. Tag `os=Windows` selects
the product that does not. Any other non-empty `os` is rejected. Monthly cost
is `retailPrice` times 730. `F1` / `F1 App` is a real `0`.

```bash
mkdir -p internal/pricing/testdata/retail/appservice
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Azure App Service' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/appservice/eastus_consumption.json
```

## Functions

`functions/eastus_consumption.json` is the Consumption page for service
`Functions` (not `Azure Functions`) in `eastus`. `NextPageLink` was null, so
this file is that single response (13 items). There is no `EP1` meter. Do not
invent one.

Classic Consumption meters publish a `0` row and a non-zero row for the same
meter. The `0` row is the included quantity, not the overage. When a positive
`retailPrice` sibling exists, use that row. `Standard Total Executions` is
priced per `10` executions. `Standard Execution Time` is priced per
`1 GB Second`.

The free grant is documented at
<https://azure.microsoft.com/pricing/details/functions/>.
Consumption includes 1,000,000 executions and 400,000 GB-s per subscription
per month. The grant is per subscription, not per function app. This plugin
applies that full grant to the one resource being priced. The Flex Consumption
grant is not applied.

Premium meters are on product `Premium Functions`: `Premium vCPU Duration`
(`1 Hour`) and `Premium Memory Duration` (`1 GiB Hour`). A dedicated Function
App SKU that matches an App Service plan uses the App Service hourly price.

```bash
mkdir -p internal/pricing/testdata/retail/functions
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Functions' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/functions/eastus_consumption.json
```

## AKS

`aks/eastus_consumption.json` is the Consumption page for service
`Azure Kubernetes Service` in `eastus`. `NextPageLink` was null, so this file
is that single response (12 items). The query does not filter `armSkuName` or
`productName`.

Control-plane rows are selected locally from product
`Azure Kubernetes Service`. Unit is `1 Hour` and type is `Consumption`.

- `Standard` uses meter `Standard Uptime SLA`. Tag `support=lts` uses
  `Standard Long Term Support` instead.
- `Free` uses meter `FreeTierInfrastructureCost Uptime SLA`. Two rows share
  that meter. The row with `effectiveEndDate` set is closed. The open row has
  an empty `effectiveEndDate` and is not `0`. Monthly cost is that open
  `retailPrice` times 730.
- Tier `Automatic` is rejected. `Azure Kubernetes Service - Automatic` meters
  are not control-plane prices.

Node pools are on-demand Virtual Machines in the cluster region. Each pool is
priced with the VM estimator, then multiplied by the pool count.

```bash
mkdir -p internal/pricing/testdata/retail/aks
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Azure Kubernetes Service' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/aks/eastus_consumption.json
```

## SQL Database

`sqldb/gp_gen5_compute_eastus.json` is the Consumption page for product
`SQL Database Single/Elastic Pool General Purpose - Compute Gen5` in `eastus`.
`NextPageLink` was null, so this file is that single response (62 items).

`sqldb/gp_storage_eastus.json` is the Consumption page for product
`SQL Database Single/Elastic Pool General Purpose - Storage` in `eastus`.
`NextPageLink` was null, so this file is that single response (5 items).

The query does not filter `armSkuName`. `GP_Gen5_2` is not an ARM SKU. Compute
rows use ARM SKUs such as `SQLDB_GP_Compute_Gen5_2`. Tests read `retailPrice`
from these files.

Compute selection is local. skuName `{n} vCore`, meter `vCore`, unit `1 Hour`
is already the price for n vCores. skuName `vCore` is the one-vCore unit row
and is not used. If `{n} vCore` is missing, the result is `NotFound`. Zone
redundancy is skuName `{n} vCore Zone Redundancy`, meter
`Zone Redundancy vCore`, and is added only when `zone_redundant=true`.

Storage selection is local. The paid meter is `General Purpose Data Stored`,
unit `1 GB/Month`. Monthly storage is `retailPrice` times size in GB, not
times 730. `General Purpose Data Stored - Free` is the included quantity at
`0` and is not the overage. Zone storage is
`General Purpose Zone Redundancy Data Stored`. IO rate meters are not
capacity.

```bash
mkdir -p internal/pricing/testdata/retail/sqldb
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'SQL Database' and productName eq 'SQL Database Single/Elastic Pool General Purpose - Compute Gen5' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/sqldb/gp_gen5_compute_eastus.json
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'SQL Database' and productName eq 'SQL Database Single/Elastic Pool General Purpose - Storage' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/sqldb/gp_storage_eastus.json
```

## Cosmos DB

`cosmosdb/eastus_consumption.json` is the Consumption page for service
`Azure Cosmos DB` in `eastus`. `NextPageLink` was null, so this file is that
single response (111 items). The query does not filter `armSkuName` or
`productName`.

Rows are selected locally. Manual throughput is product `Azure Cosmos DB`,
sku `RUs`, meter `100 RU/s`, unit `1/Hour`. The leading integer in the meter
name is the RU block. Monthly RU cost is `ru_per_second / that integer`,
times `retailPrice`, times 730. Storage is meter `Data Stored`, unit
`1 GB/Month`, and only when `size_gb` is set. Monthly storage is
`retailPrice` times size in GB, not times 730. An omitted `size_gb` has no
storage component. `multi_master=true` uses sku `mRUs`, meter `100 Multi-master RU/s`, and
that sku's `Data Stored` row.

sku `Free`, sku `Free Tier`, sku `RUm`, and meter `1000 RU/m` are not
selected. Serverless is product `Azure Cosmos DB serverless`, sku `RUs`,
meter `1M RUs`, unit `1M`. Monthly cost is `request_units / 1000000` times
`retailPrice`, not times 730, and has no storage component. The saved page
has no serverless storage meter. Autoscale is product
`Azure Cosmos DB autoscale`. Meters ending in `100 RUs` share one retail
price. The block is the `100` in that suffix. Do not also multiply by 1.5.
The saved page has no autoscale storage meter, so storage stays the
provisioned `Data Stored` row on sku `RUs`.

```bash
mkdir -p internal/pricing/testdata/retail/cosmosdb
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Azure Cosmos DB' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/cosmosdb/eastus_consumption.json
```

## Savings plans

These files record the Savings Plans spike. Tests do not read them.
Numbers and row counts are in `docs/findings/savings-plans.md`.

`api-version=2023-01-01-preview` puts a `savingsPlan` array on some
Consumption meters. `priceType eq 'SavingsPlan'` returns `Count` 0.
Rejected `priceType` values return HTTP 400, and that body is saved.
`curl -sS` is used so an error body is not dropped. `-f` would drop it.
Each file is the response body plus one trailing newline.

`Standard_D2s_v3` and `Standard_B1s` in `eastus` are the comparison SKUs.
`Standard_D2als_v7` in `eastus` is queried only because those two have no
Reservation rows. `pricetype_reservation_page1.json`,
`pricetype_reservation_preview_page1.json`, and
`pricetype_devtestconsumption_page1.json` are first pages only.
`NextPageLink` is in each file. Later pages were not downloaded. Every
other successful response here has `NextPageLink` null. HTTP 400 bodies
have no `NextPageLink`.
`contains_product_dsv3_pricetype_reservation.json` is Dedicated Host rows,
not `Standard_D2s_v3`.

Savings Plans can be shown per SKU from the preview array. The answer in
the findings is yes.

```bash
mkdir -p internal/pricing/testdata/retail/savingsplan
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'SavingsPlan'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_savingsplan.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'SavingsPlan'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_savingsplan_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'savingsplan'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_savingsplan_lower.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'savingsplan'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_savingsplan_lower_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=priceType eq 'SavingsPlan'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_savingsplan.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=priceType eq 'SavingsPlan'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_savingsplan_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=priceType eq 'Savings Plan'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_savings_plan_spaced.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'ComputeSavingsPlan'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_computesavingsplan.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'NotAType'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_notatype.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'OnDemand'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_ondemand.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Spot'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_spot.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Reserved'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_reserved.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'ReservedInstance'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_reservedinstance.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2021-10-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption_apiver_2021-10-01-preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption_apiver_2023-01-01.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode 'meterRegion=primary' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption_preview_primary.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'DevTestConsumption'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_devtestconsumption.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'DevTestConsumption'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_devtestconsumption_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=priceType eq 'DevTestConsumption'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_devtestconsumption_page1.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_nofilter.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_nofilter_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and (priceType eq 'Consumption' or priceType eq 'Reservation')" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_consumption_or_reservation_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_pricetype_reservation_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armSkuName eq 'Standard_D2s_v3' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/standard_d2s_v3_pricetype_reservation_noregion.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and skuName eq 'D2s v3' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_skuname_d2s_v3_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Virtual Machines' and contains(meterName, 'D2s v3') and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_vm_meter_contains_d2s_v3_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=priceType eq 'Reservation' and contains(meterName, 'D2s v3')" \
  -o internal/pricing/testdata/retail/savingsplan/contains_meter_d2s_v3_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and contains(armSkuName, 'D2s_v3') and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_contains_armsku_d2s_v3_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=priceType eq 'Reservation' and contains(armSkuName, 'D2s_v3')" \
  -o internal/pricing/testdata/retail/savingsplan/contains_armsku_d2s_v3_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=priceType eq 'Reservation' and contains(skuName, 'D2s v3')" \
  -o internal/pricing/testdata/retail/savingsplan/contains_skuname_d2s_v3_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Virtual Machines' and productName eq 'Virtual Machines DSv3 Series' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_vm_product_dsv3_series_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=productName eq 'Virtual Machines DSv3 Series' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/product_vm_dsv3_series_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Virtual Machines' and productName eq 'Virtual Machines DSv3 Series Windows' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_vm_product_dsv3_series_windows_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and reservationTerm eq '1 Year'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2s_v3_reservationterm_1_year.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=priceType eq 'Reservation' and contains(productName, 'DSv3')" \
  -o internal/pricing/testdata/retail/savingsplan/contains_product_dsv3_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Virtual Machines' and contains(productName, 'Savings Plan')" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_vm_contains_product_savings_plan.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and contains(productName, 'Savings Plan')" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_contains_product_savings_plan_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and serviceName eq 'Savings Plan'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_servicename_savings_plan.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and savingsPlan/any(t: t/term eq '1 Year')" \
  -o internal/pricing/testdata/retail/savingsplan/savingsplan_term_filter.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_b1s_pricetype_consumption_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_b1s_nofilter_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_b1s_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2als_v7'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2als_v7_nofilter_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2als_v7' and priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2als_v7_pricetype_reservation.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2als_v7' and (priceType eq 'Consumption' or priceType eq 'Reservation')" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2als_v7_pricetype_consumption_or_reservation_preview.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2als_v7' and reservationTerm eq '1 Year'" \
  -o internal/pricing/testdata/retail/savingsplan/eastus_standard_d2als_v7_reservationterm_1_year.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_reservation_page1.json
curl -sS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode 'api-version=2023-01-01-preview' \
  --data-urlencode "\$filter=priceType eq 'Reservation'" \
  -o internal/pricing/testdata/retail/savingsplan/pricetype_reservation_preview_page1.json
```

## Regions

`regions/standard_b1s_<region>.json` is the Consumption page for SKU
`Standard_B1s`, service `Virtual Machines`, in that region. The regions are
`eastus`, `westus2`, `northeurope`, and `not-a-region`. `NextPageLink` was
null on each response, so nothing was followed. Each file is that single
response plus one trailing newline. `not-a-region` has `Count` 0 and an
empty `Items` array.

Tests pass `Items` to `selectVMItem` for the non-Windows on-demand row.
`SortRegionPrices` reads those rows. A virtual machine quote returns the
other regions on `region_prices` and keeps the requested region as the
parent cost.

```bash
mkdir -p internal/pricing/testdata/retail/regions
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_B1s' and serviceName eq 'Virtual Machines' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/regions/standard_b1s_eastus.json
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'westus2' and armSkuName eq 'Standard_B1s' and serviceName eq 'Virtual Machines' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/regions/standard_b1s_westus2.json
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'northeurope' and armSkuName eq 'Standard_B1s' and serviceName eq 'Virtual Machines' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/regions/standard_b1s_northeurope.json
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'not-a-region' and armSkuName eq 'Standard_B1s' and serviceName eq 'Virtual Machines' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/regions/standard_b1s_not-a-region.json
```

## Golden

`../golden/` holds one expected monthly cost for each supported type.
`TestGolden` calls `GetProjectedCost` and compares `cost_per_month` to that
number. A changed stored cost fails the test.

| Type | Expected cost | Retail fixture |
| --- | --- | --- |
| `compute/VirtualMachine` | `../golden/compute_virtual_machine.txt` | `spot/standard_d2s_v3_eastus.json` |
| `storage/ManagedDisk` | `../golden/storage_managed_disk.txt` | `disk/premium_ssd_lrs_eastus.json` |
| `storage/BlobStorage` | `../golden/storage_blob_storage.txt` | `blob/hot_lrs_eastus.json` |
| `storage/StorageAccount` | `../golden/storage_storage_account.txt` | `storageaccount/general_block_blob_v2_eastus.json` |
| `web/AppServicePlan` | `../golden/web_app_service_plan.txt` | `appservice/eastus_consumption.json` |
| `web/FunctionApp` | `../golden/web_function_app.txt` | `functions/eastus_consumption.json` |
| `containerservice/KubernetesCluster` | `../golden/containerservice_kubernetes_cluster.txt` | `aks/eastus_consumption.json` |
| `sql/Database` | `../golden/sql_database.txt` | `sqldb/gp_gen5_compute_eastus.json` and `sqldb/gp_storage_eastus.json` |
| `cosmosdb/Account` | `../golden/cosmosdb_account.txt` | `cosmosdb/eastus_consumption.json` |

`disk/premium_ssd_lrs_eastus.json` copies the P4 `5.28` and P10 `19.71` rows.
The live meter names are `P4 LRS Disk` and `P10 LRS Disk` on product
`Premium SSD Managed Disks`. `blob/hot_lrs_eastus.json` copies the Hot LRS
write `0.0001` and the base data-stored `0.0208` row (100 GB is `2.08`).
The live product is `Blob Storage`, skuName `Hot LRS`, and higher
`tierMinimumUnits` bands are not the list price. Those two files are not a
full live API page.

Refresh means replacing the stored number after a deliberate fixture update.
Do not edit the quote so a stale number passes. A separate live snapshot
for two virtual machines and two managed disks is refreshed with
`-update-golden`. See `../golden/README.md`.

## Refresh

Run this from the repository root. It overwrites the fixture with the live
response and does not change the query.

```bash
mkdir -p internal/pricing/testdata/retail/spot
curl -fsS -G 'https://prices.azure.com/api/retail/prices' \
  --data-urlencode "\$filter=armRegionName eq 'eastus' and armSkuName eq 'Standard_D2s_v3' and priceType eq 'Consumption'" \
  -o internal/pricing/testdata/retail/spot/standard_d2s_v3_eastus.json
```
