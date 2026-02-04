package azureclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	// maxPaginationPages is a safety limit to prevent infinite pagination loops.
	maxPaginationPages = 1000

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
	}, nil
}

// Close releases resources held by the client.
// It closes idle connections in the connection pool.
func (c *Client) Close() {
	if transport, ok := c.httpClient.HTTPClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// ErrPaginationLimitExceeded is returned when pagination exceeds the safety limit.
var ErrPaginationLimitExceeded = fmt.Errorf("pagination limit exceeded (%d pages)", maxPaginationPages)

// GetPrices queries the Azure Retail Prices API with the given filter.
// It automatically handles pagination and returns all matching price items.
// A safety limit of 1000 pages is enforced to prevent infinite loops.
func (c *Client) GetPrices(ctx context.Context, query PriceQuery) ([]PriceItem, error) {
	var allItems []PriceItem

	// Build initial URL with filter
	requestURL := c.baseURL
	if filter := buildFilterQuery(query); filter != "" {
		requestURL = fmt.Sprintf("%s?$filter=%s", c.baseURL, url.QueryEscape(filter))
	}

	// Paginate through all results with safety limit
	for page := 0; requestURL != "" && page < maxPaginationPages; page++ {
		items, nextURL, err := c.fetchPage(ctx, requestURL)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)
		requestURL = nextURL
	}

	// Check if we hit the safety limit
	if requestURL != "" {
		return nil, ErrPaginationLimitExceeded
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
		body, _ := io.ReadAll(resp.Body)
		// Use appropriate sentinel errors for specific failure modes
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			return nil, "", fmt.Errorf("%w: status %d: %s", ErrRateLimited, resp.StatusCode, string(body))
		case http.StatusServiceUnavailable:
			return nil, "", fmt.Errorf("%w: status %d: %s", ErrServiceUnavailable, resp.StatusCode, string(body))
		default:
			return nil, "", fmt.Errorf("%w: status %d: %s", ErrRequestFailed, resp.StatusCode, string(body))
		}
	}

	// Parse response
	var priceResp PriceResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&priceResp); decodeErr != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalidResponse, decodeErr)
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
