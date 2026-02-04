package azureclient

import "github.com/rs/zerolog"

// zerologAdapter adapts zerolog.Logger to retryablehttp.LeveledLogger interface.
type zerologAdapter struct {
	logger zerolog.Logger
}

// Error logs an error message with optional key-value pairs.
func (z *zerologAdapter) Error(msg string, keysAndValues ...interface{}) {
	z.logger.Error().Fields(toFields(keysAndValues)).Msg(msg)
}

// Warn logs a warning message with optional key-value pairs.
func (z *zerologAdapter) Warn(msg string, keysAndValues ...interface{}) {
	z.logger.Warn().Fields(toFields(keysAndValues)).Msg(msg)
}

// Info logs an info message with optional key-value pairs.
func (z *zerologAdapter) Info(msg string, keysAndValues ...interface{}) {
	z.logger.Info().Fields(toFields(keysAndValues)).Msg(msg)
}

// Debug logs a debug message with optional key-value pairs.
func (z *zerologAdapter) Debug(msg string, keysAndValues ...interface{}) {
	z.logger.Debug().Fields(toFields(keysAndValues)).Msg(msg)
}

// toFields converts a slice of key-value pairs to a map for zerolog.
// Keys must be strings; non-string keys are skipped.
// Odd-length slices have their last element ignored.
func toFields(keysAndValues []interface{}) map[string]interface{} {
	fields := make(map[string]interface{})
	// Loop condition i+1 < len ensures we never access out-of-bounds
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string) //nolint:gosec // G602: bounds checked by loop condition i+1 < len
		if !ok {
			continue
		}
		fields[key] = keysAndValues[i+1]
	}
	return fields
}
