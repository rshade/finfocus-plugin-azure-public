# Research: Configure CI Pipeline (GitHub Actions)

**Feature**: 005-ci-pipeline
**Date**: 2026-02-03

## Research Tasks

### 1. GitHub Actions Best Practices for Go Projects

**Decision**: Use official actions (checkout@v6, setup-go@v6) with Go module caching enabled

**Rationale**:

- Official actions are well-maintained and receive security updates
- actions/setup-go@v6 has built-in caching support via `cache: true`
- v6 versions are current stable releases (as of 2026)

**Alternatives Considered**:

- Custom Docker image with Go pre-installed: Rejected - adds maintenance burden
- Third-party Go actions: Rejected - less reliable than official actions
- No caching: Rejected - significantly slower builds (2-3x longer)

### 2. golangci-lint Installation Method

**Decision**: Use `golangci/golangci-lint-action@v6` with version pinned to v2.8.0

**Rationale**:

- Official GitHub Action maintained by golangci-lint team
- Handles caching of golangci-lint binary automatically
- Better integration with GitHub Actions (annotations, problem matchers)
- Version pinning ensures reproducible builds across CI runs
- Simpler workflow configuration (single step instead of install + run)

**Alternatives Considered**:

- curl install script: More manual, requires separate install and run steps
- `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`: Slower
- Pre-installed in runner: Not available by default on ubuntu-latest

### 3. Workflow Trigger Configuration

**Decision**: Trigger on `pull_request` (targeting main) and `push` (to main)

**Rationale**:

- `pull_request` validates PRs before merge
- `push` to main validates direct commits (merged PRs, hotfixes)
- Both triggers use same job definition (DRY)

**Alternatives Considered**:

- Separate workflows for PR and push: Rejected - duplication, harder to maintain
- Include other branches: Rejected - scope is main branch only per requirements
- workflow_dispatch: Not needed - CI is automatic, not manual

### 4. Caching Strategy

**Decision**: Use actions/setup-go@v6 built-in caching (`cache: true`)

**Rationale**:

- Automatically caches `~/go/pkg/mod` (module cache)
- Uses go.sum as cache key for automatic invalidation
- No additional configuration needed

**Alternatives Considered**:

- Manual actions/cache@v4: More control but unnecessary complexity
- No caching: Rejected - violates 30% performance improvement requirement
- Cache build artifacts: Not needed - Go builds are fast, modules are the bottleneck

### 5. Job Structure

**Decision**: Single job with sequential steps (test -> lint -> build)

**Rationale**:

- Simpler workflow file
- Fail-fast behavior (default) stops on first failure
- All steps use same Go environment
- No need for parallel jobs (small codebase)

**Alternatives Considered**:

- Separate jobs for test/lint/build: More parallelism but:
  - Requires matrix or multiple job definitions
  - Each job pays checkout + setup overhead
  - Overkill for small codebase
- Build first then test: Rejected - tests should run early to fail fast

### 6. Permissions Configuration

**Decision**: Minimal permissions with `contents: read`

**Rationale**:

- Security best practice - principle of least privilege
- CI only reads code, doesn't need write access
- Aligns with FR-009 requirement

**Alternatives Considered**:

- Default permissions: Rejected - overly broad, security risk
- No explicit permissions: Rejected - unclear intent

### 7. Timeout Configuration

**Decision**: 10-minute timeout for lint step (via Makefile)

**Rationale**:

- golangci-lint can be slow on first run or large codebases
- Makefile already has `--timeout=10m` configured
- Consistent with CLAUDE.md guidance about lint timeouts

**Alternatives Considered**:

- 5-minute timeout: May fail on cold cache
- No timeout: Risk of hanging indefinitely
- Per-step timeout in workflow: Redundant with Makefile timeout

## Summary

All technical decisions align with:

- GitHub Actions best practices
- Project constitution requirements
- Existing Makefile targets
- Security principles (minimal permissions)

No NEEDS CLARIFICATION items remain. Ready for Phase 1 design.
