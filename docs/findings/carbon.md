# Carbon footprint data sources

Research for issue
[#56](https://github.com/rshade/finfocus-plugin-azure-public/issues/56).
This note does not estimate carbon and does not call an authenticated API.
The formula in the issue is a question. It is not a result to hardcode.
`ImpactMetrics` is not filled.

`GetProjectedCostResponse.impact_metrics` already exists. It is
`../finfocus-spec/sdk/go/proto/finfocus/v1/costsource.pb.go` lines
1495-1496. `ImpactMetric` is lines 934-942 (`kind`, `value`, `unit`).
`METRIC_KIND_CARBON_FOOTPRINT` is line 32. A later task has a field.
This spike does not write it.

## 1. Azure Carbon Optimization API

Excluded. It needs authentication.

[Export carbon optimization emissions data](https://learn.microsoft.com/en-us/azure/carbon-optimization/api-export-data)
says export requires a service principal, a tenant ID, a client ID, and a
client secret, then a bearer token from
`https://login.microsoftonline.com/<tenant ID>/oauth2/token` with
`resource=https://management.azure.com`. The report request is
`POST https://management.azure.com/providers/Microsoft.Carbon/carbonEmissionReports?api-version=2025-04-01`
with `Authorization: Bearer`. `subscriptionList` is required. A denied
subscription in the sample response says `Carbon Optimization Reader
permisison required`.

[Assign access to Carbon optimization](https://learn.microsoft.com/en-us/azure/carbon-optimization/permissions)
says emissions data uses Azure RBAC. View Emissions is required at
subscription scope. The roles that can view emissions without an extra
assignment are Subscription Owner, Subscription Contributor, and
Subscription Reader. Other users need the Carbon Optimization Reader role.

That is an Azure subscription, a tenant, and a secret. It violates
`CONTEXT.md` ("No Authenticated Azure APIs"). This plugin does not call it.

The same export page says emissions data for the previous month is
available by day 19 of the current month. Reports are for subscriptions
and resources that already exist. They are not a price-list row for a SKU
that has not been deployed. The sample `MonthlySummaryData` includes
`carbonIntensity` with the example value 22. The page does not define that
field's unit. 22 is sample JSON, not a regional intensity.

The Retail Prices API does not carry a carbon field. The property table on
[Azure Retail Prices](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices)
lists currency, tier, reservation term, retail price, unit price, region,
location, dates, meter, product, SKU, service, unit of measure, type,
primary-meter flag, ARM SKU, and savings-plan term. Carbon is not in that
table.

## 2. Cloud Carbon Footprint

Source:
[Methodology](https://www.cloudcarbonfootprint.org/docs/methodology).

The page says estimates are point estimates without confidence intervals,
are not a replacement for cloud-provider data, and cannot be guaranteed
accurate.

### Does the method work without usage metrics

Not as published. The operational formula on that page is:

```text
(Cloud provider service usage)
  x (Cloud energy conversion factors [kWh])
  x (Cloud provider PUE)
  x (grid emissions factors [metric tons CO2e])
```

Total CO2e on that page also adds embodied emissions for compute.

Azure usage, by default, comes from the Azure Consumption Management API.
That API is authenticated. Compute watt-hours are:

```text
Average Watts = Min Watts + Avg vCPU Utilization * (Max Watts - Min Watts)
Compute Watt-Hours = Average Watts * vCPU Hours
```

vCPU hours are pulled from usage or billing data. For AWS and Azure, the
page says the usage amount is multiplied by the product vCPU count.

When measured CPU utilization is missing, the page falls back to 50%
(hyperscale data centers in 2020, from the 2016 U.S. Data Center Energy
Usage Report). That is an assumption, not a measured watt value. The page
does not publish a path that starts from a VM size and a region with no
hours and no utilization.

A projected always-on estimate could substitute an hours assumption and
the 50% fallback. This spike does not choose those inputs.

### Can a VM size, memory, and region be enough

Only with extra assumptions, and not for every current size.

Region selects a grid factor. vCPU count and processor select min and max
watts. Memory is not part of the compute watt formula. The memory section
says SPECPower min and max watts already include some memory. Extra memory
energy is added only when gigabytes per physical CPU exceed the SPECPower
average for that micro-architecture. The summarized formula is:

```text
Kilowatt hours = Memory (GB) exceeding SPECPower database average
  x Memory coefficient
  x usage amount (Hours)
```

The coefficient on that page is 0.000392 kilowatt hour per gigabyte hour.
Azure compute instances use the same extra-memory approach as AWS. The
page points at `azure-instances.csv` for that list. Direct "Memory
Duration" usage rows, when present, use gigabyte-hours times the same
coefficient. A projected quote has no Memory Duration row.

The fetched
[azure-instances.csv](https://raw.githubusercontent.com/cloud-carbon-footprint/cloud-carbon-coefficients/main/data/azure-instances.csv)
has 595 data rows and 393 distinct `Virtual Machine` names. Columns are
series, virtual machine, microarchitecture, instance vCPUs, instance
memory, platform vCPUs, platform memory, storage fields, and GPU fields.
There is no watt column. No name starts with `Standard_`. `B1S` is
present and its microarchitecture is `Unknown`. There is no `Bsv2` row,
and no row whose name or microarchitecture contains Ice Lake, Sapphire,
Emerald, Cobalt, or Altra. The `v5` rows found are `E* v5` with
microarchitecture `Unknown`. `v7` has no rows. 129 rows are `Unknown`.

Unknown micro-architecture uses the provider average. On the methodology
page that average for Azure is min watts 0.78 and max watts 3.76. The
trunk file below publishes different averages. A later task still has to
map `Standard_*` ARM SKU names onto this list, and current series are
missing from the file.

### Coefficients that are public

On the methodology page, Appendix I for Azure:

- Average minimum watts at 0% CPU: 0.78
- Average maximum watts at 100% CPU: 3.76
- Average CPU utilization: 50%
- HDD: 0.65 watt-hours per terabyte-hour
- SSD: 1.2 watt-hours per terabyte-hour
- Networking: 0.001 kilowatt-hours per gigabyte
- Memory: 0.000392 kilowatt-hours per gigabyte
- Average PUE in that appendix: 1.125

The same page's PUE section says the Azure PUE in use is 1.185. Both
numbers are on the page. This spike does not pick one. See question 4.

Appendix III publishes GPU idle and 100% watts. The rows are Tesla M60
35 and 306, T4 8 and 71, Tesla K80 35 and 306, Tesla V100 35 and 306,
Tesla A100 46 and 407, K520 26 and 229, A10G 18 and 153, Tesla P4 9 and
76.5, Tesla P100 36 and 306, Tesla P40 30 and 255, Radeon Pro V520 26
and 229, Alveo U250 27 and 229.5. The page says SPECPower has no GPU
energy data. GPU hours replace vCPU hours because the whole GPU is
assigned. The idle and peak ratio is taken from Teads AWS measurements
and applied to Azure.

Per-processor watts are in
[AzureFootprintEstimationConstants.ts](https://raw.githubusercontent.com/cloud-carbon-footprint/cloud-carbon-footprint/trunk/packages/azure/src/domain/AzureFootprintEstimationConstants.ts)
on trunk. CPU min and max watts in that file:

| Processor | Min watts | Max watts |
| --- | --- | --- |
| Cascade Lake | 0.64 | 3.97 |
| Skylake | 0.65 | 4.26 |
| Broadwell | 0.71 | 3.69 |
| Haswell | 1 | 4.74 |
| Coffee Lake | 1.14 | 5.42 |
| Sandy Bridge | 2.17 | 8.58 |
| Ivy Bridge | 3.04 | 8.25 |
| AMD EPYC 1st gen | 0.82 | 2.55 |
| AMD EPYC 2nd gen | 0.47 | 1.69 |
| AMD EPYC 3rd gen | 0.45 | 2.02 |

That file's fallback when the processor is missing is `MIN_WATTS_AVG`
0.74 and `MAX_WATTS_AVG` 3.54. The methodology page's Azure averages are
0.78 and 3.76. The two fetched sources disagree. This spike does not
average them and does not choose one.

The same file also publishes GPU min and max watts (T4 8 and 71, Tesla
K80 35 and 306, Tesla P100 36 and 306, Tesla V100 35 and 306, Tesla M60
35 and 306, Tesla P40 30 and 255, Tesla A100 46 and 407, Alveo U250 27
and 229.5), `MEMORY_COEFFICIENT` 0.000392 (comment: kWh / Gb),
`NETWORKING_COEFFICIENT` 0.001 (comment: kWh / Gb), `SSDCOEFFICIENT` 1.2
and `HDDCOEFFICIENT` 0.65 (comment: watt hours / terabyte hour),
`PUE_AVG` 1.185, `AVG_CPU_UTILIZATION_2020` 50, and replication factors
`STORAGE_LRS` 3, `STORAGE_ZRS` 3, `STORAGE_GRS` 6, `STORAGE_GZRS` 6,
`STORAGE_DISKS` 3, `DATABASE_MYSQL` 3, `COSMOS_DB` 4, `SQL_DB` 3,
`REDIS_CACHE` 2, `DEFAULT` 1. `getPUE` returns only `PUE_AVG`. There is
no per-region Azure PUE in that file.

[cloud-carbon-coefficients](https://github.com/cloud-carbon-footprint/cloud-carbon-coefficients)
was archived by the owner on May 11, 2023. The README says new SPECpower
data is released every quarter and that the notebook was replaced by
[ccf-coefficients](https://github.com/cloud-carbon-footprint/ccf-coefficients).
This spike did not fetch that CLI's output files. The methodology page
still links the archived `data/` directory for the processor list.

## 3. Static carbon intensity

### What is the kgCO2/kWh for each Azure region

kgCO2/kWh for each Azure region is not published on the sources fetched
for this spike. The sources that do publish a regional factor use metric
tons CO2e per kWh, and they do not cover every Azure region. This spike
does not convert units.

Appendix V of the
[methodology](https://www.cloudcarbonfootprint.org/docs/methodology)
says the application supports a subset of Azure regions, because
Consumption API names do not always match the Azure website. The table
below is that page, copied as published. The unit column is metric tons
CO2e per kWh.

| Region | Location | Metric tons CO2e per kWh | Cited source |
| --- | --- | --- | --- |
| `Central US*` | Iowa | 0.000426254 | EPA |
| `East US*` | Virginia | 0.000379069 | EPA |
| `East US 2*` | Virginia | 0.000379069 | EPA |
| `East US 3` | Georgia | 0.000379069 | EPA |
| `North Central US*` | Illinois | 0.000410608 | EPA |
| `South Central US*` | Texas | 0.000373231 | EPA |
| `West Central US` | Wyoming | 0.000322167 | EPA |
| `West US*` | California | 0.000322167 | EPA |
| `West US 2*` | Washington | 0.000322167 | EPA |
| `West US 3` | Arizona | 0.000322167 | EPA |
| `East Asia*` | Hong Kong | 0.00071 | carbonfootprint.com |
| `Southeast Asia*` | Singapore | 0.000408 | carbonfootprint.com |
| `South Africa North` | Johannesburg | 0.0009006 | carbonfootprint.com |
| `South Africa West` | South Africa | 0.0009006 | carbonfootprint.com |
| `South Africa` | South Africa | 0.0009006 | carbonfootprint.com |
| `Australia` | Australia | 0.00079 | carbonfootprint.com |
| `Australia Central` | Canberra | 0.00079 | carbonfootprint.com |
| `Australia Central 2` | Canberra | 0.00079 | carbonfootprint.com |
| `Australia East` | New South Wales | 0.00079 | carbonfootprint.com |
| `Australia South East` | Victoria | 0.00096 | carbonfootprint.com |
| `Japan` | Japan | 0.0004658 | carbonfootprint.com |
| `Japan West` | Osaka | 0.0004658 | carbonfootprint.com |
| `Japan East` | Tokyo, Saitama | 0.0004658 | carbonfootprint.com |
| `Korea` | Korea | 0.0004156 | carbonfootprint.com |
| `Korea East` | Korea | 0.0004156 | carbonfootprint.com |
| `Korea South` | Korea | 0.0004156 | carbonfootprint.com |
| `India` | India | 0.0007082 | carbonfootprint.com |
| `India West*` | Mumbai | 0.0007082 | carbonfootprint.com |
| `India Central*` | Pune | 0.0007082 | carbonfootprint.com |
| `India South` | Chennai | 0.0007082 | carbonfootprint.com |
| `North Europe` | Ireland | 0.0002786 | EEA |
| `West Europe` | Netherlands | 0.0003284 | EEA |
| `France` | France | 0.00005128 | carbonfootprint.com |
| `France Central` | Paris | 0.00005128 | carbonfootprint.com |
| `France South` | France | 0.00005128 | carbonfootprint.com |
| `Sweden Central` | Gävle and Sandviken | 0.00000567 | carbonfootprint.com |
| `Switzerland` | Switzerland | 0.00000567 | carbonfootprint.com |
| `Switzerland North` | Zürich | 0.00000567 | carbonfootprint.com |
| `Switzerland West` | Switzerland | 0.00000567 | carbonfootprint.com |
| `UK` | United Kingdom | 0.000225 | EEA |
| `UK South` | London | 0.000225 | EEA |
| `UK West` | Cardiff | 0.000228 | EEA |
| `Germany` | Germany | 0.00033866 | carbonfootprint.com |
| `Germany North` | Germany | 0.00033866 | carbonfootprint.com |
| `Germany West Central` | Frankfurt | 0.00033866 | carbonfootprint.com |
| `Norway` | Norway | 0.00000762 | carbonfootprint.com |
| `Norway East` | Oslo | 0.00000762 | carbonfootprint.com |
| `Norway West` | Norway | 0.00000762 | carbonfootprint.com |
| `United Arab Emirates` | United Arab Emirates | 0.0004041 | carbonfootprint.com |
| `United Arab Emirates North` | Dubai | 0.0004041 | carbonfootprint.com |
| `United Arab Emirates Central` | United Arab Emirates | 0.0004041 | carbonfootprint.com |
| `Canada` | Canada | 0.00012 | carbonfootprint.com |
| `Canada Central` | Toronto | 0.00012 | carbonfootprint.com |
| `Canada East` | Quebec City | 0.00012 | carbonfootprint.com |
| `Brazil` | Brazil | 0.0000617 | carbonfootprint.com |
| `Brazil South` | São Paulo State | 0.0000617 | carbonfootprint.com |
| `Brazil South East` | Brazil | 0.0000617 | carbonfootprint.com |

The page marks some of those names with an asterisk and says sub-regions
share the primary region's intensity. Regions absent from this table are
not published on this page.

The archived
[grid-emissions-factors-azure.csv](https://raw.githubusercontent.com/cloud-carbon-footprint/cloud-carbon-coefficients/main/data/grid-emissions-factors-azure.csv)
has 23 data rows. The header unit is `CO2e (metric ton/kWh)`. It is a
smaller set, and several numbers differ from the table above. Examples
from that file: `East US` 0.000415755, `West Europe` 0.00039,
`North Europe` 0.000316, `Sweden Central` 0.000009, `UK West` 0.000228.
The `Finland Central` cell is the text `000077` and the
`Germany West Central` cell is the text `000402`. Those two cells are not
a decimal coefficient on the file fetched. The file does not publish
kgCO2/kWh.

`AZURE_EMISSIONS_FACTORS_METRIC_TON_PER_KWH` in the trunk constants file
publishes metric tons per kWh for named region keys. United States keys
are assigned `US_NERC_REGIONS_EMISSIONS_FACTORS` (`MRO`, `SERC`, `RFC`,
`TRE`, `WECC`). The numeric NERC values are not in the file fetched.
`UNKNOWN` in that file is 0.0003512799615, with the comment "Average of
above regions". Non-US literals in that file also differ from Appendix V.
Examples: `EU_WEST` 0.0003284 (Appendix V `West Europe` is 0.0003284, so
that one matches), `EU_SWITZERLAND` 0.00001152 (Appendix V `Switzerland`
is 0.00000567), `EU_NORWAY` 0.00000762 (matches Appendix V). A later task
cannot treat these three artifacts as one dataset.

### Is Microsoft sustainability data usable

Not as a public per-region kgCO2/kWh table, and not without
authentication for the customer-specific numbers.

[Azure emissions calculation methodology](https://learn.microsoft.com/en-us/power-bi/connect-data/azure-emissions-calculation-methodology)
allocates Azure emissions from relative customer usage in a datacenter
region. It names grid emission factors, renewable purchases, and
infrastructure power as inputs. It does not publish those factors. The
unit it names for the result is metric tons of carbon dioxide equivalent
(MTCO2E), for the customer's allocated emissions, not kgCO2/kWh by
region. It excludes Azure Government, China, and the sovereign Germany
regions listed on that page. Scope 1 and 2 are attributed from usage
time. Equipment lifetime defaults to six years. The page says the
methodology will be revised over time. That is not a downloadable
intensity table.

[Emissions Impact Dashboard for Azure](https://learn.microsoft.com/en-us/power-bi/connect-data/service-connect-to-emissions-impact-dashboard)
is a Power BI app for a billing account. It requires a Billing Account
Administrator for EA Direct, MCA, or MPA. It uses Azure consumed revenue
plus Microsoft energy and carbon data. It does not publish a regional
kgCO2/kWh table. The page says the dashboard hosted in Power BI retires
on March 31, 2027, and points at Azure Carbon Optimization, which is the
authenticated API in question 1. A February 2024 methodology change
recalculated historical customer data. That is a customer report, not a
static file this plugin can vendor.

The original Microsoft sentence for the fleet PUE is quoted on the CCF
methodology page (question 4). This spike did not fetch the Microsoft
page that sentence came from.

### How often carbon intensity changes

No fetched source states one refresh period for "Azure region carbon
intensity."

- CCF's carbon section says US factors are EPA eGRID NERC-region factors,
  annual, and that the data sources are averages over a year. It names
  eGRID2023 and says the averages are pre-2023 and ignore time of day.
  Outside the US it names country factors and, for most of Europe, EEA
  factors. It also says an Electricity Maps API token can replace the
  default. That API was not called. It needs a token.
- The three CCF artifacts above already disagree, so the published figure
  is not stable across the pages fetched in this spike.
- The archived coefficients README says SPECpower results are released
  every quarter. That sentence is about the watt-coefficient notebook,
  not about a regional kgCO2/kWh series. The repository is archived.
- Customer emissions from Carbon Optimization are monthly: previous month
  by day 19. The dashboard page says a month's emissions are available by
  the 15th day after month end. Both require an account. Neither is a
  public intensity series.

A static table would be stale on a schedule this spike cannot name,
because the public copies already differ.

### Can a small static table be embedded

A coefficient table is small enough to embed as data. It is not the Azure
retail catalog. Whether to embed one is a product choice for a later
task, because the copies disagree. See question 8.

## 4. VM power

### Can watts be estimated from vCPU and memory

Not as a measured watt value. CCF estimates average watts from min watts,
max watts, and utilization, then multiplies by vCPU hours. Min and max
watts are per processor micro-architecture, not a function this spike
can fit from vCPU count and memory gigabytes alone. Memory uses the
separate coefficient in question 2, and only the gigabytes above the
SPECPower baseline, times hours.

If the processor is unknown, the methodology page uses Azure average min
0.78 and max 3.76 watts. The trunk constants file uses 0.74 and 3.54.
Neither number is a wattage of a named Azure VM.

### Does Microsoft publish TDP for Azure VM hardware

Not on the size page fetched. The
[B family size series](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/general-purpose/b-family)
page lists vCPU counts, memory, processor names (AMD EPYC 7763v,
Ampere Altra at 3.0 GHz, Intel Xeon Platinum 8573C, 8473C, and 8370C),
disk, and network. TDP and watts are not on that page. Those processor
names are also absent from the CCF Azure instance file in question 2.
Watts for those CPUs are not published on the sources fetched here.

### Does CCF publish PUE

Yes. The methodology page publishes three Azure figures, and they are
not the same number:

- PUE section: Azure 1.185. The footnote quotes a Microsoft sustainability
  statement: weighted owned-and-operated fleet-wide PUE, trailing
  12-month average, is 1.185, and the latest designs achieve an annual
  PUE of 1.125. GCP publishes its own PUE. AWS on that page is a guess
  from public information, 1.135.
- Appendix I, Azure: average PUE 1.125.
- Trunk `PUE_AVG`: 1.185, and `getPUE` ignores region.

This spike does not choose 1.125 or 1.185. A per-region Azure PUE is not
published on those two sources.

## 5. How the AWS plugin calculates carbon

Local checkout
`../finfocus-plugin-aws-public/internal/carbon/estimator.go`.
`CalculateCarbonGrams` (lines 97-115):

```text
avgWatts = minWatts + utilization * (maxWatts - minWatts)
energyKWh = avgWatts * vCPUCount * hours / 1000
energyWithPUE = energyKWh * AWSPUE
carbonGrams = energyWithPUE * gridIntensity * 1000000
```

`AWSPUE` is 1.135 in
`../finfocus-plugin-aws-public/internal/carbon/constants.go`.
The comment says the grid factor is metric tons CO2e per kWh, and the
multiply by 1,000,000 converts to grams. The result unit written by
`../finfocus-plugin-aws-public/internal/plugin/projected.go` lines
399-405 is `gCO2e`, kind `METRIC_KIND_CARBON_FOOTPRINT`.

`GetUtilization` in `utilization.go` uses a per-resource override, then
the request value, then `DefaultUtilization` 0.50. EC2 projected cost
passes `carbon.HoursPerMonth`, which is 730, not a measured runtime.
Unknown instance types return no carbon metric and still return the
price.

Min and max watts come from the embedded AWS CSV. `instance_specs.go`
reads column 14 (`PkgWatt @ Idle`) and column 17 (`PkgWatt @ 100%`).
The Go comment calls those watts per vCPU, and the formula multiplies
by vCPU count. This spike does not decide whether that column is already
a package total. GPU watts are added separately when enabled. The grid
map in `grid_factors.go` is AWS region codes. It has no Azure region.

That is a CCF-shaped compute estimate with AWS constants, an assumed
730 hours, and a 50% utilization default. It is not a reading of Azure
Carbon Optimization.

## 6. Formula in the issue

The issue writes:

```text
carbon = power_kw x hours x carbon_intensity_kgCO2_per_kwh x PUE
```

That shape matches CCF compute only if `power_kw` is already the
utilization-weighted average, `hours` is vCPU-hours or the watts are
already scaled by vCPU count, and the intensity unit matches the factor.
CCF publishes metric tons CO2e per kWh, not kgCO2/kWh. CCF also adds
embodied compute emissions, and separate memory, storage, networking,
GPU-hour, and replication terms. The issue formula has none of those.

This spike does not rewrite the formula into constants. A later task
would have to name the watt source, the utilization, the hours, the PUE
(1.125 or 1.185 on the pages above), and the intensity vintage before
any number is code.

## 7. Can power_kw be derived without utilization

No measured `power_kw`. Average watts on the CCF page depend on
utilization. The only published substitute is the 50% constant. Min and
max watts without that constant are a range, not one power figure.
Memory energy does not use CPU utilization, but it still needs hours and
the SPECPower excess, which this plugin does not have for a quote.

Azure size pages fetched here do not publish TDP, so there is no
nameplate watt to substitute.

## 8. Does static intensity data violate "No Bulk Data Embedding"

`CONTEXT.md` says: "We do NOT embed the Azure pricing catalog (which is
massive). We fetch data dynamically based on the requested resources."

A carbon-intensity table is not that catalog. The Retail Prices API is
the pricing source, and question 1 records that it has no carbon field.
The archived Azure instance file is 595 data rows and contains no prices.
The processor watt map is one TypeScript object. Embedding that class of
table does not, by the sentence in `CONTEXT.md`, embed the retail
catalog.

It is still a bad default until one source is pinned. The methodology
page, the archived grid CSV, and the trunk constants file do not match.
Embedding all three would not make a number. Embedding one without
saying which one copies a stale or conflicting table into the binary.

The issue's note that a small region table is likely acceptable as
configuration is consistent with the wording of the boundary. It is not
permission to invent the missing regions or to fill `ImpactMetrics` in
this spike.

## Data sources

| Source | Fits the no-auth boundary | What it does not give |
| --- | --- | --- |
| Azure Carbon Optimization API | No. Entra token, subscription, RBAC. | A prospective SKU estimate. It reports allocated emissions for existing subscriptions. |
| Emissions Impact Dashboard | No. Billing-account admin. Retires 2027-03-31. | A public kgCO2/kWh table. |
| Azure emissions methodology | The page is public. The customer factors are not. | Numeric grid factors. |
| Retail Prices API | Yes. No carbon fields. | Any carbon number. |
| CCF methodology and trunk constants | Yes. Public pages, no Azure login. | One agreed watt, PUE, or regional factor. Usage hours still come from an authenticated consumption API in the CCF app. |
| Archived `azure-instances.csv` | Yes. Public, read-only since 2023-05-11. | Watts, `Standard_*` names, and current CPU families. 129 rows are `Unknown`. |
| B family size page | Yes. | TDP or watts. |

## Accuracy against Carbon Optimization

Not published on the sources fetched. The CCF methodology page says the
estimates are not a replacement for provider data and are not guaranteed
accurate. Carbon Optimization returns scope totals for a subscription's
past month. No fetched page states an error bound between those reports
and a CCF estimate for an Azure VM size. This spike does not invent one.

## Recommendation

Implement later. A labeled compute estimate is possible from public CCF
material only after a later task pins one coefficient source, maps
current Azure SKU names to a processor, and states the hours and
utilization assumptions. Matching Azure Carbon Optimization is not
feasible from public data. This spike does not have a single watt value,
a single PUE, a single regional factor, or measured utilization.

## Follow-up issue

Not opened. Suggested title:

```text
Implement a labeled Azure VM carbon estimate from one pinned CCF source
```

Suggested body:

```text
Depends on docs/findings/carbon.md (AZ-2.16, issue 56).

Goal: fill GetProjectedCostResponse.impact_metrics for an Azure VM
with METRIC_KIND_CARBON_FOOTPRINT. The field already exists on
finfocus-spec. Do not add a new RPC field.

Do not call the Azure Carbon Optimization API, the Emissions Impact
Dashboard, or any other authenticated Azure API. Do not read the
Retail Prices API for carbon. It has no carbon field.

Before any coefficient is copied into Go, pin exactly one public
source and record the commit or page date in the PR. The methodology
page, the archived grid CSV, and
AzureFootprintEstimationConstants.ts on trunk do not agree. Appendix I
lists Azure average PUE 1.125. The PUE section and PUE_AVG list 1.185.
Azure average min/max watts are 0.78 and 3.76 on the methodology page
and 0.74 and 3.54 in the trunk file. Regional factors are metric tons
CO2e per kWh, not kgCO2/kWh, and the copies differ. Do not convert
units unless the chosen source publishes the conversion. Do not fill
a region the pinned source omits.

Map ARM SKU names such as Standard_B1s onto the pinned processor list.
azure-instances.csv has no Standard_ prefix, no Bsv2 row, and no Ice
Lake, Sapphire Rapids, Emerald Rapids, Cobalt, or Altra row. Unknown
processors must not silently use an average unless the pinned source
says to, and the response must say that the processor was unknown.

State the hours and the utilization in the metric or in metadata.
CCF uses measured vCPU hours and falls back to 50% utilization only
when utilization is missing. This plugin has neither measurement.
Copying the AWS plugin's 730 hours and 0.50 is an assumption and must
be labeled as an assumption, not as Azure telemetry.

Do not claim parity with Carbon Optimization. No public source fetched
for the spike gives an error bound. Storage, SQL, Cosmos DB, and GPU
sizes are out of scope until the VM path has one pinned source.
```
