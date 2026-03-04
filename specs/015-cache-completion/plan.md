# Implementation Plan: Cache Layer Completion (v0.3.0 Gaps)

**Branch**: `015-cache-completion` | **Date**: 2026-03-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/015-cache-completion/spec.md`

## Summary

Complete the v0.3.0 Caching Layer milestone by addressing two remaining
gaps in the CachedClient delivered by #12:

1. **FINFOCUS_CACHE_TTL env var** (#13): Read an environment variable
   at startup to override the default 24-hour cache TTL, following
   the existing `FINFOCUS_PLUGIN_PORT` pattern in `main.go`.
2. **Eviction callback logging** (#15): Register an eviction callback
   with the `expirable.LRU` to emit debug-level structured logs when
   entries are evicted.

Both changes are small, isolated, and touch existing files only.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: `github.com/hashicorp/golang-lru/v2/expirable`,
`github.com/rs/zerolog`
**Storage**: N/A — in-memory only (stateless constraint)
**Testing**: `go test` with `go test -race` for concurrency
**Target Platform**: Linux server (gRPC plugin subprocess)
**Project Type**: Single Go module
**Performance Goals**: Cache hits <10ms p99, eviction callback <1ms
**Constraints**: No persistent storage, no authenticated APIs
**Scale/Scope**: 2 files modified, 2 test files updated, ~50 lines new code

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Code Quality**: Linting via `make lint`, error handling for
  invalid env var values, no new complexity (simple parsing + callback)
- [x] **Testing**: TDD workflow, tests for env var parsing (valid,
  invalid, unset, zero), tests for eviction callback logging, race
  detector required
- [x] **User Experience**: Startup log shows effective TTL, eviction
  logs at debug level, invalid env var produces warning with fallback
- [x] **Documentation**: Godoc on new/modified functions, CLAUDE.md
  updated with env var, README updated with configuration option
- [x] **Performance**: Eviction callback is debug log only (zerolog
  nop at non-debug levels), no allocation overhead on critical path
- [x] **Architectural Constraints**: No authenticated APIs, no
  persistent storage, no infrastructure mutation, no bulk data

## Project Structure

### Documentation (this feature)

```text
specs/015-cache-completion/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/finfocus-plugin-azure-public/
└── main.go                    # MODIFY: add FINFOCUS_CACHE_TTL parsing

internal/azureclient/
├── cache.go                   # MODIFY: add eviction callback to NewLRU
├── cache_test.go              # MODIFY: add eviction logging tests
└── cachekey.go                # NO CHANGE (already complete)
```

**Structure Decision**: No new files needed. Both changes modify
existing files in their natural locations. The env var is read in
`main.go` (following the existing `FINFOCUS_PLUGIN_PORT` pattern) and
the eviction callback is registered in `cache.go` (where the LRU is
constructed).

## Design Decisions

### D1: Env var reading location

**Decision**: Read `FINFOCUS_CACHE_TTL` in `main.go` and set
`cacheConfig.TTL` before passing to `NewCachedClient`.

**Rationale**: Follows the existing pattern for `FINFOCUS_PLUGIN_PORT`
(lines 62-77) and `FINFOCUS_LOG_LEVEL` (lines 38-50). Keeps
`azureclient` package free of env var dependencies (pure library).

**Alternatives rejected**:

- Reading env var inside `DefaultCacheConfig()`: Would couple the
  library to os.Getenv, making it harder to test and reuse.

### D2: Invalid env var handling

**Decision**: Log a warning with the invalid value and fall back to
the default 24-hour TTL. Do NOT fail startup.

**Rationale**: Matches the `FINFOCUS_LOG_LEVEL` pattern (lines 44-49)
where invalid values log a warning and fall back to defaults. A
misconfigured cache TTL should not prevent the plugin from serving
cost estimates.

### D3: Eviction callback and reason inference

**Decision**: Register an eviction callback with the LRU constructor.
The callback receives the evicted key and value. To infer the eviction
reason, compare `CachedResult.CreatedAt + config.TTL` against
`time.Now()`:

- If `now >= createdAt + TTL`: reason is "expired"
- Otherwise: reason is "lru"

**Rationale**: The `golang-lru/v2/expirable` library does not pass an
eviction reason to the callback. Time-based inference is reliable
because the library evicts expired entries before LRU candidates.

**Alternatives rejected**:

- Generic "evicted" without reason: Loses the diagnostic value that
  distinguishes capacity pressure from stale data cleanup.
- Forking the library to add reason: Over-engineering for a debug log.

### D4: Eviction callback logger access

**Decision**: The eviction callback is a closure that captures the
`CachedClient`'s logger and config at construction time in
`NewCachedClient`.

**Rationale**: The callback must have access to the logger and TTL
config to emit structured logs and infer eviction reason. A closure
over the `CachedClient` fields is the simplest approach.

## Complexity Tracking

No constitution violations. No complexity justification needed.
