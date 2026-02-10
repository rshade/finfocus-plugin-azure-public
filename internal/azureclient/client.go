package azureclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog"
)

const (
	// maxErrorBodyBytes is the maximum number of bytes to read from error response bodies.
	// Used for response snippets in error messages (256 bytes + 1 for truncation detection).
	maxErrorBodyBytes = 257

	// maxSnippetLen is the maximum length of a response snippet included in error messages.
	maxSnippetLen = 256

	// maxResponseBodyBytes is the maximum number of bytes to read from a success response body.
	// Azure API returns max 1000 items per page; 10 MB is a generous safety limit.
	maxResponseBodyBytes = 10 * 1024 * 1024

	// HTTP transport configuration for production use.
	transportMaxIdleConns        = 100
	transportMaxIdleConnsPerHost = 10 // Higher than default (2) for single-host API
	transportIdleConnTimeout     = 90 // seconds
)

// Client is an HTTP client for querying the Azure Retail Prices API.
type Client struct {
	httpClient *retryablehttp.Client
	baseURL    string
	userAgent  string
	logger     zerolog.Logger
}

// NewClient creates a new Azure Retail Prices API client.
// It validates the configuration and returns an error if invalid.
func NewClient(config Config) (*Client, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	// Apply defaults for empty fields
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	if config.UserAgent == "" {
		config.UserAgent = "finfocus-plugin-azure-public"
	}

	// Create retryable HTTP client with optimized transport for production
	transport := &http.Transport{
		MaxIdleConns:        transportMaxIdleConns,
		MaxIdleConnsPerHost: transportMaxIdleConnsPerHost,
		IdleConnTimeout:     transportIdleConnTimeout * time.Second,
	}

	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient.Transport = transport
	retryClient.RetryMax = config.RetryMax
	retryClient.RetryWaitMin = config.RetryWaitMin
	retryClient.RetryWaitMax = config.RetryWaitMax
	retryClient.HTTPClient.Timeout = config.Timeout
	retryClient.CheckRetry = customRetryPolicy
	retryClient.ErrorHandler = retryablehttp.PassthroughErrorHandler
	retryClient.Logger = &zerologAdapter{logger: config.Logger}

	// Custom backoff that respects Retry-After header
	retryClient.Backoff = func(minWait, maxWait time.Duration, attemptNum int, resp *http.Response) time.Duration {
		// Check Retry-After header first
		if retryAfter := parseRetryAfter(resp); retryAfter > 0 {
			// Use Retry-After as minimum, but cap at maxWait
			if retryAfter > maxWait {
				return maxWait
			}
			return retryAfter
		}

		// Fall back to exponential backoff
		return retryablehttp.DefaultBackoff(minWait, maxWait, attemptNum, resp)
	}

	return &Client{
		httpClient: retryClient,
		baseURL:    config.BaseURL,
		userAgent:  config.UserAgent,
		logger:     config.Logger,
	}, nil
}

// Close releases resources held by the client.
// It closes idle connections in the connection pool.
func (c *Client) Close() {
	if transport, ok := c.httpClient.HTTPClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// GetPrices queries the Azure Retail Prices API with the given filter.
// It automatically handles pagination and returns all matching price items.
// A safety limit of 1000 pages is enforced to prevent infinite loops.
// All errors include query context (region, SKU, service) for debugging.
func (c *Client) GetPrices(ctx context.Context, query PriceQuery) ([]PriceItem, error) {
	var allItems []PriceItem
	qctx := formatQueryContext(query)

	// Build initial URL with filter
	requestURL := c.baseURL
	if filter := buildFilterQuery(query); filter != "" {
		requestURL = fmt.Sprintf("%s?$filter=%s", c.baseURL, url.QueryEscape(filter))
	}

	// Paginate through all results with safety limit
	for page := 0; requestURL != "" && page < MaxPaginationPages; page++ {
		items, nextURL, err := c.fetchPage(ctx, requestURL)
		if err != nil {
			c.logError(query, requestURL, page, err)
			return nil, fmt.Errorf("%s page %d: %w", qctx, page, err)
		}
		allItems = append(allItems, items...)
		requestURL = nextURL
	}

	// Check if we hit the safety limit
	if requestURL != "" {
		err := fmt.Errorf("%s: %w", qctx, ErrPaginationLimitExceeded)
		c.logError(query, requestURL, -1, ErrPaginationLimitExceeded)
		return nil, err
	}

	// Check for empty results
	if len(allItems) == 0 {
		err := fmt.Errorf("%s: %w: no pricing data", qctx, ErrNotFound)
		c.logError(query, requestURL, -1, ErrNotFound)
		return nil, err
	}

	return allItems, nil
}

// fetchPage fetches a single page of results from the API.
func (c *Client) fetchPage(ctx context.Context, requestURL string) ([]PriceItem, string, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check for context cancellation
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	// Check for non-success status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		snippet := string(body)
		if len(snippet) > maxSnippetLen {
			snippet = snippet[:maxSnippetLen]
		}
		// Use appropriate sentinel errors for specific failure modes
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			return nil, "", fmt.Errorf("%w: status %d: %s", ErrRateLimited, resp.StatusCode, snippet)
		case http.StatusServiceUnavailable:
			return nil, "", fmt.Errorf("%w: status %d: %s", ErrServiceUnavailable, resp.StatusCode, snippet)
		case http.StatusNotFound:
			return nil, "", fmt.Errorf("%w: status %d: %s", ErrNotFound, resp.StatusCode, snippet)
		default:
			return nil, "", fmt.Errorf("%w: status %d: %s", ErrRequestFailed, resp.StatusCode, snippet)
		}
	}

	// Parse response with bounded read to prevent excessive memory usage
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if readErr != nil {
		return nil, "", fmt.Errorf("%w: reading response: %w", ErrInvalidResponse, readErr)
	}
	var priceResp PriceResponse
	if decodeErr := json.Unmarshal(bodyBytes, &priceResp); decodeErr != nil {
		snippet := string(bodyBytes)
		if len(snippet) > maxSnippetLen {
			snippet = snippet[:maxSnippetLen]
		}
		return nil, "", fmt.Errorf("%w: %w (response: %s)", ErrInvalidResponse, decodeErr, snippet)
	}

	return priceResp.Items, priceResp.NextPageLink, nil
}

