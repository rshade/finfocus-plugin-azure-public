# Feature Specification: Integration Tests with Live Azure Retail Prices API

**Feature Branch**: `022-integration-tests`
**Created**: 2026-03-12
**Status**: Draft
**Input**: User description: "Create integration tests with live Azure
Retail Prices API"

## Clarifications

### Session 2026-03-12

- Q: What tolerance band for price range assertions? → A: ±25% (tight),
  with periodic reference price updates when Azure adjusts pricing

## User Scenarios & Testing *(mandatory)*

### User Story 1 - VM Cost Estimation Validation (Priority: P1)

A developer runs integration tests to verify that the EstimateCost RPC returns
accurate, non-zero pricing for common VM SKUs against the live Azure Retail
Prices API. This validates the full pipeline: gRPC request, descriptor mapping,
HTTP query, parsing, and cost calculation.

**Why this priority**: The core value proposition of the plugin is accurate VM
cost estimation. If this pipeline is broken, the entire plugin is unusable.

**Independent Test**: Can be tested by running a single integration test that
calls EstimateCost with a known VM SKU (Standard_B1s, eastus) and verifying the
returned cost falls within an expected range.

**Acceptance Scenarios**:

1. **Given** the plugin is configured with a live Azure API client, **When**
   EstimateCost is called with Standard_B1s in eastus, **Then** the response
   contains a non-zero monthly cost within ±25% of the reference price
2. **Given** the plugin is configured with a live Azure API client, **When**
   EstimateCost is called with Standard_D2s_v3 in eastus, **Then** the response
   contains a non-zero monthly cost within ±25% of the reference price
3. **Given** EstimateCost has been called once for a specific VM, **When** the
   same call is made again, **Then** the response is served from cache (faster
   response time) with identical pricing

---

### User Story 2 - Managed Disk Cost Estimation Validation (Priority: P2)

A developer runs integration tests to verify that Managed Disk cost estimation
works correctly against the live API, covering multiple disk types and the
size-to-tier mapping logic.

**Why this priority**: Disk estimation is the second supported resource type and
exercises a different pricing model (monthly vs hourly). Validating it ensures
multi-resource-type support works end-to-end.

**Independent Test**: Can be tested by calling EstimateCost with a Managed Disk
resource type and verifying the returned monthly cost is positive.

**Acceptance Scenarios**:

1. **Given** the plugin is configured with a live Azure API client, **When**
   EstimateCost is called with a 128GB Standard_LRS disk in eastus, **Then** the
   response contains a positive monthly cost
2. **Given** the plugin is configured with a live Azure API client, **When**
   EstimateCost is called with a 256GB Premium_SSD_LRS disk in eastus, **Then**
   the response contains a positive monthly cost greater than the Standard_LRS
   cost

---

### User Story 3 - Error Handling Validation (Priority: P2)

A developer runs integration tests to verify that invalid or unsupported inputs
produce correct, well-structured error responses rather than silent failures or
panics.

**Why this priority**: Proper error handling is critical for downstream consumers
who rely on gRPC status codes for programmatic error classification.

**Independent Test**: Can be tested by calling EstimateCost with an invalid SKU
and verifying the appropriate gRPC error code is returned.

**Acceptance Scenarios**:

1. **Given** the plugin is configured with a live Azure API client, **When**
   EstimateCost is called with a non-existent SKU, **Then** a NotFound gRPC
   error is returned
2. **Given** the plugin is configured with a live Azure API client, **When**
   EstimateCost is called with missing required attributes, **Then** an
   InvalidArgument gRPC error is returned

---

### User Story 4 - CI Pipeline Integration (Priority: P3)

A CI system runs integration tests on pushes to main (or via manual trigger) to
catch regressions before releases. Tests can be selectively skipped in
environments without API access.

**Why this priority**: CI integration provides ongoing regression protection but
depends on the tests themselves (P1/P2) existing first.

**Independent Test**: Can be validated by triggering the CI workflow and
observing that integration tests run (or are skipped based on configuration).

**Acceptance Scenarios**:

1. **Given** integration tests exist with the `integration` build tag, **When**
   `go test -tags=integration ./...` is run, **Then** all integration tests
   execute
2. **Given** the `SKIP_INTEGRATION` environment variable is set to "true",
   **When** integration tests are invoked, **Then** all integration tests are
   skipped gracefully
3. **Given** the CI pipeline is configured, **When** code is pushed to main,
   **Then** integration tests run as part of the pipeline (or can be triggered
   manually)

---

### Edge Cases

- What happens when the Azure API is temporarily unavailable? Tests should fail
  with a clear error message, not hang indefinitely (enforced by context
  timeout).
- What happens when Azure changes pricing? Tests use range-based assertions (not
  exact values) to tolerate normal price fluctuations.
- What happens when rate limits are hit? Tests include delays between queries (12
  seconds between calls) to stay under the 5 queries/minute budget.
- What happens when an invalid disk size is provided (e.g., 0 or negative)? The
  test should verify appropriate error handling.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Integration tests MUST use the `integration` build tag so they are
  excluded from normal `go test` runs
- **FR-002**: Tests MUST query the live Azure Retail Prices API (no mocks or
  stubs)
- **FR-003**: Tests MUST verify that EstimateCost returns a non-zero cost for
  known VM SKUs (Standard_B1s, Standard_D2s_v3)
- **FR-004**: Tests MUST verify cost values fall within a ±25% tolerance band
  around a reference price rather than asserting exact values. Reference prices
  should be updated periodically when Azure adjusts pricing.
- **FR-005**: Tests MUST cover both Virtual Machine and Managed Disk resource
  types
- **FR-006**: Tests MUST verify cache behavior by making duplicate calls and
  asserting the second call is served from cache
- **FR-007**: Tests MUST enforce rate limiting with a minimum 12-second delay
  between API queries (max 5 queries/minute)
- **FR-008**: Tests MUST be skippable via the `SKIP_INTEGRATION=true`
  environment variable
- **FR-009**: Tests MUST verify that invalid inputs (non-existent SKU, missing
  attributes) produce appropriate gRPC error codes
- **FR-010**: Tests MUST use context timeouts to prevent hanging on API
  unavailability

### Key Entities

- **Integration Test Suite**: Collection of end-to-end tests exercising the full
  EstimateCost pipeline against live Azure API
- **Test Fixture**: Predefined VM SKUs, disk configurations, and regions with
  known expected price ranges
- **Rate Limiter**: Mechanism to throttle test execution to respect Azure API
  rate limits

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All integration tests pass when run against the live Azure API
  with `go test -tags=integration`
- **SC-002**: VM cost estimates for Standard_B1s in eastus fall within ±25% of
  the reference hourly price (updated periodically to track Azure pricing)
- **SC-003**: Cache hit on duplicate calls is verified via cache stats counter
  increment (deterministic, not timing-based)
- **SC-004**: Tests complete within 5 minutes total (accounting for
  rate-limiting delays)
- **SC-005**: Tests are skipped cleanly (no errors, no API calls) when
  `SKIP_INTEGRATION=true`
- **SC-006**: Invalid input tests return expected gRPC error codes (NotFound,
  InvalidArgument)

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80% for business
  logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (external API interactions)
- [x] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (e.g., cache hits <10ms, API calls <2s
  p95)
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
