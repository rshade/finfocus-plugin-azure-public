# Contracts: CostSourceService Method Stubs

**Feature**: 004-costsource-stubs
**Date**: 2026-02-02

## Overview

This feature implements existing gRPC contracts defined in `finfocus-spec`.
No new contracts are introduced.

## Contract Source

All gRPC service definitions are in:

- Package: `github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1`
- Proto file: `finfocus/v1/costsource.proto`

## CostSourceService Interface

```protobuf
service CostSourceService {
  rpc Name(NameRequest) returns (NameResponse);
  rpc Supports(SupportsRequest) returns (SupportsResponse);
  rpc GetActualCost(GetActualCostRequest) returns (GetActualCostResponse);
  rpc GetProjectedCost(GetProjectedCostRequest) returns (GetProjectedCostResponse);
  rpc GetPricingSpec(GetPricingSpecRequest) returns (GetPricingSpecResponse);
  rpc EstimateCost(EstimateCostRequest) returns (EstimateCostResponse);
  rpc GetRecommendations(GetRecommendationsRequest) returns (GetRecommendationsResponse);
  rpc DismissRecommendation(DismissRecommendationRequest) returns (DismissRecommendationResponse);
  rpc GetBudgets(GetBudgetsRequest) returns (GetBudgetsResponse);
  rpc GetPluginInfo(GetPluginInfoRequest) returns (GetPluginInfoResponse);
  rpc DryRun(DryRunRequest) returns (DryRunResponse);
}
```

## Implementation Notes

This feature does not modify contracts.
It implements stub handlers for existing contract methods.

For full contract documentation, see:

- [finfocus-spec repository](https://github.com/rshade/finfocus-spec)
- Generated Go types in `sdk/go/proto/finfocus/v1/costsource.pb.go`
