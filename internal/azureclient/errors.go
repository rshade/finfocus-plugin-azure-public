package azureclient

import "errors"

// Sentinel errors returned by the client.
var (
	// ErrInvalidConfig is returned when the client configuration is invalid.
	ErrInvalidConfig = errors.New("invalid client configuration")

	// ErrInvalidResponse is returned when the API response cannot be parsed.
	ErrInvalidResponse = errors.New("invalid API response")

	// ErrRateLimited is returned when the API returns HTTP 429 and the response
	// is processed (e.g., when retries are disabled for this status code).
	// Note: When retries ARE enabled and exhausted, ErrRequestFailed is returned
	// wrapping the underlying retry library's error.
	ErrRateLimited = errors.New("rate limited")

	// ErrServiceUnavailable is returned when the API returns HTTP 503 and the
	// response is processed. Note: When retries ARE enabled and exhausted,
	// ErrRequestFailed is returned wrapping the underlying retry library's error.
	ErrServiceUnavailable = errors.New("service unavailable")

	// ErrRequestFailed is returned when the request fails, including when all
	// retry attempts are exhausted. The wrapped error contains details about
	// the failure (e.g., network error, retry exhaustion).
	ErrRequestFailed = errors.New("request failed")
)
