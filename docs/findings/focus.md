# FOCUS alignment

AZ-2.14 maps issue #46 onto finfocus-spec v0.7.0. `GetProjectedCost` stays
one price. It does not grow a FOCUS record.

`GetProjectedCostResponse` is
`../finfocus-spec/sdk/go/proto/finfocus/v1/costsource.pb.go` lines 1484-1672.
The struct ends at `cost_breakdown` (line 1669). There is no `focus_record`.
`metadata` (line 1635) is a string map for plugin hints. `cost_breakdown`
(line 1669) is component amounts of that one monthly price. FOCUS columns
are not written into either map.

`ActualCostResult.FocusRecord` is lines 2323-2325. The comment says the
field is optional and will eventually replace the legacy fields.
`GetActualCostRequest` is lines 1132-1168. Its fields are `resource_id`,
`start`, `end`, `tags`, `arn`, `dry_run`, `page_size`, and `page_token`.
There is no billing-account field.

`ValidateFocusRecord` calls `validateMandatoryFields` in
`../finfocus-spec/sdk/go/pluginsdk/focus_conformance.go`. Lines 200-201
reject an empty `billing_account_id`. `Build` in
`../finfocus-spec/sdk/go/pluginsdk/focus_builder.go` lines 575-581 calls
that validation.

Production calls `buildFocusRecord` with an empty billing account id.
`Build` returns an error that names `billing_account_id`. That error is
logged at warn and `FocusRecord` stays nil. The scaled actual cost is the
same value as before. No billing account id, invoice id, or tag is invented.

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
