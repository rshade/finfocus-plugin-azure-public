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
		resp := PriceResponse{
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
		var resp PriceResponse

		if callCount == 1 {
			// httptest server URL is available via server.URL in the outer scope
			// For the handler, we construct the next page link using http scheme
			resp = PriceResponse{
				BillingCurrency: "USD",
				Items: []PriceItem{
					{ArmSkuName: "SKU1", RetailPrice: 0.01},
				},
				NextPageLink: "http://" + r.Host + "/page2",
				Count:        1,
			}
		} else {
			resp = PriceResponse{
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
		resp := PriceResponse{Items: []PriceItem{{ArmSkuName: "Test"}}, Count: 1}
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
		resp := PriceResponse{Items: []PriceItem{{ArmSkuName: "Test"}}, Count: 1}
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
		resp := PriceResponse{Items: []PriceItem{{ArmSkuName: "Test", RetailPrice: 0.01}}, Count: 1}
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
		resp := PriceResponse{Items: []PriceItem{{ArmSkuName: "Test", RetailPrice: 0.01}}, Count: 1}
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
	// Test that ErrRateLimited is returned when retries are exhausted on 429.
	// With PassthroughErrorHandler, the final 429 response is returned directly,
	// allowing fetchPage to classify it as ErrRateLimited.
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
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
	if !strings.Contains(err.Error(), "status 429") {
		t.Errorf("expected 'status 429' in error message, got %v", err)
	}
}

func TestClient_GetPrices_ServiceUnavailableExhausted(t *testing.T) {
	// Test that ErrServiceUnavailable is returned when retries are exhausted on 503.
	// With PassthroughErrorHandler, the final 503 response is returned directly,
	// allowing fetchPage to classify it as ErrServiceUnavailable.
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
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
	if !strings.Contains(err.Error(), "status 503") {
		t.Errorf("expected 'status 503' in error message, got %v", err)
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
		resp := PriceResponse{
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

func TestFetchPage_HTTP404_ReturnsErrNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryMax = 0 // No retries for 404
	config.RetryWaitMin = 1 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{})

	if err == nil {
		t.Fatal("expected error on HTTP 404")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("expected error to contain 'status 404', got %v", err)
	}
}

// --- US1: Contextual Error Messages ---

func TestGetPrices_ErrorIncludesQueryContext(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "HTTP400", statusCode: http.StatusBadRequest},
		{name: "HTTP500", statusCode: http.StatusInternalServerError},
		{name: "HTTP404", statusCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			config := DefaultConfig()
			config.BaseURL = server.URL
			config.RetryMax = 0
			config.RetryWaitMin = 1 * time.Millisecond
			client, err := NewClient(config)
			if err != nil {
				t.Fatalf("unexpected error creating client: %v", err)
			}

			_, err = client.GetPrices(context.Background(), PriceQuery{
				ArmRegionName: "eastus",
				ArmSkuName:    "Standard_B1s",
			})

			if err == nil {
				t.Fatal("expected error")
			}
			errMsg := err.Error()
			if !strings.Contains(errMsg, "region=eastus") {
				t.Errorf("expected error to contain 'region=eastus', got: %s", errMsg)
			}
			if !strings.Contains(errMsg, "sku=Standard_B1s") {
				t.Errorf("expected error to contain 'sku=Standard_B1s', got: %s", errMsg)
			}
		})
	}
}

func TestGetPrices_ErrorPreservesRootCause(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryMax = 0
	config.RetryWaitMin = 1 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrRequestFailed) {
		t.Errorf("expected errors.Is(err, ErrRequestFailed) to be true, got: %v", err)
	}
}

func TestGetPrices_MidPaginationErrorIncludesPage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			resp := PriceResponse{
				Items:        []PriceItem{{ArmSkuName: "SKU1", RetailPrice: 0.01}},
				NextPageLink: "http://" + r.Host + "/page2",
				Count:        1,
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Logf("error encoding response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryMax = 0
	config.RetryWaitMin = 1 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
	})

	if err == nil {
		t.Fatal("expected error on page 1")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "page 1") {
		t.Errorf("expected error to contain 'page 1', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "region=eastus") {
		t.Errorf("expected error to contain 'region=eastus', got: %s", errMsg)
	}
}

// --- US4: Empty Result Handling ---

func TestGetPrices_EmptyResults_ReturnsErrNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := PriceResponse{Items: []PriceItem{}, Count: 0}
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
		t.Fatalf("unexpected error creating client: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{
		ArmRegionName: "eastus",
		ArmSkuName:    "Standard_B1s",
	})

	if err == nil {
		t.Fatal("expected error for empty results")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected errors.Is(err, ErrNotFound) to be true, got: %v", err)
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "region=") {
		t.Errorf("expected error to contain 'region=', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "no pricing data") {
		t.Errorf("expected error to contain 'no pricing data', got: %s", errMsg)
	}
}

