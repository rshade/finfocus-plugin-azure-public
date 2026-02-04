// Package azureclient provides an HTTP client for the Azure Retail Prices API.
//
// The client automatically handles retry logic for transient failures (HTTP 429, 503),
// respects Retry-After headers, and provides structured logging via zerolog.
package azureclient

import (
	"time"

	"github.com/rs/zerolog"
)

// Configuration defaults.
const (
	// DefaultBaseURL is the Azure Retail Prices API endpoint.
	DefaultBaseURL = "https://prices.azure.com/api/retail/prices"

	// DefaultRetryMax is the default number of retry attempts.
	DefaultRetryMax = 3

	// DefaultRetryWaitMinSeconds is the default minimum backoff in seconds.
	DefaultRetryWaitMinSeconds = 1

	// DefaultRetryWaitMaxSeconds is the default maximum backoff in seconds.
	DefaultRetryWaitMaxSeconds = 30

	// DefaultTimeoutSeconds is the default request timeout in seconds.
	DefaultTimeoutSeconds = 60
)

// Config holds configuration for creating a Client.
type Config struct {
	// BaseURL is the Azure Retail Prices API base URL.
	// Default: "https://prices.azure.com/api/retail/prices"
	BaseURL string

	// RetryMax is the maximum number of retry attempts.
	// Default: 3 (total of 4 attempts including initial request)
	RetryMax int

	// RetryWaitMin is the minimum duration to wait before retrying.
	// Default: 1 second
	RetryWaitMin time.Duration

	// RetryWaitMax is the maximum duration to wait before retrying.
	// Default: 30 seconds
	RetryWaitMax time.Duration

	// Timeout is the timeout for individual HTTP requests.
	// Default: 60 seconds
	Timeout time.Duration

	// Logger is the zerolog logger for retry events.
	// Default: zerolog.Nop() (no logging)
	Logger zerolog.Logger

	// UserAgent is the User-Agent header value.
	// Default: "finfocus-plugin-azure-public"
	UserAgent string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:      DefaultBaseURL,
		RetryMax:     DefaultRetryMax,
		RetryWaitMin: DefaultRetryWaitMinSeconds * time.Second,
		RetryWaitMax: DefaultRetryWaitMaxSeconds * time.Second,
		Timeout:      DefaultTimeoutSeconds * time.Second,
		Logger:       zerolog.Nop(),
		UserAgent:    "finfocus-plugin-azure-public",
	}
}

// PriceQuery contains filter parameters for querying prices.
type PriceQuery struct {
	// ArmRegionName filters by Azure region (e.g., "eastus").
	ArmRegionName string

	// ArmSkuName filters by SKU name (e.g., "Standard_B1s").
	ArmSkuName string

	// ServiceName filters by service (e.g., "Virtual Machines").
	ServiceName string

	// ProductName filters by product name.
	ProductName string

	// CurrencyCode filters by currency (default: "USD").
	CurrencyCode string
}

// PriceItem represents a single price entry from the Azure API.
// Field names match Azure API JSON field names (using json tags).
type PriceItem struct {
	ArmRegionName        string  `json:"armRegionName"`
	ArmSkuName           string  `json:"armSkuName"`
	CurrencyCode         string  `json:"currencyCode"`
	EffectiveStartDate   string  `json:"effectiveStartDate"`
	IsPrimaryMeterRegion bool    `json:"isPrimaryMeterRegion"`
	MeterID              string  `json:"meterId"`
	MeterName            string  `json:"meterName"`
	ProductID            string  `json:"productId"`
	ProductName          string  `json:"productName"`
	RetailPrice          float64 `json:"retailPrice"`
	ServiceFamily        string  `json:"serviceFamily"`
	ServiceID            string  `json:"serviceId"`
	ServiceName          string  `json:"serviceName"`
	SkuID                string  `json:"skuId"`
	SkuName              string  `json:"skuName"`
	TierMinimumUnits     float64 `json:"tierMinimumUnits"`
	Type                 string  `json:"type"`
	UnitOfMeasure        string  `json:"unitOfMeasure"`
	UnitPrice            float64 `json:"unitPrice"`
}

// priceResponse is the internal response envelope from Azure API.
type priceResponse struct {
	BillingCurrency    string      `json:"BillingCurrency"`
	CustomerEntityID   string      `json:"CustomerEntityId"`
	CustomerEntityType string      `json:"CustomerEntityType"`
	Items              []PriceItem `json:"Items"`
	NextPageLink       string      `json:"NextPageLink"`
	Count              int         `json:"Count"`
}
