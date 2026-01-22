package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rshade/finfocus-plugin-azure-public/internal/pricing"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

func main() {
	// Initialize dependencies to prevent pruning
	var _ *retryablehttp.Client
	var _ zerolog.Logger

	// Configure logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Create the plugin implementation
	plugin := pricing.NewCalculator()

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info().Msg("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Start serving the plugin
	config := pluginsdk.ServeConfig{
		Plugin: plugin,
		Port:   0, // Let the system choose a port
	}

	// This is just a stub main to verify dependencies and interfaces
	// In a real run, pluginsdk.Serve blocks
	_ = config
	_ = ctx
}
