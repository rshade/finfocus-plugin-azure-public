# Data Model: CostSourceService Method Stubs

**Feature**: 004-costsource-stubs
**Date**: 2026-02-02

## Overview

This feature does not introduce new data entities.
It implements stub methods using existing gRPC protobuf message types defined in `finfocus-spec`.

## Existing Entities (No Changes)

### Calculator

The plugin implementation type. Already exists in `internal/pricing/calculator.go`.

```go
type Calculator struct {
    finfocusv1.UnimplementedCostSourceServiceServer
    logger zerolog.Logger
}
```

**Fields**:

- `UnimplementedCostSourceServiceServer`: Embedded for forward compatibility
- `logger`: Structured logger for observability

**Relationships**: None (stateless)

### Request/Response Types (from finfocus-spec)

All message types are defined in `github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1`:

| RPC Method | Request Type | Response Type |
| ---------- | ------------ | ------------- |
| Name | NameRequest | NameResponse |
| Supports | SupportsRequest | SupportsResponse |
| GetPluginInfo | GetPluginInfoRequest | GetPluginInfoResponse |
| EstimateCost | EstimateCostRequest | EstimateCostResponse |
| GetActualCost | GetActualCostRequest | GetActualCostResponse |
| GetProjectedCost | GetProjectedCostRequest | GetProjectedCostResponse |
| GetPricingSpec | GetPricingSpecRequest | GetPricingSpecResponse |
| GetRecommendations | GetRecommendationsRequest | GetRecommendationsResponse |
| DismissRecommendation | DismissRecommendationRequest | DismissRecommendationResponse |
| GetBudgets | GetBudgetsRequest | GetBudgetsResponse |
| DryRun | DryRunRequest | DryRunResponse |

## State Transitions

N/A - Plugin is stateless. Each RPC is independent.

## Validation Rules

1. All requests are validated by gRPC protobuf deserialization
2. No additional validation needed for stub methods
3. Supports() always returns `supported: false` regardless of input

## Data Volume Assumptions

- No data storage
- In-memory only (logger reference)
- Zero allocation for Unimplemented responses
