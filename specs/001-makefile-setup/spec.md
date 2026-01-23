# Feature Specification: Makefile Build System

**Feature Branch**: `001-makefile-setup`
**Created**: 2026-01-22
**Status**: Draft
**Input**: User description: "Setup Makefile with build, test, lint targets"

## Clarifications

### Session 2026-01-22

- Q: When a developer runs `make` without any arguments, what should happen? → A: Display help text (list available targets with descriptions)
- Q: Should the Makefile validate tool prerequisites before executing targets? → A: Follow AWS plugin pattern - provide `make ensure` target to install tools, but no automatic validation (let native tool errors bubble up)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Build Versioned Binary (Priority: P1)

As a developer, I want to build the plugin binary with embedded version information so that I can identify which version is running in production.

**Why this priority**: Without a working build system, no development work can be validated or deployed. This is the foundational capability that everything else depends on.

**Independent Test**: Can be fully tested by running `make build` and executing `./finfocus-plugin-azure-public --version` (or equivalent version check via RPC), which should display the calculated version string.

**Acceptance Scenarios**:

1. **Given** a clean working directory, **When** I run `make build`, **Then** a binary named `finfocus-plugin-azure-public` is created in the project root
2. **Given** the binary is built, **When** I execute the binary with version flag, **Then** it displays the version in format MAJOR.MINOR.PATCH-dev
3. **Given** git tags exist (e.g., v0.1.2), **When** I run `make build`, **Then** the version is calculated as 0.1.3-dev (next patch version)
4. **Given** no git tags exist, **When** I run `make build`, **Then** the version defaults to 0.0.1-dev

---

### User Story 2 - Run Test Suite (Priority: P2)

As a CI engineer, I want to run all unit tests with race detection enabled so that I can identify concurrency issues before they reach production.

**Why this priority**: Testing is critical for code quality but depends on having a working build system first. This enables continuous validation of code changes.

**Independent Test**: Can be fully tested by running `make test` which should execute the Go test suite with race detection and report results.

**Acceptance Scenarios**:

1. **Given** the project has test files, **When** I run `make test`, **Then** all tests are executed with the `-race` flag
2. **Given** all tests pass, **When** I run `make test`, **Then** the command exits with status code 0
3. **Given** any test fails, **When** I run `make test`, **Then** the command exits with non-zero status and displays failure details
4. **Given** verbose output is needed, **When** I run `make test`, **Then** the `-v` flag is used to show detailed test output

---

### User Story 3 - Lint Code (Priority: P2)

As a code reviewer, I want to run linting checks so that I can catch code quality issues before PR submission.

**Why this priority**: Linting ensures code quality and catches potential bugs early, but is dependent on the build system being in place. It's a quality gate rather than a functional requirement.

**Independent Test**: Can be fully tested by running `make lint` which should execute golangci-lint with appropriate timeout settings and report any issues found.

**Acceptance Scenarios**:

1. **Given** the project has Go code, **When** I run `make lint`, **Then** golangci-lint runs with a timeout of at least 10 minutes
2. **Given** code has no linting issues, **When** I run `make lint`, **Then** the command exits with status code 0
3. **Given** code has linting issues, **When** I run `make lint`, **Then** the command exits with non-zero status and displays all issues
4. **Given** lint takes longer than expected, **When** I run `make lint`, **Then** it does not timeout prematurely (supports long-running checks per CLAUDE.md)

---

### User Story 4 - Clean Build Artifacts (Priority: P3)

As a developer, I want to clean build artifacts so that I can ensure a fresh build when needed.

**Why this priority**: Cleaning is a maintenance operation that's useful but not critical for core development workflows. It's a convenience feature.

**Independent Test**: Can be fully tested by building the binary, running `make clean`, and verifying the binary is removed.

**Acceptance Scenarios**:

1. **Given** a binary exists in the project root, **When** I run `make clean`, **Then** the binary is deleted
2. **Given** no binary exists, **When** I run `make clean`, **Then** the command succeeds without error

---

### User Story 5 - Install Development Tools (Priority: P3)

As a new developer, I want to install required development tools so that I can start building and testing immediately.

**Why this priority**: Tool installation is a setup operation that's useful but not critical for core development workflows. Experienced developers may already have tools installed.

**Independent Test**: Can be fully tested by running `make ensure` which should install golangci-lint and report success.

**Acceptance Scenarios**:

1. **Given** golangci-lint is not installed, **When** I run `make ensure`, **Then** golangci-lint is installed successfully
2. **Given** development tools are already installed, **When** I run `make ensure`, **Then** the command succeeds without error

---

### User Story 6 - View Available Targets (Priority: P3)

As a new developer, I want to see available make targets so that I understand what commands are available.

**Why this priority**: Documentation is helpful for onboarding but not critical for core functionality. Developers can also read the Makefile directly.

**Independent Test**: Can be fully tested by running `make help` which should display a list of available targets with brief descriptions.

**Acceptance Scenarios**:

