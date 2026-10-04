# loadbalancer-pricing Specification

## Purpose

Price an Azure Standard Load Balancer from the Retail Prices API: the included rules meter, rule
overage, and processed data, with a fallback to the `Global` price region.

## Requirements

### Requirement: Included rules meter with Global fallback

For `network/LoadBalancer` (and Pulumi tokens such as `azure:lb/loadBalancer:LoadBalancer`), the
quote SHALL read the resource region first and SHALL fall back to price region `Global` when that
page has no included-rules meter. An empty SKU SHALL mean `Standard`. With `rule_count` omitted,
the monthly cost SHALL be the `Standard Included LB Rules and Outbound Rules` meter (unit
`1 Hour`) times 730, reported as breakdown key `rules`, with no `rule_overage` or
`data_processed` component.

Tests: `TestGetProjectedCostLoadBalancerIncludedRules`,
`TestGetProjectedCostLoadBalancerUsesRegionalPage`

#### Scenario: Regional page is empty

- **WHEN** a load balancer in `eastus` with no SKU and no tags is priced and the regional page is
  empty
- **THEN** the plugin queries Azure at least twice, reading the `Global` page second
- **AND** `cost_per_month` equals the included rules retail price times 730
- **AND** `billing_detail` mentions `Global` and that `rule_count` was omitted

#### Scenario: Regional page has the meter

- **WHEN** the `eastus` page contains the included rules meter
- **THEN** exactly one price query is made and the cost is not zero

### Requirement: Rule overage and data processed

When `rule_count` exceeds 5, the quote SHALL add `Standard Overage LB Rules and Outbound Rules`
(unit `1/Hour`) times the extra rule count times 730 as `rule_overage`. When `data_processed_gb`
is set, the quote SHALL add `Standard Data Processed` (unit `1 GB`) times that amount as
`data_processed`. The monthly cost SHALL be the sum of the components.

Tests: `TestGetProjectedCostLoadBalancerOverageAndData`

#### Scenario: Seven rules and 100 GB

- **WHEN** `rule_count=7` and `data_processed_gb=100` on SKU `Standard`
- **THEN** `rules` is the included price times 730, `rule_overage` is 2 times the overage price
  times 730, and `data_processed` is 100 times the data price
- **AND** `billing_detail` contains `above the included 5`

### Requirement: Zero rules has no hourly charge

`rule_count=0` SHALL produce no hourly rules charge.

Tests: `TestGetProjectedCostLoadBalancerNoRules`

#### Scenario: No rules and no data

- **WHEN** `rule_count=0` and no `data_processed_gb` is set
- **THEN** `cost_per_month` is 0
- **AND** `billing_detail` contains `no hourly charge`

### Requirement: Non-Standard SKUs are refused

A load balancer SKU other than `Standard`, such as `Gateway`, SHALL return `InvalidArgument`
naming the SKU.

Tests: `TestGetProjectedCostLoadBalancerRejectsOtherSKU`

#### Scenario: Gateway SKU

- **WHEN** a load balancer with SKU `Gateway` is priced
- **THEN** the call fails with `InvalidArgument` and the message contains `Gateway`
