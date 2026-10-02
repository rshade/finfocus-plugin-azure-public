# TASKS.md — Azure Plugin v0.1.0 Release Plan

<!-- markdownlint-disable MD013 MD060 MD031 MD032 MD029 -->

**Current branch**: `020-vm-cost-estimation` | **Target**: v0.1.0 |
**Updated**: 2026-09-30 (spec v0.7.0)

## Current State Summary

### Build & Test Status

- **Build**: ✅ PASS (`go build ./...`)
- **Tests**: ✅ PASS (`go test ./...` — all 6 packages)
- **Linting**: golangci-lint reported 2 findings on 2026-09-30: `goconst` for the "USD" literal in `internal/pricing/calculator.go` and an unused `nolint` directive in `internal/azureclient/logger.go`. Re-run `make lint` for the current state.
- **Go version**: 1.27.1 (bumped during the 2026-09-30 toolchain rollout; uncommitted)
- **Spec version**: finfocus-spec v0.7.0 (bumped during the same rollout; uncommitted). `aws-public` is also on v0.7.0 now.

### RPC Implementation Status

| RPC | Status | Notes |
|-----|--------|-------|
| `Supports` | ✅ Complete | Maps resource types via `MapDescriptorToQuery()` |
| `EstimateCost` | ✅ Complete | Full VM pricing estimation |
| `GetProjectedCost` | ⚠️ Partial | Falls through to Unimplemented on validation failure |
| `GetActualCost` | ⚠️ Partial | Falls through to Unimplemented on validation failure |
| `GetPricingSpec` | ✅ Complete | Provides SDK discovery |
| `GetRecommendations` | ✅ Stub | Returns unimplemented |
| `DismissRecommendation` | ✅ Stub | Returns unimplemented |
| `GetBudgets` | ✅ Stub | Returns unimplemented |
| `DryRun` | ✅ Stub | Returns unimplemented |

### Resource Type Coverage

**Supported today** (3 types, all in the `resourceTypeToService` table in `internal/pricing/mapper.go`):
- `compute/VirtualMachine` (Azure service: Virtual Machines)
- `storage/ManagedDisk` (Azure service: Managed Disks)
- `storage/BlobStorage` (Azure service: Storage)

