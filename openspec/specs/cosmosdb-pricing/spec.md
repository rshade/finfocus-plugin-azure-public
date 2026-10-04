# cosmosdb-pricing Specification

## Purpose

Price an Azure Cosmos DB account from the Retail Prices API for manual provisioned throughput,
multi-region writes, autoscale, and serverless, with optional storage.

## Requirements

### Requirement: Cosmos DB mapping and query

`cosmosdb/Account` (case-insensitive) and `azure:cosmosdb/account:Account` SHALL map to service
`Azure Cosmos DB` with empty `ArmSkuName` and `ProductName`; no SKU is required and a descriptor
`Sku` such as `RUm` or `1000 RU/m` SHALL NOT enter the filter. A quote SHALL make one query whose
filter has region, service, `priceType eq 'Consumption'`, and the requested currency. A missing
region SHALL be `ErrMissingRequiredFields`. A type that only starts with the account type, and
`azure:cosmosdb/database:Database`, SHALL be `ErrUnsupportedResourceType` (`Unimplemented` from
`GetProjectedCost`).

Tests: `TestMapDescriptorToQueryCosmosAccount`, `TestCosmosQueryOmitsArmSKU`,
`TestGetProjectedCostCosmosPrefixIsUnsupported`

#### Scenario: One query without SKU

- **WHEN** an account with `Sku` `1000 RU/m` and `currency=EUR` is priced
- **THEN** exactly one query is sent containing `currencyCode eq 'EUR'`
- **AND** the filter contains no `armSkuName`, `productName`, or `1000 RU/m`

### Requirement: Manual provisioned throughput

The default model SHALL use sku `RUs`, meter `100 RU/s`, unit `1/Hour`, on product
`Azure Cosmos DB`. Monthly `ru` SHALL be `(ru_per_second / N) * retailPrice * 730`, where N is the
leading integer of the meter name. `rus` SHALL alias `ru_per_second`, with `ru_per_second`
winning. Zero-priced `Free Tier` rows SHALL NOT be selected. `unit_price` SHALL be the RU retail
price and `cost_per_month` the component sum. A meter name that does not start with an integer
SHALL be `InvalidArgument` naming the meter.

Tests: `TestGetProjectedCostCosmosProvisionedFromFixture`,
`TestGetProjectedCostCosmosRUBlockComesFromMeterName`, `TestGetProjectedCostCosmosOverGRPC`,
`TestGetProjectedCostCosmosMeterMustStartWithInteger`, `TestGetActualCostCosmosDefaultWindow`

#### Scenario: 400 RU/s and 10 GB

- **WHEN** `ru_per_second=400` and `size_gb=10` are priced
- **THEN** `ru` is (400 / 100) times the RU price times 730 and `storage` is 10 times the
  `Data Stored` price

#### Scenario: Renamed meter block

- **WHEN** the RU meter is renamed `50 RU/s`
- **THEN** the RU divisor is 50

### Requirement: Optional storage

When `size_gb` (or `sizeGb`) is set, the quote SHALL add `storage` from meter `Data Stored`,
unit `1 GB/Month`, times `size_gb`, not multiplied by 730. When it is omitted, there SHALL be no
storage component. A `size_gb` of 0 SHALL be `InvalidArgument`.

Tests: `TestGetProjectedCostCosmosProvisionedFromFixture`,
`TestGetProjectedCostCosmosOmitsStorageWhenSizeMissing`, `TestGetProjectedCostCosmosInvalidArgument`

#### Scenario: Size omitted

- **WHEN** only `ru_per_second=400` is set
- **THEN** the breakdown has only `ru`

### Requirement: Multi-region writes

Tag `multi_master=true` SHALL use sku `mRUs` for both the RU meter (`100 Multi-master RU/s`) and
`Data Stored`. `multi_master=false` SHALL stay on sku `RUs`.

Tests: `TestGetProjectedCostCosmosMultiMasterFromFixture`,
`TestGetProjectedCostCosmosProvisionedFromFixture`

#### Scenario: Multi-master account

- **WHEN** `multi_master=true`, `ru_per_second=400`, and `size_gb=10`
- **THEN** `ru` uses the `mRUs` multi-master meter and differs from the single-master cost

### Requirement: Serverless

`pricing_model=serverless` (any case) SHALL use meter `1M RUs`, unit `1M`, on product
`Azure Cosmos DB serverless`, with `ru` equal to `(request_units / 1000000) * retailPrice`, not
multiplied by 730. There SHALL be no storage component even when `size_gb` is set or a storage
row exists, and `billing_detail` SHALL say `no storage meter`. A serverless meter with another
unit SHALL be `InvalidArgument` naming the unit.

Tests: `TestGetProjectedCostCosmosServerlessFromFixture`,
`TestGetProjectedCostCosmosServerlessOmitsStorageMeter`,
`TestGetProjectedCostCosmosServerlessRejectsUnit`

#### Scenario: Two million request units

- **WHEN** `pricing_model=serverless` and `request_units=2000000`
- **THEN** `ru` is 2 times the `1M RUs` price and there is no `storage` key

### Requirement: Autoscale

`pricing_model=autoscale` (any case) SHALL use the meter ending in `100 RUs` on product
`Azure Cosmos DB autoscale`, not the `AP1 Entry Price` row, with `ru` equal to
`(ru_per_second / 100) * retailPrice * 730` and no 1.5 multiplier. Storage SHALL use a
`Data Stored` row on the autoscale product when one exists, and the provisioned `Data Stored`
row otherwise. Autoscale RU rows with different prices SHALL be `InvalidArgument` naming the
meter.

Tests: `TestGetProjectedCostCosmosAutoscaleFromFixture`,
`TestGetProjectedCostCosmosAutoscaleStorageMeter`,
`TestGetProjectedCostCosmosAutoscalePricesMustAgree`

#### Scenario: Fixture with no autoscale storage row

- **WHEN** `pricing_model=autoscale`, `ru_per_second=400`, and `size_gb=10`
- **THEN** `ru` is 4 times the autoscale price times 730 and `storage` is the provisioned
  `Data Stored` price times 10

### Requirement: Invalid input and missing meters

A missing or invalid `ru_per_second` or `rus`, a missing region, an unknown `pricing_model`
(including `reserved` and `manual`), and serverless without `request_units` SHALL be
`InvalidArgument` naming the input, and a missing `size_gb` SHALL NOT be named. A missing paid
RU, storage, multi-master, serverless, or autoscale meter SHALL be `NotFound` naming the meter.

Tests: `TestGetProjectedCostCosmosInvalidArgument`,
`TestGetProjectedCostCosmosMissingMeterIsNotFound`

#### Scenario: Manual pricing model

- **WHEN** `pricing_model=manual` is set
- **THEN** the call fails with `InvalidArgument` naming `manual`

#### Scenario: Missing serverless meter

- **WHEN** the page has no `1M RUs` row and `pricing_model=serverless`
- **THEN** the call fails with `NotFound` naming `1M RUs`
