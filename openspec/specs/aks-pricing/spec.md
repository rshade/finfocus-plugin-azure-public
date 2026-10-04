# aks-pricing Specification

## Purpose

Price an Azure Kubernetes Service cluster from the Retail Prices API: the control plane by tier
and, when tagged, its node pools as on-demand virtual machines.

## Requirements

### Requirement: AKS mapping and query

`containerservice/KubernetesCluster` (case-insensitive) and
`azure:containerservice/kubernetesCluster:KubernetesCluster` SHALL map to service
`Azure Kubernetes Service` with empty `ArmSkuName` and `ProductName`, currency `USD` by default.
A missing tier SHALL be `ErrMissingRequiredFields` naming `tier`, and a type that only starts
with the AKS type SHALL be `ErrUnsupportedResourceType` (`Unimplemented` from
`GetProjectedCost`, unsupported from `Supports`).

Tests: `TestMapDescriptorToQueryAKS`, `TestSupportsAKS`,
`TestGetProjectedCostAKSPrefixIsUnsupported`

#### Scenario: Tier from tag

- **WHEN** a descriptor has type `ContainerService/KubernetesCluster` and tag `tier=Standard`
- **THEN** the query has service `Azure Kubernetes Service`, region `eastus`, and no SKU or
  product

#### Scenario: Prefix type

- **WHEN** the type is `containerservice/kubernetesclusterextra`
- **THEN** `GetProjectedCost` returns `Unimplemented` and `Supports` reports unsupported

### Requirement: Standard control plane and node pools

The Standard tier SHALL bill meter `Standard Uptime SLA` (unit `1 Hour`) times 730 as component
`control_plane`. Node pools SHALL come from tags `node_pool_N_sku` and `node_pool_N_count`, with
optional `node_pool_N_name` (default `pool_N`), each priced as one on-demand VM times 730
times the count under key `node_pool_<name>` (name lower-cased). A cluster `priority=Spot` tag
SHALL NOT price nodes at the Spot rate. `cost_per_month` SHALL equal the component sum, and the
AKS price filter SHALL carry region, service, `priceType eq 'Consumption'`, and currency but no
`armSkuName` or `productName`.

Tests: `TestGetProjectedCostAKSStandardTwoPools`, `TestGetProjectedCostAKSOverGRPC`

#### Scenario: Two pools

- **WHEN** SKU `Standard` has pool 1 with count 2 and pool 2 with count 1
- **THEN** the breakdown is `control_plane`, `node_pool_pool_1` (2 nodes), and
  `node_pool_pool_2` (1 node)
- **AND** `pricing_category` is Standard and `billing_detail` mentions `Standard` and `730`

#### Scenario: Named pool

- **WHEN** tag `node_pool_1_name=System` is set
- **THEN** the first pool's key is `node_pool_system`

### Requirement: Long term support and Premium

Tag `support=lts` (any case) on the Standard tier, and tier `Premium` with or without
`supportPlan`, SHALL bill meter `Standard Long Term Support` instead of `Standard Uptime SLA`.
`supportPlan=AKSLongTermSupport` on the Standard or Free tier, and `support=lts` on the Free
tier, SHALL be `InvalidArgument`.

Tests: `TestGetProjectedCostAKSLongTermSupport`,
`TestAKSControlPlane_RealPulumiProperties_ReadsTier`,
`TestGetProjectedCostAKSRejectsTier`

#### Scenario: LTS tag

- **WHEN** SKU `standard` has tag `support=LTS`
- **THEN** `control_plane` is the `Standard Long Term Support` price times 730
- **AND** `billing_detail` mentions `Long Term`

#### Scenario: Premium tier

- **WHEN** classic `skuTier=Premium` is set without `supportPlan`
- **THEN** the control plane meter is `Standard Long Term Support`

### Requirement: Free control plane is zero

The Free tier SHALL price the control plane at 0 without querying the AKS price page, even when
an open `FreeTierInfrastructureCost Uptime SLA` row exists, and `billing_detail` SHALL say that
meter is not billed. Node pools on a Free cluster SHALL still be priced. Currency SHALL be `USD`.

Tests: `TestGetProjectedCost_AKSFreeTier_PricesControlPlaneAtZero`

#### Scenario: Open free meter in the fixture

- **WHEN** SKU `Free` is priced and the AKS page has a non-zero free-tier meter
- **THEN** `control_plane` is 0, no AKS price query is made
- **AND** `billing_detail` contains `Free`, `FreeTierInfrastructureCost Uptime SLA`, and
  `not billed`

#### Scenario: Free cluster with pools

- **WHEN** SKU `Free` has two node pool tag sets
- **THEN** the pools are priced and `control_plane` is 0

### Requirement: Tier sources and precedence

The descriptor `Sku` (or tag `sku`) SHALL win over the generic tag `tier`, unless it is native
`Base` or the Pulumi unknown placeholder; then native `sku.tier`, classic `skuTier`, or tag
`tier` SHALL be read, with the per-type properties winning over `tier`. Native `Base` with no
tier SHALL be `InvalidArgument`. Tier `Automatic`, in `Sku` or a tag, SHALL be `InvalidArgument`
even when `sku.tier` is set.

Tests: `TestGetProjectedCostAKSTierSources`, `TestAKSControlPlane_RealPulumiProperties_ReadsTier`,
`TestGetProjectedCostAKSRejectsTier`

#### Scenario: Sku beats tier tag

- **WHEN** `Sku` is `Standard` and tag `tier=Free`
- **THEN** the Standard control plane is billed

#### Scenario: Native Base with sku.tier

- **WHEN** `Sku` is `Base` and tag `sku.tier=Standard`
- **THEN** the meter is `Standard Uptime SLA`

#### Scenario: Automatic

- **WHEN** `Sku` is `Automatic`, with or without `sku.tier=Standard`
- **THEN** the call fails with `InvalidArgument`

### Requirement: Invalid node pools and missing meters

A node pool tag set with a missing, zero, or non-integer `node_pool_N_count`, a missing
`node_pool_N_sku`, or a gap (pool 2 without pool 1) SHALL be `InvalidArgument` naming the tag. A
missing control plane meter SHALL be `NotFound` naming the meter, and a node SKU with no VM price
SHALL be `NotFound`. A cluster with no node pool tags SHALL say `node pools not included` in
`billing_detail`.

Tests: `TestGetProjectedCostAKSRejectsNodePools`, `TestGetProjectedCostAKSMissingMeterIsNotFound`,
`TestGetProjectedCostAKSNodeNotFound`,
`TestGetProjectedCost_AKSWithoutNodePools_SaysPoolsNotIncluded`

#### Scenario: Fractional count

- **WHEN** `node_pool_1_count=1.5`
- **THEN** the call fails with `InvalidArgument` naming `node_pool_1_count`

#### Scenario: Missing Standard meter

- **WHEN** the AKS page has no `Standard Uptime SLA` row
- **THEN** the call fails with `NotFound` naming `Standard Uptime SLA`

#### Scenario: No pools

- **WHEN** a Standard cluster has no `node_pool_N_*` tags
- **THEN** `billing_detail` contains `node pools not included`
