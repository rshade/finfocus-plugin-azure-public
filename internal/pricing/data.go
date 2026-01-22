package pricing

// This file contains pricing calculation utilities and data structures.
// Implement your cloud provider pricing logic here.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PricingData represents pricing information for resources.
type PricingData struct {
	Provider     string             `json:"provider"`
	Region       string             `json:"region"`
	ResourceType string             `json:"resource_type"`
	Pricing      map[string]float64 `json:"pricing"`
}

// LoadPricingData loads pricing data from a JSON file.
func LoadPricingData(path string) ([]PricingData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pricing data: %w", err)
	}

	var pricing []PricingData
	if err := json.Unmarshal(data, &pricing); err != nil {
		return nil, fmt.Errorf("parsing pricing data: %w", err)
	}

	return pricing, nil
}

// SavePricingData saves pricing data to a JSON file.
func SavePricingData(path string, data []PricingData) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling pricing data: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("writing pricing data: %w", err)
	}

	return nil
}

// Example pricing data structure - customize for your provider
func CreateExamplePricingData() []PricingData {
	return []PricingData{
		{
			Provider:     "aws",
			Region:       "us-east-1",
			ResourceType: "aws:ec2:Instance",
			Pricing: map[string]float64{
				"t3.micro":  0.0104,
				"t3.small":  0.0208,
				"t3.medium": 0.0416,
				"t3.large":  0.0832,
				"t3.xlarge": 0.1664,
			},
		},
	}
}
