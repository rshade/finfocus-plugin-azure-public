# Feature Specification: Comprehensive Error Handling for Azure API Failures

**Feature Branch**: `008-azure-error-handling`
**Created**: 2026-02-09
**Status**: Draft
**Input**: User description: "Implement comprehensive error handling for HTTP failures, API errors, and edge cases when querying the Azure Retail Prices API."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Contextual Error Messages for Pricing Queries (Priority: P1)

As FinFocus Core, when a pricing query fails, I receive an error message that includes the query context (region, SKU, service name) so I can identify exactly which resource lookup failed without needing to correlate logs manually.

**Why this priority**: Without contextual error messages, operators cannot quickly diagnose which pricing query failed in a multi-resource cost estimation. This is the most fundamental improvement needed for operational reliability.

**Independent Test**: Can be fully tested by issuing a pricing query with known-bad parameters and verifying the returned error includes the query context fields. Delivers immediate value for debugging failed cost estimations.

**Acceptance Scenarios**:

1. **Given** the Azure API returns an HTTP error, **When** the error propagates to the caller, **Then** the error message includes the region, SKU, and service name that were queried.
2. **Given** the Azure API returns a network error (DNS failure, connection refused), **When** the error propagates to the caller, **Then** the error message includes the target URL and the filter parameters used.
3. **Given** any error occurs during a pricing query, **When** the error is returned, **Then** the original root cause is preserved and can be extracted by the caller.

---

### User Story 2 - Error Classification for Retry Decisions (Priority: P2)

As cost estimation logic within FinFocus Core, I need to distinguish between retryable errors (temporary failures) and non-retryable errors (bad request, not found) so I can decide whether to retry at the application level or fail fast with an appropriate user-facing message.

**Why this priority**: Error classification enables intelligent retry behavior at the orchestration layer, preventing unnecessary retries on permanent failures and improving response times for transient issues.

**Independent Test**: Can be fully tested by simulating different HTTP failure modes and verifying each maps to the correct error category. Delivers value by enabling callers to make informed retry decisions.

**Acceptance Scenarios**:

1. **Given** the Azure API returns HTTP 404 (resource not found), **When** the error is classified, **Then** it is categorized as a non-retryable "not found" error that maps to a "NotFound" status in the application layer.
2. **Given** the Azure API returns HTTP 429 (rate limited) and retries are exhausted, **When** the error propagates, **Then** it is categorized as a retryable "resource exhausted" error.
3. **Given** the Azure API returns HTTP 500 (internal server error) and retries are exhausted, **When** the error propagates, **Then** it is categorized as a temporary "internal" error indicating the upstream service failed.
4. **Given** a network error occurs (timeout, connection reset), **When** the error propagates, **Then** it is categorized as a retryable "unavailable" error.

---

### User Story 3 - Structured Error Logging for Troubleshooting (Priority: P3)

As an operator monitoring the FinFocus plugin, I want all pricing query errors to be logged with structured fields (region, SKU, service, error type, HTTP status) at appropriate severity levels so I can set up alerts and dashboards for different failure categories.

**Why this priority**: Structured logging enables proactive monitoring and faster incident response. Without structured fields, operators must parse unstructured error messages manually, which is error-prone and slow.

**Independent Test**: Can be fully tested by triggering various error conditions and verifying the structured log output contains the expected fields at the correct severity level. Delivers value for production observability.

**Acceptance Scenarios**:

1. **Given** an HTTP 4xx error occurs (except rate limiting), **When** the error is logged, **Then** the log entry is at "warn" level and includes structured fields for region, SKU, service, HTTP status code, and the filter used.
2. **Given** an HTTP 5xx error occurs, **When** the error is logged, **Then** the log entry is at "error" level and includes the same structured fields plus an indication that retries were attempted.
3. **Given** a network-level error occurs (DNS, timeout, connection), **When** the error is logged, **Then** the log entry is at "debug" level and includes the target URL and error description.
4. **Given** a JSON parsing error occurs from a malformed response, **When** the error is logged, **Then** the log entry includes a truncated snippet of the response body (first 256 characters) for diagnostic purposes.

---

### User Story 4 - Empty Result Set Handling (Priority: P2)

As FinFocus Core, when a pricing query returns zero results (the resource/SKU combination has no pricing data), I receive a specific "pricing data not found" error rather than an empty success response, so I can handle missing pricing data explicitly in cost estimation logic.

**Why this priority**: Empty results are a common edge case (typos in SKU names, unsupported regions) that must be handled distinctly from actual errors. Returning an empty success would cause silent miscalculations in cost estimation.

**Independent Test**: Can be fully tested by querying for a non-existent SKU/region combination and verifying a specific "not found" error is returned. Delivers value by preventing silent failures in cost estimation.

**Acceptance Scenarios**:

