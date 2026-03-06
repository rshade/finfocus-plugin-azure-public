# Research: VM Cost Estimation (EstimateCost RPC)

**Feature**: 020-vm-cost-estimation
**Date**: 2026-03-04

## Research Summary

### R-001: Current EstimateCost Implementation State

**Decision**: The EstimateCost RPC is already partially implemented in
`internal/pricing/calculator.go:95-120`. The existing code handles the happy
path (query -> cache -> price extraction -> response) but has gaps against the
spec.

**Existing code path**:

```text
EstimateCostRequest.Attributes
  -> estimateQueryFromRequest()     [calculator.go:259-283]
  -> azureclient.PriceQuery
  -> CachedClient.GetPrices()       [cache.go]
  -> CachedResult.Items
  -> unitPriceAndCurrency()         [calculator.go:361-377]
  -> pluginsdk.NewEstimateCostResponse(WithEstimateCost(currency, unitPrice*730))
```

**Rationale**: Build on existing implementation rather than rewriting.

**Alternatives considered**: Full rewrite using `MapDescriptorToQuery` — rejected
because `EstimateCostRequest` uses `Attributes` (structpb.Struct), not
`ResourceDescriptor`, so the mapper doesn't apply here.

---

### R-002: Proto Response Field Constraints

**Decision**: The `EstimateCostResponse` proto (finfocus-spec v0.5.7) only
supports four fields:

- `currency` (string) — ISO 4217
- `cost_monthly` (double) — 730 hours/month
- `pricing_category` (FocusPricingCategory enum)
- `spot_interruption_risk_score` (double, 0.0-1.0)

**Impact on spec requirements**:

<!-- markdownlint-disable MD013 -->

| Spec Requirement | Proto Support | Resolution |
| --- | --- | --- |
| FR-002: Hourly cost | No field | Derive: `cost_monthly / 730` |
| FR-003: Monthly cost | `cost_monthly` | Directly supported |
| FR-004: Yearly cost | No field | Derive: `cost_monthly * 12` |
| FR-006: Cost breakdown | No field | **Deferred** — proto update needed |

<!-- markdownlint-enable MD013 -->

**Rationale**: Cannot add fields to a proto we don't own. The spec's hourly and
yearly requirements are aspirational for the response format — the calculation
correctness can still be validated by checking that `cost_monthly == hourly * 730`.

**Alternatives considered**: Adding custom metadata via `structpb.Struct` —
rejected as it would be non-standard and break client compatibility.

---

### R-003: PricingCategory for Consumption Pricing

**Decision**: Set `PricingCategory` to `FOCUS_PRICING_CATEGORY_STANDARD` for
all Consumption-type Azure pricing. The existing code leaves it as `UNSPECIFIED`.

**Rationale**: Per proto docs, "Use STANDARD for on-demand/pay-as-you-go
resources." Azure Consumption pricing is pay-as-you-go.

---

### R-004: Resource Type Validation Gap

**Decision**: Add `ResourceType` field validation to `EstimateCost`. Currently,
`estimateQueryFromRequest` ignores the `ResourceType` field entirely. A request
for `network/LoadBalancer` with valid region/SKU succeeds (returns VM pricing).

**Implementation approach**: Check `ResourceType` contains
`compute/virtualMachine` (case-insensitive) before proceeding. Return
`codes.Unimplemented` for unsupported types. Empty `ResourceType` is allowed for
backward compatibility.

**Rationale**: FR-009 requires Unimplemented for unsupported resource types.

**Alternatives considered**: Reusing `MapDescriptorToQuery` — rejected because
`EstimateCostRequest` doesn't have a `ResourceDescriptor`; the type string is
just a flat field.

---

### R-005: Error Handling Improvements

**Decision**: Improve error granularity in `EstimateCost`:

1. **Missing fields**: Return `InvalidArgument` with specific field names
   (currently returns generic `Unimplemented`)
2. **Unsupported type**: Return `Unimplemented` with resource type in message
3. **No pricing data**: Already handled via `ErrNotFound` -> `NotFound`
4. **Context cancellation**: Already handled via `MapToGRPCStatus`

**Rationale**: FR-007 requires InvalidArgument identifying all missing fields.
The current `Unimplemented` response for missing fields is incorrect.

---

### R-006: Structured Logging for EstimateCost

**Decision**: Add structured log fields to the EstimateCost method:

- `region`, `sku`, `resource_type` on request entry
- `result_status` ("success", "cache_hit", "error") on completion
- `cost_monthly`, `currency` on success

**Rationale**: FR-012 requires structured logging with these fields. The current
implementation only logs "handling EstimateCost request" with no context.

---

### R-007: Cache Integration Verification

**Decision**: Cache integration is already complete. `CachedClient.GetPrices`
handles:

- Cache hits (no API call, returns `CachedResult` with `ExpiresAt`)
- Cache misses (API call, caches result, returns fresh data)
- TTL expiration (evicts expired entries, fetches fresh)

No changes needed to cache infrastructure. The existing
`TestEstimateCostUsesCachedClient` test verifies the integration.

**Rationale**: FR-010 (cached pricing for repeated queries) is satisfied by
existing `CachedClient` wrapper.
