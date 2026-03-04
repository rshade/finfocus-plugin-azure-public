# Implementation Plan: VM Cost Estimation (EstimateCost RPC)

**Branch**: `020-vm-cost-estimation` | **Date**: 2026-03-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/020-vm-cost-estimation/spec.md`

## Summary

Complete the EstimateCost RPC implementation to return accurate monthly VM cost
estimates from Azure Retail Prices API data. The happy path is already
functional — this plan addresses gaps in validation, error handling, structured
logging, and PricingCategory population. The proto response only supports
`cost_monthly`, `currency`, `pricing_category`, and `spot_interruption_risk_score`;
hourly/yearly/breakdown fields are not available in the current spec.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: finfocus-spec v0.5.7 (pluginsdk), zerolog v1.34.0,
hashicorp/golang-lru/v2 (cache), hashicorp/go-retryablehttp (HTTP client)
**Storage**: N/A — in-memory LRU+TTL cache only (stateless constraint)
**Testing**: `go test` with table-driven tests, `-race` detector, `make lint`
**Target Platform**: Linux server (gRPC plugin)
**Project Type**: Single Go module
**Performance Goals**: Cache hit <10ms (p99), cache miss <2s (p95)
**Constraints**: No authenticated Azure APIs, no persistent storage, no
infrastructure mutation, no bulk data embedding
**Scale/Scope**: Single RPC method enhancement (~100 LOC changes)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

<!-- markdownlint-disable MD013 -->

- [x] **Code Quality**: Plan includes linting checks (`make lint`), explicit error handling with field-specific InvalidArgument errors, and no complexity increase (changes are in existing functions <15 cyclomatic complexity)
- [x] **Testing**: Plan includes TDD workflow (tests before implementation), ≥80% coverage target for estimation logic, race detector for cached client tests
- [x] **User Experience**: No lifecycle changes needed (port announcement, health checks, graceful shutdown unchanged). Adds structured logging with region/sku/result_status fields. Improves error messages from generic "Unimplemented" to specific InvalidArgument with field names
- [x] **Documentation**: Plan includes godoc comment updates for modified functions, CLAUDE.md update for EstimateCost usage, docstring coverage ≥80% maintained
- [x] **Performance**: No performance changes — existing cache infrastructure (24h TTL, LRU eviction) and retry logic (3 retries, exponential backoff) remain unchanged. Response time targets unchanged
- [x] **Architectural Constraints**: Plan DOES NOT violate "Hard No's" — uses only unauthenticated Azure Retail Prices API, no persistent storage, no infrastructure mutation, no bulk data embedding

<!-- markdownlint-enable MD013 -->

## Project Structure

### Documentation (this feature)

```text
specs/020-vm-cost-estimation/
├── plan.md              # This file
├── research.md          # Phase 0 output — research findings
├── data-model.md        # Phase 1 output — entity definitions and data flow
├── quickstart.md        # Phase 1 output — usage guide
├── contracts/           # Phase 1 output — proto contract reference
│   └── estimate_cost.proto
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/pricing/
├── calculator.go        # EstimateCost method (MODIFY)
├── calculator_test.go   # EstimateCost tests (MODIFY)
├── errors.go            # Error sentinels and gRPC mapping
├── mapper.go            # ResourceDescriptor mapper
└── mapper_test.go       # Mapper tests

internal/azureclient/
├── cache.go             # CachedClient (NO CHANGE)
├── client.go            # HTTP client (NO CHANGE)
├── types.go             # PriceQuery, PriceItem (NO CHANGE)
└── errors.go            # Sentinel errors (NO CHANGE)

internal/estimation/
└── cost.go              # HourlyToMonthly, HourlyToYearly
```

**Structure Decision**: Existing Go module structure. All changes are in
`internal/pricing/calculator.go` and `internal/pricing/calculator_test.go`.
No new files or packages needed.

## Implementation Changes

### 1. Resource Type Validation (FR-009)

**File**: `internal/pricing/calculator.go` — `EstimateCost` method

Add validation before `estimateQueryFromRequest`:

- If `resource_type` is non-empty and does not contain `compute/virtualMachine`
  (case-insensitive substring match via `strings.Contains` +
  `strings.ToLower`), return `codes.Unimplemented`. This handles all known
  formats: `compute/virtualMachine`, `compute/VirtualMachine`,
  `azure:compute/virtualMachine:VirtualMachine`, etc.
- Empty `resource_type` is allowed for backward compatibility

### 2. Field-Specific Error Messages (FR-007)

**File**: `internal/pricing/calculator.go` — `estimateQueryFromRequest`

Change return type from `(PriceQuery, bool)` to `(PriceQuery, error)`:

- Return `nil` error on success
- Return `InvalidArgument` error listing missing fields: "region", "sku", or
  both when `ArmRegionName` or `ArmSkuName` are empty

### 3. PricingCategory Population (R-003)

**File**: `internal/pricing/calculator.go` — `EstimateCost` method

Add `pluginsdk.WithPricingCategory(pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD)`
to the response builder. Azure Consumption pricing = Standard/on-demand.

### 4. Structured Logging (FR-012)

**File**: `internal/pricing/calculator.go` — `EstimateCost` method

Enhance logging:

- Request entry: log `region`, `sku`, `resource_type` fields
- Success: log `cost_monthly`, `currency`, `result_status=success`
- Cache hit: log `result_status=cache_hit` (if distinguishable)
- Error: log `result_status=error` with error details

### 5. Test Coverage

**File**: `internal/pricing/calculator_test.go`

New test cases (table-driven):

- `TestEstimateCost_UnsupportedResourceType_ReturnsUnimplemented`
- `TestEstimateCost_MissingRegion_ReturnsInvalidArgument`
- `TestEstimateCost_MissingSKU_ReturnsInvalidArgument`
- `TestEstimateCost_MissingBothFields_ReturnsInvalidArgument`
- `TestEstimateCost_EmptyResourceType_Succeeds` (backward compat)
- `TestEstimateCost_ValidRequest_ReturnsPricingCategoryStandard`
- Verify existing `TestEstimateCostUsesCachedClient` still passes

## Complexity Tracking

No constitution violations. All changes are incremental improvements to an
existing method with no new abstractions or architectural changes.
