# storage-pricing Specification

## Purpose

Price Azure storage accounts and blob storage capacity from the `General Block Blob v2` product
of the Azure Retail Prices API, using the `Data Stored` meter for the access tier and redundancy.

## Requirements

### Requirement: Storage account capacity is retail price times size

`GetProjectedCost` for `storage/StorageAccount` (including Pulumi
`azure:storage/storageAccount:StorageAccount`) SHALL price the `{Tier} {Redundancy} Data Stored`
row (unit `1 GB/Month`) of `General Block Blob v2` and SHALL return `cost_per_month` equal to
`retailPrice * size_gb`, not multiplied by 730. `unit_price` SHALL be the per-GB price, and the
monthly amount SHALL be under the `storage` breakdown key. The SKU SHALL be accepted as `Sku`
in any case, or as tags `tier` or `access_tier` plus `redundancy`; `capacity_gb` SHALL be an
alias of `size_gb`.

Tests: `TestGetProjectedCostStorageAccountHotLRSFromFixture`,
`TestGetProjectedCostStorageAccountOverGRPC`

#### Scenario: Hot LRS, 100 GB

- **WHEN** a storage account with `Sku=Hot LRS` and `size_gb=100` is quoted in `eastus`
- **THEN** `cost_per_month` is the `Hot LRS Data Stored` retail price times 100
- **AND** it is not that amount times 730

#### Scenario: Tags instead of Sku

- **WHEN** `Sku` is empty and the tags are `access_tier=hot`, `redundancy=lrs`, `size_gb=100`
- **THEN** the quote equals the `Hot LRS` quote

### Requirement: Storage account query shape

The storage account query SHALL filter on region, service `Storage`, product
`General Block Blob v2`, price type `Consumption`, and the requested currency, and SHALL NOT
filter on `armSkuName`. `MapDescriptorToQuery` SHALL leave `ArmSkuName` empty, and SHALL return
`ErrMissingRequiredFields` naming `sku` when neither `Sku` nor the tier tags are set. `Supports`
SHALL accept a storage account described by `tier` and `redundancy` tags.

Tests: `TestStorageAccountQueryOmitsArmSKU`, `TestMapDescriptorToQueryStorageAccount`,
`TestSupportsStorageAccountFromTags`

#### Scenario: EUR query

- **WHEN** a `Hot LRS` account is quoted with tag `currency=EUR`
- **THEN** the `$filter` contains `productName eq 'General Block Blob v2'` and
  `currencyCode eq 'EUR'`
- **AND** it does not contain `armSkuName`

### Requirement: SKU resolution from real Pulumi properties

The descriptor `Sku` SHALL win over the `tier` and `redundancy` tags. Classic `accountTier` plus
`accountReplicationType` SHALL resolve to `{accessTier} {Redundancy}` with access tier default
`Hot` (for example `RAGRS` becomes `RA-GRS`), and SHALL win over the plugin `tier` and
`redundancy` tags. A native ARM SKU such as `Standard_GRS` SHALL take its access tier from
`accessTier` or `access_tier`, default `Hot`. Native `kind=StorageV2` SHALL be priced. Native
`kind=BlobStorage`, classic `accountKind=Storage`, and `Premium` performance SHALL be rejected.

Tests: `TestGetProjectedCostStorageAccountSKUBeatsTags`,
`TestStorageAccountSKU_RealPulumiProperties_ResolvesTierAndRedundancy`,
`TestMapDescriptorToQuery_UnpriceablePulumiInput_ReturnsError`

#### Scenario: Native ARM SKU

- **WHEN** `Sku` is `Standard_RAGZRS` with no access tier
- **THEN** the storage SKU is `Hot RA-GZRS`

#### Scenario: BlobStorage kind

- **WHEN** an `azure-native:storage:StorageAccount` has `Sku=Standard_LRS` and `kind=BlobStorage`
- **THEN** `MapDescriptorToQuery` returns an error

### Requirement: Storage account input errors

An unknown tier (`Premium`), redundancy (`LOCAL`), or access tier (`Frozen`) SHALL return
`InvalidArgument` naming that value. A missing size SHALL return `InvalidArgument` naming
`size_gb`. When the product has no `Data Stored` row for the SKU, the call SHALL return
`NotFound` naming the tier and the redundancy.

Tests: `TestGetProjectedCostStorageAccountRejectsUnknownSKU`,
`TestGetProjectedCostStorageAccountMissingSize`,
`TestGetProjectedCostStorageAccountMissingMeterIsNotFound`

#### Scenario: Archive ZRS missing

- **WHEN** an `Archive ZRS` account is quoted and the page has no `Archive ZRS Data Stored` row
- **THEN** the call fails with `NotFound` and the message names `Archive` and `ZRS`

### Requirement: Blob storage uses Data Stored meters with marginal bands

`GetProjectedCost` for `storage/BlobStorage` SHALL price only meters whose name contains
`Data Stored`, ignoring operation meters. Each GB SHALL be priced at the band it falls in, where
a band starts at its `tierMinimumUnits`. A size inside the first band SHALL be
`retailPrice * size_gb`, and `unit_price` SHALL be the first-band price. A page with no
`Data Stored` meter SHALL be `ErrNotFound`.

Tests: `TestGetProjectedCostBlobScalesBySizeAndPrefersDataStored`,
`TestChosenBlobStoredWriteOperationsReturnsNotFound`

#### Scenario: First band

- **WHEN** `Hot LRS` blob storage of 100 GB is quoted with a first band of 0.0208
- **THEN** `cost_per_month` is `0.0208 * 100`

#### Scenario: Two bands

- **WHEN** the size is 60000 GB and the second band starts at 51200 GB at 0.019968
- **THEN** `cost_per_month` is `51200 * 0.0208 + 8800 * 0.019968`

### Requirement: Blob storage falls back to the legacy Blob Storage product

The blob query SHALL filter on product `General Block Blob v2` and `skuName` equal to the given
SKU. When that product has a `Data Stored` row for the SKU, the quote SHALL use it with one
request. Otherwise the quote SHALL make a second request for the legacy `Blob Storage` product,
price from it, and add a `billing_detail` note naming `Blob Storage`. When neither product sells
the SKU, the call SHALL return `NotFound` naming the SKU.

Tests: `TestQuoteBlob_Query_UsesGeneralBlockBlobV2Product`,
`TestQuoteBlob_ProductWithoutSKU_FallsBackToLegacyProduct`,
`TestQuoteBlob_NeitherProductSellsSKU_ReturnsNotFoundNamingSKU`

#### Scenario: Hot GRS only on the legacy product

- **WHEN** `Hot GRS` 100 GB is quoted in `israelcentral` and only `Blob Storage` has the row at
  0.065494
- **THEN** `cost_per_month` is 6.5494 after two requests
- **AND** `billing_detail` names `Blob Storage`

#### Scenario: Neither product

- **WHEN** `Cold RA-GRS` is sold by neither product
- **THEN** the call fails with `NotFound` and the error names `Cold RA-GRS`
