# Tasks: ResourceDescriptor to Azure Filter Mapping

**Input**: Design documents from
`/specs/016-descriptor-filter-mapping/`
**Prerequisites**: plan.md, spec.md, research.md,
data-model.md, contracts/mapper.go

**Tests**: Included — spec requires >=80% test coverage
(SC-005) and plan specifies TDD workflow.

**Organization**: Tasks grouped by user story (US1-US4) for
independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no
  dependencies)
- **[Story]**: Which user story this task belongs to
  (US1, US2, US3, US4)
- Exact file paths included in all descriptions

## Phase 1: Setup

**Purpose**: Add sentinel errors and gRPC mappings needed
by all user stories

- [x] T001 Add ErrUnsupportedResourceType and ErrMissingRequiredFields sentinel errors to internal/pricing/errors.go
- [x] T002 Add gRPC status mappings for ErrUnsupportedResourceType (Unimplemented) and ErrMissingRequiredFields (InvalidArgument) to MapToGRPCStatus in internal/pricing/errors.go
- [x] T003 [P] Add table-driven tests for new gRPC mappings (direct and wrapped) in internal/pricing/errors_test.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core mapper infrastructure that MUST be complete
before ANY user story testing

**CRITICAL**: No user story work can begin until this phase
is complete

- [x] T004 Create internal/pricing/mapper.go with resourceTypeToService registry map (compute/virtualmachine -> "Virtual Machines", storage/manageddisk -> "Managed Disks", storage/blobstorage -> "Storage")
- [x] T005 Add resolveField helper function (primary field with tag fallback) to internal/pricing/mapper.go
- [x] T006 Implement MapDescriptorToQuery function with provider validation, resource type lookup, field resolution, multi-field validation, and default USD currency in internal/pricing/mapper.go
- [x] T007 [P] Add SupportedResourceTypes function returning canonical resource type names (sorted) to internal/pricing/mapper.go

**Checkpoint**: Foundation ready — mapper compiles and user
story testing can begin

---

## Phase 3: User Story 1 — Map VM Resource to Pricing Query (Priority: P1) MVP

**Goal**: Translate VM resource descriptions (SKU, region)
into correct Azure pricing query filters targeting
"Virtual Machines" service

**Independent Test**: Provide a VM ResourceDescriptor with
known SKU and region, verify the PriceQuery contains correct
Azure-specific field mappings

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before
> implementation tasks are marked done**

- [x] T008 [P] [US1] Write table-driven test for valid VM mapping (provider "azure", type "compute/VirtualMachine", SKU "Standard_B1s", region "eastus") verifying PriceQuery fields in internal/pricing/mapper_test.go
- [x] T009 [P] [US1] Write test for VM mapping preserving full Azure SKU name (e.g., "Standard_D2s_v3") without normalization in internal/pricing/mapper_test.go
- [x] T010 [P] [US1] Write test for VM mapping defaulting to USD currency when no currency preference in internal/pricing/mapper_test.go

### Integration for User Story 1

- [x] T011 [US1] Wire MapDescriptorToQuery into Calculator.Supports() to return Supported:true for valid VM descriptors in internal/pricing/calculator.go
- [x] T012 [US1] Add Supports() test for valid VM descriptor returning Supported:true in internal/pricing/calculator_test.go

**Checkpoint**: VM resource descriptions produce correct
PriceQuery; Supports() returns true for VMs

---

## Phase 4: User Story 2 — Map Disk Resource to Pricing Query (Priority: P2)

**Goal**: Translate managed disk and blob storage resource
descriptions into correct Azure pricing query filters
targeting "Managed Disks" or "Storage" services

**Independent Test**: Provide a ManagedDisk
ResourceDescriptor, verify PriceQuery targets
"Managed Disks" service; provide BlobStorage descriptor,
verify PriceQuery targets "Storage" service

### Tests for User Story 2

