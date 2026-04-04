# Quickstart: Integration Tests

**Branch**: `022-integration-tests` | **Date**: 2026-03-12

## Running Integration Tests

```bash
# Run all integration tests (includes rate-limiting delays ~1-2 min)
go test -v -tags=integration -timeout=5m ./examples/...

# Skip integration tests via environment variable
SKIP_INTEGRATION=true go test -tags=integration ./examples/...
```

## What Gets Tested

1. **VM Estimation** — Standard_B1s and Standard_D2s_v3 in eastus, verifying
   monthly cost within ±25% of reference prices
2. **Disk Estimation** — 128GB Standard_LRS and 256GB Premium_SSD_LRS in eastus,
   verifying positive monthly costs
3. **Cache Behavior** — Duplicate calls use cache (verified via hit counters)
4. **Error Handling** — Invalid SKU returns NotFound, missing attributes returns
   InvalidArgument

## Updating Reference Prices

When Azure adjusts pricing and tests start failing:

1. Run the failing test with `-v` to see actual prices returned
2. Update the reference price constants in the test file
3. Verify the new values pass with ±25% tolerance

## Test Architecture

```text
Test → Calculator.EstimateCost()
         → CachedClient.GetPrices()
            → Client.GetPrices() (live Azure API)
               → https://prices.azure.com/api/retail/prices
```

Each test constructs the full Client → CachedClient → Calculator chain.
No mocks, no gRPC server — direct method calls against live API.
