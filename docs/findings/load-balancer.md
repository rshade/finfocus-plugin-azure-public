# Load Balancer prices

Standard Load Balancer publishes several meters, so one row is not the whole
bill. This quote prices the Standard rules meters and states the usage it
assumed.

## What the live API returned

On 2026-10-01 the filter `serviceName eq 'Load Balancer' and armRegionName eq 'eastus' and priceType eq 'Consumption'`
returned no rows and no next page.

The same service and price type at `armRegionName eq 'Global'` returned 12
rows. The Standard rows used here are:

| `skuName` | `meterName` | `unitOfMeasure` | `retailPrice` |
| --- | --- | --- | --- |
| `Standard` | `Standard Included LB Rules and Outbound Rules` | `1 Hour` | 0.025 |
| `Standard` | `Standard Overage LB Rules and Outbound Rules` | `1/Hour` | 0.01 |
| `Standard` | `Standard Data Processed` | `1 GB` | 0.005 |

Rows whose meter name ends in `Free` are 0 and are not selected. Gateway
meters and the cross-region meters are a different shape and are not quoted.

## Assumptions

The public price table bills the first 5 load-balancing and outbound rules
as one hourly charge. That charge is the included meter. Additional rules
use the overage meter. Inbound NAT rules are not counted.

- Omitted `rule_count` bills the included meter once and says so.
- `rule_count` 0 has no hourly charge. The cost is 0 because no rules are configured, and the billing note says that.
- `rule_count` past 5 adds `(count - 5)` times the overage meter times 730.
- Omitted `data_processed_gb` adds no data component. A set value uses the data-processed meter.

A regional page that already contains the included meter is used as it is.
An empty regional page is read again at `Global`.
