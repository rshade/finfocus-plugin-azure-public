# Research: Pagination Handler for Azure API Responses

**Feature**: `010-pagination-handler`
**Date**: 2026-03-01

## R1: Current Pagination Implementation State

**Decision**: Enhance existing pagination loop, do not rewrite.

**Rationale**: The `GetPrices()` method in `client.go:112-148` already
implements the full pagination flow: loop over `fetchPage()`, aggregate
items, check safety limit, handle empty results. The changes needed
are targeted: reduce `MaxPaginationPages` from 1000 to 10 and add
progress logging inside the loop.

**Alternatives considered**:

- Separate `paginate()` method extracted from `GetPrices()` — rejected
  because the loop is only 10 lines and extraction adds indirection
  without reducing complexity.
- Iterator/channel-based pagination — rejected because callers need the
  full result set (no streaming use case exists).

## R2: Page Limit Value

**Decision**: Use constant `MaxPaginationPages = 10`.

**Rationale**: The spec (FR-003) and issue both specify max 10 pages.
Azure Retail Prices API returns up to 100 items per page
(confirmed via API documentation and live testing). 10 pages = ~1,000
items covers all realistic pricing queries (a specific SKU across all
regions returns ~60-120 items).

**Alternatives considered**:

- Configurable via `Config` struct — rejected per spec. The limit is a
  safety guard, not a tuning parameter. Can be added later if needed.
- Higher limit (100, 1000) — rejected. The current value of 1000 was a
  placeholder. Queries returning >1,000 items indicate a filter that is
  too broad, not a legitimate use case.

## R3: Pagination Progress Log Level

**Decision**: Use `debug` level for pagination progress logs.

**Rationale**: Per spec assumptions, debug level avoids noise in
production while remaining available for troubleshooting. Pagination is
a normal, expected operation — not a warning or error condition. The
`zerolog` library supports per-level filtering, so operators can enable
debug logs when investigating slow queries.

**Alternatives considered**:

- `info` level — rejected. Every multi-page query would produce
  multiple log lines in production. Pagination is internal plumbing,
  not a user-visible event.
- `trace` level — rejected. zerolog's `Trace()` is less commonly
  configured. `debug` is the standard level for "internal flow
  visibility" in this project.

## R4: Log Fields for Pagination Progress

**Decision**: Log `page` (int), `items_this_page` (int),
`total_items` (int) fields with message "pagination progress".

**Rationale**: These three fields give operators complete visibility:
which page was fetched, how many items it contained, and the running
total. The message "pagination progress" is consistent with the
existing "pricing query error" message pattern.

**Alternatives considered**:

- Include `next_page_url` — rejected. URLs contain query parameters
  that are already visible in the initial request. Logging them on
  every page adds noise.
- Include query context fields (region, sku, service) — considered
  but deferred. The query context is already logged on errors. For
  debug-level progress, the page numbers are sufficient.

## R5: Empty Page with NextPageLink Edge Case

**Decision**: Follow the link. Do not treat empty items as end-of-data.

**Rationale**: Per spec edge case 1, an empty items list with a
next-page link should be followed because the next page may contain
results. The existing code handles this correctly — `append(allItems,
items...)` with an empty slice is a no-op, and the loop continues
because `nextURL != ""`.

**Alternatives considered**:

- Treat empty page as end — rejected. Azure API documentation does
  not guarantee non-empty pages. Stopping early could lose data.

## R6: Test Strategy

**Decision**: Add 5 new test cases to `client_test.go` using
`httptest` mock servers.

**Rationale**: The spec requires testing for: pagination limit exceeded
(FR-003/FR-004), exactly-at-limit (US3 scenario 2), empty page with
NextPageLink (edge case), progress logging (FR-005), and context
cancellation mid-pagination (FR-008). All can be tested with
`httptest.NewServer` handlers that control response content and
page count.

**Alternatives considered**:

- Separate `pagination_test.go` file — rejected. Tests are
  closely related to existing `client_test.go` tests and share the
  same mock server patterns.
- Integration test for pagination — already exists implicitly via
  `TestAzureClient_LiveAPI_VirtualMachines` which queries across
  regions (may return >100 items). Add explicit multi-page integration
  test.
