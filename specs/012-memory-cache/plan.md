# Implementation Plan: Thread-Safe In-Memory Cache

**Branch**: `012-memory-cache` | **Date**: 2026-03-02 |
**Spec**: [spec.md](spec.md)
**Input**: Feature specification from
`/specs/012-memory-cache/spec.md`

## Summary

Add an in-memory LRU cache with TTL to the Azure pricing
plugin, eliminating redundant Azure Retail Prices API calls
within the plugin's process lifetime. The cache uses
`hashicorp/golang-lru/v2/expirable` for thread-safe LRU with
automatic TTL expiry, and sets `expires_at` on all gRPC cost
responses so that callers (finfocus CLI, third-party) can
manage their own L2 caches. Requires bumping finfocus-spec from
v0.5.4 to v0.5.7 for `expires_at` fields and `BatchCost` RPC.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**:

- `hashicorp/golang-lru/v2` v2.0.7 (new — LRU cache with TTL)
- `rshade/finfocus-spec` v0.5.7 (bump — `expires_at`, BatchCost)
- `hashicorp/go-retryablehttp` v0.7.7 (existing — HTTP retry)
- `rs/zerolog` v1.34.0 (existing — structured logging)

**Storage**: In-memory only (stateless constraint)
**Testing**: `go test -race ./...` via `make test`
**Target Platform**: Linux server (gRPC plugin process)
**Project Type**: Single Go module
**Performance Goals**: Cache hits <1ms (p99), 100+ concurrent
gRPC requests without degradation
**Constraints**: <10ms p99 cache hit latency, max 1000 cache
entries, L1 TTL 24h, `expires_at` TTL 4h
**Scale/Scope**: Single-process in-memory cache, ~1000 entries

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after
Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes linting checks
  (`make lint`), error handling (FR-015 no error caching,
  sentinel error propagation), complexity kept low (thin
  wrapper over golang-lru)
- [x] **Testing**: Plan includes TDD workflow, >=80% coverage
  target, race detector for concurrent code (`go test -race`)
- [x] **User Experience**: Plan addresses plugin lifecycle
  (cache initializes in constructor, no startup delay),
  observability (cache hit/miss ratio logging per
  constitution Section III), error handling (gRPC error
  mapping preserved)
- [x] **Documentation**: Plan includes godoc comments for
  all exported cache functions, CLAUDE.md updates for cache
  configuration, docstring coverage >=80%
- [x] **Performance**: Cache hits <10ms p99 (constitution),
  <1ms target (spec), 100 concurrent requests, 1000 max
  entries with LRU eviction
- [x] **Architectural Constraints**: No authenticated APIs,
  no persistent storage (in-memory only), no infrastructure
  mutation, no bulk data embedding

**Constitution Alignment — TTL Default**:

The constitution (Section V) specifies "Cache TTL MUST default
to 24 hours." The L1 internal cache uses exactly this value.
The separate `expires_at` TTL (4 hours) is a gRPC response
hint, not the internal cache TTL, so no deviation exists.
No constitution amendment required.

## Project Structure

### Documentation (this feature)

```text
specs/012-memory-cache/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── cache-api.md     # Cache interface contract
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── azureclient/
│   ├── cache.go         # Cache wrapper (new)
│   ├── cache_test.go    # Cache unit tests (new)
│   ├── cachekey.go      # Key normalization (new)
│   ├── cachekey_test.go # Key normalization tests (new)
│   ├── client.go        # Existing — no changes
│   ├── errors.go        # Existing — no changes
│   ├── filter.go        # Existing — no changes
│   ├── types.go         # Existing — no changes
│   └── ...
├── pricing/
│   ├── calculator.go    # Modified — inject cache, set
│   │                    #   expires_at on responses
│   ├── calculator_test.go # Modified — test cache integration
│   ├── errors.go        # Existing — no changes
│   └── ...
└── ...

cmd/finfocus-plugin-azure-public/
└── main.go              # Modified — construct cache, pass to
                         #   Calculator
```

**Structure Decision**: The cache lives in `internal/azureclient`
because it wraps Azure pricing data and uses `PriceQuery` /
`PriceItem` types defined there. The key normalization is
co-located since it operates on `PriceQuery` dimensions. The
`pricing` package consumes the cache and produces `expires_at`
on gRPC responses.

## Complexity Tracking

> No constitution violations. L1 TTL matches the
> constitution's 24-hour default. The `expires_at` TTL
> (4 hours) is a new concept not covered by the
> constitution.

**Estimated Cyclomatic Complexity** (target: <15 per function):

- `CacheKey`: ~3 (string normalization, fixed field order)
- `NewCachedClient`: ~4 (config validation, LRU construction)
- `CachedClient.GetPrices`: ~6 (cache check, miss delegation,
  result wrapping, stats, observability check)
- `Calculator.EstimateCost`: ~5 (query build, cache call, error
  mapping, expires_at setting)

All functions well under the 15-function complexity limit.
