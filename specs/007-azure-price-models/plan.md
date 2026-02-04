# Implementation Plan: Azure Retail Prices API Data Models

**Branch**: `007-azure-price-models` | **Date**: 2026-02-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/007-azure-price-models/spec.md`

## Summary

Enhance existing Azure Retail Prices API data models in `internal/azureclient/types.go` to:

1. Add missing fields documented in Azure API (`location`, `reservationTerm`, `availabilityId`)
2. Export `PriceResponse` struct for external package usage
3. Add comprehensive godoc documentation for all fields
4. Add dedicated unit tests for JSON marshaling/unmarshaling

The models already exist with 18 fields implemented. Adding 3 missing fields brings total to 21. This is an enhancement, not a greenfield implementation.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: `encoding/json` (stdlib), `github.com/rs/zerolog` (logging)
**Storage**: N/A - stateless plugin (in-memory only)
**Testing**: `go test` with table-driven tests, race detector
**Target Platform**: Linux server (gRPC plugin)
**Project Type**: Single project (Go module)
**Performance Goals**: Standard JSON marshaling (<1ms for typical responses)
**Constraints**: No authenticated Azure APIs, no persistent storage
**Scale/Scope**: ~20 fields per PriceItem, responses up to 1000 items

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with `.specify/memory/constitution.md`:

- [x] **Code Quality**: Plan includes linting checks (`make lint`), error handling via JSON decode errors, no complex functions (data structs only)
- [x] **Testing**: Plan includes TDD workflow (tests first), ≥80% coverage target for models, no concurrent code requiring race detector
- [x] **User Experience**: N/A for data models - no plugin lifecycle changes
- [x] **Documentation**: Plan includes godoc comments for all exported types and fields
- [x] **Performance**: Standard JSON marshaling performance, no special optimization needed
- [x] **Architectural Constraints**: Plan DOES NOT violate "Hard No's":
  - ✅ No authenticated Azure APIs (models only, no API calls)
  - ✅ No persistent storage (pure data structures)
  - ✅ No infrastructure mutation (read-only types)
  - ✅ No bulk data embedding (types, not data)

## Project Structure

### Documentation (this feature)

```text
specs/007-azure-price-models/
├── plan.md              # This file
├── research.md          # Phase 0 output - Azure API field analysis
├── data-model.md        # Phase 1 output - struct definitions
├── quickstart.md        # Phase 1 output - usage examples
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
└── azureclient/
    ├── types.go         # MODIFY: Add missing fields, export PriceResponse, add godoc
    └── types_test.go    # CREATE: Dedicated JSON marshal/unmarshal tests
```

**Structure Decision**: Modify existing `internal/azureclient/types.go` file. The models are already co-located with the HTTP client that uses them. No new packages needed.

## Complexity Tracking

No violations. This is a straightforward enhancement of existing data structures.

## Existing Code Analysis

### Current State (`internal/azureclient/types.go`)

**Implemented:**

- `Config` struct - client configuration (complete)
- `PriceQuery` struct - query parameters (complete)
- `PriceItem` struct - 18 fields implemented
- `priceResponse` struct - unexported, has all envelope fields

**Missing from PriceItem (per Azure API docs):**

| Field | Type | Status |
|-------|------|--------|
| `location` | string | Missing |
| `availabilityId` | *string | Missing (nullable) |
| `reservationTerm` | string | Missing (optional for reservations) |
| `savingsPlan` | []SavingsPlanPrice | Missing (preview API only) |

**Issues to Address:**

1. `priceResponse` is unexported → Export as `PriceResponse`
2. No godoc comments on individual fields → Add comprehensive documentation
3. No dedicated unit tests for marshaling → Create `types_test.go`

## Implementation Tasks

### Task 1: Add Missing Fields to PriceItem

Add the following fields to `PriceItem`:

```go
// Location is the human-readable Azure datacenter location (e.g., "US East").
Location string `json:"location"`

// AvailabilityID is the optional availability identifier.
// May be empty or null in API responses.
AvailabilityID string `json:"availabilityId,omitempty"`

