# Quickstart: VM Cost Estimation

**Feature**: 020-vm-cost-estimation

## Overview

The EstimateCost RPC returns the estimated monthly cost for an Azure VM based on
its region and SKU. Pricing data is fetched from the Azure Retail Prices API
(unauthenticated) and cached in-memory for 24 hours.

## Prerequisites

- Go 1.25.7+
- Network access to `https://prices.azure.com`

## Build and Run

```bash
make build
./bin/finfocus-plugin-azure-public
# Output: PORT=XXXXX
```

## gRPC Request Example

Using `grpcurl`:

```bash
# Estimate cost for a Standard_B1s VM in East US
grpcurl -plaintext -d '{
  "resource_type": "azure:compute/virtualMachine:VirtualMachine",
  "attributes": {
    "location": "eastus",
    "vmSize": "Standard_B1s"
  }
}' localhost:PORT finfocus.v1.CostSourceService/EstimateCost
```

Example response (actual `costMonthly` varies with current Azure pricing):

```json
{
  "currency": "USD",
  "costMonthly": 14.60,
  "pricingCategory": "FOCUS_PRICING_CATEGORY_STANDARD"
}
```

## Attribute Reference

| Attribute | Required | Description | Example |
| --- | --- | --- | --- |
| `location` | Yes | Azure region | `eastus`, `westus2` |
| `vmSize` | Yes | Azure VM SKU name | `Standard_B1s`, `Standard_D2s_v3` |
| `currencyCode` | No | ISO 4217 currency (default: USD) | `EUR`, `GBP` |
| `serviceName` | No | Azure service (default: VMs) | `Virtual Machines` |

Aliases: `region` for `location`, `sku`/`armSkuName` for `vmSize`, `currency`
for `currencyCode`.

## Error Responses

<!-- markdownlint-disable MD013 -->

| Scenario | gRPC Code | Example Message |
| --- | --- | --- |
| Missing region/SKU | `INVALID_ARGUMENT` | `missing required field(s): region, sku` |
| Unsupported type | `UNIMPLEMENTED` | `unsupported resource type: network/LoadBalancer` |
| VM SKU not found | `NOT_FOUND` | `query [region=eastus sku=X]: not found` |
| Rate limited | `RESOURCE_EXHAUSTED` | Rate limit error with retry hint |
| API unavailable | `UNAVAILABLE` | Service unavailable error |

<!-- markdownlint-enable MD013 -->

## Integration Test

```bash
# Run integration tests against live Azure API
go test -tags=integration ./examples/...
```

## Configuration

| Environment Variable | Default | Description |
| --- | --- | --- |
| `FINFOCUS_PLUGIN_PORT` | 0 (ephemeral) | gRPC listen port |
| `FINFOCUS_LOG_LEVEL` | info | Log level |
| `FINFOCUS_CACHE_TTL` | 24h | Cache TTL duration |
