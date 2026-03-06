# Feature Specification: VM Cost Estimation (EstimateCost RPC)

**Feature Branch**: `020-vm-cost-estimation`
**Created**: 2026-03-04
**Status**: Draft
**Input**: GitHub Issue #17 — Implement EstimateCost RPC for Azure VM cost estimation

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Estimate Linux VM Cost (Priority: P1)

As the FinFocus Core system, I want to call EstimateCost with a Virtual Machine
descriptor (region, SKU) and receive accurate hourly and monthly cost estimates
so that I can present cost projections to end users.

**Why this priority**: This is the primary use case — without it the plugin
provides no value. All other stories depend on cost estimation working for the
happy path.

**Independent Test**: Can be fully tested by sending an EstimateCost request with
a known VM SKU (e.g., Standard_B1s in eastus) and verifying the response contains
non-zero hourly, monthly, and yearly costs with the correct currency.

**Acceptance Scenarios**:

1. **Given** a valid VM descriptor with region "eastus" and SKU "Standard_B1s",
   **When** EstimateCost is called, **Then** the response includes a non-zero
   `cost_monthly` equal to the Azure retail hourly price multiplied by 730.
2. **Given** a valid VM descriptor, **When** EstimateCost is called, **Then** the
   response includes the currency code (default USD).
3. **Given** a valid VM descriptor, **When** EstimateCost is called, **Then** the
   response includes `pricing_category` set to `STANDARD` (Consumption pricing).

---

### User Story 2 - Clear Errors for Invalid Requests (Priority: P2)

As a consumer of the EstimateCost RPC, I want clear and specific error responses
when I provide an invalid or incomplete descriptor so that I can correct my
request without guesswork.

**Why this priority**: Proper error handling is essential for a reliable
integration. Without it, callers cannot distinguish between bad input, missing
pricing data, and transient failures.

**Independent Test**: Can be fully tested by sending malformed requests and
verifying the correct error code and message are returned for each case.

**Acceptance Scenarios**:

1. **Given** a descriptor missing the region field, **When** EstimateCost is
   called, **Then** the response is an InvalidArgument error identifying the
   missing field.
2. **Given** a descriptor missing the SKU field, **When** EstimateCost is called,
   **Then** the response is an InvalidArgument error identifying the missing field.
3. **Given** a descriptor with an unsupported resource type (i.e., one that does
   not contain "compute/virtualMachine", case-insensitive), **When** EstimateCost
   is called, **Then** the response is an Unimplemented error. Empty resource
   type is allowed for backward compatibility.
4. **Given** a descriptor for a VM SKU that does not exist in Azure pricing data,
   **When** EstimateCost is called, **Then** the response is a NotFound error.

---

### User Story 3 - Cached Pricing for Repeated Queries (Priority: P3)

As an operator, I want repeated EstimateCost queries for the same VM
configuration to be served from cache so that response times are fast and
Azure API calls are minimized.

**Why this priority**: Caching improves performance and reduces external API
dependency, but is an optimization on top of the core functionality.

**Independent Test**: Can be tested by issuing the same EstimateCost request
twice and observing (via logs or metrics) that the second request is a cache hit
with no outbound API call.

**Acceptance Scenarios**:

1. **Given** a valid VM descriptor that has been queried before within the cache
   TTL, **When** EstimateCost is called again with the same parameters, **Then**
   the response is returned from cache (no external API call is made).
2. **Given** a cached pricing result, **When** EstimateCost returns the response,
   **Then** cache expiration metadata is available out-of-band via the
   `CachedResult.ExpiresAt` field (propagated to `GetActualCost` and
   `GetProjectedCost` responses); `EstimateCostResponse` itself does not carry
   a cache expiration field in the current proto revision.
3. **Given** that the cache TTL has expired for a previously queried VM, **When**
   EstimateCost is called, **Then** the system fetches fresh pricing data from the
   external source.

---

### Edge Cases

- What happens when Azure returns multiple price items for the same VM SKU?
  The system uses the first Consumption-type item's retail price.
- What happens when the Azure API is temporarily unavailable? The system returns
  an Unavailable error and retries are handled at the transport layer.
- What happens when the Azure API rate-limits the request? The system returns a
  ResourceExhausted error.
- What happens when both primary descriptor fields and tag fallbacks are empty?
  The system returns an InvalidArgument error listing all missing fields.
