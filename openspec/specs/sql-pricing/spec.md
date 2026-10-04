# sql-pricing Specification

## Purpose

Price Azure SQL Database General Purpose Gen5 provisioned compute and storage from the Retail
Prices API, including zone redundancy, and refuse the purchasing models the plugin does not price.

## Requirements

### Requirement: SQL mapping and queries

`sql/Database` and `azure:sql/database:Database` SHALL map to service `SQL Database` with empty
`ArmSkuName` and `ProductName` in the mapped query and currency `USD` by default. Missing `vcores`
on the tag form SHALL be `ErrMissingRequiredFields` naming `vcores`. A type that only starts with
`sql/database`, and `mysql/database`, SHALL be `ErrUnsupportedResourceType`, and
`GetProjectedCost` SHALL return `Unimplemented` for them without naming `AZ-2.7`. Pricing SHALL
make one query per product, compute and storage, each filtered by region, service, product,
`priceType eq 'Consumption'`, and the requested currency, never by `armSkuName`.

Tests: `TestMapDescriptorToQuerySQLDatabase`, `TestSQLQueryUsesProductNotArmSKU`,
`TestGetProjectedCostSQLPrefixIsUnsupported`

#### Scenario: Two product queries

- **WHEN** SKU `GP_Gen5_2` with `size_gb=100` and `currency=EUR` is priced
- **THEN** exactly two queries are sent, one for
  `SQL Database Single/Elastic Pool General Purpose - Compute Gen5` and one for
  `SQL Database Single/Elastic Pool General Purpose - Storage`
- **AND** each filter contains `currencyCode eq 'EUR'` and neither contains `GP_Gen5_2`

### Requirement: GP Gen5 compute and storage

Compute SHALL use sku `{n} vCore`, meter `vCore`, unit `1 Hour`, and monthly `retailPrice * 730`
without multiplying by n. Storage SHALL use meter `General Purpose Data Stored`, unit
`1 GB/Month`, and monthly `retailPrice * size_gb` without multiplying by 730. Components SHALL be
`compute` and `storage`, `unit_price` SHALL be the hourly compute price, and `cost_per_month`
SHALL be their sum. The SKU SHALL be `GP_Gen5_{n}` in any case, or tags `tier`
(`GeneralPurpose` or `GP`), `hardware` (`Gen5`), and `vcores`; `sizeGb` SHALL alias `size_gb`.
The SKU SHALL win over the tags.

Tests: `TestGetProjectedCostSQLGPGen5FromFixture`, `TestGetProjectedCostSQLSKUBeatsTags`,
`TestGetProjectedCostSQLOverGRPC`

#### Scenario: Two vCores and 100 GB

- **WHEN** SKU `GP_Gen5_2` with `size_gb=100` is priced
- **THEN** `compute` is the `2 vCore` price times 730 and `storage` is the stored price times 100

#### Scenario: SKU beats vcores tag

- **WHEN** SKU `GP_Gen5_2` has tag `vcores=8`
- **THEN** `compute` is the `2 vCore` price times 730

### Requirement: Zone redundancy replaces storage

Tag `zone_redundant=true`, or Pulumi `zoneRedundant=true`, SHALL add component
`zone_redundancy_compute` from meter `Zone Redundancy vCore` (sku `{n} vCore Zone Redundancy`)
times 730, and SHALL bill storage on meter `General Purpose Zone Redundancy Data Stored` instead
of `General Purpose Data Stored`. `zone_redundant=false` SHALL omit the zone components. The
pricing spec for a zone-redundant database SHALL list one storage meter.

Tests: `TestGetProjectedCost_SQLZoneRedundant_StorageUsesZoneRate`,
`TestGetProjectedCost_SQLZoneRedundantWithoutLocalStorageRow_PricesZoneStorage`,
`TestGetPricingSpec_SQLZoneRedundant_ListsOneStorageMeter`,
`TestSQLRequestFrom_PulumiZoneRedundant_EnablesZoneRedundancy`,
`TestGetProjectedCostSQLGPGen5FromFixture`

#### Scenario: Calculator totals

- **WHEN** `GP_Gen5_2` with `zone_redundant=true` is priced in `eastus`
- **THEN** `cost_per_month` is 378.58 for 100 GB and 585.58 for 1000 GB

#### Scenario: No local storage row

- **WHEN** the storage page has no `General Purpose Data Stored` row
- **THEN** a zone-redundant quote still succeeds at the zone storage rate

### Requirement: Missing meters are NotFound

A missing `{n} vCore` row (even with the 1-vCore unit row present), a missing zone vCore row, a
missing zone storage row on a zone-redundant quote, or a missing paid storage row (with only
`General Purpose Data Stored - Free` present) SHALL return `NotFound`.

Tests: `TestGetProjectedCostSQLMissingVCoreRowIsNotFound`,
`TestGetProjectedCostSQLMissingZoneVCoreRowIsNotFound`,
`TestGetProjectedCost_SQLZoneRedundantWithoutZoneStorageRow_ReturnsNotFound`,
`TestGetProjectedCostSQLMissingPaidStorageIsNotFound`

#### Scenario: Only the free storage row

- **WHEN** the storage page has the free meter but no paid `General Purpose Data Stored` row
- **THEN** the call fails with `NotFound`

### Requirement: Other models are Unimplemented

DTU (`S0`), serverless (`GP_S_Gen5_2`), Business Critical (`BC_Gen5_2`), Hyperscale
(`HS_Gen5_2`), and other hardware (`GP_Fsv2_2`) SHALL return `Unimplemented` with a message
naming the model and `AZ-2.7`.

Tests: `TestGetProjectedCostSQLRefusesOtherModels`

#### Scenario: Business Critical

- **WHEN** SKU `BC_Gen5_2` is priced
- **THEN** the call fails with `Unimplemented` and the message contains `Business Critical` and
  `AZ-2.7`

### Requirement: Required fields and Pulumi SKU

A missing region, `size_gb`, or `vcores` SHALL be `InvalidArgument` naming the field. The SKU
SHALL be classic `skuName`, or native `sku.name` plus `sku.capacity` appended only to `GP_`,
`BC_`, and `HS_` names (`GP_Gen5` and 4 is `GP_Gen5_4`). A vCore name with no capacity, or a
non-numeric or negative capacity, SHALL be `InvalidArgument`.

Tests: `TestGetProjectedCostSQLMissingFields`,
`TestSQLDatabaseSKU_RealPulumiProperties_ComposesVCoreSKU`,
`TestGetProjectedCost_NativeSQLWithCapacity_PricesVCores`,
`TestGetProjectedCost_NativeSQLWithoutCapacity_ReturnsInvalidArgument`

#### Scenario: Native capacity

- **WHEN** `azure-native:sql:Database` has `Sku` `GP_Gen5` and tag `sku.capacity=4`
- **THEN** the quote succeeds and `billing_detail` names `GP_Gen5_4`

#### Scenario: Native without capacity

- **WHEN** `Sku` is `GP_Gen5` and `sku.capacity` is absent
- **THEN** the call fails with `InvalidArgument` naming `sku.capacity`
