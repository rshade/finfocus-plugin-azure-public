# Tasks: FOCUS 1.3 Cost Record Alignment

**Input**: Design documents from `specs/023-focus-13-record/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: Required. The constitution makes TDD non-negotiable. Each test task
comes before its implementation task and must fail first.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [x] T001 Confirm the baseline: `go test ./internal/pricing/ -run Focus -count=1`
  passes on the branch before any change

## Phase 2: Foundational

- [x] T002 Add the FOCUS unit constants `Hours` and `Months` and the provider
  name `Microsoft` in `internal/pricing/focus.go`

## Phase 3: User Story 1 - FOCUS 1.3 provider columns (P1)

- [x] T003 [US1] Test: `TestBuildFocusRecord_FOCUS13Providers_SetsMandatoryNames`
  in `internal/pricing/focus_test.go`. It asserts the service, host, invoice
  issuer, provider, and publisher names are `Microsoft`, and the account name
  equals the id
- [x] T004 [US1] Implement: `WithIdentity("Microsoft", id, id)`,
  `WithPublisher`, `WithServiceProvider`, `WithHostProvider`, and `WithInvoice`
  in `buildFocusRecord`

## Phase 4: User Story 2 - Cost and pricing columns (P1)

- [x] T005 [US2] Test: `TestFocusPricingBasis_Quotes_ReturnMeterOrWindowBasis`
  (table: single hourly meter, scale set count 3, single monthly meter,
  multi-meter, zero-price meter, GB-month meter) in
  `internal/pricing/focus_test.go`
- [x] T006 [US2] Test: `TestBuildFocusRecord_CostColumns_AgreeWithUnitPrice`
  asserts contracted cost and contracted unit price are set, and
  `list_unit_price × pricing_quantity == list_cost` and
  `contracted_unit_price × pricing_quantity == contracted_cost`
- [x] T007 [US2] Implement `focusPricingBasis` and wire
  `WithPricing`, `WithUsage`, `WithContractedCost`, `WithContractedUnitPrice`
  in `internal/pricing/focus.go`
- [x] T008 [US2] Update existing expectations in
  `internal/pricing/focus_test.go` (`hour` becomes `Hours`, quantity rules)

## Phase 5: User Story 3 - Service category and subcategory (P2)

- [x] T009 [US3] Test: `TestFocusServiceClass_EverySupportedType_MatchesMicrosoftMapping`
  table over `SupportedResourceTypes()` plus Pulumi tokens, with a parent
  check against the FOCUS 1.3 allowed pairs used
- [x] T010 [US3] Implement `focusServiceClass` replacing `focusServiceCategory`
  and set `WithServiceSubcategory` in `internal/pricing/focus.go`
- [x] T011 [US3] Confirm `TestFocusServiceCategory` and
  `TestFocusNativeFunctionWebApp` still pass with disks under Compute (neither
  needed a change)

## Phase 6: Cross-cutting validation

- [x] T012 Test: `TestBuildFocusRecord_EverySupportedType_PassesAggregateValidation`
  runs `pluginsdk.ValidateFocusRecordWithOptions` in aggregate mode with zero
  errors for each supported type (FR-007, SC-001)
- [x] T013 Confirm FR-008: projected, estimate, and DryRun tests and goldens
  pass with no change

## Phase 7: User Story 4 - Documentation (P3)

- [x] T014 [P] [US4] Write `docs/focus-mapping.md` covering each set column and
  its Azure source, deliberately empty columns with reasons, and the
  finfocus-spec#612 note
- [x] T015 [P] [US4] Update the CLAUDE.md FOCUS paragraph under "Other cost RPCs"

## Phase 8: Polish

- [x] T016 Run `make build`, `make test`, `make lint`, markdownlint-cli2, and
  vale on changed markdown

## Phase 9: Review follow-up (PR #74)

- [x] T017 Test: `TestGetActualCost_VMSizeKeys_BuildsFocusRecord` and
  `TestGetActualCost_RealQuotes_SetFocusPricingBasis` in
  `internal/pricing/focus_actual_test.go`, over gRPC with realistic retail
  rows for VM, Spot, scale set, Windows, disk, AKS, SQL, Functions, and Load
  Balancer
- [x] T018 Test: `TestBuildFocusRecord_ProductionCostPath_QuantityIsExact`
  (24 and 72 exactly)
- [x] T019 Implement: `focusServiceClass` returns the Services.csv ServiceName
  and no longer goes through `MapDescriptorToQuery`; a Function App on a plan
  SKU is Azure App Service
- [x] T020 Implement: `quoteMeter.count`, counted quantities, PricingUnits.csv
  units, and unset unit prices for blended quotes
- [x] T021 Update `docs/focus-mapping.md`, CLAUDE.md, and these artifacts

## Dependencies

- T002 comes before every implementation task.
- In each story, the test comes before the implementation.
- US1, US2, and US3 all change `focus.go`, so run them in order.
- T014 and T015 run in parallel after T010.
