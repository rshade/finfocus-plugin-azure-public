# Feature Specification: Implement GetProjectedCost RPC

**Feature Branch**: `023-projected-cost-rpc`
**Created**: 2026-04-04
**Status**: Draft
**Input**: GitHub Issue #59 — Promote GetProjectedCost partial implementation to production-ready RPC

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Projected VM Cost Lookup (Priority: P1)

A FinFocus platform consumer sends a `GetProjectedCost` request with a valid
Azure VM resource descriptor (provider, region, SKU, resource type). The system
queries Azure Retail Prices (via cached client), computes projected monthly cost,
and returns a complete response including unit price, currency, monthly cost,
a human-readable billing explanation, pricing category, and a cache expiry hint.

**Why this priority**: This is the primary happy path — a correctly-formed VM
request returning a fully populated response. Every other behavior depends on
this working first.

**Independent Test**: Can be fully tested by sending a valid
`GetProjectedCostRequest` with `provider=azure`, `region=eastus`,
`sku=Standard_B1s`, `resource_type=compute/VirtualMachine` and asserting all
response fields are populated with correct values.

**Acceptance Scenarios**:

1. **Given** a Calculator with a cached client that has Azure pricing data,
   **When** a request with provider "azure", region "eastus", SKU
   "Standard_B1s", and resource type "compute/VirtualMachine" is received,
   **Then** the response includes `unit_price` (hourly rate), `currency`
   ("USD"), `cost_per_month` (unit_price × 730), `billing_detail` with a
   human-readable explanation, and `pricing_category` set to STANDARD.

2. **Given** a Calculator with a cached client,
   **When** a valid projected cost request is made,
   **Then** the response includes an `expires_at` timestamp propagated from
   the cache layer's `ExpiresAt` field.

---

### User Story 2 - Validation Error Feedback (Priority: P1)

A consumer sends a `GetProjectedCost` request with missing or incorrect fields.
The system returns a specific gRPC error code with a message identifying exactly
which fields are missing or which values are invalid, rather than a generic
"Unimplemented" response.

**Why this priority**: Clear error feedback is essential for consumer
integration. Without it, callers cannot distinguish between "this feature
doesn't exist" and "you sent bad input."

**Independent Test**: Can be tested by sending requests with various
combinations of missing/invalid fields and asserting the correct gRPC code
and field names appear in the error message.

**Acceptance Scenarios**:

1. **Given** a Calculator with a cached client,
   **When** a request is received with an empty region field,
   **Then** the system returns `InvalidArgument` with "region" in the error
   message.

2. **Given** a Calculator with a cached client,
   **When** a request is received with an empty SKU field,
   **Then** the system returns `InvalidArgument` with "sku" in the error
   message.

3. **Given** a Calculator with a cached client,
   **When** a request is received with both region and SKU missing,
   **Then** the system returns `InvalidArgument` listing both missing fields
   in a single error message.

4. **Given** a Calculator with a cached client,
   **When** a request is received with provider "gcp" (not "azure"),
   **Then** the system returns `Unimplemented` indicating the provider is
   unsupported (maps through `ErrUnsupportedResourceType`).

5. **Given** a Calculator with a cached client,
   **When** a request is received with an unsupported resource type (e.g.,
   "network/LoadBalancer"),
   **Then** the system returns `Unimplemented` indicating the resource type
   is not supported.

6. **Given** a Calculator with a cached client,
   **When** a request is received with a nil resource descriptor,
   **Then** the system returns `InvalidArgument`.

---

### User Story 3 - Structured Observability (Priority: P2)

An operator monitoring the plugin can observe structured log entries for every
`GetProjectedCost` request — including request parameters on entry, pricing
results on success, and categorized failure information on errors — enabling
effective debugging and alerting.

**Why this priority**: Observability is critical for production operation but
is not user-facing functionality. It follows the existing `EstimateCost`
pattern already established in the codebase.

**Independent Test**: Can be tested by capturing log output during request
processing and asserting that structured fields (region, sku, resource_type,
provider, result_status, cost_per_month, currency) appear at the correct
log levels.

**Acceptance Scenarios**:

1. **Given** a Calculator processing a valid projected cost request,
   **When** the request is received,
   **Then** an Info-level log entry includes `region`, `sku`,
   `resource_type`, and `provider` fields.

2. **Given** a Calculator processing a valid projected cost request,
   **When** the request succeeds,
   **Then** an Info-level log entry includes `cost_per_month`, `currency`,
   `unit_price`, and `result_status=success`.

3. **Given** a Calculator processing a request with missing fields,
   **When** validation fails,
   **Then** a Warn-level log entry includes `result_status=error` and the
   validation error.

4. **Given** a Calculator processing a valid request,
   **When** the Azure API or cache lookup fails,
   **Then** an Error-level log entry includes `result_status=error` and the
   API error details.

---

### User Story 4 - Graceful Degradation Without Cache (Priority: P3)

When the cached client is not configured (nil), the system returns a clear
`Unimplemented` status indicating the functionality is unavailable, rather
than panicking or returning an ambiguous error.

**Why this priority**: This is a defensive guard for deployment scenarios
where the cache layer is not initialized. It preserves the existing behavior
while making it explicit.

**Independent Test**: Can be tested by creating a Calculator without a cached
client and sending a valid projected cost request.

**Acceptance Scenarios**:

