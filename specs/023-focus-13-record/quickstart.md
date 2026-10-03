# Quickstart: FOCUS 1.3 Cost Record

Set a billing account id, then call `GetActualCost`:

```bash
export FINFOCUS_BILLING_ACCOUNT_ID=example-account
go test ./internal/pricing/ -run 'Focus' -count=1 -v
```

For a single Linux virtual machine over 24 hours, the record carries:

- `service_provider_name` and `host_provider_name`: `Microsoft`
- `service_category`: `Compute`; `service_subcategory`: `Virtual Machines`
- `pricing_unit`: `Hours`, `pricing_quantity`: `24`, `list_unit_price`: the
  meter's hourly retail price
- `list_cost`, `billed_cost`, `effective_cost`, `contracted_cost`: price × 24

`docs/focus-mapping.md` lists every column and its Azure source.
