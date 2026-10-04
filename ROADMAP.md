# finfocus-plugin-azure-public Strategic Roadmap

## Vision

To provide accurate, real-time Azure cost estimates for FinFocus by
querying the Azure Retail Prices API, ensuring resilience and performance
through intelligent caching and robust transport logic.

This project follows the **OpenSpec** change process (`openspec/specs/`).
See [CONTEXT.md](./CONTEXT.md) for architectural boundaries.

---

## Immediate Focus (v0.1.0 - Publish the Release)

**Goal:** v0.1.0 was cut on 2026-10-04, but its GitHub release has no
binaries. Release Please tagged it `finfocus-plugin-azure-public-v0.1.0`,
and GoReleaser rejects that tag as non-semver. Until a `v0.1.0` release
carries the platform archives, nobody can install the plugin and the
FinFocus registry cannot list it.

- [ ] [#95](https://github.com/rshade/finfocus-plugin-azure-public/issues/95)
  Tag releases as `vX.Y.Z` so GoReleaser publishes the v0.1.0 assets [S]

The FinFocus registry entry, rshade/finfocus#1685, merges after this.

**What v0.1.0 prices:** virtual machines (including Spot and scale sets),
managed disks, blob storage, storage accounts, App Service plans, Function
Apps, AKS, SQL Database `GP_Gen5`, Cosmos DB accounts, and Standard Load
Balancer rules, from the public Retail Prices API. Inputs come from the
descriptor `attributes` first, then tags. `GetActualCost` is the projected
quote times hours over 730, priced from the request descriptor when the host
sends one. It is a running-cost estimate, not billed spend. Verified end to
end against FinFocus core v0.4.2.

---

## Near-Term Vision (v0.2.0)

**Goal:** Move the change process to OpenSpec once v0.1.0 is published, and
pick up finfocus-spec fixes in patch releases.

- [ ] [#92](https://github.com/rshade/finfocus-plugin-azure-public/issues/92)
  Adopt OpenSpec after v0.1.0 and freeze Spec Kit history [L]

Its gate is a published `v0.1.0` release, so it follows [#95](https://github.com/rshade/finfocus-plugin-azure-public/issues/95).

**Upstream follow-ups** (tracked elsewhere; adopt in a patch release such
as v0.1.1, never a blocker):

- rshade/finfocus-spec#625: let plugins run conformance with their own
  sample resource
- rshade/finfocus-spec#626: keep handler gRPC status codes instead of
  rewrapping them as `Internal`
- rshade/finfocus#1670, #1682, #1683, #1684: core error reporting, version
  display, `plugin inspect` lookup, and the actual-cost `sku` tag

---

## Future Vision (Long-Term)

**Database services beyond `GP_Gen5`:** DTU, serverless, Business
Critical, Hyperscale, and elastic pools return `Unimplemented` today
(AZ-2.7). Cosmos DB covers manual, autoscale, and serverless RU pricing.

**More resource types:** NAT Gateway, Cache for Redis, and PostgreSQL and
MySQL servers are not priced. AKS `defaultNodePool` and `agentPoolProfiles`
are not priced (AZ-7.5, AZ-7.6).

**Carbon footprint:** findings only, in `docs/findings/carbon.md`. Azure
publishes no carbon data through the Retail Prices API, and its Carbon
Optimization API needs authentication. A boundary-safe estimator would use
the Cloud Carbon Footprint methodology or static regional intensity data.

---

## Completed Milestones

### 2026-Q4

- [x] [#93](https://github.com/rshade/finfocus-plugin-azure-public/issues/93) `pricing`: Price GetActualCost from the v0.7.4 request descriptor. Closed 2026-10-04. [S]
- [x] [#90](https://github.com/rshade/finfocus-plugin-azure-public/issues/90) `pricing`: Read Pulumi inputs from v0.7.3 descriptor attributes. Closed 2026-10-04. [M]
- [x] [#87](https://github.com/rshade/finfocus-plugin-azure-public/issues/87) `build`: Adopt finfocus-spec v0.7.2 manifest writer and validator. Closed 2026-10-03. [S]
- [x] [#84](https://github.com/rshade/finfocus-plugin-azure-public/issues/84) `transport`: Advertise only the azure provider. Closed 2026-10-03. [S]
- [x] [#79](https://github.com/rshade/finfocus-plugin-azure-public/issues/79) `pricing`: Quote blob storage from General Block Blob v2. Closed 2026-10-03. [S]
- [x] [#77](https://github.com/rshade/finfocus-plugin-azure-public/issues/77) `pricing`: Bill zone-redundant SQL storage at the zone rate. Closed 2026-10-03. [S]
- [x] [#75](https://github.com/rshade/finfocus-plugin-azure-public/issues/75) `cache`: Cache empty price pages; live VM integration references. Closed 2026-10-03. [S]
- [x] [#69](https://github.com/rshade/finfocus-plugin-azure-public/issues/69) `pricing`: Derive SKU from per-type Pulumi properties and dotted tags. Closed 2026-10-03. [M]
- [x] [#53](https://github.com/rshade/finfocus-plugin-azure-public/issues/53) `testing`: Pricing accuracy check against the Pricing Calculator. Closed 2026-10-03. [S]
- [x] [#46](https://github.com/rshade/finfocus-plugin-azure-public/issues/46) `pricing`: FOCUS 1.3 actual-cost record. Closed 2026-10-03. [L]
- [x] [#44](https://github.com/rshade/finfocus-plugin-azure-public/issues/44) `pricing`: GetPricingSpec per resource for plugin discovery. Closed 2026-10-03. [M]
- [x] [#43](https://github.com/rshade/finfocus-plugin-azure-public/issues/43) `pricing`: DryRun descriptor validation. Closed 2026-10-03. [S]
- [x] [#48](https://github.com/rshade/finfocus-plugin-azure-public/issues/48) `pricing`: App Service plan and Function App estimation. Closed 2026-10-03. [M]
- [x] [#49](https://github.com/rshade/finfocus-plugin-azure-public/issues/49) `pricing`: AKS control plane and node pool estimation. Closed 2026-10-03. [M]
- [x] [#60](https://github.com/rshade/finfocus-plugin-azure-public/issues/60) `pricing`: GetActualCost as the monthly quote times hours over 730. Closed 2026-10-02. [M]
- [x] [#59](https://github.com/rshade/finfocus-plugin-azure-public/issues/59) `pricing`: GetProjectedCost monthly retail quote. Closed 2026-10-02. [M]
- [x] [#57](https://github.com/rshade/finfocus-plugin-azure-public/issues/57) `research`: Savings Plans pricing in the Retail Prices API. Closed 2026-10-02. [S]
- [x] [#56](https://github.com/rshade/finfocus-plugin-azure-public/issues/56) `research`: Carbon footprint data sources (findings only). Closed 2026-10-02. [M]
- [x] [#55](https://github.com/rshade/finfocus-plugin-azure-public/issues/55) `testing`: Chaos tests for Azure API failures. Closed 2026-10-02. [S]
- [x] [#54](https://github.com/rshade/finfocus-plugin-azure-public/issues/54) `testing`: Performance benchmarks and load tests. Closed 2026-10-02. [S]
- [x] [#52](https://github.com/rshade/finfocus-plugin-azure-public/issues/52) `testing`: Regression suite with golden pricing data. Closed 2026-10-02. [M]
- [x] [#51](https://github.com/rshade/finfocus-plugin-azure-public/issues/51) `research`: SQL Database and Cosmos DB pricing mapping. Closed 2026-10-02. [M]
- [x] [#50](https://github.com/rshade/finfocus-plugin-azure-public/issues/50) `pricing`: Storage account capacity estimation. Closed 2026-10-02. [M]
- [x] [#47](https://github.com/rshade/finfocus-plugin-azure-public/issues/47) `pricing`: Cross-region price comparison for VM SKUs. Closed 2026-10-02. [L]
- [x] [#45](https://github.com/rshade/finfocus-plugin-azure-public/issues/45) `pricing`: Consumption, Spot, Savings Plan, and Reservation options. Closed 2026-10-02. [L]
- [x] [#42](https://github.com/rshade/finfocus-plugin-azure-public/issues/42) `pricing`: Spot VM pricing. Closed 2026-10-02. [S]

### 2026-Q2

- [x] [#20](https://github.com/rshade/finfocus-plugin-azure-public/issues/20) `testing`: Integration tests against the live Retail Prices API. Closed 2026-04-04. [L]

### 2026-Q1

- [x] [#61](https://github.com/rshade/finfocus-plugin-azure-public/issues/61) `transport`: Remove boundary-violating RPC stubs. Closed 2026-03-13. [S]
- [x] [#18](https://github.com/rshade/finfocus-plugin-azure-public/issues/18) `pricing`: Managed disk cost estimation. Closed 2026-03-13. [M]
- [x] [#17](https://github.com/rshade/finfocus-plugin-azure-public/issues/17) `pricing`: VM cost estimation (EstimateCost). Closed 2026-03-06. [L]
- [x] [#16](https://github.com/rshade/finfocus-plugin-azure-public/issues/16) `pricing`: ResourceDescriptor to Azure filter mapping. Closed 2026-03-04. [L]
- [x] [#19](https://github.com/rshade/finfocus-plugin-azure-public/issues/19) `estimation`: Hourly to monthly cost utilities. Closed 2026-03-04. [S]
- [x] [#13](https://github.com/rshade/finfocus-plugin-azure-public/issues/13) `cache`: TTL-based cache eviction. Closed 2026-03-04. [S]
- [x] [#15](https://github.com/rshade/finfocus-plugin-azure-public/issues/15) `cache`: Cache hit and miss observability. Closed 2026-03-04. [S]
- [x] [#12](https://github.com/rshade/finfocus-plugin-azure-public/issues/12) `cache`: Thread-safe in-memory cache. Closed 2026-03-03. [M]
- [x] [#14](https://github.com/rshade/finfocus-plugin-azure-public/issues/14) `cache`: Cache key normalization. Closed 2026-03-03. [S]
- [x] [#10](https://github.com/rshade/finfocus-plugin-azure-public/issues/10) `azureclient`: Pagination handler for API responses. Closed 2026-03-03. [M]
- [x] [#9](https://github.com/rshade/finfocus-plugin-azure-public/issues/9) `azureclient`: OData filter query builder. Closed 2026-03-01. [M]
- [x] [#11](https://github.com/rshade/finfocus-plugin-azure-public/issues/11) `azureclient`: Error handling for Azure API failures. Closed 2026-02-28. [S]
- [x] [#7](https://github.com/rshade/finfocus-plugin-azure-public/issues/7) `azureclient`: HTTP client with retry. Closed 2026-02-04. [M]
- [x] [#8](https://github.com/rshade/finfocus-plugin-azure-public/issues/8) `azureclient`: Retail Prices API data models. Closed 2026-02-04. [S]
- [x] [#3](https://github.com/rshade/finfocus-plugin-azure-public/issues/3) `build`: CI pipeline on GitHub Actions. Closed 2026-02-03. [L]
- [x] [#5](https://github.com/rshade/finfocus-plugin-azure-public/issues/5) `transport`: CostSourceService method stubs. Closed 2026-02-03. [M]
- [x] [#6](https://github.com/rshade/finfocus-plugin-azure-public/issues/6) `logging`: zerolog structured logging. Closed 2026-02-03. [S]
- [x] [#4](https://github.com/rshade/finfocus-plugin-azure-public/issues/4) `transport`: gRPC server with port discovery. Closed 2026-02-02. [M]
- [x] [#2](https://github.com/rshade/finfocus-plugin-azure-public/issues/2) `build`: Makefile with build, test, and lint targets. Closed 2026-01-23. [S]
- [x] [#1](https://github.com/rshade/finfocus-plugin-azure-public/issues/1) `build`: Go module and dependencies. Closed 2026-01-22. [S]

---

## Boundary Safeguards

The following features violate architectural constraints defined in
[CONTEXT.md](./CONTEXT.md) and are not planned:

- **No Authenticated Azure APIs**: Do not require Azure Subscription,
  Tenant ID, or `az login`. Strictly consume the unauthenticated
  Retail Prices API.
- **No Persistent Storage**: In-memory TTL cache only. No databases
  or filesystem writes.
- **No Infrastructure Mutation**: Read-only cost calculation from
  `ResourceDescriptor` inputs.
- **No Bulk Data Embedding**: Fetch pricing dynamically, never embed
  the Azure pricing catalog.
- **Cost Optimization Recommendations**: Requires usage data + Azure
  authentication (delegate to `finfocus-plugin-azure-authenticated`).
- **Budget & Alerting**: Requires persistent storage (FinFocus Core
  responsibility).
- **Historical Cost Analysis**: Requires Azure Cost Management API
  authentication (delegate to `finfocus-plugin-azure-authenticated`).

---

## Milestone Progress

<!-- markdownlint-disable MD013 -->

| Milestone | Status | Progress |
| --- | --- | --- |
| Pre-release: Scaffold & Transport | Complete | 6/6 (100%) |
| Pre-release: Azure Client | Complete | 5/5 (100%) |
| Pre-release: Caching Layer | Complete | 4/4 (100%) |
| v0.1.0 - Core Estimation | Released, assets pending (#95) | 8/8 (100%) |
| v0.2.0 - Quality & Testing | Complete | 4/4 (100%) |
| v0.3.0 - Extended Services | Complete | 6/6 (100%) |

<!-- markdownlint-enable MD013 -->

The v0.1.0, v0.2.0, and v0.3.0 GitHub milestones have no open issues but
are not closed.

**Known limitations**: `DryRunResponse` has no filter field (#43). Function
Apps reject `EP1` and Flex (#48). The AKS Free control plane is 0, and the
0.05 USD per hour retail meter is not billed (#49).

LOE Key: [S] = Small (1-2 hours), [M] = Medium (half day to 1 day),
[L] = Large (multi-day)