1. **Given** the Azure API returns a successful response with zero items (Count: 0), **When** the response is processed, **Then** a "pricing data not found" error is returned that includes the query parameters used.
2. **Given** the Azure API returns a successful response with one or more items, **When** the response is processed, **Then** no error is returned and the items are provided to the caller.

---

### Edge Cases

- What happens when the Azure API returns a valid HTTP 200 response but the JSON body is malformed or truncated?
- What happens when the Azure API returns HTML instead of JSON (e.g., a load balancer error page)?
- What happens when the Azure API returns a 200 with an error message embedded in the response body (API-level error)? **Resolution**: If the JSON structure differs from PriceResponse, it fails JSON parsing (handled by FR-007). If it parses but yields zero items, it is caught by empty result detection (FR-003).
- What happens when multiple pages of results are being fetched and an error occurs mid-pagination?
- What happens when the response body exceeds expected size limits?
- What happens when the Azure API returns an unexpected HTTP status code not covered by standard handling (e.g., 418)?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST wrap all errors returned from pricing queries with the query context (region, SKU name, service name) that triggered the failure.
- **FR-002**: System MUST classify errors into 5 distinct categories: not found, rate limited, temporarily unavailable, request failure (covering both network errors and non-retryable HTTP failures), and invalid response.
- **FR-003**: System MUST return a specific "pricing data not found" error when the Azure API returns a successful response with zero items, including the query parameters in the error message.
- **FR-004**: System MUST preserve the original error cause through error wrapping so callers can inspect the full error chain.
- **FR-005**: System MUST log all pricing query errors with structured fields: region, SKU, service name, error category, HTTP status code (when applicable), and request URL.
- **FR-006**: System MUST log errors at differentiated severity levels: network errors at debug, client errors (4xx) at warn, server errors (5xx) at error level.
- **FR-007**: System MUST include a truncated response body snippet (up to 256 characters) in errors caused by JSON parsing failures, to aid in diagnosing malformed responses.
- **FR-008**: System MUST map error categories to appropriate application-layer status codes for propagation to the calling service (e.g., not found, resource exhausted, internal, unavailable).
- **FR-009**: System MUST NOT expose sensitive information (API keys, internal URLs beyond the public API endpoint) in error messages or logs.
- **FR-010**: System MUST handle mid-pagination failures by returning the error with context about which page failed, without returning partial results.

### Key Entities

- **Sentinel Errors**: Classified errors implemented as Go sentinel error values (`ErrNotFound`, `ErrRateLimited`, `ErrServiceUnavailable`, `ErrRequestFailed`, `ErrInvalidResponse`) with query context added via `fmt.Errorf("%w")` wrapping. Callers use `errors.Is()` for programmatic classification.
- **ErrorCategory**: The classification of an error (not found, rate limited, temporarily unavailable, request failure, invalid response) used by callers to determine appropriate handling strategy.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of error messages returned from pricing queries include the query context (region, SKU, service) enabling operators to identify the failing resource without additional log correlation.
- **SC-002**: All error conditions produce structured log entries with at least 4 contextual fields (region, SKU, service, error category), enabling dashboard and alert creation.
- **SC-003**: Callers can programmatically distinguish between at least 4 error categories (not found, rate limited, temporary failure, permanent failure) to make informed retry and fallback decisions.
- **SC-004**: Empty result sets are detected and reported as errors in 100% of cases, preventing silent miscalculations in cost estimation.
- **SC-005**: Test coverage for error handling paths reaches at least 80%, covering all identified error categories and edge cases.
- **SC-006**: No error message or log entry exposes sensitive internal information beyond the public API endpoint URL.

## Assumptions

- The Azure Retail Prices API is a public, unauthenticated API and its error response format follows standard HTTP conventions (status codes, JSON error bodies).
- Retry logic for transient errors (HTTP 429, 503) is already handled at the HTTP client layer; this feature focuses on error classification and context enrichment after retries are exhausted.
- The truncated response snippet (256 characters) for JSON parsing errors is sufficient for diagnostic purposes without creating excessively large log entries.
- Log severity levels (debug for network, warn for 4xx, error for 5xx) align with the project's operational monitoring strategy.
- Partial results from mid-pagination failures are not useful and should be discarded in favor of a clean error.

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (>=80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (external API interactions)
- [ ] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (e.g., cache hits <10ms, API calls <2s p95)
- [x] Observability requirements specified (logging, metrics)

### Documentation

- [ ] README.md updates identified (if user-facing changes)
- [x] API documentation needs outlined (godoc comments, contracts)
- [x] Docstring coverage >=80% maintained (all exported symbols documented)
- [ ] Examples/quickstart guide planned (if new capability)

### Performance & Reliability

- [x] Performance targets specified (response times, throughput)
- [x] Reliability requirements defined (retry logic, error handling)
- [x] Resource constraints considered (memory, connections, cache TTL)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data
