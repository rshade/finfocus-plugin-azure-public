# resource-mapping Specification

## Purpose

Turn a FinFocus `ResourceDescriptor` into an Azure Retail Prices `PriceQuery`: accept the Azure
providers and Pulumi type tokens, resolve region and SKU from fields, tags, and attributes, and
reject incomplete or unsupported descriptors with the right error.

## Requirements

### Requirement: Supported types map to Azure services

`MapDescriptorToQuery` SHALL map each supported canonical resource type to its Azure service name
(`compute/VirtualMachine` to `Virtual Machines`, `storage/ManagedDisk` to `Managed Disks`,
`storage/BlobStorage` to `Storage`), SHALL pass the region and SKU through without normalization
(`Standard_D2s_v3` stays `Standard_D2s_v3`), and SHALL default the currency to `USD`.
`SupportedResourceTypes` SHALL list exactly the ten canonical types, sorted:
`compute/VirtualMachine`, `containerservice/KubernetesCluster`, `cosmosdb/Account`,
`network/LoadBalancer`, `sql/Database`, `storage/BlobStorage`, `storage/ManagedDisk`,
`storage/StorageAccount`, `web/AppServicePlan`, `web/FunctionApp`.

Tests: `TestMapDescriptorToQuery_ValidVM`, `TestMapDescriptorToQuery_DiskResources`,
`TestSupportedResourceTypes`

#### Scenario: Virtual machine query

- **WHEN** a descriptor has provider `azure`, type `compute/VirtualMachine`, region `eastus`, and
  SKU `Standard_B1s`
- **THEN** the query has `ArmRegionName` `eastus`, `ArmSkuName` `Standard_B1s`, service
  `Virtual Machines`, and currency `USD`

### Requirement: Case-insensitive types and Pulumi type tokens

Resource type matching SHALL be case-insensitive. The classifier SHALL accept classic (`azure:`)
and Azure Native (`azure-native:`) Pulumi type tokens for every supported type, including
`azure:compute/linuxVirtualMachine:LinuxVirtualMachine`,
`azure:compute/windowsVirtualMachine:WindowsVirtualMachine`, `azure-native:compute:VirtualMachine`,
`azure-native:compute:VirtualMachineScaleSet`, `azure:compute/managedDisk:ManagedDisk`,
`azure-native:compute:Disk`, `azure:storage/account:Account`,
`azure-native:storage:StorageAccount`, `azure:appservice/servicePlan:ServicePlan`,
`azure-native:web:AppServicePlan`, `azure:mssql/database:Database`, `azure-native:sql:Database`,
`azure-native:documentdb:DatabaseAccount`, `azure:lb/loadBalancer:LoadBalancer`, and
`azure-native:network:LoadBalancer`. `azure-native:web:WebApp` SHALL be a Function App only when
tag `kind` is `FunctionApp`; without it, and for `azure-native:networkcloud:VirtualMachine`, the
classifier SHALL return `Unimplemented`. Per-type Pulumi SKU properties (classic
`storageAccountType`, `skuName`, `skuTier`, `accountTier` plus `accountReplicationType`, native
`sku.tier`) SHALL satisfy the SKU requirement, while inputs the quote cannot price (native AKS
`Base` with no tier, `Automatic`, native SQL `GP_Gen5` with no vCore count, plan `osType`
`WindowsContainer`, `workerCount` 0, native storage `kind` `BlobStorage`) SHALL return an error.
A request SHALL get the same answer from every cost RPC whether its provider is `azure` or
`azure-native`.

Tests: `TestMapDescriptorToQuery_CaseInsensitiveResourceType`, `TestSupportsRealPulumiTokens`,
`TestAzureNativeVMQuoteMatchesAzure`, `TestCostRPCs_AzureNativeTokens_SameForBothProviders`,
`TestMapDescriptorToQuery_RealPulumiProperties_AcceptsPerTypeSKU`,
`TestMapDescriptorToQuery_UnpriceablePulumiInput_ReturnsError`

#### Scenario: Upper-case canonical type

- **WHEN** the resource type is `COMPUTE/VIRTUALMACHINE`
- **THEN** the query service is `Virtual Machines`

#### Scenario: Native web app that is not a function

- **WHEN** the type is `azure-native:web:WebApp` and tag `kind` is absent
- **THEN** `Supports` reports unsupported
- **AND** classification fails with `Unimplemented`

#### Scenario: Provider does not change the answer

- **WHEN** an Azure Native resource from the real Pulumi plan is sent with provider `azure` and
  again with provider `azure-native`
- **THEN** `GetProjectedCost`, `Supports`, `DryRun`, and `GetPricingSpec` return equal responses
  or the same error code and message

### Requirement: Tag fallback and primary-field precedence

When the descriptor `Region` or `Sku` is empty, the mapper SHALL read tag `region` or tag `sku`.
A non-empty primary field SHALL take precedence over the tag.

Tests: `TestMapDescriptorToQuery_TagFallback`,
`TestMapDescriptorToQuery_PrimaryFieldTakesPrecedence`

#### Scenario: Tags fill empty fields

- **WHEN** `Region` and `Sku` are empty and tags are `region=eastus`, `sku=Standard_B1s`
- **THEN** the query region is `eastus` and the SKU is `Standard_B1s`

#### Scenario: Primary fields win

- **WHEN** `Region` is `eastus`, `Sku` is `Standard_B1s`, and tags name `westus2` and
  `Standard_D2s_v3`
- **THEN** the query uses `eastus` and `Standard_B1s`

### Requirement: Missing fields are reported together as InvalidArgument

