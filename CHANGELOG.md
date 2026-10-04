# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 0.1.0 (2026-10-04)


### Features

* **azureclient:** add Azure Retail Prices API data models ([#28](https://github.com/rshade/finfocus-plugin-azure-public/issues/28)) ([4e1ba46](https://github.com/rshade/finfocus-plugin-azure-public/commit/4e1ba46402c12cdaf2e105832a28419189381862)), closes [#8](https://github.com/rshade/finfocus-plugin-azure-public/issues/8)
* **azureclient:** add cache key normalization with eviction logging and TTL config ([#34](https://github.com/rshade/finfocus-plugin-azure-public/issues/34)) ([b1a6a3a](https://github.com/rshade/finfocus-plugin-azure-public/commit/b1a6a3a39979a3802504e43d2aa3637c86594a8b)), closes [#14](https://github.com/rshade/finfocus-plugin-azure-public/issues/14)
* **azureclient:** add comprehensive error handling for Azure API failures ([#30](https://github.com/rshade/finfocus-plugin-azure-public/issues/30)) ([b99f20f](https://github.com/rshade/finfocus-plugin-azure-public/commit/b99f20fbe67c6ff6f9b0ded9728cab159ae4b141)), closes [#11](https://github.com/rshade/finfocus-plugin-azure-public/issues/11)
* **azureclient:** add HTTP client with retry for Azure Retail Prices API ([#27](https://github.com/rshade/finfocus-plugin-azure-public/issues/27)) ([dbc2444](https://github.com/rshade/finfocus-plugin-azure-public/commit/dbc24449d51abcd2db69a0a7db269717389dae72)), closes [#7](https://github.com/rshade/finfocus-plugin-azure-public/issues/7)
* **azureclient:** add OData filter query builder with fluent API ([#31](https://github.com/rshade/finfocus-plugin-azure-public/issues/31)) ([ba74e33](https://github.com/rshade/finfocus-plugin-azure-public/commit/ba74e33f77de541b16154c261a89cbef1e7db2ee)), closes [#9](https://github.com/rshade/finfocus-plugin-azure-public/issues/9)
* **azureclient:** add pagination handler with safety limit and progress logging ([#32](https://github.com/rshade/finfocus-plugin-azure-public/issues/32)) ([9885a80](https://github.com/rshade/finfocus-plugin-azure-public/commit/9885a80c8e0322331255017239a81f553b458fba)), closes [#10](https://github.com/rshade/finfocus-plugin-azure-public/issues/10)
* **azureclient:** add thread-safe in-memory LRU cache with TTL ([#33](https://github.com/rshade/finfocus-plugin-azure-public/issues/33)) ([f7a9b67](https://github.com/rshade/finfocus-plugin-azure-public/commit/f7a9b678181027808e5eec4eeeef7cdcfd54f25b)), closes [#12](https://github.com/rshade/finfocus-plugin-azure-public/issues/12)
* **azureclient:** complete cache layer with TTL eviction and observability ([#36](https://github.com/rshade/finfocus-plugin-azure-public/issues/36)) ([1832dfd](https://github.com/rshade/finfocus-plugin-azure-public/commit/1832dfd201bc9e4078adab89e74077fb7d7f640b)), closes [#13](https://github.com/rshade/finfocus-plugin-azure-public/issues/13) [#15](https://github.com/rshade/finfocus-plugin-azure-public/issues/15)
* **build:** implement makefile build system with version injection ([#22](https://github.com/rshade/finfocus-plugin-azure-public/issues/22)) ([aac482c](https://github.com/rshade/finfocus-plugin-azure-public/commit/aac482cff1eacdd0d05fca1d4e56af9248d8c9b5)), closes [#2](https://github.com/rshade/finfocus-plugin-azure-public/issues/2)
* **estimation:** add cost conversion utilities for hourly/monthly/yearly rates ([#35](https://github.com/rshade/finfocus-plugin-azure-public/issues/35)) ([a7cf0ee](https://github.com/rshade/finfocus-plugin-azure-public/commit/a7cf0ee55276254d9ba1f975adf4cd135da3f7f3)), closes [#19](https://github.com/rshade/finfocus-plugin-azure-public/issues/19)
* **logging:** implement zerolog structured logging with trace ID propagation ([#24](https://github.com/rshade/finfocus-plugin-azure-public/issues/24)) ([f38864b](https://github.com/rshade/finfocus-plugin-azure-public/commit/f38864bbe5c42343c5a5f76dbc291c1d5f597b2c)), closes [#6](https://github.com/rshade/finfocus-plugin-azure-public/issues/6)
* **manifest:** list supported Azure resource types in the plugin manifest ([15dfeca](https://github.com/rshade/finfocus-plugin-azure-public/commit/15dfeca9410f3af5cacca626f343eb118c1e5767)), closes [#44](https://github.com/rshade/finfocus-plugin-azure-public/issues/44)
* **plugin:** advertise only the azure provider ([b48531a](https://github.com/rshade/finfocus-plugin-azure-public/commit/b48531a06804ec07b75793f8e5752a000bf8e170)), closes [#84](https://github.com/rshade/finfocus-plugin-azure-public/issues/84)
* **pricing:** accept azure-native type tokens (AZ-6.15) ([8669e06](https://github.com/rshade/finfocus-plugin-azure-public/commit/8669e06ae4f5648a32920fc08f3ea9639b7f37fc))
* **pricing:** accept Spot and storage capacity aliases ([787881e](https://github.com/rshade/finfocus-plugin-azure-public/commit/787881ec945bbdefc97a2afa431b3a9eb68ef283)), closes [#42](https://github.com/rshade/finfocus-plugin-azure-public/issues/42) [#50](https://github.com/rshade/finfocus-plugin-azure-public/issues/50)
* **pricing:** add ResourceDescriptor to Azure PriceQuery mapper ([#37](https://github.com/rshade/finfocus-plugin-azure-public/issues/37)) ([f546985](https://github.com/rshade/finfocus-plugin-azure-public/commit/f54698547cb053c60b982520b45cbd6c88981355)), closes [#16](https://github.com/rshade/finfocus-plugin-azure-public/issues/16)
* **pricing:** advertise real capabilities and serve plugin inspect ([1d7cdda](https://github.com/rshade/finfocus-plugin-azure-public/commit/1d7cdda2e8a825881a92e550c4f3219c3d79d7c5)), closes [#44](https://github.com/rshade/finfocus-plugin-azure-public/issues/44)
* **pricing:** align the actual-cost FOCUS record with FOCUS 1.3 ([4fc287c](https://github.com/rshade/finfocus-plugin-azure-public/commit/4fc287c8382b042c0e3e1eb6c14e32e432834e43)), closes [#46](https://github.com/rshade/finfocus-plugin-azure-public/issues/46)
* **pricing:** attach a FOCUS record when an account id is set (AZ-6.5) ([d2a5b17](https://github.com/rshade/finfocus-plugin-azure-public/commit/d2a5b17c4d42a5a2bb35763e6486e9ba470a7214))
* **pricing:** implement CostSourceService method stubs ([#25](https://github.com/rshade/finfocus-plugin-azure-public/issues/25)) ([31b88c9](https://github.com/rshade/finfocus-plugin-azure-public/commit/31b88c9a8cff8567c3f859337e9d656d8dbbabb2)), closes [#5](https://github.com/rshade/finfocus-plugin-azure-public/issues/5)
* **pricing:** implement EstimateCost RPC for VM pricing ([#58](https://github.com/rshade/finfocus-plugin-azure-public/issues/58)) ([7a6347f](https://github.com/rshade/finfocus-plugin-azure-public/commit/7a6347f7840bacef207fd51191e95ea2431ba34a))
* **pricing:** implement EstimateCost RPC for VM pricing ([#58](https://github.com/rshade/finfocus-plugin-azure-public/issues/58)) ([#62](https://github.com/rshade/finfocus-plugin-azure-public/issues/62)) ([c7a1f4f](https://github.com/rshade/finfocus-plugin-azure-public/commit/c7a1f4fb587127bad768bc9522d7615d14ed6a8c)), closes [#18](https://github.com/rshade/finfocus-plugin-azure-public/issues/18)
* **pricing:** parse savings plan rates and reservation term totals (AZ-6.3) ([e3040ef](https://github.com/rshade/finfocus-plugin-azure-public/commit/e3040ef62b179604668e95a5d4746ef5012a6ff3))
* **pricing:** price every mapped type from EstimateCost (AZ-6.7) ([7f75229](https://github.com/rshade/finfocus-plugin-azure-public/commit/7f752294dbb81798d15e018bce60716d78b4af91))
* **pricing:** price GetActualCost from the request descriptor ([d5ec8ea](https://github.com/rshade/finfocus-plugin-azure-public/commit/d5ec8ea6fed77530d6097c188f602548998bce74)), closes [#93](https://github.com/rshade/finfocus-plugin-azure-public/issues/93)
* **pricing:** price Standard Load Balancer rules (AZ-6.9) ([e8aca80](https://github.com/rshade/finfocus-plugin-azure-public/commit/e8aca80e03bdb12d87779b13196d8273aa30893b))
* **pricing:** quote Pulumi virtual machines ([7a9124c](https://github.com/rshade/finfocus-plugin-azure-public/commit/7a9124c90ad932eed8125d738288f487e70ffc4a))
* **pricing:** quote Pulumi virtual machines and scale sets ([a28c4ea](https://github.com/rshade/finfocus-plugin-azure-public/commit/a28c4ea7945c008e5b16b20d9ab7f862a8a24b3a))
* **pricing:** read per-type Pulumi SKU properties ([923cf75](https://github.com/rshade/finfocus-plugin-azure-public/commit/923cf7506809de9fac5cafd9d206d8e40e683b02)), closes [#69](https://github.com/rshade/finfocus-plugin-azure-public/issues/69)
* **pricing:** read Pulumi inputs from descriptor attributes ([d508847](https://github.com/rshade/finfocus-plugin-azure-public/commit/d50884761d99bfba53cceee8e4503368980d5a74)), closes [#90](https://github.com/rshade/finfocus-plugin-azure-public/issues/90)
* **pricing:** return VM price options and region prices ([dd67633](https://github.com/rshade/finfocus-plugin-azure-public/commit/dd676331338f658e8d9886295993cba37667034d)), closes [#45](https://github.com/rshade/finfocus-plugin-azure-public/issues/45) [#47](https://github.com/rshade/finfocus-plugin-azure-public/issues/47)


### Bug Fixes

* **cache:** cache empty price pages and read VM references live ([3e99008](https://github.com/rshade/finfocus-plugin-azure-public/commit/3e99008f1428ccf9e22f2ab71de8d66058534454)), closes [#75](https://github.com/rshade/finfocus-plugin-azure-public/issues/75)
* **cache:** reject bodies that are not price pages and expire empty pages sooner ([cc9dd1e](https://github.com/rshade/finfocus-plugin-azure-public/commit/cc9dd1e39c016aacb5fa94cb3b016f70a14068e4)), closes [#75](https://github.com/rshade/finfocus-plugin-azure-public/issues/75)
* **ci:** lint workflow files and fall back the release token ([b040e17](https://github.com/rshade/finfocus-plugin-azure-public/commit/b040e176268f7d68bdd992a42a05f4dcd1c6404b))
* **pricing:** bill storage bands and optional cosmos size (AZ-6.14) ([28596d8](https://github.com/rshade/finfocus-plugin-azure-public/commit/28596d8e166cae42bcde01d74cb93697d595d75d))
* **pricing:** bill zone-redundant SQL storage at the zone rate ([3f1ddb4](https://github.com/rshade/finfocus-plugin-azure-public/commit/3f1ddb4d2020807add5be3a8883a6205e7ac07e5)), closes [#77](https://github.com/rshade/finfocus-plugin-azure-public/issues/77)
* **pricing:** count FOCUS quantities and map services like Azure ([83ee980](https://github.com/rshade/finfocus-plugin-azure-public/commit/83ee9800805b0f57e0423749bc1155d6ad269b8c)), closes [#46](https://github.com/rshade/finfocus-plugin-azure-public/issues/46)
* **pricing:** drop unknown attribute values and bound the merged tags ([93a16d5](https://github.com/rshade/finfocus-plugin-azure-public/commit/93a16d565d036b8c23c09a7e18ee1f2e7c781f0e)), closes [#90](https://github.com/rshade/finfocus-plugin-azure-public/issues/90)
* **pricing:** enforce descriptor limits and tighten actual-cost tests ([7a07d7b](https://github.com/rshade/finfocus-plugin-azure-public/commit/7a07d7bdbb669918d5617bae0cde8ebbeb237b95)), closes [#93](https://github.com/rshade/finfocus-plugin-azure-public/issues/93)
* **pricing:** fall back to the legacy blob product where GPv2 lacks a sku ([27606f7](https://github.com/rshade/finfocus-plugin-azure-public/commit/27606f7c74a20698acec9f122cb3d62d9b8a9c48)), closes [#79](https://github.com/rshade/finfocus-plugin-azure-public/issues/79)
* **pricing:** honour Spot priority on EstimateCost (AZ-6.2) ([45edfdd](https://github.com/rshade/finfocus-plugin-azure-public/commit/45edfdd6a340f15a5e799cb6fccdf2c64392050f))
* **pricing:** keep pricing spec rates core would multiply exact ([d3dd153](https://github.com/rshade/finfocus-plugin-azure-public/commit/d3dd15348cdc6e9c0dfe7034ad97ea9229217825)), closes [#44](https://github.com/rshade/finfocus-plugin-azure-public/issues/44)
* **pricing:** note the request-unit product has no storage meter (AZ-6.8) ([0fcaba2](https://github.com/rshade/finfocus-plugin-azure-public/commit/0fcaba2daada4232c95b25bbc1e3db5372d8e6dd))
* **pricing:** price blob storage from General Block Blob v2 ([73b754c](https://github.com/rshade/finfocus-plugin-azure-public/commit/73b754ca4a8abe9c9d78ac7c0a2513a8a6101125)), closes [#79](https://github.com/rshade/finfocus-plugin-azure-public/issues/79)
* **pricing:** price the AKS Free control plane at zero ([d775704](https://github.com/rshade/finfocus-plugin-azure-public/commit/d7757048e7a62da1afc98cffb2a92fa9165507f0)), closes [#49](https://github.com/rshade/finfocus-plugin-azure-public/issues/49)
* **pricing:** refuse unpriceable Pulumi SKU inputs and bill disk tiers ([bf4e885](https://github.com/rshade/finfocus-plugin-azure-public/commit/bf4e8857f1e3e5c0afede225f500b24d0cf0eec5)), closes [#69](https://github.com/rshade/finfocus-plugin-azure-public/issues/69)
* **pricing:** send Web services as Other with FOCUS columns ([8a36a26](https://github.com/rshade/finfocus-plugin-azure-public/commit/8a36a269c9a50ba3d19286e7fe7ada527d846f29)), closes [#46](https://github.com/rshade/finfocus-plugin-azure-public/issues/46)
* **pricing:** stop a Windows token using the Linux meter (AZ-6.13) ([24444a4](https://github.com/rshade/finfocus-plugin-azure-public/commit/24444a4ebe9a25bae8feeb79b62ff5b9cd941471))


### Documentation

* amend constitution to v1.1.0 (docstring coverage threshold) ([#29](https://github.com/rshade/finfocus-plugin-azure-public/issues/29)) ([937003e](https://github.com/rshade/finfocus-plugin-azure-public/commit/937003e142aadad1436a4565fe9d47b61fff07fb))
* create constitution v1.0.0 (quality, testing, UX, documentation, performance) ([82e1201](https://github.com/rshade/finfocus-plugin-azure-public/commit/82e12018f4be4f582bc60c616bb74098abb00968))
* describe provider as the cloud and azure-native as a package ([620e2fa](https://github.com/rshade/finfocus-plugin-azure-public/commit/620e2fa400e726b85ad4c9be7f9aca64ee4a6add)), closes [#84](https://github.com/rshade/finfocus-plugin-azure-public/issues/84)
* mark 43, 48, and 49 done in the roadmap summary ([a383e67](https://github.com/rshade/finfocus-plugin-azure-public/commit/a383e67e4ade02f9a7fdb0d22cb3323f2a32ee88))
* match the guides to the priced types (AZ-6.11) ([ce03a65](https://github.com/rshade/finfocus-plugin-azure-public/commit/ce03a658c3ebde1723824dff12b0548fee8fd892))
* note that actual cost lacks resource inputs in the readme ([7fc8317](https://github.com/rshade/finfocus-plugin-azure-public/commit/7fc8317449647ba532952b827be4a604e1475490)), closes [#90](https://github.com/rshade/finfocus-plugin-azure-public/issues/90)
* **pricing:** link the spec issues for extra prices (AZ-6.4) ([80a96be](https://github.com/rshade/finfocus-plugin-azure-public/commit/80a96beb4f2e49eddea1182ca96e4c166072c1d7))
* record the delivered pricing issues ([eadffe3](https://github.com/rshade/finfocus-plugin-azure-public/commit/eadffe3222953a662910f7e541f8ffb31f5b936d)), closes [#51](https://github.com/rshade/finfocus-plugin-azure-public/issues/51) [#56](https://github.com/rshade/finfocus-plugin-azure-public/issues/56) [#57](https://github.com/rshade/finfocus-plugin-azure-public/issues/57) [#59](https://github.com/rshade/finfocus-plugin-azure-public/issues/59) [#60](https://github.com/rshade/finfocus-plugin-azure-public/issues/60)
* revert the hand-written changelog note ([d057ce4](https://github.com/rshade/finfocus-plugin-azure-public/commit/d057ce49dcc4e61bbe78617a04195696eb7ffa74))
* say actual cost receives no resource inputs ([4be801f](https://github.com/rshade/finfocus-plugin-azure-public/commit/4be801f00d068586dc68afaf4536efc611dfa8c2)), closes [#90](https://github.com/rshade/finfocus-plugin-azure-public/issues/90)
* write the superpowers run report (AZ-6.13) ([fc55406](https://github.com/rshade/finfocus-plugin-azure-public/commit/fc554067896d90eef48bdaa9b2e950d82d8f17a3))

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
- Standard Load Balancer quotes bill the included rules meter. A regional page with no rows is read again at price region `Global`. Omitted `rule_count` uses that meter once and covers up to 5 rules. A higher `rule_count` adds the overage meter. `data_processed_gb` is charged only when set. Gateway and cross-region meters are not quoted.
- Providers `azure` and `azure-native` are accepted, including the classic and native type tokens for the ten priced types. An `azure-native:web:WebApp` resource is priced as a function app only when `kind` is `FunctionApp`.
- `azure:compute/windowsVirtualMachine:WindowsVirtualMachine` is not quoted. That token no longer returns the Linux meter. `EstimateCost` and the FOCUS record accept `azure-native:web:WebApp` when `kind` is `FunctionApp`. A managed disk pricing spec uses `per_month` for the tier price.
- Guides describe `GetActualCost` as the public retail month times hours over 730. It is a running-cost estimate. It does not read billed spend.