1. **Given** a Calculator without a cached client (nil),
   **When** any `GetProjectedCost` request is received,
   **Then** the system returns `Unimplemented` with a descriptive message.

### Edge Cases

- What happens when Azure API returns an empty price list for a valid
  region/SKU combination? The system propagates `NotFound` via
  `MapToGRPCStatus`.
- What happens when the resource descriptor has valid region/SKU in tags
  but not in primary fields? The `MapDescriptorToQuery` function falls back
  to tags, so the request succeeds.
- What happens when provider is "Azure" (capitalized)? Provider matching is
  case-insensitive, so it succeeds.
- What happens when resource type includes the full URN format (e.g.,
  "azure:compute/virtualMachine:VirtualMachine")? The mapper does NOT
  parse URN format — it performs a direct lowercase lookup. Consumers
  must use the short form (e.g., "compute/VirtualMachine").

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST validate the resource descriptor using
  `MapDescriptorToQuery()` and return specific gRPC error codes based on
  sentinel errors (`ErrMissingRequiredFields` → `InvalidArgument`,
  `ErrUnsupportedResourceType` → `Unimplemented`).
- **FR-002**: System MUST return `InvalidArgument` when region and/or SKU
  are missing, with the specific field names listed in the error message.
- **FR-003**: System MUST return `Unimplemented` when the provider is
  not "azure" (case-insensitive comparison), since the plugin does not
  implement non-Azure providers.
- **FR-004**: System MUST return `Unimplemented` for resource types not
  in the supported set (compute/VirtualMachine, storage/ManagedDisk,
  storage/BlobStorage).
- **FR-005**: System MUST return `Unimplemented` when the cached client is
  nil (no pricing data source available).
- **FR-006**: System MUST set `pricing_category` to
  `FOCUS_PRICING_CATEGORY_STANDARD` on all successful responses.
- **FR-007**: System MUST set `billing_detail` with a human-readable
  explanation including the data source, SKU, region, unit rate, and
  calculation method (e.g., hourly rate × 730 hours/month).
- **FR-008**: System MUST propagate `expires_at` from the cache layer's
  `CachedResult.ExpiresAt` field on successful responses.
- **FR-009**: System MUST map all `azureclient` errors through
  `MapToGRPCStatus()` to produce appropriate gRPC status codes.
- **FR-010**: System MUST emit structured log entries at request entry
  (Info level) with `region`, `sku`, `resource_type`, and `provider` fields.
- **FR-011**: System MUST emit structured log entries on success (Info level)
  with `cost_per_month`, `currency`, `unit_price`, and `result_status=success`.
- **FR-012**: System MUST emit structured log entries on validation failure
  (Warn level) with `result_status=error`.
- **FR-013**: System MUST emit structured log entries on API/cache failure
  (Error level) with `result_status=error`.
- **FR-014**: System MUST return `InvalidArgument` when the resource
  descriptor is nil.

### Key Entities

- **ResourceDescriptor**: The input entity containing provider, region, SKU,
  resource type, and optional tags. Used to identify the Azure resource whose
  projected cost is being requested.
- **GetProjectedCostResponse**: The output entity containing unit_price,
  currency, cost_per_month, billing_detail, pricing_category, and expires_at.
  Provides the consumer with projected monthly cost and metadata.
- **PriceQuery**: The internal entity that `MapDescriptorToQuery()` produces
  from a ResourceDescriptor. Used to query the Azure Retail Prices API via
  the cached client.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All valid projected cost requests return responses with all
  five core fields populated (unit_price, currency, cost_per_month,
  billing_detail, pricing_category) — 100% of the time.
- **SC-002**: Validation errors identify the exact missing or invalid fields,
  enabling consumers to correct requests without trial-and-error — every
  error message names the specific fields.
- **SC-003**: All existing `GetProjectedCost` tests continue to pass without
  modification (backward compatibility preserved).
- **SC-004**: Every `GetProjectedCost` request produces at least one
  structured log entry with all required fields, enabling operators to
  trace any request through the system.
- **SC-005**: Error responses use the correct gRPC status codes
  (`InvalidArgument`, `Unimplemented`, `NotFound`, etc.) matching the
  established `EstimateCost` error handling pattern.

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (existing integration tests in `examples/`)
- [x] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (cache hits <10ms, API calls <2s p95)
- [x] Observability requirements specified (logging, metrics)

### Documentation

- [x] README.md updates identified (if user-facing changes)
- [x] API documentation needs outlined (godoc comments, contracts)
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

- The existing `MapDescriptorToQuery()` function in `mapper.go` provides
  sufficient validation for all resource types, including correct sentinel
  error mapping.
- The `billing_detail` string format is informational only and not parsed
  by consumers — its exact format may evolve.
- VM projected cost uses the same hourly-to-monthly conversion as
  `EstimateCost` (unit_price × 730 hours/month).
- The `projectedQueryFromRequest()` function will be refactored to use
  `MapDescriptorToQuery()` internally, replacing the current boolean-return
  pattern.
- Managed Disk and Blob Storage resource types are validated by the mapper
  but their full projected cost paths may remain stubs until separate
  feature work is completed.

## Dependencies

- Depends on: Issue #17 (VM cost estimation pattern — already completed)
- Blocked by: none (all infrastructure exists)
- SDK dependency: `finfocus-spec v0.5.7` (pluginsdk response helpers)
