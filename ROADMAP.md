# finfocus-plugin-azure-public Strategic Roadmap

This roadmap outlines the development of the Azure Retail Pricing plugin for FinFocus. It follows the **Spec-Driven Development (Speckit)** workflow.

## Mission Statement
To provide accurate, real-time Azure cost estimates for FinFocus by querying the Azure Retail Prices API, ensuring resilience and performance through intelligent caching and robust transport logic.

---

## Phase 1: Scaffold & Transport (v0.1.0)
**Goal:** Establish the plugin structure, build system, and basic gRPC connectivity.

**Milestone:** [v0.1.0 - Scaffold & Transport](https://github.com/rshade/finfocus-plugin-azure-public/milestone/1)

**Issues:**
- [#1](https://github.com/rshade/finfocus-plugin-azure-public/issues/1) Initialize Go module and project dependencies
- [#2](https://github.com/rshade/finfocus-plugin-azure-public/issues/2) Setup Makefile with build, test, lint targets
- [#3](https://github.com/rshade/finfocus-plugin-azure-public/issues/3) Configure CI pipeline (GitHub Actions)
- [#4](https://github.com/rshade/finfocus-plugin-azure-public/issues/4) Implement gRPC server with port discovery
- [#5](https://github.com/rshade/finfocus-plugin-azure-public/issues/5) Implement CostSourceService method stubs
- [#6](https://github.com/rshade/finfocus-plugin-azure-public/issues/6) Implement zerolog structured logging

**Verification:**
- Binary builds and starts successfully
- PORT announcement appears on stdout
- All logs output to stderr in JSON format
- gRPC calls accepted without crashing

## Phase 2: The Azure Client (v0.2.0)
**Goal:** Implement the HTTP client capable of querying the Azure Retail API reliably.

**Milestone:** [v0.2.0 - Azure Client](https://github.com/rshade/finfocus-plugin-azure-public/milestone/2)

**Issues:**
- [#7](https://github.com/rshade/finfocus-plugin-azure-public/issues/7) Implement HTTP client with retry logic for Azure Retail Prices API
- [#8](https://github.com/rshade/finfocus-plugin-azure-public/issues/8) Define Azure Retail Prices API data models
- [#9](https://github.com/rshade/finfocus-plugin-azure-public/issues/9) Implement OData filter query builder
- [#10](https://github.com/rshade/finfocus-plugin-azure-public/issues/10) Implement pagination handler for Azure API responses
- [#11](https://github.com/rshade/finfocus-plugin-azure-public/issues/11) Implement comprehensive error handling for Azure API failures

**Verification:**
- Can query live Azure API for VM pricing
- Pagination follows NextPageLink correctly
- 429 errors trigger retry with backoff
- Integration tests pass with live API

## Phase 3: The Caching Layer (v0.3.0)
**Goal:** Prevent API throttling and improve performance for repetitive lookups.

**Milestone:** [v0.3.0 - Caching Layer](https://github.com/rshade/finfocus-plugin-azure-public/milestone/3)

**Issues:**
- [#12](https://github.com/rshade/finfocus-plugin-azure-public/issues/12) Implement thread-safe in-memory cache
- [#13](https://github.com/rshade/finfocus-plugin-azure-public/issues/13) Implement TTL-based cache eviction logic
- [#14](https://github.com/rshade/finfocus-plugin-azure-public/issues/14) Implement cache key normalization
- [#15](https://github.com/rshade/finfocus-plugin-azure-public/issues/15) Add cache observability (hit/miss metrics and logging)

**Verification:**
- 100 concurrent requests for same SKU show >80% cache hit rate
- Race detector passes (go test -race)
- Cache size stays bounded (LRU eviction works)
- Cache metrics logged periodically

## Phase 4: Field Mapping & Estimation (v0.4.0)
**Goal:** Connect the FinFocus generic `ResourceDescriptor` to Azure-specific queries.

**Milestone:** [v0.4.0 - Field Mapping & Estimation](https://github.com/rshade/finfocus-plugin-azure-public/milestone/4)

**Issues:**
- [#16](https://github.com/rshade/finfocus-plugin-azure-public/issues/16) Implement ResourceDescriptor to Azure filter mapping
- [#17](https://github.com/rshade/finfocus-plugin-azure-public/issues/17) Implement VM cost estimation (EstimateCost RPC)
- [#18](https://github.com/rshade/finfocus-plugin-azure-public/issues/18) Implement Managed Disk cost estimation
- [#19](https://github.com/rshade/finfocus-plugin-azure-public/issues/19) Create cost calculation utilities (hourly to monthly conversion)
- [#20](https://github.com/rshade/finfocus-plugin-azure-public/issues/20) Create integration tests with live Azure Retail Prices API

**Verification:**
- EstimateCost returns accurate costs for Standard_B1s VM
- Managed Disk estimates scale with size
- Estimates within 5% of Azure Pricing Calculator
- Integration tests pass against live API

---

## Future Vision

### v0.5.0 - Quality & Operations (Planned)

**Goal:** Establish production-ready quality standards with comprehensive testing,
validation, and documentation.

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

### v0.6.0 - Extended Service Coverage (Planned)

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

### Out of Scope

The following features violate architectural constraints and are not planned:

- **Cost Optimization Recommendations**: Requires usage data + Azure authentication (delegate to separate plugin)
- **Budget & Alerting**: Requires persistent storage (FinFocus Core responsibility)
- **Historical Cost Analysis**: Requires Azure Cost Management API authentication (delegate to separate plugin)

### Completed Milestones

- ✅ **v0.1.0** - Scaffold & Transport (6 issues)
- ✅ **v0.2.0** - Azure Client (5 issues)
- ✅ **v0.3.0** - Caching Layer (4 issues)
- ✅ **v0.4.0** - Field Mapping & Estimation (5 issues)

**Total Core Roadmap**: 20 issues across 4 phases