- [x] T013 [P] [US2] Write test for ManagedDisk mapping (SKU "Premium_LRS", region "westus2") producing ServiceName "Managed Disks" in internal/pricing/mapper_test.go
- [x] T014 [P] [US2] Write test for ManagedDisk Standard_LRS tier mapping producing ServiceName "Managed Disks" in internal/pricing/mapper_test.go
- [x] T015 [P] [US2] Write test for BlobStorage mapping (region "eastus") producing ServiceName "Storage" in internal/pricing/mapper_test.go

### Integration for User Story 2

- [x] T016 [US2] Add Supports() test for ManagedDisk descriptor returning Supported:true in internal/pricing/calculator_test.go

**Checkpoint**: Disk resource descriptions produce correct
PriceQuery with appropriate Azure service names

---

## Phase 5: User Story 3 — Validate Resource Description Completeness (Priority: P2)

**Goal**: Return clear, actionable error messages identifying
all missing required fields (region, SKU) when resource
descriptions are incomplete; support tag fallback

**Independent Test**: Provide incomplete ResourceDescriptors
(missing region, missing SKU, missing both), verify specific
error messages naming all missing fields; provide tag-only
descriptors, verify tag fallback works

### Tests for User Story 3

- [x] T017 [P] [US3] Write test for missing region (no primary, no tag) returning ErrMissingRequiredFields with "region" in message in internal/pricing/mapper_test.go
- [x] T018 [P] [US3] Write test for missing SKU (no primary, no tag) returning ErrMissingRequiredFields with "sku" in message in internal/pricing/mapper_test.go
- [x] T019 [P] [US3] Write test for both region and SKU missing returning single error naming both fields in internal/pricing/mapper_test.go
- [x] T020 [P] [US3] Write test for tag fallback (empty Region, Tags["region"]="eastus") producing correct PriceQuery.ArmRegionName in internal/pricing/mapper_test.go
- [x] T021 [P] [US3] Write test for empty-string fields treated as missing (Region="" with no tag fallback) in internal/pricing/mapper_test.go
- [x] T022 [P] [US3] Write test for primary field taking precedence over tag (Region="eastus", Tags["region"]="westus2") in internal/pricing/mapper_test.go
- [x] T023 [P] [US3] Write test for nil descriptor input returning clear error (not panic) in internal/pricing/mapper_test.go

### Integration for User Story 3

- [x] T024 [US3] Add Supports() test for incomplete descriptor returning Supported:false with reason in internal/pricing/calculator_test.go

**Checkpoint**: Validation errors name all missing fields;
tag fallback works; primary fields take precedence; nil
input is handled safely

---

## Phase 6: User Story 4 — Handle Unsupported Resource Types (Priority: P3)

**Goal**: Return clear "unimplemented" response for
unsupported resource types and non-azure providers,
including the unsupported type/provider name

**Independent Test**: Provide a ResourceDescriptor with
unknown type ("network/LoadBalancer") or non-azure provider
("aws"), verify ErrUnsupportedResourceType is returned with
the type/provider name

### Tests for User Story 4

- [x] T025 [P] [US4] Write test for unknown resource type ("network/LoadBalancer") returning ErrUnsupportedResourceType with type name in internal/pricing/mapper_test.go
- [x] T026 [P] [US4] Write test for completely unknown type ("custom/Widget") returning ErrUnsupportedResourceType in internal/pricing/mapper_test.go
- [x] T027 [P] [US4] Write test for non-azure provider ("aws") returning ErrUnsupportedResourceType with provider name in internal/pricing/mapper_test.go
- [x] T028 [P] [US4] Write test for case-insensitive resource type matching ("Compute/VirtualMachine", "COMPUTE/VIRTUALMACHINE") in internal/pricing/mapper_test.go

### Integration for User Story 4

- [x] T029 [US4] Add Supports() test for unsupported type returning Supported:false with reason including type name in internal/pricing/calculator_test.go

**Checkpoint**: Unsupported types/providers produce clear
error messages; case-insensitive matching works

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Validation, documentation, and compliance tasks
across all user stories

### Constitution Compliance Tasks

