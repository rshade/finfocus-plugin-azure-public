# Implementation Plan: Integration Tests with Live Azure API

**Branch**: `022-integration-tests` | **Date**: 2026-03-30 |
**Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/022-integration-tests/spec.md`

## Summary

Implement end-to-end integration tests that exercise the full EstimateCost
pipeline (`Calculator` → `CachedClient` → `Client` → live Azure Retail Prices
API) for both VM and Managed Disk resource types. Tests use ±25% range
assertions against reference prices, cache hit verification via atomic
`CacheStats` counters, rate-limited sequential execution (12s between API
calls), and `SKIP_INTEGRATION` env var support. Tests live in the existing
`examples/` package — no CI workflow changes needed.

## Technical Context

**Language/Version**: Go 1.25.7 (from `go.mod`)
**Primary Dependencies**: `azureclient` (HTTP client with retry),
`pricing` (Calculator, mapper, disk), `finfocus-spec` v0.5.7 (pluginsdk,
`finfocusv1` proto), `zerolog` v1.34.0 (logging), `golang-lru/v2` v2.0.7
(cache)
**Storage**: N/A — stateless, in-memory LRU+TTL cache only
**Testing**: `go test -v -tags=integration -timeout=5m ./examples/...`
**Target Platform**: Linux (CI: ubuntu-latest), local dev
**Project Type**: Single Go module
(`github.com/rshade/finfocus-plugin-azure-public`)
**Performance Goals**: Tests complete within 5 minutes total (rate-limiting
delays ~48s for 4 API calls + execution)
**Constraints**: Max 5 API queries/minute (12s delay), ±25% price tolerance,
no Azure auth required (public API only)
**Scale/Scope**: 1 new test file (~250 lines), 8 test functions, 4 helper
functions, ~4-6 reference price constants

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Code Quality**: Test file uses `gofmt`/`goimports`, helper functions
  have godoc comments, error handling is explicit (`t.Fatalf` on setup
  failures, `t.Errorf` on assertion failures), no magic numbers (named
  constants for prices and tolerance)
- [x] **Testing**: Integration tests cover all external API interactions
  (Azure Retail Prices API via `Calculator.EstimateCost`), no concurrent
  shared state (sequential execution), table-driven structure not needed
  (each test has unique setup/assertions)
- [x] **User Experience**: Tests validate gRPC error codes are actionable
  (`NotFound` for invalid SKU, `InvalidArgument` for missing attrs),
  structured logging present via zerolog in Calculator chain
- [x] **Documentation**: Test file has godoc package comment (shared with
  `examples` package), quickstart.md provides run instructions, README update
  with integration test section planned
- [x] **Performance**: Tests complete within 5 minutes (constitution allows
  up to 30s for integration tests per individual test, total suite ~90s),
  rate limiting enforced (12s delays), context timeouts prevent hanging
  (30s per `EstimateCost` call)
- [x] **Architectural Constraints**: Uses only unauthenticated Azure Retail
  Prices API (`https://prices.azure.com/api/retail/prices`), no persistent
  storage, read-only, no bulk data embedding

## Project Structure

### Documentation (this feature)

```text
specs/022-integration-tests/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: research decisions
├── data-model.md        # Phase 1: test entities
├── quickstart.md        # Phase 1: how to run
├── contracts/
│   └── test_helpers.go  # Phase 1: helper signatures
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit.tasks - not created here)
```

### Source Code (repository root)

```text
examples/
├── azure_client_integration_test.go    # Existing: raw client tests
└── estimate_cost_integration_test.go   # NEW: full pipeline tests
```

**Structure Decision**: New integration tests go in the existing `examples/`
package. This follows the established project pattern
(`examples/azure_client_integration_test.go`) and requires zero CI workflow
changes — the `integration` job in `.github/workflows/test.yml:53` already
runs `go test -v -tags=integration -timeout=2m ./examples/...`.

## Implementation Details

### New File: `examples/estimate_cost_integration_test.go`

**Build tag**: `//go:build integration`
**Package**: `examples`

#### Helper Functions (unexported, file-scoped)

**`skipIfDisabled(t *testing.T)`**: Checks `os.Getenv("SKIP_INTEGRATION")`
and calls `t.Skip()` if set to `"true"`.

**`newTestCalculator(t *testing.T) (*pricing.Calculator, *azureclient.CachedClient)`**:
Constructs the full pipeline:

1. `config := azureclient.DefaultConfig()` — base URL, retry policy, timeouts
2. `config.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()`
3. `client, err := azureclient.NewClient(config)` — HTTP client
4. `cacheConfig := azureclient.DefaultCacheConfig()` — 1000 entries, 24h TTL
5. `cacheConfig.Logger = config.Logger`
6. `cachedClient, err := azureclient.NewCachedClient(client, cacheConfig)`
7. `calc := pricing.NewCalculator(config.Logger, cachedClient)` — variadic

