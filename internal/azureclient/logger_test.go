package azureclient

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestZerologAdapter_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	adapter := &zerologAdapter{logger: logger}

	adapter.Error("test error", "key1", "value1", "key2", 42)

	output := buf.String()
	if !strings.Contains(output, `"level":"error"`) {
		t.Error("expected error level")
	}
	if !strings.Contains(output, "test error") {
		t.Error("expected message in output")
	}
}

func TestZerologAdapter_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	adapter := &zerologAdapter{logger: logger}

	adapter.Warn("test warning", "attempt", 2)

	output := buf.String()
	if !strings.Contains(output, `"level":"warn"`) {
		t.Error("expected warn level")
	}
}

func TestZerologAdapter_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	adapter := &zerologAdapter{logger: logger}

	adapter.Info("test info")

	output := buf.String()
	if !strings.Contains(output, `"level":"info"`) {
		t.Error("expected info level")
	}
}

func TestZerologAdapter_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	adapter := &zerologAdapter{logger: logger}

	adapter.Debug("test debug", "url", "https://example.com")

	output := buf.String()
	if !strings.Contains(output, `"level":"debug"`) {
		t.Error("expected debug level")
	}
}

func TestZerologAdapter_KeyValuePairs(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	adapter := &zerologAdapter{logger: logger}

	adapter.Info("test", "string_key", "string_value", "int_key", 123, "bool_key", true)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if logEntry["string_key"] != "string_value" {
		t.Errorf("expected string_key=string_value, got %v", logEntry["string_key"])
	}
	if logEntry["int_key"] != float64(123) {
		t.Errorf("expected int_key=123, got %v", logEntry["int_key"])
	}
	if logEntry["bool_key"] != true {
		t.Errorf("expected bool_key=true, got %v", logEntry["bool_key"])
	}
}

func TestZerologAdapter_OddKeyValuePairs(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	adapter := &zerologAdapter{logger: logger}

	// Odd number of key-value pairs should not panic
	adapter.Info("test", "key1", "value1", "orphan_key")

	output := buf.String()
	if !strings.Contains(output, "test") {
		t.Error("expected message in output even with odd key-value pairs")
	}
}

func TestToFields_Empty(t *testing.T) {
	fields := toFields(nil)
	if len(fields) != 0 {
		t.Errorf("expected empty map, got %v", fields)
	}

	fields = toFields([]interface{}{})
	if len(fields) != 0 {
		t.Errorf("expected empty map, got %v", fields)
	}
}

func TestToFields_ValidPairs(t *testing.T) {
	keysAndValues := []interface{}{"key1", "value1", "key2", 42}
	fields := toFields(keysAndValues)

	if fields["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", fields["key1"])
	}
	if fields["key2"] != 42 {
		t.Errorf("expected key2=42, got %v", fields["key2"])
	}
}
