# Implementation Plan: HTTP Client with Retry Logic

**Branch**: `006-http-client-retry` | **Date**: 2026-02-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/006-http-client-retry/spec.md`

## Summary

Implement an HTTP client using `hashicorp/go-retryablehttp` to query the Azure Retail Prices API (`https://prices.azure.com/api/retail/prices`) with automatic retry logic for transient failures. The client handles HTTP 429 (rate limit) and 503 (service unavailable) responses with exponential backoff, respects Retry-After headers, and provides structured logging for all retry events via zerolog.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: `github.com/hashicorp/go-retryablehttp` (HTTP client with retry), `github.com/rs/zerolog` (structured logging)
**Storage**: N/A - stateless plugin (in-memory only)
**Testing**: `go test`, `go test -race`, table-driven tests, mock HTTP server via `httptest`
**Target Platform**: Linux server (gRPC plugin process)
**Project Type**: Single project (Go module)
**Performance Goals**: Cache miss with API call <2s (p95), <5s (p99)
**Constraints**: 60s request timeout, max 3 retries, exponential backoff 1s-30s, no authenticated APIs
**Scale/Scope**: Single Azure Retail Prices API endpoint, single HTTP client instance per plugin

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes linting checks (`make lint`), error handling strategy (explicit errors, no silent failures), and complexity justification (not needed - standard HTTP client patterns)
- [x] **Testing**: Plan includes TDD workflow (tests before implementation), ≥80% coverage target, race detector for concurrent code (httptest mock server tests)
- [x] **User Experience**: Plan addresses plugin lifecycle (client integrates with existing gRPC server), observability (zerolog structured logging for all retries), and error handling (gRPC error codes with context)
- [x] **Documentation**: Plan includes godoc comments for exported functions, README updates (Azure API usage), and CLAUDE.md updates (HTTP client patterns)
- [x] **Performance**: Plan includes performance targets (60s timeout, <2s p95 response), reliability guarantees (retry logic with backoff), and resource constraints (connection pooling via retryablehttp defaults)
- [x] **Architectural Constraints**: Plan DOES NOT violate "Hard No's":
  - ✅ No authenticated Azure APIs (uses unauthenticated prices.azure.com)
  - ✅ No persistent storage (HTTP client is stateless)
  - ✅ No infrastructure mutation (read-only pricing queries)
  - ✅ No bulk data embedding (fetches pricing on-demand)

## Project Structure

### Documentation (this feature)

```text
specs/006-http-client-retry/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── client/
│   └── client.go              # Existing - cloud provider client (template)
├── azureclient/               # NEW - Azure-specific HTTP client
│   ├── client.go              # RetryableHTTP client wrapper
│   ├── client_test.go         # Unit tests with mock HTTP server
│   ├── retry.go               # Custom retry policy implementation
│   ├── retry_test.go          # Retry policy unit tests
│   └── types.go               # Request/Response types for Azure API
├── logging/
│   ├── request.go             # Existing - request logging
│   └── request_test.go        # Existing tests
└── pricing/
    ├── calculator.go          # Existing - pricing calculator
    ├── calculator_test.go     # Existing tests
    └── data.go                # Existing - pricing data structures

examples/
└── azure_client_integration_test.go  # Integration test against live API
```

**Structure Decision**: New `internal/azureclient` package separates Azure-specific HTTP client from generic `internal/client` template. This follows the existing pattern where domain-specific packages live under `internal/`.

## Complexity Tracking

> No violations requiring justification. Standard HTTP client patterns with well-understood retry logic.
