# Feature Specification: CostSourceService Method Stubs

**Feature Branch**: `004-costsource-stubs`
**Created**: 2026-02-02
**Status**: Draft
**Input**: GitHub Issue #5 - Implement CostSourceService method stubs

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Plugin Identity Query (Priority: P1)

FinFocus Core system queries the Azure Public plugin to retrieve its identity for registration and display purposes.

**Why this priority**: Identity is the foundational capability - without Name() and GetPluginInfo() working, Core cannot identify or register the plugin. This is the first RPC called during plugin discovery.

**Independent Test**: Can be fully tested by starting the plugin and making a Name() RPC call, verifying the response contains "azure-public".

**Acceptance Scenarios**:

1. **Given** plugin is running, **When** Core calls Name() RPC, **Then** returns NameResponse with name "azure-public"
2. **Given** plugin is running, **When** Core calls GetPluginInfo() RPC, **Then** returns GetPluginInfoResponse with name, version, spec_version, and providers containing "azure"

---

### User Story 2 - Resource Support Query (Priority: P2)

FinFocus Core queries whether the plugin can provide pricing for a specific Azure resource type.

**Why this priority**: After identity, Core needs to know what the plugin supports. For v0.1.0, plugin returns "not supported" for all resources since pricing logic isn't implemented yet.

**Independent Test**: Can be fully tested by calling Supports() with any resource type and verifying it returns supported: false with a reason.

**Acceptance Scenarios**:

1. **Given** plugin is running, **When** Core calls Supports() with any resource type, **Then** returns SupportsResponse with supported=false and reason explaining not yet implemented
2. **Given** plugin is running, **When** Core calls Supports() with malformed request, **Then** returns SupportsResponse with supported=false (no crash)

---

### User Story 3 - Cost Estimation Request (Priority: P3)

FinFocus Core requests cost estimation for an Azure resource. For v0.1.0, plugin signals this capability is not yet available.

**Why this priority**: Cost estimation is the plugin's eventual primary purpose, but for scaffold phase, returning Unimplemented gracefully is sufficient.

**Independent Test**: Can be fully tested by calling EstimateCost() RPC and verifying it returns an Unimplemented gRPC status code.

**Acceptance Scenarios**:

1. **Given** plugin is running, **When** Core calls EstimateCost(), **Then** returns gRPC status Unimplemented with message "not yet implemented"
2. **Given** plugin is running, **When** Core calls any unimplemented RPC, **Then** returns appropriate Unimplemented status (no panic or crash)

---

### User Story 4 - Plugin Stability Under Unknown RPCs (Priority: P4)

Plugin consumers need assurance that calling any RPC method will return a proper response, not crash the plugin.

**Why this priority**: Stability is essential for integration testing workflows. Even if features aren't implemented, the plugin must remain operational.

**Independent Test**: Can be fully tested by calling each of the 11 RPC methods and verifying each returns either a valid response or Unimplemented status without crashing.

**Acceptance Scenarios**:

1. **Given** plugin is running, **When** Core calls GetActualCost(), **Then** returns Unimplemented status
2. **Given** plugin is running, **When** Core calls GetProjectedCost(), **Then** returns Unimplemented status
3. **Given** plugin is running, **When** Core calls GetPricingSpec(), **Then** returns Unimplemented status
4. **Given** plugin is running, **When** Core calls GetRecommendations(), **Then** returns Unimplemented status
5. **Given** plugin is running, **When** Core calls DismissRecommendation(), **Then** returns Unimplemented status
6. **Given** plugin is running, **When** Core calls GetBudgets(), **Then** returns Unimplemented status
7. **Given** plugin is running, **When** Core calls DryRun(), **Then** returns Unimplemented status

---

### Edge Cases

- Plugin receives malformed protobuf messages (gRPC handles automatically)
- Plugin receives request with nil context (should not panic)
- Multiple concurrent RPC calls to different methods (should handle safely)
- Plugin is queried immediately after startup (should respond correctly)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Plugin MUST implement Name() RPC returning NameResponse{name: "azure-public"}
- **FR-002**: Plugin MUST implement Supports() RPC returning SupportsResponse{supported: false, reason: "not yet implemented"}
- **FR-003**: Plugin MUST implement GetPluginInfo() RPC returning valid metadata including name, version, spec_version, and providers list
- **FR-004**: Plugin MUST implement EstimateCost() RPC returning gRPC status Unimplemented with message "not yet implemented"
- **FR-005**: Plugin MUST implement GetActualCost() RPC returning gRPC status Unimplemented
- **FR-006**: Plugin MUST implement GetProjectedCost() RPC returning gRPC status Unimplemented
- **FR-007**: Plugin MUST implement GetPricingSpec() RPC returning gRPC status Unimplemented
- **FR-008**: Plugin MUST implement GetRecommendations() RPC returning gRPC status Unimplemented
- **FR-009**: Plugin MUST implement DismissRecommendation() RPC returning gRPC status Unimplemented
- **FR-010**: Plugin MUST implement GetBudgets() RPC returning gRPC status Unimplemented
- **FR-011**: Plugin MUST implement DryRun() RPC returning gRPC status Unimplemented
- **FR-012**: Plugin MUST NOT panic or crash when any RPC is called
- **FR-013**: Plugin MUST log each RPC call at Info level for observability

### Key Entities

- **Calculator**: The plugin implementation type that embeds UnimplementedCostSourceServiceServer and implements stub methods
- **PluginInfo**: Metadata structure containing name, version, spec_version, providers list, and optional metadata map

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 11 CostSourceService RPC methods respond without crashing (100% coverage)
- **SC-002**: Name() returns correct plugin name in under 10ms
- **SC-003**: GetPluginInfo() returns complete metadata in under 10ms
- **SC-004**: Supports() responds with valid structure in under 10ms
- **SC-005**: Unimplemented methods return proper gRPC status codes
- **SC-006**: Plugin can handle 100 concurrent RPC calls without degradation
- **SC-007**: Unit test coverage for all 11 RPC methods reaches 80% or higher

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80% for business logic)
- [x] Error handling strategy is defined (return Unimplemented status, no silent failures)
- [x] Code complexity is considered (simple stub implementations, each under 15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (gRPC client calling each RPC)
- [x] Performance test criteria specified (response times under 10ms)

### User Experience

- [x] Error messages are user-friendly and actionable ("not yet implemented" is clear)
- [x] Response time expectations defined (<10ms for metadata, <100ms for operations)
- [x] Observability requirements specified (Info-level logging for each RPC call)

### Documentation

- [x] README.md updates identified (N/A - internal stubs, no user-facing changes)
- [x] API documentation needs outlined (godoc comments on each method)
- [x] Examples/quickstart guide planned (N/A - not needed for stubs)

### Performance & Reliability

- [x] Performance targets specified (<10ms response time for metadata RPCs)
- [x] Reliability requirements defined (return proper status codes, no panics)
- [x] Resource constraints considered (stateless, minimal memory footprint)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data

## Assumptions

1. The plugin already has the basic gRPC server infrastructure from previous work
2. The Calculator type already embeds UnimplementedCostSourceServiceServer
3. The pluginsdk.Serve() function handles gRPC server registration
4. Version information is injected at build time via ldflags
5. The zerolog logger is already available in the Calculator struct
