# Gap table: real Pulumi properties vs CORE vs PLUGIN (azure-public)

Basis:

- REAL = genuine `pulumi preview --json` output (preview-classic.json, preview-native.json) and schema paths (azure 6.40.0, azure-native 3.28.0).
- CORE = what finfocus core hands the plugin. Derived by running a verbatim copy of `engine.ConvertValueToString` plus the real
  `finfocus-spec@v0.7.0 mapping.ExtractAzureSKU/ExtractAzureRegion` over the genuine preview inputs
  (coresim/main.go, result core-view.json). This is a faithful SIMULATION of core logic, not the core binary. Core path:
  `ingest.extractForwardResource` (newState.inputs) -> `MapResource` (type token, inputs) -> `engine.ConvertToProto` (flatten to strings)
  -> `proto.resolveSKUAndRegion` -> `Sku` / `Region`, and `Tags` = the whole flattened map.
- PLUGIN = code read in finfocus-plugin-azure-public `internal/pricing/*.go` (read-only; NOT executed, so runtime consequences are code-reading conclusions).

Core flattening rules that drive most gaps (engine.go ConvertValueToString):

1. SKU keys are top-level only and exactly `vmSize`, then `sku`, then `tier`. Region keys: `location`, then `region`.
2. A map value is flattened to its `value`, else `id`, else `name`, else the single inner value, else Go `map[...]` text.
   So `sku: {name: P1v3, tier: Premium, capacity: 2}` becomes `P1v3` (capacity and tier are lost);
   `hardwareProfile: {vmSize: X}` becomes Tags["hardwareProfile"]="X", which nothing reads;
   `defaultNodePool: {name: system, vmSize: ...}` becomes Tags["defaultNodePool"]="system".
3. A one-element array is flattened to that element; multi-element arrays are comma-joined. `__defaults` markers (classic) make nested maps multi-key, so they print as `map[...]`.
4. Values that depend on another resource are unknown at preview and appear as the sentinel string `04da6b54-80e4-46f7-96ec-b56ff0331ba9` (e.g. `servicePlanId`, `serverId`, `resourceGroupName` when taken from `${rg.name}` in azure-native).
5. Region: only the type's own `location` is read. Child types without `location` have no region.

Fix owner key: **P** plugin, **C** core adapter (`internal/proto/adapter.go` / `internal/engine`), **S** finfocus-spec `pluginsdk/mapping` (shared key lists; core calls it), **J** join across resources (core, needs plan-wide resolution; not possible inside the plugin).

Consequence key: OK works; ERR clear error / unsupported (no wrong price); WRONG silently wrong price.

## Cross-cutting gaps (biggest first)

