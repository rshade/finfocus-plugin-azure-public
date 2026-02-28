// Package contracts defines the API contract for error handling in the
// azureclient and pricing packages. This file is a design artifact and
// is NOT compiled as part of the project.
//
// It documents the expected function signatures and behavior for the
// error handling feature (008-azure-error-handling).
package contracts

// --- azureclient package: errors.go ---

// Sentinel errors (existing + new):
//
//   ErrInvalidConfig           - configuration validation failure
//   ErrInvalidResponse         - JSON parse failure (includes response snippet)
//   ErrRateLimited             - HTTP 429 when response is processed directly
//   ErrServiceUnavailable      - HTTP 503 when response is processed directly
//   ErrRequestFailed           - generic HTTP/network failure, retry exhaustion
//   ErrNotFound                - HTTP 404 or empty result set (NEW)
//   ErrPaginationLimitExceeded - pagination safety limit (MOVED from client.go)

// --- azureclient package: client.go ---

// GetPrices contract changes:
//
// Current:  func (c *Client) GetPrices(ctx, query) ([]PriceItem, error)
// After:    Same signature, but:
//   - Errors are wrapped with query context:
//     fmt.Errorf("query [region=%s sku=%s service=%s]: %w", ...)
//   - Empty results (0 items after all pages) return:
//     fmt.Errorf("%w: query [region=%s sku=%s service=%s]: no pricing data",
//       ErrNotFound, ...)
//   - Mid-pagination errors include page number:
//     fmt.Errorf("query [...] page %d: %w", page, err)
//   - All errors are logged with structured fields before returning

// fetchPage contract changes:
//
// Current:  func (c *Client) fetchPage(ctx, url) ([]PriceItem, string, error)
// After:    Same signature, but:
//   - HTTP 404 returns: fmt.Errorf("%w: status 404: %s", ErrNotFound, body)
//   - JSON errors include response snippet (up to 256 bytes):
//     fmt.Errorf("%w: %w (response: %.256s)", ErrInvalidResponse, err, body)
//   - Response body for error cases read via io.LimitReader (256 bytes max)

// --- pricing package: errors.go (NEW) ---

// MapToGRPCStatus maps an azureclient error to a gRPC status code and message.
//
// Contract:
//   func MapToGRPCStatus(err error) *status.Status
//
// Mapping (evaluated via errors.Is in priority order):
//   context.Canceled           -> codes.Canceled
//   context.DeadlineExceeded   -> codes.DeadlineExceeded
//   ErrNotFound                -> codes.NotFound
//   ErrRateLimited             -> codes.ResourceExhausted
//   ErrServiceUnavailable      -> codes.Unavailable
//   ErrRequestFailed           -> codes.Internal
//   ErrInvalidResponse         -> codes.Internal
//   ErrInvalidConfig           -> codes.Internal
//   ErrPaginationLimitExceeded -> codes.Internal
//   default                    -> codes.Internal
//
// The error message from err.Error() is preserved in the gRPC status.
