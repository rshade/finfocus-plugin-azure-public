# finfocus-plugin-azure-public Strategic Roadmap

## Vision

To provide accurate, real-time Azure cost estimates for FinFocus by
querying the Azure Retail Prices API, ensuring resilience and performance
through intelligent caching and robust transport logic.

This project follows the **Spec-Driven Development (Speckit)** workflow.
See [CONTEXT.md](./CONTEXT.md) for architectural boundaries.

---

## Immediate Focus (v0.1.0 - Core Estimation)

**Goal:** First release. Quote the ten mapped types from the public Retail
Prices API. `GetProjectedCost` is the monthly quote. `GetActualCost` is
that quote times hours over 730, a running-cost estimate, not billed spend.

### Priced in the working tree

Virtual machines, including Spot, managed disks, blob storage, storage
accounts, App Service plans, Function Apps, AKS, SQL Database
`GP_Gen5`, Cosmos DB accounts, and Standard Load Balancer rules.

Still open on this milestone: live integration as a CI gate, and estimates
within 5% of the Pricing Calculator. Owner values for that check are empty.

Not quoted: NAT Gateway, Cache for Redis, database servers for PostgreSQL
and MySQL, Gateway meters, and cross-region load balancer meters. Virtual
machine scale sets are quoted from the VM meter times `instances` or
`sku.capacity`.

A virtual machine quote returns `price_options` and `region_prices`.
Both lists are advisory. The monthly cost stays the selected row.
`GetActualCost` uses the request `billing_account_id` when the caller
sends one. A dry run ignores that field. An empty request id falls
back to `FINFOCUS_BILLING_ACCOUNT_ID`. An empty id leaves the FOCUS
record unset. This file does not close GitHub issues.