// ReservationTerm is the commitment term for reservation pricing.
// Only present when Type is "Reservation" (e.g., "1 Year", "3 Years").
ReservationTerm string `json:"reservationTerm,omitempty"`
```

### Task 2: Export PriceResponse

Rename `priceResponse` to `PriceResponse` and add godoc:

```go
// PriceResponse represents the envelope returned by the Azure Retail Prices API.
// It contains a paginated list of price items along with metadata.
//
// Example JSON response:
//
//	{
//	  "BillingCurrency": "USD",
//	  "CustomerEntityId": "Default",
//	  "CustomerEntityType": "Retail",
//	  "Items": [...],
//	  "NextPageLink": "https://prices.azure.com/...",
//	  "Count": 100
//	}
type PriceResponse struct {
    // BillingCurrency is the currency code for all prices in the response.
    BillingCurrency string `json:"BillingCurrency"`

    // CustomerEntityID identifies the customer entity.
    CustomerEntityID string `json:"CustomerEntityId"`

    // CustomerEntityType indicates the type of pricing (e.g., "Retail").
    CustomerEntityType string `json:"CustomerEntityType"`

    // Items contains the price entries for this page of results.
    Items []PriceItem `json:"Items"`

    // NextPageLink is the URL to fetch the next page of results.
    // Empty string indicates no more pages.
    NextPageLink string `json:"NextPageLink"`

    // Count is the number of items in this page.
    Count int `json:"Count"`
}
```

### Task 3: Add Godoc Comments to All PriceItem Fields

Add descriptive comments to every field explaining:

- What the field represents
- Example values where helpful
- Any constraints or optionality

### Task 4: Create types_test.go

Create dedicated tests for JSON marshaling:

1. `TestPriceItem_UnmarshalJSON` - Verify all fields populated from sample JSON
2. `TestPriceItem_MarshalJSON` - Verify round-trip produces equivalent JSON
3. `TestPriceItem_MissingOptionalFields` - Verify graceful handling of nulls/missing
4. `TestPriceResponse_UnmarshalJSON` - Verify envelope parsing
5. `TestPriceResponse_Pagination` - Verify NextPageLink handling

### Task 5: Update client.go References

Update `client.go` to use exported `PriceResponse` instead of `priceResponse`.

## Test Strategy

### Unit Tests (types_test.go)

```go
func TestPriceItem_UnmarshalJSON(t *testing.T) {
    // Table-driven test with real Azure API sample JSON
    // Verify all 21+ fields are correctly populated
}

func TestPriceItem_RoundTrip(t *testing.T) {
    // Create struct → marshal → unmarshal → compare
    // Ensures JSON tags are bidirectionally correct
}

func TestPriceItem_MissingOptionalFields(t *testing.T) {
    // JSON without reservationTerm, availabilityId
    // Verify no error, fields are zero values
}

func TestPriceResponse_UnmarshalJSON(t *testing.T) {
    // Full response envelope with Items array
    // Verify Count, NextPageLink, Items populated
}
```

### Integration Tests

Existing integration tests in `examples/` already cover end-to-end API calls. No new integration tests needed for data models.

## Success Metrics

| Criteria | Target | Verification |
|----------|--------|--------------|
| Field accuracy | 100% | All Azure API fields mapped |
| Round-trip fidelity | 100% | Unmarshal→Marshal produces equivalent JSON |
| Test coverage | ≥80% | `go test -cover ./internal/azureclient/...` |
| Godoc quality | All fields | Manual review of `go doc` output |
| Lint compliance | 0 errors | `make lint` passes |

## Dependencies

None. This feature has no external dependencies beyond stdlib `encoding/json`.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Azure API adds new fields | Medium | Low | Use `json:",omitempty"` for optional fields; unknown fields ignored by default |
| Breaking change to client.go | Low | Medium | Rename is internal; update all references in same commit |
| Field type mismatch | Low | High | Validate against real API responses in integration tests |
