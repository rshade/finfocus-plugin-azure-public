# Feature Specification: ResourceDescriptor to Azure Filter Mapping

**Feature Branch**: `016-descriptor-filter-mapping`
**Created**: 2026-03-03
**Status**: Draft
**Input**: GitHub Issue #16 - Map ResourceDescriptor fields from
FinFocus to Azure Retail Prices API filter parameters

## Clarifications

### Session 2026-03-03

- Q: Which Azure service name should disk resources map to:
  "Storage" or "Managed Disks"?
  → A: Support both. Different disk subtypes map to different
  Azure service names (e.g., managed disks → "Managed Disks",
  blob/file storage → "Storage").
- Q: Should tags be used as fallback when primary region/SKU
  fields are empty?
  → A: Yes. Use `tags["region"]` and `tags["sku"]` as fallback
  when the primary `region` and `sku` fields are empty. Primary
  fields always take precedence over tags.
- Q: Should resource type matching be case-sensitive or
  case-insensitive?
  → A: Case-insensitive. Normalize resource type strings
  before matching to handle casing variations from callers.
- Q: How should non-azure provider resources be handled?
  → A: Return the same "unimplemented" response as unsupported
  resource types. No distinct "wrong provider" error needed.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Map VM Resource to Pricing Query (Priority: P1)

When the cost estimation system receives a resource description for a virtual machine,
it translates the VM's characteristics (SKU, region) into the correct pricing query
filters so that Azure pricing data can be retrieved for that specific VM configuration.

**Why this priority**: Virtual machines are the most common cloud
resource type and represent the highest-value cost estimation target.
Without VM mapping, no cost estimation is possible.

**Independent Test**: Can be fully tested by providing a VM resource
description with known SKU and region values and verifying the
resulting query filter contains the correct Azure-specific field
mappings for virtual machine pricing.

**Acceptance Scenarios**:

1. **Given** a resource description with provider "azure",
   resource type "compute/VirtualMachine",
   SKU "Standard_B1s", and region "eastus",
   **When** the system maps it to a pricing query filter,
   **Then** the filter targets the "Virtual Machines" service
   with the correct SKU name and region values.

2. **Given** a resource description with provider "azure",
   resource type "compute/VirtualMachine",
   and SKU "Standard_D2s_v3",
   **When** the system maps it to a pricing query filter,
   **Then** the filter preserves the full Azure SKU name
   as provided in the descriptor.

3. **Given** a resource description with provider "azure",
   resource type "compute/VirtualMachine",
   and no explicit currency preference,
   **When** the system maps it to a pricing query filter,
   **Then** the filter defaults to USD currency.

---

### User Story 2 - Map Disk Resource to Pricing Query (Priority: P2)

When the cost estimation system receives a resource description for a managed disk,
it translates the disk's characteristics (disk type/tier, region) into the correct
pricing query filters so that Azure pricing data can be retrieved for that disk.

**Why this priority**: Managed disks are the second most common resource type attached
to VMs and are essential for complete infrastructure cost estimation.

**Independent Test**: Can be fully tested by providing a disk
resource description with known disk type and region values and
verifying the resulting query filter targets managed disk pricing.

**Acceptance Scenarios**:

1. **Given** a resource description with provider "azure",
   resource type "storage/ManagedDisk",
   SKU "Premium_LRS", and region "westus2",
   **When** the system maps it to a pricing query filter,
   **Then** the filter targets the "Managed Disks" service
   with the correct disk SKU and region.

2. **Given** a resource description with provider "azure",
   resource type "storage/ManagedDisk",
   and SKU "Standard_LRS",
   **When** the system maps it to a pricing query filter,
   **Then** the filter targets the "Managed Disks" service
   with the standard disk tier for pricing lookup.

3. **Given** a resource description with provider "azure",
   resource type "storage/BlobStorage",
   and region "eastus",
   **When** the system maps it to a pricing query filter,
   **Then** the filter targets the "Storage" service.

