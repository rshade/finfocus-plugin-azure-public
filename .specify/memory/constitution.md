<!--
Sync Impact Report:
Version: 1.0.0 → 1.0.0 (Initial constitution creation)
Modified Principles: N/A (new constitution)
Added Sections:
  - Code Quality Standards
  - Testing Standards
  - User Experience Consistency
  - Documentation Standards
  - Performance & Reliability Requirements
  - Governance
Templates Status:
  ✅ plan-template.md - Constitution Check section aligns with new principles
  ✅ spec-template.md - Requirements sections align with code quality & testing principles
  ✅ tasks-template.md - Task categorization supports all principle-driven work
Follow-up TODOs: None
-->

# finfocus-plugin-azure-public Constitution

## Core Principles

### I. Code Quality Standards

**Non-Negotiable Requirements:**

- All Go code MUST pass `golangci-lint` with project configuration (`.golangci-lint.yml` MUST NOT be modified without explicit justification)
- Code MUST follow standard Go idioms and formatting (`gofmt`, `goimports`)
- Exported functions, types, and packages MUST have godoc comments
- Cyclomatic complexity MUST NOT exceed 15 per function (enforced by linter)
- Code duplication MUST be eliminated through appropriate abstraction (not premature abstraction)
- Error handling MUST be explicit - no silent failures, all errors logged or returned
- Magic numbers MUST be replaced with named constants with clear intent
- File size SHOULD be <300 lines; larger files MUST be justified and broken into logical modules where possible

**Rationale:** Code quality directly impacts maintainability, debugging efficiency, and onboarding speed. The project uses automated linting to enforce standards consistently.

### II. Testing Standards (NON-NEGOTIABLE)

**Test-Driven Development (TDD) Requirements:**

- Tests MUST be written BEFORE implementation for all new features
- Tests MUST fail initially, then pass after implementation (Red-Green-Refactor cycle)
- Unit test coverage MUST be ≥80% for business logic (pricing calculations, field mapping, cache logic)
- Integration tests MUST cover all external API interactions (Azure Retail Prices API)
- Race detector (`go test -race`) MUST pass for all concurrent code (especially caching layer)

**Test Quality Requirements:**

- Each test MUST have a single, clear purpose (no redundant test cases)
- Table-driven tests MUST be used for variations on the same behavior
- Test names MUST describe the scenario being tested (format: `Test<Function>_<Scenario>_<ExpectedOutcome>`)
- Mock external dependencies (HTTP client) for unit tests; use real API sparingly for integration tests
- Tests MUST execute in <1 second for the entire suite (unit tests); integration tests allowed up to 30 seconds

**What NOT to Test:**

- Do NOT write unit tests for pure CRUD operations requiring live HTTP clients (use integration tests in `examples/`)
- Do NOT over-mock; if a dependency doesn't provide an interface, integration testing may be more appropriate
- Do NOT create complex mocking infrastructure or helper functions that wrap struct literals

**Rationale:** TDD ensures specification correctness before implementation. High test coverage prevents regressions. Fast tests enable rapid development cycles.

### III. User Experience Consistency

**Plugin Lifecycle Guarantees:**

- Plugin MUST announce listening port via stdout in format `PORT=XXXXX` (and ONLY this - no other stdout pollution)
- Plugin MUST accept gRPC connections immediately after port announcement
- Plugin MUST respond to health checks within 100ms
- Plugin MUST handle graceful shutdown on SIGTERM/SIGINT (drain in-flight requests, max 10s)
- Plugin MUST never crash; all panics MUST be recovered and logged as fatal errors

**API Stability:**

- gRPC method signatures MUST NOT break backward compatibility (use protocol buffer evolution)
- Error messages MUST be actionable and include context (resource type, query parameters, API response codes)
- Error codes MUST follow gRPC status codes consistently
- Response times MUST be predictable (cache hits <10ms, cache misses with API call <2s p95)

**Observability:**

- All logs MUST be structured JSON via `zerolog` and MUST go to stderr only
- All errors MUST be logged with severity level (error, warn, info, debug)
- Cache hit/miss ratio MUST be logged every 1000 requests or 5 minutes (whichever comes first)
- API request failures MUST be logged with full context (status code, URL, retry attempt)

**Rationale:** Consistent behavior builds trust. Clear error messages reduce support burden. Observability enables debugging in production.

### IV. Documentation Standards (NON-NEGOTIABLE)

**User-Facing Documentation Requirements:**

- README.md MUST contain:
  - Project purpose and scope (1-2 paragraphs)
  - Installation instructions (from source)
  - Basic usage examples (at least 2 common scenarios)
  - Supported Azure resource types (updated with each new resource support)
  - Configuration options and environment variables
  - Troubleshooting guide for common issues
- CLAUDE.md (project context) MUST be updated when:
  - Core architectural constraints change
  - New essential commands are added
  - Development workflow patterns are established
  - Repeated issues/solutions are identified
- ROADMAP.md MUST reflect current development priorities and completed milestones
- Changelog MUST be maintained (format: Keep a Changelog v1.0.0)

**Code Documentation Requirements:**

- Exported Go functions MUST have godoc comments explaining:
  - Purpose of the function
  - Parameters and their constraints
  - Return values and error conditions
  - Example usage for non-obvious functions
