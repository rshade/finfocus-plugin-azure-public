# Real Pulumi inputs

Owner-owned, read-only for run agents. **Do not edit these files, and never invent a property
name.** Every property name the plugin reads must appear in `pulumi-property-map.json` or in a
`newState.inputs` block of the genuine previews below.

## What is here

| File | What it is | Genuine? |
| --- | --- | --- |
| `preview-native.json` | raw `pulumi preview --json` for azure-native 3.28.0, one resource per cost-relevant type, 19 steps | yes |
| `preview-classic.json` | raw `pulumi preview --json` for azure (classic) 6.40.0, 27 steps. Ran through a local auth stand-in because the provider validates credentials on configure | yes |
| `pulumi-property-map.json` and `.md` | per provider and type: the price-relevant input properties with path, type, allowed values, defaults, and the Windows, Spot and region rules, from the provider schemas | schema-derived |
| `core-view.json` | what the finfocus core hands the plugin for each resource (`Sku`, `Region`, `Tags`), from a copy of the core's flattening code run over the genuine inputs | simulation of the core, not the core binary |
| `gap-table.md` | real names against core against plugin, with the consequence and who owns each fix | code reading |
| `plan-expected.json` | the result a correct plugin must give for each resource in the genuine previews, from the live Retail Prices API by an independent script | independent |

Fake values: the previews were made with zero-GUID credentials, a fake password, and the
sentinel `04da6b54-80e4-46f7-96ec-b56ff0331ba9` that Pulumi prints for a value that depends on
another resource. Nothing was created in any cloud.

## What the core really sends today

The core flattens every input to a string: an object becomes its `value`, `id` or `name`, or its
single inner value, so `hardwareProfile: {vmSize: X}` arrives as the tag `hardwareProfile=X`, and
`sku: {name: P1v3, capacity: 2}` arrives as `P1v3` with the capacity lost. The SKU is read only
from the top-level keys `vmSize`, `sku` and `tier`. `core-view.json` is the plugin's real input
contract today.

## Rules for the comparison test (AZ-7.1)

1. **Two input sets.** Build each request from `core-view.json` (today), and from a deterministic
   helper that flattens the genuine inputs with dotted keys (`hardwareProfile.vmSize`,
   `sku.capacity`), which is what the core would send after the fix the plan asks for. Report the
   two results separately.
2. **Statuses in `plan-expected.json`.**
   - `ok`: the monthly cost must be within `max(0.01, 0.5%)` of `expected_monthly`.
   - `usage_required`: the response must state the assumption it made, or be an explicit error. Never zero.
   - `needs_parent_resource`: an explicit error or note saying the cost is on the referenced resource. Never zero.
   - `ambiguous`: a documented choice, recorded in the report. Never zero.
   - `unsupported_must_error`: an explicit unsupported error. Never a price for a different product.
3. **The Hybrid Benefit rule.** `licenseType` of `Windows_Server` is Azure Hybrid Benefit: the
   licence is already paid, so the compute rate is the base (Linux) rate. The expected value reflects
   that, and `alternative_if_licence_included` gives the Windows-meter price for a resource with no
   `licenseType`. State the rule in the response note.
4. **Calculator values still win** where the owner has filled `calculator-values.csv`.
5. **Write the results** (resource, input set, expected, actual, verdict) to
   `.superpowers/real-plan-results.md` and paste the table in the report.
