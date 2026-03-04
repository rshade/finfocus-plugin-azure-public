# Implementation Plan: ResourceDescriptor to Azure Filter Mapping

**Branch**: `016-descriptor-filter-mapping` |
**Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature spec from
`/specs/016-descriptor-filter-mapping/spec.md`

## Summary

Map `finfocusv1.ResourceDescriptor` fields (provider, resource
type, SKU, region, tags) to `azureclient.PriceQuery` structs for
Azure Retail Prices API lookups. Pure data transformation with
validation, tag fallback, case-insensitive resource type matching,
and clear error reporting for missing fields. No external calls,
no auth, no storage.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: finfocus-spec v0.5.4
(`finfocusv1.ResourceDescriptor`), internal `azureclient`
(PriceQuery, FilterBuilder)
**Storage**: N/A — pure data transformation, no I/O
**Testing**: `go test` with table-driven tests, race detector
(`go test -race`)
**Target Platform**: Linux gRPC plugin (called by Calculator
gRPC handler)
**Project Type**: Single Go project
**Performance Goals**: <1ms per mapping (pure in-memory struct
translation)
**Constraints**: No authenticated Azure APIs, no persistent
storage, no infrastructure mutation
**Scale/Scope**: 3 resource types initially (VM, ManagedDisk,
BlobStorage), extensible registry

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after
Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes linting checks
  (`make lint`), explicit error handling (multi-field
  validation errors), cyclomatic complexity well under 15
  (simple switch/map lookup + validation)
- [x] **Testing**: Plan includes TDD workflow (tests before
  implementation), >=80% coverage target, no concurrent code
  so race detector is N/A but will still run
- [x] **User Experience**: Mapping is internal to plugin;
  errors surface through gRPC status codes via existing
  `MapToGRPCStatus`. Structured zerolog logging for mapping
  failures. No lifecycle changes needed.
- [x] **Documentation**: Plan includes godoc comments for all
  exported types/functions, CLAUDE.md update for new mapping
  capability, docstring coverage >=80%
- [x] **Performance**: <1ms target for pure data
  transformation. No external calls, no retry logic, no
  connection pooling needed. Zero resource constraints.
- [x] **Architectural Constraints**: Plan DOES NOT violate
  any "Hard No's" — no authenticated APIs, no persistent
  storage, no infrastructure mutation, no bulk data
  embedding. Pure in-memory mapping only.

## Project Structure

### Documentation (this feature)

```text
specs/016-descriptor-filter-mapping/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── mapper.go        # Go interface/function signatures
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── pricing/
│   ├── calculator.go      # MODIFY: wire mapper into Supports()
│   ├── calculator_test.go # MODIFY: Supports() tests
│   ├── mapper.go          # NEW: MapDescriptorToQuery, registry
│   ├── mapper_test.go     # NEW: table-driven mapping tests
│   ├── errors.go          # EXISTING: add mapping error vars
│   ├── errors_test.go     # MODIFY: tests for new error vars
│   ├── data.go            # UNCHANGED
│   └── data_test.go       # UNCHANGED
└── azureclient/           # UNCHANGED (consumed as-is)
    ├── types.go           # PriceQuery (target struct)
    └── filter.go          # FilterBuilder (downstream)
```

**Structure Decision**: New mapping logic lives in
`internal/pricing/mapper.go` within the existing `pricing`
package. This avoids unnecessary package proliferation — the
mapper is a direct collaborator of the Calculator and operates
in the same domain. The `azureclient` package is consumed as-is
(PriceQuery is the output target).

## Complexity Tracking

No constitution violations. All requirements fit within
existing architectural constraints. No complexity
justifications needed.
