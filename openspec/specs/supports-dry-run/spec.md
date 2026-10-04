# supports-dry-run Specification

## Purpose

Tell FinFocus core whether the plugin can price a resource, and let `finfocus plugin inspect`
validate a descriptor and read the FOCUS field mappings, without calling the Azure Retail Prices
API.

## Requirements

### Requirement: Supports accepts complete supported descriptors

`Supports` SHALL return `supported=true` for a descriptor with provider `azure`, a supported type,
a region, and a SKU, including when the SKU is only in `attributes` (native SQL `sku.name` plus
`sku.capacity`).

Tests: `TestSupports_ValidVM_ReturnsTrue`, `TestSupports_ManagedDisk_ReturnsTrue`,
`TestSupportsAndDryRun_SkuOnlyInAttributes_AreSupported`

#### Scenario: Managed disk

- **WHEN** `Supports` receives provider `azure`, type `storage/ManagedDisk`, SKU `Premium_LRS`,
  region `westus2`
- **THEN** `supported` is true

### Requirement: Supports reports unsupported with a reason instead of an error

`Supports` SHALL NOT return an error for a descriptor it cannot price. It SHALL return
`supported=false` with a non-empty reason for a nil request, a descriptor missing region and SKU,
an unknown type (the reason names the type), a descriptor with no provider, attributes over the
byte limit (the reason names `byte limit`), and tags over the count or length limit (the reason
names the limit).

Tests: `TestSupportsWithNilRequest`, `TestSupports_IncompleteDescriptor_ReturnsFalse`,
`TestSupports_UnsupportedType_ReturnsFalse`, `TestSupports_EmptyProvider_StaysUnsupported`,
`TestOversizeAttributes_EachRPC_RejectsOrReportsUnsupported`,
`TestDescriptorLimits_EachRPC_RejectOverLimitTags`

#### Scenario: Unknown type

- **WHEN** `Supports` receives type `custom/Widget`
- **THEN** no error is returned
- **AND** `supported` is false and the reason contains `custom/Widget`

#### Scenario: Provider missing

- **WHEN** `Supports` receives `compute/VirtualMachine` with region and SKU but no provider
- **THEN** `supported` is false

### Requirement: DryRun request validation and delegation

`DryRun` SHALL return `InvalidArgument` and no response for a nil request or a request with no
resource. `DryRun` SHALL return the same response as `HandleDryRun` for the same request.

Tests: `TestHandleDryRunNilResource`, `TestDryRunDelegatesToHandleDryRun`

#### Scenario: Nil resource

- **WHEN** `DryRun` receives a request with no resource
- **THEN** the status is `InvalidArgument` and the response is nil

### Requirement: DryRun never calls Azure

`DryRun` SHALL NOT send any request to the Azure Retail Prices API, whether called directly or
over gRPC through `pluginsdk.NewServer`. For a complete descriptor of every supported type it SHALL
report `resource_type_supported=true`, `configuration_valid=true`, and no configuration errors. The
response SHALL NOT contain the OData filter (`armRegionName`, `priceType eq`, `$filter`).

Tests: `TestHandleDryRunEverySupportedType`, `TestDryRunOverGRPCDoesNotCallHTTP`

#### Scenario: Every supported type over gRPC

- **WHEN** `DryRun` is called over gRPC for each type in `SupportedResourceTypes`
- **THEN** each response is supported and valid
- **AND** the price server receives no request

### Requirement: Known type with missing fields

A supported type with missing identity fields SHALL return `resource_type_supported=true`,
`configuration_valid=false`, and `configuration_errors` naming the missing fields. Attributes over
the byte limit or tags over the limits SHALL also give a supported type with an invalid
configuration.

Tests: `TestHandleDryRunMissingFields`,
`TestHandleDryRun_PluginInspectRequest_ReturnsFieldMappings`,
`TestOversizeAttributes_EachRPC_RejectsOrReportsUnsupported`,
`TestDescriptorLimits_EachRPC_RejectOverLimitTags`

#### Scenario: VM without region and SKU

- **WHEN** `DryRun` receives provider `azure` and type `compute/VirtualMachine` only
- **THEN** the type is supported, the configuration is not valid
- **AND** `configuration_errors` mention `region` and `sku`

### Requirement: Unsupported type is a response, not an error

An unknown type, or a provider or type token from another cloud, SHALL return a response with
`resource_type_supported=false`, `configuration_valid=true`, no configuration errors, and no field
mappings.

Tests: `TestHandleDryRunUnsupported`, `TestHandleDryRun_EmptyProviderOtherCloud_IsUnsupported`

#### Scenario: Another cloud's token with no provider

- **WHEN** `DryRun` receives only type `aws:ec2/instance:Instance`
- **THEN** no error is returned and `resource_type_supported` is false

### Requirement: Empty provider is inferred from the type token

When the descriptor provider is empty, `DryRun` SHALL infer provider `azure` from an `azure:` or
`azure-native:` token or a bare canonical type, so the type-only request that
`finfocus plugin inspect <plugin> <type>` sends is a supported type with field mappings and a
configuration error naming `region`.

Tests: `TestDryRunDescriptor_EmptyProvider_InfersAzureForEveryPackage`,
`TestHandleDryRun_PluginInspectRequest_ReturnsFieldMappings`

#### Scenario: plugin inspect request

- **WHEN** `DryRun` is called over gRPC with only type `azure-native:documentdb:DatabaseAccount`
- **THEN** `resource_type_supported` is true and `configuration_valid` is false
- **AND** `configuration_errors` mention `region`

### Requirement: FOCUS field mappings

For a supported type, `DryRun` SHALL return one mapping per name in `pluginsdk.FocusFieldNames()`,
with no duplicates and no condition description. Exactly `billed_cost`, `billing_currency`,
`charge_description`, `list_unit_price`, and `pricing_category` SHALL be `SUPPORTED`; every other
field, including `provider_name`, `invoice_id`, `effective_cost`, `service_name`, and `region_id`,
SHALL be `UNSUPPORTED`.

Tests: `TestHandleDryRunEverySupportedType`, `TestHandleDryRunMissingFields`

#### Scenario: Supported subset

- **WHEN** `DryRun` succeeds for `compute/VirtualMachine`
- **THEN** five mappings are `SUPPORTED` and `provider_name` is `UNSUPPORTED`
