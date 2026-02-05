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

// PriceItem represents a single price entry from the Azure Retail Prices API.
// Each item corresponds to a specific SKU/meter combination in a region.
//
// Field names match Azure API JSON field names using json struct tags.
// All fields use Go types appropriate for their JSON counterparts.
//
// Example JSON:
//
//	{
//	  "currencyCode": "USD",
//	  "retailPrice": 0.0104,
//	  "armRegionName": "eastus",
//	  "skuName": "B1s",
//	  "serviceName": "Virtual Machines",
//	  "type": "Consumption"
//	}
type PriceItem struct {
	// ArmRegionName is the Azure Resource Manager region identifier (e.g., "eastus").
	// This is the programmatic region name used in ARM templates.
	ArmRegionName string `json:"armRegionName"`

	// ArmSkuName is the SKU identifier used in Azure Resource Manager (e.g., "Standard_B1s").
	// This is the value used when provisioning resources via ARM.
	ArmSkuName string `json:"armSkuName"`

	// AvailabilityID is the availability identifier.
	// Often empty or null in API responses.
	AvailabilityID string `json:"availabilityId,omitempty"`

	// CurrencyCode is the ISO 4217 currency code for the price (e.g., "USD").
	CurrencyCode string `json:"currencyCode"`

	// EffectiveStartDate is when this price became effective (ISO 8601 format).
	// Example: "2020-08-01T00:00:00Z"
	EffectiveStartDate string `json:"effectiveStartDate"`

	// IsPrimaryMeterRegion indicates whether this is the primary region for the meter.
	// When true, this region is used for billing purposes.
	IsPrimaryMeterRegion bool `json:"isPrimaryMeterRegion"`

	// Location is the human-readable Azure datacenter location (e.g., "US East").
	// This is the display name shown in the Azure portal.
	Location string `json:"location"`

	// MeterID is the unique identifier for the billing meter (UUID format).
	// This is used in Azure cost management and billing.
	MeterID string `json:"meterId"`

	// MeterName is the human-readable name for the meter (e.g., "B1s").
	MeterName string `json:"meterName"`

	// ProductID is the unique identifier for the product.
	ProductID string `json:"productId"`

	// ProductName is the full product name (e.g., "Virtual Machines BS Series").
	ProductName string `json:"productName"`

	// ReservationTerm is the commitment period for reservation pricing.
	// Only present when Type is "Reservation".
	// Values: "1 Year", "3 Years"
	ReservationTerm string `json:"reservationTerm,omitempty"`

	// RetailPrice is the Microsoft retail price without any discount.
	// This is the list price for the resource.
	RetailPrice float64 `json:"retailPrice"`

	// ServiceFamily is the service category (e.g., "Compute", "Storage", "Networking").
	ServiceFamily string `json:"serviceFamily"`

	// ServiceID is the unique identifier for the Azure service.
	ServiceID string `json:"serviceId"`

	// ServiceName is the name of the Azure service (e.g., "Virtual Machines").
	ServiceName string `json:"serviceName"`

	// SkuID is the unique identifier for this SKU (e.g., "DZH318Z0BQPS/00TG").
	SkuID string `json:"skuId"`

	// SkuName is the display name for the SKU (e.g., "B1s").
	// This is a human-friendly version of the SKU identifier.
	SkuName string `json:"skuName"`

	// TierMinimumUnits is the minimum units of consumption for this price tier.
	// Value is 0 for non-tiered pricing.
	TierMinimumUnits float64 `json:"tierMinimumUnits"`

	// Type indicates the pricing model for this item.
	// Values: "Consumption", "Reservation", "DevTestConsumption"
	Type string `json:"type"`

	// UnitOfMeasure describes the billing unit (e.g., "1 Hour", "1 GB", "10K Transactions").
	UnitOfMeasure string `json:"unitOfMeasure"`

	// UnitPrice is the price per unit of measure.
	// For most resources, this equals RetailPrice.
	UnitPrice float64 `json:"unitPrice"`
}

// PriceResponse represents the envelope returned by the Azure Retail Prices API.
// It contains a paginated list of price items along with metadata.
//
// The API returns up to 1000 items per request. When NextPageLink is non-empty,
// additional pages are available.
//
// Example response:
//
//	{
//	  "BillingCurrency": "USD",
//	  "CustomerEntityId": "Default",
//	  "CustomerEntityType": "Retail",
//	  "Items": [...],
//	  "NextPageLink": "https://prices.azure.com/api/retail/prices?$skip=1000",
//	  "Count": 1000
//	}
type PriceResponse struct {
	// BillingCurrency is the currency code for all prices in the response.
	// Typically "USD" unless a different currency was requested.
	BillingCurrency string `json:"BillingCurrency"`

	// CustomerEntityID identifies the customer entity for pricing.
	// Default value is "Default" for retail pricing.
	CustomerEntityID string `json:"CustomerEntityId"`

	// CustomerEntityType indicates the type of pricing returned.
	// Value is "Retail" for the public retail prices API.
	CustomerEntityType string `json:"CustomerEntityType"`

	// Items contains the price entries for this page of results.
	// Maximum 1000 items per response.
	Items []PriceItem `json:"Items"`

	// NextPageLink is the URL to fetch the next page of results.
	// Empty string indicates no more pages are available.
	NextPageLink string `json:"NextPageLink"`

	// Count is the number of items in this page of results.
	Count int `json:"Count"`
}
