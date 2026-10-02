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
- Blob Storage and Storage Accounts bill capacity across volume bands. A size past the first `tierMinimumUnits` uses the next band's rate for the remainder.
- Cosmos DB provisioned quotes omit storage when `size_gb` is absent. The scale pricing model does the same.
- `EstimateCost` honours Virtual Machine `priority=Spot` and returns pricing category Dynamic. The interruption score stays 0 because no risk source is available.
- The Retail Prices client keeps a preview `savingsPlan` array on a Consumption meter. `ReservationHourly` treats a Reservation `retailPrice` as the term total (8,760 hours for one year, 26,280 for three). No RPC returns those extra prices. The spec proposals are issues 588 and 589.
- `GetActualCost` attaches a FOCUS record when `FINFOCUS_BILLING_ACCOUNT_ID` is set. An empty setting leaves the record unset and still returns the scaled retail cost. The request has no billing-account field (spec issue 590).
- `EstimateCost` prices every mapped resource type. Virtual machines and managed disks keep their attribute parsers. The other types use the same quote as `GetProjectedCost`.
- Cosmos DB request-unit quotes say that product publishes no storage meter. A live query on 2026-10-01 returned only the `1M RUs` meter. The AKS Free control-plane meter is 0.05 USD per hour, effective 2026-10-01, which is 36.50 per month.