| # | Gap | Evidence | Consequence | Owner |
|---|---|---|---|---|
| 1 | Classic provider fills `priority: "Regular"` in inputs when omitted (`__defaults` lists `priority`), including for `LinuxVirtualMachine`. Plugin `prioritySpot` accepts only empty or `Spot`; `Regular` is `InvalidArgument "unsupported priority"`. | preview-classic.json `linuxVmRegular` inputs.priority=Regular; spot.go prioritySpot | ERR for every on-demand classic Linux VM (once SKU is fixed) | P |
| 2 | VM size property is `size` (classic Linux/Windows VM), `sku` (classic VMSS), `hardwareProfile.vmSize` (native, nested). Core only reads top-level `vmSize` (legacy `azure:compute/virtualMachine` only). | core-view.json: azure/linuxVm sku="", azure-native/linuxVm sku="" | ERR missing sku for the two main VM types | C or S (add `size`; flatten `hardwareProfile.vmSize`); P (read `size` tag as fallback) |
| 3 | Windows/Linux is not a property on native VMs (presence of `osProfile.windowsConfiguration`), and a type token on classic VMs. Plugin has no Windows check except the classic WindowsVirtualMachine token (Unimplemented). Native Windows VM, legacy `azure:compute/virtualMachine` with `osProfileWindowsConfig`, and Windows App Service plans all price as Linux. | preview-native.json windowsVm; app-service `os` tag read only as `Windows` | WRONG (silent, undercount) | P (detect `osProfile.windowsConfiguration`, `storageOsDisk.osType`, plan `osType`/`reserved`/`kind`); C (flatten these to a flat `os` key) |
| 4 | Object-valued `sku` collapses to `.name`: AKS `sku.name` is `Base|Automatic`, not the tier (`sku.tier`), so native AKS reaches the plugin as tier `Base` -> `InvalidArgument unsupported tier`; native SQL `sku{name: GP_Gen5, capacity: 4}` becomes `GP_Gen5` (vCores lost) -> refused as DTU; App Service/VMSS `sku.capacity` lost. | core-view.json azure-native/aks sku=Base, azure-native/sqlDb sku=GP_Gen5 | ERR, or lost quantity | C (typed per-resource flattening, not a generic name-pick) |
| 5 | Classic SKU lives in differently named top-level keys the core ignores: `skuName` (servicePlan, mssql database, natGateway, orchestrated VMSS), `skuTier` (AKS), `storageAccountType` (managedDisk), `accountTier`+`accountReplicationType` (storage). | core-view.json: sku="" for azure/plan, azure/sqlDb, azure/disk, azure/storage, azure/aks, azure/nat | ERR missing sku across almost all classic types | C/S (per-type key table), P (accept these tags) |
| 6 | Type tokens: native Cosmos module is `cosmosdb` in 3.28.0 (`azure-native:cosmosdb:DatabaseAccount`; no `documentdb` resources in the schema), plugin matches only `documentdb:databaseaccount`. Classic `Linux/WindowsFunctionApp` (`appservice/linuxFunctionApp`) do not match segment `appservice/functionapp`. Native function app is `web:WebApp` with `kind: "functionapp,linux"`; plugin compares `kind` to the exact string `FunctionApp`. | schema key listing; appservice.go isNativeFunctionWebApp | ERR unsupported (no price) | P |
| 7 | Cost-bearing child resources have no region and no SKU of their own: `azure:mssql/database` (no `location`, only `serverId`), classic/native Cosmos database/container (`accountName`), AKS node pools (`kubernetesClusterId` / `resourceName`), Web/Function apps (`servicePlanId` / `serverFarmId`). | preview inputs show only sentinel UUIDs for the reference | ERR missing region/sku | J (core resolves references from plan/state), plugin cannot |
| 8 | Quantities are never multiplied: App Service `workerCount` / native `sku.capacity`, VMSS `instances` / `sku.capacity`, Cosmos `locations[]`/`geoLocations[]` replicas, AKS `defaultNodePool.nodeCount`. | plugin has no read of these keys | WRONG (undercount) where a type is otherwise priced | P+C |
| 9 | Disk SKU names: real `Premium_LRS`, `StandardSSD_LRS`, `UltraSSD_LRS`, `PremiumV2_LRS`; plugin disk table keys are `premium_ssd_lrs`, `standard_lrs`, `standardssd_lrs`, ... (no `premium_lrs`, no Ultra/PremiumV2). Size key `diskSizeGB` (native, capital GB) vs plugin keys `size_gb|sizeGb|diskSizeGb|capacity_gb`. The plugin's own fixture (`testdata/pulumi/azure-plan.json`) uses synthetic names (`vmSize`, `size_gb`, `Premium_SSD_LRS`) that real Pulumi never emits. | disk.go supportedDiskTypes; fixture head | ERR unsupported disk type / missing size | P (accept ARM names, `diskSizeGB`); fixtures should be replaced with genuine preview output |
| 10 | Spot is expressed correctly for VMs (`priority: Spot` top-level on classic VM and native VM), so the plugin's `priority` tag works; but nested forms are missed: native VMSS `virtualMachineProfile.priority`, native AKS `agentPoolProfiles[].scaleSetPriority`, native AgentPool `scaleSetPriority`, classic node pool `priority`. | map JSON `rules.spot` | OK for VM; unsupported types otherwise | P |

