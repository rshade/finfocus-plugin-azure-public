// Package logging provides plugin-specific logging utilities.
// This package wraps the pluginsdk logging functions for consistent usage patterns
// across the Azure public pricing plugin.
package logging

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// maxTraceIDLen is the maximum length of trace IDs to prevent log bloat from
// malicious or buggy clients sending oversized values.
const maxTraceIDLen = 128

// RequestLogger creates a request-scoped logger with trace ID from context.
// It uses pluginsdk.TraceIDFromContext to extract the trace ID from gRPC metadata.
//
// If a trace ID is present and non-empty, a child logger is returned with the
// trace_id field added. If no trace ID is present (or it's empty), the base
// logger is returned unchanged.
//
// Trace IDs longer than 128 characters are truncated to prevent log bloat.
//
// All existing fields on the base logger (plugin, version, etc.) are preserved.
func RequestLogger(ctx context.Context, base zerolog.Logger) zerolog.Logger {
	traceID := pluginsdk.TraceIDFromContext(ctx)
	if traceID == "" {
		return base
	}
	if len(traceID) > maxTraceIDLen {
		traceID = traceID[:maxTraceIDLen]
	}
	return base.With().Str("trace_id", traceID).Logger()
}
