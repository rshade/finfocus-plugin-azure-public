package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus-plugin-azure-public/internal/logging"
)

func TestRequestLogger_WithTraceID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := zerolog.New(&buf).With().Str("plugin", "test").Logger()

	ctx := pluginsdk.ContextWithTraceID(context.Background(), "trace-123")
	logger := logging.RequestLogger(ctx, base)

	logger.Info().Msg("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["trace_id"] != "trace-123" {
		t.Errorf("expected trace_id=%q, got %q", "trace-123", logEntry["trace_id"])
	}
	if logEntry["plugin"] != "test" {
		t.Errorf("expected plugin=%q, got %q", "test", logEntry["plugin"])
	}
}

func TestRequestLogger_WithoutTraceID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := zerolog.New(&buf).With().Str("plugin", "test").Logger()

	logger := logging.RequestLogger(context.Background(), base)
	logger.Info().Msg("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if _, exists := logEntry["trace_id"]; exists {
		t.Errorf("expected no trace_id field, but got %v", logEntry["trace_id"])
	}
	if logEntry["plugin"] != "test" {
		t.Errorf("expected plugin=%q, got %q", "test", logEntry["plugin"])
	}
}

func TestRequestLogger_PreservesExistingFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := zerolog.New(&buf).With().
		Str("plugin", "azure-public").
		Str("version", "1.0.0").
		Logger()

	ctx := pluginsdk.ContextWithTraceID(context.Background(), "abc-456")
	logger := logging.RequestLogger(ctx, base)

	logger.Info().Msg("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["plugin"] != "azure-public" {
		t.Errorf("expected plugin=%q, got %q", "azure-public", logEntry["plugin"])
	}
	if logEntry["version"] != "1.0.0" {
		t.Errorf("expected version=%q, got %q", "1.0.0", logEntry["version"])
	}
	if logEntry["trace_id"] != "abc-456" {
		t.Errorf("expected trace_id=%q, got %q", "abc-456", logEntry["trace_id"])
	}
}

func TestRequestLogger_EmptyTraceIDNotAdded(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := zerolog.New(&buf).With().Str("plugin", "test").Logger()

	// Context with empty trace ID should not add trace_id field
	ctx := pluginsdk.ContextWithTraceID(context.Background(), "")
	logger := logging.RequestLogger(ctx, base)

	logger.Info().Msg("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if _, exists := logEntry["trace_id"]; exists {
		t.Errorf("expected no trace_id field for empty string, but got %v", logEntry["trace_id"])
	}
}

func TestRequestLogger_TruncatesLongTraceID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := zerolog.New(&buf).With().Str("plugin", "test").Logger()

	// Create a trace ID longer than 128 characters
	longTraceID := strings.Repeat("x", 200)

	ctx := pluginsdk.ContextWithTraceID(context.Background(), longTraceID)
	logger := logging.RequestLogger(ctx, base)

	logger.Info().Msg("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	traceID, ok := logEntry["trace_id"].(string)
	if !ok {
		t.Fatalf("expected trace_id to be a string, got %T", logEntry["trace_id"])
	}

	if len(traceID) != 128 {
		t.Errorf("expected trace_id to be truncated to 128 chars, got %d chars", len(traceID))
	}
}
