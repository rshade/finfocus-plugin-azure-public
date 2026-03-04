# Feature Specification: Cache Layer Completion (v0.3.0 Gaps)

**Feature Branch**: `015-cache-completion`
**Created**: 2026-03-03
**Status**: Complete
**Input**: Remaining gaps from #13 (TTL env var config) and #15 (eviction logging)

## Context

The CachedClient delivered in #12 already satisfies the majority of
acceptance criteria for issues #13, #14, and #15. Issue #14 (cache key
normalization) was verified complete and closed. This spec covers the
two remaining gaps:

1. **#13 gap**: No environment variable override for cache TTL
2. **#15 gap**: No structured logging when cache entries are evicted

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Environment Variable TTL Override (Priority: P1)

As an operator deploying the plugin, I want to override the default
cache TTL via an environment variable so I can tune cache freshness
without rebuilding the binary. For example, setting a shorter TTL in
staging environments to validate pricing changes faster, or disabling
caching entirely for debugging.

**Why this priority**: Operators need runtime configurability without
code changes. This is the only gap blocking #13 closure.

**Independent Test**: Can be fully tested by setting
`FINFOCUS_CACHE_TTL` to various values and verifying the cache
behaves accordingly.

**Acceptance Scenarios**:

1. **Given** `FINFOCUS_CACHE_TTL=10s` is set, **When** the plugin
   starts, **Then** cache entries expire after 10 seconds instead of
   the default 24 hours.
2. **Given** `FINFOCUS_CACHE_TTL=0s` is set, **When** the plugin
   starts, **Then** caching is disabled and every lookup goes to the
   Azure API.
3. **Given** `FINFOCUS_CACHE_TTL` is not set, **When** the plugin
   starts, **Then** the default 24-hour TTL is used.
4. **Given** `FINFOCUS_CACHE_TTL=banana` (invalid duration), **When**
   the plugin starts, **Then** an error is logged and the default TTL
   is used as a fallback.

---

### User Story 2 - Eviction Event Logging (Priority: P2)

As a developer debugging cache behavior, I want to see structured log
entries when cache entries are evicted (by TTL expiry or LRU overflow)
so I can understand cache churn and tune capacity or TTL settings.

**Why this priority**: Observability for eviction events completes the
cache monitoring story. Without it, operators can see hits and misses
but cannot distinguish between TTL expiry and LRU pressure as causes
of cache misses. This is the only gap blocking #15 closure.

**Independent Test**: Can be fully tested by configuring a small cache,
filling it beyond capacity, and verifying eviction log entries appear.

**Acceptance Scenarios**:

1. **Given** the cache is at maximum capacity, **When** a new entry is
   added causing LRU eviction, **Then** a debug-level log entry is
   emitted with the evicted key and reason "lru".
2. **Given** a cached entry whose TTL has expired, **When** the entry
   is accessed or cleaned up, **Then** a debug-level log entry is
   emitted with the evicted key and reason "expired".
3. **Given** the logger is not configured (nop logger), **When**
   evictions occur, **Then** no performance penalty is incurred from
   the eviction callback.

---

### Edge Cases

- What happens when `FINFOCUS_CACHE_TTL` is set to a negative
  duration (e.g., `-5s`)? Should be treated as invalid input with
  fallback to default.
- What happens when `FINFOCUS_CACHE_TTL` exceeds the ExpiresAtTTL?
  The existing overflow protection in CachedClient already caps the
  caller-facing hint, so no special handling needed.
- What happens when eviction logging is enabled but the cache is
  under extremely high throughput? Debug-level logs are only emitted
  when the logger is configured at debug level, so production
  deployments at info level incur no overhead.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST read the `FINFOCUS_CACHE_TTL` environment
  variable at startup and use its value as the internal cache TTL.
- **FR-002**: System MUST parse the environment variable as a standard
  duration string (e.g., "10s", "1h", "24h").
- **FR-003**: System MUST fall back to the default 24-hour TTL when
  the environment variable is not set or contains an invalid value.
- **FR-004**: System MUST log a warning when the environment variable
  contains an invalid duration value, including the invalid value in
  the log entry.
- **FR-005**: System MUST log an informational message at startup
  indicating the effective cache TTL in use.
- **FR-006**: System MUST emit a debug-level structured log entry when
  a cache entry is evicted, including the evicted cache key.
- **FR-007**: System MUST include an eviction reason field in eviction
  log entries to distinguish LRU eviction from TTL expiry.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Setting `FINFOCUS_CACHE_TTL=1s` causes cache entries to
  expire within 2 seconds (verifiable in tests).
- **SC-002**: Setting `FINFOCUS_CACHE_TTL=0s` results in zero cache
  hits across any number of repeated identical queries.
- **SC-003**: Eviction events produce structured log entries visible
  at debug log level containing both the key and reason.
- **SC-004**: All new functionality passes Go's race detector under
  concurrent access (`go test -race`).
- **SC-005**: Existing cache tests continue to pass with no
  regressions.

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (>=80%
  for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic
  complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then
  format)
- [x] Integration test needs identified (external API interactions)
- [x] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (cache hits <10ms p99, API
  calls <2s p95)
- [x] Observability requirements specified (logging, metrics)

### Documentation

- [x] README.md updates identified (if user-facing changes)
- [x] API documentation needs outlined (godoc comments, contracts)
- [x] Docstring coverage >=80% maintained (all exported symbols
  documented)
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

- The `FINFOCUS_CACHE_TTL` environment variable is read once at plugin
  startup. Changing it requires a plugin restart.
- The eviction callback provided to the LRU library may be called from
  any goroutine, so it must be safe for concurrent use (zerolog
  loggers are already thread-safe).
- Debug-level eviction logs are acceptable for production since most
  deployments run at info level, making the callback a no-op in
  practice.
- The `golang-lru/v2/expirable` library's eviction callback fires for
  both LRU and TTL evictions, but does not distinguish between them.
  The eviction reason may need to be inferred (e.g., by checking if
  the entry's age exceeds TTL at eviction time).
