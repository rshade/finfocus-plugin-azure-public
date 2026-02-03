# Research: CostSourceService Method Stubs

**Feature**: 004-costsource-stubs
**Date**: 2026-02-02
**Status**: Complete

## Executive Summary

No research required. All technical decisions are pre-determined by the existing
codebase and finfocus-spec SDK.

## Technical Decisions

### 1. Method Stub Pattern

**Decision**: Use gRPC `status.Error(codes.Unimplemented, "not yet implemented")`
for unimplemented methods.

**Rationale**: This is the standard gRPC pattern for signaling unimplemented
functionality. The `UnimplementedCostSourceServiceServer` already uses this
pattern, but we want explicit implementations for observability (logging).

**Alternatives Considered**:

- Return empty responses: Rejected - misleading to callers
- Return custom error messages: Rejected - non-standard, harder to handle client-side

### 2. Implemented Methods vs Stubs

**Decision**: Implement Name(), Supports(), and GetPluginInfo() with real
responses. All other methods return Unimplemented.

**Rationale**:

- Name() and GetPluginInfo() are required for plugin discovery
- Supports() provides clear signal that pricing is not yet available
- Other methods require Azure pricing data not yet implemented

**Method Categorization**:

| Method | Behavior | Reason |
| ------ | -------- | ------ |
| Name() | Return NameResponse | Plugin identity |
| GetPluginInfo() | Return valid metadata | Plugin registration |
| Supports() | Return supported=false | Signal not yet implemented |
| EstimateCost() | Return Unimplemented | Requires Azure pricing lookup |
| GetActualCost() | Return Unimplemented | Requires cost history |
| GetProjectedCost() | Return Unimplemented | Requires Azure pricing lookup |
| GetPricingSpec() | Return Unimplemented | Requires pricing schema |
| GetRecommendations() | Return Unimplemented | Not supported by public API |
| DismissRecommendation() | Return Unimplemented | Not supported |
| GetBudgets() | Return Unimplemented | Not supported by public API |
| DryRun() | Return Unimplemented | Requires field mapping |

### 3. Logging Pattern

**Decision**: Use existing `logging.RequestLogger(ctx, c.logger)` for all new
methods.

**Rationale**: Consistent with existing GetPluginInfo() and EstimateCost()
implementations. Provides trace ID propagation.

### 4. Name() RPC Method

**Decision**: Implement Name() as a gRPC RPC method returning "azure-public".

**Rationale**: The existing `Name() string` method returns `"azure-public"` but
the gRPC interface requires `Name(context.Context, *NameRequest) (*NameResponse, error)`.
The RPC method will return "azure-public" for consistency with GetPluginInfo().

## Dependencies Verified

| Dependency | Version | Purpose | Status |
| ---------- | ------- | ------- | ------ |
| finfocus-spec | v0.5.4 | gRPC interface definitions | Available |
| zerolog | v1.34.0 | Structured logging | Available |
| google.golang.org/grpc | v1.78.0 | gRPC status codes | Available |

## No Outstanding Questions

All technical decisions are determined by:

1. Existing codebase patterns
2. finfocus-spec SDK interface requirements
3. gRPC best practices
