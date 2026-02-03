# Quickstart: CostSourceService Method Stubs

**Feature**: 004-costsource-stubs
**Date**: 2026-02-02

## Overview

This feature implements stub methods for the CostSourceService interface.
After implementation, the plugin will respond to all 11 RPC methods without crashing.

## Testing the Stubs

### 1. Run Unit Tests

```bash
make test
```

Expected output: All tests pass, including new stub method tests.

### 2. Start the Plugin

```bash
make build
./bin/finfocus-plugin-azure-public
```

Expected output:

```text
PORT=XXXXX
```

### 3. Test with grpcurl

Install grpcurl if needed:

```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

Test Name RPC:

```bash
grpcurl -plaintext localhost:XXXXX finfocus.v1.CostSourceService/Name
```

Expected response:

```json
{
  "name": "finfocus-plugin-azure-public"
}
```

Test Supports RPC:

```bash
grpcurl -plaintext -d '{"resource_type": "azure:compute:VirtualMachine"}' \
  localhost:XXXXX finfocus.v1.CostSourceService/Supports
```

Expected response:

```json
{
  "supported": false,
  "reason": "not yet implemented"
}
```

Test Unimplemented RPC:

```bash
grpcurl -plaintext localhost:XXXXX finfocus.v1.CostSourceService/GetBudgets
```

Expected response:

```text
ERROR:
  Code: Unimplemented
  Message: not yet implemented
```

## Method Response Summary

| Method | Response | Status |
| ------ | -------- | ------ |
| Name() | `{"name": "finfocus-plugin-azure-public"}` | OK |
| GetPluginInfo() | `{"name": "azure-public", "version": "0.1.0", ...}` | OK |
| Supports() | `{"supported": false, "reason": "not yet implemented"}` | OK |
| EstimateCost() | Unimplemented | Stub |
| GetActualCost() | Unimplemented | Stub |
| GetProjectedCost() | Unimplemented | Stub |
| GetPricingSpec() | Unimplemented | Stub |
| GetRecommendations() | Unimplemented | Stub |
| DismissRecommendation() | Unimplemented | Stub |
| GetBudgets() | Unimplemented | Stub |
| DryRun() | Unimplemented | Stub |

## Integration with FinFocus Core

Once stubs are implemented, the plugin can be registered with FinFocus Core
for integration testing. Core will:

1. Call GetPluginInfo() to verify compatibility
2. Call Supports() to discover supported resources (none yet)
3. Handle Unimplemented errors gracefully for other methods

No Core configuration changes needed - it handles Unimplemented status automatically.