## Per-type table

Region column: property read by core. All "OK" regions assume the user wrote the ARM name (`westeurope`); the plugin does no display-name normalization (`West Europe`): UNVERIFIED how Azure Retail API filters treat it (the plugin passes the value as `armRegionName`).

### azure (classic) 6.40.0

| Type | Real price properties | CORE passes (Sku / Region / notable Tags) | PLUGIN reads | Consequence | Fix |
|---|---|---|---|---|---|
| `compute/linuxVirtualMachine` | `size`, `location`, `priority` (default Regular, filled in preview), `evictionPolicy`, `maxBidPrice`, `osDisk.storageAccountType`/`diskSizeGb`, `licenseType` | Sku="" / westeurope / Tags size, priority=Regular, osDisk=map[...] | Sku or Tags sku (Supports), descriptorSKU also `vmSize`,`armSkuName`; `priority` tag | ERR missing sku; then ERR `unsupported priority Regular` | C/S add `size`; P read `size`, accept Regular |
| `compute/windowsVirtualMachine` | same + `licenseType` (Hybrid Benefit) | Sku="" / westeurope | token rejected (`Unimplemented` in classifyResource; not matched by VM predicate) | ERR (honest) | P if Windows meters wanted: Windows meter + licenseType |
| `compute/virtualMachine` (legacy) | `vmSize`, `location`, `storageOsDisk.osType`, `osProfileWindowsConfig`, `storageOsDisk.managedDiskType` | Sku=vmSize OK / OK | Linux meter only | OK for Linux; WRONG for Windows (priced Linux) | P |
| `compute/linuxVirtualMachineScaleSet`, `windowsVirtualMachineScaleSet`, `orchestratedVirtualMachineScaleSet` | `sku` (size) or `skuName`, `instances`, `priority`, `maxBidPrice` | Sku=sku (orchestrated: ""), OK | type not matched | ERR unsupported | P (new type + multiply by `instances`) |
| `compute/managedDisk` | `storageAccountType`, `diskSizeGb`, `tier`, `diskIopsReadWrite`, `diskMbpsReadWrite` | Sku="" / OK / Tags diskSizeGb OK, storageAccountType | Sku/Tags sku,`disk_type`,`diskType`; size `diskSizeGb` OK; type table lacks `Premium_LRS` | ERR missing disk_type; then ERR unsupported disk type | P (read `storageAccountType`, accept ARM names), C/S |
| `storage/account` | `accountTier`, `accountReplicationType`, `accountKind`, `accessTier` (Hot|Cool; default Hot) | Sku="" / OK / Tags accessTier, accountReplicationType | Sku `Hot LRS` or Tags `tier`+`redundancy` | ERR missing sku | P (read `accessTier` default Hot + `accountReplicationType`) |
| `appservice/servicePlan` | `skuName`, `osType` (Linux|Windows|WindowsContainer), `workerCount` | Sku="" / OK / Tags skuName, osType, workerCount | Sku/Tags sku; `os` tag (only `Windows`) | ERR missing sku; after fix Windows plans WRONG (priced Linux), workerCount ignored | P (read `skuName`,`osType`,`workerCount`), C/S |
| `appservice/linuxWebApp`, `windowsWebApp` | none (`servicePlanId` reference) | Sku="" / OK | not supported | ERR unsupported (correct: cost is on the plan) | none, or J to roll cost up |
| `appservice/linuxFunctionApp`, `windowsFunctionApp` | `servicePlanId`, `storageAccountName` (plan SKU decides Consumption/Premium/Dedicated) | Sku="" / OK | segment `appservice/functionapp` does not match `appservice/linuxfunctionapp` | ERR unsupported | P (token), J (plan SKU) |
| `containerservice/kubernetesCluster` | `skuTier` (Free|Standard|Premium), `supportPlan`, `defaultNodePool.vmSize`/`nodeCount`/`osDiskType` | Sku="" / OK / Tags skuTier, supportPlan, defaultNodePool="system" | Sku/Tags sku,tier; `support`=lts; `node_pool_N_sku/count` | ERR missing tier; after fix node-pool VMs silently omitted, LTS ignored (`supportPlan` vs `support`) | P (read `skuTier`, `supportPlan`), C (expand `defaultNodePool` to `node_pool_1_*`) |
| `containerservice/kubernetesClusterNodePool` | `vmSize`, `nodeCount`, `priority`, `spotMaxPrice`, `osType` | Sku=vmSize / region="" | not supported | ERR unsupported | P + J (cluster location) |
| `mssql/database` | `skuName` (e.g. `GP_Gen5_4`), `maxSizeGb`, `zoneRedundant`, `licenseType`, `readReplicaCount`; no `location` | Sku="" / region="" / Tags skuName, maxSizeGb, zoneRedundant | Sku `GP_Gen5_4` form OK; size `size_gb`..; `zone_redundant`; region | ERR missing region/size_gb/sku; after fix `zoneRedundant`/`licenseType` ignored (WRONG for zone-redundant) | J (region from `serverId`), P (read `skuName`,`maxSizeGb`,`zoneRedundant`), C/S |
| `cosmosdb/account` | `offerType`, `kind`, `geoLocations[]`, `multipleWriteLocationsEnabled`, `capabilities[].name` (EnableServerless), `capacity.totalThroughputLimit` | Sku="" / OK / Tags multipleWriteLocationsEnabled, geoLocations=map[...] | Tags `ru_per_second`/`rus`, `pricing_model`, `multi_master`, `request_units`, `size_gb` | ERR missing ru_per_second; after fix multi-region and serverless WRONG/ignored | P + J (throughput is on `cosmosdb/sqlDatabase`,`sqlContainer`) |
| `cosmosdb/sqlDatabase`, `sqlContainer` | `throughput` or `autoscaleSettings.maxThroughput` | region="" | not supported | ERR unsupported | J + P |
| `lb/loadBalancer` | `sku` (Basic|Standard|Gateway; docs say default Standard), `skuTier` | Sku=sku / OK | Sku; Basic/Gateway rejected; `rule_count`,`data_processed_gb` | OK for Standard (usage tags absent -> default behaviour) | none |
| `network/natGateway` | `skuName`, `idleTimeoutInMinutes`, `zones` | Sku="" / OK | not supported | ERR unsupported (cost-bearing: hourly + data) | P |

