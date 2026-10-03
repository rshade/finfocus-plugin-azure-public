# Quickstart: FOCUS 1.3 Cost Record

The tests set the billing account id themselves (`SetBillingAccountID` or the
request `billing_account_id`), so no environment variable is needed:

```bash
go test ./internal/pricing/ -run 'Focus' -count=1 -v
```

A running plugin builds the record when the request carries
`billing_account_id` or the process has `FINFOCUS_BILLING_ACCOUNT_ID` set.

For a single Linux virtual machine over 24 hours, the record carries:

- `service_provider_name` and `host_provider_name`: `Microsoft`
- `service_category`: `Compute`; `service_subcategory`: `Virtual Machines`
- `service_name`: `Virtual Machines`
- `pricing_unit`: `Hours`, `pricing_quantity`: exactly `24`, `list_unit_price`:
  the meter's hourly retail price
- `list_cost`, `billed_cost`, `effective_cost`, `contracted_cost`: price × 24

`docs/focus-mapping.md` lists every column and its Azure source.
