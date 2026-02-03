// Package main provides the entry point for the finfocus-plugin-azure-public plugin.
// This plugin implements cost estimation for Azure public cloud resources using
// the unauthenticated Azure Retail Prices API.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus-plugin-azure-public/internal/pricing"
)

// version is the plugin version, set at build time via ldflags.
// Example: go build -ldflags "-X main.version=1.0.0" ./cmd/finfocus-plugin-azure-public.
var version = "dev"

// main is the entry point that delegates to run() and handles exit codes.
// This pattern ensures all defer statements execute properly before process exit.
func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run contains the main application logic, returning an error on failure.
// This function configures logging, initializes the plugin instance,
// and runs the plugin server until a shutdown signal is received.
func run() error {
	// Parse log level from environment using SDK (FINFOCUS_LOG_LEVEL > LOG_LEVEL > info)
	level := zerolog.InfoLevel
	lvl := pluginsdk.GetLogLevel()
	if lvl != "" {
		parsed, err := zerolog.ParseLevel(lvl)
		if err == nil {
			level = parsed
		} else {
			// Log warning for invalid level (will be logged at info level since logger not yet created)
			// We create a temporary logger to emit the warning
			tempLogger := pluginsdk.NewPluginLogger("azure-public", version, zerolog.InfoLevel, nil)
			tempLogger.Warn().Str("value", lvl).Err(err).Msg("invalid log level, falling back to info")
		}
	}

	// Create logger using SDK utility (outputs JSON to stderr)
	logger := pluginsdk.NewPluginLogger("azure-public", version, level, nil)

	// Log startup
	logger.Info().Msg("plugin started")

	// Validate and determine port
	// Explicit validation ensures invalid (non-numeric) values fail with clear error
	// rather than silently falling back to ephemeral port
	var port int
	if portStr := os.Getenv("FINFOCUS_PLUGIN_PORT"); portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil {
			logger.Error().Str("value", portStr).Msg("FINFOCUS_PLUGIN_PORT must be numeric")
			return fmt.Errorf("invalid FINFOCUS_PLUGIN_PORT: %q is not numeric: %w", portStr, err)
		}
		logger.Debug().Int("port", port).Msg("using configured port")
	} else {
		port = pluginsdk.GetPort() // Check fallback env var or return 0 for ephemeral
		if port > 0 {
			logger.Debug().Int("port", port).Msg("using configured port from fallback env")
		} else {
			logger.Debug().Msg("using ephemeral port")
		}
	}

	// Create plugin instance with logger
	azurePlugin := pricing.NewCalculator(logger)

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info().Msg("received shutdown signal")
		cancel()
	}()

	// Serve using pluginsdk
	config := pluginsdk.ServeConfig{
		Plugin: azurePlugin,
		Port:   port,
		PluginInfo: &pluginsdk.PluginInfo{
			Name:        "finfocus-plugin-azure-public",
			Version:     version,
			SpecVersion: pluginsdk.SpecVersion,
			Providers:   []string{"azure"},
			Metadata: map[string]string{
				"type": "public-pricing-fallback",
			},
		},
	}

	if err := pluginsdk.Serve(ctx, config); err != nil {
		logger.Error().Err(err).Msg("server error")
		return err
	}

	return nil
}
