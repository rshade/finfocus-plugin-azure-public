# price-client Specification

## Purpose

Query the Azure Retail Prices API with a bounded retry policy, follow pagination up to a fixed
limit, build OData filters deterministically, and classify failures into sentinel errors that
map to gRPC status codes.

## Requirements

### Requirement: Retry policy

The client SHALL retry network errors, HTTP 429, and HTTP 503. It SHALL NOT retry HTTP 200, other
4xx statuses (400, 401, 403, 404), or other 5xx statuses (500, 502, 504). A cancelled context
SHALL stop retrying and return `context.Canceled`.

Tests: `TestCustomRetryPolicy_ContextCancelled`, `TestCustomRetryPolicy_NetworkError`,
`TestCustomRetryPolicy_RateLimit429`, `TestCustomRetryPolicy_ServiceUnavailable503`,
`TestCustomRetryPolicy_NoRetryOn4xx`, `TestCustomRetryPolicy_NoRetryOn5xx`,
`TestCustomRetryPolicy_Success200`, `TestClient_GetPrices_Retry429`,
`TestClient_GetPrices_Retry503`, `TestClient_GetPrices_NoRetryOn400`,
`TestClient_GetPrices_Logging`

#### Scenario: Rate limit recovers after retries

- **WHEN** the API answers 429 twice and then a price page
- **THEN** `GetPrices` succeeds after exactly 3 requests

#### Scenario: Bad request is not retried

- **WHEN** the API answers 400
- **THEN** `GetPrices` returns an error after exactly 1 request

### Requirement: Retry-After parsing

The client SHALL read `Retry-After` as whole seconds or as an HTTP date. A missing header, a nil
response, an unparsable value, a floating point value (`5.5`), zero, a negative number, and a
date in the past SHALL all yield a zero wait.

Tests: `TestParseRetryAfter_Seconds`, `TestParseRetryAfter_EmptyHeader`,
`TestParseRetryAfter_InvalidValue`, `TestParseRetryAfter_HTTPDate`,
`TestParseRetryAfter_NegativeDuration`, `TestParseRetryAfter_ZeroSeconds`,
`TestParseRetryAfter_NegativeSeconds`, `TestParseRetryAfter_LargeNumber`,
`TestParseRetryAfter_FloatingPoint`, `TestParseRetryAfter_NilResponse`

#### Scenario: Seconds and dates

- **WHEN** `Retry-After` is `5`
- **THEN** the wait is 5 seconds
- **AND** an HTTP date 10 seconds ahead yields a wait of about 10 seconds

### Requirement: Error classification

`GetPrices` SHALL classify failures with sentinel errors that `errors.Is` recognizes: HTTP 404 is
`ErrNotFound` with `status 404` in the message; 429 after retries is `ErrRateLimited` with
`status 429`; 503 after retries is `ErrServiceUnavailable` with `status 503`; other non-retried
statuses such as 400 are `ErrRequestFailed`; a 200 body that is not valid JSON is
`ErrInvalidResponse` and the message SHALL carry at most the first 256 bytes of the body. A
cancelled context SHALL surface `context.Canceled`. An invalid client configuration (negative
`RetryMax`, zero `Timeout`, `RetryWaitMin` greater than `RetryWaitMax`) SHALL be `ErrInvalidConfig`.

Tests: `TestFetchPage_HTTP404_ReturnsErrNotFound`, `TestClient_GetPrices_RateLimitExhausted`,
`TestClient_GetPrices_ServiceUnavailableExhausted`, `TestGetPrices_ErrorPreservesRootCause`,
`TestClient_GetPrices_InvalidJSON`, `TestFetchPage_InvalidJSON_IncludesResponseSnippet`,
`TestFetchPage_LargeResponseBody_TruncatedAt256Bytes`, `TestClient_GetPrices_ContextCancelled`,
`TestNewClient_InvalidConfig`, `TestNewClient_DefaultConfig`

#### Scenario: Invalid body snippet is truncated

- **WHEN** the API answers 200 with a 500-byte non-JSON body
- **THEN** the error is `ErrInvalidResponse`
- **AND** the message contains a 256-byte snippet and not the full body

### Requirement: Query context and empty results

Every `GetPrices` error SHALL include the query context, such as `region=eastus` and
`sku=Standard_B1s`, and the zero-based page index of the failing request (the second request is
`page 1`). A query whose pages hold no rows SHALL return `ErrNotFound` whose message includes the
query context and `no pricing data`.

Tests: `TestGetPrices_ErrorIncludesQueryContext`, `TestGetPrices_MidPaginationErrorIncludesPage`,
`TestGetPrices_EmptyResults_ReturnsErrNotFound`, `TestGetPrices_NonEmptyResults_ReturnsSuccess`,
`TestClient_GetPrices_Success`

#### Scenario: Empty answer is not found

- **WHEN** the API answers 200 with `Items` empty
- **THEN** `GetPrices` returns `ErrNotFound`
- **AND** the message contains `region=` and `no pricing data`

#### Scenario: Second page fails

- **WHEN** the first page succeeds with a `NextPageLink` and the second answers 500
- **THEN** the error message contains `page 1` and `region=eastus`

### Requirement: Pagination

`GetPrices` SHALL follow `NextPageLink` and return the rows of every page in order, including
through an empty page that has a next link. It SHALL stop after `MaxPaginationPages` (10) pages:
exactly 10 pages succeed, and a query that still has a next link after 10 pages SHALL return
`ErrPaginationLimitExceeded` without an 11th request. Cancelling the context between pages SHALL
return `context.Canceled`. Each page after the first SHALL log a debug `pagination progress`
entry with `page`, `items_this_page`, and `total_items`; a single-page query SHALL log none.

