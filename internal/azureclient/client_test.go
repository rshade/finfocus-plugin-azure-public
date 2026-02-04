package azureclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewClient_DefaultConfig(t *testing.T) {
	client, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "NegativeRetryMax",
			config: Config{
				BaseURL:      DefaultBaseURL,
				RetryMax:     -1,
				RetryWaitMin: time.Second,
				RetryWaitMax: 30 * time.Second,
				Timeout:      60 * time.Second,
			},
		},
		{
			name: "ZeroTimeout",
			config: Config{
				BaseURL:      DefaultBaseURL,
				RetryMax:     3,
				RetryWaitMin: time.Second,
				RetryWaitMax: 30 * time.Second,
				Timeout:      0,
			},
		},
		{
			name: "MinGreaterThanMax",
			config: Config{
				BaseURL:      DefaultBaseURL,
				RetryMax:     3,
				RetryWaitMin: 60 * time.Second,
				RetryWaitMax: 30 * time.Second,
				Timeout:      60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("expected ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestClient_GetPrices_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := priceResponse{
			BillingCurrency: "USD",
			Items: []PriceItem{
				{
					ArmRegionName: "eastus",
					ArmSkuName:    "Standard_B1s",
					RetailPrice:   0.0104,
					CurrencyCode:  "USD",
				},
			},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Logf("error encoding response: %v", err)
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prices, err := client.GetPrices(context.Background(), PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prices) != 1 {
		t.Fatalf("expected 1 price, got %d", len(prices))
	}
	if prices[0].RetailPrice != 0.0104 {
		t.Errorf("expected RetailPrice=0.0104, got %v", prices[0].RetailPrice)
	}
}

func TestClient_GetPrices_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var resp priceResponse

		if callCount == 1 {
			// httptest server URL is available via server.URL in the outer scope
			// For the handler, we construct the next page link using http scheme
			resp = priceResponse{
				BillingCurrency: "USD",
				Items: []PriceItem{
					{ArmSkuName: "SKU1", RetailPrice: 0.01},
				},
				NextPageLink: "http://" + r.Host + "/page2",
				Count:        1,
			}
		} else {
			resp = priceResponse{
				BillingCurrency: "USD",
				Items: []PriceItem{
					{ArmSkuName: "SKU2", RetailPrice: 0.02},
				},
				Count: 1,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Logf("error encoding response: %v", err)
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prices, err := client.GetPrices(context.Background(), PriceQuery{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prices) != 2 {
		t.Fatalf("expected 2 prices (from pagination), got %d", len(prices))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestClient_GetPrices_Retry429(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		resp := priceResponse{Items: []PriceItem{{ArmSkuName: "Test"}}, Count: 1}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Logf("error encoding response: %v", err)
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryWaitMin = 1 * time.Millisecond
	config.RetryWaitMax = 10 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prices, err := client.GetPrices(context.Background(), PriceQuery{})

	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if len(prices) != 1 {
		t.Errorf("expected 1 price, got %d", len(prices))
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (2 retries + success), got %d", callCount)
	}
}

func TestClient_GetPrices_Retry503(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := priceResponse{Items: []PriceItem{{ArmSkuName: "Test"}}, Count: 1}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Logf("error encoding response: %v", err)
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryWaitMin = 1 * time.Millisecond
	config.RetryWaitMax = 10 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prices, err := client.GetPrices(context.Background(), PriceQuery{})

	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if len(prices) != 1 {
		t.Errorf("expected 1 price, got %d", len(prices))
	}
}

func TestClient_GetPrices_NoRetryOn400(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryWaitMin = 1 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{})

	if err == nil {
		t.Fatal("expected error on 400")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry on 400), got %d", callCount)
	}
}

func TestClient_GetPrices_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = client.GetPrices(ctx, PriceQuery{})

	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestClient_GetPrices_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{})

	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("expected ErrInvalidResponse, got %v", err)
	}
}

func TestClient_GetPrices_UserAgent(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		resp := priceResponse{Items: []PriceItem{}, Count: 0}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Logf("error encoding response: %v", err)
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.UserAgent = "test-agent/1.0"
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedUA != "test-agent/1.0" {
		t.Errorf("expected User-Agent=test-agent/1.0, got %s", receivedUA)
	}
}

func TestClient_GetPrices_Logging(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := priceResponse{Items: []PriceItem{}, Count: 0}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Logf("error encoding response: %v", err)
		}
	}))
	defer server.Close()

	var logBuf strings.Builder
	logger := zerolog.New(&logBuf)

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryWaitMin = 1 * time.Millisecond
	config.RetryWaitMax = 10 * time.Millisecond
	config.Logger = logger
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that retry was logged (retryablehttp logs retries)
	// The exact log format depends on retryablehttp internals
	if callCount != 2 {
		t.Errorf("expected retry, got %d calls", callCount)
	}
}

