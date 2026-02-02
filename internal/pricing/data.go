package pricing

// This file contains pricing calculation utilities and data structures.
// Implement your cloud provider pricing logic here.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Example instance type pricing (USD/hour).
const (
	priceT3Micro  = 0.0104
	priceT3Small  = 0.0208
	priceT3Medium = 0.0416
	priceT3Large  = 0.0832
	priceT3XLarge = 0.1664
)

// Data represents pricing information for resources.
type Data struct {
	Provider     string             `json:"provider"`
	Region       string             `json:"region"`
	ResourceType string             `json:"resource_type"`
	Pricing      map[string]float64 `json:"pricing"`
}

// LoadPricingData loads pricing data from a JSON file.
func LoadPricingData(path string) ([]Data, error) {
	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pricing data: %w", err)
	}

	var pricing []Data
	if unmarshalErr := json.Unmarshal(fileData, &pricing); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing pricing data: %w", unmarshalErr)
	}

	return pricing, nil
}

// SavePricingData saves pricing data to a JSON file.
func SavePricingData(path string, data []Data) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling pricing data: %w", err)
	}

	dir := filepath.Dir(path)
	if mkdirErr := os.MkdirAll(dir, 0o750); mkdirErr != nil {
		return fmt.Errorf("creating directory: %w", mkdirErr)
	}

	if writeErr := os.WriteFile(path, jsonData, 0o600); writeErr != nil {
		return fmt.Errorf("writing pricing data: %w", writeErr)
	}

	return nil
}

// CreateExamplePricingData creates example pricing data for testing.
func CreateExamplePricingData() []Data {
	return []Data{
		{
			Provider:     "aws",
			Region:       "us-east-1",
			ResourceType: "aws:ec2:Instance",
			Pricing: map[string]float64{
				"t3.micro":  priceT3Micro,
				"t3.small":  priceT3Small,
				"t3.medium": priceT3Medium,
				"t3.large":  priceT3Large,
				"t3.xlarge": priceT3XLarge,
			},
		},
	}
}