1. **Given** the Makefile is present, **When** I run `make help`, **Then** all available targets are listed with descriptions
2. **Given** I run `make` without arguments, **When** the default target is executed, **Then** it displays help text (same as `make help`)

---

### Edge Cases

- What happens when git is not available or the directory is not a git repository? (Version should default to 0.0.1-dev)
- What happens when golangci-lint is not installed? (Lint target fails with native tool error; user should run `make ensure` to install)
- What happens when `make build` is run twice without `make clean`? (Binary should be rebuilt/overwritten)
- What happens when the main package does not have a version variable to inject? (Build should succeed but version flag may not work)
- What happens when `make ensure` is run on a system without Go installed? (Should fail with clear error about Go being required)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Makefile MUST provide a `build` target that compiles the binary to `./finfocus-plugin-azure-public`
- **FR-002**: Build target MUST inject version information via ldflags using pattern `MAJOR.MINOR.NEXT_PATCH-dev`
- **FR-003**: Version calculation MUST be based on the latest git tag (e.g., v0.1.2 → 0.1.3-dev)
- **FR-004**: Version calculation MUST default to 0.0.1-dev when no git tags exist
- **FR-005**: Makefile MUST provide a `test` target that runs `go test -v -race ./...`
- **FR-006**: Makefile MUST provide a `lint` target that runs `golangci-lint run --timeout=10m ./...`
- **FR-007**: Makefile MUST provide a `clean` target that removes the binary artifact
- **FR-008**: Makefile MUST provide a `help` target that displays available targets with descriptions
- **FR-009**: Makefile default target (invoked by `make` without arguments) MUST display help text
- **FR-010**: Makefile MUST provide an `ensure` target that installs development dependencies (golangci-lint)
- **FR-011**: Build, test, and lint targets MUST NOT validate tool prerequisites (native tool errors bubble up)
- **FR-012**: LDFLAGS MUST use the format `-X main.version=$(DEV_VERSION)`
- **FR-013**: Build command MUST follow the pattern: `go build -ldflags "$(LDFLAGS)" -o finfocus-plugin-azure-public ./cmd/finfocus-plugin-azure-public`

### Key Entities

Not applicable for this build system feature (no data entities involved).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers can build the binary in under 30 seconds on typical development machines
- **SC-002**: Test suite executes in under 2 minutes for projects with up to 100 test cases
- **SC-003**: Linting completes within 10 minutes even for large codebases
- **SC-004**: 100% of developers can successfully run `make ensure` followed by `make build`, `make test`, and `make lint` without manual intervention
- **SC-005**: Version information is accurately embedded and retrievable from built binaries
- **SC-006**: CI pipeline can execute `make build && make test && make lint` as a single validation workflow
- **SC-007**: Developers receive clear, actionable error messages when required tools are missing (native tool error output)

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (N/A - this is a build system feature)
- [x] Error handling strategy is defined (Makefile targets return appropriate exit codes; clear error messages)
- [x] Code complexity is considered (Makefile scripts are simple shell commands with minimal complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (Manual validation by running each make target)
- [x] Performance test criteria specified (Build time <30s, test time <2min, lint time <10min)

### User Experience

- [x] Error messages are user-friendly and actionable (Standard make/go/golangci-lint error output)
- [x] Response time expectations defined (Build <30s, Test <2min, Lint <10min)
- [x] Observability requirements specified (Verbose test output via `-v` flag; lint results display all issues)

### Documentation

- [x] README.md updates identified (Should document available make targets and usage)
- [x] API documentation needs outlined (Not applicable - build system has no public API)
- [x] Examples/quickstart guide planned (Help target provides usage guidance)

### Performance & Reliability

- [x] Performance targets specified (Build <30s, Test <2min, Lint <10min)
- [x] Reliability requirements defined (Proper exit codes, idempotent operations)
- [x] Resource constraints considered (Lint timeout set to 10m per CLAUDE.md guidance)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data

## Assumptions

1. The main package is located at `./cmd/finfocus-plugin-azure-public/main.go`
2. The main package has a variable `version` that can be set via ldflags
3. Go 1.25.5+ is already installed (prerequisite for all make targets)
4. Git is available for version calculation
5. Development tools (golangci-lint) can be installed via `go install` commands
6. The project follows standard Go module structure
7. The binary name matches the module name: `finfocus-plugin-azure-public`
8. The Makefile already exists but may need validation/enhancement (per issue description)
9. The default make target (invoked by `make` without arguments) displays help text to guide new developers
10. Tool prerequisite approach follows AWS plugin pattern: provide `ensure` target, but let native tool errors bubble up in other targets

## Dependencies

- Go 1.25.5 or later (as specified in CLAUDE.md)
- golangci-lint (for lint target)
- Git (for version calculation)
- Standard Unix make utility

## Out of Scope

- Cross-compilation for multiple platforms (Darwin, Windows, Linux)
- Release builds with release version numbers (only dev versions)
- Docker-based builds
- Continuous integration pipeline configuration
- Binary installation/distribution mechanisms
- Code generation or asset embedding
- Dependency vendoring
