// cmd/api/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"task-forge/internal/config"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const shutdownTimeout = 5 * time.Second

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		log.Fatal().
			Err(err).
			Msg("application error")
	}
}

func run(ctx context.Context) error {
	// Logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	log.Logger = zerolog.
		New(os.Stderr).
		With().
		Timestamp().
		Logger()

	// Configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	report := cfg.Validate()
	report.Log(&log.Logger)

	if report.HasFatal() {
		return fmt.Errorf("configuration validation failed")
	}

	log.Info().Msg("configuration loaded successfully")

	// Signal handling
	ctx, stop := signal.NotifyContext(
		ctx,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

  // ...

	// HTTP server
	server := &http.Server{
		Addr: ":" + cfg.App.Port,
		// Handler: router,
	}

	serverErr := make(chan error, 1)

	go func() {
		log.Info().
			Str("port", cfg.App.Port).
			Msg("starting HTTP server")

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		return fmt.Errorf("HTTP server failed: %w", err)
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("HTTP server shutdown failed: %w", err)
	}

	log.Info().Msg("HTTP server stopped")

	return nil
}
