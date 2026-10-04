# plugin-info Specification

## Purpose

Report the plugin identity, the SDK spec version, the providers, and the RPCs
the plugin serves, so FinFocus core routes only to implemented methods.

## Requirements

### Requirement: Plugin identity and spec version

`Name` SHALL return `azure-public`. `GetPluginInfo` SHALL report the SDK spec
version (`pluginsdk.SpecVersion`), both when called directly and when served
through the SDK server.

Tests: `TestCalculatorName`, `TestGetPluginInfoReturnsSpecVersion`,
`TestGetPluginInfoServedSpecVersion`

#### Scenario: Served info carries the SDK spec version

- **WHEN** `GetPluginInfo` is called over gRPC through `pluginsdk.NewServer`
- **THEN** `spec_version` equals `pluginsdk.SpecVersion`

### Requirement: Capabilities list only served RPCs

`GetPluginInfo` SHALL list exactly the capabilities `PROJECTED_COSTS`,
`ACTUAL_COSTS`, `PRICING_SPEC`, `ESTIMATE_COST`, and `DRY_RUN`, and SHALL set
metadata `type=public-pricing-fallback`. The legacy SDK metadata SHALL include
`supports_dry_run=true` and SHALL NOT advertise batch cost or recommendations.

Tests: `TestGetPluginInfo_Direct_ListsImplementedCapabilities`,
`TestGetPluginInfo_OverGRPC_SendsExplicitCapabilitiesAndType`

#### Scenario: Unserved RPCs are not advertised

- **WHEN** `GetPluginInfo` is called over gRPC
- **THEN** the capabilities are those five values in that order
- **AND** metadata has no `supports_batch_cost`, `max_batch_size`, or `supports_recommendations` key

### Requirement: Providers advertise azure

`GetPluginInfo` SHALL advertise the single provider `azure`. Descriptors with
provider `azure-native` are still priced; that is a resource-mapping rule, not
an advertised provider.

Tests: `TestGetPluginInfo_OverGRPC_AdvertisesOnlyAzure`,
`TestGetPluginInfoReturnsProviders`

#### Scenario: Provider list

- **WHEN** `GetPluginInfo` is called over gRPC
- **THEN** `providers` is exactly `["azure"]`

### Requirement: Serve configuration matches GetPluginInfo

The process SHALL pass the same plugin info to `pluginsdk.ServeConfig` that
`GetPluginInfo` returns: name, version, capabilities, and providers agree.

Tests: `TestServeConfig_PluginInfo_MatchesGetPluginInfo`

#### Scenario: One source of plugin info

- **WHEN** the serve configuration is built for a calculator
- **THEN** its name, version, capabilities, and providers equal the `GetPluginInfo` response

### Requirement: Unserved RPCs report Unimplemented

Served through the SDK server, `GetBudgets` and `DismissRecommendation` SHALL
return gRPC `Unimplemented`, not `Internal`, because the plugin does not serve
budgets or recommendations.

Tests: `TestUnservedRPCs_OverGRPC_ReturnUnimplemented`

#### Scenario: Budgets are not supported

- **WHEN** `GetBudgets` is called over gRPC
- **THEN** the status code is `Unimplemented`

#### Scenario: Dismiss is not supported

- **WHEN** `DismissRecommendation` is called over gRPC with a recommendation id
- **THEN** the status code is `Unimplemented`
