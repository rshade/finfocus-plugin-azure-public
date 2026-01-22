# Tasks: Go Module and Dependency Initialization

**Feature**: `001-go-module-init`
**Status**: Ready for Implementation

## Summary

This feature initializes the Go module and its core dependencies. It ensures that `go.mod` and `go.sum` are correctly populated and that all required libraries are available for subsequent development phases.

## Phase 1: Setup

- [x] T001 Initialize Go module with `github.com/rshade/finfocus-plugin-azure-public` in root directory
- [x] T002 Set Go version to `1.25.5` in `go.mod`

## Phase 2: Foundational

- [x] T003 [P] Create directory structure `cmd/plugin` and `internal/pricing`
- [x] T004 Create skeleton `cmd/plugin/main.go` to hold dependencies

## Phase 3: User Story 1 - Developer Package Import (Priority: P1) 🎯 MVP

**Goal**: Resolve all required dependencies so packages can be imported without errors.
**Independent Test**: `go build ./...` succeeds with the dependency stub.

- [x] T005 [P] [US1] Add `github.com/rshade/finfocus-spec` v0.5.4 dependency via `go get`
- [x] T006 [P] [US1] Add `github.com/hashicorp/go-retryablehttp` v0.7.7 dependency via `go get`
- [x] T007 [P] [US1] Add `github.com/rs/zerolog` v1.33.0 dependency via `go get`
- [x] T008 [P] [US1] Add `google.golang.org/grpc` dependency via `go get`
- [x] T009 [US1] Update `cmd/plugin/main.go` to import and use types from all dependencies to prevent pruning
- [x] T010 [US1] Run `go mod tidy` and verify `go.mod` contains all required entries
- [x] T011 [US1] Execute `go build ./...` to verify successful dependency resolution

## Phase 4: User Story 2 - Build Engineer Version Verification (Priority: P2)

**Goal**: Ensure explicit version constraints and checksum validation.
**Independent Test**: `go mod verify` returns success.

- [x] T012 [US2] Audit `go.mod` to ensure direct dependencies have explicit version constraints
- [x] T013 [US2] Execute `go mod verify` to confirm checksum validation in `go.sum`
- [x] T014 [US2] Run `go list -m all` and verify output against `data-model.md` requirements

## Phase 5: User Story 3 - Plugin Maintainer SDK Compatibility (Priority: P3)

**Goal**: Validate compatibility with `finfocus-spec` gRPC interfaces.
**Independent Test**: Stub implementation of `CostSourceServiceServer` compiles.

- [x] T015 [US3] Add stub implementation of `finfocus.v1.CostSourceServiceServer` in `internal/pricing/calculator.go`
- [x] T016 [US3] Update `cmd/plugin/main.go` to reference the stub implementation and SDK server helpers
- [x] T017 [US3] Run `go build ./...` to verify interface compatibility

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T018 Run `go mod tidy` one final time to ensure cleanliness
- [x] T019 Update `README.md` with "Getting Started" section referencing the dependency setup
- [x] T020 Ensure `CLAUDE.md` reflect any new common commands discovered during setup
- [x] T021 [P] Verify no Azure Auth libraries are present in dependency graph (`go list -m all | grep -v azidentity`)

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after US1 is functional.
- **User Story 3 (P3)**: Can start after US1 is functional.

### Within Each User Story

- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- Models within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

## Parallel Execution Example

```bash
# Launch all US1 dependencies together:
Task: "Add github.com/rshade/finfocus-spec v0.5.4 dependency via go get"
Task: "Add github.com/hashicorp/go-retryablehttp v0.7.7 dependency via go get"
Task: "Add github.com/rs/zerolog v1.33.0 dependency via go get"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently (`go build ./...`)
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories