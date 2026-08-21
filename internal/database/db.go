// internal/database/db.go
package database

import (
	"context"
	"fmt"
	"time"

	"task-forge/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog"
)

// OpenPostgres creates a connection pool with PostgreSQL.
func OpenPostgres(ctx context.Context, cfg config.DatabaseConfig, mode string, log zerolog.Logger) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pgxpool config: %w", err)
	}

	poolCfg.MaxConns = 25
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	// Integration of the pgx logger with zerolog
	poolCfg.ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   &PgxZerologAdapter{logger: log},
		LogLevel: mapPgxLogLevel(mode),
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	log.Info().
		Str("host", cfg.Host).
		Str("dbname", cfg.Name).
		Str("environment", mode).
		Msg("PostgreSQL connected")

	return pool, nil
}

func mapPgxLogLevel(mode string) tracelog.LogLevel {
	switch mode {
	case "prod", "release", "production":
		return tracelog.LogLevelWarn
	case "dev", "debug", "development":
		return tracelog.LogLevelInfo
	default:
		return tracelog.LogLevelError
	}
}