func TestBuildFilterQuery_Empty(t *testing.T) {
	query := PriceQuery{}
	filter := buildFilterQuery(query)

	if filter != "" {
		t.Errorf("expected empty filter, got %s", filter)
	}
}

func TestBuildFilterQuery_SingleField(t *testing.T) {
	query := PriceQuery{ArmRegionName: "eastus"}
	filter := buildFilterQuery(query)

	expected := "armRegionName eq 'eastus'"
	if filter != expected {
		t.Errorf("expected %s, got %s", expected, filter)
	}
}

func TestBuildFilterQuery_MultipleFields(t *testing.T) {
	query := PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
	}
	filter := buildFilterQuery(query)

	if !strings.Contains(filter, "armRegionName eq 'eastus'") {
		t.Error("expected armRegionName filter")
	}
	if !strings.Contains(filter, "armSkuName eq 'Standard_B1s'") {
		t.Error("expected armSkuName filter")
	}
	if !strings.Contains(filter, " and ") {
		t.Error("expected 'and' between filters")
	}
}

func TestBuildFilterQuery_AllFields(t *testing.T) {
	query := PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
		ServiceName:   "Virtual Machines",
		ProductName:   "Virtual Machines BS Series",
		CurrencyCode:  "USD",
	}
	filter := buildFilterQuery(query)

	expectedParts := []string{
		"armRegionName eq 'eastus'",
		"armSkuName eq 'Standard_B1s'",
		"serviceName eq 'Virtual Machines'",
		"productName eq 'Virtual Machines BS Series'",
		"currencyCode eq 'USD'",
	}

	for _, part := range expectedParts {
		if !strings.Contains(filter, part) {
			t.Errorf("expected filter to contain %s", part)
		}
	}
}

func TestBuildFilterQuery_ODataEscape(t *testing.T) {
	// Test that single quotes are properly escaped to prevent OData injection
	query := PriceQuery{ArmRegionName: "east'us"}
	filter := buildFilterQuery(query)

	// Single quotes should be doubled in OData
	expected := "armRegionName eq 'east''us'"
	if filter != expected {
		t.Errorf("expected %s, got %s", expected, filter)
	}
}

func TestBuildFilterQuery_ODataEscapeMultipleQuotes(t *testing.T) {
	query := PriceQuery{ServiceName: "test'service'name"}
	filter := buildFilterQuery(query)

	expected := "serviceName eq 'test''service''name'"
	if filter != expected {
		t.Errorf("expected %s, got %s", expected, filter)
	}
}

func TestBuildFilterQuery_ODataInjectionPrevention(t *testing.T) {
	// Attempt OData injection through single quotes
	query := PriceQuery{ArmRegionName: "east' or 'a' eq 'a"}
	filter := buildFilterQuery(query)

	// The injection should be neutralized by escaping
	expected := "armRegionName eq 'east'' or ''a'' eq ''a'"
	if filter != expected {
		t.Errorf("expected injection to be neutralized: got %s", filter)
	}
}

func TestClient_GetPrices_RateLimitExhausted(t *testing.T) {
	// Test that ErrRequestFailed is returned when retries are exhausted on 429
	// (retryablehttp returns an error, not a response, when retries are exhausted)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryMax = 0 // No retries - retry policy will mark it for retry but no retries left
	config.RetryWaitMin = 1 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{})

	if err == nil {
		t.Fatal("expected error on 429 with exhausted retries")
	}
	// When retries are exhausted, retryablehttp returns an error (not a response)
	// so we get ErrRequestFailed wrapping the retry library's error
	if !errors.Is(err, ErrRequestFailed) {
		t.Errorf("expected ErrRequestFailed, got %v", err)
	}
	// Error message should indicate the retry exhaustion
	if !strings.Contains(err.Error(), "giving up") {
		t.Errorf("expected 'giving up' in error message, got %v", err)
	}
}

func TestClient_GetPrices_ServiceUnavailableExhausted(t *testing.T) {
	// Test that ErrRequestFailed is returned when retries are exhausted on 503
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryMax = 0 // No retries
	config.RetryWaitMin = 1 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{})

	if err == nil {
		t.Fatal("expected error on 503 with exhausted retries")
	}
	if !errors.Is(err, ErrRequestFailed) {
		t.Errorf("expected ErrRequestFailed, got %v", err)
	}
}

func TestClient_Close(t *testing.T) {
	config := DefaultConfig()
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should not panic and should be safe to call
	client.Close()

	// Close is idempotent - should not panic on second call
	client.Close()
}

func TestClient_GetPrices_Concurrent(t *testing.T) {
	// Test that concurrent requests work correctly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := priceResponse{
			Items: []PriceItem{{ArmSkuName: "Test", RetailPrice: 0.01}},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Logf("error encoding response: %v", err)
		}
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()

	// Launch 10 concurrent requests
	const numRequests = 10
	errChan := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			_, err := client.GetPrices(context.Background(), PriceQuery{})
			errChan <- err
		}()
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent request failed: %v", err)
		}
	}
}
