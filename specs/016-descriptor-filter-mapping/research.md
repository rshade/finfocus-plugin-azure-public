# Research: ResourceDescriptor to Azure Filter Mapping

**Feature**: 016-descriptor-filter-mapping
**Date**: 2026-03-03

## Research Task 1: Package Placement for Mapping Logic

**Context**: Where should the `ResourceDescriptor → PriceQuery`
translation live?

**Decision**: `internal/pricing/mapper.go` within the existing
`pricing` package.

**Rationale**:

- The `Calculator` (gRPC handler) in `pricing/calculator.go`
  is the direct consumer of the mapper — it receives
  `ResourceDescriptor` from gRPC requests and needs
  `PriceQuery` to call `azureclient.Client.GetPrices`.
- Creating a separate package (`internal/mapper` or
  `internal/pricing/mapper`) adds import complexity without
  meaningful separation of concerns.
- The mapping is a pricing-domain concept — translating
  "what resource?" into "what price query?" — so it belongs
  in the pricing package.
- File size will be well under 300 lines (simple registry +
  validation + mapping function).

**Alternatives considered**:

- `internal/mapper` — rejected: unnecessary package for a
  single function; creates circular dependency risk if it
  needs pricing-specific types.
- `internal/pricing/mapper/` subpackage — rejected:
  over-engineering for 3 resource types; Go convention
  prefers flat packages.

## Research Task 2: ResourceDescriptor Field Mapping

**Context**: How to map `finfocusv1.ResourceDescriptor` fields
to `azureclient.PriceQuery`?

**Decision**: Direct field access with tag fallback,
table-driven resource type lookup.

**Rationale**:

`ResourceDescriptor` (from finfocus-spec v0.5.4 proto) has
first-class fields:

| ResourceDescriptor | PriceQuery | Notes |
|---|---|---|
| `.Region` | `.ArmRegionName` | Fallback: `Tags["region"]` |
| `.Sku` | `.ArmSkuName` | Fallback: `Tags["sku"]` |
| resource type → lookup | `.ServiceName` | Case-insensitive |
| (default "USD") | `.CurrencyCode` | Not in descriptor |

The `pluginsdk/mapping` package (`ExtractAzureSKU`,
`ExtractAzureRegion`) works on `map[string]string` property
bags — useful for `EstimateCostRequest.Attributes` but NOT for
`ResourceDescriptor` which has typed fields. Direct field
access is simpler and type-safe.

**Alternatives considered**:

- Use `pluginsdk/mapping.ExtractAzureSKU/Region` by converting
  tags to property map — rejected: adds unnecessary
  indirection; ResourceDescriptor already has `.Sku` and
  `.Region` as first-class fields.

## Research Task 3: Resource Type Registry Pattern

**Context**: How to map resource type strings to Azure service
names?

**Decision**: Package-level `map[string]string` with normalized
(lowercased) keys.

**Rationale**:

```go
var resourceTypeToService = map[string]string{
    "compute/virtualmachine": "Virtual Machines",
    "storage/manageddisk":    "Managed Disks",
    "storage/blobstorage":    "Storage",
}
```

- Keys are lowercased at definition time; input is lowercased
  before lookup (FR-012).
- Simple `map` lookup is O(1) and trivially extensible —
  adding a new resource type is a single map entry.
- No interface or registry pattern needed — this is a static
  mapping that changes only at compile time.

**Alternatives considered**:

- `switch` statement — rejected: less extensible, harder to
  test exhaustively.
- Interface-based registry with `Register()` — rejected:
  over-engineering for static mappings; no runtime
  registration needed.

## Research Task 4: Multi-Field Validation Error Pattern

**Context**: How to report all missing fields in a single error
(FR-004)?

**Decision**: Accumulate missing field names in a `[]string`,
join into single error.

**Rationale**:

```go
var missing []string
if region == "" {
    missing = append(missing, "region")
}
if sku == "" {
    missing = append(missing, "sku")
}
if len(missing) > 0 {
    return nil, fmt.Errorf(
        "missing required fields: %s",
        strings.Join(missing, ", "),
    )
}
```

- Simple, idiomatic Go — no error accumulator library needed.
- Produces clear, actionable error messages:
  `"missing required fields: region, sku"`.
- Aligns with FR-004: "identifying all missing fields in a
  single error".

**Alternatives considered**:

- `errors.Join` (Go 1.20+) — rejected: produces multi-line
  output; a single formatted string is more readable for
  gRPC status messages.
- Custom `ValidationError` type — rejected: over-engineering;
  `fmt.Errorf` is sufficient for the 2-field validation case.

## Research Task 5: Provider Validation Strategy

**Context**: How to handle non-azure providers (FR-005)?

**Decision**: Check `Provider` field first; return sentinel
error for non-"azure" providers.

**Rationale**:

- Per clarification: non-azure providers return same
  "unimplemented" response as unsupported resource types.
  No distinct "wrong provider" error needed.
- Check provider before resource type — fail fast on wrong
  provider.
- Case-insensitive comparison on provider string (consistent
  with resource type matching).
- Use a dedicated sentinel error `ErrUnsupportedResourceType`
  that includes the unsupported type name, mappable to gRPC
  `codes.Unimplemented` via `MapToGRPCStatus`.

**Alternatives considered**:

- Separate `ErrUnsupportedProvider` vs
  `ErrUnsupportedResourceType` — rejected per clarification:
  "No distinct 'wrong provider' error needed."

## Research Task 6: Tag Fallback Behavior

**Context**: How to implement tag fallback for region and SKU
(FR-011)?

**Decision**: Helper function
`resolveField(primary, tagKey, tags)` that returns the
resolved value or empty string.

**Rationale**:

```go
func resolveField(
    primary, tagKey string,
    tags map[string]string,
) string {
    if primary != "" {
        return primary
    }
    if tags != nil {
        return tags[tagKey]
    }
    return ""
}
```

- Primary fields always take precedence (FR-011).
- Empty strings treated as missing (FR-010).
- Tags map may be nil (proto default), so nil-check required.
- Single helper avoids duplicating the fallback logic for
  region and SKU.

**Alternatives considered**:

- Inline the fallback logic — rejected: duplicates 4 lines
  for each field; a helper is justified for DRY without being
  a premature abstraction.
