# Research: Makefile Build System

**Status**: Complete
**Date**: 2026-01-22

## Decisions

### Build Tool: GNU Make
- **Decision**: Use standard GNU Make.
- **Rationale**: Ubiquitous, standard in Go ecosystem, satisfies requirements without extra dependencies.
- **Alternatives**: Mage, Taskfile (rejected to minimize non-standard tooling).

### Versioning Strategy
- **Decision**: Use `git describe --tags` logic or fallback to `0.0.1-dev`.
- **Rationale**: Provides semantic versioning based on git tags, standard practice.

### Linting
- **Decision**: `golangci-lint`.
- **Rationale**: De facto standard for Go.
