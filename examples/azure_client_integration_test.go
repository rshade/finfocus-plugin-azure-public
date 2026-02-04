//go:build integration

// Package examples contains integration tests for the Azure client.
package examples

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-plugin-azure-public/internal/azureclient"
)

// TestAzureClient_LiveAPI_StandardB1s queries the live Azure Retail Prices API
// for Standard_B1s VM pricing in eastus region.
//
// Run with: go test -tags=integration ./examples/...
func TestAzureClient_LiveAPI_StandardB1s(t *testing.T) {
	// Create a logger for debugging
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	config := azureclient.DefaultConfig()
	config.Logger = logger

	client, err := azureclient.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := azureclient.PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
		CurrencyCode:  "USD",
	}

	prices, err := client.GetPrices(ctx, query)
	if err != nil {
		t.Fatalf("failed to get prices: %v", err)
	}

	if len(prices) == 0 {
		t.Fatal("expected at least one price item for Standard_B1s in eastus")
	}

	t.Logf("Found %d price items for Standard_B1s in eastus", len(prices))

	// Verify we got reasonable data
	for i, price := range prices {
		if price.ArmRegionName != "eastus" {
			t.Errorf("price[%d]: expected region eastus, got %s", i, price.ArmRegionName)
		}
		if price.ArmSkuName != "Standard_B1s" {
			t.Errorf("price[%d]: expected SKU Standard_B1s, got %s", i, price.ArmSkuName)
		}
		if price.RetailPrice <= 0 {
			t.Errorf("price[%d]: expected positive retail price, got %f", i, price.RetailPrice)
		}
		t.Logf("  - %s: $%.4f %s/%s", price.MeterName, price.RetailPrice, price.CurrencyCode, price.UnitOfMeasure)
	}
}

// TestAzureClient_LiveAPI_VirtualMachines queries VM pricing across regions.
func TestAzureClient_LiveAPI_VirtualMachines(t *testing.T) {
	config := azureclient.DefaultConfig()
	client, err := azureclient.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	query := azureclient.PriceQuery{
		ServiceName:  "Virtual Machines",
		ArmSkuName:   "Standard_D2s_v3",
		CurrencyCode: "USD",
	}

	prices, err := client.GetPrices(ctx, query)
	if err != nil {
		t.Fatalf("failed to get prices: %v", err)
	}

	if len(prices) == 0 {
		t.Fatal("expected at least one price item for Standard_D2s_v3")
	}

	t.Logf("Found %d price items for Standard_D2s_v3 across all regions", len(prices))

	// Count unique regions
	regions := make(map[string]bool)
	for _, price := range prices {
		regions[price.ArmRegionName] = true
	}
	t.Logf("Available in %d regions", len(regions))
}