**Milestone:**
[v0.1.0 - Core Estimation](https://github.com/rshade/finfocus-plugin-azure-public/milestone/4)

- [x] [#17](https://github.com/rshade/finfocus-plugin-azure-public/issues/17)
  Implement VM cost estimation (EstimateCost RPC) [L]
- [x] [#18](https://github.com/rshade/finfocus-plugin-azure-public/issues/18)
  Implement Managed Disk cost estimation [M]
- [x] [#20](https://github.com/rshade/finfocus-plugin-azure-public/issues/20)
  Create integration tests with live Azure Retail Prices API [L]
- [x] [#59](https://github.com/rshade/finfocus-plugin-azure-public/issues/59)
  Implement GetProjectedCost RPC for Azure pricing projection [M]
- [x] [#60](https://github.com/rshade/finfocus-plugin-azure-public/issues/60)
  Implement GetActualCost RPC for Azure historical cost lookup [M]
- [x] [#61](https://github.com/rshade/finfocus-plugin-azure-public/issues/61)
  Remove boundary-violating RPC stubs [S]

**Verification:**

- ~~EstimateCost returns accurate costs for Standard_B1s VM~~ (done)
- ~~Managed Disk estimates scale with size~~ (done)
- Estimates within 5% of Azure Pricing Calculator
- Integration tests exist behind the integration build tag and are not a CI gate
- ~~GetProjectedCost returns the monthly retail quote~~ (done)
- ~~GetActualCost returns that quote times hours over 730~~ (done)
- ~~Recommendations, dismiss, and budgets stay on the embedded unimplemented server~~ (done)

---

## Near-Term Vision (v0.2.0 - Quality & Testing)

**Goal:** Establish production-ready quality standards with comprehensive
testing, validation, and documentation.

**Milestone:**
[v0.2.0 - Quality & Testing](https://github.com/rshade/finfocus-plugin-azure-public/milestone/5)

**Testing & Validation:**

- [x] [#52](https://github.com/rshade/finfocus-plugin-azure-public/issues/52)
  Implement regression test suite with golden pricing data [M]
- [ ] [#53](https://github.com/rshade/finfocus-plugin-azure-public/issues/53)
  Implement pricing accuracy validation against Azure Pricing Calculator [S]
- [x] [#54](https://github.com/rshade/finfocus-plugin-azure-public/issues/54)
  Implement performance benchmarking and load testing [S]
- [x] [#55](https://github.com/rshade/finfocus-plugin-azure-public/issues/55)
  Implement chaos testing for Azure API failure scenarios [S]

**Documentation:**

- Complete README with installation and usage instructions
- Supported resources list with pricing details
- Configuration guide (regions, cache settings, options)
- Troubleshooting guide
- Contributing guide
- Changelog maintenance

**Operational Readiness:**

- Enhanced CI/CD workflows for scheduled E2E testing
- Test account setup documentation

---

## Future Vision (Long-Term)

Checkboxes below are the original milestone list. The priced-now section
says what this tree does. A commit on `main` closes an issue. This file
does not.

### v0.3.0 - Extended Services

**Milestone:**
[v0.3.0 - Extended Services](https://github.com/rshade/finfocus-plugin-azure-public/milestone/6)

**Spot VM Pricing:**

- [x] [#42](https://github.com/rshade/finfocus-plugin-azure-public/issues/42)
  Add Spot VM pricing support [S]
- Up to 90% savings for fault-tolerant workloads
- Reuses VM estimation logic, adds `Tags["pricing_model"] = "spot"`

**Plugin Discovery & Validation:**

- [x] [#43](https://github.com/rshade/finfocus-plugin-azure-public/issues/43)
  Implement DryRun validation RPC [S]
- [ ] [#44](https://github.com/rshade/finfocus-plugin-azure-public/issues/44)
  Implement GetPricingSpec RPC for plugin discovery [M]
- DryRun: Validate descriptors without API calls, preview OData filter
- PricingSpec: Machine-readable schema of supported resource types

**App Service & Functions:**

- [x] [#48](https://github.com/rshade/finfocus-plugin-azure-public/issues/48)
  Implement App Service & Azure Functions cost estimation [M]
- App Service Plans: clear SKU mapping (B1, S1, P1v2, etc.)
- Functions: consumption pricing based on executions and GB-s

**Azure Kubernetes Service (AKS):**

- [x] [#49](https://github.com/rshade/finfocus-plugin-azure-public/issues/49)
  Implement AKS cluster cost estimation [M]
- Reuses VM estimation for node pools (#17)
- Adds cluster management fee lookup (free vs paid tier)

**Storage Accounts:**

- [x] [#50](https://github.com/rshade/finfocus-plugin-azure-public/issues/50)
  Implement Storage Accounts capacity-based cost estimation [M]
- Tiers: Hot, Cool, Archive (different per-GB rates)
- Redundancy: LRS, GRS, RA-GRS affect pricing
- Scoped to capacity only (transactions are usage-based)

### v0.4.0 - Advanced Pricing & Intelligence

**Multi-Pricing Comparison:**

- [x] [#45](https://github.com/rshade/finfocus-plugin-azure-public/issues/45)
  Multi-pricing model comparison (Consumption vs Reserved vs
  Savings Plans) [L]
- Return Consumption, 1-Year RI, and 3-Year RI side-by-side
- Calculate savings percentages (25-72% typical)

**Regional Intelligence:**

- [x] [#47](https://github.com/rshade/finfocus-plugin-azure-public/issues/47)
  Regional price heatmap — cross-region cost comparison for SKUs [L]
- Query all regions for a given SKU, sorted by price
- Identify cheapest/most expensive regions (30-40% variation typical)

**FinOps Standards Alignment:**

- [ ] [#46](https://github.com/rshade/finfocus-plugin-azure-public/issues/46)
  Align response fields with FOCUS 1.3 specification [L]
- Map to FOCUS columns: `ListCost`, `EffectiveCost`, `BilledCost`,
  `PricingUnit`, `ServiceCategory`, `ChargeType`

### v0.5.0+ - Database Services & Sustainability

**Database Services** (requires research spike #51):

- Azure SQL Database: DTU-based vs vCore-based pricing models,
  multiple tiers (Basic, Standard, Premium, Hyperscale)
- Cosmos DB: RU/s-based pricing, multi-region and consistency
  levels affect cost
- Elastic pools add complexity (shared resources)

**Carbon Footprint Estimation** (requires research spike #56):

- Aligned with AWS plugin's carbon feature
- Azure does NOT publish carbon data via Retail Prices API
- Viable approaches (boundary-safe):
  - Cloud Carbon Footprint open-source methodology
  - Static carbon intensity data by Azure region (kgCO2/kWh)
- Not viable: Azure Carbon Optimization API (requires auth)

### Research Spikes (Unversioned)

- [x] [#51](https://github.com/rshade/finfocus-plugin-azure-public/issues/51)
  Research spike: Azure SQL Database & Cosmos DB pricing mapping [M]
- [x] [#56](https://github.com/rshade/finfocus-plugin-azure-public/issues/56)
  Research spike: Carbon footprint estimation data sources [M]
- [x] [#57](https://github.com/rshade/finfocus-plugin-azure-public/issues/57)
  Research spike: Savings Plans pricing in Azure Retail Prices API [S]

---

## Completed Milestones

### 2026-Q2

#### v0.1.0 - Core Estimation (continued)

- [x] [#20](https://github.com/rshade/finfocus-plugin-azure-public/issues/20)
  Create integration tests with live Azure Retail Prices API.
  Closed 2026-04-04. [L]

### 2026-Q1

#### v0.1.0 - Core Estimation (partial)

- [x] [#61](https://github.com/rshade/finfocus-plugin-azure-public/issues/61)
  Remove boundary-violating RPC stubs. Closed 2026-03-13. [S]
- [x] [#16](https://github.com/rshade/finfocus-plugin-azure-public/issues/16)
  Implement ResourceDescriptor to Azure filter mapping [L]
- [x] [#17](https://github.com/rshade/finfocus-plugin-azure-public/issues/17)
  Implement VM cost estimation (EstimateCost RPC) [L]
- [x] [#18](https://github.com/rshade/finfocus-plugin-azure-public/issues/18)
  Implement Managed Disk cost estimation [M]
- [x] [#19](https://github.com/rshade/finfocus-plugin-azure-public/issues/19)
  Create cost calculation utilities (hourly to monthly conversion) [S]

#### Pre-release: Caching Layer

- [x] [#12](https://github.com/rshade/finfocus-plugin-azure-public/issues/12)
  Implement thread-safe in-memory cache [M]
- [x] [#13](https://github.com/rshade/finfocus-plugin-azure-public/issues/13)
  Implement TTL-based cache eviction logic [S]
- [x] [#14](https://github.com/rshade/finfocus-plugin-azure-public/issues/14)
  Implement cache key normalization [S]
- [x] [#15](https://github.com/rshade/finfocus-plugin-azure-public/issues/15)
  Add cache observability (hit/miss metrics and logging) [S]

#### Pre-release: Azure Client

- [x] [#7](https://github.com/rshade/finfocus-plugin-azure-public/issues/7)
  Implement HTTP client with retry logic [M]
- [x] [#8](https://github.com/rshade/finfocus-plugin-azure-public/issues/8)
  Define Azure Retail Prices API data models [S]
- [x] [#9](https://github.com/rshade/finfocus-plugin-azure-public/issues/9)
  Implement OData filter query builder [M]
- [x] [#10](https://github.com/rshade/finfocus-plugin-azure-public/issues/10)
  Implement pagination handler for Azure API responses [M]
- [x] [#11](https://github.com/rshade/finfocus-plugin-azure-public/issues/11)
  Implement comprehensive error handling for Azure API failures [S]

#### Pre-release: Scaffold & Transport

- [x] [#1](https://github.com/rshade/finfocus-plugin-azure-public/issues/1)
  Initialize Go module and project dependencies [S]
- [x] [#2](https://github.com/rshade/finfocus-plugin-azure-public/issues/2)
  Setup Makefile with build, test, lint targets [S]
- [x] [#3](https://github.com/rshade/finfocus-plugin-azure-public/issues/3)
  Configure CI pipeline (GitHub Actions) [L]
- [x] [#4](https://github.com/rshade/finfocus-plugin-azure-public/issues/4)
  Implement gRPC server with port discovery [M]
- [x] [#5](https://github.com/rshade/finfocus-plugin-azure-public/issues/5)
  Implement CostSourceService method stubs [M]
- [x] [#6](https://github.com/rshade/finfocus-plugin-azure-public/issues/6)
  Implement zerolog structured logging [S]

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
| v0.1.0 - Core Estimation | Active | 8/8 (100%) |
| v0.2.0 - Quality & Testing | Active | 3/4 (75%) |
| v0.3.0 - Extended Services | Active | 2/6 (33%) |

<!-- markdownlint-enable MD013 -->

**Completed Issues**: #1-#20, #42, #45, #47, #50-#52, #54-#57, #59-#61

**Checklist still open here**: 43, 44, 46, 48, 49, and 53.
Item 43 has no filter field on `DryRunResponse`. Item 44 returns one
pricing spec. Item 46 has no `charge_type` field, and
`commitment_discount_type` stays empty. Item 48 rejects `EP1` and Flex.
Item 49 prices AKS Free from the 0.05 USD per hour meter. Item 53 waits
on owner values.

The counts in the table follow the milestone checklists. The
priced-now list is the working tree. GitHub milestones were not
closed.

LOE Key: [S] = Small (1-2 days), [M] = Medium (3-5 days),
[L] = Large (5+ days)
