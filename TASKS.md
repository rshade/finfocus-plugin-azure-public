# TASKS.md — Azure Plugin v0.1.0 Release Plan

<!-- markdownlint-disable MD013 MD060 MD031 MD032 MD029 -->

**Current branch**: `run/grok-20261003` | **Target**: v0.1.0 |
**Updated**: 2026-10-03. No release tag.

## Current State Summary

### Build & Test Status

- **Build**: passing at the AZ-6.10 commit (`go build ./...`)
- **Tests**: `go test -count=1 -v ./...` passed after AZ-6.10. The only skips are nine accuracy cases. Each names `owner_monthly_usd`.
- **Linting**: the 2026-09-30 note of two findings is obsolete. Re-run `make lint` for a fresh result.
- **Go version**: 1.27.1, committed
- **Spec version**: `github.com/rshade/finfocus-spec` `v0.7.1`. `pluginsdk.SpecVersion` is `v0.7.1`.

### RPC Implementation Status

| RPC | Status | Notes |
| --- | --- | --- |
| `Supports` | Complete | Maps resource types |
| `EstimateCost` | Complete | Every mapped type. Virtual machines and managed disks keep their parsers. |
| `GetProjectedCost` | Complete | Monthly retail quote |
| `GetActualCost` | Complete | Monthly quote times hours over 730. A running-cost estimate, not billed spend. |
| `GetPricingSpec` | Complete | One spec for the quoted resource |
| `GetRecommendations` | Stub | Embedded unimplemented server |
| `DismissRecommendation` | Stub | Embedded unimplemented server |
| `GetBudgets` | Stub | Embedded unimplemented server |
| `DryRun` | Complete | Validates a descriptor and does not call the API |

### Resource Type Coverage

Mapped types (ten):

- `compute/VirtualMachine` (Spot is `priority=Spot`)
- `storage/ManagedDisk`
- `storage/BlobStorage`
- `storage/StorageAccount`
- `web/AppServicePlan`
- `web/FunctionApp`
- `containerservice/KubernetesCluster`
- `sql/Database` (`GP_Gen5` provisioned)
- `cosmosdb/Account`
- `network/LoadBalancer` (Standard rules)

Not priced: NAT Gateway, virtual machine scale sets, Cache for Redis, and database servers for PostgreSQL and MySQL. Gateway and cross-region load balancer meters are not quoted.

