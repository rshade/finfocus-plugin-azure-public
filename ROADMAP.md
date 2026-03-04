# finfocus-plugin-azure-public Strategic Roadmap

## Vision

To provide accurate, real-time Azure cost estimates for FinFocus by
querying the Azure Retail Prices API, ensuring resilience and performance
through intelligent caching and robust transport logic.

This project follows the **Spec-Driven Development (Speckit)** workflow.
See [CONTEXT.md](./CONTEXT.md) for architectural boundaries.

---

## Immediate Focus (v0.3.0 - Caching Layer)

**Goal:** Prevent API throttling and improve performance for repetitive
lookups.

**Milestone:**
[v0.3.0 - Caching Layer](https://github.com/rshade/finfocus-plugin-azure-public/milestone/3)

- [ ] [#13](https://github.com/rshade/finfocus-plugin-azure-public/issues/13)
  Implement TTL-based cache eviction logic [S]
- [x] [#14](https://github.com/rshade/finfocus-plugin-azure-public/issues/14)
  Implement cache key normalization [S]
- [ ] [#15](https://github.com/rshade/finfocus-plugin-azure-public/issues/15)
  Add cache observability (hit/miss metrics and logging) [S]

**Verification:**

- 100 concurrent requests for same SKU show >80% cache hit rate
- Race detector passes (go test -race)
- Cache size stays bounded (LRU eviction works)
- Cache metrics logged periodically

---

## Near-Term Vision (v0.4.0 - Field Mapping & Estimation)

**Goal:** Connect the FinFocus generic `ResourceDescriptor` to
Azure-specific queries.

**Milestone:**
[v0.4.0 - Field Mapping & Estimation](https://github.com/rshade/finfocus-plugin-azure-public/milestone/4)

- [ ] [#16](https://github.com/rshade/finfocus-plugin-azure-public/issues/16)
  Implement ResourceDescriptor to Azure filter mapping [L]
- [ ] [#17](https://github.com/rshade/finfocus-plugin-azure-public/issues/17)
  Implement VM cost estimation (EstimateCost RPC) [L]
- [ ] [#18](https://github.com/rshade/finfocus-plugin-azure-public/issues/18)
  Implement Managed Disk cost estimation [M]
- [ ] [#19](https://github.com/rshade/finfocus-plugin-azure-public/issues/19)
  Create cost calculation utilities (hourly to monthly conversion) [S]
- [ ] [#20](https://github.com/rshade/finfocus-plugin-azure-public/issues/20)
  Create integration tests with live Azure Retail Prices API [L]

**Verification:**

- EstimateCost returns accurate costs for Standard_B1s VM
- Managed Disk estimates scale with size
- Estimates within 5% of Azure Pricing Calculator
- Integration tests pass against live API

---

## Future Vision (Long-Term)

### v0.5.0 - Quality & Operations

**Goal:** Establish production-ready quality standards with comprehensive
testing, validation, and documentation.

**Testing & Validation:**

- E2E tests with real Azure Retail Prices API (weekly/monthly CI runs)
- Pricing accuracy validation against official Azure Pricing Calculator
  (within ±5%)
- Regional coverage verification for all target Azure regions
- Regression test suite with golden pricing data
- Performance benchmarking and load testing
- Chaos testing for API failure scenarios

**Documentation:**

- Complete README with installation and usage instructions
- Supported resources list with pricing details
- Configuration guide (regions, cache settings, options)
- Troubleshooting guide
- Contributing guide
- Changelog maintenance

**Regional Coverage:**

- Document all supported Azure regions
- Verify pricing data accuracy for each region
- Handle region-specific resource variations
- Test edge cases (new regions, deprecated regions)

**Operational Readiness:**

- Enhanced CI/CD workflows for scheduled E2E testing
- Test account setup documentation
- Budget alerts for test accounts ($50/month limit)
- Resource tagging strategy for cost tracking

See [issues.md](./issues.md) for detailed implementation guidance on
testing and validation.

### v0.6.0 - Extended Service Coverage

**Services:**

- App Service & Functions cost estimation
- Azure Kubernetes Service (AKS) cluster estimation
- Storage Accounts (capacity-based)
- Spot VM pricing support

**Research Spikes:**

- Azure SQL Database / Cosmos DB pricing mapping
- Savings Plans / Reserved Instances API availability
- Carbon footprint data sources

See [BRAINSTORMING.md](./BRAINSTORMING.md) for detailed analysis of
each feature.

### v0.7.0+ - Advanced Features (Pending Research)

**Database Services** (conditional on research):

- Azure SQL Database cost estimation
- Cosmos DB (RU/s-based pricing)

**Advanced Pricing Models** (conditional on research):

- Reserved Instances support
- Savings Plans discount calculation

**Sustainability** (conditional on research):

- Carbon footprint estimation (aligned with AWS plugin)

---

## Completed Milestones

### Q1 2026

#### v0.1.0 - Scaffold & Transport

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

#### v0.2.0 - Azure Client

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

#### v0.3.0 - Caching Layer (partial)

- [x] [#12](https://github.com/rshade/finfocus-plugin-azure-public/issues/12)
  Implement thread-safe in-memory cache [M]
- [x] [#14](https://github.com/rshade/finfocus-plugin-azure-public/issues/14)
  Implement cache key normalization [S]

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
  authentication (delegate to separate plugin).
- **Budget & Alerting**: Requires persistent storage (FinFocus Core
  responsibility).
- **Historical Cost Analysis**: Requires Azure Cost Management API
  authentication (delegate to separate plugin).

---

## Milestone Progress

| Milestone | Status | Progress |
| --- | --- | --- |
| v0.1.0 - Scaffold & Transport | Complete | 6/6 (100%) |
| v0.2.0 - Azure Client | Complete | 5/5 (100%) |
| v0.3.0 - Caching Layer | Active | 2/4 (50%) |
| v0.4.0 - Field Mapping & Estimation | Not Started | 0/5 (0%) |

**Completed Issues**: #1, #2, #3, #4, #5, #6, #7, #8, #9, #10, #11, #12, #14

**Total Core Roadmap**: 20 issues across 4 phases (13 completed)

LOE Key: [S] = Small (1-2 days), [M] = Medium (3-5 days),
[L] = Large (5+ days)
