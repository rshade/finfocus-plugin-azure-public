// Package main provides the entry point for the finfocus-plugin-azure-public plugin.
// This plugin implements cost estimation for Azure public cloud resources using
// the unauthenticated Azure Retail Prices API.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-plugin-azure-public/internal/pricing"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
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
	if lvl := pluginsdk.GetLogLevel(); lvl != "" {
		if parsed, err := zerolog.ParseLevel(lvl); err == nil {
			level = parsed
		}
	}

	// Create logger using SDK utility (outputs JSON to stderr)
	logger := pluginsdk.NewPluginLogger("azure-public", version, level, nil)

	// Log startup
	logger.Info().Msg("plugin started")

	// Determine port with SDK (FINFOCUS_PLUGIN_PORT > ephemeral)
	port := pluginsdk.GetPort()
	if port > 0 {
		logger.Debug().Int("port", port).Msg("using configured port")
	} else {
		logger.Debug().Msg("using ephemeral port")
	}

	// Create plugin instance
	azurePlugin := pricing.NewCalculator()

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
