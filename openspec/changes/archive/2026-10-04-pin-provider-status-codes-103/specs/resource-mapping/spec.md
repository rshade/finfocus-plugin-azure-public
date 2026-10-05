## ADDED Requirements

### Requirement: Unpriced providers and types return documented status codes

Served through the SDK, `GetProjectedCost`, `GetActualCost`, and
`GetPricingSpec` SHALL return `InvalidArgument` with a message containing
`unsupported provider: <provider>` when the descriptor's provider is not `azure`
or `azure-native`. FinFocus core's conformance check accepts only `NotFound` or
`InvalidArgument` for such a resource. An empty provider SHALL be
`InvalidArgument` naming `provider`, and `GetActualCost` without a descriptor,
which reads the provider from the request tags, SHALL apply the same rules.
They SHALL return `Unimplemented` with a
message containing `unsupported resource type` for an Azure type the plugin does
not price, and `EstimateCost`, whose request carries only a resource type, SHALL
return `Unimplemented` for a type it does not price. `Supports` SHALL answer
unsupported with a reason, and `DryRun` SHALL report the type unsupported, without
an error, for both inputs.

Tests: `TestDescriptorRPCs_UnpricedInput_ReturnDocumentedCode`

#### Scenario: Core conformance AWS resource

- **WHEN** `GetProjectedCost` is called with provider `aws`, type `invalid:resource`, region `us-east-1`, SKU `non-existent`
- **THEN** the status is `InvalidArgument` with message `unsupported provider: aws`
- **AND** `GetActualCost` and `GetPricingSpec` return the same code

#### Scenario: Unknown Azure type

- **WHEN** a cost RPC is called with provider `azure` and type `azure:foo/bar:Baz`
- **THEN** the status is `Unimplemented` with a message containing `unsupported resource type`

#### Scenario: Supports declines instead of failing

- **WHEN** `Supports` is called with either input
- **THEN** it returns `supported=false` with a non-empty reason and no error

## MODIFIED Requirements

### Requirement: Unsupported types and providers

An unknown resource type, or a provider other than `azure` or `azure-native`, SHALL make
`MapDescriptorToQuery` return an error wrapping `ErrUnsupportedResourceType` that names the type or
provider. `MapToGRPCStatus` SHALL map that error, wrapped or not, to `Unimplemented` and keep the
message. The cost RPCs check the provider before the mapper runs, so this mapper error reaches a
caller only as the reason in a `Supports` or `DryRun` response; the codes the cost RPCs return are
in "Unpriced providers and types return documented status codes".

Tests: `TestMapDescriptorToQuery_UnsupportedResourceType`,
`TestMapDescriptorToQuery_NonAzureProvider`,
`TestMapToGRPCStatus`, `TestMapToGRPCStatus_WrappedUnsupportedResourceType`

#### Scenario: Unknown type

- **WHEN** the type is `custom/Widget`
- **THEN** the error wraps `ErrUnsupportedResourceType` and contains `custom/Widget`

#### Scenario: Another cloud

- **WHEN** the provider is `aws`
- **THEN** the error wraps `ErrUnsupportedResourceType` and contains `aws`
