# Feature Specification: Azure Retail Prices API Data Models

**Feature Branch**: `007-azure-price-models`
**Created**: 2026-02-03
**Status**: Draft
**Input**: GitHub Issue #8 - Define Go structs for Azure Retail Prices API request/response data structures with proper JSON marshaling tags.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - HTTP Client Response Parsing (Priority: P1)

As the HTTP client, I want to unmarshal JSON responses from the Azure Retail Prices API into strongly-typed Go structs so that I can work with pricing data in a type-safe manner.

**Why this priority**: This is the foundational capability. Without proper response parsing, no pricing queries can function. All downstream features depend on this.

**Independent Test**: Can be fully tested by providing sample JSON from the Azure API and verifying all fields are correctly populated in the Go structs.

**Acceptance Scenarios**:

1. **Given** a valid JSON response from Azure Retail Prices API, **When** the response is unmarshaled into PriceResponse, **Then** all fields (Items, NextPageLink, Count) are correctly populated
2. **Given** a JSON response with multiple pricing items, **When** unmarshaled, **Then** each PriceItem contains correct values for retailPrice, currencyCode, armRegionName, and all other fields
3. **Given** a JSON response with a NextPageLink, **When** unmarshaled, **Then** the NextPageLink field contains the pagination URL for fetching additional results

---

### User Story 2 - Type-Safe Field Access (Priority: P1)

As cost estimation logic, I want type-safe access to pricing fields so that I can perform calculations without runtime type errors.

**Why this priority**: Equally critical as parsing - type safety prevents calculation errors and enables IDE support for developers.

**Independent Test**: Can be tested by writing code that accesses all pricing fields and verifying compile-time type checking works correctly.

**Acceptance Scenarios**:

1. **Given** a populated PriceItem struct, **When** accessing retailPrice, **Then** the value is a float64 suitable for arithmetic operations
2. **Given** a populated PriceItem struct, **When** accessing string fields (currencyCode, armRegionName, etc.), **Then** values are properly typed strings
3. **Given** a PriceItem with all fields populated, **When** performing price calculations, **Then** no type assertions or conversions are required

---

### User Story 3 - Mock Data Construction (Priority: P2)

As a test writer, I want to construct mock pricing data easily so that I can write comprehensive tests without calling the live Azure API.

**Why this priority**: Important for testing but builds on the foundation of the struct definitions from P1 stories.

**Independent Test**: Can be tested by creating PriceItem structs with known values and marshaling them to JSON to verify correctness.

**Acceptance Scenarios**:

1. **Given** a need for test data, **When** constructing a PriceItem struct literal, **Then** all fields can be easily populated with test values
2. **Given** a constructed PriceItem, **When** marshaled to JSON, **Then** the output matches the expected Azure API format
3. **Given** a PriceResponse with mock items, **When** used in tests, **Then** it behaves identically to real API responses

---

### User Story 4 - Field Documentation (Priority: P3)

As a developer, I want struct tags and godoc comments documenting field meanings so that I understand what each field represents without consulting external documentation.

**Why this priority**: Documentation improves developer experience but is not required for functionality.

**Independent Test**: Can be verified by reviewing godoc output and ensuring each field has descriptive comments.

**Acceptance Scenarios**:

1. **Given** the PriceItem struct definition, **When** viewing godoc, **Then** each field has a descriptive comment explaining its purpose
2. **Given** the PriceResponse struct, **When** viewing example documentation, **Then** a sample JSON response is included showing the expected format

---

### Edge Cases

- What happens when optional fields are missing from the JSON response? (System should handle gracefully with zero values)
- How does the system handle unknown/additional fields in the response? (JSON unmarshaling should ignore unknown fields)
- What happens with null values in numeric fields? (Should default to zero value for the type)
- How are pagination responses handled when there are no more pages? (NextPageLink should be empty string)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST define a `PriceResponse` struct with fields: Items ([]PriceItem), NextPageLink (string), Count (int)
- **FR-002**: System MUST define a `PriceItem` struct containing all essential pricing fields from the Azure Retail Prices API
- **FR-003**: All struct fields MUST have JSON struct tags matching the exact field names in the Azure API response (camelCase)
- **FR-004**: Numeric fields (retailPrice, unitPrice, tierMinimumUnits) MUST be typed as float64
- **FR-005**: String identifier fields (meterId, productId, skuId) MUST be typed as string
- **FR-006**: Date fields (effectiveStartDate) MUST be typed as string to preserve ISO8601 format
- **FR-007**: System MUST support JSON round-trip (unmarshal then marshal produces equivalent JSON)
- **FR-008**: System MUST handle missing optional fields gracefully during unmarshaling (use Go zero values)