---

### User Story 3 - Validate Resource Description Completeness (Priority: P2)

When the cost estimation system receives a resource description that is missing required
fields (such as region or SKU), it returns a clear, actionable error message indicating
exactly which fields are missing, so that callers can correct their request.

**Why this priority**: Validation prevents confusing downstream errors from the pricing
service and provides a better developer experience. Equal priority with disk mapping
because both are needed for a production-ready system.

**Independent Test**: Can be fully tested by providing incomplete resource descriptions
(missing region, missing SKU) and verifying that specific, descriptive error messages
are returned identifying the missing fields.

**Acceptance Scenarios**:

1. **Given** a VM resource description missing the region field,
   **When** the system attempts to map it to a pricing query filter,
   **Then** an error is returned indicating that "region" is
   required for VM cost estimation.

2. **Given** a resource description missing the SKU field,
   **When** the system attempts to map it to a pricing query filter,
   **Then** an error is returned indicating that "sku" is required
   for cost estimation.

3. **Given** a resource description with empty `region` field
   but `tags["region"]` set to "eastus",
   **When** the system maps it to a pricing query filter,
   **Then** the filter uses "eastus" from the tag fallback.

4. **Given** a resource description missing both region and SKU
   in primary fields and tags,
   **When** the system attempts to map it to a pricing query
   filter,
   **Then** the error message identifies all missing required
   fields (not just the first one).

---

### User Story 4 - Handle Unsupported Resource Types (Priority: P3)

When the cost estimation system receives a resource description for a resource type
that is not yet supported (e.g., networking, databases), it returns a clear "not
implemented" response indicating which resource type is unsupported, so that callers
know the limitation rather than receiving a confusing generic error.

**Why this priority**: Graceful handling of unsupported types is important for a
production system but not blocking for the initial VM and disk use cases.

**Independent Test**: Can be fully tested by providing a resource
description with an unknown resource type and verifying that a clear
"unimplemented" response is returned with the unsupported type name.

**Acceptance Scenarios**:

1. **Given** a resource description with resource type "network/LoadBalancer",
   **When** the system attempts to map it to a pricing query filter,
   **Then** an "unimplemented" response is returned that includes
   the resource type name.

2. **Given** a resource description with a completely unknown
   resource type "custom/Widget",
   **When** the system attempts to map it to a pricing query filter,
   **Then** the response clearly indicates this resource type is not supported.

---

### Edge Cases

- What happens when a resource description has an empty string
  for SKU or region (present but blank)?
- Non-azure providers (e.g., "aws") return the same
  "unimplemented" response as unsupported resource types.
- Resource type matching is case-insensitive; casing
  variations (e.g., "Compute/VirtualMachine") are handled.
- When tags and primary fields both contain region or SKU,
  primary fields take precedence (tags are fallback only).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST translate virtual machine resource descriptions into
  pricing query filters targeting the "Virtual Machines" service with the correct
  ARM region name and ARM SKU name.
- **FR-002**: System MUST translate disk/storage resource
  descriptions into pricing query filters using the correct
  Azure service name per subtype: "Managed Disks" for managed
  disk resources, "Storage" for blob/file storage resources.
- **FR-003**: System MUST validate that required fields (region,
  SKU) are resolvable from the resource description — either
  from primary fields or tag fallback — before attempting to
  build a pricing query filter.
- **FR-004**: System MUST return a descriptive error when required
  fields are missing, identifying all missing fields in a single
  error (not fail-fast on the first missing field).
- **FR-005**: System MUST return an "unimplemented" response for
  resource types that do not have a defined mapping or for
  non-azure providers, including the unsupported resource type
  or provider name in the response.
- **FR-006**: System MUST default to "USD" currency when no currency preference is
  specified in the resource description.
