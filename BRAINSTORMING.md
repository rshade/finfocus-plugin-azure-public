# Brainstorming Session - Future Features

**Date**: 2026-01-21
**Session**: Post-v0.4.0 Planning

This document captures brainstorming for features beyond the core v0.1-0.4 roadmap.

---

## Priority 1: Additional Azure Services (v0.5.0)

### 1. App Service & Functions

**Description**: Cost estimation for Azure App Service (Web Apps) and Azure Functions (serverless compute).

**Rationale**:
- Common PaaS services with significant cost impact
- Predictable pricing based on App Service Plan tiers
- Functions have both consumption and premium plans

**Boundary Check**: ✅ OK
- Uses public Azure Retail Prices API
- No authentication required
- Stateless cost calculation

**Effort**: Medium
- App Service: Service plans have clear SKU mapping (B1, S1, P1v2, etc.)
- Functions: Consumption pricing based on executions and GB-s
- Need to map descriptor fields: plan_tier, sku, region

**Impact**: High - Many applications use App Service/Functions

**Dependencies**:
- v0.4.0 complete (VM/disk estimation working)
- Field mapping infrastructure from #16

**Next Step**: Create GitHub issue for v0.5.0 milestone

---

### 2. Azure SQL Database & Cosmos DB

**Description**: Cost estimation for managed database services.

**Rationale**:
- Critical infrastructure components with complex pricing
- Multiple pricing tiers (Basic, Standard, Premium, Hyperscale)
- Cosmos DB has request unit (RU/s) based pricing

**Boundary Check**: ✅ OK (with caveats)
- Azure SQL: Standard tier pricing available via API
- Cosmos DB: Requires mapping RU/s to pricing SKUs
- Elastic pools add complexity (shared resources)

**Effort**: Large
- Azure SQL: DTU-based vs vCore-based pricing models
- Cosmos DB: Multi-region, consistency levels affect cost
- Need descriptor fields: database_type, tier, dtu/vcore, storage_gb

**Impact**: High - Database costs are often significant

**Dependencies**:
- v0.4.0 complete
- Research spike: Map Cosmos DB RU/s to pricing SKUs

**Next Step**: Research spike issue, then implementation issue

---

### 3. Azure Kubernetes Service (AKS)

**Description**: Cost estimation for AKS clusters.

**Rationale**:
- Popular container orchestration platform
- Costs include: VM node pools + cluster management fee
- Complex because clusters have multiple node pools with different VM types

