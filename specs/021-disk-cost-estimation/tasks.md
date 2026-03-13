# Tasks: Managed Disk Cost Estimation

**Input**: Design documents from `/specs/021-disk-cost-estimation/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Included per constitution (TDD is NON-NEGOTIABLE — tests MUST be written BEFORE implementation).

**Organization**: Tasks grouped by user story. US1+US2 are both P1 and tightly coupled (estimation + validation).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create new file skeleton for disk-specific logic

- [x] T001 Create `internal/pricing/disk.go` with package declaration, doc comment, and placeholder constants/types for disk type mapping and tier capacity table
- [x] T002 Create `internal/pricing/disk_test.go` with package declaration and import block

**Checkpoint**: New files exist, `make build` passes

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Build disk type mapping table, tier capacity table, and utility functions that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T003 [P] Write table-driven tests for `normalizeDiskType()` in `internal/pricing/disk_test.go` — cover all 6 supported types (Standard_LRS, StandardSSD_LRS, Premium_SSD_LRS, Standard_ZRS, StandardSSD_ZRS, Premium_ZRS), case-insensitive input, Premium_SSD_LRS→Premium_LRS normalization, and unsupported type error
- [x] T004 [P] Write table-driven tests for `tierForSize()` in `internal/pricing/disk_test.go` — cover exact tier boundaries (32, 128, 512, 1024), ceiling match (100→128, 200→512), smallest tier (4 GiB), largest tier (32767 GiB), and error for size > max tier
- [x] T005 [P] Write tests for `isManagedDiskResourceType()` in `internal/pricing/disk_test.go` — cover `storage/managedDisk`, `azure:storage/managedDisk:ManagedDisk`, case variations, and negative cases (VM type, empty string, partial match)

### Implementation for Foundational

- [x] T006 [P] Implement `supportedDiskTypes` map and `normalizeDiskType()` function in `internal/pricing/disk.go` — map user-facing disk type names to Azure armSkuName values (e.g., `Premium_SSD_LRS` → `Premium_LRS`), return error for unsupported types
- [x] T007 [P] Implement `diskTierCapacities` table and `tierForSize()` function in `internal/pricing/disk.go` — static table of 14 tier numbers to GiB capacities, ceiling-match function that returns tier name (e.g., "P10") for a given prefix and size_gb, rounds up fractional sizes
- [x] T008 [P] Implement `isManagedDiskResourceType()` helper in `internal/pricing/disk.go` — pattern matching for `storage/manageddisk` segment in resource type string (case-insensitive), consistent with existing `isVirtualMachineResourceType()` pattern
- [x] T009 Verify all foundational tests pass: run `go test ./internal/pricing/... -run "TestNormalizeDiskType|TestTierForSize|TestIsManagedDiskResourceType" -v`

**Checkpoint**: All disk utility functions work, `make test` passes, foundational tests green

---

## Phase 3: User Story 1+2 — Estimate Monthly Disk Cost + Validate Fields (Priority: P1) MVP

**Goal**: EstimateCost accepts disk resource type, validates required fields (region, disk_type, size_gb), queries Azure pricing, selects correct tier, and returns monthly cost. Missing/invalid fields return clear InvalidArgument errors.

**Independent Test**: Send EstimateCost request with disk attributes → verify non-zero monthly cost returned. Send request with missing fields → verify InvalidArgument error naming all missing fields.

### Tests for User Story 1+2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T010 [P] [US1] Write table-driven test `TestEstimateCost_Disk_Success` in `internal/pricing/calculator_test.go` — mock CachedClient returning PriceItems with disk meter names (P10, S30, etc.), verify monthly cost equals tier's retailPrice directly (no HoursPerMonth multiplication), verify currency and pricing category
- [x] T011 [P] [US2] Write table-driven test `TestEstimateCost_Disk_ValidationErrors` in `internal/pricing/calculator_test.go` — cover missing region, missing disk_type, missing size_gb, all fields missing (single error message), size_gb=0, size_gb=-1, unsupported disk type
- [x] T012 [P] [US1] Write test `TestEstimateCost_Disk_ResourceTypeRouting` in `internal/pricing/calculator_test.go` — verify disk resource type routes to disk path, VM resource type still routes to VM path, empty resource type falls through to VM path (backward compat)

### Implementation for User Story 1+2

- [x] T013 [US1] Implement `estimateDiskQueryFromRequest()` in `internal/pricing/calculator.go` — extract region (location/region aliases), disk_type (diskType/sku aliases), size_gb (sizeGb/diskSizeGb aliases), currency from EstimateCostRequest attributes, validate all required fields present, validate size_gb > 0, normalize disk_type via `normalizeDiskType()`, build PriceQuery with ServiceName "Managed Disks"
- [x] T014 [US1] Implement `selectDiskTierPrice()` in `internal/pricing/disk.go` — given []PriceItem results and target size_gb, determine tier prefix from disk type, call `tierForSize()` for ceiling match, filter items by matching meterName, return retailPrice and currency, handle no-match error
- [x] T015 [US1] Modify `EstimateCost()` in `internal/pricing/calculator.go` — replace `isVirtualMachineResourceType()` gate with resource-type routing: check `isManagedDiskResourceType()` first → call disk estimation path, then check `isVirtualMachineResourceType()` → existing VM path, else → Unimplemented error. Disk path: call `estimateDiskQueryFromRequest()`, call `cachedClient.GetPrices()`, call `selectDiskTierPrice()`, return cost_monthly directly (no HoursPerMonth), add structured logging with disk_type, size_gb, tier fields
- [x] T016 [US1] Verify US1+US2 tests pass: run `go test ./internal/pricing/... -run "TestEstimateCost_Disk" -v -race`

**Checkpoint**: Disk cost estimation works end-to-end with mocked client. Validation errors are clear. VM path unchanged. `make test` passes.

---

## Phase 4: User Story 3 — Distinguish Disk Types (Priority: P2)

**Goal**: All 6 supported disk types (3 LRS + 3 ZRS) return correct, distinct pricing. Unsupported types (Ultra Disk, Premium SSD v2) return InvalidArgument.

**Independent Test**: Send EstimateCost for each of 6 disk types → verify different costs returned. Send request for UltraSSD_LRS → verify InvalidArgument.

### Tests for User Story 3

- [x] T017 [P] [US3] Write table-driven test `TestEstimateCost_Disk_AllTypes` in `internal/pricing/calculator_test.go` — 6 test cases (one per supported type), mock CachedClient with type-specific PriceItems, verify each type produces correct armSkuName in query and returns expected cost
- [x] T018 [P] [US3] Write test `TestEstimateCost_Disk_UnsupportedTypes` in `internal/pricing/calculator_test.go` — cover UltraSSD_LRS, PremiumV2_LRS, and made-up type → verify InvalidArgument error with descriptive message

### Implementation for User Story 3

- [x] T019 [US3] Verify all 6 disk types are in `supportedDiskTypes` map in `internal/pricing/disk.go` — ensure Standard_ZRS, StandardSSD_ZRS, Premium_ZRS are included with correct armSkuName mappings and tier prefixes (S, E, P respectively; ZRS meter names use " ZRS" suffix)
- [x] T020 [US3] Update `selectDiskTierPrice()` in `internal/pricing/disk.go` to handle ZRS meter name format — ZRS tiers have meterName like "P10 ZRS" instead of "P10", add suffix handling based on redundancy type
- [x] T021 [US3] Verify US3 tests pass: run `go test ./internal/pricing/... -run "TestEstimateCost_Disk_AllTypes|TestEstimateCost_Disk_UnsupportedTypes" -v`

**Checkpoint**: All 6 disk types work correctly. ZRS variants produce higher costs than LRS equivalents (when using real API). Unsupported types rejected. `make test` passes.

---

## Phase 5: User Story 4 — Scale Cost by Provisioned Size (Priority: P2)

**Goal**: Costs correctly scale with disk size via ceiling-match tier mapping. 256GB costs more than 128GB. 32GB maps to smallest tier. Very large disks map to highest tier.

**Independent Test**: Send EstimateCost with different size_gb values for same disk type → verify costs increase with size.

### Tests for User Story 4

- [x] T022 [P] [US4] Write table-driven test `TestEstimateCost_Disk_SizeScaling` in `internal/pricing/calculator_test.go` — mock CachedClient with multiple tier PriceItems (P4=32GB@$1, P10=128GB@$5, P20=512GB@$20), test size_gb=32 → P4 price, size_gb=100 → P10 price (ceiling match), size_gb=128 → P10 price (exact), size_gb=200 → P20 price
- [x] T023 [P] [US4] Write test `TestEstimateCost_Disk_SizeEdgeCases` in `internal/pricing/calculator_test.go` — cover size_gb=0.5 (rounds up to 1, maps to smallest tier), size_gb=32767 (maps to largest tier), size_gb=99999 (exceeds max tier → NotFound error), fractional size_gb=128.5 (rounds up to 129 → P15/256GB tier)

### Implementation for User Story 4

- [x] T024 [US4] Review and verify ceiling-match logic in `tierForSize()` handles all edge cases in `internal/pricing/disk.go` — ensure fractional rounding (math.Ceil), size > max tier error, exact boundary match, and consistent behavior across all tier prefixes
- [x] T025 [US4] Verify US4 tests pass: run `go test ./internal/pricing/... -run "TestEstimateCost_Disk_Size" -v`

**Checkpoint**: Size-to-tier mapping is accurate for all boundary conditions. `make test` passes.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Compliance, documentation, and quality verification

### Constitution Compliance Tasks

- [x] T026 [P] Run `make lint` and fix all linting issues across modified files
- [x] T027 [P] Run `go test -race ./internal/pricing/...` to verify thread safety with cached client
- [x] T028 [P] Verify test coverage >=80% for `internal/pricing/` package: run `go test -cover ./internal/pricing/...`
- [x] T029 [P] Add godoc comments to all new exported functions/types in `internal/pricing/disk.go` (docstring coverage MUST be >=80%)
- [x] T030 [P] Update CLAUDE.md with Managed Disk cost estimation section (supported disk types, usage example, attribute aliases)
- [x] T031 [P] Update README.md supported resource types table to include `storage/ManagedDisk`
- [x] T032 Verify all structured logging in disk estimation path includes `disk_type`, `size_gb`, `tier`, `region`, `result_status` fields via zerolog
- [x] T033 Verify error messages are actionable: include disk type, size, region in error context (consistent with VM error pattern)
- [x] T034 Run full test suite: `make test` — verify no regressions in VM estimation
- [x] T035 Run `make build` to verify clean compilation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **US1+US2 (Phase 3)**: Depends on Foundational (Phase 2)
- **US3 (Phase 4)**: Depends on US1+US2 (Phase 3) — extends type handling
- **US4 (Phase 5)**: Depends on US1+US2 (Phase 3) — extends size handling; can run in PARALLEL with US3
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **US1+US2 (P1)**: Can start after Foundational — no dependencies on US3/US4
- **US3 (P2)**: Depends on US1+US2 for the base estimation path. Extends disk type coverage.
- **US4 (P2)**: Depends on US1+US2 for the base estimation path. Can run in PARALLEL with US3 (different concerns: type mapping vs size mapping).

### Within Each User Story

- Tests MUST be written and FAIL before implementation (TDD)
- Utility functions before integration functions
- Core implementation before error handling refinement
- Story complete and green before moving to next priority

### Parallel Opportunities

- T003, T004, T005 (foundational tests) can all run in parallel
- T006, T007, T008 (foundational implementation) can all run in parallel
- T010, T011, T012 (US1+US2 tests) can all run in parallel
- T017, T018 (US3 tests) can run in parallel
- T022, T023 (US4 tests) can run in parallel
- **US3 and US4 can run in parallel** after US1+US2 completes
- All Polish tasks marked [P] can run in parallel

---

## Parallel Example: Foundational Phase

```text
# Launch all foundational tests together (TDD: write tests first):
Task: T003 "Tests for normalizeDiskType() in internal/pricing/disk_test.go"
Task: T004 "Tests for tierForSize() in internal/pricing/disk_test.go"
Task: T005 "Tests for isManagedDiskResourceType() in internal/pricing/disk_test.go"

