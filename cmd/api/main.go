// cmd/api/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"task-forge/internal/cache"
	"task-forge/internal/config"
	"task-forge/internal/database"
	"task-forge/internal/handler"
	"task-forge/internal/middleware"
	"task-forge/internal/repository"
	"task-forge/internal/service"
	"task-forge/internal/validator"
	"time"

	"github.com/gin-gonic/gin"
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

	// Database
	db, err := database.OpenPostgres(ctx, cfg.Database, cfg.App.Mode, log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	log.Info().Msg("Connected to database")

	// Migrations
	if cfg.Migrations.Auto {
		if err := database.RunMigrations(cfg.Database, cfg.Migrations, log.Logger); err != nil {
			log.Fatal().Err(err).Msg("Failed to run migrations")
		}
	}

	// Redis Cache
	cacheService, err := cache.NewCacheService(cfg.Redis, log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer func() {
		if err := cacheService.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing Redis connection")
		}
	}()

	// Repositories
	repos := repository.NewRepositories(db, log.Logger)
	log.Info().Msg("Repositories initialized")

	// Services
	jwtManager := service.NewJWTManager(cfg.App.EncryptionKey, cfg.JWTExpiration())
	services := service.NewServices(repos, jwtManager, log.Logger)
	log.Info().Msg("Services initialized")

	// Validator
	appValidator := validator.NewValidator()
	log.Info().Msg("Validator initialized")

	// Middlewares
	middlewares := middleware.NewMiddlewares(cacheService, jwtManager, log.Logger)
	log.Info().Msg("Middlewares initialized")

	// Handlers & Router
	handlers := handler.NewHandlers(
		services,
		appValidator,
		cacheService,
		jwtManager,
		int(cfg.JWTExpiration().Seconds()),
		cfg.App.Mode,
		cfg.App.EncryptionKey,
		log.Logger,
	)

	if cfg.App.Mode == "release" || cfg.App.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger(log.Logger))

	handlers.RegisterRoutes(router, middlewares)
	log.Info().Msg("Routes registered")

	// Swagger UI
	swagger := router.Group("/swagger")
	{
		swagger.GET("/", gin.WrapH(http.HandlerFunc(handler.SwaggerUI)))
		swagger.GET("/swagger.yaml", gin.WrapH(http.HandlerFunc(handler.SwaggerYAML)))
	}
	log.Info().Str("url", "http://localhost:"+cfg.App.Port+"/swagger/").Msg("Swagger UI enabled")

	// HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.App.Port,
		Handler: router,
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

// ginLogger middleware for logging HTTP requests via zerolog.
func ginLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		event := logger.Info()
		if status >= 400 {
			event = logger.Warn()
		}
		if status >= 500 {
			event = logger.Error()
		}

		event.
			Int("status", status).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", query).
			Str("ip", c.ClientIP()).
			Str("user-agent", c.Request.UserAgent()).
			Dur("latency", latency).
			Msg("HTTP request")
	}
}
