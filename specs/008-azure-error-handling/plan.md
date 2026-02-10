# Implementation Plan: Comprehensive Error Handling for Azure API Failures

**Branch**: `008-azure-error-handling` | **Date**: 2026-02-09 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/008-azure-error-handling/spec.md`

## Summary

Enhance the Azure Retail Prices API client with comprehensive error handling:
contextual error messages enriched with query parameters (region, SKU, service),
error classification via sentinel errors enabling callers to distinguish
retryable from non-retryable failures, structured zerolog logging at
differentiated severity levels, empty result detection, response body snippets
for JSON parse failures, and a gRPC status code mapping layer in the pricing
package.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: `github.com/hashicorp/go-retryablehttp` (HTTP retry),
`github.com/rs/zerolog` (structured logging),
`google.golang.org/grpc` (gRPC status codes)
**Storage**: N/A - stateless plugin (in-memory only)
**Testing**: `go test` with table-driven tests, `go test -race`
**Target Platform**: Linux server (gRPC plugin process)
**Project Type**: Single Go module
**Performance Goals**: No additional latency overhead; error paths must not
block longer than the failed request itself
**Constraints**: Error handling must not introduce new dependencies; must work
with existing retryablehttp retry layer
**Scale/Scope**: ~5 files modified, ~2 new files, ~200-300 lines of new code

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Code Quality**: Plan includes linting checks (`make lint`), explicit
  error handling (sentinel errors + wrapping), and complexity kept low
  (each new function <15 cyclomatic complexity)
- [x] **Testing**: Plan includes TDD workflow (tests written before
  implementation), >=80% coverage target for error handling paths, race
  detector for concurrent client usage
- [x] **User Experience**: Plan addresses observability (structured logging with
  region/SKU/service fields at differentiated severity levels), error handling
  (contextual messages, error classification), and gRPC status code mapping
- [x] **Documentation**: Plan includes godoc comments for all new exported
  symbols (ErrNotFound, MapToGRPCStatus), CLAUDE.md updates for error handling
  patterns, docstring coverage >=80% maintained
- [x] **Performance**: No additional latency; error paths return immediately
  after failure detection; response body reads capped at 256 bytes for snippets
- [x] **Architectural Constraints**: DOES NOT use authenticated Azure APIs,
  DOES NOT introduce persistent storage, DOES NOT mutate infrastructure,
  DOES NOT embed bulk pricing data

## Project Structure

### Documentation (this feature)

```text
specs/008-azure-error-handling/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── error-contract.go
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── azureclient/
│   ├── errors.go          # MODIFY: Add ErrNotFound, move ErrPaginationLimitExceeded
│   ├── client.go          # MODIFY: Add logger field, query context wrapping,
│   │                      #   empty result detection, response snippets, HTTP 404,
│   │                      #   structured error logging
│   ├── client_test.go     # MODIFY: Add tests for new error handling paths
│   ├── types.go           # NO CHANGE
│   ├── retry.go           # NO CHANGE
│   ├── logger.go          # NO CHANGE
│   ├── logger_test.go     # NO CHANGE
│   ├── retry_test.go      # NO CHANGE
│   └── types_test.go      # NO CHANGE
├── pricing/
│   ├── calculator.go      # NO CHANGE (stubs - gRPC mapping used when stubs are replaced)
│   ├── calculator_test.go # NO CHANGE
│   └── errors.go          # NEW: gRPC status code mapping helper
└── pricing/
    └── errors_test.go     # NEW: tests for gRPC mapping
