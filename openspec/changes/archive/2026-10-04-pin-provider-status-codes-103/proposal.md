# Proposal

## Why

Issue #103 reported that a non-Azure resource gets different gRPC codes from
different RPCs and proposed one code, probably `Unimplemented`, across the cost
RPCs. Checked against FinFocus core on 2026-10-04 (`origin/main` 2223683):

- Core's engine falls back to the next plugin on any error code, and calls
  `Supports()` first, so the code does not change routing.
- Core's own conformance check (`internal/conformance/cost.go`,
  `testGetProjectedCostInvalid`) sends provider `aws`, type
  `invalid:resource`, and accepts only `NotFound` or `InvalidArgument`.
  `Unimplemented` would fail it.

Measured on `main`, the plugin's behavior is already consistent once the
request shapes are taken into account:

- A provider other than `azure` or `azure-native` gets `InvalidArgument`
  ("unsupported provider") from `GetProjectedCost`, `GetActualCost`, and
  `GetPricingSpec`.
- An Azure type the plugin does not price gets `Unimplemented` from those
  RPCs and from `EstimateCost`.
- `EstimateCost` has no provider field, so its `Unimplemented` for an AWS type
  token is the unsupported-type rule, not a different answer for the same
  input.
- `Supports` and `DryRun` answer "unsupported" in a response, not an error.

Keep that behavior and pin it with a requirement and a test, so a later change
cannot drift to a code core's conformance check rejects.

## What Changes

- No runtime behavior change.
- New requirement in `resource-mapping` with an over-gRPC table test.
- CLAUDE.md states the codes and the reason.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `resource-mapping`: add a requirement for the status codes of unpriced
  providers and types across the descriptor RPCs.

## Impact

- `internal/pricing/provider_status_test.go` (new test).
- `openspec/specs/resource-mapping/spec.md` after archive; CLAUDE.md.