- What happens when the request context is cancelled during a pricing lookup?
  The system returns a Cancelled error promptly.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept an EstimateCost request containing a VM
  descriptor with region, SKU, and optional currency code.
- **FR-002**: System MUST calculate `cost_monthly` as the Azure retail hourly
  price for Linux Consumption-type VMs multiplied by 730 (365 × 24 / 12).
  Callers can derive hourly cost as `cost_monthly / 730` and yearly cost as
  `cost_monthly * 12`. *(Proto `EstimateCostResponse` only exposes
  `cost_monthly`; hourly and yearly fields are deferred to a future spec
  revision.)*
- **FR-003**: *(Merged into FR-002.)* The monthly cost MUST equal the Azure
  retail hourly price multiplied by 730.
- **FR-004**: *(Deferred.)* Hourly and yearly costs are derivable from
  `cost_monthly` by callers. Dedicated response fields require a proto update
  in finfocus-spec.
- **FR-005**: System MUST return the currency code in the response (defaulting
  to USD when not specified).
- **FR-006**: *(Deferred.)* Cost breakdown line items require a proto update in
  finfocus-spec. For v0.4.0, a single implicit line item ("VM compute") is
  represented by the `cost_monthly` value.
- **FR-007**: System MUST return an InvalidArgument error when required fields
  (region, SKU) are missing from the descriptor, identifying all missing fields.
- **FR-008**: System MUST return a NotFound error when no pricing data exists for
  the specified VM configuration.
- **FR-009**: System MUST return an Unimplemented error when the resource type is
  not a supported compute/VirtualMachine type.
- **FR-010**: System MUST use cached pricing data for repeated identical queries
  within the cache TTL period.
- **FR-011**: System MUST support attribute key alias resolution for region
  (`location`/`region`) and SKU (`vmSize`/`sku`/`armSkuName`) fields so that
  callers can use any common key name. *(Already implemented in
  `estimateQueryFromRequest`.)*
- **FR-012**: System MUST log cost estimation requests with structured fields
  including region, SKU, and result status (success/error). Cache hit/miss
  logging is handled at the `CachedClient` layer (debug level) and is not
  repeated in the `EstimateCost` method.

### Key Entities

- **Cost Estimate**: The response payload containing `cost_monthly`, `currency`,
  and `pricing_category`. Represents a point-in-time pricing snapshot for a
  specific VM configuration. *(Hourly/yearly costs and cost breakdown line items
  are deferred to a future proto revision.)*
- **VM Descriptor**: The input identifying a virtual machine by region, SKU,
  and optionally currency. Maps to an Azure pricing query.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A valid EstimateCost request returns a non-zero cost response
  within 2 seconds for cache-miss scenarios and within 10 milliseconds for
  cache-hit scenarios.
- **SC-002**: Cost estimates for known VM SKUs are accurate to within 5% of the
  published Azure retail pricing calculator values.
- **SC-003**: All invalid or incomplete requests return descriptive error
  responses that identify the specific problem (missing fields, unsupported type,
  or no data found).
- **SC-004**: Repeated identical queries result in cache hits at least 95% of the
  time during normal operation (within cache TTL).
- **SC-005**: Test coverage for cost estimation business logic meets or exceeds
  80%.

<!-- markdownlint-disable MD013 -->

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (external API interactions)
- [x] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (cache hits <10ms, API calls <2s p95)
- [x] Observability requirements specified (logging, metrics)

### Documentation

- [x] README.md updates identified (if user-facing changes)
- [x] API documentation needs to be outlined (godoc comments, contracts)
- [x] Docstring coverage ≥80% maintained (all exported symbols documented)
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

## Assumptions

- The Azure Retail Prices API is publicly accessible without authentication,
  consistent with the project's architectural constraint.
- For v0.4.0, only Linux VM Consumption pricing is in scope. Windows, spot,
  and reservation pricing are deferred to future releases.
- The "first matching price item" strategy is sufficient for v0.4.0. Future
  versions may implement more sophisticated selection (e.g., filtering by meter
  name or product family).
- Monthly hours constant of 730 (365 × 24 / 12) is the agreed standard across
  the FinFocus ecosystem.
- The existing cache infrastructure (LRU+TTL with ExpiresAt hint)
  is sufficient and does not need modification for this feature.