// validateConfig validates the client configuration.
func validateConfig(config Config) error {
	if config.RetryMax < 0 {
		return fmt.Errorf("%w: RetryMax must be >= 0", ErrInvalidConfig)
	}
	if config.Timeout <= 0 {
		return fmt.Errorf("%w: Timeout must be > 0", ErrInvalidConfig)
	}
	if config.RetryWaitMin > config.RetryWaitMax {
		return fmt.Errorf("%w: RetryWaitMin must be <= RetryWaitMax", ErrInvalidConfig)
	}
	return nil
}

// formatQueryContext formats query fields for inclusion in error messages.
func formatQueryContext(query PriceQuery) string {
	var parts []string
	if query.ArmRegionName != "" {
		parts = append(parts, "region="+query.ArmRegionName)
	}
	if query.ArmSkuName != "" {
		parts = append(parts, "sku="+query.ArmSkuName)
	}
	if query.ServiceName != "" {
		parts = append(parts, "service="+query.ServiceName)
	}
	if len(parts) == 0 {
		return "query []"
	}
	return "query [" + strings.Join(parts, " ") + "]"
}

// errorCategory maps sentinel errors to category strings for structured logging.
func errorCategory(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrServiceUnavailable):
		return "service_unavailable"
	case errors.Is(err, ErrRequestFailed):
		return "request_failed"
	case errors.Is(err, ErrInvalidResponse):
		return "invalid_response"
	case errors.Is(err, ErrInvalidConfig):
		return "invalid_config"
	case errors.Is(err, ErrPaginationLimitExceeded):
		return "pagination_limit_exceeded"
	default:
		return "unknown"
	}
}

// logError logs a pricing query error with structured fields at the appropriate severity level.
func (c *Client) logError(query PriceQuery, requestURL string, page int, err error) {
	category := errorCategory(err)

	// Determine log level based on error type
	var event *zerolog.Event
	switch {
	case errors.Is(err, ErrNotFound):
		event = c.logger.Debug()
	case errors.Is(err, ErrRateLimited):
		event = c.logger.Warn()
	case errors.Is(err, ErrServiceUnavailable):
		event = c.logger.Error()
	case errors.Is(err, ErrInvalidResponse):
		event = c.logger.Error()
	case errors.Is(err, ErrPaginationLimitExceeded):
		event = c.logger.Error()
	case errors.Is(err, ErrRequestFailed):
		// Determine if it's 4xx or 5xx from the error message
		errStr := err.Error()
		switch {
		case strings.Contains(errStr, "status 4"):
			event = c.logger.Warn()
		case strings.Contains(errStr, "status 5"):
			event = c.logger.Error()
		default:
			// Network errors, context cancellation, etc.
			event = c.logger.Debug()
		}
	default:
		event = c.logger.Debug()
	}

	event = event.
		Str("region", query.ArmRegionName).
		Str("sku", query.ArmSkuName).
		Str("service", query.ServiceName).
		Str("url", requestURL).
		Str("error_category", category)

	if page >= 0 {
		event = event.Int("page", page)
	}

	event.Err(err).Msg("pricing query error")
}

// escapeODataString escapes a string for use in an OData filter.
// Single quotes are escaped by doubling them (OData standard).
func escapeODataString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// buildFilterQuery builds an OData filter query from a PriceQuery.
func buildFilterQuery(query PriceQuery) string {
	var filters []string

	if query.ArmRegionName != "" {
		filters = append(filters, fmt.Sprintf("armRegionName eq '%s'", escapeODataString(query.ArmRegionName)))
	}
	if query.ArmSkuName != "" {
		filters = append(filters, fmt.Sprintf("armSkuName eq '%s'", escapeODataString(query.ArmSkuName)))
	}
	if query.ServiceName != "" {
		filters = append(filters, fmt.Sprintf("serviceName eq '%s'", escapeODataString(query.ServiceName)))
	}
	if query.ProductName != "" {
		filters = append(filters, fmt.Sprintf("productName eq '%s'", escapeODataString(query.ProductName)))
	}
	if query.CurrencyCode != "" {
		filters = append(filters, fmt.Sprintf("currencyCode eq '%s'", escapeODataString(query.CurrencyCode)))
	}

	return strings.Join(filters, " and ")
}
