package azureclient

import (
"context"
"net/http"
"net/http/httptest"
"testing"
"time"

"github.com/rs/zerolog"
)

func TestZeroValueLogger_NoPanic(t *testing.T) {
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusNotFound)
}))
defer server.Close()

// Create config with ZERO-VALUE logger (not Nop())
config := Config{
BaseURL:      server.URL,
RetryMax:     0,
RetryWaitMin: 1 * time.Millisecond,
RetryWaitMax: 1 * time.Second,
Timeout:      5 * time.Second,
// Logger is intentionally not set - will be zero-value
}

client, err := NewClient(config)
if err != nil {
t.Fatalf("unexpected error creating client: %v", err)
}

// This should trigger logError with a zero-value logger
_, err = client.GetPrices(context.Background(), PriceQuery{
ArmRegionName: "eastus",
})

if err == nil {
t.Fatal("expected error for 404 response")
}

// If we get here without panic, the zero-value logger is safe
t.Log("✓ No panic with zero-value logger")
}

func TestNilEvent_Safe(t *testing.T) {
var logger zerolog.Logger
event := logger.Debug()

// This should be safe even though event is nil
event.Str("test", "value").Msg("message")

t.Log("✓ Nil event is safe to use")
}