### Key Entities

- **PriceResponse**: Top-level container for API responses. Contains a collection of pricing items, pagination link for fetching additional results, and total count of items.
- **PriceItem**: Individual pricing record representing a single SKU/meter combination. Contains pricing information (retailPrice, unitPrice), resource identification (armRegionName, productName, skuName), service categorization (serviceName, serviceFamily), and billing metadata (unitOfMeasure, type).

### PriceItem Required Fields

Based on Azure Retail Prices API documentation:

| Field               | Go Type | JSON Tag              | Description                                        |
| ------------------- | ------- | --------------------- | -------------------------------------------------- |
| currencyCode        | string  | `currencyCode`        | ISO currency code (e.g., "USD")                    |
| tierMinimumUnits    | float64 | `tierMinimumUnits`    | Minimum units for tiered pricing                   |
| retailPrice         | float64 | `retailPrice`         | Listed retail price                                |
| unitPrice           | float64 | `unitPrice`           | Price per unit of measure                          |
| armRegionName       | string  | `armRegionName`       | Azure region identifier (e.g., "eastus")           |
| location            | string  | `location`            | Human-readable location (e.g., "US East")          |
| effectiveStartDate  | string  | `effectiveStartDate`  | ISO8601 timestamp when price became effective      |
| meterId             | string  | `meterId`             | Unique meter identifier (UUID)                     |
| meterName           | string  | `meterName`           | Human-readable meter name                          |
| productId           | string  | `productId`           | Unique product identifier (UUID)                   |
| skuId               | string  | `skuId`               | Unique SKU identifier (UUID)                       |
| productName         | string  | `productName`         | Product display name                               |
| skuName             | string  | `skuName`             | SKU display name                                   |
| serviceName         | string  | `serviceName`         | Azure service name                                 |
| serviceFamily       | string  | `serviceFamily`       | Service category                                   |
| unitOfMeasure       | string  | `unitOfMeasure`       | Billing unit description                           |
| type                | string  | `type`                | Pricing type (e.g., "Consumption", "Reservation")  |
| isPrimaryMeterRegion| bool    | `isPrimaryMeterRegion`| Whether this is the primary region for the meter   |
| armSkuName          | string  | `armSkuName`          | ARM SKU identifier                                 |

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All JSON fields from the Azure Retail Prices API response are correctly mapped to Go struct fields with 100% accuracy
- **SC-002**: JSON unmarshaling succeeds for sample responses containing all documented field types
- **SC-003**: JSON round-trip (unmarshal → marshal → unmarshal) produces identical struct values
- **SC-004**: Missing optional fields in JSON responses result in Go zero values without errors
- **SC-005**: Unit test coverage for data models reaches at least 80%
- **SC-006**: All struct fields have descriptive godoc comments

## Constitution Compliance *(mandatory)*

### Quality Standards

- [x] Feature requirements include test coverage expectations (≥80% for business logic)
- [x] Error handling strategy is defined (no silent failures)
- [x] Code complexity is considered (functions <15 cyclomatic complexity)

### Testing Requirements

- [x] Test scenarios defined for all user stories (Given/When/Then format)
- [x] Integration test needs identified (external API interactions) - N/A for pure data models
- [x] Performance test criteria specified (if applicable) - N/A for data structures

### User Experience

- [x] Error messages are user-friendly and actionable - N/A for data models
- [x] Response time expectations defined - N/A for data structures
- [x] Observability requirements specified - N/A for data models

### Documentation

- [x] README.md updates identified - N/A (internal package)
- [x] API documentation needs outlined (godoc comments for all fields)
- [x] Examples/quickstart guide planned - Example JSON in godoc

### Performance & Reliability

- [x] Performance targets specified - Standard JSON marshaling performance
- [x] Reliability requirements defined - Graceful handling of missing fields
- [x] Resource constraints considered - No special constraints for data models

### Architectural Constraints Check

- [x] DOES NOT require authenticated Azure APIs
- [x] DOES NOT introduce persistent storage
- [x] DOES NOT mutate infrastructure
- [x] DOES NOT embed bulk pricing data

## Assumptions

- The Azure Retail Prices API response format is stable and follows the documented schema
- All UUID fields in the API response are string-formatted (not binary UUIDs)
- Date/time fields use ISO8601 string format and will be parsed as strings (not time.Time) to preserve exact format
- The `type` field uses "type" as the JSON key despite being a Go reserved word (handled with json tag)
- Pagination uses NextPageLink URL for fetching additional results; empty string indicates no more pages
