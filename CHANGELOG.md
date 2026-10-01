# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-10-01

### Added

- Virtual Machines, with Spot selected by the priority tag
- Managed Disks
- Blob Storage
- Storage Accounts
- App Service plans
- Function Apps
- AKS control plane
- SQL Database, GP Gen5 provisioned
- Cosmos DB accounts
- `GetProjectedCost`, `GetActualCost`, `DryRun`, and `GetPricingSpec`

### Fixed

- Managed Disk and Blob Storage queries use the Retail Prices `productName` and `skuName`. The live disk meter is `{tier} {LRS|ZRS} Disk` on service `Storage`. Blob capacity uses the `Blob Storage` base tier.
- `GetPluginInfo` reports spec version `v0.7.0`. `EstimateCost` selects the on-demand Linux VM meter. Blob Storage returns not found when no `Data Stored` meter matches.
