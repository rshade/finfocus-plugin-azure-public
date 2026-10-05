# Tasks

## 1. Tests first

- [x] 1.1 Add `TestDescriptorRPCs_UnpricedInput_ReturnDocumentedCode` over gRPC for the core conformance AWS resource, an AWS EC2 instance, a GCP instance, an empty provider, the tags-only actual-cost path, and an unknown Azure type, across `GetProjectedCost`, `GetActualCost`, `GetPricingSpec`, `Supports`, `DryRun`, plus `EstimateCost` with an AWS type. Verify: `go test -count=1 -run '^TestDescriptorRPCs_UnpricedInput_ReturnDocumentedCode$' ./internal/pricing/`. Break check: changing the provider error in `classifyResource` (`internal/pricing/cost.go`) to `Unimplemented`, as issue #103 proposed, fails all three cost RPCs for the core conformance resource.

## 2. Docs and gates

- [x] 2.1 CLAUDE.md "Error Sentinels" states the codes and the core conformance reason. Verify: `grep -n 'unsupported provider' CLAUDE.md`.
- [x] 2.2 Gates. Verify: `make build`, `make test`, `make lint`, `make spec-check` pass.
