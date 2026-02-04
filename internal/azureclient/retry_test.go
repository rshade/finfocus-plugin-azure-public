package azureclient

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestCustomRetryPolicy_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	shouldRetry, err := customRetryPolicy(ctx, nil, nil)

	if shouldRetry {
		t.Error("expected no retry when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestCustomRetryPolicy_NetworkError(t *testing.T) {
	ctx := context.Background()
	networkErr := errors.New("connection refused")

	shouldRetry, err := customRetryPolicy(ctx, nil, networkErr)

	if !shouldRetry {
		t.Error("expected retry on network error")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestCustomRetryPolicy_RateLimit429(t *testing.T) {
	ctx := context.Background()
	resp := &http.Response{StatusCode: http.StatusTooManyRequests}

	shouldRetry, err := customRetryPolicy(ctx, resp, nil)

	if !shouldRetry {
		t.Error("expected retry on 429")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestCustomRetryPolicy_ServiceUnavailable503(t *testing.T) {
	ctx := context.Background()
	resp := &http.Response{StatusCode: http.StatusServiceUnavailable}

	shouldRetry, err := customRetryPolicy(ctx, resp, nil)

	if !shouldRetry {
		t.Error("expected retry on 503")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestCustomRetryPolicy_NoRetryOn4xx(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"BadRequest400", http.StatusBadRequest},
		{"Unauthorized401", http.StatusUnauthorized},
		{"Forbidden403", http.StatusForbidden},
		{"NotFound404", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			resp := &http.Response{StatusCode: tt.statusCode}

			shouldRetry, err := customRetryPolicy(ctx, resp, nil)

			if shouldRetry {
				t.Errorf("expected no retry on %d", tt.statusCode)
			}
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
		})
	}
}

func TestCustomRetryPolicy_NoRetryOn5xx(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"InternalServerError500", http.StatusInternalServerError},
		{"BadGateway502", http.StatusBadGateway},
		{"GatewayTimeout504", http.StatusGatewayTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			resp := &http.Response{StatusCode: tt.statusCode}

			shouldRetry, err := customRetryPolicy(ctx, resp, nil)

			if shouldRetry {
				t.Errorf("expected no retry on %d", tt.statusCode)
			}
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
		})
	}
}

func TestCustomRetryPolicy_Success200(t *testing.T) {
	ctx := context.Background()
	resp := &http.Response{StatusCode: http.StatusOK}

	shouldRetry, err := customRetryPolicy(ctx, resp, nil)

	if shouldRetry {
		t.Error("expected no retry on 200")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"5"}},
	}

	duration := parseRetryAfter(resp)

	if duration != 5*time.Second {
		t.Errorf("expected 5s, got %v", duration)
	}
}

func TestParseRetryAfter_EmptyHeader(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}

	duration := parseRetryAfter(resp)

	if duration != 0 {
		t.Errorf("expected 0, got %v", duration)
	}
}

func TestParseRetryAfter_InvalidValue(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"invalid"}},
	}

	duration := parseRetryAfter(resp)

	if duration != 0 {
		t.Errorf("expected 0 for invalid value, got %v", duration)
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	// Use a future time
	futureTime := time.Now().Add(10 * time.Second)
	httpDate := futureTime.UTC().Format(http.TimeFormat)

	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{httpDate}},
	}

	duration := parseRetryAfter(resp)

	// Allow some tolerance for test execution time
	if duration < 9*time.Second || duration > 11*time.Second {
		t.Errorf("expected ~10s, got %v", duration)
	}
}

func TestParseRetryAfter_NegativeDuration(t *testing.T) {
	// Use a past time
	pastTime := time.Now().Add(-10 * time.Second)
	httpDate := pastTime.UTC().Format(http.TimeFormat)

	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{httpDate}},
	}

	duration := parseRetryAfter(resp)

	// Past times should return 0 (don't wait negative time)
	if duration > 0 {
		t.Errorf("expected 0 for past time, got %v", duration)
	}
}

func TestParseRetryAfter_ZeroSeconds(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"0"}},
	}

	duration := parseRetryAfter(resp)

	// Zero seconds should return 0
	if duration != 0 {
		t.Errorf("expected 0 for '0', got %v", duration)
	}
}

func TestParseRetryAfter_NegativeSeconds(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"-5"}},
	}

	duration := parseRetryAfter(resp)

	// Negative numbers parsed as integers should return 0
	if duration != 0 {
		t.Errorf("expected 0 for '-5', got %v", duration)
	}
}

func TestParseRetryAfter_LargeNumber(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"3600"}}, // 1 hour
	}

	duration := parseRetryAfter(resp)

	expected := 3600 * time.Second
	if duration != expected {
		t.Errorf("expected %v, got %v", expected, duration)
	}
}

func TestParseRetryAfter_FloatingPoint(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"5.5"}},
	}

	duration := parseRetryAfter(resp)

	// Floating point is not valid per RFC 7231, should return 0
	if duration != 0 {
		t.Errorf("expected 0 for floating point '5.5', got %v", duration)
	}
}

func TestParseRetryAfter_NilResponse(t *testing.T) {
	duration := parseRetryAfter(nil)

	if duration != 0 {
		t.Errorf("expected 0 for nil response, got %v", duration)
	}
}
