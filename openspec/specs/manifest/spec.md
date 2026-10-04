# manifest Specification

## Purpose

Keep the committed `manifest.json` and `manifest.yaml` generated from the plugin's own catalog,
so the resource types, billing modes, capabilities, and methods they list match what the plugin
serves.

## Requirements

### Requirement: Committed files match the generated manifest

`manifest.json` and `manifest.yaml` SHALL equal, byte for byte, `pluginsdk.MarshalManifestJSON`
and `MarshalManifestYAML` of the expected manifest: name `azure-public`, version `pluginVersion`,
spec version `pluginsdk.SpecVersion` without the `v` prefix, provider `azure`, resource types
`SupportedResourceTypes()`, billing modes `specBillingModes()`, and installation method binary.

Tests: `TestExpectedManifest_CommittedFiles_MatchExpected`

#### Scenario: A type is added without regenerating

- **WHEN** `SupportedResourceTypes()` gains a type and the files are not regenerated
- **THEN** the test fails and prints the regenerate command

### Requirement: Manifest JSON passes registry validation

The committed `manifest.json` SHALL pass `registry.ValidatePluginManifest` as written.

Tests: `TestExpectedManifest_CommittedJSON_PassesRegistryValidation`

#### Scenario: Validate the committed file

- **WHEN** `registry.ValidatePluginManifest` reads `manifest.json`
- **THEN** it returns no error

### Requirement: Capabilities and methods match what is served

The manifest capabilities SHALL be `PluginInfo().Capabilities` under their manifest names,
followed by `caching`. The service definition SHALL be `CostSourceService` in package
`finfocus.v1` and SHALL list exactly the methods `Calculator` serves: `Name`, `Supports`,
`GetProjectedCost`, `GetActualCost`, `GetPricingSpec`, `EstimateCost`, `GetPluginInfo`, and
`DryRun`. A method from `registry.AllServiceMethods()` SHALL be listed if and only if it does not
return the generated stub error `method <name> not implemented`.

Tests: `TestExpectedManifest_CommittedFiles_MatchExpected`,
`TestExpectedManifest_Methods_MatchServedRPCs`

#### Scenario: Stubbed RPC is not listed

- **WHEN** an RPC in `registry.AllServiceMethods()` returns the generated stub error
- **THEN** it is absent from the manifest methods

### Requirement: Billing modes are exactly the modes specRate returns

`specBillingModes()` SHALL be sorted and SHALL list every mode a `specRate` branch returns
(`per_data_transfer_gb`, `per_gb_month`, `per_hour`, `per_month`, `per_ru`, `per_second`,
`per_vcpu_hour`), and SHALL NOT list the `not_implemented` fallback.

Tests: `TestSpecRate_EveryBranch_ReturnsListedBillingMode`

#### Scenario: Unrecognised unit

- **WHEN** `specRate` is given a meter with unit `1M` and no spec unit
- **THEN** it returns `not_implemented`, which `specBillingModes()` does not list