Tests: `TestClient_GetPrices_Pagination`, `TestClient_GetPrices_ThreePageQueryReturnsAll250Items`,
`TestClient_GetPrices_SinglePageQueryDoesNotRequestAdditionalPages`,
`TestClient_GetPrices_EmptyPageWithNextLinkFollowsPagination`,
`TestClient_GetPrices_ContextCancellationMidPagination`,
`TestClient_GetPrices_ExceedingTenPagesReturnsPaginationLimitExceeded`,
`TestClient_GetPrices_ExactlyTenPagesSucceeds`,
`TestClient_GetPrices_MultiPageQueryLogsPaginationProgress`,
`TestClient_GetPrices_SinglePageQueryEmitsNoPaginationProgressLogs`

#### Scenario: Three pages

- **WHEN** the API returns pages of 100, 100, and 50 rows
- **THEN** `GetPrices` returns 250 rows from `SKU-001` to `SKU-250` after 3 requests

#### Scenario: Page limit

- **WHEN** every page has a next link for 11 pages
- **THEN** `GetPrices` returns `ErrPaginationLimitExceeded` after 10 requests

### Requirement: OData filter output

The filter SHALL always contain a price type clause, `priceType eq 'Consumption'` by default; a
non-blank `Type` or `PriceQuery.PriceType` SHALL replace it. Clauses SHALL be joined with ` and `
in sorted order, so the output does not depend on call order. Blank names or values SHALL be
omitted, a named field set twice SHALL keep the last value, and `Or` groups with two or more
conditions SHALL be parenthesized and sorted inside. Single quotes in values SHALL be doubled.
A query with `APIVersion` SHALL send `api-version` beside `$filter`.

Tests: `TestEscapeODataValue`, `TestIsBlank`, `TestConditionConstructors`,
`TestFilterBuilderSingleFieldFilters`, `TestFilterBuilderMultiFieldFilters`,
`TestFilterBuilderTypeDefaultAndOverride`, `TestFilterBuilderFieldMethod`,
`TestFilterBuilderBuildMinimal`, `TestFilterBuilderLastWriteWins`, `TestFilterBuilderOrGroups`,
`TestFilterBuilderMixedAndOr`, `TestFilterBuilderDeterministicOrdering`,
`TestFilterBuilderFluentChaining`, `TestFilterBuilderEscapingInOutput`,
`TestFilterBuilderEdgeCases`, `TestBuildFilterQuery_Empty`, `TestBuildFilterQuery_SingleField`,
`TestBuildFilterQuery_MultipleFields`, `TestBuildFilterQuery_AllFields`,
`TestBuildFilterQuery_PriceTypeOverride`, `TestBuildFilterQuery_ODataEscape`,
`TestBuildFilterQuery_ODataEscapeMultipleQuotes`, `TestBuildFilterQuery_ODataInjectionPrevention`,
`TestClient_GetPrices_SendsAPIVersion`

#### Scenario: Or group with an and clause

- **WHEN** the builder has `Or(Region("eastus"), Region("westus2"))` and `Service("Virtual Machines")`
- **THEN** `Build()` returns `(armRegionName eq 'eastus' or armRegionName eq 'westus2') and
  priceType eq 'Consumption' and serviceName eq 'Virtual Machines'`

#### Scenario: Injection is neutralized

- **WHEN** the region is `east' or 'a' eq 'a`
- **THEN** the filter is `armRegionName eq 'east'' or ''a'' eq ''a' and priceType eq 'Consumption'`

### Requirement: Structured error logging

A failed query SHALL log a `pricing query error` entry with `region`, `sku`, and
`error_category` fields. The level SHALL be `warn` for HTTP 400, `error` for HTTP 500 and invalid
JSON, and `debug` for an empty result. A zero-value logger SHALL NOT panic.

Tests: `TestGetPrices_LogsErrorWithStructuredFields`, `TestGetPrices_LogSeverityDifferentiation`,
`TestZeroValueLogger_NoPanic`

#### Scenario: Severity by category

- **WHEN** the API answers 500 for region `eastus` and SKU `Standard_B1s`
- **THEN** the `pricing query error` entry has level `error`, `region=eastus`, and
  `sku=Standard_B1s`

### Requirement: gRPC status mapping

`MapToGRPCStatus` SHALL map nil to `OK`, `context.Canceled` to `Canceled`,
`context.DeadlineExceeded` to `DeadlineExceeded`, `ErrNotFound` to `NotFound`, `ErrRateLimited`
to `ResourceExhausted`, `ErrServiceUnavailable` to `Unavailable`, `ErrUnsupportedResourceType` to
`Unimplemented`, `ErrMissingRequiredFields` to `InvalidArgument`, and `ErrRequestFailed`,
`ErrInvalidResponse`, `ErrInvalidConfig`, `ErrPaginationLimitExceeded`, and unknown errors to
`Internal`. Wrapped sentinels SHALL map the same way, and the status message SHALL equal the
error text.

Tests: `TestMapToGRPCStatus`, `TestMapToGRPCStatus_WrappedErrors`,
`TestMapToGRPCStatus_WrappedUnsupportedResourceType`,
`TestMapToGRPCStatus_WrappedMissingRequiredFields`, `TestMapToGRPCStatus_PreservesErrorMessage`

#### Scenario: Wrapped rate limit

- **WHEN** the error is `ErrRateLimited` wrapped with `status 429: too many requests`
- **THEN** the status code is `ResourceExhausted`
- **AND** the status message equals the error text