**Boundary Check**: ✅ OK (with delegation)
- Node pool VMs: Already supported via VM estimation (#17)
- Cluster management: Free tier exists, paid tier has hourly cost
- Load balancers, public IPs: Separate line items

**Effort**: Medium
- Can reuse VM estimation for node pools
- Add cluster management fee lookup
- Need descriptor fields: node_pools (array), cluster_tier

**Impact**: High - Container adoption is growing

**Dependencies**:
- v0.4.0 complete (VM estimation)
- Multi-resource estimation pattern (cluster = VMs + management)

**Next Step**: Create implementation issue for v0.5.0

---

### 4. Storage Accounts (Blob, File, Queue, Table)

**Description**: Cost estimation for Azure Storage services.

**Rationale**:
- Fundamental service used by nearly all Azure deployments
- Pricing based on: storage tier (hot/cool/archive), capacity, transactions

**Boundary Check**: ⚠️ Partial OK
- Storage capacity: Can estimate based on provisioned/estimated GB
- Transactions: Usage-based, cannot estimate without actual usage data
- Recommendation: Estimate storage capacity only, note transaction costs separately

**Effort**: Medium
- Storage tiers: Hot, Cool, Archive have different per-GB rates
- Redundancy: LRS, GRS, RA-GRS affect pricing
- Need descriptor fields: storage_type, tier, capacity_gb, redundancy

**Impact**: Medium-High - Common service but costs often secondary to compute

**Dependencies**:
- v0.4.0 complete
- Pattern for capacity-based vs usage-based pricing

**Next Step**: Create implementation issue with scope limited to capacity

---

## Priority 2: Advanced Pricing Models (Research First)

### 5. Savings Plans & Reserved Instances

**Description**: Support for discounted pricing models (1-year or 3-year commitments).

**Rationale**:
- Significant cost savings (up to 72% for RIs, 65% for Savings Plans)
- Critical for accurate cost optimization recommendations
- Complex mapping: commitment term, payment option, scope

**Boundary Check**: ⚠️ Requires Research
- Azure API may provide reservation pricing separately
- Need to map ResourceDescriptor to indicate reservation intent
- Question: How does FinFocus Core express "I want RI pricing"?

**Effort**: Unknown - Research needed
- Feasibility check: Does Azure Retail Prices API include RI/Savings Plan rates?
- Mapping: How to indicate commitment term in ResourceDescriptor?
- Calculation: Apply discount percentage vs lookup separate SKU?

**Impact**: Game-changer for cost optimization

**Dependencies**:
- v0.4.0 complete
- Research spike: API availability and mapping approach

**Next Step**: **Create research spike issue** to explore:
1. Azure API support for RI/Savings Plan pricing
2. ResourceDescriptor extension needed (new fields or tags)
3. Prototype implementation approach

---

### 6. Spot Instances (Already in Future Vision)

**Description**: Support for Azure Spot VMs (preemptible instances with deep discounts).

**Rationale**:
- Up to 90% cost savings for fault-tolerant workloads
- Simpler than Reservations (no commitment, just spot pricing lookup)
- Already identified in ROADMAP Future Vision

**Boundary Check**: ✅ OK
- Spot pricing available via Azure Retail Prices API
- Filter: type eq 'Spot' instead of 'Consumption'
- No authentication required

**Effort**: Small
- Reuse VM estimation logic (#17)
- Add pricing type parameter (Consumption vs Spot)
- Need descriptor field: pricing_type or spot: true

**Impact**: Valuable for cost-sensitive workloads

**Dependencies**:
- v0.4.0 complete (#17 VM estimation)

**Next Step**: Move from Future Vision to v0.5.0 concrete issue

---

## Priority 3: Carbon Estimation (Research First)

### 7. Carbon Footprint Tracking

**Description**: Estimate carbon emissions for Azure resources aligned with AWS plugin's carbon feature.

**Rationale**:
- Sustainability reporting increasingly important
- AWS plugin provides carbon estimates via Cloud Carbon Footprint (CCF)
- Consistency across FinFocus plugins

**Boundary Check**: ⚠️ Requires Research
- Azure does NOT publish carbon data via Retail Prices API
- Possible sources:
  1. Azure Carbon Optimization (requires authentication - ❌ violates boundary)
  2. Cloud Carbon Footprint open-source methodology (✅ OK - client-side calculation)
  3. Manual carbon intensity data by region (✅ OK - embed static data)

**Effort**: Unknown - Depends on data source
- CCF approach: Requires CPU/memory/storage metrics + region carbon intensity
- Static data: Embed kgCO2/kWh per Azure region, calculate from VM specs
- AWS plugin: Check implementation for reference

**Impact**: Nice-to-have - Growing importance for ESG reporting

**Dependencies**:
- v0.4.0 complete
- Research spike: Identify viable carbon data source

**Next Step**: **Create research spike issue** to explore:
1. Azure carbon data availability (public APIs, datasets)
2. CCF methodology applicability
3. Static carbon intensity data collection
4. Prototype calculation approach

---

## Priority 4: Testing & Validation (v0.5.0)

### 8. Regression Tests Against Known Prices

**Description**: Test suite with golden pricing data to prevent calculation regressions.

**Rationale**:
- Ensures pricing calculation logic remains accurate
- Catches bugs before production
- Provides confidence during refactoring

**Boundary Check**: ✅ OK
- Use snapshot testing: capture API responses, verify calculations
- Update snapshots when Azure pricing changes (documented process)

**Effort**: Small-Medium
- Create golden test data (JSON fixtures)
- Write snapshot comparison tests
- Document update procedure for price changes

**Impact**: High - Prevents regressions

**Dependencies**:
- v0.4.0 complete

**Next Step**: Create implementation issue for v0.5.0

---

### 9. Azure Pricing Calculator Compatibility Tests

**Description**: Validate plugin estimates against official Azure Pricing Calculator.

**Rationale**:
- Azure Pricing Calculator is source of truth
- Ensures plugin accuracy builds trust
- Identifies mapping errors

**Boundary Check**: ✅ OK
- Manual process: Calculate in Azure calculator, assert plugin within threshold
- Automated: Could scrape calculator (fragile) or use documented examples

**Effort**: Small
- Create test cases: Known VM/disk configurations
- Assert plugin estimate within ±5% of calculator
- Document test methodology

**Impact**: High - Validates accuracy

**Dependencies**:
- v0.4.0 complete

**Next Step**: Create implementation issue for v0.5.0 (manual tests initially)

---

### 10. Performance Benchmarking & Load Testing

**Description**: Validate cache effectiveness and concurrent request handling under load.

**Rationale**:
- Ensures plugin performs well in production
- Validates cache hit rate targets (>80%)
- Identifies bottlenecks before deployment

**Boundary Check**: ✅ OK
- Use Go benchmarking: benchmark tests for critical paths
- Load testing: Concurrent gRPC requests
- No external dependencies

**Effort**: Small
- Write benchmark tests: `func BenchmarkEstimateCost(b *testing.B)`
- Use `testing/quick` for property-based testing
- Create load test script: concurrent gRPC calls

**Impact**: Medium-High - Prevents performance issues

**Dependencies**:
- v0.3.0 complete (cache)
- v0.4.0 complete (estimation)

**Next Step**: Create implementation issue for v0.5.0

---

### 11. Chaos Testing for API Failures

**Description**: Test plugin resilience when Azure API is unreachable, rate-limiting, or returning errors.

**Rationale**:
- Validates retry logic (#7)
- Ensures graceful degradation
- Builds confidence in production reliability

**Boundary Check**: ✅ OK
- Use fault injection: mock HTTP failures
- Test scenarios: timeout, 429, 503, connection refused
- Verify: retries, logging, error messages

**Effort**: Small
- Write chaos tests using HTTP mock server
- Inject faults: latency, errors, partial responses
- Assert: appropriate retries and error handling

**Impact**: Medium - Prevents production surprises

**Dependencies**:
- v0.2.0 complete (#7 retry logic)

**Next Step**: Create implementation issue for v0.5.0

---

## Additional Ideas Explored

### 12. Cost Optimization Recommendations (Deferred)

**Description**: Implement GetRecommendations() RPC for right-sizing and reservation advice.

**Boundary Check**: ❌ Violates Constraints
- Requires actual usage data (CPU, memory utilization)
- Cannot determine under-utilization without Azure credentials
- Recommendation: Delegate to separate plugin with authentication

**Next Step**: Document as out-of-scope, suggest separate plugin

---

### 13. Budget & Alerting (Deferred)

**Description**: Implement GetBudgets() RPC to track projected vs actual costs.

**Boundary Check**: ❌ Violates Constraints
- Requires persistent storage (budgets must survive restarts)
- In-memory only violates "no persistent storage" rule
- Recommendation: FinFocus Core should handle budgets, not plugin

**Next Step**: Document as out-of-scope (Core responsibility)

---

### 14. Historical Cost Analysis (Deferred)

**Description**: Implement GetActualCost() RPC using Azure Cost Management API.

**Boundary Check**: ❌ Violates Constraints
- Azure Cost Management API requires authentication
- Violates "No Authenticated Azure APIs" rule
- Recommendation: Create separate authenticated plugin

**Next Step**: Document as out-of-scope, suggest finfocus-plugin-azure-authenticated

---

## Summary & Prioritization

### Ready for v0.5.0 (Concrete Issues)
1. ✅ **App Service & Functions** - High priority, clear path
2. ✅ **AKS** - Builds on VM estimation
3. ✅ **Storage Accounts (capacity only)** - Medium priority
4. ✅ **Spot Instances** - Low effort, high value
5. ✅ **Regression Testing** - Quality improvement
6. ✅ **Azure Calculator Compatibility Tests** - Quality improvement
7. ✅ **Performance Benchmarking** - Quality improvement
8. ✅ **Chaos Testing** - Quality improvement

### Requires Research Spike First
1. ⚠️ **Azure SQL & Cosmos DB** - Complex pricing, need mapping research
2. ⚠️ **Savings Plans & Reserved Instances** - API availability unknown
3. ⚠️ **Carbon Footprint** - Data source unclear

### Out of Scope (Boundary Violations)
1. ❌ **Cost Optimization Recommendations** - Requires usage data + auth
2. ❌ **Budget & Alerting** - Requires persistent storage
3. ❌ **Historical Cost Analysis** - Requires authenticated API

---

## Recommended v0.5.0 Scope

**Services** (4 issues):
- Issue #21: App Service & Functions cost estimation
- Issue #22: AKS cluster cost estimation
- Issue #23: Storage Accounts capacity-based estimation
- Issue #24: Spot VM pricing support

**Testing** (4 issues):
- Issue #25: Regression test suite with golden pricing data
- Issue #26: Azure Pricing Calculator compatibility tests
- Issue #27: Performance benchmarking and load testing
- Issue #28: Chaos testing for API failure scenarios

**Research Spikes** (3 issues):
- Issue #29: Research Azure SQL/Cosmos DB pricing mapping
- Issue #30: Research Savings Plans/RI API availability
- Issue #31: Research carbon data sources for Azure

**Total**: 11 issues for v0.5.0 (8 implementation + 3 research)

---

## Future Milestones

**v0.6.0** - Databases (pending research)
- Azure SQL Database estimation
- Cosmos DB estimation

**v0.7.0** - Advanced Pricing (pending research)
- Reserved Instances support
- Savings Plans support

**v0.8.0** - Carbon Tracking (pending research)
- Carbon footprint estimation

**v1.0.0** - Production Ready
- All core services supported
- Comprehensive test coverage
- Documentation complete
- Performance validated
