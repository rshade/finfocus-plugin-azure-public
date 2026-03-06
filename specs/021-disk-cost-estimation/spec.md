# Feature Specification: Managed Disk Cost Estimation

**Feature Branch**: `021-disk-cost-estimation`
**Created**: 2026-03-05
**Status**: Draft
**Input**: GitHub Issue #18 - Implement Managed Disk cost estimation

## Clarifications

### Session 2026-03-05

- Q: How should the system map a requested size_gb to an Azure pricing tier when the size doesn't match an exact tier boundary? → A: Ceiling match — map to the smallest tier >= requested size (e.g., 200GB maps to P20/512GB tier), consistent with Azure's own provisioning behavior.
- Q: Should the system support both tiered and per-GB pricing models for disks? → A: Tiered only — all three supported disk types (Standard_LRS, StandardSSD_LRS, Premium_SSD_LRS) use fixed monthly price per tier. No per-GB pricing path needed.
- Q: Should Ultra Disk, Premium SSD v2, and ZRS redundancy variants be supported? → A: Include ZRS variants (Standard_ZRS, StandardSSD_ZRS, Premium_ZRS) since they use the same tiered pricing model. Ultra Disk and Premium SSD v2 are explicitly out of scope.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Estimate Monthly Cost for a Managed Disk (Priority: P1)

FinFocus Core sends an EstimateCost request with a Managed Disk resource type, specifying disk type (e.g., Standard_LRS), provisioned size in GB, and region. The system queries Azure pricing for the matching disk SKU and returns an accurate monthly cost estimate.

**Why this priority**: This is the core capability — without it, disk cost estimation does not exist. It provides the foundational value that all other stories build on.

**Independent Test**: Can be fully tested by sending an EstimateCost request with disk attributes and verifying a non-zero monthly cost is returned with correct currency.

**Acceptance Scenarios**:

1. **Given** a valid EstimateCost request with resource_type `azure:storage/managedDisk:ManagedDisk`, location `eastus`, disk_type `Standard_LRS`, and size_gb `128`, **When** the system processes the request, **Then** a monthly cost greater than zero is returned with currency "USD"
2. **Given** a valid EstimateCost request for a Premium_SSD_LRS disk, **When** the system processes the request, **Then** the returned monthly cost is higher than the equivalent Standard_LRS disk
3. **Given** a valid disk cost request, **When** the Azure pricing API returns results, **Then** the response includes `FOCUS_PRICING_CATEGORY_STANDARD` pricing category

---

### User Story 2 - Validate Disk Descriptor Fields (Priority: P1)

When required fields are missing or invalid, the system returns clear, actionable error messages indicating which fields are missing, consistent with the existing VM validation pattern.

**Why this priority**: Equal to P1 because error handling is essential for a reliable API — callers must know why requests fail.

**Independent Test**: Can be tested by sending requests with missing fields and verifying appropriate error codes and messages.

**Acceptance Scenarios**:

1. **Given** a disk EstimateCost request without `location` or `region`, **When** the system validates the request, **Then** it returns an InvalidArgument error naming the missing field
2. **Given** a disk EstimateCost request without `disk_type`, **When** the system validates the request, **Then** it returns an InvalidArgument error naming the missing field
3. **Given** a disk EstimateCost request without `size_gb`, **When** the system validates the request, **Then** it returns an InvalidArgument error naming the missing field
4. **Given** a disk EstimateCost request with all required fields empty, **When** the system validates the request, **Then** all missing fields are reported in a single error message

---

### User Story 3 - Distinguish Disk Types (Priority: P2)

Users can estimate costs for different disk types (Standard HDD, Standard SSD, Premium SSD) and the system correctly maps each to its corresponding Azure pricing SKU, returning distinct pricing for each tier.

**Why this priority**: Disk type distinction is important for accurate estimates but relies on the core estimation path (P1) being functional first.

**Independent Test**: Can be tested by sending requests for each supported disk type and verifying different cost results.

**Acceptance Scenarios**:

1. **Given** a request for `Standard_LRS` (HDD), **When** the system estimates cost, **Then** pricing reflects the HDD tier
2. **Given** a request for `StandardSSD_LRS` (Standard SSD), **When** the system estimates cost, **Then** pricing reflects the Standard SSD tier
3. **Given** a request for `Premium_SSD_LRS` (Premium SSD), **When** the system estimates cost, **Then** pricing reflects the Premium SSD tier and is higher than Standard alternatives
4. **Given** a request for a ZRS variant (e.g., `Premium_ZRS`), **When** the system estimates cost, **Then** pricing reflects the ZRS tier and is higher than the equivalent LRS variant
5. **Given** a request for an unsupported disk type (e.g., Ultra Disk), **When** the system processes the request, **Then** an InvalidArgument error is returned indicating the unsupported disk type

---

### User Story 4 - Scale Cost by Provisioned Size (Priority: P2)

Disk costs scale based on provisioned size using ceiling-match tier mapping: the system selects the smallest Azure pricing tier whose capacity is >= the requested size_gb (e.g., 200GB maps to P20/512GB tier). This matches Azure's own provisioning behavior where you are billed for the tier, not the exact bytes used.