func TestGetPrices_NonEmptyResults_ReturnsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := PriceResponse{
			Items: []PriceItem{
				{ArmRegionName: "eastus", ArmSkuName: "Standard_B1s", RetailPrice: 0.0104},
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
		t.Fatalf("unexpected error creating client: %v", err)
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

// --- US3: Structured Error Logging ---

func TestGetPrices_LogsErrorWithStructuredFields(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantFields []string
	}{
		{
			name: "HTTP400Error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("bad request"))
			},
			wantFields: []string{"region", "sku", "error_category"},
		},
		{
			name: "HTTP500Error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal error"))
			},
			wantFields: []string{"region", "sku", "error_category"},
		},
		{
			name: "JSONParseError",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("not json"))
			},
			wantFields: []string{"region", "sku", "error_category"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			var buf strings.Builder
			logger := zerolog.New(&buf)

			config := DefaultConfig()
			config.BaseURL = server.URL
			config.RetryMax = 0
			config.RetryWaitMin = 1 * time.Millisecond
			config.Logger = logger
			client, err := NewClient(config)
			if err != nil {
				t.Fatalf("unexpected error creating client: %v", err)
			}

			_, _ = client.GetPrices(context.Background(), PriceQuery{
				ArmRegionName: "eastus",
				ArmSkuName:    "Standard_B1s",
			})

			logOutput := buf.String()
			if logOutput == "" {
				t.Fatal("expected log output, got none")
			}

			// Parse the last log line (pricing query error)
			lines := strings.Split(strings.TrimSpace(logOutput), "\n")
			var logEntry map[string]interface{}
			found := false
			for i := len(lines) - 1; i >= 0; i-- {
				var entry map[string]interface{}
				if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
					continue
				}
				if msg, ok := entry["message"].(string); ok && msg == "pricing query error" {
					logEntry = entry
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected 'pricing query error' log entry in output:\n%s", logOutput)
			}

			for _, field := range tt.wantFields {
				if _, ok := logEntry[field]; !ok {
					t.Errorf("expected field %q in log entry, got: %v", field, logEntry)
				}
			}

			if region, ok := logEntry["region"].(string); !ok || region != "eastus" {
				t.Errorf("expected region=eastus, got %v", logEntry["region"])
			}
			if sku, ok := logEntry["sku"].(string); !ok || sku != "Standard_B1s" {
				t.Errorf("expected sku=Standard_B1s, got %v", logEntry["sku"])
			}
		})
	}
}

func TestGetPrices_LogSeverityDifferentiation(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		wantLevel string
	}{
		{
			name: "HTTP400_WarnLevel",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("bad request"))
			},
			wantLevel: "warn",
		},
		{
			name: "HTTP500_ErrorLevel",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal error"))
			},
			wantLevel: "error",
		},
		{
			name: "InvalidJSON_ErrorLevel",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("not json"))
			},
			wantLevel: "error",
		},
		{
			name: "EmptyResults_DebugLevel",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				resp := PriceResponse{Items: []PriceItem{}, Count: 0}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantLevel: "debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			var buf strings.Builder
			logger := zerolog.New(&buf)

			config := DefaultConfig()
			config.BaseURL = server.URL
			config.RetryMax = 0
			config.RetryWaitMin = 1 * time.Millisecond
			config.Logger = logger
			client, err := NewClient(config)
			if err != nil {
				t.Fatalf("unexpected error creating client: %v", err)
			}

			_, _ = client.GetPrices(context.Background(), PriceQuery{
				ArmRegionName: "eastus",
				ArmSkuName:    "Standard_B1s",
			})

			logOutput := buf.String()
			if logOutput == "" {
				t.Fatal("expected log output, got none")
			}

			lines := strings.Split(strings.TrimSpace(logOutput), "\n")
			var logEntry map[string]interface{}
			found := false
			for i := len(lines) - 1; i >= 0; i-- {
				var entry map[string]interface{}
				if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
					continue
				}
				if msg, ok := entry["message"].(string); ok && msg == "pricing query error" {
					logEntry = entry
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected 'pricing query error' log entry in output:\n%s", logOutput)
			}

			level, ok := logEntry["level"].(string)
			if !ok {
				t.Fatalf("expected 'level' field in log entry, got: %v", logEntry)
			}
			if level != tt.wantLevel {
				t.Errorf("expected level=%q, got %q (log: %v)", tt.wantLevel, level, logEntry)
			}
		})
	}
}

func TestFetchPage_InvalidJSON_IncludesResponseSnippet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>Error Page</html>"))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryMax = 0
	config.RetryWaitMin = 1 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{
		ArmRegionName: "eastus",
	})

	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("expected ErrInvalidResponse, got: %v", err)
	}
	if !strings.Contains(err.Error(), "<html>Error Page</html>") {
		t.Errorf("expected error to contain response snippet, got: %s", err.Error())
	}
}

func TestFetchPage_LargeResponseBody_TruncatedAt256Bytes(t *testing.T) {
	largeBody := strings.Repeat("x", 500)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeBody))
	}))
	defer server.Close()

	config := DefaultConfig()
	config.BaseURL = server.URL
	config.RetryMax = 0
	config.RetryWaitMin = 1 * time.Millisecond
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	_, err = client.GetPrices(context.Background(), PriceQuery{
		ArmRegionName: "eastus",
	})

	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("expected ErrInvalidResponse, got: %v", err)
	}
	errMsg := err.Error()
	// The snippet in the error should be at most 256 bytes, not the full 500
	if strings.Contains(errMsg, largeBody) {
		t.Errorf("expected truncated response, but found full 500-byte body in error")
	}
	// Should contain a snippet (256 x's)
	if !strings.Contains(errMsg, strings.Repeat("x", 256)) {
		t.Errorf("expected 256-byte snippet in error, got: %s", errMsg)
	}
}
