# Research: Integration Tests with Live Azure Retail Prices API

**Branch**: `022-integration-tests` | **Date**: 2026-03-30

## R1: Test File Location

**Decision**: Place new integration tests in `examples/` package

**Rationale**: The project already has `examples/azure_client_integration_test.go`
with the `//go:build integration` tag. The CI workflow at
`.github/workflows/test.yml` already runs
`go test -v -tags=integration -timeout=2m ./examples/...`. Adding tests to this
package requires zero CI changes.

**Alternatives considered**:

- `tests/integration/` — would require new CI config and breaks existing pattern
- `internal/pricing/` with build tag — mixes unit and integration tests

## R2: Calculator Setup for Integration Tests

**Decision**: Construct a real `azureclient.Client` → `CachedClient` →
`Calculator` chain in each test (or shared test helper)

**Rationale**: `NewCalculator(logger, cachedClient...)` accepts a variadic
`*azureclient.CachedClient` (`calculator.go:33-44`). Without it, the
`cachedClient` field is nil and cost lookups fail. Integration tests must
exercise the full pipeline.

**Construction chain** (exact API):

```text
azureclient.DefaultConfig()       → Config          (types.go:62-73)
azureclient.NewClient(config)     → *Client         (client.go:47)
azureclient.DefaultCacheConfig()  → CacheConfig     (cache.go:38-45)
azureclient.NewCachedClient(c,cc) → *CachedClient   (cache.go:67)
pricing.NewCalculator(log, cc)    → *Calculator      (calculator.go:33)
```

**Alternatives considered**:

- Test via gRPC server — adds unnecessary complexity for testing business logic
- Test `azureclient.Client` directly — already covered by existing tests, misses
  Calculator logic

## R3: Reference Prices for ±25% Assertions

**Decision**: Define reference prices as constants in the test file, with
comments documenting when they were last verified. Use helper function for range
assertions.

**Rationale**: Constants are easy to update and grep for. A helper like
`assertInRange(t, actual, reference, tolerance)` keeps test bodies clean.

**Alternatives considered**:

- External JSON file with prices — overengineered for 4-5 reference values
- No reference prices, just `> 0` — too loose per clarification (±25% chosen)

## R4: Rate Limiting Between Tests

**Decision**: Use `time.Sleep(12 * time.Second)` between sequential test
functions. Tests that reuse cached results (cache hit tests) do not need delays.

**Rationale**: Azure Retail Prices API has informal rate limits. 12-second gaps
keep under 5 queries/minute. Cache hit tests make no API call, so no delay
needed.

**Alternatives considered**:

- Global rate limiter — overengineered for sequential test execution
- `t.Parallel()` with semaphore — adds complexity, sequential is simpler

## R5: Cache Verification Method

**Decision**: Use `CachedClient.Stats()` to check `Hits` and `Misses` counters
rather than timing-based assertions.

**Rationale**: Timing-based assertions are flaky (depend on system load, network
latency). `CachedClient.Stats()` (`cache.go:145-147`) returns
`*CacheStats{Hits, Misses: atomic.Int64}` (`cache.go:48-51`). Checking
`stats.Hits.Load() > 0` after a second call is deterministic.

**Alternatives considered**:

- Timing-based (second call < 10ms) — flaky in CI environments
- Custom cache wrapper — unnecessary when stats are already exposed

## R6: CI Workflow Changes

**Decision**: No CI changes needed. Existing `integration` job already covers
the `examples/` package.

**Rationale**: `.github/workflows/test.yml` line 53:
`go test -v -tags=integration -timeout=2m ./examples/...` — new test files
in `examples/` are automatically included. The 2-minute timeout is sufficient
for the planned test suite (rate limiting delays total ~48s for 4 API calls).

**Alternatives considered**:

- Separate workflow for new tests — unnecessary duplication
- Add `SKIP_INTEGRATION` env var to CI — can be added later if needed