### azure-native 3.28.0

| Type | Real price properties | CORE passes | PLUGIN reads | Consequence | Fix |
|---|---|---|---|---|---|
| `compute:VirtualMachine` | `hardwareProfile.vmSize`, `location`, `priority` (Regular|Low|Spot, no default), `billingProfile.maxPrice`, `osProfile.windowsConfiguration`/`linuxConfiguration`, `storageProfile.osDisk.managedDisk.storageAccountType`, `licenseType` | Sku="" / OK / Tags hardwareProfile="Standard_D4s_v5", osProfile=map[...], priority OK | Sku/Tags sku,vmSize; `priority` | ERR missing sku; after fix Windows priced as Linux (WRONG); Spot OK; priority `Low` = ERR | C (flatten `hardwareProfile.vmSize`->`vmSize`, OS), P |
| `compute:VirtualMachineScaleSet` | `sku.name`, `sku.capacity`, `virtualMachineProfile.priority`, OS profile blocks | Sku=sku.name / OK | type not matched (suffix `:compute:virtualmachine` != `...scaleset`) | ERR unsupported | P (+ capacity) |
| `compute:Disk` | `sku.name` (Premium_LRS...), `diskSizeGB`, `tier`, `osType` | Sku=Premium_LRS OK / OK / Tags diskSizeGB | size keys miss `diskSizeGB`; disk table lacks `Premium_LRS` | ERR missing size_gb; then ERR unsupported disk type | P |
| `storage:StorageAccount` | `sku.name` (`Standard_GRS`...), `kind`, `accessTier` | Sku=Standard_GRS / OK / Tags accessTier | Sku in form `Hot LRS` | ERR `unsupported storage sku "Standard_GRS"` | P (parse `<tier>_<redundancy>` + `accessTier` default Hot) |
| `web:AppServicePlan` | `sku.name`, `sku.tier`, `sku.capacity`, `kind`, `reserved` (true=Linux), `isXenon`, `zoneRedundant` | Sku=P1v3 OK / OK / Tags kind, reserved | Sku OK; `os` tag only | OK for Linux P1v3 (1 worker); WRONG for Windows plans (priced Linux) and capacity>1 (undercount) | C (emit `os` from `reserved`/`kind`, `capacity`), P |
| `web:WebApp` | `kind` ("functionapp,linux", "app,linux"), `reserved`, `serverFarmId` | Sku="" / OK / Tags kind="functionapp,linux" | `kind` EqualFold `FunctionApp` | ERR unsupported (not detected as function app) | P (substring match on `functionapp`), J (plan SKU) |
| `containerservice:ManagedCluster` | `sku.tier` (Free|Standard|Premium), `sku.name` (Base|Automatic), `supportPlan`, `agentPoolProfiles[].vmSize`/`count`/`scaleSetPriority` | Sku="Base" (name wins) / OK / Tags agentPoolProfiles="system" | Sku -> tier | ERR `unsupported tier "Base"`; node pools omitted; LTS ignored | C (use `sku.tier`; expand pools to `node_pool_N_*`), P |
| `containerservice:AgentPool` | `vmSize`, `count`, `scaleSetPriority`, `spotMaxPrice`, `osType` | Sku=vmSize / region="" | not supported | ERR unsupported | P + J |
| `sql:Database` | `sku.name`+`sku.capacity`+`sku.family`+`sku.tier`, `maxSizeBytes` (bytes), `zoneRedundant`, `licenseType`, `location` | Sku="GP_Gen5" (capacity lost) / OK / Tags maxSizeBytes, zoneRedundant | Sku `GP_Gen5_4`; `size_gb`; `zone_redundant` | ERR (2-part sku refused as DTU; no size_gb) | C (compose `<name>_<capacity>`; bytes->GB), P (`maxSizeBytes`, `zoneRedundant`) |
| `cosmosdb:DatabaseAccount` | `databaseAccountOfferType`, `kind`, `locations[]`, `enableMultipleWriteLocations`, `capabilities[]` | Sku="" / OK | token suffix `documentdb:databaseaccount` only | ERR unsupported | P (add `cosmosdb:databaseaccount`) |
| `cosmosdb:SqlResourceSqlDatabase`, `SqlResourceSqlContainer` | `options.throughput`, `options.autoscaleSettings.maxThroughput` | region="" | not supported | ERR unsupported | J + P |
| `network:LoadBalancer` | `sku.name`, `sku.tier` | Sku=Standard / OK | Sku | OK for Standard | none |
| `network:NatGateway` | `sku.name`, `zones` | Sku=Standard / OK | not supported | ERR unsupported | P |

## What is UNVERIFIED

- Plugin runtime behaviour: conclusions come from reading code (and the CORE view from a copy of core logic), not from running the plugin or the real `finfocus` binary against these plans.
- Classic `location` omitted (falls back to resource group in the provider?) not tested for classic; for azure-native an omitted `location` was filled from `azure-native:location` stack config (`vmNoLocation` -> westeurope), not from the resource group location.
- Whether the Azure Retail Prices API accepts display-style regions (`West Europe`) as `armRegionName`.
- Default `sku` of classic `lb/loadBalancer` when omitted (docs say Standard; the schema has no `default`; I always set it).
- `pulumi up`/state shape (outputs populated, real IDs) was not exercised; preview `outputs` are null for creates.
