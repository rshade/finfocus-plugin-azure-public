# Data Model: ResourceDescriptor to Azure Filter Mapping

**Feature**: 016-descriptor-filter-mapping
**Date**: 2026-03-03

## Entities

### 1. ResourceDescriptor (Input — finfocus-spec v0.5.4)

**Source**: `github.com/rshade/finfocus-spec/sdk/go/proto/
finfocus/v1.ResourceDescriptor`

| Field | Type | Req | Description |
| --- | --- | --- | --- |
| `Provider` | `string` | Yes | "azure", "aws", "gcp" |
| `ResourceType` | `string` | Yes | "compute/VirtualMachine" |
| `Sku` | `string` | No | "Standard_B1s" |
| `Region` | `string` | No | "eastus" |
| `Tags` | `map[string]string` | No | Fallback values |
| `Id` | `string` | No | Correlation ID (unused) |
| `Arn` | `string` | No | Resource ID (unused) |

**Validation rules**:

- `Provider` must be "azure" (case-insensitive); others →
  `ErrUnsupportedResourceType`
- `ResourceType` must match a known mapping
  (case-insensitive); unknown → `ErrUnsupportedResourceType`
- `Region` must be resolvable (primary field or
  `Tags["region"]`); blank = missing
- `Sku` must be resolvable (primary field or
  `Tags["sku"]`); blank = missing
- All missing required fields reported in a single error

### 2. PriceQuery (Output — internal/azureclient)

**Source**: `internal/azureclient.PriceQuery`

| Field | Type | Source | Default |
| --- | --- | --- | --- |
| `ArmRegionName` | `string` | `Region` or tag | (required) |
| `ArmSkuName` | `string` | `Sku` or tag | (required) |
| `ServiceName` | `string` | Type lookup | (required) |
| `CurrencyCode` | `string` | (not in desc) | "USD" |
| `ProductName` | `string` | (not used) | "" |

**Note on PriceType**: `PriceQuery` has no `PriceType` field.
The "Consumption" default (FR-007) is enforced downstream by
`FilterBuilder.Build()` in `azureclient/filter.go`, which
always includes `priceType eq 'Consumption'` unless
explicitly overridden via `.Type()`.

### 3. Resource Type Registry (Static Lookup)

**Location**: Package-level variable in
`internal/pricing/mapper.go`

| Key (normalized) | Azure Service Name |
| --- | --- |
| `compute/virtualmachine` | `Virtual Machines` |
| `storage/manageddisk` | `Managed Disks` |
| `storage/blobstorage` | `Storage` |

**Normalization**: `strings.ToLower(resourceType)` before
lookup.

### 4. Error Types (New sentinel errors)

| Error | Condition | gRPC Code |
| --- | --- | --- |
| `ErrUnsupportedResourceType` | Unknown type / non-azure | `Unimplemented` |
| `ErrMissingRequiredFields` | Region/SKU not resolvable | `InvalidArgument` |

## Relationships

```text
ResourceDescriptor
    │
    ├─ .Provider ──(validate "azure")
    │
    ├─ .ResourceType ──(lowercase)──→ resourceTypeToService
    │                                      │
    │                                      ▼
    │                              PriceQuery.ServiceName
    │
    ├─ .Region ──resolveField()──→ PriceQuery.ArmRegionName
    │    └─ .Tags["region"] (fallback)
    │
    └─ .Sku ──resolveField()──→ PriceQuery.ArmSkuName
         └─ .Tags["sku"] (fallback)

    (default "USD") ──→ PriceQuery.CurrencyCode
```

## State Transitions

N/A — pure stateless data transformation. No state machine.

## Downstream Usage

```text
gRPC Request (Supports, EstimateCost, DryRun)
    │
    ▼
Calculator (pricing/calculator.go)
    │ extracts ResourceDescriptor from request
    ▼
MapDescriptorToQuery (pricing/mapper.go)  ← THIS FEATURE
    │ returns PriceQuery or error
    ▼
azureclient.Client.GetPrices(ctx, query)  ← existing
    │ returns []PriceItem
    ▼
Cost calculation / response building      ← future
```
