## ADDED Requirements

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
