package client

// This file contains the cloud provider client implementation.
// Implement API clients for your specific cloud provider here.

import (
	"context"
	"fmt"
)

// Client represents a client for your cloud provider's APIs.
type Client struct {
	// Add your cloud provider client configuration here
	// Examples:
	// - AWS SDK client
	// - Azure SDK client  
	// - GCP SDK client
	// - Custom API client
}

// Config holds configuration for the cloud provider client.
type Config struct {
	// Add configuration fields specific to your provider
	// Examples:
	// APIKey      string
	// Region      string
	// Credentials *Credentials
}

// NewClient creates a new cloud provider client.
func NewClient(config Config) (*Client, error) {
	// [TEMPLATE] Implementation Required: Client Initialization
	// Initialize your cloud provider's SDK client using the provided config.
	//
	// Example:
	// return &Client{
	//     api: someprovider.NewClient(config.APIKey),
	// }, nil
	
	return &Client{}, nil
}

// GetResourceCost retrieves actual cost data for a specific resource.
func (c *Client) GetResourceCost(ctx context.Context, resourceID string, startTime, endTime int64) (float64, error) {
	// [TEMPLATE] Implementation Required: Actual Cost Retrieval
	// This method should call the appropriate billing/cost management API
	// to get the actual cost for the given resource and time range.
	//
	// Examples:
	// - AWS Cost Explorer API
	// - Azure Cost Management API
	// - GCP Cloud Billing API
	
	return 0.0, fmt.Errorf("not implemented: GetResourceCost for resource %s", resourceID)
}

// ValidateCredentials checks if the client credentials are valid.
func (c *Client) ValidateCredentials(ctx context.Context) error {
	// [TEMPLATE] Implementation Required: Credential Validation
	// Make a lightweight API call to verify that the credentials are valid
	// and have the necessary permissions.
	
	return fmt.Errorf("not implemented: ValidateCredentials")
}

// GetSupportedRegions returns the list of supported regions.
func (c *Client) GetSupportedRegions(ctx context.Context) ([]string, error) {
	// [TEMPLATE] Implementation Required: Region Discovery
	// Return the list of regions supported by your cloud provider.
	// This can be hardcoded or fetched dynamically.
	
	return []string{"us-east-1", "us-west-2"}, nil
}
