# Implementation Plan: Makefile Build System

**Branch**: `001-makefile-setup` | **Date**: 2026-01-22 | **Spec**: [specs/001-makefile-setup/spec.md](spec.md)
**Input**: Feature specification from `/specs/001-makefile-setup/spec.md`

## Summary

Implement a comprehensive `Makefile` to standardize the development lifecycle. Key capabilities include:
1.  **Build**: Compile the binary with version injection via LDFLAGS (`-X main.version=...`).
2.  **Test**: Run unit tests with race detection enabled (`-race`).
3.  **Lint**: execute `golangci-lint` with a 10-minute timeout.
4.  **Utility**: Provide `clean`, `ensure` (dependencies), and `help` targets.

This aligns with the "Hard No's" by avoiding any runtime architectural changes; it purely facilitates the build process.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: `golangci-lint` (development tool), `git` (versioning)
**Storage**: N/A
**Testing**: `go test` with `-race` flag
**Target Platform**: Linux (primary), macOS, Windows (via WSL/Git Bash)
**Project Type**: Single (Go Plugin)
**Performance Goals**: Build <30s, Test <2m, Lint <10m
**Constraints**: Must work in standard shell environments
**Scale/Scope**: Repository-wide build configuration

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes `lint` target using `golangci-lint` with strict timeout.
- [x] **Testing**: Plan includes `test` target with `-race` detection (Constitution II).
- [x] **User Experience**: `make help` and default target provide clear guidance; `make ensure` helps onboarding.
- [x] **Documentation**: `quickstart.md` will document make usage; `make help` provides self-documentation.
- [x] **Performance**: Targets are optimized for speed; `clean` ensures artifact removal.
- [x] **Architectural Constraints**: Purely a build tool; violates no "Hard No's" (no Azure Auth, no DB, etc.).

## Project Structure

### Documentation (this feature)

```text
specs/001-makefile-setup/
├── plan.md              # This file
├── research.md          # Phase 0 output (skipped - low complexity)
├── data-model.md        # Phase 1 output (N/A)
├── quickstart.md        # Phase 1 output (Make usage guide)
├── contracts/           # Phase 1 output (N/A)
└── tasks.md             # Phase 2 output
```

### Source Code

```text
/
├── Makefile             # New file: The build definition
└── .golangci.yml        # Existing or New: Lint configuration
```

**Structure Decision**: Single `Makefile` in root.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | | |