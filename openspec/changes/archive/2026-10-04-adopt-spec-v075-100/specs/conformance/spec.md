## ADDED Requirements

### Requirement: SDK conformance passes with an Azure sample resource

Served through the finfocus-spec SDK server, the plugin SHALL pass every check
of the SDK conformance suite at Basic, Standard, and Advanced level when the
suite sends the Azure sample resource: provider `azure`, resource type
`azure:compute/virtualMachine:VirtualMachine`, region `eastus`, SKU
`Standard_B1s`. The check runs against a fixture price server, with no
network.

Tests: `TestSDKConformance_AzureSample_PassesEveryLevel`

#### Scenario: Every level passes

- **WHEN** the conformance suite runs at Basic, Standard, and Advanced with the Azure sample resource
- **THEN** each level reports zero failed checks
- **AND** each level reports at least one passed check
