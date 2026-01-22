# Implementation Plan: Go Module and Dependency Initialization

**Branch**: `001-go-module-init` | **Date**: 2026-01-21 | **Spec**: [specs/001-go-module-init/spec.md](./spec.md)
**Input**: Feature specification from `specs/001-go-module-init/spec.md`

## Summary

Initialize the Go module `github.com/rshade/finfocus-plugin-azure-public` and declare core dependencies in `go.mod`. This establishes the project foundation, enabling subsequent development of the gRPC server and Azure client. It specifically targets `finfocus-spec` for plugin protocols, `go-retryablehttp` for resilient API calls, and `zerolog` for structured logging.

**Critical Implementation Detail**: A stub file `cmd/plugin/main.go` MUST be created. To definitively prevent `go mod tidy` from pruning dependencies, this file MUST **actually use** the imported packages (e.g., by declaring variables of their types like `var _ *retryablehttp.Client` or `var _ zerolog.Logger`), rather than relying solely on blank imports.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**:
- `github.com/rshade/finfocus-spec` v0.5.4 (Plugin SDK)
- `github.com/hashicorp/go-retryablehttp` v0.7.7 (HTTP Transport)
- `github.com/rs/zerolog` v1.33.0 (Logging)
- `google.golang.org/grpc` v1.50.0+ (RPC)
**Storage**: N/A (Stateless)
**Testing**: `go test` (standard toolchain), `go mod verify`
**Target Platform**: Linux (Container/Server)
**Project Type**: Single gRPC Plugin
**Performance Goals**: Dependency resolution < 2 minutes.
**Constraints**: Must NOT include Azure SDK Auth libraries.
**Scale/Scope**: ~10 lines of `go.mod` configuration.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes linting checks (`golangci-lint` to be added later, `go mod tidy` now), error handling strategy (standard Go toolchain errors).
- [x] **Testing**: Plan includes verification commands (`go mod verify`, `go list -m all`) which serve as acceptance tests.
- [x] **User Experience**: N/A for this foundational feature (no runtime UX yet, only developer UX).
- [x] **Documentation**: README instructions for setup are part of the plan.
- [x] **Performance**: N/A for runtime.
- [x] **Architectural Constraints**: Plan DOES NOT violate "Hard No's" (no auth libs, no storage, no mutation). Confirmed by `research.md`.

## Project Structure

### Documentation (this feature)

```text
specs/001-go-module-init/
├── plan.md              # This file
├── research.md          # Dependency version choices
├── data-model.md        # go.mod structure definition
├── quickstart.md        # Usage instructions for developers
└── checklists/requirements.md # Existing requirements
```

### Source Code (repository root)

```text
/mnt/c/GitHub/go/src/github.com/rshade/finfocus-plugin-azure-public/
├── go.mod               # The primary artifact
├── go.sum               # Checksums
├── Makefile             # (Future)
└── cmd/plugin/main.go   # (Skeleton entry point to verify imports)
```

**Structure Decision**: Standard Go project layout with `cmd/` for entry points and `internal/` for logic. `go.mod` sits at root.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |