# Research: Comprehensive Error Handling for Azure API Failures

**Feature Branch**: `008-azure-error-handling`
**Date**: 2026-02-09

## R1: Existing Error Handling Patterns in Codebase

**Decision**: Continue using sentinel errors with `fmt.Errorf("%w")` wrapping.

**Rationale**: The codebase already uses this pattern consistently in
`internal/azureclient/errors.go` with 5 sentinel errors. Callers use
`errors.Is()` for classification. This is idiomatic Go and requires minimal
changes to extend.

**Alternatives considered**:

- Custom error struct type (`PricingError`): Would require callers to use
  `errors.As()` type assertions, breaking the existing pattern. Over-
  engineering for the current scope.
- Error codes enum: Go convention prefers sentinel errors over numeric codes.
  Less readable and non-idiomatic.

## R2: Azure Retail Prices API Error Behavior

**Decision**: Handle HTTP 404 as a distinct error case, treat empty results
(Count: 0) as "not found", and handle HTML responses as invalid JSON.

**Rationale**: Research of the Azure Retail Prices API behavior shows:

- HTTP 404: Rare, returned for malformed API paths (not for missing SKUs).
  The API returns empty results (Count: 0) for unknown SKU/region combos.
- Empty results: The most common "not found" scenario. API returns HTTP 200
  with `{"Items": [], "Count": 0}`.
- HTML responses: Can occur when Azure infrastructure returns error pages
  (load balancer, CDN). These fail JSON parsing and should be treated as
  `ErrInvalidResponse`.
- HTTP 400: Returned for malformed OData filters. Should be `ErrRequestFailed`
  with descriptive message.

**Alternatives considered**:

- Treating empty results as success (return empty slice): Would cause silent
  miscalculations in cost estimation. Rejected per spec FR-003.
- Separate error for HTML responses: Unnecessary granularity. JSON parse
  failure with response snippet provides sufficient diagnostic info.

## R3: gRPC Status Code Mapping Best Practices

**Decision**: Map azureclient sentinel errors to gRPC status codes in the
pricing package using a simple `errors.Is()` switch.

**Rationale**: Google's gRPC error model recommends mapping domain errors to
the most semantically appropriate status code. The mapping is:

| Sentinel Error               | gRPC Code                | Reasoning                           |
| ---------------------------- | ------------------------ | ----------------------------------- |
| `ErrNotFound`                | `codes.NotFound`         | Resource/pricing data doesn't exist |
| `ErrRateLimited`             | `codes.ResourceExhausted`| Upstream rate limit                 |
| `ErrServiceUnavailable`      | `codes.Unavailable`      | Upstream temporarily down           |
| `ErrRequestFailed`           | `codes.Internal`         | Generic upstream failure            |
| `ErrInvalidResponse`         | `codes.Internal`         | Upstream returned bad data          |
| `ErrPaginationLimitExceeded` | `codes.Internal`         | Safety limit hit                    |
| `ErrInvalidConfig`           | `codes.Internal`         | Plugin misconfiguration             |
| `context.Canceled`           | `codes.Canceled`         | Client canceled request             |
| `context.DeadlineExceeded`   | `codes.DeadlineExceeded` | Request timed out                   |
| Network errors               | `codes.Unavailable`      | Cannot reach upstream               |

**Alternatives considered**:

- Mapping in azureclient package: Would introduce gRPC dependency in HTTP
  client package. Rejected to keep dependency graph clean.
- Using gRPC status directly in azureclient: Same issue. The HTTP client
  should be transport-agnostic.

## R4: Structured Logging Severity Strategy

**Decision**: Log errors at differentiated severity levels in `GetPrices()`.

**Rationale**: Aligns with the project's observability constitution
(Section III) and standard Go structured logging practices:

| Error Type              | Log Level | Reasoning                             |
| ----------------------- | --------- | ------------------------------------- |
| Network errors          | Debug     | Transient, handled by retries         |
| HTTP 4xx (except 429)   | Warn      | Client-side issue, needs attention    |
| HTTP 429 (rate limit)   | Warn      | Operational concern, not critical     |
| HTTP 5xx                | Error     | Upstream failure, needs investigation |
| JSON parse failure      | Error     | Unexpected response, needs attention  |
| Empty results           | Debug     | Expected for unknown SKUs             |
| Pagination limit        | Error     | Safety limit, unexpected data volume  |

Structured fields for all log entries: `region`, `sku`, `service`,
`error_category`, `http_status` (when applicable).

**Alternatives considered**:

- All errors at Error level: Too noisy, makes alerts meaningless.
- Network errors at Warn: Too noisy given retries handle these automatically.
- Empty results at Warn: Too noisy for legitimate "not found" queries.

## R5: Response Body Snippet Strategy

**Decision**: Read up to 256 bytes of response body for JSON parse error
diagnostics using `io.LimitReader`.

**Rationale**: 256 bytes is enough to identify whether the response is HTML,
XML, or truncated JSON without creating large error messages or log entries.
`io.LimitReader` prevents unbounded memory allocation from large responses.

**Alternatives considered**:

- Full body in error: Memory risk for large responses (MB-sized HTML pages).
- 64 bytes: Too short to identify response format.
- 512 bytes: Unnecessarily verbose for error messages.
- Storing body bytes separately: Over-engineering; the snippet in the error
  message is sufficient for diagnostics.
