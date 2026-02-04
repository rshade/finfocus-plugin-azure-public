package azureclient

import (
	"encoding/json"
	"strings"
	"testing"
)

// Sample JSON from Azure Retail Prices API for testing.
// See research.md for field documentation.

const samplePriceItemJSON = `{
  "currencyCode": "USD",
  "tierMinimumUnits": 0.0,
  "retailPrice": 0.0104,
  "unitPrice": 0.0104,
  "armRegionName": "eastus",
  "location": "US East",
  "effectiveStartDate": "2020-08-01T00:00:00Z",
  "meterId": "000a794b-bdb0-58be-a0cd-0c3a0f222923",
  "meterName": "B1s",
  "productId": "DZH318Z0BQPS",
  "skuId": "DZH318Z0BQPS/00TG",
  "availabilityId": null,
  "productName": "Virtual Machines BS Series",
  "skuName": "B1s",
  "serviceName": "Virtual Machines",
  "serviceId": "DZH313Z7MMC8",
  "serviceFamily": "Compute",
  "unitOfMeasure": "1 Hour",
  "type": "Consumption",
  "isPrimaryMeterRegion": true,
  "armSkuName": "Standard_B1s"
}`

const sampleReservationPriceItemJSON = `{
  "currencyCode": "USD",
  "tierMinimumUnits": 0.0,
  "retailPrice": 3285.0,
  "unitPrice": 3285.0,
  "armRegionName": "eastus",
  "location": "US East",
  "effectiveStartDate": "2023-01-01T00:00:00Z",
  "meterId": "abc123-reservation-meter",
  "meterName": "D2s v3 Reserved",
  "productId": "DZH318Z0BQPS",
  "skuId": "DZH318Z0BQPS/RESV",
  "productName": "Virtual Machines Dv3 Series",
  "skuName": "D2s v3",
  "serviceName": "Virtual Machines",
  "serviceId": "DZH313Z7MMC8",
  "serviceFamily": "Compute",
  "unitOfMeasure": "1 Year",
  "type": "Reservation",
  "reservationTerm": "1 Year",
  "isPrimaryMeterRegion": true,
  "armSkuName": "Standard_D2s_v3"
}`

const samplePriceResponseJSON = `{
  "BillingCurrency": "USD",
  "CustomerEntityId": "Default",
  "CustomerEntityType": "Retail",
  "Items": [
    {
      "currencyCode": "USD",
      "retailPrice": 0.0104,
      "armRegionName": "eastus",
      "skuName": "B1s",
      "type": "Consumption"
    }
  ],
  "NextPageLink": "https://prices.azure.com/api/retail/prices?$skip=1000",
  "Count": 1
}`