- **FR-007**: System MUST default to "Consumption" pricing type
  for all query filters unless explicitly overridden.
  *Note: This default is enforced by the existing
  `FilterBuilder.Build()` method (see `azureclient/filter.go`),
  not by `MapDescriptorToQuery`. The mapper produces a
  `PriceQuery` which has no `PriceType` field; the
  downstream `FilterBuilder` applies the Consumption default
  when constructing the OData `$filter` string.*
- **FR-008**: System MUST preserve Azure SKU names exactly as
  provided (no normalization or transformation of SKU strings).
- **FR-009**: System MUST map resource type identifiers to
  the corresponding Azure service names:
  "compute/VirtualMachine" → "Virtual Machines",
  "storage/ManagedDisk" → "Managed Disks",
  "storage/BlobStorage" → "Storage".
- **FR-010**: System MUST handle empty-string field values the
  same as missing fields (treat blank SKU or region as absent).
- **FR-011**: System MUST use `tags["region"]` as fallback when
  the primary `region` field is empty, and `tags["sku"]` as
  fallback when the primary `sku` field is empty. Primary
  fields always take precedence over tags.
- **FR-012**: System MUST perform case-insensitive matching on
  resource type identifiers (e.g., "Compute/VirtualMachine"
  and "compute/virtualmachine" both match the VM mapping).

### Key Entities

- **Resource Description**: An incoming description of a cloud resource containing
  provider, resource type, SKU, region, and optional tags.
  Represents the "what" a
  caller wants to estimate costs for.
- **Pricing Query Filter**: The translated set of field-value pairs needed to query
  Azure pricing. Contains Azure-specific service name, ARM region name, ARM SKU name,
  currency, and price type.
- **Resource Type Map**: A lookup table that associates resource type identifiers
  (e.g., "compute/VirtualMachine") with Azure service names (e.g., "Virtual Machines")
  and field extraction rules.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of VM resource descriptions with valid SKU and region produce
  correct pricing query filters on first attempt (zero mismatches in field mapping).
- **SC-002**: 100% of managed disk resource descriptions with valid disk type and
  region produce correct pricing query filters on first attempt.
- **SC-003**: Resource descriptions missing required fields return errors that name
  every missing field in a single response (no partial error reporting).
- **SC-004**: Unsupported resource types return a clear "unimplemented" indication
  within the same response cycle (no silent failures or hangs).
- **SC-005**: All mapping logic achieves at least 80% test
  coverage with unit tests covering each supported resource
  type, each validation rule, and each edge case.
- **SC-006**: Mapping from resource description to pricing query filter completes
  in under 1 millisecond (pure data transformation, no external calls).

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations
  (>=80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (external API interactions)
- [x] Performance test criteria specified (if applicable)

### User Experience

- [x] Error messages are user-friendly and actionable
- [x] Response time expectations defined (e.g., cache hits <10ms, API calls <2s p95)
- [x] Observability requirements specified (logging, metrics)

### Documentation

- [x] README.md updates identified (if user-facing changes)
- [x] API documentation needs outlined (godoc comments, contracts)
- [x] Docstring coverage >=80% maintained (all exported symbols documented)
- [x] Examples/quickstart guide planned (if new capability)

### Performance & Reliability

- [x] Performance targets specified (response times, throughput)
- [x] Reliability requirements defined (retry logic, error handling)
- [x] Resource constraints considered (memory, connections, cache TTL)

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data

## Assumptions

- Resource type identifiers follow the "category/ResourceName" convention
  (e.g., "compute/VirtualMachine", "storage/ManagedDisk") as established in the
  finfocus-spec protobuf definitions.
- Azure SKU names are passed through as-is from the resource description to the
  pricing query filter. No SKU normalization or alias resolution is in scope.
- Only "azure" provider resources are handled by this plugin. Resource descriptions
  with other providers are out of scope and should be rejected early.
- The default currency (USD) and default pricing type (Consumption) are consistent
  with the existing pricing client behavior and do not need to be configurable
  at this stage.
- The initial release supports only two resource types (VM, Disk). Additional types
  (networking, databases, etc.) will be added in future iterations.