A nil descriptor, or one whose region or SKU is missing or empty after tag fallback, SHALL return
an error wrapping `ErrMissingRequiredFields` that names every missing field. The status SHALL be
`InvalidArgument` with message `missing required field(s): region, sku` when both are missing, and
the field list SHALL survive wrapping.

Tests: `TestMapDescriptorToQuery_MissingRegion`, `TestMapDescriptorToQuery_MissingSKU`,
`TestMapDescriptorToQuery_BothFieldsMissing`,
`TestMapDescriptorToQuery_EmptyStringTreatedAsMissing`,
`TestMapDescriptorToQuery_NilDescriptor`, `TestMissingFieldsError_Wrapped_KeepsStatusAndFields`,
`TestMapToGRPCStatus_WrappedMissingRequiredFields`

#### Scenario: Both fields missing

- **WHEN** a VM descriptor has no region, no SKU, and no tags
- **THEN** the error wraps `ErrMissingRequiredFields`
- **AND** the message names both `region` and `sku`

### Requirement: Unsupported types and providers

An unknown resource type, or a provider other than `azure` or `azure-native`, SHALL make
`MapDescriptorToQuery` return an error wrapping `ErrUnsupportedResourceType` that names the type or
provider. `MapToGRPCStatus` SHALL map that error, wrapped or not, to `Unimplemented` and keep the
message.

Tests: `TestMapDescriptorToQuery_UnsupportedResourceType`,
`TestMapDescriptorToQuery_NonAzureProvider`,
`TestMapToGRPCStatus`, `TestMapToGRPCStatus_WrappedUnsupportedResourceType`

#### Scenario: Unknown type

- **WHEN** the type is `custom/Widget`
- **THEN** the error wraps `ErrUnsupportedResourceType` and contains `custom/Widget`

#### Scenario: Another cloud

- **WHEN** the provider is `aws`
- **THEN** the error wraps `ErrUnsupportedResourceType` and contains `aws`

### Requirement: Pulumi unknown placeholder is skipped

The Pulumi unknown value `04da6b54-80e4-46f7-96ec-b56ff0331ba9` SHALL be treated as absent wherever
it appears: an attribute value containing it SHALL produce no tag, and a per-type SKU property
holding it SHALL fall through to the next source. A VM whose region or size is only that
placeholder SHALL fail with `InvalidArgument` naming `region` or `sku` without calling Azure.

Tests: `TestAttributeTags_UnknownPlaceholder_SkipsEveryValueContainingIt`,
`TestDiskSKU_RealPulumiProperties_ResolvesDiskType`,
`TestGetProjectedCost_UnknownAttributes_MatchTagOnlyResults`,
`TestGetProjectedCost_NativeScaleSetUnknownCapacity_FallsBackToTag`

#### Scenario: Unknown location attribute

- **WHEN** attribute `location` is the placeholder and `hardwareProfile.vmSize` is `Standard_B2s`
- **THEN** `GetProjectedCost` returns `InvalidArgument` naming `region`
- **AND** no price request is sent

#### Scenario: Unknown disk property

- **WHEN** tag `storageAccountType` is the placeholder and `disk_type` is `Standard_LRS`
- **THEN** the disk SKU is `Standard_LRS`

### Requirement: Attributes merge into tags

Descriptor `attributes` SHALL be flattened into tags the way FinFocus core formats them: nested
keys joined with `.` (`sku.name`, `sku.capacity`), the parent key of an object also set to a
summary value (`sku` is `GP_Gen5` for `{name: GP_Gen5, capacity: 4}`, `hardwareProfile` is the
`vmSize`), lists joined with `,` and indexed (`zones.0`), and keys starting with `__` skipped.
On a key conflict the attribute value SHALL win; tag-only keys SHALL be kept and the request
descriptor SHALL NOT be modified. A descriptor without attributes SHALL be returned unchanged.

Tests: `TestAttributeTags_Flatten_MatchesCoreTagFormat`,
`TestAttributeTags_ValueShapes_MatchCoreConvertValueToString`,
`TestWithAttributeTags_NoAttributes_ReturnsSameDescriptor`,
`TestWithAttributeTags_Conflict_AttributesWin`,
`TestGetProjectedCost_NativeScaleSetCapacityInAttributes_PricesInstances`

#### Scenario: Attribute overrides a stale tag

- **WHEN** tag `sku.capacity` is `2` and attribute `sku.capacity` is `5`
- **THEN** the merged tag `sku.capacity` is `5`
- **AND** the original descriptor tag is still `2`

### Requirement: Attribute and tag size limits

Attributes larger than `pluginsdk.MaxAttributesBytes` SHALL be rejected with `InvalidArgument`.
Flattening SHALL stay bounded: at most `maxAttributeTags` tags and keys of at most
`maxAttributeDepth` segments, keeping shallow keys. A descriptor with more than 256 tags, or a tag
value longer than 2048 bytes, SHALL fail every cost RPC with `InvalidArgument` naming the limit
and SHALL NOT query Azure.

Tests: `TestWithAttributeTags_OverSizeLimit_ReturnsInvalidArgument`,
`TestAttributeTags_DeepAndWide_StaysBounded`, `TestDescriptorLimits_EachRPC_RejectOverLimitTags`

#### Scenario: Too many tags

- **WHEN** a descriptor carries 300 tags
- **THEN** `GetProjectedCost`, `GetActualCost`, and `GetPricingSpec` return `InvalidArgument`
  containing `tag count 300 exceeds maximum 256`
- **AND** no price request is sent