func TestPriceResponse_UnmarshalJSON(t *testing.T) {
	var resp PriceResponse
	if err := json.Unmarshal([]byte(samplePriceResponseJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal PriceResponse: %v", err)
	}

	// Verify envelope fields
	if resp.BillingCurrency != "USD" {
		t.Errorf("expected BillingCurrency=USD, got %s", resp.BillingCurrency)
	}
	if resp.CustomerEntityID != "Default" {
		t.Errorf("expected CustomerEntityID=Default, got %s", resp.CustomerEntityID)
	}
	if resp.CustomerEntityType != "Retail" {
		t.Errorf("expected CustomerEntityType=Retail, got %s", resp.CustomerEntityType)
	}
	if resp.NextPageLink != "https://prices.azure.com/api/retail/prices?$skip=1000" {
		t.Errorf("unexpected NextPageLink: %s", resp.NextPageLink)
	}
	if resp.Count != 1 {
		t.Errorf("expected Count=1, got %d", resp.Count)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}

	// Verify item in envelope
	item := resp.Items[0]
	if item.CurrencyCode != "USD" {
		t.Errorf("expected item CurrencyCode=USD, got %s", item.CurrencyCode)
	}
	if item.RetailPrice != 0.0104 {
		t.Errorf("expected item RetailPrice=0.0104, got %f", item.RetailPrice)
	}
}

func TestPriceItem_UnmarshalJSON_AllFields(t *testing.T) {
	var item PriceItem
	if err := json.Unmarshal([]byte(samplePriceItemJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal PriceItem: %v", err)
	}

	// Pricing fields
	if item.RetailPrice != 0.0104 {
		t.Errorf("RetailPrice: expected 0.0104, got %f", item.RetailPrice)
	}
	if item.UnitPrice != 0.0104 {
		t.Errorf("UnitPrice: expected 0.0104, got %f", item.UnitPrice)
	}
	if item.TierMinimumUnits != 0.0 {
		t.Errorf("TierMinimumUnits: expected 0.0, got %f", item.TierMinimumUnits)
	}
	if item.CurrencyCode != "USD" {
		t.Errorf("CurrencyCode: expected USD, got %s", item.CurrencyCode)
	}

	// Resource fields
	if item.ArmRegionName != "eastus" {
		t.Errorf("ArmRegionName: expected eastus, got %s", item.ArmRegionName)
	}
	if item.Location != "US East" {
		t.Errorf("Location: expected 'US East', got %s", item.Location)
	}
	if item.ArmSkuName != "Standard_B1s" {
		t.Errorf("ArmSkuName: expected Standard_B1s, got %s", item.ArmSkuName)
	}
	if item.SkuName != "B1s" {
		t.Errorf("SkuName: expected B1s, got %s", item.SkuName)
	}
	if item.SkuID != "DZH318Z0BQPS/00TG" {
		t.Errorf("SkuID: expected DZH318Z0BQPS/00TG, got %s", item.SkuID)
	}

	// Service fields
	if item.ServiceName != "Virtual Machines" {
		t.Errorf("ServiceName: expected 'Virtual Machines', got %s", item.ServiceName)
	}
	if item.ServiceID != "DZH313Z7MMC8" {
		t.Errorf("ServiceID: expected DZH313Z7MMC8, got %s", item.ServiceID)
	}
	if item.ServiceFamily != "Compute" {
		t.Errorf("ServiceFamily: expected Compute, got %s", item.ServiceFamily)
	}
	if item.ProductName != "Virtual Machines BS Series" {
		t.Errorf("ProductName: expected 'Virtual Machines BS Series', got %s", item.ProductName)
	}
	if item.ProductID != "DZH318Z0BQPS" {
		t.Errorf("ProductID: expected DZH318Z0BQPS, got %s", item.ProductID)
	}

	// Meter fields
	if item.MeterID != "000a794b-bdb0-58be-a0cd-0c3a0f222923" {
		t.Errorf("MeterID: expected 000a794b-bdb0-58be-a0cd-0c3a0f222923, got %s", item.MeterID)
	}
	if item.MeterName != "B1s" {
		t.Errorf("MeterName: expected B1s, got %s", item.MeterName)
	}
	if item.UnitOfMeasure != "1 Hour" {
		t.Errorf("UnitOfMeasure: expected '1 Hour', got %s", item.UnitOfMeasure)
	}
	if !item.IsPrimaryMeterRegion {
		t.Error("IsPrimaryMeterRegion: expected true")
	}

	// Temporal fields
	if item.EffectiveStartDate != "2020-08-01T00:00:00Z" {
		t.Errorf("EffectiveStartDate: expected 2020-08-01T00:00:00Z, got %s", item.EffectiveStartDate)
	}

	// Type fields
	if item.Type != "Consumption" {
		t.Errorf("Type: expected Consumption, got %s", item.Type)
	}

	// Optional fields - availabilityId is null in sample, should be empty string
	if item.AvailabilityID != "" {
		t.Errorf("AvailabilityID: expected empty string for null, got %s", item.AvailabilityID)
	}
	// ReservationTerm should be empty for Consumption type
	if item.ReservationTerm != "" {
		t.Errorf("ReservationTerm: expected empty for Consumption, got %s", item.ReservationTerm)
	}
}

func TestPriceItem_UnmarshalJSON_ReservationTerm(t *testing.T) {
	var item PriceItem
	if err := json.Unmarshal([]byte(sampleReservationPriceItemJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal reservation PriceItem: %v", err)
	}

	if item.Type != "Reservation" {
		t.Errorf("Type: expected Reservation, got %s", item.Type)
	}
	if item.ReservationTerm != "1 Year" {
		t.Errorf("ReservationTerm: expected '1 Year', got %s", item.ReservationTerm)
	}
	if item.RetailPrice != 3285.0 {
		t.Errorf("RetailPrice: expected 3285.0, got %f", item.RetailPrice)
	}
}

func TestPriceItem_UnmarshalJSON_MissingOptionalFields(t *testing.T) {
	// Test that missing optional fields don't cause errors.
	// Go's encoding/json handles:
	// - Missing fields: left at zero value
	// - null values: left at zero value for strings
	// - Unknown fields: ignored by default

	minimalJSON := `{
		"retailPrice": 0.01,
		"currencyCode": "USD",
		"armRegionName": "westus"
	}`

	var item PriceItem
	if err := json.Unmarshal([]byte(minimalJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal minimal PriceItem: %v", err)
	}

	// Required fields should be populated
	if item.RetailPrice != 0.01 {
		t.Errorf("RetailPrice: expected 0.01, got %f", item.RetailPrice)
	}
	if item.CurrencyCode != "USD" {
		t.Errorf("CurrencyCode: expected USD, got %s", item.CurrencyCode)
	}
	if item.ArmRegionName != "westus" {
		t.Errorf("ArmRegionName: expected westus, got %s", item.ArmRegionName)
	}

	// Optional/missing fields should be zero values
	if item.Location != "" {
		t.Errorf("Location: expected empty string for missing field, got %s", item.Location)
	}
	if item.ReservationTerm != "" {
		t.Errorf("ReservationTerm: expected empty string for missing field, got %s", item.ReservationTerm)
	}
	if item.AvailabilityID != "" {
		t.Errorf("AvailabilityID: expected empty string for missing field, got %s", item.AvailabilityID)
	}
	if item.UnitPrice != 0 {
		t.Errorf("UnitPrice: expected 0 for missing field, got %f", item.UnitPrice)
	}
	if item.IsPrimaryMeterRegion {
		t.Error("IsPrimaryMeterRegion: expected false for missing field")
	}
}

func TestPriceItem_UnmarshalJSON_NullValues(t *testing.T) {
	// Test explicit null values
	jsonWithNulls := `{
		"retailPrice": 0.01,
		"currencyCode": "USD",
		"armRegionName": "eastus",
		"location": null,
		"reservationTerm": null,
		"availabilityId": null
	}`

	var item PriceItem
	if err := json.Unmarshal([]byte(jsonWithNulls), &item); err != nil {
		t.Fatalf("failed to unmarshal PriceItem with nulls: %v", err)
	}

	// null string values become empty strings in Go
	if item.Location != "" {
		t.Errorf("Location: expected empty string for null, got %s", item.Location)
	}
	if item.ReservationTerm != "" {
		t.Errorf("ReservationTerm: expected empty string for null, got %s", item.ReservationTerm)
	}
	if item.AvailabilityID != "" {
		t.Errorf("AvailabilityID: expected empty string for null, got %s", item.AvailabilityID)
	}
}

func TestPriceItem_TypeSafety(t *testing.T) {
	// Verify that pricing fields are correct Go types for calculations.
	// This is a compile-time verification encoded as a test.

	item := PriceItem{
		RetailPrice:      0.0104,
		UnitPrice:        0.0104,
		TierMinimumUnits: 0.0,
	}

	// Verify float64 types allow arithmetic
	var hourlyRate float64 = item.RetailPrice
	var hoursPerMonth float64 = 730.0
	monthlyCost := hourlyRate * hoursPerMonth
	if monthlyCost < 7.0 || monthlyCost > 8.0 {
		t.Errorf("float64 arithmetic failed: %f * %f = %f", hourlyRate, hoursPerMonth, monthlyCost)
	}

	// Verify string types
	var _ string = item.MeterID
	var _ string = item.ProductID
	var _ string = item.SkuID
	var _ string = item.ServiceID

	// Verify bool type
	var _ bool = item.IsPrimaryMeterRegion
}

func TestPriceItem_MarshalJSON(t *testing.T) {
	item := PriceItem{
		RetailPrice:          0.0104,
		UnitPrice:            0.0104,
		TierMinimumUnits:     0.0,
		CurrencyCode:         "USD",
		ArmRegionName:        "eastus",
		Location:             "US East",
		ArmSkuName:           "Standard_B1s",
		SkuName:              "B1s",
		SkuID:                "DZH318Z0BQPS/00TG",
		ServiceName:          "Virtual Machines",
		ServiceID:            "DZH313Z7MMC8",
		ServiceFamily:        "Compute",
		ProductName:          "Virtual Machines BS Series",
		ProductID:            "DZH318Z0BQPS",
		MeterID:              "000a794b-bdb0-58be-a0cd-0c3a0f222923",
		MeterName:            "B1s",
		UnitOfMeasure:        "1 Hour",
		IsPrimaryMeterRegion: true,
		EffectiveStartDate:   "2020-08-01T00:00:00Z",
		Type:                 "Consumption",
	}

	jsonBytes, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("failed to marshal PriceItem: %v", err)
	}

	// Verify JSON uses camelCase field names (Azure API format)
	jsonStr := string(jsonBytes)
	expectedFields := []string{
		`"retailPrice":`,
		`"unitPrice":`,
		`"currencyCode":`,
		`"armRegionName":`,
		`"location":`,
		`"armSkuName":`,
		`"serviceName":`,
		`"isPrimaryMeterRegion":`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("expected JSON to contain %s", field)
		}
	}
}

func TestPriceItem_MarshalJSON_OmitsEmptyOptional(t *testing.T) {
	item := PriceItem{
		RetailPrice:   0.01,
		CurrencyCode:  "USD",
		ArmRegionName: "eastus",
		// ReservationTerm and AvailabilityID are empty
	}

	jsonBytes, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("failed to marshal PriceItem: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Fields with omitempty should not appear when empty
	if strings.Contains(jsonStr, `"reservationTerm"`) {
		t.Error("expected reservationTerm to be omitted when empty")
	}
	if strings.Contains(jsonStr, `"availabilityId"`) {
		t.Error("expected availabilityId to be omitted when empty")
	}
}

func TestPriceItem_RoundTrip(t *testing.T) {
	// Test that unmarshal -> marshal -> unmarshal produces identical values
	original := PriceItem{
		RetailPrice:          0.0104,
		UnitPrice:            0.0104,
		TierMinimumUnits:     0.0,
		CurrencyCode:         "USD",
		ArmRegionName:        "eastus",
		Location:             "US East",
		ArmSkuName:           "Standard_B1s",
		SkuName:              "B1s",
		SkuID:                "DZH318Z0BQPS/00TG",
		ServiceName:          "Virtual Machines",
		ServiceID:            "DZH313Z7MMC8",
		ServiceFamily:        "Compute",
		ProductName:          "Virtual Machines BS Series",
		ProductID:            "DZH318Z0BQPS",
		MeterID:              "000a794b-bdb0-58be-a0cd-0c3a0f222923",
		MeterName:            "B1s",
		UnitOfMeasure:        "1 Hour",
		IsPrimaryMeterRegion: true,
		EffectiveStartDate:   "2020-08-01T00:00:00Z",
		Type:                 "Consumption",
		ReservationTerm:      "1 Year",
		AvailabilityID:       "test-availability",
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var roundTripped PriceItem
	if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Compare values
	if original.RetailPrice != roundTripped.RetailPrice {
		t.Errorf("RetailPrice mismatch: %f != %f", original.RetailPrice, roundTripped.RetailPrice)
	}
	if original.CurrencyCode != roundTripped.CurrencyCode {
		t.Errorf("CurrencyCode mismatch: %s != %s", original.CurrencyCode, roundTripped.CurrencyCode)
	}
	if original.ArmRegionName != roundTripped.ArmRegionName {
		t.Errorf("ArmRegionName mismatch: %s != %s", original.ArmRegionName, roundTripped.ArmRegionName)
	}
	if original.Location != roundTripped.Location {
		t.Errorf("Location mismatch: %s != %s", original.Location, roundTripped.Location)
	}
	if original.ReservationTerm != roundTripped.ReservationTerm {
		t.Errorf("ReservationTerm mismatch: %s != %s", original.ReservationTerm, roundTripped.ReservationTerm)
	}
	if original.AvailabilityID != roundTripped.AvailabilityID {
		t.Errorf("AvailabilityID mismatch: %s != %s", original.AvailabilityID, roundTripped.AvailabilityID)
	}
	if original.IsPrimaryMeterRegion != roundTripped.IsPrimaryMeterRegion {
		t.Errorf("IsPrimaryMeterRegion mismatch: %v != %v",
			original.IsPrimaryMeterRegion, roundTripped.IsPrimaryMeterRegion)
	}
}

func TestPriceResponse_MarshalJSON(t *testing.T) {
	resp := PriceResponse{
		BillingCurrency:    "USD",
		CustomerEntityID:   "Default",
		CustomerEntityType: "Retail",
		Items: []PriceItem{
			{
				RetailPrice:   0.01,
				CurrencyCode:  "USD",
				ArmRegionName: "eastus",
			},
		},
		NextPageLink: "https://prices.azure.com/api/retail/prices?$skip=1000",
		Count:        1,
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal PriceResponse: %v", err)
	}

	// Verify JSON uses PascalCase field names (Azure API format for envelope)
	jsonStr := string(jsonBytes)
	expectedFields := []string{
		`"BillingCurrency":`,
		`"CustomerEntityId":`,
		`"CustomerEntityType":`,
		`"Items":`,
		`"NextPageLink":`,
		`"Count":`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("expected JSON to contain %s", field)
		}
	}
}

func TestPriceResponse_RoundTrip(t *testing.T) {
	original := PriceResponse{
		BillingCurrency:    "EUR",
		CustomerEntityID:   "TestEntity",
		CustomerEntityType: "Retail",
		Items: []PriceItem{
			{
				RetailPrice:   100.50,
				CurrencyCode:  "EUR",
				ArmRegionName: "westeurope",
				Type:          "Consumption",
			},
			{
				RetailPrice:     5000.00,
				CurrencyCode:    "EUR",
				ArmRegionName:   "westeurope",
				Type:            "Reservation",
				ReservationTerm: "3 Years",
			},
		},
		NextPageLink: "",
		Count:        2,
	}

	// Marshal
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var roundTripped PriceResponse
	if err := json.Unmarshal(jsonBytes, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Compare
	if original.BillingCurrency != roundTripped.BillingCurrency {
		t.Errorf("BillingCurrency mismatch")
	}
	if original.CustomerEntityID != roundTripped.CustomerEntityID {
		t.Errorf("CustomerEntityID mismatch")
	}
	if original.Count != roundTripped.Count {
		t.Errorf("Count mismatch: %d != %d", original.Count, roundTripped.Count)
	}
	if len(original.Items) != len(roundTripped.Items) {
		t.Fatalf("Items length mismatch: %d != %d", len(original.Items), len(roundTripped.Items))
	}
	if original.Items[1].ReservationTerm != roundTripped.Items[1].ReservationTerm {
		t.Errorf("Nested ReservationTerm mismatch")
	}
}
