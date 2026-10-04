# disk-pricing Specification

## Purpose

Price Azure managed disks from the Azure Retail Prices API by mapping the disk type and size to
a billed disk tier and returning that tier's monthly price.

## Requirements

### Requirement: Supported disk types

The disk type SHALL be matched case-insensitively against `Standard_LRS`, `StandardSSD_LRS`,
`Premium_SSD_LRS`, `Standard_ZRS`, `StandardSSD_ZRS`, and `Premium_ZRS`, plus the ARM name
`Premium_LRS`. Standard HDD SHALL use tier prefix `S`, Standard SSD prefix `E`, and Premium SSD
prefix `P`, with redundancy `LRS` or `ZRS`. `UltraSSD_LRS`, `PremiumV2_LRS`, an empty value, and
any other value SHALL be rejected.

Tests: `TestNormalizeDiskType`, `TestNormalizeDiskType_ARMNames_MapToPremiumSSD`

#### Scenario: Premium SSD names

- **WHEN** the disk type is `Premium_SSD_LRS`, `premium_ssd_lrs`, or `Premium_LRS`
- **THEN** it resolves to tier prefix `P` and redundancy `LRS`

#### Scenario: Unsupported disk

- **WHEN** the disk type is `UltraSSD_LRS`
- **THEN** normalization returns an error

### Requirement: Size maps to the ceiling tier

The size tier SHALL be the smallest tier whose capacity is at least the requested size in GB,
across tiers from 4 GiB to 32767 GiB. A fractional size SHALL round up to the next tier. A size
above 32767 GiB SHALL be an error.

Tests: `TestTierForSize`

#### Scenario: Ceiling match

- **WHEN** a Premium disk is 100 GB
- **THEN** the tier is `P10`

#### Scenario: Too large

- **WHEN** a Premium disk is 99999 GB
- **THEN** tier selection returns an error

### Requirement: Monthly tier price is not multiplied by 730

`GetProjectedCost` for a managed disk SHALL select the row whose meter is exactly
`{tier} {LRS|ZRS} Disk` (so `Disk Mount` rows are not selected) and SHALL return its
`retailPrice` as `cost_per_month` unchanged, with the same amount under the `storage` breakdown
key. A tier with no matching row SHALL be an error.

Tests: `TestGetProjectedCostDiskUsesMonthlyTierPrice`, `TestSelectDiskTierPrice`

#### Scenario: 100 GB Premium SSD

- **WHEN** `GetProjectedCost` is called for `Premium_SSD_LRS` with `size_gb=100` and rows `P4 LRS`
  at 5.28 and `P10 LRS` at 19.71
- **THEN** `cost_per_month` is 19.71
- **AND** `cost_breakdown["storage"]` is 19.71

#### Scenario: ZRS row

- **WHEN** tier `P10` with redundancy `ZRS` is selected from rows that include `P10 LRS Disk` and
  `P10 ZRS Disk`
- **THEN** the price is the `P10 ZRS Disk` row

### Requirement: Disk type and size from real Pulumi properties

The disk type SHALL be the descriptor `Sku` when set, then classic `storageAccountType` or native
`sku.name`, then the plugin tag `disk_type`. The Pulumi unknown placeholder SHALL be skipped. The
Pulumi `tier` property SHALL NOT be read as the disk type. The size SHALL also be read from native
`diskSizeGB`. Resource type matching SHALL accept `storage/manageddisk` and the
`azure:storage/manageddisk:ManagedDisk` token in any case and SHALL NOT match a longer type such as
`storage/manageddiskset`.

Tests: `TestDiskSKU_RealPulumiProperties_ResolvesDiskType`,
`TestDescriptorSizeGB_NativeDiskSizeGB_IsRead`, `TestIsManagedDiskResourceType`

#### Scenario: Descriptor Sku wins

- **WHEN** `Sku` is `Standard_LRS` and tag `storageAccountType` is `Premium_LRS`
- **THEN** the disk type is `Standard_LRS`

#### Scenario: Unknown placeholder

- **WHEN** `storageAccountType` is the Pulumi unknown placeholder and `disk_type` is `Standard_LRS`
- **THEN** the disk type is `Standard_LRS`

### Requirement: Premium performance tier override

For a Premium SSD disk, a Pulumi `tier` that names a higher P tier than the size tier SHALL be
billed instead of the size tier. A lower P tier (any case) and the unknown placeholder SHALL be
ignored. A value that is not an existing P tier (`Fast`, `P31`) SHALL return `InvalidArgument`.
A non-Premium disk SHALL ignore `tier`.

Tests: `TestDiskBillingTier_PerformanceTier_BillsTheHigherTier`,
`TestGetProjectedCost_ClassicDiskWithPerformanceTier_PricesThatTier`

#### Scenario: P30 on a 256 GB disk

- **WHEN** a classic disk has `storageAccountType=Premium_LRS`, `diskSizeGb=256`, and `tier=P30`
- **THEN** `cost_per_month` is the `P30 LRS` price (135.17), not the `P15 LRS` price
- **AND** `billing_detail` names `P30`

#### Scenario: Invalid tier

- **WHEN** a Premium disk has `tier=Fast`
- **THEN** the billing tier fails with `InvalidArgument`