A virtual machine quote returns `price_options` ([spec issue 588](https://github.com/rshade/finfocus-spec/issues/588)) and `region_prices` ([spec issue 589](https://github.com/rshade/finfocus-spec/issues/589)). Both lists are advisory. The monthly cost stays the selected row. `GetActualCost` uses request `billing_account_id` when the caller sends one ([spec issue 590](https://github.com/rshade/finfocus-spec/issues/590)). A dry run ignores that field. An empty request id falls back to `FINFOCUS_BILLING_ACCOUNT_ID`. An empty id leaves the FOCUS record unset.

### Release Infrastructure

- `.goreleaser.yaml` is present (AZ-3.3)
- Release workflow files are present
- release-please configuration is present
- `pluginVersion` (`internal/pricing/calculator.go`) is hard-coded at `0.1.0`;
  release-please has no `extra-files` entry, so a release does not bump it or
  the manifest files
- No version tag in this run (AZ-5.1 is SKIPPED)
- The core registry file was not edited

### File Evidence

- Entry point: `cmd/finfocus-plugin-azure-public/main.go`
- Pricing: `internal/pricing`
- Client and cache: `internal/azureclient`

## Dependency-Ordered Phases to v0.1.0

### Phase 1: Dependency & Environment Alignment (AZ-1.x)

**Rationale**: Fix foundational gaps to match current spec and Go ecosystem.

**Completion**: 2026-09-30 — Go 1.27.1, finfocus-spec v0.7.0, all tests passing

#### AZ-1.1 — Upgrade Go to 1.27.1

**Status:** DONE, `go list -m -f '{{.GoVersion}}'` prints `1.27.1` and `grep finfocus-spec go.mod` prints `github.com/rshade/finfocus-spec v0.7.0`. Break check: `head -2 go.mod | grep "go 1.27.1"` exits 1 because the directive is on line 3.

**Description**: Update `go.mod` to Go 1.27.1 to match AWS plugin baseline and ensure compatibility with latest finfocus-spec versions.

**Files**: `go.mod`

**Acceptance Check**:
```bash
head -2 go.mod | grep "go 1.27.1"
go mod verify
go build ./...
```

**Linked issues**: (infrastructure, no GitHub issue)

---

#### AZ-1.2 — Upgrade finfocus-spec to v0.7.0

**Status:** DONE, grep "finfocus-spec" go.mod | grep "v0.7.0", github.com/rshade/finfocus-spec v0.7.0

**Description**: Bump `github.com/rshade/finfocus-spec` to v0.7.0 (latest). v0.7.0
adds FOCUS 1.4 fields and `cost_breakdown` map support on GetProjectedCostResponse.
Done in the 2026-09-30 rollout (uncommitted). Verify no breaking changes in proto imports.
See "Open questions" section for v0.7.0 impact analysis.

**Files**: `go.mod`, `go.sum`, `cmd/finfocus-plugin-azure-public/main.go` (minor SDK
method changes if any)

**Acceptance Check**:
```bash
grep "finfocus-spec" go.mod | grep "v0.7.0"
go mod tidy && go mod verify
go build ./...
go test ./...
```

**Linked issues**: (infrastructure, no GitHub issue)

---

#### AZ-1.3 — Update README Go Version Reference

**Status:** DONE, grep "Go 1.27" README.md, - Go 1.27.1 or higher

**Description**: Update README.md to reflect Go 1.27.1 requirement (currently says 1.25.5).

**Files**: `README.md` (line ~18)

**Acceptance Check**:
```bash
grep "Go 1.27" README.md
```

**Linked issues**: (infrastructure, no GitHub issue)

---

### Phase 2: RPC Implementations and Resource Type Coverage (AZ-2.x)

**Rationale**: Move from partial stubs to production-ready, fully-tested RPCs
(AZ-2.1, AZ-2.2), then cover every Azure resource type in scope (AZ-2.3 to
AZ-2.8). A plugin that prices three types is not a v0.1.0.

#### AZ-2.1 — Finalize GetProjectedCost RPC [Issue #59]

**Status:** DONE, go test -count=1 -run 'TestGetProjectedCost|TestGetActualCost' ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.019s

**Description**: Promote the working `GetProjectedCost` implementation to production-ready status with structured validation, error handling, and logging to match AWS plugin pattern. Key tasks:
- Replace silent `ok=false` fallback with explicit `codes.InvalidArgument` errors
- Add `pricing_category` field (set to `STANDARD`)
- Add human-readable `billing_detail` explanation
- Implement structured logging (request entry, success/failure with context)
- Validate: region/sku present, provider="azure", resource type supported
- Propagate `expires_at` from cache (already implemented)

**Files**:
- `internal/pricing/calculator.go` (GetProjectedCost method, ~60 lines)
- `internal/pricing/calculator_test.go` (add 8-10 test cases)

**Acceptance Check**:
```bash
# Tests for all error paths pass
go test -v -run TestGetProjectedCost ./internal/pricing/...

# No Unimplemented fallthrough on validation errors
./finfocus-plugin-azure-public 2>&1 &
grpcurl -plaintext -d '{
  "resource": {"provider": "azure", "resourceType": "compute/VirtualMachine", "sku": "Standard_B1s"}
}' localhost:PORT finfocus.v1.CostSourceService/GetProjectedCost
# Should return InvalidArgument (code 3), not Unimplemented (code 12)

# Verify pricing_category and billing_detail in response
grpcurl ... | jq '.pricingCategory, .billingDetail' | grep -i standard
```

**Linked issues**: [#59](https://github.com/rshade/finfocus-plugin-azure-public/issues/59)

---

#### AZ-2.2 — Finalize GetActualCost RPC [Issue #60]

**Status:** DONE, go test -count=1 -run 'TestGetProjectedCost|TestGetActualCost' ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.019s

**Description**: Promote the working `GetActualCost` implementation to production-ready status by delegating to GetProjectedCost for rates, then scaling by runtime hours (AWS plugin pattern). Key tasks:
- Replace silent `ok=false` fallback with explicit `codes.InvalidArgument` errors
- Implement timestamp resolution (start/end defaults, validation)
- Calculate `actual_cost = (monthly_rate) * (runtime_hours / 730)`
- Set `UsageAmount` and `UsageUnit` to reflect hours
- Implement confidence levels: HIGH (both timestamps), MEDIUM (start only), LOW (neither)
- Add `Source` field annotation with confidence: `"azure-retail-prices[confidence:HIGH]"`
- Structured logging (request, success/failure with cost and runtime)

**Files**:
- `internal/pricing/calculator.go` (GetActualCost method, ~80 lines)
- `internal/pricing/calculator_test.go` (add 10-12 test cases)

**Acceptance Check**:
```bash
# Tests for all error paths + timestamp logic pass
go test -v -run TestGetActualCost ./internal/pricing/...

# No Unimplemented fallthrough on validation errors
grpcurl -plaintext -d '{
  "resource": {"provider": "azure", "resourceType": "compute/VirtualMachine", "region": "eastus"},
  "start": "2026-09-01T00:00:00Z",
  "end": "2026-09-02T00:00:00Z"
}' localhost:PORT finfocus.v1.CostSourceService/GetActualCost
# Should return InvalidArgument (missing sku), not Unimplemented

# Verify confidence level and runtime calculation
grpcurl ... | jq '.results[0].source, .results[0].usageAmount' | grep -i confidence
```

**Linked issues**: [#60](https://github.com/rshade/finfocus-plugin-azure-public/issues/60)

---

#### Method for every resource type task (AZ-2.3 to AZ-2.9)

1. **Discover, do not guess.** Query the public Azure Retail Prices API
   (`https://prices.azure.com/api/retail/prices`, no credentials) for the service,
   and read the real `serviceName`, `productName`, `skuName`, `meterName`,
   `unitOfMeasure` and `priceType` values. Record the responses as golden fixtures
   under the test data directory. Tests use fixtures. Live calls are an opt-in smoke
   test, skipped by default and never part of CI.
2. **State the unit.** Each type prices in a specific unit (hours, GB-month, vCore
   hours, request units, executions). Write the unit and the conversion in the
   code and the test.
3. **Extend the two tables together:** `resourceTypeToService` and
   `canonicalResourceTypes` in `internal/pricing/mapper.go`. Use the same
   `module/Type` naming the existing three use, and record the exact type names
   chosen in the run report.
4. **Explicit errors.** A missing meter, an unknown SKU or an unsupported variant
   returns a gRPC error naming what was missing. Never a zero cost.
5. **`Supports` must answer true for the new type and false for unknown ones.**
6. **Table-driven tests** over the fixtures, including the failure cases, and one
   test that goes through a real gRPC server for each new type.

#### AZ-2.3 — Spot VM pricing support [Issue #42]

**Status:** DONE, go test -count=1 ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.030s

**Description**: Price Virtual Machines running at Spot priority. Azure exposes Spot
(and Low Priority) as separate rows of the Virtual Machines service, distinguished by
the SKU or meter name. Decide from the fixtures how a descriptor says "spot" (a tag or
a property) and document it.

**Files**: `internal/pricing/` (VM estimation), tests and fixtures.

**Acceptance Criteria**: a Spot VM priced below the same on-demand SKU in the same
region, from fixtures; an on-demand VM is unchanged; `go test -count=1 ./internal/pricing/...`.

**Linked issues**: [#42](https://github.com/rshade/finfocus-plugin-azure-public/issues/42)

#### AZ-2.4 — Storage Accounts capacity-based estimation [Issue #50]

**Status:** DONE, go test -count=1 ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.037s

**Description**: Estimate Storage Account cost from capacity (GB-month) and access
tier (Hot, Cool, Cold, Archive) and redundancy (LRS, ZRS, GRS and so on), beyond the
existing Blob storage type. Read the meters for each tier and redundancy from the API.

**Files**: `internal/pricing/` (new storage account estimator), tests and fixtures.

**Acceptance Criteria**: each tier and redundancy combination in scope priced from
fixtures; an unsupported combination returns an error; tests pass.

**Linked issues**: [#50](https://github.com/rshade/finfocus-plugin-azure-public/issues/50)

#### AZ-2.5 — App Service and Azure Functions [Issue #48]

**Status:** DONE, go test -count=1 ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.115s

**Description**: Price App Service plans (by plan SKU, per hour) and Azure Functions
(Consumption: executions and GB-seconds with the free grant, Premium and Dedicated:
by plan). State the free grant handling explicitly.

**Files**: `internal/pricing/` (new estimators), tests and fixtures.

**Acceptance Criteria**: an App Service plan SKU and a Functions Consumption example
priced from fixtures with the unit conversions shown; Premium priced per hour; tests pass.

**Linked issues**: [#48](https://github.com/rshade/finfocus-plugin-azure-public/issues/48)

#### AZ-2.6 — AKS cluster cost estimation [Issue #49]

**Status:** DONE, go test -count=1 ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.045s

**Description**: Price an AKS cluster: the control plane tier (Free, or Standard with
the uptime SLA) and the node pools, which are Virtual Machines priced with the
existing VM estimator. Read the real control plane meters from the API. Do not assume
a value from memory.

**Files**: `internal/pricing/` (new estimator that composes the VM estimator), tests
and fixtures.

**Acceptance Criteria**: a cluster with two node pools priced as control plane plus
node costs, each component visible (a `cost_breakdown` map is appropriate and must
follow the spec's rules); tests pass.

**Linked issues**: [#49](https://github.com/rshade/finfocus-plugin-azure-public/issues/49)

#### AZ-2.7 — Azure SQL Database mapping and estimation [Issue #51]

**Status:** DONE, go test -count=1 ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.055s

**Description**: First the spike: write down how SQL Database pricing maps, from the
API (vCore and DTU purchasing models, provisioned versus serverless, compute plus
storage). Then implement the estimation for the models you can price from the API.
Keep the spike findings in the repository so the next person need not rediscover them.

**Files**: `internal/pricing/` (new estimator), a findings document, tests and fixtures.

**Acceptance Criteria**: at least the vCore provisioned model priced from fixtures,
with unsupported models returning an explicit error; findings document present; tests pass.

**Linked issues**: [#51](https://github.com/rshade/finfocus-plugin-azure-public/issues/51)

#### AZ-2.8 — Cosmos DB mapping and estimation [Issue #51]

**Status:** DONE, go test -count=1 ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.069s

**Description**: Cosmos DB prices by provisioned throughput (request units per second)
or serverless, plus storage. Map what the API exposes, implement the provisioned and
serverless estimates, and record the findings with the SQL Database ones.

**Files**: `internal/pricing/` (new estimator), findings document, tests and fixtures.

**Acceptance Criteria**: provisioned throughput plus storage priced from fixtures, and
serverless if the API exposes it; an unsupported API or model returns an explicit error;
tests pass.

**Linked issues**: [#51](https://github.com/rshade/finfocus-plugin-azure-public/issues/51)

#### AZ-2.9 — Stretch: aws-public parity types

**Status:** DONE via AZ-6.9 for Standard rules. The other stretch types stay unpriced: NAT Gateway, virtual machine scale sets, Cache for Redis, and database servers for PostgreSQL and MySQL. Not delivered: Gateway and cross-region meters.

**Description**: Standard Load Balancer rules are quoted in AZ-6.9. The remaining stretch types stay unpriced: NAT Gateway, virtual machine scale sets, Cache for Redis, and database servers for PostgreSQL and MySQL.

**Files**: `internal/pricing/`, tests and fixtures.

**Acceptance Criteria**: each type attempted is fully tested or not started. A half-done
type is worse than none.

#### AZ-2.10 — DryRun validation RPC [Issue #43]

**Status:** DONE, go test -count=1 ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.072s

**Description**: Implement the stubbed `DryRun` RPC. It validates a `ResourceDescriptor`
without calling Azure: is the type supported, are the required fields present, and what
OData filter would be sent. It is `MapDescriptorToQuery` plus the filter builder with the
HTTP call left out. The issue is labelled spec-first: check `../finfocus-spec` v0.7.0
for the `DryRun` request and response messages and use what exists. Do not invent
fields. If the spec lacks something the issue wants, list it under decisions needed.

**Files**: `internal/pricing/` (`Calculator.DryRun`), tests.

**Acceptance Criteria**: valid, invalid and missing-field descriptors each produce the
right response; no HTTP request is made (prove it with a client that fails the test if
called); covers every resource type in scope; tests pass.

**Linked issues**: [#43](https://github.com/rshade/finfocus-plugin-azure-public/issues/43)

#### AZ-2.11 — GetPricingSpec RPC [Issue #44]

**Status:** DONE, go test -count=1 ./internal/pricing/, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.071s

**Description**: Implement the stubbed `GetPricingSpec` RPC so the plugin describes what
it can price: resource types, required and optional fields, supported currencies. Build
it from the mapper tables so it cannot drift from what `Supports` answers. Do this after
AZ-2.3 to AZ-2.8 so the spec lists every type. Check the spec v0.7.0 messages first.

**Files**: `internal/pricing/` (`Calculator.GetPricingSpec`), tests.

**Acceptance Criteria**: a test that compares the pricing spec's resource types with the
mapper tables and fails if they differ; every supported type is listed; tests pass.

**Linked issues**: [#44](https://github.com/rshade/finfocus-plugin-azure-public/issues/44)

#### AZ-2.12 — Savings Plans spike [Issue #57]

**Status:** DONE via AZ-6.3. `PriceItem` parses the nested `savingsPlan` array. `ReservationHourly` divides the term total by 8760 or 26280. A virtual machine quote returns those prices on `price_options`. See AZ-2.13.

**Description**: A research spike. Determine, from the public Retail Prices API, whether
Azure Savings Plans pricing is exposed, which `priceType` or filter values identify it,
whether it is per-resource or per-spend-commitment, and how it differs from Reserved
Instances. Query the real API and record the responses. Do not answer from memory.

**Deliverable**: a findings document in the repository, with the recorded API responses
as fixtures, and a clear yes or no on whether Savings Plans can be shown per SKU. It
feeds AZ-2.13.

**Acceptance Criteria**: each research question in the issue answered with evidence or
marked "not answerable from the public API".

**Linked issues**: [#57](https://github.com/rshade/finfocus-plugin-azure-public/issues/57)

#### AZ-2.13 — Multi-pricing model comparison [Issue #45]

**Status:** DONE, `go test -count=1 -run 'TestGetProjectedCostVMAlternativesFromFixtures|TestGetProjectedCostVMReservationOptionsFromFixtures|TestVMQuoteIgnoresAdvisoryFetchErrors' ./internal/pricing/` passed. A virtual machine quote returns Consumption, Spot, Savings Plan, and Reservation on `price_options` when the page has them. `Standard_D2s_v3` has no reservation rows, so Reservation is omitted. `Standard_D2als_v7` returns 1 Year and 3 Years. The savings fraction is tested. A failed extra query leaves the selected cost unchanged. The selected monthly cost does not include the options.

**Description**: For a query, return Consumption, 1-Year Reserved and 3-Year Reserved
prices side by side, plus Savings Plans if AZ-2.12 shows they are available. The
Retail Prices API exposes these through `priceType` and `reservationTerm`. The issue
is labelled spec-first: the open question is where the extra price points go in the
response. Read `../finfocus-spec` v0.7.0 (for example the `metadata` map and
`cost_breakdown` on `GetProjectedCostResponse`, and the `focus.proto` commitment
fields). If no suitable field exists, stop at that point, list it under decisions
needed, and do not invent one.

**Files**: `internal/pricing/` and `internal/azureclient/` (the filter builder), tests
and fixtures.

**Acceptance Criteria**: the three price points for a VM SKU from fixtures, with the
savings percentage computed and tested; the decision on the response shape recorded.

**Linked issues**: [#45](https://github.com/rshade/finfocus-plugin-azure-public/issues/45)

#### AZ-2.14 — FOCUS alignment [Issue #46]

**Status:** DONE for the record builder. `go test -count=1 -run 'TestGetActualCostRequestBillingAccountOverGRPC|TestGetActualCostRequestBillingAccountWithoutProcessOverGRPC|TestGetActualCostDryRunIgnoresRequestBillingAccount|TestGetActualCostFocusRecordOverGRPC|TestGetActualCostOmitsFocusRecordWithoutAccountOverGRPC' ./internal/pricing/` passed. Request `billing_account_id` wins. A dry run ignores it. An empty request id uses `FINFOCUS_BILLING_ACCOUNT_ID`. Empty leaves FocusRecord nil. Issue #46 stays open: `charge_type` has no proto field, and `commitment_discount_type` stays empty.

**Description**: The issue asks to align response fields with FOCUS 1.3 column names.
The spec has moved: v0.7.0 adds FOCUS 1.4 columns and a FOCUS record builder in
`sdk/go/pluginsdk/focus_builder.go`. Re-scope the issue against what the spec now
provides, use the SDK's builder and columns instead of custom mapping, and record any
place the issue's column table and the spec disagree.

**Files**: `internal/pricing/`, tests.

**Acceptance Criteria**: the mapped columns validated with the SDK's own validation;
a table in the findings document showing each column from the issue and where it
landed; tests pass.

**Linked issues**: [#46](https://github.com/rshade/finfocus-plugin-azure-public/issues/46)

#### AZ-2.15 — Regional price comparison [Issue #47]

**Status:** DONE, `go test -count=1 -run 'TestGetProjectedCostVMRegionPricesFromFixtures' ./internal/pricing/` passed. A virtual machine quote returns other regions on `region_prices`, cheapest found row first. The requested region stays the parent cost. A miss is omitted. A Spot quote uses the Linux Spot row.

**Description**: For a SKU, query the Retail Prices API across regions and return the
prices sorted, so a user sees that one region costs more than another. The issue prefers
one broad query without a region filter. The open question is how this is exposed
through the RPCs. Read the issue body and the spec. If the spec has no place for a
multi-region result, stop at that point and list it under decisions needed.

**Files**: `internal/pricing/` and `internal/azureclient/`, tests and fixtures.

**Acceptance Criteria**: a sorted multi-region result for a VM SKU from fixtures,
handling regions with no price; the exposure decision recorded.

**Linked issues**: [#47](https://github.com/rshade/finfocus-plugin-azure-public/issues/47)

#### AZ-2.16 — Carbon footprint spike [Issue #56]

**Status:** DONE, markdownlint docs/findings/carbon.md, exit 0, no findings

**Description**: A research spike on carbon footprint data sources for Azure that need
no authentication. The Azure Carbon Optimization API needs authentication and is
excluded. Evaluate the Cloud Carbon Footprint methodology (can it estimate from VM
specs and region, without usage metrics?) and static regional carbon intensity data
(is there public per-region data, and how often does it change?). Use real sources and
cite them.

**Deliverable**: a findings document with sources and a recommendation. It does not
implement carbon estimation. That would be a separate issue.

**Acceptance Criteria**: each research question in the issue answered with a cited
source or marked unanswerable.

**Linked issues**: [#56](https://github.com/rshade/finfocus-plugin-azure-public/issues/56)

---

### Phase 3: Quality Gates & Release Infrastructure (AZ-3.x)

This phase is the release-plan quality-gate list. AZ-3.1 and AZ-3.2 are DONE in the status lines below. The January plan in `IMPLEMENTATION_SUMMARY.md` uses Phase 3 for the caching layer, which is a different list.

**Rationale**: Ensure production readiness and establish release pipeline.

**Completion**: 2026-09-30 — mise.toml, goreleaser, workflows, Makefile targets all in place

#### AZ-3.1 — Run Full Linting & Fix Findings

**Status:** DONE, `golangci-lint run ./...`, exit 0, `0 issues.` The one golines finding in `internal/pricing/chaos_test.go` was a wrapped call.

**Description**: Execute `make lint` and resolve all golangci-lint findings to ensure code quality. Address any issues from the upgrade to Go 1.27.1 and spec v0.7.0.

**Files**: (all .go files, if violations found)

**Acceptance Check**:
```bash
make lint
echo "Exit code: $?"  # Must be 0
golangci-lint run --timeout=10m ./... | grep -i error | wc -l  # Should be 0
```

**Linked issues**: (infrastructure, no GitHub issue)

---

#### AZ-3.2 — Add Docstring Coverage (≥80% per Constitution)

**Status:** DONE, `python3 -c 'exec("import pathlib,re\nd=re.compile(r\"^(func|type) ([A-Z][A-Za-z0-9_]*)\")\nc=t=0\nfor b in (\"cmd\",\"internal\"):\n  for p in pathlib.Path(b).rglob(\"*.go\"):\n    if p.name.endswith(\"_test.go\"): continue\n    ls=p.read_text().splitlines()\n    for i,l in enumerate(ls):\n      m=d.match(l)\n      if not m: continue\n      t+=1\n      j=i-1\n      while j>=0 and ls[j].strip()==\"\": j-=1\n      if j>=0 and ls[j].lstrip().startswith(\"//\") and ls[j].lstrip()[2:].lstrip().startswith(m.group(2)): c+=1\nprint(\"%d/%d = %.2f%%\"%(c,t,(100*c/t if t else 0)))")'`, 43/43 = 100.00%

**Description**: Ensure all exported types, functions, and packages have godoc comments per constitution Section IV (Docstring Coverage Enforcement). Run `go doc` checks or similar tooling.

**Files**: (all .go files lacking godoc)

**Acceptance Check**:
```bash
# Manual or tool-based coverage check (constitution requires ≥80%)
# Placeholder: verify no exported types/funcs lack comments
go doc -all ./internal/pricing | grep "^[A-Z]" | wc -l
# All should have comments in generated doc
```

**Linked issues**: (infrastructure, no GitHub issue)

---

#### AZ-3.3 — Create goreleaser Configuration

**Status:** DONE, `make goreleaser-check`, exit 0, configuration is valid

**Description**: Create `.goreleaser.yaml` to automate binary builds for Linux/amd64, Darwin/amd64, and Darwin/arm64. Reference AWS plugin's `.goreleaser.yaml` for structure. Include version injection via ldflags.

**Files**:
- `.goreleaser.yaml` (new, ~40 lines)
- `.gitignore` (add `dist/` if not present)

**Acceptance Check**:
```bash
cat .goreleaser.yaml | grep -E "binary:|platforms:|ldflags:"
# Verify contains build matrix and version flag

goreleaser build --snapshot
ls dist/finfocus-plugin-azure-public-linux-amd64/finfocus-plugin-azure-public
# Binary should exist and be executable
```

**Linked issues**: (infrastructure, no GitHub issue)

---

#### AZ-3.4 — Add Release Workflow (GitHub Actions)

**Status:** DONE, `grep -E 'types: \[created\]|workflow_dispatch:|release --clean' .github/workflows/release.yml`, release created and workflow_dispatch run goreleaser release --clean

**Description**: Create `.github/workflows/release.yml` to build and publish binaries to GitHub Releases on git tag. Pattern from AWS plugin. Include:
- Trigger on `v*.*.*` tags
- Run goreleaser
- Publish to GitHub Releases
- Generate checksums

**Files**:
- `.github/workflows/release.yml` (new, ~40 lines)

**Acceptance Check**:
```bash
cat .github/workflows/release.yml | grep -E "on:.*tag|goreleaser|upload-release-asset"
# Verify contains tag trigger and release step

gh workflow list --repo rshade/finfocus-plugin-azure-public | grep release
```

**Linked issues**: (infrastructure, no GitHub issue)

---

#### AZ-3.5 — Add Release-Please Configuration

**Status:** DONE. `initial-version` is `0.1.0`. `.release-please-manifest.json` `"."` stays `0.0.0` until that release merges. A manifest of `0.1.0` before the first release would record 0.1.0 as already shipped.

**Description**: Create `release-please-config.json` and `.release-please-manifest.json` to automate semver bumping and CHANGELOG generation (pattern from AWS plugin). Enables one-click releases via GitHub UI.

**Files**:
- `release-please-config.json` (new, ~20 lines)
- `.release-please-manifest.json` (stays `{".": "0.0.0"}` until the 0.1.0 release merges)

**Acceptance Check**:
```bash
cat release-please-config.json | jq '.packages."."'
# Verify contains plugin name and version config

gh workflow list --repo rshade/finfocus-plugin-azure-public | grep release-please || \
  gh workflow enable release-please --repo rshade/finfocus-plugin-azure-public
```

**Linked issues**: (infrastructure, no GitHub issue)

---

#### AZ-3.6 — Add Release-Please Workflow (GitHub Actions)

**Status:** DONE, `grep -E 'push:|main|release-please-action' .github/workflows/release-please.yml`, push to main and workflow_dispatch run release-please-action@v5.0.0

**Description**: Create `.github/workflows/release-please.yml` to run release-please bot on main branch. Automatically creates Release PRs and merges on approval. Reference AWS plugin.

**Files**:
- `.github/workflows/release-please.yml` (new, ~25 lines)

**Acceptance Check**:
```bash
cat .github/workflows/release-please.yml | grep -E "on:.*push.*main|release-please|pull-request-header"
# Verify contains main branch trigger and release-please step
```

**Linked issues**: (infrastructure, no GitHub issue)

---

#### AZ-3.7 — Regression test suite with golden pricing data [Issue #52]

**Status:** DONE, `go test -count=1 -run 'TestGolden$|TestGoldenLiveSnapshots|TestGoldenLiveToleranceBounds' ./internal/pricing/` passed. `TestUpdateGoldenFromLiveAPI` skips unless `-tags=integration -update-golden`. The live snapshots, captured 2026-10-02, are Standard_B1s eastus 7.592, Standard_D2s_v3 eastus 70.08, Standard_LRS 128 GB 5.888, and Premium_SSD_LRS 256 GB 38.012142.

**Description**: Golden-file tests: recorded Azure API responses as inputs and expected
`EstimateCost` outputs as snapshots, with a documented update procedure for when prices
change. The fixtures recorded in AZ-2.3 to AZ-2.8 are the natural starting set. Put them
under one layout (for example `testdata/golden/` and `testdata/expected/`) with a README
on how to refresh them.

**Files**: `testdata/`, `internal/pricing/` tests.

**Acceptance Criteria**: every resource type in scope has at least one golden case; a
deliberate change to a price or a formula makes a golden test fail; the update procedure
is documented and works.

**Linked issues**: [#52](https://github.com/rshade/finfocus-plugin-azure-public/issues/52)

#### AZ-3.8 — Pricing accuracy validation against the Azure Pricing Calculator [Issue #53]

**Status:** BLOCKED-ON-INPUT, go test -count=1 ./internal/pricing/ -run TestCalculatorAccuracy, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.013s

**Description**: Test cases that compare the plugin's estimates with the Azure Pricing
Calculator, within plus or minus 5%. **The calculator is a web application and its
values cannot be fetched by the agent.** Build the harness and the table of cases, leave
the calculator values as clearly marked inputs the owner supplies, and skip those cases
with a message until they are filled in. **Never invent a calculator value.**

**Files**: `internal/pricing/` tests.

**Acceptance Criteria**: the harness runs, skips unfilled cases with a clear message, and
fails a case whose estimate is outside the tolerance when a value is supplied.
**Status** for this task is `BLOCKED-ON-INPUT` until the owner supplies values.

**Linked issues**: [#53](https://github.com/rshade/finfocus-plugin-azure-public/issues/53)

#### AZ-3.9 — Performance benchmarking and load testing [Issue #54]

**Status:** DONE via AZ-6.6. `TestCacheHitRateOverGRPC` uses a real gRPC server. 128 hits, 1 miss, ratio 0.992248.

**Description**: The load test is `TestCacheHitRateOverGRPC`: a real gRPC server and client, concurrent calls, and a cache hit rate higher than 80%. The recorded run is 128 hits, 1 miss, ratio 0.992248. Benchmarks for the estimation path live beside it.

**Files**: `internal/pricing/` and `internal/azureclient/` benchmark and load tests.

**Acceptance Criteria**: `go test -bench` runs; the load test asserts the cache hit rate;
baseline numbers recorded with the machine noted; nothing flaky.

**Linked issues**: [#54](https://github.com/rshade/finfocus-plugin-azure-public/issues/54)

#### AZ-3.10 — Chaos testing for Azure API failures [Issue #55]

**Status:** DONE, go test -count=1 ./internal/pricing/ -run TestChaos, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.429s

**Description**: Fault injection against a mock HTTP server: timeouts, rate limiting with
`Retry-After`, server errors, malformed responses, and a partly failing sequence. Verify
the retry logic and that failures surface as the right gRPC status codes, never a
silent zero cost. Make the timeouts short so the suite stays fast.

**Files**: `internal/azureclient/` and `internal/pricing/` tests.

**Acceptance Criteria**: each failure scenario asserts both the retry behaviour and the
gRPC code; the suite runs in seconds; tests pass.

**Linked issues**: [#55](https://github.com/rshade/finfocus-plugin-azure-public/issues/55)

---

### Phase 4: Registry & Core Integration (AZ-4.x)

**Rationale**: Enable discovery and installation from finfocus core.

#### AZ-4.1 — Register Plugin in finfocus Core Registry

**Status:** DONE, registry entry recorded in the task report, ../finfocus not edited

**Description**: Add Azure plugin entry to `../finfocus/internal/registry/registry.json` (read-only in this repo, added via core PR). This step is performed by a core maintainer or via a PR to the main finfocus repo.

**Note**: This task is a handoff. The Azure plugin repo needs to:
1. Create v0.1.0 release (via AZ-3.x + AZ-5.1)
2. File a PR in finfocus core to register entry

**Registration entry** (example):
```json
"azure-public": {
  "name": "azure-public",
  "description": "Azure public pricing data",
  "repository": "rshade/finfocus-plugin-azure-public",
  "author": "FinFocus Team",
  "license": "Apache-2.0",
  "homepage": "https://github.com/rshade/finfocus-plugin-azure-public",
  "supported_providers": ["azure"],
  "capabilities": ["cost_projection"],
  "security_level": "official",
  "min_spec_version": "0.7.0",
  "asset_hints": {
    "asset_prefix": "finfocus-plugin-azure-public"
  }
}
```

The example `min_spec_version` is `0.7.0`.

**Files**: `../finfocus/internal/registry/registry.json` (external to this repo)

**Acceptance Check**:
```bash
cd ../finfocus
grep -A 10 '"azure-public"' internal/registry/registry.json | grep "asset_prefix"
# Entry should be present in merged main

finfocus plugin list | grep azure
# CLI should discover azure-public plugin
```

**Linked issues**: (infrastructure, depends on AZ-5.1)

---

### Phase 5: Create Initial Release (AZ-5.x)

**Rationale**: Publish v0.1.0 as the first stable release.

#### AZ-5.1 — Tag and Release v0.1.0

**Status:** SKIPPED, release tag is outside this run

**Description**: Create annotated git tag `v0.1.0` and push to GitHub. GitHub Actions (`release.yml`) automatically builds binaries and publishes to Releases. This marks the official stable point for the mapped resource types and the working `GetProjectedCost` and `GetActualCost` calls. The tag itself stays outside this run.

**Files**: (git tag only)

**Acceptance Check**:
```bash
git tag v0.1.0 -m "Release v0.1.0: Azure plugin with VM, Disk, and Storage support"
git push origin v0.1.0

# Wait for GitHub Actions to complete
gh release view v0.1.0 --repo rshade/finfocus-plugin-azure-public

# Verify binaries exist
gh release view v0.1.0 --repo rshade/finfocus-plugin-azure-public --json assets \
  | jq '.assets[] | .name' | grep -E "linux.*amd64|darwin"
```

**Linked issues**: (milestone marker)

---

#### AZ-5.2 — Update CHANGELOG.md for v0.1.0

**Status:** DONE, markdownlint CHANGELOG.md, exit 0

**Description**: Document initial release in CHANGELOG.md using Keep a Changelog format. Release-please typically auto-generates this, but manual review/addition may be needed.

**Files**: `CHANGELOG.md` (create or append)

**Acceptance Check**:
```bash
head -30 CHANGELOG.md | grep -E "^## \[0.1.0\]|### Added|### Features"
# Verify section exists with meaningful entries
```

**Linked issues**: (infrastructure, no GitHub issue)

---

### Phase 6: Run 2 carryover (AZ-6.x)

**Rationale**: Run 1 (2026-09-30) was audited on 2026-10-01 against the code and the live
Azure Retail Prices API. Pricing for the resource types was confirmed correct. The items
below are what it did not deliver, or delivered less than its task text. Process rules
for the run (commit per task, spec-gap issues, the Not delivered register) are in
`superpowers-prompt.md`.

#### AZ-6.1 — Checkpoint, run branch, and a re-verifiable AZ-1.1

**Status:** DONE, `git rev-parse --short HEAD` of the checkpoint is `0d26960` on `run/grok-20261001`, and `go list -m -f '{{.GoVersion}}'` prints `1.27.1`. Break check: `head -2 go.mod | grep "go 1.27.1"` exits 1. `main` stays at `8d369ef`.

**Description**: Create the run branch and the checkpoint commit. Re-verify the Go and
spec versions with a command that can pass, such as
`go list -m -f '{{.GoVersion}}'` and `grep finfocus-spec go.mod`.

**Acceptance Criteria**: the checkpoint commit exists, AZ-1.1 carries an honest status.

#### AZ-6.2 — Spot pricing: finish the edges [Issue #42]

**Status:** DONE, `go test -count=1 -run 'TestEstimateCostSpotD2sV3Eastus|TestGetProjectedCostSpotVMFromFixture|TestOracleComparison$' ./internal/pricing/` passed. Spot `EstimateCost` for `Standard_D2s_v3` eastus is 13.73568 per month with category Dynamic. An empty priority stays on demand. `LowPriority` is InvalidArgument. Break check: the test failed on the on-demand total 70.08 before `selectVMItem` received the Spot flag.

**Description**: Spot is reachable today through `GetProjectedCost` with the resource tag
`priority=Spot` (`descriptorSpot` in `spot.go`), and `selectVMItem` picks the Linux Spot row
correctly. Audit on 2026-10-01 found the edges missing: `EstimateCost` and the regional
comparison always pass `spot=false`, and nothing documents the tag. Confirm the response sets
`pricing_category` to Spot for a spot quote (check the spec). Honour the tag in `EstimateCost`,
add it to `Supports` and the pricing spec documentation, and document it in `README.md`.

**Acceptance Criteria**: a gRPC test with `priority=Spot` gets the live-verified spot price
(`Standard_D2s_v3`, eastus: 13.74 per month, from the oracle), the default request still gets
on-demand, `EstimateCost` agrees, and an unknown priority value is `InvalidArgument`.

#### AZ-6.3 — Parse Savings Plans and fix reservation handling [Issue #57]

**Description**: Savings Plan prices appear in a nested `savingsPlan` array only when
`api-version=2023-01-01-preview` is used. Filtering `priceType eq 'SavingsPlan'` returns
nothing. Parse the array in `internal/azureclient`, capture a real response as a fixture,
and compute savings fractions from hourly prices. Reservation `retailPrice` is a total
for the term even though `unitOfMeasure` says "1 Hour": divide by 8,760 for one year or
26,280 for three before comparing.

**Acceptance Criteria**: a test reads the parsed values from the live-captured fixture
and fails if the array is ignored (break check). The findings document states which
parts need the spec change in AZ-6.4.

**Status:** DONE, `go test -count=1 -run 'TestPriceItemParsesSavingsPlanFixture|TestReservationHourlyUsesTermTotal' ./internal/estimation/` passed. `PriceItem` keeps the nested array. `ReservationHourly` divides the term total by 8760 or 26280. Break check: the test failed to compile (`SavingsPlan` undefined, `ReservationHourly` undefined) before the field and function existed. The repeated alternative-price list landed after this task. A virtual machine quote returns it on `price_options`. See AZ-2.13.

#### AZ-6.4 — File the spec issues for multi-pricing and regional comparison [Issues #45, #47]

**Description**: `GetProjectedCostResponse` and `EstimateCostResponse` have no repeated
field for alternative prices or per-region prices. Prove it, check for existing issues,
and file them in `finfocus-spec` per section 4b: a `repeated PriceOption` (category,
model, term, unit price, monthly cost, upfront cost, savings fraction) and a
`repeated RegionPrice` (region, unit price, monthly cost, currency), additive and
advisory, never summed into the primary cost.

**Acceptance Criteria**: two issue URLs (or links to existing issues) in the status line
and in the Not delivered register. The pure functions stay.

**Status:** DONE, spec v0.7.0 `costsource.proto` has one `unit_price` (line 371) and one `cost_per_month` (line 375) on `GetProjectedCostResponse`, and one `cost_monthly` (line 1233) on `EstimateCostResponse`. No `PriceOption` or `RegionPrice` message exists. Issue search before filing found no match. Filed [spec issue 588](https://github.com/rshade/finfocus-spec/issues/588) and [spec issue 589](https://github.com/rshade/finfocus-spec/issues/589). Break check: a matching open issue would have been linked instead of filing a new one. The fields later landed on the spec. Virtual machine quotes fill `price_options` and `region_prices`. See AZ-2.13 and AZ-2.15.

#### AZ-6.5 — FOCUS record in production [Issue #46]

**Description**: `ValidateFocusRecord` rejects an empty `billing_account_id`, so run 1
emits no record. Find how a billing account id can reach the plugin (a request tag or
plugin configuration) and emit a validated record when it is given. If the spec has no
carrier, file the issue. Do not invent an account id.

**Acceptance Criteria**: a gRPC test with an id returns a record that passes
`ValidateFocusRecord`, and a test without one proves the documented fallback.

**Status:** DONE, `go test -count=1 -run 'TestGetActualCostFocusRecordOverGRPC|TestGetActualCostOmitsFocusRecordWithoutAccountOverGRPC' ./internal/pricing/` passed. `SetBillingAccountID` feeds the record. The environment variable `FINFOCUS_BILLING_ACCOUNT_ID` sets it in the process. Empty leaves FocusRecord nil and logs why. Break check: the test failed to compile (`SetBillingAccountID` undefined) before the setter existed. Request `billing_account_id` now wins over the process setting. A dry run ignores it. [Spec issue 590](https://github.com/rshade/finfocus-spec/issues/590) is closed. No account id is invented. Issue #46 stays open because `charge_type` is absent.

#### AZ-6.6 — A real gRPC load test [Issue #54]

**Description**: Start a gRPC server and drive it with a client from concurrent
goroutines. Assert the cache hit rate and that no request fails.

**Acceptance Criteria**: the test goes through the network stack, and a break check
(for example disabling the cache) makes it fail.

**Status:** DONE, `go test -count=1 -run 'TestCacheHitRateOverGRPC$' ./internal/pricing/` passed. 128 hits, 1 miss, ratio 0.992248. No request failed. Break check: cache TTL 0 recorded 0 hits and 129 misses and the test failed, then the hour TTL was restored.

#### AZ-6.7 — EstimateCost and DryRun for every type [Issue #43]

**Description**: `EstimateCost` covers VMs and managed disks only. Route it for the other
resource types, or return `Unimplemented` naming the task id and list the types in the
register. Run the gRPC `DryRun` test over every supported type, not only Cosmos.

**Acceptance Criteria**: a table-driven gRPC test covers every type in `mapper.go`.

**Status:** DONE, `go test -count=1 -run 'TestEstimateCostEveryMappedTypeOverGRPC|TestDryRunOverGRPCDoesNotCallHTTP' ./internal/pricing/` passed. Every mapped type is priced or returns NotFound from an empty price list, and DryRun over gRPC covers each type. Break check: blob `EstimateCost` was Unimplemented before the shared quote.

#### AZ-6.8 — Cosmos serverless storage and the AKS Free tier [Issues #51, #49]

**Description**: Serverless Cosmos accounts with data are under-quoted because the storage
meter is excluded, and live serverless has no storage meter. Say so in the quote notes
and the register. A live `FreeTierInfrastructureCost Uptime SLA` AKS meter at 0.05 per
hour is effective from 2026-10-01: confirm it against the live API and handle it.

**Acceptance Criteria**: live queries pasted, tests updated, behaviour documented.

**Status:** DONE, `go test -count=1 -run 'TestGetProjectedCostCosmosServerlessOmitsStorageMeter|TestGetProjectedCostAKSFreeOpenMeter' ./internal/pricing/` passed. Live AKS filter on service `Azure Kubernetes Service` and meter `FreeTierInfrastructureCost Uptime SLA` returned one open row at 0.05 USD per hour, effective 2026-10-01, month 36.50. Live Cosmos filter on that request-unit product returned only meter `1M RUs` at 0.25 USD per `1M`. The quote note says that product publishes no storage meter. Not delivered: a storage component for that product, because the live API has none. Superseded for AKS by PR #70 (2026-10-02): the Free control plane is now 0 with a note, because the published AKS pricing page and the Pricing Calculator show no Free-tier charge; the meter is reported, not billed.

#### AZ-6.9 — Load Balancer (stretch AZ-2.9)

**Description**: Load Balancer pricing is Global-only (an `armRegionName` of eastus returns
nothing). Standard rule-hours meters are 0.025 per hour. Implement a Global-fallback
estimate with the usage assumptions stated, or record why not.

**Acceptance Criteria**: live-verified query, an honest status, no silent zero.

**Status:** DONE, `go test -count=1 -run 'TestGetProjectedCostLoadBalancer|TestGetPricingSpecEverySupportedType' ./internal/pricing/` passed. Live filter `serviceName eq 'Load Balancer' and armRegionName eq 'eastus'` returned no rows. The same service filter at price region `Global` returned the Standard included rules meter at 0.025 USD per hour. Omitted `rule_count` bills that meter once, 18.25 for 730 hours, and the quote says so. `rule_count` 0 has no hourly charge. Gateway is `InvalidArgument`. Not delivered: Gateway and cross-region meters. Break check: skipping the `Global` retry made the regional case `NotFound`.

#### AZ-6.10 — Test hygiene

**Description**: remove the stale "GetProjectedCost not implemented yet" skip in
`calculator_test.go`; document the two undocumented exported helpers in
`internal/azureclient/cache_test.go`; make every skipped accuracy test state which
owner value is missing.

**Acceptance Criteria**: `go test -v ./... | grep SKIP` shows only skips with a stated
input requirement or `-short`.

**Status:** DONE, `go test -count=1 -v ./...` passed. The only skips are the nine accuracy cases. Each says `owner value not supplied: owner_monthly_usd` and names the case. `TestProjectedCostSupported` prices a virtual machine. Break check: a Windows product name made that test `NotFound`. `newTestClient` and `newTestCachedClient` document the clients they build.

#### AZ-6.11 — Documentation matches the code

**Description**: update `ROADMAP.md`, `CONTEXT.md`, `IMPLEMENTATION_SUMMARY.md`,
`README.md` and the `TASKS.md` text that contradicts the plugin. State that
`GetActualCost` means a running-cost estimate from public retail prices, not billed
spend.

**Acceptance Criteria**: markdownlint clean, and no document claims a feature the code
lacks or denies one it has.

**Status:** DONE, the Markdown check on the touched guides exited 0. The old current-state table said only three resource types and said `GetProjectedCost` falls through to Unimplemented. That table now lists the ten mapped types, and `GetActualCost` is the monthly quote times hours over 730. Break check: the three-type sentence is gone.

#### AZ-6.12 — Calculator accuracy values [Issue #53]

**Description**: the owner reads values from the Azure Pricing Calculator into
`internal/pricing/testdata/oracle/calculator-values.csv`. Make `TestCalculatorAccuracy` read
that file (case id to `owner_monthly_usd`) so the 9 skipped cases and the oracle cases that
have a value run against it, within 5%. A row with an empty value is skipped with a reason that
names the case. Never invent or copy a value, and never fill the file yourself.

**Acceptance Criteria**: with the file as supplied by the owner, every filled row is compared;
the report lists which rows were empty. Status is BLOCKED-ON-INPUT for any type with no filled
row.

**Status:** BLOCKED-ON-INPUT, `go test -count=1 -run 'TestCalculatorAccuracy$|TestParseCalculatorOwnerValues' ./internal/pricing/` passed. All 22 rows in `calculator-values.csv` have an empty `owner_monthly_usd`. Each skip names the case and `owner_monthly_usd`. The file was not edited. Break check: parsing an empty cell as 0 failed `TestParseCalculatorOwnerValues`.

#### AZ-6.13 — Whole-branch review and report

**Description**: review the whole run branch, fix Critical and Important findings in new
commits, then write `superpowers-run-report.md` with the Not delivered register.

**Acceptance Criteria**: report follows section 8 of the prompt.

**Status:** DONE, `go test -count=1 -run 'TestWindowsVMQuoteIsNotLinuxPrice|TestEstimateCostNativeFunctionWebApp|TestFocusNativeFunctionWebApp|TestGetPricingSpecEverySupportedType$' ./internal/pricing/` passed after the branch review. `superpowers-run-report.md` follows section 8. The ambiguous oracle judge was left as its instructions require. Break check: `TestWindowsVMQuoteIsNotLinuxPrice` returned a Linux month before that token was rejected.

#### AZ-6.14 — Oracle comparison gate (run this right after AZ-6.1)

**Status:** DONE, `go test -count=1 -run TestOracleComparison ./internal/pricing/` passed, 51 cases in `.superpowers/oracle-results.md`. Break check: doubling `unit * pluginsdk.HoursPerMonth` in the VM quote failed 19 cases, then restored. `git diff -- internal/pricing/testdata/oracle/` empty. Ambiguous AKS free chose 36.50. Ambiguous SQL zone redundancy returned NotFound because the fixture has no base compute row. Spot `EstimateCost` was fixed in AZ-6.2: an empty priority stays on demand, priority Spot is the Linux Spot row, the category is Dynamic, and the score stays 0.

**Description**: `internal/pricing/testdata/oracle/` holds an independent expected-price table
(about 50 cases, derived from the live Retail Prices API by a script that never read this
plugin). Write the comparison test that its `README.md` specifies: offline against the recorded
rows, a real gRPC server, every case. Run it before any other Phase 6 task, because its failures
are the plugin's real pricing bugs and should drive the order of the rest. Known candidates:
blob storage ignores volume bands (the 60,000 GB case), the zone-redundant SQL and Free-tier AKS
cases, `S10 ZRS` and the unknown SKU must return errors, and the VM cases across regions.

**Acceptance Criteria**: a results table (case, plugin, expected, difference, verdict) in
`.superpowers/oracle-results.md` and in the report. Every `ok` case is within tolerance, or is a
named finding with a fix commit or a Not delivered entry. The oracle files are unchanged
(`git diff` on them is empty). Show a break check: change one multiplier and watch cases fail.

#### AZ-6.15 — End to end through core with a real Pulumi plan

**Description**: Prove the whole path: Pulumi plan JSON, core, plugin, price. Build the plugin and
core into a temporary directory (`go build -o "$TMPDIR/..."`, nothing written inside `../finfocus`),
install the plugin under a temporary `FINFOCUS_HOME`, and run
`finfocus cost projected --pulumi-json <plan>` on a plan fixture kept in this repository, with
one resource per supported type. Use the real Pulumi type tokens a user has, both the
`azure-native:` and the classic `azure:` forms, and report which tokens core passes and which the
plugin's `Supports` accepts. Read `../finfocus/test/e2e/azure_test.go` first. Compare each
resource's cost with the oracle.

**Acceptance Criteria**: pasted CLI output per resource, a table of token forms accepted and
rejected, and every rejected real token either fixed or in the Not delivered register.

**Status:** DONE, `finfocus cost projected --pulumi-json testdata/pulumi/azure-plan.json --output json` exited 0. Both token families priced the ten types. The monthly total was 994.51764 USD. A scale set and a NAT gateway were declined. Break check: `TestSupportsRealPulumiTokens` failed on `unsupported provider: azure-native` before that provider was accepted.

### Phase 7: Real Pulumi inputs and review fixes (AZ-7.x), run 3

**Rationale**: the 2026-10-02 review of runs 1 and 2 found the pricing correct but the input
names invented. A genuine `pulumi preview --json` for both providers shows the plugin cannot price
most real programs. Evidence and the gap table are in `internal/pricing/testdata/pulumi-real/`
(`README.md`, `gap-table.md`, `pulumi-property-map.json`, `core-view.json`, `plan-expected.json`).
Property names must come from there (rule 25). Each type task is finished and committed before the
next.

#### AZ-7.1 — Real-plan comparison harness (run first)

**Status:** DONE, `go test -count=1 -timeout 120s ./internal/pricing/ -run 'TestRealPulumiPlan$|TestRealPlanDottedVMSize'` passed. `azure/legacyVm` matches both input sets and is the ratchet. Break check: `descriptorSKU` ignored `Sku` and renamed `vmSize` to `instanceSize`; both legacy VM rows failed with missing sku, then the function was restored.

**Description**: Write the test specified in `testdata/pulumi-real/README.md`: for every resource in
the genuine previews, build a request from `core-view.json` (today) and from
`core-view-proposed.json` (dotted nested tags and a per-type SKU source, what the core would send after
the fix), call `GetProjectedCost` through a real gRPC server (offline price
rows), and compare with `plan-expected.json`. Its failures set the order of AZ-7.2 to AZ-7.9.

**Acceptance Criteria**: a results table in `.superpowers/real-plan-results.md` and the report, per
input set. A failing resource is a named finding with a fix commit or a Not delivered entry. A break
check: change one real property name in the plugin and watch a resource fail.

#### AZ-7.2 — Virtual machines

**Description**: Classic `size` (and `vmSize` for the legacy type), `location`, and `priority`
including the provider default `Regular` (on-demand, never an error). Native `hardwareProfile.vmSize`
(today the core sends the bare tag `hardwareProfile=<size>`, and `hardwareProfile.vmSize` once
flattened). Windows: classic `windowsVirtualMachine`, native `osProfile.windowsConfiguration`, legacy
`osProfileWindowsConfig`; `licenseType` `Windows_Server` or `Windows_Client` is Hybrid Benefit and
prices at the base rate with a note, no `licenseType` uses the Windows meter. Scale sets: classic
`sku` and `instances`, native `sku.name` and `sku.capacity`, multiplied. Spot through `priority`.

**Acceptance Criteria**: every VM and scale set row of `plan-expected.json` passes for both input
sets, and a native Windows VM is never priced on the Linux meter without the Hybrid Benefit note.

**Status:** The VM rows and the classic scale set pass both input sets.
`go test -count=1 -timeout 180s ./internal/pricing/` passed.
`azure-native/vmss` does not match `plan-expected.json`. The core input prices one
on-demand instance (83.95, want 251.85) because that view has no `sku.capacity`.
The dotted input is `NotFound`: `virtualMachineProfile.priority` is `Spot` and the
fixture rows are the on-demand meter only. Break check: renaming the `size` tag to
`instanceSize` made `azure/linuxVm`, `azure/linuxVmRegular`, and `azure/windowsVm`
fail with missing sku. The tag was restored and the package test passed again.

#### AZ-7.3 — Managed disks

**Description**: Classic `storageAccountType` and `diskSizeGb`; native `sku.name` and `diskSizeGB`
(capital GB). Accept the real ARM names `Premium_LRS`, `StandardSSD_LRS`, `Standard_LRS`, and the
ZRS forms; return an explicit unsupported error for Ultra and Premium v2 unless priced. Map size to
tier (256 GiB Premium is P15).

**Acceptance Criteria**: the disk rows pass; the old invented names are no longer the only accepted
ones.

**Status:** DONE in #69. `azure/disk` and `azure-native/disk` pass both input sets and are in the
ratchet. Break check: renaming `storageAccountType` made both `azure/disk` rows fail with missing
`disk_type`.

#### AZ-7.4 — Storage accounts

**Description**: Classic `accountTier`, `accountReplicationType`, `accessTier` (default Hot); native
`sku.name` such as `Standard_GRS` plus `accessTier`. There is no capacity input: state the size
assumed in the response, or return an explicit error (the `usage_required` rule).

**Acceptance Criteria**: the storage rows pass the `usage_required` rule for both input sets.

#### AZ-7.5 — App Service plans

**Description**: Classic `skuName`, `osType` (Linux, Windows, WindowsContainer), `workerCount`;
native `sku.name` (the core sends it as the SKU), `sku.capacity`, `kind`, `reserved`. A plan with
no `kind` and no `reserved` is Windows. Multiply by the worker count.

**Acceptance Criteria**: the plan rows pass; no Windows plan is priced on a Linux meter.

**Status:** Partly done in #69. Classic `skuName`, `osType`, and `workerCount`, and native
`sku.capacity`, are read. `azure/plan`, `azure/winPlan`, and the dotted `azure-native/plan` pass.
Open: native `kind` and `reserved` (a native plan with neither is still priced as Linux), and the
core view of `azure-native/plan`, which has no capacity.

#### AZ-7.6 — AKS

**Description**: Classic `skuTier` and `defaultNodePool` (`vmSize`, `nodeCount`); native `sku.tier`
(not `sku.name`, which is `Base` or `Automatic`) and `agentPoolProfiles[]`. Today the core flattens
the pools to a bare name, so node pools need the dotted form: price the control plane and say that
node pools were not included when the data is missing. Node pool child resources are
`needs_parent_resource`. Free tier: done in PR #70. The control plane is 0, and the response names the
live `FreeTierInfrastructureCost` meter as not billed, because the published
pricing page and the Pricing Calculator show no Free-tier charge.

**Acceptance Criteria**: the AKS rows pass; the register has the Free tier assumption.

#### AZ-7.7 — SQL Database

**Description**: Classic `skuName` (`GP_Gen5_4`), `maxSizeGb`, `zoneRedundant`, `licenseType`, and no
`location` (join from `serverId`: `needs_parent_resource`); native `sku.name` and `sku.capacity`
(today lost), `maxSizeBytes`. Serverless `GP_S_*` returns an explicit unsupported error, never the
provisioned price. Zone redundancy is settled by #77: the zone vCore meter is a surcharge on compute,
and zone storage replaces local storage. `licenseType: BasePrice` is ambiguous in the Retail API: take
a documented reading and record it.

**Acceptance Criteria**: the SQL rows pass; the ambiguity is in the register with both readings.

#### AZ-7.8 — Cosmos DB

**Description**: Native token `azure-native:cosmosdb:DatabaseAccount` (the plugin matches only
`documentdb`); classic `cosmosdb/account`. Throughput lives on child resources
(`options.throughput`, `autoscaleSettings.maxThroughput`, classic `throughput`): the account is
`usage_required`, the children `needs_parent_resource`. Multi-region and multi-write multiply cost:
state it.

**Acceptance Criteria**: the Cosmos rows pass for both providers.

#### AZ-7.9 — Function apps and web apps

**Description**: Classic `appservice/linuxFunctionApp`, `windowsFunctionApp`, `linuxWebApp`,
`windowsWebApp`; native `web:WebApp` with `kind` containing `functionapp` or `app`. Their cost is on
the plan referenced by `servicePlanId` or `serverFarmId`. Return an explicit error that names the
plan, never zero and never an "unsupported" that hides the reason.

**Acceptance Criteria**: all function and web app rows pass the `needs_parent_resource` rule.

#### AZ-7.10 — Check the filed core issues

**Description**: The core changes are filed in `rshade/finfocus`: #1608 (additive dotted tag keys for
nested inputs), #1609 (per-type Azure SKU source) and #1610 (cross-resource references from
`propertyDependencies`). Do not draft or file them again. As AZ-7.1 to AZ-7.9 show real behaviour, compare it
with what those issues claim and write any correction or missing fact as a draft in
`.superpowers/issue-drafts/` for the owner. Pay attention to the double-counting invariant in #1610 and the
tag cap and secret-leaf rules in #1608.

**Acceptance Criteria**: a list of confirmed claims and discrepancies in the report, each with evidence.

#### AZ-7.11 — Check the spec docs change

**Description**: The spec docs issue is `rshade/finfocus-spec` #609, with the pull request #610 (documents what
the host sends in `tags` today, and marks dotted keys as a recommendation). Do not file anything. Compare the
PR's text with what you observe through `core-view.json` and the plugin tests, and list any discrepancy.

**Acceptance Criteria**: a list of confirmed statements and discrepancies in the report.

#### AZ-7.12 — Review fixes

**Description**: (1) `grpc` to v1.83.2, prove GO-2026-6443 clears with govulncheck before and after,
make the CI govulncheck step blocking; (2) AKS Free tier in the register and the response; (3) remove
the stale `TODO` and comment at `calculator_test.go` lines 258 to 262; (4) the header of this
file says "nine" skips and names the old branch: correct it; (5) `PluginInfo.Name` is
`finfocus-plugin-azure-public` in `main.go` and `azure-public` in `calculator.go`: decide, make them
agree, and test what is served; (6) a misattached doc comment on `isVirtualMachineResourceType`;
(7) `CHANGELOG.md` was hand-edited: Release Please owns it, restore it; (8) refresh `ROADMAP.md`,
`TASKS.md` and `IMPLEMENTATION_SUMMARY.md` for the issues closed on 2026-10-02; (9) the AZ-6.7 test
accepts NotFound on empty rows: make it assert real prices for blob, AKS, Cosmos and App Service.

**Acceptance Criteria**: each item has a commit and a test or command output.

#### AZ-7.13 — Fixture and end to end

**Description**: Replace `testdata/pulumi/azure-plan.json` (invented names) with a plan built from the
genuine previews, and rerun the end-to-end test through core with it. Report which resources core
prices, which return an explicit error, and the totals.

**Acceptance Criteria**: pasted CLI output per resource, matching the AZ-7.1 table.

#### AZ-7.14 — Whole-branch review and report

**Description**: review the whole run branch, fix Critical and Important findings in new commits, then
write `superpowers-run-report.md` with the Not delivered register and the assumptions register.

**Acceptance Criteria**: report follows section 8 of the prompt.

## Issue Index

### Legend

- **roadmap/current**: Critical for v0.1.0 (high priority)
- **roadmap/next**: Post-v0.1.0, immediate follow-up
- **roadmap/future**: Backlog for v0.2+
- **spec-first**: Requires finfocus-spec update
- **effort/small**: 1-2 days | **effort/medium**: 3-5 days | **effort/large**: 5+ days

### Open issues

Closed on `main` by earlier commits: #42, #43, #45, #47, #50, #51, #52, #54, #55, #56, #57, #59, #60, #61.
Issues #48 and #49 are closed by PR #70, which adds live integration tests in
`examples/projected_cost_integration_test.go`.

| # | Title | Why it stays open |
| --- | --- | --- |
| [#53](https://github.com/rshade/finfocus-plugin-azure-public/issues/53) | Pricing accuracy vs the Azure Pricing Calculator | **AZ-3.8** BLOCKED-ON-INPUT. Every `owner_monthly_usd` cell is empty. |
| [#46](https://github.com/rshade/finfocus-plugin-azure-public/issues/46) | FOCUS 1.3 column alignment | **AZ-2.14** emits a record. `charge_type` has no proto field. `commitment_discount_type` stays empty. |
| [#44](https://github.com/rshade/finfocus-plugin-azure-public/issues/44) | GetPricingSpec for plugin discovery | **AZ-2.11** returns one spec for the quoted resource, not a catalog. |

**Summary** (updated 2026-10-03):
- Three issues stay open. The reasons are in the table.
- Standard Load Balancer rules are quoted. NAT Gateway, virtual machine scale
  sets, Cache for Redis, and database servers for PostgreSQL and MySQL have
  no issue and are not priced.

---

## Open Questions

### Specification & Dependencies

1. **v0.7.0 upgrade impact** (answered 2026-10-01): spec v0.7.0 adds a
   `cost_breakdown` map and optional FOCUS 1.4 fields.
   - `cost_breakdown`: field 15 on `GetProjectedCostResponse`. The resource
     quotes populate it, and the parts sum to the monthly cost.
   - **Trace ID**: the SDK logs the host trace id on validation errors. No
     plugin change.
   - **FOCUS 1.4 fields**: a record is attached when the request sets
     `billing_account_id`, or, when that field is empty, when
     `FINFOCUS_BILLING_ACCOUNT_ID` is set. A dry run ignores the request
     id. [Spec issue 590](https://github.com/rshade/finfocus-spec/issues/590)
     is closed.

2. **Go version cascade**: The upgrade from Go 1.25.7 to 1.27.1 (AZ-1.1) is on
   `main`. The Test workflow passed on that tree.

3. **Confidence level encoding** (answered in run 1: kept in `Source`, the v0.7.0 proto has no field for it): Issue #60 references encoding confidence in the
   `Source` field as `"azure-retail-prices[confidence:HIGH]"`. Is this format
   standardized across plugins, or should this be a separate field? Check v0.7.0
   proto for ActualCostResult.confidence_level or similar field.

### Release & Registry

4. **Release assets**: AWS plugin releases include checksums and signatures. Should Azure plugin v0.1.0 include these, or is a simple goreleaser setup with multiple OS/arch binaries sufficient for initial release?

5. **Registry addition timing**: AZ-4.1 (registry entry) depends on a PR to the finfocus core repo. Should this be filed immediately after AZ-5.1 (v0.1.0 tag is pushed), or should an Azure plugin maintainer prepare the PR in parallel?

6. **Backwards compatibility**: This plugin is not in the core registry, so
   there is no installed base to break. The AZ-4.1 example sets
   `min_spec_version` to `0.7.0`. Whether a core build on an older spec
   accepts that field set was not checked in this repository.

### Testing & QA

7. **Live API tests**: Opt-in tests under `examples/` call the public Retail Prices API with `-tags=integration`. They are not a CI gate. Unit tests keep using recorded pages.

8. **Docstring coverage enforcement**: Constitution requires ≥80% docstring coverage. What tool/script should be used to verify this in CI? (AZ-3.2)

### Resource Expansion

9. **Stretch types (AZ-2.9)**: the parity types with `aws-public` (Load Balancer, NAT
   Gateway, Virtual Machine Scale Sets, Redis, PostgreSQL and MySQL) have no issue yet.
   The owner has not yet confirmed which, if any, are required for v0.1.0. Until then
   they are a stretch after everything else is verified.
10. **Calculator values for #53 (AZ-3.8)**: the Azure Pricing Calculator is a web
    application. The owner needs to supply the expected monthly costs for the
    sample configurations. Until then AZ-3.8 stays BLOCKED-ON-INPUT.
11. **Spec-first issues (#44, #46)**: #45 and #47 are delivered on
    `price_options` and `region_prices`. #43 is resolved and closed:
    `DryRunResponse` has no filter field by spec design, so the filter is
    logged. #44 returns one pricing spec. #46 still has no `charge_type`
    field.

---

## Verification Commands

### Build & Test Verification

```bash
# Phase 1 (dependencies)
go mod verify
go build ./...

# Phase 2 (RPC implementation)
go test -v -run TestGetProjectedCost ./internal/pricing/...
go test -v -run TestGetActualCost ./internal/pricing/...

# Phase 3 (quality gates)
make lint
go test -race ./...

# Phase 5 (release)
git tag v0.1.0
gh release view v0.1.0 --repo rshade/finfocus-plugin-azure-public
```

### Runtime Verification (after binary build)

```bash
# Start plugin
./finfocus-plugin-azure-public &
PLUGIN_PID=$!

# Test Supports RPC
grpcurl -plaintext -d '{
  "resource": {
    "provider": "azure",
    "resourceType": "compute/VirtualMachine",
    "sku": "Standard_B1s",
    "region": "eastus"
  }
}' localhost:$PORT finfocus.v1.CostSourceService/Supports

# Test EstimateCost RPC
grpcurl -plaintext -d '{
  "resource": {
    "provider": "azure",
    "resourceType": "compute/VirtualMachine",
    "sku": "Standard_B1s",
    "region": "eastus"
  }
}' localhost:$PORT finfocus.v1.CostSourceService/EstimateCost

kill $PLUGIN_PID
```
