# Implementation Plan: GetProjectedCost RPC

**Branch**: `023-projected-cost-rpc` | **Date**: 2026-04-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/023-projected-cost-rpc/spec.md`

## Summary

Promote `GetProjectedCost` from a partial stub to a production-ready RPC by
replacing the boolean-return validation in `projectedQueryFromRequest()` with
`MapDescriptorToQuery()` (sentinel errors → gRPC codes), adding structured
zerolog logging at all decision points, and populating `billing_detail` and
`pricing_category` in the response. Only the VM cost path is fully
implemented; disk/blob pass validation but return Unimplemented.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: finfocus-spec v0.5.7 (pluginsdk), zerolog v1.34.0, google.golang.org/grpc, golang-lru/v2 (cache)
**Storage**: N/A — stateless plugin (in-memory LRU+TTL cache only)
**Testing**: Go standard `testing` package, table-driven tests, `httptest` for mock servers
**Target Platform**: Linux container (gRPC server)
**Project Type**: Single Go project
**Performance Goals**: Cache hit <10ms p99, cache miss <2s p95
**Constraints**: No authenticated Azure APIs, no persistent storage, no infrastructure mutation
**Scale/Scope**: ~150 lines of production code changes, ~200 lines of test additions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan uses existing `MapDescriptorToQuery()` for validation (no new complexity), `MapToGRPCStatus()` for error mapping, structured logging via zerolog. Linting via `make lint`.
- [x] **Testing**: Plan follows TDD — tests written first for all error paths and success path. ≥80% coverage target. No concurrent code changes (cache layer is pre-existing and tested).
- [x] **User Experience**: Error messages include specific field names. All gRPC status codes are consistent with `EstimateCost` pattern. Structured logging at Info/Warn/Error levels.
- [x] **Documentation**: Godoc comments updated for refactored functions. CLAUDE.md updated with GetProjectedCost section. README updates for supported resource types.
- [x] **Performance**: No new performance concerns — reuses existing cached client and mapper. Response times bounded by cache TTL (24h default).
- [x] **Architectural Constraints**: DOES NOT introduce authenticated APIs, persistent storage, infrastructure mutation, or bulk data embedding. Uses existing unauthenticated Azure Retail Prices API client.

## Project Structure

### Documentation (this feature)

```text
specs/023-projected-cost-rpc/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 research findings
├── data-model.md        # Phase 1 entity model
├── quickstart.md        # Phase 1 usage guide
├── contracts/
│   └── projected-cost-contract.md  # Behavioral contract
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
internal/pricing/
├── calculator.go        # GetProjectedCost() refactored + projectedQueryFromRequest() removed
├── calculator_test.go   # New test cases + existing test fixture update
├── mapper.go            # No changes (MapDescriptorToQuery already sufficient)
├── errors.go            # No changes (MapToGRPCStatus already maps all sentinels)
└── ...                  # Other files unchanged
```

**Structure Decision**: All changes are within the existing `internal/pricing`
package. No new files or packages needed. The refactoring simplifies the
code by removing `projectedQueryFromRequest()` and replacing it with the
existing `MapDescriptorToQuery()`.

## Implementation Design

### Phase 2: Task Decomposition

#### Task 1: Write Tests for Validation Error Paths (TDD Red)

Write failing tests for all validation scenarios before changing production
code. Table-driven test covering:

- Nil request → InvalidArgument
- Nil resource descriptor → InvalidArgument
- Wrong provider ("gcp") → Unimplemented (via ErrUnsupportedResourceType)
- Unsupported resource type ("network/LoadBalancer") → Unimplemented
- Missing region → InvalidArgument with "region" in message
- Missing SKU → InvalidArgument with "sku" in message
- Missing both region and SKU → InvalidArgument with both in message
- Nil cachedClient → Unimplemented
- Valid VM request → success with all response fields

**File**: `internal/pricing/calculator_test.go`
**Depends on**: Nothing

#### Task 2: Refactor GetProjectedCost to Use MapDescriptorToQuery (TDD Green)

Replace `projectedQueryFromRequest()` usage with direct
`MapDescriptorToQuery()` call. Handle the refactoring in this order:

1. Extract resource descriptor from request (nil check)
2. Call `MapDescriptorToQuery(req.GetResource())` for validation
3. Map returned errors via `MapToGRPCStatus()`
4. Guard on nil cachedClient
5. Route by resource type: VM computes cost, disk/blob return Unimplemented
6. Call `cachedClient.GetPrices()` → `unitPriceAndCurrency()`
7. Build response with `WithProjectedCostDetails()`,
   `WithProjectedCostPricingCategory()`, `WithProjectedCostExpiresAt()`

**File**: `internal/pricing/calculator.go`
**Depends on**: Task 1 (tests exist to validate)

#### Task 3: Add Structured Logging

Add zerolog structured logging at all decision points, matching the
`EstimateCost` pattern:

- Request entry: Info with region, sku, resource_type, provider
- Validation failure: Warn with result_status=error
- cachedClient nil: Warn with result_status=error
- API/cache failure: Error with result_status=error
- Success: Info with cost_monthly, currency, unit_price, result_status=success

**Files**: `internal/pricing/calculator_test.go` (TDD test), `internal/pricing/calculator.go` (implementation)
**Depends on**: Task 2 (refactored method exists). TDD: write logging test first (capturing zerolog output), then implement logging.

#### Task 4: Update Existing Test Fixtures

Update `TestGetProjectedCostSetsExpiresAtFromCache` to use correct
resource type format: `"compute/VirtualMachine"` instead of
`"azure:compute/virtualMachine:VirtualMachine"`.

Also rewrite `TestProjectedCostSupported` to use Azure provider and
resource type (`provider="azure"`, `resource_type="compute/VirtualMachine"`,
`region="eastus"`, `sku="Standard_B1s"`) instead of the current AWS
placeholder data, and remove the `t.Skip()` call.

**File**: `internal/pricing/calculator_test.go`
**Depends on**: Task 2 (refactored method handles resource type)

#### Task 5: Remove projectedQueryFromRequest Function

After all tests pass with the new implementation, remove the now-unused
`projectedQueryFromRequest()` function. Verify no other callers exist.

**File**: `internal/pricing/calculator.go`
**Depends on**: Tasks 2, 4 (all callers migrated)

#### Task 6: Update Documentation

- Update CLAUDE.md with GetProjectedCost section (response fields,
  supported resource types, error codes)
- Verify godoc comments on refactored functions
- Run `make lint` to validate

**Files**: `CLAUDE.md`, `internal/pricing/calculator.go`
**Depends on**: Task 5 (implementation finalized)

#### Task 7: Quality Gates

- `make build` succeeds
- `make test` passes (all tests including race detector)
- `make lint` passes
- Verify docstring coverage ≥80%

**Depends on**: All previous tasks

### Dependency Graph

```text
Task 1 (Tests) ──→ Task 2 (Refactor) ──→ Task 3 (Logging) ──→ Task 5 (Cleanup)
                         │                                          │
                         └──→ Task 4 (Fix fixtures) ───────────────┘
                                                                    │
                                                              Task 6 (Docs)
                                                                    │
                                                              Task 7 (Gates)
```

## Key Design Decisions

### D1: Remove projectedQueryFromRequest Instead of Wrapping

The function currently duplicates validation logic from `MapDescriptorToQuery()`.
Rather than making it a thin wrapper, remove it entirely and call the mapper
directly. This eliminates a layer of indirection and a potential source of
divergence.

### D2: Resource Type Routing After Validation

After `MapDescriptorToQuery()` succeeds, check the resolved service name:
- `"Virtual Machines"` → compute cost (hourly × 730)
- `"Managed Disks"` / `"Storage"` → return Unimplemented with descriptive message

This ensures disk/blob requests get clear feedback ("storage/ManagedDisk cost
projection not yet implemented") rather than silent failures.

### D3: billing_detail Format

Use the format from the issue example:
`"Azure Retail Prices API: Standard_B1s in eastus at $0.0104/hr * 730 hrs/mo"`

This provides: data source, SKU, region, unit rate, and calculation method.

## Complexity Tracking

No constitution violations. No complexity justifications needed.
