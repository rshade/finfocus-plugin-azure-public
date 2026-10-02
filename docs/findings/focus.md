# FOCUS alignment

AZ-2.14 maps issue #46 onto the SDK FOCUS record builder.
`GetProjectedCost` stays one monthly price. It does not attach a FOCUS
record. A virtual machine quote can also return advisory `price_options`
and `region_prices`. Those lists stay out of the monthly cost.

`GetActualCost` attaches `FocusRecord` when a billing account id is
available. `GetActualCostRequest.billing_account_id` wins when the caller
sends one. A dry run ignores that field. An empty request id falls back
to `SetBillingAccountID`, which reads `FINFOCUS_BILLING_ACCOUNT_ID` at
startup. When both are empty, `Build` returns an error that names
`billing_account_id`, the error is logged at warn, and `FocusRecord`
stays nil. When an id is present, the record passes `ValidateFocusRecord`.
The scaled actual cost is the same value either way. No billing account
id, invoice id, or tag is invented. Tags are filters. The request id is
a record field, and it is not a price filter.
[Spec issue 590](https://github.com/rshade/finfocus-spec/issues/590)
added the request field.

`ValidateFocusRecord` calls `validateMandatoryFields` in
`../finfocus-spec/sdk/go/pluginsdk/focus_conformance.go`. An empty
`billing_account_id` is rejected. `Build` in
`../finfocus-spec/sdk/go/pluginsdk/focus_builder.go` calls that
validation.

Issue #46 stays open. The column table below still disagrees with the
issue checklist. `ChargeType` has no proto field. This plugin leaves
`commitment_discount_type` empty, because a retail price is not a
commitment discount.

The line numbers in the table were read from spec v0.7.0 `focus.pb.go`.
The plugin now depends on `github.com/rshade/finfocus-spec`
`v0.7.1-0.20261002115132-9eccf57a87b5`. Those line numbers are the
v0.7.0 reading.

## Issue #46 columns

Field lines below are `FocusCostRecord` in
`../finfocus-spec/sdk/go/proto/finfocus/v1/focus.pb.go` unless the row names
another file. `ChargeType` has no field. `CommitmentDiscountType` does, under
the proto name `commitment_discount_type`.

| Issue column | v0.7.0 field | Line read |
| --- | --- | --- |
| `ListCost` | `list_cost` | 727 |
| `EffectiveCost` | `effective_cost` | 729 |
| `BilledCost` | `billed_cost` | 725 |
| `PricingUnit` | `pricing_unit` | 669 |
| `PricingQuantity` | `pricing_quantity` | 667 |
| `Provider` | `provider_name`, deprecated; replacement `service_provider_name` | 629 and 784 |
| `ServiceName` | `service_name` | 691 |
| `ServiceCategory` | `service_category` | 689 |
| `Region` | `region_id` and `region_name` | 719 and 721 |
| `ResourceType` | `resource_type` | 707 |
| `ChargeType` | absent | no `charge_type`; `charge_category` is 656 and `charge_class` is 659 |
| `PricingCategory` | `pricing_category` | 665 |
| `CommitmentDiscountType` | `commitment_discount_type` | 757 |
| `CurrencyCode` | `billing_currency` | 650 |

`Provider` is not a v0.7.0 field name. Line 629 is the deprecated
`provider_name`. Line 784 is `service_provider_name`. The builder calls
`WithIdentity` and does not call `WithServiceProvider`. Setting both logs a
deprecation warning (`focus_builder.go` lines 590-599).

`ChargeType` is absent. A search of `focus.pb.go` and
`../finfocus-spec/proto/finfocus/v1/focus.proto` found no `charge_type`.
The charge classification fields are `charge_category` and `charge_class`.

`CommitmentDiscountType` is the FOCUS column name. The proto field is
`commitment_discount_type` (`focus.proto` lines 365-367, generated at
`focus.pb.go` line 757). This plugin leaves it empty. Retail prices are not
a commitment discount, and the builder does not set a commitment discount id.

`CurrencyCode` is not the proto name. The FOCUS field is `billing_currency`
(line 650). `GetProjectedCostResponse.Currency` is the separate field
`currency` at `costsource.pb.go` line 1490. There is no `currency_code` on
`FocusCostRecord`. An empty quote currency is passed through. It is not
replaced with `USD`. `Build` then fails on `billing_currency`.
