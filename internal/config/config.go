// internal/config/config.go
package config

import (
	"fmt"

	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type ValidationSeverity string

const (
	SeverityWarning ValidationSeverity = "warning"
	SeverityFatal   ValidationSeverity = "fatal"
)

type ValidationError struct {
	Severity ValidationSeverity
	Field    string
	Message  string
}

type ValidationReport struct {
	Errors []ValidationError
}

const MigrationsPath = "/app/migrations"

type Config struct {
	App        AppConfig       `mapstructure:"app"`
	Database   DatabaseConfig  `mapstructure:"database"`
	Migrations MigrationConfig `mapstructure:"migrations"`
	Logging    LoggingConfig   `mapstructure:"logging"`
	Redis      RedisConfig     `mapstructure:"redis"`
	JWT        JWTConfig       `mapstructure:"jwt"`
}

type AppConfig struct {
	Port   string `mapstructure:"port"`
	Mode   string `mapstructure:"mode"`
	Secret string `mapstructure:"secret"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type MigrationConfig struct {
	Auto     bool   `mapstructure:"auto"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Path     string `mapstructure:"-"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
}

type JWTConfig struct {
	Expiry int `mapstructure:"expiry"`
}

// Load the application configuration.
func Load() (*Config, error) {
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// =========================================================================
	// APP
	// =========================================================================

	if err := v.BindEnv("app.port", "APP_PORT"); err != nil {
		return nil, fmt.Errorf("bind APP_PORT: %w", err)
	}

	if err := v.BindEnv("app.mode", "APP_MODE"); err != nil {
		return nil, fmt.Errorf("bind APP_MODE: %w", err)
	}

	if err := v.BindEnv("app.secret", "APP_JWT_SECRET"); err != nil {
		return nil, fmt.Errorf("bind APP_JWT_SECRET: %w", err)
	}

	v.SetDefault("app.mode", "release")

	// =========================================================================
	// LOGGING
	// =========================================================================

	if err := v.BindEnv("logging.level", "APP_LOGGING_LEVEL"); err != nil {
		return nil, fmt.Errorf("bind APP_LOGGING_LEVEL: %w", err)
	}

	v.SetDefault("logging.level", "info")

	// =========================================================================
	// JWT
	// =========================================================================

	if err := v.BindEnv("jwt.expiry", "APP_JWT_EXPIRY"); err != nil {
		return nil, fmt.Errorf("bind APP_JWT_EXPIRY: %w", err)
	}

	v.SetDefault("jwt.expiry", 30)

	// =========================================================================
	// DATABASE
	// =========================================================================

	if err := v.BindEnv("database.host", "DB_HOST"); err != nil {
		return nil, fmt.Errorf("bind DB_HOST: %w", err)
	}

	if err := v.BindEnv("database.port", "DB_PORT"); err != nil {
		return nil, fmt.Errorf("bind DB_PORT: %w", err)
	}

	if err := v.BindEnv("database.name", "DB_NAME"); err != nil {
		return nil, fmt.Errorf("bind DB_NAME: %w", err)
	}

	if err := v.BindEnv("database.user", "DB_USER"); err != nil {
		return nil, fmt.Errorf("bind DB_USER: %w", err)
	}

	if err := v.BindEnv("database.password", "DB_PASSWORD"); err != nil {
		return nil, fmt.Errorf("bind DB_PASSWORD: %w", err)
	}

	// =========================================================================
	// MIGRATIONS
	// =========================================================================

	if err := v.BindEnv("migrations.auto", "MIGRATIONS_AUTO"); err != nil {
		return nil, fmt.Errorf("bind APP_MIGRATIONS_AUTO: %w", err)
	}

	if err := v.BindEnv("migrations.user", "MIGRATION_DB_USER"); err != nil {
		return nil, fmt.Errorf("bind MIGRATION_DB_USER: %w", err)
	}

	if err := v.BindEnv("migrations.password", "MIGRATION_DB_PASSWORD"); err != nil {
		return nil, fmt.Errorf("bind MIGRATION_DB_PASSWORD: %w", err)
	}

	v.SetDefault("migrations.auto", false)

	// =========================================================================
	// REDIS
	// =========================================================================

	if err := v.BindEnv("redis.addr", "REDIS_ADDR"); err != nil {
		return nil, fmt.Errorf("bind REDIS_ADDR: %w", err)
	}

	if err := v.BindEnv("redis.password", "REDIS_PASSWORD"); err != nil {
		return nil, fmt.Errorf("bind REDIS_PASSWORD: %w", err)
	}

	v.SetDefault("redis.addr", "redis:6379")

	// =========================================================================
	// UNMARSHAL
	// =========================================================================

	var cfg Config

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg.Migrations.Path = MigrationsPath

	return &cfg, nil
}

// Validate checks the application configuration.
func (c *Config) Validate() ValidationReport {
	var report ValidationReport

	// =========================================================================
	// APP
	// =========================================================================

	if strings.TrimSpace(c.App.Port) == "" {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "app.port",
			Message:  "APP_PORT is required",
		})
	} else {
		port, err := strconv.Atoi(c.App.Port)
		if err != nil || port < 1 || port > 65535 {
			report.Errors = append(report.Errors, ValidationError{
				Severity: SeverityFatal,
				Field:    "app.port",
				Message:  fmt.Sprintf("APP_PORT must be a valid TCP port (1-65535), got %q", c.App.Port),
			})
		}
	}

	if strings.TrimSpace(c.App.Secret) == "" {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "app.secret",
			Message:  "APP_JWT_SECRET is required",
		})
	} else if len(c.App.Secret) < 32 {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityWarning,
			Field:    "app.secret",
			Message:  "APP_JWT_SECRET is shorter than recommended (minimum 32 characters)",
		})
	}

	// =========================================================================
	// APP MODE
	// =========================================================================

	switch strings.ToLower(strings.TrimSpace(c.App.Mode)) {
	case "debug", "development", "dev", "release", "production", "prod":
		// Valid modes.
	default:
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityWarning,
			Field:    "app.mode",
			Message: fmt.Sprintf(
				"unknown APP_MODE %q, recommended values: debug, development, release, production",
				c.App.Mode,
			),
		})
	}

	// =========================================================================
	// LOGGING
	// =========================================================================

	switch strings.ToLower(strings.TrimSpace(c.Logging.Level)) {
	case "debug", "info", "warn", "warning", "error":
		// Valid levels.
	default:
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityWarning,
			Field:    "logging.level",
			Message: fmt.Sprintf(
				"unknown APP_LOGGING_LEVEL %q, recommended values: debug, info, warn, error",
				c.Logging.Level,
			),
		})
	}

	// =========================================================================
	// JWT
	// =========================================================================

	if c.JWT.Expiry <= 0 {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "jwt.expiry",
			Message:  "APP_JWT_EXPIRY must be greater than 0",
		})
	}

	// =========================================================================
	// DATABASE
	// =========================================================================

	if strings.TrimSpace(c.Database.Host) == "" {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "database.host",
			Message:  "DB_HOST is required",
		})
	}

	if c.Database.Port < 1 || c.Database.Port > 65535 {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "database.port",
			Message:  "DB_PORT must be between 1 and 65535",
		})
	}

	if strings.TrimSpace(c.Database.Name) == "" {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "database.name",
			Message:  "DB_NAME is required",
		})
	}

	if strings.TrimSpace(c.Database.User) == "" {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "database.user",
			Message:  "DB_USER is required",
		})
	}

	if strings.TrimSpace(c.Database.Password) == "" {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "database.password",
			Message:  "DB_PASSWORD is required",
		})
	}

	// =========================================================================
	// REDIS
	// =========================================================================

	if strings.TrimSpace(c.Redis.Addr) == "" {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "redis.addr",
			Message:  "REDIS_ADDR is required",
		})
	}

	if strings.TrimSpace(c.Redis.Password) == "" {
		report.Errors = append(report.Errors, ValidationError{
			Severity: SeverityFatal,
			Field:    "redis.password",
			Message:  "REDIS_PASSWORD is required because Redis is configured with requirepass",
		})
	}

	// =========================================================================
	// MIGRATIONS
	// =========================================================================

	if c.Migrations.Auto {
		if strings.TrimSpace(c.Migrations.User) == "" {
			report.Errors = append(report.Errors, ValidationError{
				Severity: SeverityFatal,
				Field:    "migrations.user",
				Message:  "MIGRATION_DB_USER is required when automatic migrations are enabled",
			})
		}

		if strings.TrimSpace(c.Migrations.Password) == "" {
			report.Errors = append(report.Errors, ValidationError{
				Severity: SeverityFatal,
				Field:    "migrations.password",
				Message:  "MIGRATION_DB_PASSWORD is required when automatic migrations are enabled",
			})
		}
	}

	return report
}

func (r ValidationReport) HasFatal() bool {
	for _, err := range r.Errors {
		if err.Severity == SeverityFatal {
			return true
		}
	}

	return false
}

func (r ValidationReport) Log(logger *zerolog.Logger) {
	for _, err := range r.Errors {
		event := logger.Warn()

		if err.Severity == SeverityFatal {
			event = logger.Error()
		}

		event.
			Str("field", err.Field).
			Str("message", err.Message).
			Msg("config validation")
	}
}

func (c *Config) JWTExpiration() time.Duration {
	return time.Duration(c.JWT.Expiry) * 24 * time.Hour
}