# Then launch all foundational implementations together:
Task: T006 "Implement supportedDiskTypes + normalizeDiskType() in internal/pricing/disk.go"
Task: T007 "Implement diskTierCapacities + tierForSize() in internal/pricing/disk.go"
Task: T008 "Implement isManagedDiskResourceType() in internal/pricing/disk.go"
```

## Parallel Example: US3 and US4 (after US1+US2)

```text
# These two story phases can run in parallel:
# Developer A: US3 (disk types)
Task: T017 → T018 → T019 → T020 → T021

# Developer B: US4 (size scaling)
Task: T022 → T023 → T024 → T025
```

---

## Implementation Strategy

### MVP First (US1+US2 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T009)
3. Complete Phase 3: US1+US2 (T010-T016)
4. **STOP and VALIDATE**: Test disk estimation independently with `make test`
5. Deploy/demo with basic Premium_SSD_LRS support

### Incremental Delivery

1. Setup + Foundational → Utility functions ready
2. Add US1+US2 → Test independently → MVP! (basic disk estimation works)
3. Add US3 → All 6 disk types supported
4. Add US4 → Size scaling verified with edge cases
5. Polish → Docs, lint, coverage verified

### Parallel Team Strategy

1. Complete Setup + Foundational together
2. One developer: US1+US2 (core estimation)
3. Once US1+US2 done:
   - Developer A: US3 (disk types)
   - Developer B: US4 (size scaling)
4. Both complete → Polish phase

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 and US2 are combined because validation (US2) is inseparable from the estimation path (US1)
- Disk prices are monthly — do NOT multiply by HoursPerMonth (critical difference from VM path)
- Static tier capacity table has 14 entries — this is NOT "bulk pricing data" (sizes only, not prices)
- ZRS meter names have " ZRS" suffix (e.g., "P10 ZRS" vs "P10") — handle in selectDiskTierPrice
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