```

**Structure Decision**: Existing Go package structure is used. Error handling
enhancements go into `internal/azureclient/` (where errors originate). The
gRPC status code mapping goes into `internal/pricing/` (where gRPC errors are
produced). No new packages needed.

## Design Decisions

### D1: Keep Sentinel Error Pattern (not custom error type)

**Decision**: Continue using sentinel errors with `fmt.Errorf("%w: context")`
wrapping rather than introducing a custom `PricingError` struct type.

**Rationale**: The existing codebase uses sentinel errors consistently.
Callers use `errors.Is()` for classification. Adding a struct type would
require callers to use type assertions, which is a different pattern. The
sentinel approach provides error classification (via `errors.Is`) and context
(via wrapping) with minimal code changes.

**Trade-off**: Query context is embedded in the error string rather than as
typed fields. This is acceptable because:

- Error messages are for humans (operators reading logs)
- Programmatic classification uses `errors.Is()` with sentinels
- Structured fields are available in the log entries (zerolog), not in the
  error itself

### D2: Context Wrapping at GetPrices Level (not fetchPage)

**Decision**: Add query context wrapping in `GetPrices()`, not in `fetchPage()`.

**Rationale**: `fetchPage` is a low-level HTTP method that operates on URLs.
`GetPrices` is the public API that knows the `PriceQuery`. Wrapping at
`GetPrices` avoids passing `PriceQuery` through `fetchPage` and keeps
separation of concerns clean.

### D3: Logger Field on Client Struct

**Decision**: Add a `logger zerolog.Logger` field to the `Client` struct,
separate from the retryablehttp logger adapter.

**Rationale**: The retryablehttp adapter only logs retry events. Application-
level error logging (structured fields, severity differentiation) needs a
direct logger reference. The logger is already available in `Config.Logger`
and just needs to be stored on the struct.

### D4: gRPC Mapping in pricing Package

**Decision**: Place the `MapToGRPCStatus` function in `internal/pricing/`
rather than `internal/azureclient/`.

**Rationale**: `azureclient` should not depend on gRPC. The pricing package
already imports `google.golang.org/grpc/codes` and `status`. This keeps the
dependency graph clean: `pricing` -> `azureclient`, not the reverse.

### D5: Response Body Snippet via io.LimitReader

**Decision**: Use `io.LimitReader` to cap response body reads at 256 bytes
for JSON parse error snippets.

**Rationale**: Prevents unbounded memory allocation from large error responses
(e.g., HTML error pages). The 256-byte cap provides enough context for
diagnosis without excessive log/error message size.

## Implementation Phases

### Phase 1: Error Infrastructure (errors.go + client.go logger)

1. Add `ErrNotFound` sentinel to `errors.go`
2. Move `ErrPaginationLimitExceeded` from `client.go` to `errors.go`
3. Add `logger zerolog.Logger` field to `Client` struct
4. Store `config.Logger` in `Client` during `NewClient()`

### Phase 2: Enhanced fetchPage Error Handling

1. Add HTTP 404 case to the status code switch in `fetchPage`
2. Add response body snippet (256 bytes) to JSON unmarshal errors
3. Cap response body reads with `io.LimitReader` for error responses

### Phase 3: GetPrices Context Enrichment + Empty Results

1. Add query context wrapping to all errors returned from `GetPrices`
2. Add empty result detection after pagination completes
3. Add structured logging at differentiated severity levels
4. Add mid-pagination error context (page number)

### Phase 4: gRPC Status Code Mapping

1. Create `internal/pricing/errors.go` with `MapToGRPCStatus` function
2. Map: ErrNotFound -> NotFound, ErrRateLimited -> ResourceExhausted,
   ErrServiceUnavailable -> Unavailable, ErrRequestFailed -> Internal,
   ErrInvalidResponse -> Internal, network errors -> Unavailable

### Phase 5: Tests + Validation

1. Tests for HTTP 404 -> ErrNotFound
2. Tests for empty results -> ErrNotFound with query context
3. Tests for JSON parse error with response snippet
4. Tests for structured logging output verification
5. Tests for gRPC status code mapping
6. Tests for mid-pagination error context
7. Run `make test`, `make lint`, verify >=80% coverage

## Complexity Tracking

No constitution violations to justify. All changes follow existing patterns.
