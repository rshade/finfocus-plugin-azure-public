package pricing

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSaveLoadPricingData_RoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "pricing.json")
	input := []Data{
		{
			Provider:     "azure",
			Region:       "eastus",
			ResourceType: "azure:vm:Instance",
			Pricing: map[string]float64{
				"Standard_B1s": 0.0123,
				"Standard_B2s": 0.0456,
			},
		},
	}

	if err := SavePricingData(path, input); err != nil {
		t.Fatalf("SavePricingData() unexpected error: %v", err)
	}

	got, err := LoadPricingData(path)
	if err != nil {
		t.Fatalf("LoadPricingData() unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, input) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, input)
	}
}

func TestLoadPricingData_FileReadError(t *testing.T) {
	t.Parallel()

	_, err := LoadPricingData(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected file read error")
	}

	if !strings.Contains(err.Error(), "reading pricing data") {
		t.Fatalf("expected reading pricing data context, got: %v", err)
	}
}

func TestLoadPricingData_InvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write invalid json fixture: %v", err)
	}

	_, err := LoadPricingData(path)
	if err == nil {
		t.Fatal("expected parse error")
	}

	if !strings.Contains(err.Error(), "parsing pricing data") {
		t.Fatalf("expected parsing pricing data context, got: %v", err)
	}
}

func TestSavePricingData_MarshalError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "pricing.json")
	input := []Data{
		{
			Provider:     "azure",
			Region:       "eastus",
			ResourceType: "azure:vm:Instance",
			Pricing: map[string]float64{
				"bad": math.NaN(),
			},
		},
	}

	err := SavePricingData(path, input)
	if err == nil {
		t.Fatal("expected marshal error")
	}

	if !strings.Contains(err.Error(), "marshaling pricing data") {
		t.Fatalf("expected marshaling pricing data context, got: %v", err)
	}
}

func TestSavePricingData_MkdirError(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	blockingFile := filepath.Join(base, "not-a-dir")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	path := filepath.Join(blockingFile, "child", "pricing.json")
	err := SavePricingData(path, CreateExamplePricingData())
	if err == nil {
		t.Fatal("expected mkdir error")
	}

	if !strings.Contains(err.Error(), "creating directory") {
		t.Fatalf("expected creating directory context, got: %v", err)
	}
}

func TestSavePricingData_WriteError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := SavePricingData(dir, CreateExamplePricingData())
	if err == nil {
		t.Fatal("expected write error")
	}

	if !strings.Contains(err.Error(), "writing pricing data") {
		t.Fatalf("expected writing pricing data context, got: %v", err)
	}
}

func TestCreateExamplePricingData(t *testing.T) {
	t.Parallel()

	got := CreateExamplePricingData()
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}

	entry := got[0]
	if entry.Provider != "aws" {
		t.Fatalf("expected provider aws, got %q", entry.Provider)
	}
	if entry.Region != "us-east-1" {
		t.Fatalf("expected region us-east-1, got %q", entry.Region)
	}
	if entry.ResourceType != "aws:ec2:Instance" {
		t.Fatalf("expected aws resource type, got %q", entry.ResourceType)
	}

	wantPrices := map[string]float64{
		"t3.micro":  priceT3Micro,
		"t3.small":  priceT3Small,
		"t3.medium": priceT3Medium,
		"t3.large":  priceT3Large,
		"t3.xlarge": priceT3XLarge,
	}
	if !reflect.DeepEqual(entry.Pricing, wantPrices) {
		t.Fatalf("unexpected pricing map: got %#v want %#v", entry.Pricing, wantPrices)
	}
}