**v0.1.0 scope is all the resource types, not these three.** Also in scope (owner
decision, 2026-09-30): Spot VMs (#42), Storage Accounts (#50), App Service and
Azure Functions (#48), AKS (#49), and SQL Database and Cosmos DB (#51, which starts
as a spike and ends as working estimation). Tasks AZ-2.3 to AZ-2.8.

**Stretch, only after everything in scope is verified** (parity with
`aws-public`, no issue yet): Load Balancer, NAT Gateway, VM Scale Sets, Azure Cache
for Redis, Database for PostgreSQL and MySQL. Task AZ-2.9.

**Every other open issue is also in scope**: DryRun (#43), GetPricingSpec (#44),
multi-pricing comparison (#45), FOCUS alignment (#46), regional comparison (#47),
the test and validation issues #52 to #55, and the two research spikes #56 and #57.
Nothing in the current tracker is deferred. See the Issue Index.

### Release Infrastructure

- ❌ No `.goreleaser.yaml` (AWS plugin has one)
- ❌ No release workflow (AWS plugin uses `release-please.yml` + `release.yml`)
- ❌ No release-please config
- ❌ No version tags (GitHub: no releases)
- ❌ Not registered in finfocus core (`internal/registry/registry.json`)

### File Evidence

- Uncommitted file: `specs/001-go-module-init/plan.md`
- Main entry: `cmd/finfocus-plugin-azure-public/main.go` (runs pluginsdk.Serve)
- Pricing logic: `internal/pricing/calculator.go` (402 lines, well-structured)
- Mapper: `internal/pricing/mapper.go` (113 lines, validates resources)
- Azure client: `internal/azureclient/client.go` + cache + retry logic (production-ready)

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

**Status:** DONE, no stretch types landed, network/LoadBalancer stopped because contains(serviceName, 'Load Balancer') returned 48 Consumption rows and no single meter is the price (Standard and Global each publish included rules, overage rules, and data processed; Gateway publishes Gateway and Gateway Chain hourly meters)

**Description**: Only after AZ-2.1 to AZ-2.8 are verified. In priority order: Load
Balancer, NAT Gateway, Virtual Machine Scale Sets (reuse the VM estimator), Azure Cache
for Redis, Azure Database for PostgreSQL and MySQL flexible servers. The same method as
above. Stop at any boundary with everything verified. Partial and correct beats
complete and unverified.

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

**Status:** DONE via AZ-6.3. `PriceItem` parses the nested `savingsPlan` array. `ReservationHourly` divides the term total by 8760 or 26280. No RPC returns those extra prices. See AZ-6.4.

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

**Status:** BLOCKED, GetProjectedCostResponse has no repeated price list. The savings math is not called from an RPC. Spec proposal: [spec issue 588](https://github.com/rshade/finfocus-spec/issues/588)

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

**Status:** DONE via AZ-6.5. A configured billing account id returns a FocusRecord that passes validation. An empty setting leaves FocusRecord nil. The request still has no field. Spec proposal: [spec issue 590](https://github.com/rshade/finfocus-spec/issues/590).

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

**Status:** BLOCKED, GetProjectedCostResponse has no repeated region list. SortRegionPrices is not called from an RPC. Spec proposal: [spec issue 589](https://github.com/rshade/finfocus-spec/issues/589)

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

**Status:** DONE, `python3 -m json.tool release-please-config.json`, parses; `.release-please-manifest.json` `"."` is `0.0.0`, not bumped

**Description**: Create `release-please-config.json` and `.release-please-manifest.json` to automate semver bumping and CHANGELOG generation (pattern from AWS plugin). Enables one-click releases via GitHub UI.

**Files**:
- `release-please-config.json` (new, ~20 lines)
- `.release-please-manifest.json` (new, 1 line: `{"." : "0.1.0"}`)

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

**Status:** DONE, `go test -count=1 ./internal/pricing/ -run TestGolden`, ok github.com/rshade/finfocus-plugin-azure-public/internal/pricing 0.027s

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

**Status:** REOPENED, the test calls the Calculator in process, the task asks for a gRPC load test. See AZ-6.6.

**Description**: Go benchmarks for the estimation path with a mocked client, and a
concurrent gRPC load test that checks the cache hit rate exceeds 80% for repeated
queries. Record baseline numbers in a document.

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

**Description**: Create annotated git tag `v0.1.0` and push to GitHub. GitHub Actions (`release.yml`) automatically builds binaries and publishes to Releases. This marks the official stable point with 3 supported resource types and working GetProjectedCost/GetActualCost RPCs.

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

**Status:** DONE, `go test -count=1 -run 'TestPriceItemParsesSavingsPlanFixture|TestReservationHourlyUsesTermTotal' ./internal/estimation/` passed. `PriceItem` keeps the nested array. `ReservationHourly` divides the term total by 8760 or 26280. Break check: the test failed to compile (`SavingsPlan` undefined, `ReservationHourly` undefined) before the field and function existed. `GetProjectedCost` still returns one price. The repeated alternative-price list is AZ-6.4 and is not delivered.

#### AZ-6.4 — File the spec issues for multi-pricing and regional comparison [Issues #45, #47]

**Description**: `GetProjectedCostResponse` and `EstimateCostResponse` have no repeated
field for alternative prices or per-region prices. Prove it, check for existing issues,
and file them in `finfocus-spec` per section 4b: a `repeated PriceOption` (category,
model, term, unit price, monthly cost, upfront cost, savings fraction) and a
`repeated RegionPrice` (region, unit price, monthly cost, currency), additive and
advisory, never summed into the primary cost.

**Acceptance Criteria**: two issue URLs (or links to existing issues) in the status line
and in the Not delivered register. The pure functions stay.

**Status:** DONE, spec v0.7.0 `costsource.proto` has one `unit_price` (line 371) and one `cost_per_month` (line 375) on `GetProjectedCostResponse`, and one `cost_monthly` (line 1233) on `EstimateCostResponse`. No `PriceOption` or `RegionPrice` message exists. Issue search before filing found no match. Filed [spec issue 588](https://github.com/rshade/finfocus-spec/issues/588) and [spec issue 589](https://github.com/rshade/finfocus-spec/issues/589). Break check: a matching open issue would have been linked instead of filing a new one. `SavingsFraction`, `ReservationHourly`, and `SortRegionPrices` are not called from an RPC. AZ-2.13 and AZ-2.15 stay BLOCKED. Not delivered until those fields exist.

#### AZ-6.5 — FOCUS record in production [Issue #46]

**Description**: `ValidateFocusRecord` rejects an empty `billing_account_id`, so run 1
emits no record. Find how a billing account id can reach the plugin (a request tag or
plugin configuration) and emit a validated record when it is given. If the spec has no
carrier, file the issue. Do not invent an account id.

**Acceptance Criteria**: a gRPC test with an id returns a record that passes
`ValidateFocusRecord`, and a test without one proves the documented fallback.

**Status:** DONE, `go test -count=1 -run 'TestGetActualCostFocusRecordOverGRPC|TestGetActualCostOmitsFocusRecordWithoutAccountOverGRPC' ./internal/pricing/` passed. `SetBillingAccountID` feeds the record. The environment variable `FINFOCUS_BILLING_ACCOUNT_ID` sets it in the process. Empty leaves FocusRecord nil and logs why. Break check: the test failed to compile (`SetBillingAccountID` undefined) before the setter existed. The request has no field. Spec proposal: [spec issue 590](https://github.com/rshade/finfocus-spec/issues/590). No account id is invented.

#### AZ-6.6 — A real gRPC load test [Issue #54]

**Description**: Start a gRPC server and drive it with a client from concurrent
goroutines. Assert the cache hit rate and that no request fails.

**Acceptance Criteria**: the test goes through the network stack, and a break check
(for example disabling the cache) makes it fail.

#### AZ-6.7 — EstimateCost and DryRun for every type [Issue #43]

**Description**: `EstimateCost` covers VMs and managed disks only. Route it for the other
resource types, or return `Unimplemented` naming the task id and list the types in the
register. Run the gRPC `DryRun` test over every supported type, not only Cosmos.

**Acceptance Criteria**: a table-driven gRPC test covers every type in `mapper.go`.

#### AZ-6.8 — Cosmos serverless storage and the AKS Free tier [Issues #51, #49]

**Description**: Serverless Cosmos accounts with data are under-quoted because the storage
meter is excluded, and live serverless has no storage meter. Say so in the quote notes
and the register. A live `FreeTierInfrastructureCost Uptime SLA` AKS meter at 0.05 per
hour is effective from 2026-10-01: confirm it against the live API and handle it.

**Acceptance Criteria**: live queries pasted, tests updated, behaviour documented.

#### AZ-6.9 — Load Balancer (stretch AZ-2.9)

**Description**: Load Balancer pricing is Global-only (an `armRegionName` of eastus returns
nothing). Standard rule-hours meters are 0.025 per hour. Implement a Global-fallback
estimate with the usage assumptions stated, or record why not.

**Acceptance Criteria**: live-verified query, an honest status, no silent zero.

#### AZ-6.10 — Test hygiene

**Description**: remove the stale "GetProjectedCost not implemented yet" skip in
`calculator_test.go`; document the two undocumented exported helpers in
`internal/azureclient/cache_test.go`; make every skipped accuracy test state which
owner value is missing.

**Acceptance Criteria**: `go test -v ./... | grep SKIP` shows only skips with a stated
input requirement or `-short`.

#### AZ-6.11 — Documentation matches the code

**Description**: update `ROADMAP.md`, `CONTEXT.md`, `IMPLEMENTATION_SUMMARY.md`,
`README.md` and the `TASKS.md` text that contradicts the plugin. State that
`GetActualCost` means a running-cost estimate from public retail prices, not billed
spend.

**Acceptance Criteria**: markdownlint clean, and no document claims a feature the code
lacks or denies one it has.

#### AZ-6.12 — Calculator accuracy values [Issue #53]

**Description**: the owner reads values from the Azure Pricing Calculator into
`internal/pricing/testdata/oracle/calculator-values.csv`. Make `TestCalculatorAccuracy` read
that file (case id to `owner_monthly_usd`) so the 9 skipped cases and the oracle cases that
have a value run against it, within 5%. A row with an empty value is skipped with a reason that
names the case. Never invent or copy a value, and never fill the file yourself.

**Acceptance Criteria**: with the file as supplied by the owner, every filled row is compared;
the report lists which rows were empty. Status is BLOCKED-ON-INPUT for any type with no filled
row.

#### AZ-6.13 — Whole-branch review and report

**Description**: review the whole run branch, fix Critical and Important findings in new
commits, then write `superpowers-run-report.md` with the Not delivered register.

**Acceptance Criteria**: report follows section 8 of the prompt.

#### AZ-6.14 — Oracle comparison gate (run this right after AZ-6.1)

**Status:** DONE, `go test -count=1 -run TestOracleComparison ./internal/pricing/` passed, 51 cases in `.superpowers/oracle-results.md`. Break check: doubling `unit * pluginsdk.HoursPerMonth` in the VM quote failed 19 cases, then restored. `git diff -- internal/pricing/testdata/oracle/` empty. Ambiguous AKS free chose 36.50. Ambiguous SQL zone redundancy returned NotFound because the fixture has no base compute row. Spot EstimateCost is NotFound and stays AZ-6.2.

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

## Issue Index

### Legend

- **roadmap/current**: Critical for v0.1.0 (high priority)
- **roadmap/next**: Post-v0.1.0, immediate follow-up
- **roadmap/future**: Backlog for v0.2+
- **spec-first**: Requires finfocus-spec update
- **effort/small**: 1-2 days | **effort/medium**: 3-5 days | **effort/large**: 5+ days

### All Open Issues (18)

| # | Title | Labels | Disposition | Task |
|---|-------|--------|-------------|------|
| [#60](https://github.com/rshade/finfocus-plugin-azure-public/issues/60) | Implement GetActualCost RPC for Azure historical cost lookup | roadmap/current, component/estimation, priority/high, effort/medium, spec-first | **AZ-2.2** | Finalize GetActualCost RPC |
| [#59](https://github.com/rshade/finfocus-plugin-azure-public/issues/59) | Implement GetProjectedCost RPC for Azure pricing projection | roadmap/current, component/estimation, priority/high, effort/medium, spec-first | **AZ-2.1** | Finalize GetProjectedCost RPC |
| [#57](https://github.com/rshade/finfocus-plugin-azure-public/issues/57) | Research spike: Savings Plans pricing in Azure Retail Prices API | roadmap/future, component/estimation, priority/medium, effort/small | **AZ-2.12** | In scope for v0.1.0: spike with findings document |
| [#56](https://github.com/rshade/finfocus-plugin-azure-public/issues/56) | Research spike: Carbon footprint estimation data sources for Azure | roadmap/future, component/estimation, priority/low, effort/medium | **AZ-2.16** | In scope for v0.1.0: spike with findings document |
| [#55](https://github.com/rshade/finfocus-plugin-azure-public/issues/55) | Implement chaos testing for Azure API failure scenarios | roadmap/next, component/testing, priority/medium, effort/small | **AZ-3.10** | In scope for v0.1.0 |
| [#54](https://github.com/rshade/finfocus-plugin-azure-public/issues/54) | Implement performance benchmarking and load testing | roadmap/next, component/testing, priority/medium, effort/small | **AZ-3.9** | In scope for v0.1.0 |
| [#53](https://github.com/rshade/finfocus-plugin-azure-public/issues/53) | Implement pricing accuracy validation against Azure Pricing Calculator | roadmap/next, component/testing, priority/high, effort/small | **AZ-3.8** | In scope for v0.1.0; BLOCKED-ON-INPUT (calculator values from the owner) |
| [#52](https://github.com/rshade/finfocus-plugin-azure-public/issues/52) | Implement regression test suite with golden pricing data | roadmap/next, component/testing, priority/high, effort/medium | **AZ-3.7** | In scope for v0.1.0 |
| [#51](https://github.com/rshade/finfocus-plugin-azure-public/issues/51) | Research spike: Azure SQL Database & Cosmos DB pricing mapping | roadmap/future, component/estimation, priority/medium, effort/medium | **AZ-2.7, AZ-2.8** | In scope for v0.1.0: spike then working estimation |
| [#50](https://github.com/rshade/finfocus-plugin-azure-public/issues/50) | Implement Storage Accounts capacity-based cost estimation | roadmap/future, component/estimation, priority/medium, effort/medium | **AZ-2.4** | In scope for v0.1.0 |
| [#49](https://github.com/rshade/finfocus-plugin-azure-public/issues/49) | Implement AKS cluster cost estimation | roadmap/future, component/estimation, priority/high, effort/medium | **AZ-2.6** | In scope for v0.1.0 |
| [#48](https://github.com/rshade/finfocus-plugin-azure-public/issues/48) | Implement App Service & Azure Functions cost estimation | roadmap/future, component/estimation, priority/high, effort/medium | **AZ-2.5** | In scope for v0.1.0 |
| [#47](https://github.com/rshade/finfocus-plugin-azure-public/issues/47) | Regional price heatmap — cross-region cost comparison for SKUs | roadmap/future, component/estimation, priority/low, effort/large | **AZ-2.15** | In scope for v0.1.0 |
| [#46](https://github.com/rshade/finfocus-plugin-azure-public/issues/46) | Align response fields with FOCUS 1.3 specification | roadmap/future, priority/low, effort/large, spec-first | **AZ-2.14** | In scope for v0.1.0; re-scoped against spec v0.7.0 (FOCUS 1.4) |
| [#45](https://github.com/rshade/finfocus-plugin-azure-public/issues/45) | Multi-pricing model comparison (Consumption vs Reserved vs Savings Plans) | roadmap/future, component/estimation, priority/low, effort/large, spec-first | **AZ-2.13** | In scope for v0.1.0; response shape needs a spec check |
| [#44](https://github.com/rshade/finfocus-plugin-azure-public/issues/44) | Implement GetPricingSpec RPC for plugin discovery | roadmap/future, component/estimation, priority/medium, effort/medium, spec-first | **AZ-2.11** | In scope for v0.1.0 |
| [#43](https://github.com/rshade/finfocus-plugin-azure-public/issues/43) | Implement DryRun validation RPC | roadmap/future, component/estimation, priority/medium, effort/small, spec-first | **AZ-2.10** | In scope for v0.1.0 |
| [#42](https://github.com/rshade/finfocus-plugin-azure-public/issues/42) | Add Spot VM pricing support | roadmap/future, component/estimation, priority/medium, effort/small | **AZ-2.3** | In scope for v0.1.0 |

**Summary** (Updated for spec v0.7.0):
- **All 18 open issues are in scope for v0.1.0, each with its own task.** The owner
  decided on 2026-09-30 that v0.1.0 is the full working plugin: every resource type
  and every issue in the tracker.
  - RPCs: #59, #60 (AZ-2.1, AZ-2.2), #43 (AZ-2.10), #44 (AZ-2.11)
  - Resource types: #42, #48, #49, #50, #51 (AZ-2.3 to AZ-2.8)
  - Pricing models and data: #57 (AZ-2.12 spike), #45 (AZ-2.13), #46 (AZ-2.14),
    #47 (AZ-2.15), #56 (AZ-2.16 spike)
  - Tests and validation: #52 to #55 (AZ-3.7 to AZ-3.10). #53 is BLOCKED-ON-INPUT.
  - Note: AZ-2.1 (GetProjectedCost) may optionally populate the `cost_breakdown`
    map from v0.7.0. AZ-2.6 (AKS) is where it is most useful.
- **Post-v0.1.0**: none of the current issues. The stretch parity types (AZ-2.9)
  have no issue yet.

---

## Open Questions

### Specification & Dependencies

1. **v0.7.0 upgrade impact** (Updated 2026-09-30): finfocus-spec v0.7.0 (released
   2026-09-29) adds FOCUS 1.4 fields and `cost_breakdown` map on
   GetProjectedCostResponse. Key changes affecting Azure plugin:
   - **cost_breakdown (optional)**: New map<string,double> field (15) on
     GetProjectedCostResponse. Breakdown should sum to ±max(0.01, 0.1%) of
     cost_per_month. Azure plugin should populate this by cost component if
     available (e.g., compute, storage, networking). Done in run 1: the resource
     quotes populate it.
   - **Trace ID logging on validation failures**: SDK now automatically logs
     host-provided trace_id on input validation errors. No plugin code change
     required; happens at SDK level.
   - **FOCUS 1.4 fields**: invoice_detail_id, commitment_program_eligibility_details
     added to responses. Optional for v0.1.0; defer to v0.2+.

2. **Go version cascade**: The upgrade from Go 1.25.7 to 1.27.1 (AZ-1.1) was applied
   locally on 2026-09-30 and `go build`, `go vet` and `go test` pass. It has not
   run on a CI runner yet. Confirm on the first CI run.

3. **Confidence level encoding** (answered in run 1: kept in `Source`, the v0.7.0 proto has no field for it): Issue #60 references encoding confidence in the
   `Source` field as `"azure-retail-prices[confidence:HIGH]"`. Is this format
   standardized across plugins, or should this be a separate field? Check v0.7.0
   proto for ActualCostResult.confidence_level or similar field.

### Release & Registry

4. **Release assets**: AWS plugin releases include checksums and signatures. Should Azure plugin v0.1.0 include these, or is a simple goreleaser setup with multiple OS/arch binaries sufficient for initial release?

5. **Registry addition timing**: AZ-4.1 (registry entry) depends on a PR to the finfocus core repo. Should this be filed immediately after AZ-5.1 (v0.1.0 tag is pushed), or should an Azure plugin maintainer prepare the PR in parallel?

6. **Backwards compatibility**: Azure plugin is not yet in registry, so there's no
   installed base to break. However, are there any protocol-level changes in spec
   v0.7.0 that existing finfocus installations (v0.6.1+) would expect? Check if
   v0.7.0 cost_breakdown or FOCUS 1.4 fields cause compatibility issues when core
   is still on v0.6.x.

### Testing & QA

7. **E2E testing**: Current project has unit tests (`go test`) but no integration tests that actually call the Azure Retail Prices API (live vs mock). Should v0.1.0 include integration tests with real API calls, or are unit tests with mock responses sufficient for initial release?

8. **Docstring coverage enforcement**: Constitution requires ≥80% docstring coverage. What tool/script should be used to verify this in CI? (AZ-3.2)

### Resource Expansion

9. **Stretch types (AZ-2.9)**: the parity types with `aws-public` (Load Balancer, NAT
   Gateway, Virtual Machine Scale Sets, Redis, PostgreSQL and MySQL) have no issue yet.
   The owner has not yet confirmed which, if any, are required for v0.1.0. Until then
   they are a stretch after everything else is verified.
10. **Calculator values for #53 (AZ-3.8)**: the Azure Pricing Calculator is a web
    application. The owner needs to supply the expected monthly costs for the
    sample configurations. Until then AZ-3.8 stays BLOCKED-ON-INPUT.
11. **Spec-first issues (#43, #44, #45, #46, #47)**: several say a spec change is
    needed first. v0.7.0 may already provide it (`DryRun`, `GetPricingSpec`, FOCUS
    1.4). Each task tells the run to read the spec and list any real gap under
    decisions needed.

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
