# Data Model: FOCUS 1.3 Cost Record Alignment

No new persisted data. The in-memory values are listed below.

## Pricing basis

| Field | Type | Rule |
| --- | --- | --- |
| quantity | float64 | window hours × count, window months × count, or window hours |
| unit | string | `Hours`, `Units/Month`, or `Months` (PricingUnits.csv) |
| unitPrice | float64 | meter price, or 0 for a blended quote |

Invariant: when `unitPrice` is set, `unitPrice × quantity == cost` within the
SDK tolerance.
`quantity > 0` whenever window hours > 0.

## Service class

| Field | Type | Rule |
| --- | --- | --- |
| name | string | Services.csv ServiceName for the resource type |
| category | `FocusServiceCategory` | from the resource type |
| subcategory | string | FOCUS 1.3 allowed value whose parent is the category |

## FocusCostRecord columns changed

| Column | Before | After |
| --- | --- | --- |
| provider_name | `azure` | `Microsoft` (deprecated, still mandatory in 1.3) |
| publisher | empty | `Microsoft` (deprecated, still mandatory in 1.3) |
| invoice_issuer | empty | `Microsoft` |
| billing_account_name | empty | billing account id |
| service_provider_name | empty | `Microsoft` |
| host_provider_name | empty | `Microsoft` |
| contracted_cost | 0 | window cost |
| contracted_unit_price | 0 | unitPrice |
| pricing_unit / consumed_unit | `hour` | `Hours`, `Units/Month`, or `Months` |
| pricing_quantity / consumed_quantity | window hours | basis quantity |
| list_unit_price | cost / hours | meter price, or unset when blended |
| service_name | Retail API service | Services.csv ServiceName |
| service_category (disk) | Storage | Compute |
| service_subcategory | empty | mapped value |
