# Implementation Plan: Managed Disk Cost Estimation

**Branch**: `021-disk-cost-estimation` | **Date**: 2026-03-05 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/021-disk-cost-estimation/spec.md`

## Summary

Extend `EstimateCost` RPC to support Azure Managed Disk cost estimation. Disks use tiered monthly pricing (not hourly like VMs), requiring resource-type routing in the calculator, a disk-type-to-SKU normalization layer, a static tier capacity table for ceiling-match size mapping, and filtering Azure API results by meter name to select the correct tier price.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: finfocus-spec v0.5.7 (pluginsdk), zerolog v1.34.0, google.golang.org/grpc, golang-lru/v2 (cache)
**Storage**: N/A — stateless plugin (in-memory LRU+TTL cache only)
**Testing**: `go test` with table-driven tests, `go test -race` for concurrency, integration tests in `examples/`
**Target Platform**: Linux server (gRPC plugin)
**Project Type**: Single Go module
**Performance Goals**: Cache hits <10ms (p99), cache misses <2s (p95)
**Constraints**: No authenticated Azure APIs, no persistent storage, no bulk pricing data embedding
**Scale/Scope**: 6 supported disk types, ~14 tiers per type, extends existing `internal/pricing` package

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Code Quality**: Plan includes linting (`make lint`), explicit error handling with gRPC status codes, complexity kept low by isolating disk logic in dedicated functions
- [x] **Testing**: TDD approach — tests written before implementation, >=80% coverage target, table-driven tests for disk type/tier variations, race detector for cached client usage
- [x] **User Experience**: Consistent error messages naming all missing fields, structured zerolog logging with disk-specific fields (`disk_type`, `size_gb`, `tier`), same response format as VM estimation
- [x] **Documentation**: Godoc comments for all new exported functions, CLAUDE.md updates for disk estimation section, README update for supported resource types
- [x] **Performance**: Reuses existing CachedClient (24h TTL, <10ms cache hits), single Azure API call per disk type + region returns all tiers, no additional API overhead vs VMs
- [x] **Architectural Constraints**: Uses only unauthenticated `prices.azure.com`, no persistent storage, read-only cost estimation, static tier sizes (not bulk pricing data)

## Project Structure

### Documentation (this feature)

```text
specs/021-disk-cost-estimation/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: API research and decisions
├── data-model.md        # Phase 1: Entity and data flow design
├── quickstart.md        # Phase 1: Usage examples
├── contracts/
│   └── estimate-cost-disk.md  # Phase 1: RPC contract for disk estimation
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/pricing/
├── calculator.go          # MODIFY: Add resource-type routing, disk estimation path
├── calculator_test.go     # MODIFY: Add disk estimation test cases
├── disk.go                # NEW: Disk type mapping, tier table, ceiling-match logic
├── disk_test.go           # NEW: Unit tests for disk utilities
├── mapper.go              # NO CHANGE: Already has storage/manageddisk mapping
├── mapper_test.go         # MODIFY: Add disk descriptor mapping tests
├── errors.go              # MODIFY: Add disk-specific error sentinels if needed
└── grpc_status.go         # NO CHANGE: Existing error mapping reused
```

**Structure Decision**: Extend existing `internal/pricing` package with a new `disk.go` file for disk-specific logic (type normalization, tier table, tier selection). Modify `calculator.go` to add resource-type routing. No new packages needed — this follows the established pattern where `calculator.go` handles RPC methods and delegates to domain logic.

## Complexity Tracking

No constitution violations. No complexity justifications needed.

## Post-Design Constitution Re-Check

- [x] **Code Quality**: Disk logic isolated in `disk.go` (<100 lines estimated). No function exceeds 15 cyclomatic complexity. Static tier table uses named constants.
- [x] **Testing**: Table-driven tests cover all 6 disk types, tier boundary conditions, missing field combinations, and error paths. Race detector applies to cached client calls (reused from VM tests).
- [x] **User Experience**: Error messages are consistent with VM pattern (`missing required field(s): region, disk_type, size_gb`). Logging includes disk-specific context fields.
- [x] **Documentation**: Godoc comments planned for all new exported symbols. CLAUDE.md disk estimation section outlined.
- [x] **Performance**: Single API query per disk type + region. Cached via existing CachedClient. Tier selection is O(n) over ~14 tiers — negligible.
- [x] **Architectural Constraints**: No authenticated APIs. No persistent storage. Static tier sizes (14 entries) are not "bulk pricing data." Read-only estimation.