- [x] T030 [P] Run `make lint` and fix all linting issues
- [x] T031 [P] Run `go test -race ./internal/pricing/...` to verify thread safety
- [x] T032 [P] Verify test coverage >=80% for internal/pricing/ with `go test -cover ./internal/pricing/...`
- [x] T033 [P] Add godoc comments to all exported functions and types in internal/pricing/mapper.go (MapDescriptorToQuery, SupportedResourceTypes, ErrUnsupportedResourceType, ErrMissingRequiredFields)
- [x] T034 [P] Verify docstring coverage >=80% across internal/pricing/ package (count exported symbols with godoc vs total exported symbols per constitution Section IV)
- [x] T035 [P] Update CLAUDE.md with mapper capability documentation and new error sentinels
- [x] T036 [P] Update README.md with supported Azure resource types (VM, ManagedDisk, BlobStorage) and mapper usage examples per constitution Section IV
- [x] T037 [P] Update CHANGELOG.md with new mapper feature entry per constitution Section IV (DEFERRED: no CHANGELOG.md exists yet — will be created during release prep)

### Additional Validation

- [x] T038 Run quickstart.md code examples as verification (manual review against implementation)
- [x] T039 Verify SupportedResourceTypes() returns sorted canonical names matching quickstart.md

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start
  immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (sentinel
  errors must exist)
- **User Stories (Phases 3-6)**: All depend on Phase 2
  (mapper must compile)
  - US1-US4 can proceed in parallel or sequentially in
    priority order
- **Polish (Phase 7)**: Depends on all user stories being
  complete

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 — no dependencies
  on other stories
- **US2 (P2)**: Can start after Phase 2 — no dependencies
  on other stories
- **US3 (P2)**: Can start after Phase 2 — no dependencies
  on other stories
- **US4 (P3)**: Can start after Phase 2 — no dependencies
  on other stories

### Within Each User Story

- Tests MUST be written and FAIL before implementation is
  considered done
- Integration tests (Supports() wiring) depend on mapper
  tests passing
- All test tasks within a story marked [P] can run in
  parallel

### Parallel Opportunities

- T003 (error tests) can run in parallel with T004-T007
  (foundational)
- T008/T009/T010 (US1 tests) can run in parallel
- T013/T014/T015 (US2 tests) can run in parallel
- T017-T023 (US3 tests) can run in parallel
- T025-T028 (US4 tests) can run in parallel
- T030-T037 (polish) can run in parallel
- All four user stories can be worked in parallel by
  different developers

---

## Parallel Example: User Story 1

```text
# Launch all US1 mapper tests together:
Task T008: "Write test for valid VM mapping"
Task T009: "Write test for SKU preservation"
Task T010: "Write test for default USD currency"

# Then sequential integration:
Task T011: "Wire mapper into Calculator.Supports()"
Task T012: "Add Supports() test for VM descriptor"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (sentinel errors + gRPC mappings)
2. Complete Phase 2: Foundational (mapper.go with registry
   and helpers)
3. Complete Phase 3: User Story 1 (VM mapping tests +
   Supports() wiring)
4. **STOP and VALIDATE**: `make test` + `make lint` — VM
   mapping works end-to-end
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> mapper infrastructure ready
2. Add US1 (VM) -> Test independently -> MVP!
3. Add US2 (Disk) -> Test independently -> Broader resource
   coverage
4. Add US3 (Validation) -> Test independently ->
   Production-ready error handling
5. Add US4 (Unsupported types) -> Test independently ->
   Graceful degradation
6. Polish -> Full compliance

### File Touch Summary

| File | Action | Phases |
| --- | --- | --- |
| `internal/pricing/errors.go` | MODIFY | 1 |
| `internal/pricing/errors_test.go` | MODIFY | 1 |
| `internal/pricing/mapper.go` | CREATE | 2 |
| `internal/pricing/mapper_test.go` | CREATE | 3, 4, 5, 6 |
| `internal/pricing/calculator.go` | MODIFY | 3 |
| `internal/pricing/calculator_test.go` | MODIFY | 3-6 |
| `CLAUDE.md` | MODIFY | 7 |
| `README.md` | MODIFY | 7 |
| `CHANGELOG.md` | MODIFY | 7 |