**Why this priority**: Size-based pricing is critical for accuracy but builds on the core estimation and disk type mapping.

**Independent Test**: Can be tested by sending requests with different size_gb values and verifying costs increase with size.

**Acceptance Scenarios**:

1. **Given** a request for a 256GB disk, **When** the system estimates cost, **Then** the cost is higher than for a 128GB disk of the same type
2. **Given** a request for a 32GB Standard_LRS disk, **When** the system estimates cost, **Then** a valid monthly cost is returned for the smallest common tier
3. **Given** a request for a very large disk (e.g., 4096GB), **When** the system estimates cost, **Then** the cost reflects the appropriate high-capacity tier

---

### Edge Cases

- What happens when size_gb is zero or negative? System returns InvalidArgument error
- What happens when size_gb is not a whole number (e.g., 128.5)? System rounds up to next integer before ceiling-match tier mapping
- What happens when the Azure pricing API returns no results for a valid disk type? System returns NotFound error with query context
- What happens when a disk type exists but is not available in the requested region? System returns NotFound error
- What happens when the request uses the legacy VM resource type but provides disk attributes? System treats it as a VM request (existing behavior unchanged)
- What happens when cache already contains pricing for the disk SKU? Cached result is returned without API call

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept EstimateCost requests with resource type containing `storage/managedDisk` (case-insensitive matching)
- **FR-002**: System MUST extract `disk_type` (or `diskType`, `sku`) from request attributes to determine the Azure disk SKU
- **FR-003**: System MUST extract `size_gb` (or `sizeGb`, `diskSizeGb`) from request attributes as a numeric value
- **FR-004**: System MUST extract `location` (or `region`) from request attributes for the Azure region
- **FR-005**: System MUST map disk types to Azure Retail Prices API query parameters using service name "Managed Disks"
- **FR-006**: System MUST return monthly cost using tiered pricing (each disk tier has a fixed monthly price; no per-GB calculation)
- **FR-007**: System MUST return InvalidArgument when any required field (region, disk_type, size_gb) is missing
- **FR-008**: System MUST report all missing fields in a single error message (consistent with VM behavior)
- **FR-009**: System MUST use the existing cache layer for disk pricing queries
- **FR-010**: System MUST continue to support VM cost estimation without regression
- **FR-011**: System MUST support these disk types: Standard_LRS, StandardSSD_LRS, Premium_SSD_LRS, Standard_ZRS, StandardSSD_ZRS, Premium_ZRS
- **FR-012**: System MUST treat Ultra Disk and Premium SSD v2 as explicitly unsupported, returning InvalidArgument with a clear message

### Key Entities

- **Disk Cost Request**: A cost estimation request containing region, disk type, and provisioned size in GB
- **Disk Pricing Tier**: An Azure pricing entry for a specific disk SKU (e.g., S10, P10, E10) that corresponds to a fixed capacity and fixed monthly price (tiered pricing model)
- **Disk Type Mapping**: The relationship between user-facing disk type names (Standard_LRS, Premium_SSD_LRS) and Azure pricing product/SKU names

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Disk cost estimation requests return accurate monthly costs within 5% of Azure Pricing Calculator values for all supported disk types
- **SC-002**: All supported disk types (Standard_LRS, StandardSSD_LRS, Premium_SSD_LRS, and their ZRS variants) return distinct pricing that reflects their relative cost hierarchy (HDD < Standard SSD < Premium SSD; ZRS variants cost more than LRS equivalents)
- **SC-003**: 100% of requests with missing required fields return clear error messages naming all missing fields
- **SC-004**: Existing VM cost estimation continues to pass all current tests without modification
- **SC-005**: Disk pricing queries benefit from the same caching behavior as VM queries (cache hits served without API calls)

## Assumptions

- Azure Managed Disk pricing is available through the public Retail Prices API at `https://prices.azure.com/api/retail/prices` with service name "Managed Disks"
- Disk types map to well-known Azure product naming conventions (e.g., Standard_LRS maps to "Standard HDD Managed Disks" product family)
- The existing `PriceQuery` structure is sufficient for disk pricing queries (using ArmSkuName for disk tier, ServiceName for "Managed Disks")
- Disk pricing from the Azure API is tiered and monthly (not hourly or per-GB), so no hours-per-month multiplication or size-based scaling is needed — the tier price is the monthly cost
- The finfocus-spec `EstimateCostResponse` supports returning monthly cost for non-hourly resources

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (>=80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (external API interactions)
- [x] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (e.g., cache hits <10ms, API calls <2s p95)
- [x] Observability requirements specified (logging, metrics)

### Documentation

- [x] README.md updates identified (if user-facing changes)
- [x] API documentation needs outlined (godoc comments, contracts)
- [x] Docstring coverage >=80% maintained (all exported symbols documented)
- [x] Examples/quickstart guide planned (if new capability)

### Performance & Reliability

- [x] Performance targets specified (response times, throughput)
- [x] Reliability requirements defined (retry logic, error handling)
- [x] Resource constraints considered (memory, connections, cache TTL)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data
