# Tasks: Configure CI Pipeline (GitHub Actions)

**Input**: Design documents from `/specs/005-ci-pipeline/`
**Prerequisites**: plan.md, spec.md, research.md, quickstart.md

**Tests**: Manual verification via GitHub Actions UI (no automated tests for workflow files)

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

This feature creates infrastructure files at repository root:

- `.github/workflows/test.yml` - CI workflow file
- `README.md` - Badge update

---

## Phase 1: Setup (Directory Structure)

**Purpose**: Create required directory structure for GitHub Actions

- [x] T001 Create `.github/workflows/` directory structure

---

## Phase 2: User Story 1 & 2 - PR and Main Branch Validation (Priority: P1)

**Goal**: CI workflow triggers on PRs targeting main and pushes to main, running
tests, lint, and build.

**Independent Test**: Open a PR against main and verify workflow runs in GitHub
Actions UI; push to main and verify workflow triggers.

**Note**: US1 (PR Validation) and US2 (Main Branch Protection) are implemented
together as they share the same workflow file with different triggers.

### Implementation for User Stories 1 & 2

- [x] T002 [US1] [US2] Create workflow file `.github/workflows/test.yml` with:
  - Workflow name: "Test"
  - Triggers: `pull_request` (branches: main) and `push` (branches: main)
  - Permissions: `contents: read`
  - Job: `test` running on `ubuntu-latest`
- [x] T003 [US1] [US2] Add checkout step using `actions/checkout@v6`
- [x] T004 [US1] [US2] Add Go setup step using `actions/setup-go@v6` with:
  - `go-version: '1.25'`
  - `cache: true` for module caching
- [x] T005 [US1] [US2] Add test step running `make test`
- [x] T006 [US1] [US2] Add lint step using `golangci/golangci-lint-action@v6` with:
  - `version: v2.8.0`
  - `args: --timeout=10m` (matches Makefile timeout)
- [x] T007 [US1] [US2] Add build step running `make build`

**Checkpoint**: PR and push triggers functional; test, lint, build steps execute

---

## Phase 3: User Story 3 - Reviewer Confidence (Priority: P2)

**Goal**: CI status visible on PRs for reviewers to see before starting review.

**Independent Test**: Submit a PR, verify CI status indicator (pending/pass/fail)
visible on PR page; verify failure details show specific errors.

### Implementation for User Story 3

- [x] T008 [US3] Verify workflow outputs clear step names in GitHub Actions UI:
  - "Checkout code"
  - "Set up Go"
  - "Run tests"
  - "Run linter" (golangci-lint-action provides this)
  - "Build binary"

**Note**: GitHub Actions natively shows CI status on PRs. This story is satisfied
by having clear step names (T002-T007) and proper workflow structure.

**Checkpoint**: PR status indicators show clear, actionable information

---

## Phase 4: User Story 4 - Security Issue Detection (Priority: P2)

**Goal**: golangci-lint catches security-related issues and reports them in logs.

**Independent Test**: Introduce a lint violation (e.g., unused variable), push,
and verify CI fails with specific error details in workflow logs.

### Implementation for User Story 4

- [x] T009 [US4] Verify lint step uses project `.golangci-lint.yml` configuration
- [x] T010 [US4] Verify lint failures show file path, line number, and error message

**Note**: golangci-lint security detection is already configured in project's
`.golangci-lint.yml`. This story validates the integration works correctly.

**Checkpoint**: Lint violations produce clear, actionable error messages

---

## Phase 5: Polish & Documentation

**Purpose**: README badge and final validation

### Documentation Tasks

- [x] T011 [P] Add CI status badge to top of `README.md`:
  - Badge format: `[![Test](https://github.com/rshade/finfocus-plugin-azure-public/actions/workflows/test.yml/badge.svg)](https://github.com/rshade/finfocus-plugin-azure-public/actions/workflows/test.yml)`

### Validation Tasks

- [x] T012 [P] Validate workflow YAML syntax using GitHub Actions linter or push
- [x] T013 [P] Verify workflow completes within 10-minute target
- [x] T014 [P] Verify cached runs are faster than uncached (30% improvement target)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - create directory
- **Phase 2 (US1 & US2)**: Depends on Phase 1 - core workflow implementation
- **Phase 3 (US3)**: Depends on Phase 2 - validates UI presentation
- **Phase 4 (US4)**: Depends on Phase 2 - validates lint integration
- **Phase 5 (Polish)**: Depends on Phase 2 - documentation and validation

### User Story Dependencies

- **US1 & US2 (P1)**: Implemented together (same workflow file)
- **US3 (P2)**: Can validate after US1/US2 complete
- **US4 (P2)**: Can validate after US1/US2 complete
- **US3 and US4**: Can run in parallel (different validation concerns)

### Task Dependencies Within Phase 2

```text
T002 (workflow file) → T003 (checkout) → T004 (Go setup) → T005 (test)
                                                        → T006 (lint action)
                                                        → T007 (build)
```

Tasks must be added sequentially to the workflow file (same file constraint).

### Parallel Opportunities

- **Phase 5**: T011, T012, T013, T014 can all run in parallel (different files/validations)
- **Phase 3 & 4**: Can run in parallel after Phase 2 complete

---

## Implementation Strategy

### MVP First (User Stories 1 & 2)

1. Complete Phase 1: Create directory
2. Complete Phase 2: Implement workflow (T002-T007)
3. **STOP and VALIDATE**: Push branch, open PR, verify CI runs
4. If working: Proceed to Phase 3-5

### Incremental Delivery

1. Phase 1 + Phase 2 → Core CI functional (MVP)
2. Phase 3 → Verify reviewer experience
3. Phase 4 → Verify lint security detection
4. Phase 5 → Add badge, final validation

### Single Developer Flow

All tasks are sequential due to single workflow file:

1. T001 → T002 → T003 → T004 → T005 → T006 → T007
2. Push and validate
3. T008 → T009 → T010 (validation tasks)
4. T011 → T012 → T013 → T014 (polish, can parallel)

---

## Notes

- All workflow tasks (T002-T007) modify same file - must be sequential
- Manual testing required via GitHub Actions UI
- Workflow filename `test.yml` matches AWS plugin convention
- No automated tests for this feature (workflow files cannot be unit tested)
- Commit after completing Phase 2 to enable validation
