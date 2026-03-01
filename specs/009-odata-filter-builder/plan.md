# Implementation Plan: OData Filter Query Builder

**Branch**: `009-odata-filter-builder` | **Date**: 2026-02-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/009-odata-filter-builder/spec.md`

## Summary

Implement a fluent, chainable `FilterBuilder` type in the `azureclient` package
that constructs OData-compatible `$filter` expressions for the Azure Retail
Prices API. The builder provides named methods for 6 common fields (region,
service, SKU, product, currency, pricing type) plus a generic `Field()` method,
supports both AND and OR logic with automatic parenthesization, includes a
default "Consumption" pricing type filter, and properly escapes special
characters. The existing `buildFilterQuery()` function will be refactored to use
the builder internally, maintaining backward compatibility with `GetPrices()`.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: None new — pure Go stdlib (`fmt`, `strings`, `sort`)
**Storage**: N/A — pure data transformation (string builder), no I/O
**Testing**: `go test` with table-driven tests, `go test -race` (no concurrency
in builder, but run for CI compliance)
**Target Platform**: Linux server (gRPC plugin)
**Project Type**: Single Go module
**Performance Goals**: Filter construction <1ms for up to 10 criteria (SC-001)
**Constraints**: No external dependencies, no I/O, no side effects
**Scale/Scope**: ~200 lines implementation + ~300 lines tests (2 new files)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: All code passes `golangci-lint`. Error handling: builder
  silently omits invalid inputs (empty/whitespace) per spec FR-009. Cyclomatic
  complexity of `Build()` estimated at ~8 (well under 15 limit). New file
  `filter.go` estimated at ~200 lines (under 300 guideline).
- [x] **Testing**: TDD workflow — tests written before implementation. Coverage
  target ≥80% with table-driven tests covering: single-field, multi-field,
  empty, whitespace, special characters, AND-only, OR-only, mixed AND/OR,
  default type, type override, generic fields, deterministic ordering. Race
  detector included in `make test`.
- [x] **User Experience**: No plugin lifecycle impact (pure data transformation).
  No logging needed (stateless string builder). Error handling: invalid inputs
  silently omitted per spec, producing valid output in all cases.
- [x] **Documentation**: All exported types, functions, and methods will have
  godoc comments. CLAUDE.md updated with FilterBuilder usage. Docstring coverage
  ≥80% maintained. Quickstart guide produced as `quickstart.md`.
- [x] **Performance**: Filter construction is O(n log n) where n = number of
  conditions (dominated by sort for determinism). SC-001 target: <1ms for 10
  criteria. No retry/timeout/connection concerns (pure computation).
- [x] **Architectural Constraints**: Pure string transformation. Does NOT use
  authenticated Azure APIs, persistent storage, infrastructure mutation, or bulk
  data embedding.

## Project Structure

### Documentation (this feature)

```text
specs/009-odata-filter-builder/
├── plan.md              # This file
├── research.md          # Phase 0: OData syntax, API fields, design decisions
├── data-model.md        # Phase 1: Entity model (FilterCondition, FilterBuilder)
├── quickstart.md        # Phase 1: Usage examples
├── contracts/           # Phase 1: Go API contract
│   └── filter_builder.go
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/azureclient/
├── filter.go            # NEW: FilterBuilder type, FilterCondition, constructors
├── filter_test.go       # NEW: Table-driven tests for all filter scenarios
├── client.go            # MODIFIED: refactor buildFilterQuery() to use FilterBuilder
├── client_test.go       # MODIFIED: update filter tests for default type
├── types.go             # UNCHANGED: PriceQuery struct retained for backward compat
├── errors.go            # UNCHANGED
├── retry.go             # UNCHANGED
└── logger.go            # UNCHANGED
```

**Structure Decision**: Two new files (`filter.go`, `filter_test.go`) added to
the existing `internal/azureclient/` package. The builder is a pure
data-transformation component with no dependencies on the HTTP client, retry
logic, or logging — keeping it in a separate file maintains single
responsibility while sharing the package namespace for seamless integration.

## Complexity Tracking

No constitution violations. All constraints satisfied within standard limits.

| Aspect       | Assessment                           |
|--------------|--------------------------------------|
| Complexity   | `Build()` ~8, other methods ~2       |
| File size    | `filter.go` ~200, tests ~300 lines   |
| Dependencies | None (stdlib only)                   |
| API surface  | 1 type, 1 value, 7 ctors, 9 methods  |
