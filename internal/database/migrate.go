// internal/database/migrate.go
package database

import (
	"database/sql"
	"fmt"

	"task-forge/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

// RunMigrations применяет SQL миграции через Goose.
func RunMigrations(cfg config.DatabaseConfig, migrationsCfg config.MigrationConfig, log zerolog.Logger) error {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		migrationsCfg.User,
		migrationsCfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sql connection for migrations: %w", err)
	}
	defer db.Close()

	// Перехватываем логи Goose в zerolog
	goose.SetLogger(&GooseZerologAdapter{logger: log})

	if err := goose.Up(db, migrationsCfg.Path); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}

	log.Info().Str("path", migrationsCfg.Path).Msg("Database migrations applied successfully")
	
	return nil
}

// GooseZerologAdapter реализует интерфейс goose.Logger.
type GooseZerologAdapter struct {
	logger zerolog.Logger
}

func (l *GooseZerologAdapter) Fatalf(format string, v ...any) {
	l.logger.Fatal().Msgf(format, v...)
}

func (l *GooseZerologAdapter) Printf(format string, v ...any) {
	l.logger.Info().Msgf(format, v...)
}