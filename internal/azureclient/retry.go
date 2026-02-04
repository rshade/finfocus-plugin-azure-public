package azureclient

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// customRetryPolicy determines whether a request should be retried.
// It retries on:
//   - Network errors (connection refused, timeout, DNS failure)
//   - HTTP 429 (Too Many Requests)
//   - HTTP 503 (Service Unavailable)
//
// It does NOT retry on:
//   - Context cancellation
//   - HTTP 4xx errors (except 429)
//   - HTTP 5xx errors (except 503)
//   - Successful responses (2xx)
func customRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// Don't retry if context is cancelled
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// Retry on network errors - we intentionally return nil error to signal retry
	if err != nil {
		return true, nil //nolint:nilerr // Intentional: err is transient, return nil to trigger retry
	}

	// Retry on rate limit (429) and service unavailable (503)
	if resp != nil {
		if resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusServiceUnavailable {
			return true, nil
		}
	}

	// Don't retry other status codes
	return false, nil
}

// parseRetryAfter parses the Retry-After header from an HTTP response.
// It supports two formats as per RFC 7231:
//   - Seconds (integer): "120" → 120 seconds
//   - HTTP-date: "Wed, 21 Oct 2015 07:28:00 GMT" → duration until that time
//
// Returns 0 if the header is missing, invalid, or represents a past time.
func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	header := resp.Header.Get("Retry-After")
	if header == "" {
		return 0
	}

	// Try parsing as seconds
	if seconds, err := strconv.Atoi(header); err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		return 0
	}

	// Try parsing as HTTP-date
	if t, err := http.ParseTime(header); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return duration
		}
		return 0
	}

	return 0
}