Returns both `calc` and `cachedClient` so tests can inspect
`cachedClient.Stats()` (`CacheStats.Hits.Load()`, `.Misses.Load()`).
Registers `t.Cleanup(func() { cachedClient.Close() })` to stop the
internal eviction goroutine. Each test should create a
`context.WithTimeout(ctx, 30*time.Second)` for EstimateCost calls.

**`assertInRange(t *testing.T, actual, reference, tolerance float64)`**:
Asserts `actual` is within `[reference*(1-tolerance), reference*(1+tolerance)]`.
Logs expected range and actual value on failure.

**`rateLimitDelay()`**: `time.Sleep(12 * time.Second)` — called after each
test that makes a live API call.

#### Reference Price Constants

```go
// Reference prices last verified: 2026-03-30
const (
    refB1sHourly              = 0.0104  // Standard_B1s, eastus
    refD2sv3Hourly            = 0.096   // Standard_D2s_v3, eastus
    refDisk128StandardMonthly = 5.89    // 128GB Standard_LRS, eastus (S10 tier)
    refDisk256PremiumMonthly  = 38.00   // 256GB Premium_SSD_LRS, eastus (P15 tier)
    priceTolerance            = 0.25    // ±25%
)
```

#### Test Functions (sequential execution order)

1. `TestEstimateCost_VM_StandardB1s` — API call, P1,
   FR-003/FR-004
2. `TestEstimateCost_VM_StandardD2sv3` — API call, P1,
   FR-003/FR-004
3. `TestEstimateCost_VM_CacheHit` — cached*, P1, FR-006
4. `TestEstimateCost_Disk_StandardLRS` — API call, P2,
   FR-005
5. `TestEstimateCost_Disk_PremiumSSD` — API call, P2,
   FR-005
6. `TestEstimateCost_Error_InvalidSKU` — API call, P2,
   FR-009
7. `TestEstimateCost_Error_MissingAttributes` — no API,
   P2, FR-009
8. `TestEstimateCost_SkipIntegration` — no API, P3,
   FR-008

*Cache hit test reuses cached results from test #1 — makes API call only if
cache is cold (first call), then verifies `Stats().Hits` on second call.

#### Request Construction Patterns

**VM request** (`calculator.go:46-85`):

```go
attrs, _ := structpb.NewStruct(map[string]any{
    "location": "eastus",
    "vmSize":   "Standard_B1s",
})
resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
    ResourceType: "azure:compute/virtualMachine:VirtualMachine",
    Attributes:   attrs,
})
```

**Disk request** (`calculator.go:305-357`):

```go
attrs, _ := structpb.NewStruct(map[string]any{
    "location":  "eastus",
    "disk_type": "Standard_LRS",
    "size_gb":   128,
})
resp, err := calc.EstimateCost(ctx, &finfocusv1.EstimateCostRequest{
    ResourceType: "azure:storage/managedDisk:ManagedDisk",
    Attributes:   attrs,
})
```

**Error cases**:

- Invalid SKU: `vmSize: "Nonexistent_ZZZZZ"` → `codes.NotFound`
- Missing attrs: empty `Attributes` struct → `codes.InvalidArgument`

### Rate Limiting Strategy

Tests run sequentially (no `t.Parallel()`). Each test calling the live API
invokes `rateLimitDelay()` after its assertion. Cache-hit tests and error
tests that fail before reaching the API skip the delay.

**Estimated total time**: 4-5 API calls × 12s = 48-60s delays + execution ≈
90s total.

### Cache Verification Approach

Test `TestEstimateCost_VM_CacheHit`:

1. Create calculator (fresh `CachedClient`)
2. Call `EstimateCost(Standard_B1s, eastus)` — cache miss
3. Record `stats := cachedClient.Stats(); misses := stats.Misses.Load()`
4. Call `EstimateCost(Standard_B1s, eastus)` again — cache hit
5. Assert `stats.Hits.Load() > 0` (deterministic, not timing-based)
6. Assert `stats.Misses.Load() == misses` (no new misses)

### CI Integration

**No changes needed**. `.github/workflows/test.yml` already has:

```yaml
integration:
  runs-on: ubuntu-latest
  timeout-minutes: 5
  steps:
    - uses: actions/checkout@v6
    - uses: actions/setup-go@v6
      with: { go-version: "1.25", cache: true }
    - run: go test -v -tags=integration -timeout=2m ./examples/...
```

The 2-minute timeout may need to be increased to 5 minutes if more tests are
added in the future. For now, the planned ~90s execution fits within 2m.

### Imports Required

```go
import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/rs/zerolog"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/structpb"

    finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
    "github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
    "github.com/rshade/finfocus-plugin-azure-public/internal/pricing"
)
```