- Complex algorithms (e.g., cache eviction, retry logic) MUST have inline comments explaining "why" not "what"
- API contracts (gRPC service implementations) MUST document:
  - Expected input constraints
  - Possible error responses
  - Performance characteristics (e.g., "may block up to 2s on cold cache")

**Rationale:** High-quality documentation reduces onboarding time from hours to minutes. Clear API documentation prevents misuse and support requests.

### V. Performance & Reliability Requirements

**Performance Targets:**

- Cache hit response time: <10ms (p99)
- Cache miss with API call: <2s (p95), <5s (p99)
- Concurrent request handling: MUST support ≥100 concurrent gRPC requests without degradation
- Memory usage: MUST stay bounded (cache LRU eviction prevents unbounded growth)
- Startup time: <500ms from process start to PORT announcement

**Reliability Guarantees:**

- Azure API failures (429, 503) MUST trigger exponential backoff retry (3 attempts, max backoff 30s)
- Network timeouts MUST be configured (HTTP client timeout: 10s per request)
- Transient errors MUST NOT cause plugin crash; MUST return gRPC error with retryable status
- Cache MUST be thread-safe (validated with `go test -race`)
- Plugin MUST operate statelessly (no persistent storage dependency)

**Resource Constraints:**

- HTTP client MUST use connection pooling (max 10 connections to Azure API)
- Cache TTL MUST default to 24 hours (configurable via environment variable)
- Pagination MUST be handled automatically for Azure API responses (no partial data returned)

**Rationale:** Performance targets ensure plugin doesn't become a bottleneck in FinFocus cost estimation workflows. Reliability guarantees ensure production readiness.

## Architectural Constraints ("Hard No's")

These constraints MUST NEVER be violated:

1. **No Authenticated Azure APIs**: Plugin MUST only use unauthenticated `https://prices.azure.com/api/retail/prices` endpoint. MUST NOT require Azure credentials (Subscription, Tenant ID, Client Secret, or `az login`).

2. **No Persistent Storage**: Plugin MUST operate statelessly. In-memory TTL cache is allowed. MUST NOT use SQLite, BoltDB, filesystem, or any long-term storage.

3. **No Infrastructure Mutation**: Plugin is read-only. Cost calculations based on `ResourceDescriptor` inputs only. MUST NOT validate if resources exist in Azure. MUST NOT create/modify/delete Azure resources.

4. **No Bulk Data Embedding**: MUST NOT embed Azure pricing catalog in binary. All pricing data MUST be fetched dynamically based on requested resources.

**Rationale:** These constraints define the plugin's scope and ensure it remains lightweight, portable, and secure.

## Development Workflow

### Version Management

- Development versions MUST follow format: `MAJOR.MINOR.NEXT_PATCH-dev`
- Versions MUST be auto-calculated from latest git tag
- Version injection via LDFLAGS: `-X main.version=$(DEV_VERSION)`

### Code Review Requirements

- All PRs MUST pass CI (build, test, lint) before merge
- All PRs MUST include tests for new functionality
- All PRs MUST update documentation if user-facing behavior changes
- Breaking changes MUST be documented in CHANGELOG.md with migration guide

### Quality Gates

Before merging to main:

1. `make build` succeeds
2. `make test` passes (all tests, including race detector)
3. `make lint` passes (can take >5 minutes; use extended timeout)
4. Integration tests pass against live Azure Retail Prices API (if applicable)
5. Documentation updated (README, godoc, CHANGELOG if needed)

### Commit Standards

- Commits MUST follow Conventional Commits format (feat:, fix:, docs:, refactor:, test:, chore:)
- MUST NOT include "🤖 Generated with [Claude Code]" or "Co-Authored-By: Claude" in commit messages
- Commit messages MUST be descriptive (explain "why" not just "what")

## Governance

### Amendment Process

- Constitution changes MUST be documented in the Sync Impact Report (HTML comment at top of this file)
- Version increments follow semantic versioning:
  - **MAJOR**: Backward-incompatible principle removals or redefinitions
  - **MINOR**: New principle/section added or materially expanded guidance
  - **PATCH**: Clarifications, wording, typo fixes
- All dependent templates (plan, spec, tasks) MUST be updated for consistency
- Amendments MUST be committed with message: `docs: amend constitution to vX.Y.Z (description)`

### Compliance Verification

- All PRs MUST be verified against constitution principles
- Violations MUST be justified in PR description or rejected
- Complexity increases MUST be explicitly justified (see plan-template.md "Complexity Tracking")
- Constitution supersedes all other practices and documentation

### Living Document

- Constitution MUST be updated when:
  - New non-negotiable standards are established
  - Project scope or architectural constraints change
  - Repeated compliance issues indicate missing/unclear guidance
- Updates MUST propagate to:
  - `.specify/templates/plan-template.md` (Constitution Check section)
  - `.specify/templates/spec-template.md` (Requirements alignment)
  - `.specify/templates/tasks-template.md` (Task categorization)
  - `CLAUDE.md` (development guidance)

**Version**: 1.0.0 | **Ratified**: 2026-01-21 | **Last Amended**: 2026-01-21
