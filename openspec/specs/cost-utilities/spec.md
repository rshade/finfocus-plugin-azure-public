# cost-utilities Specification

## Purpose

Provide pure conversions between hourly, monthly, and yearly rates, savings fractions, and
reservation hourly rates, so every quote uses the same 730-hour month and rounding.

## Requirements

### Requirement: Rate conversions with currency rounding

`HourlyToMonthly` SHALL multiply by 730 hours, `HourlyToYearly` by 8760 hours, and
`MonthlyToHourly` SHALL divide by 730. Each result SHALL be rounded half away from zero to two
decimal places. Zero and negative inputs SHALL convert the same way.

Tests: `Test_roundCurrency`, `TestHourlyToMonthly`, `TestHourlyToYearly`, `TestMonthlyToHourly`,
`TestRoundTripConsistency`, `TestAllResultsTwoDecimalPlaces`

#### Scenario: Standard rate

- **WHEN** the hourly rate is 0.10
- **THEN** `HourlyToMonthly` returns 73.00 and `HourlyToYearly` returns 876.00

#### Scenario: Rounding

- **WHEN** `HourlyToMonthly(0.105)` and `MonthlyToHourly(5.00)` are called
- **THEN** they return 76.65 and 0.01

### Requirement: Savings fraction

`SavingsFraction(onDemand, other)` SHALL return `(onDemand - other) / onDemand` without rounding.
An `onDemand` of zero or less, or a negative `other`, SHALL return an error naming that argument.
An `other` above `onDemand` SHALL give a negative fraction.

Tests: `TestSavingsFraction`, `TestSavingsFractionD2sV3Fixture`,
`TestPriceItemParsesSavingsPlanFixture`

#### Scenario: Savings plan terms from the preview body

- **WHEN** the eastus `Standard_D2s_v3` Linux row (0.096 per hour) is decoded with its nested
  `savingsPlan` array (1 Year 0.06624, 3 Years 0.04512)
- **THEN** `PriceItem.SavingsPlan` holds both terms
- **AND** the fractions are 0.31 and 0.53

### Requirement: Reservation hourly rate

`ReservationHourly` SHALL treat a Reservation `retailPrice` as the term total and divide it by
8760 hours for `1 Year` and 26280 hours for `3 Years`.

Tests: `TestReservationHourlyUsesTermTotal`

#### Scenario: Reservation fixture

- **WHEN** each Reservation row of the eastus `Standard_D2als_v7` fixture is converted
- **THEN** the hourly rate equals `retailPrice / 8760` for 1 Year and `retailPrice / 26280` for
  3 Years
