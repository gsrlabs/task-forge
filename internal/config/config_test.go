// internal/config/config_test.go
package config

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	validConfig := func() *Config {
		return &Config{
			App: AppConfig{
				Port:          "8080",
				Mode:          "debug",
				EncryptionKey: "this-is-a-32-character-long-key!",
			},
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "user",
				Password: "password",
			},
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "redispass",
			},
			JWT: JWTConfig{
				Secret: "this-is-a-32-character-long-secret!",
				Expiry: 24,
			},
			Migrations: MigrationConfig{
				Auto: false,
			},
			Logging: LoggingConfig{
				Level: "info",
			},
		}
	}

	tests := []struct {
		name           string
		modifyConfig   func(*Config)
		expectedFatal  bool
		expectedErrors int
		errorFields    []string
	}{
		{
			name:           "valid config - no errors",
			modifyConfig:   func(c *Config) {},
			expectedFatal:  false,
			expectedErrors: 0,
		},

		{
			name: "missing app.port - fatal",
			modifyConfig: func(c *Config) {
				c.App.Port = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"app.port"},
		},
		{
			name: "invalid app.port - not a number",
			modifyConfig: func(c *Config) {
				c.App.Port = "abc"
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"app.port"},
		},
		{
			name: "invalid app.port - out of range (0)",
			modifyConfig: func(c *Config) {
				c.App.Port = "0"
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"app.port"},
		},
		{
			name: "invalid app.port - out of range (70000)",
			modifyConfig: func(c *Config) {
				c.App.Port = "70000"
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"app.port"},
		},
		{
			name: "missing app.encryption_key - fatal",
			modifyConfig: func(c *Config) {
				c.App.EncryptionKey = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"app.encryption_key"},
		},
		{
			name: "short app.encryption_key - warning",
			modifyConfig: func(c *Config) {
				c.App.EncryptionKey = "short-key"
			},
			expectedFatal:  false,
			expectedErrors: 1,
			errorFields:    []string{"app.encryption_key"},
		},
		{
			name: "unknown app.mode - warning",
			modifyConfig: func(c *Config) {
				c.App.Mode = "unknown"
			},
			expectedFatal:  false,
			expectedErrors: 1,
			errorFields:    []string{"app.mode"},
		},
		{
			name: "valid app.mode - debug",
			modifyConfig: func(c *Config) {
				c.App.Mode = "debug"
			},
			expectedFatal:  false,
			expectedErrors: 0,
		},
		{
			name: "valid app.mode - production",
			modifyConfig: func(c *Config) {
				c.App.Mode = "production"
			},
			expectedFatal:  false,
			expectedErrors: 0,
		},

		{
			name: "unknown logging.level - warning",
			modifyConfig: func(c *Config) {
				c.Logging.Level = "unknown"
			},
			expectedFatal:  false,
			expectedErrors: 1,
			errorFields:    []string{"logging.level"},
		},
		{
			name: "valid logging.level - debug",
			modifyConfig: func(c *Config) {
				c.Logging.Level = "debug"
			},
			expectedFatal:  false,
			expectedErrors: 0,
		},

		{
			name: "missing jwt.secret - fatal",
			modifyConfig: func(c *Config) {
				c.JWT.Secret = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"jwt.secret"},
		},
		{
			name: "short jwt.secret - warning",
			modifyConfig: func(c *Config) {
				c.JWT.Secret = "short"
			},
			expectedFatal:  false,
			expectedErrors: 1,
			errorFields:    []string{"jwt.secret"},
		},
		{
			name: "invalid jwt.expiry - zero",
			modifyConfig: func(c *Config) {
				c.JWT.Expiry = 0
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"jwt.expiry"},
		},
		{
			name: "invalid jwt.expiry - negative",
			modifyConfig: func(c *Config) {
				c.JWT.Expiry = -1
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"jwt.expiry"},
		},

		{
			name: "missing database.host - fatal",
			modifyConfig: func(c *Config) {
				c.Database.Host = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"database.host"},
		},
		{
			name: "invalid database.port - zero",
			modifyConfig: func(c *Config) {
				c.Database.Port = 0
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"database.port"},
		},
		{
			name: "invalid database.port - out of range",
			modifyConfig: func(c *Config) {
				c.Database.Port = 70000
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"database.port"},
		},
		{
			name: "missing database.name - fatal",
			modifyConfig: func(c *Config) {
				c.Database.Name = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"database.name"},
		},
		{
			name: "missing database.user - fatal",
			modifyConfig: func(c *Config) {
				c.Database.User = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"database.user"},
		},
		{
			name: "missing database.password - fatal",
			modifyConfig: func(c *Config) {
				c.Database.Password = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"database.password"},
		},

		{
			name: "missing redis.addr - fatal",
			modifyConfig: func(c *Config) {
				c.Redis.Addr = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"redis.addr"},
		},
		{
			name: "missing redis.password - fatal",
			modifyConfig: func(c *Config) {
				c.Redis.Password = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"redis.password"},
		},

		{
			name: "migrations.auto=false - no credentials required",
			modifyConfig: func(c *Config) {
				c.Migrations.Auto = false
				c.Migrations.User = ""
				c.Migrations.Password = ""
			},
			expectedFatal:  false,
			expectedErrors: 0,
		},
		{
			name: "migrations.auto=true - missing user - fatal",
			modifyConfig: func(c *Config) {
				c.Migrations.Auto = true
				c.Migrations.User = ""
				c.Migrations.Password = "password"
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"migrations.user"},
		},
		{
			name: "migrations.auto=true - missing password - fatal",
			modifyConfig: func(c *Config) {
				c.Migrations.Auto = true
				c.Migrations.User = "user"
				c.Migrations.Password = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"migrations.password"},
		},
		{
			name: "migrations.auto=true - both credentials present",
			modifyConfig: func(c *Config) {
				c.Migrations.Auto = true
				c.Migrations.User = "user"
				c.Migrations.Password = "password"
			},
			expectedFatal:  false,
			expectedErrors: 0,
		},

		{
			name: "email.mode=mailtrap - missing api_key - fatal",
			modifyConfig: func(c *Config) {
				c.Email.Mode = "mailtrap"
				c.Mailtrap.APIKey = ""
				c.Mailtrap.FromEmail = "test@example.com"
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"mailtrap.api_key"},
		},
		{
			name: "email.mode=mailtrap - missing from_email - fatal",
			modifyConfig: func(c *Config) {
				c.Email.Mode = "mailtrap"
				c.Mailtrap.APIKey = "api-key"
				c.Mailtrap.FromEmail = ""
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"email.from_email"},
		},
		{
			name: "email.mode=mailtrap - both present",
			modifyConfig: func(c *Config) {
				c.Email.Mode = "mailtrap"
				c.Mailtrap.APIKey = "api-key"
				c.Mailtrap.FromEmail = "test@example.com"
			},
			expectedFatal:  false,
			expectedErrors: 0,
		},

		{
			name: "email.mode=smtp - missing host - fatal",
			modifyConfig: func(c *Config) {
				c.Email.Mode = "smtp"
				c.SMTP.Host = ""
				c.SMTP.Port = 587
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"smtp.host"},
		},
		{
			name: "email.mode=smtp - invalid port",
			modifyConfig: func(c *Config) {
				c.Email.Mode = "smtp"
				c.SMTP.Host = "smtp.example.com"
				c.SMTP.Port = 70000
			},
			expectedFatal:  true,
			expectedErrors: 1,
			errorFields:    []string{"smtp.port"},
		},
		{
			name: "email.mode=smtp - valid config",
			modifyConfig: func(c *Config) {
				c.Email.Mode = "smtp"
				c.SMTP.Host = "smtp.example.com"
				c.SMTP.Port = 587
			},
			expectedFatal:  false,
			expectedErrors: 0,
		},

		{
			name: "multiple fatal errors",
			modifyConfig: func(c *Config) {
				c.App.Port = ""
				c.App.EncryptionKey = ""
				c.Database.Host = ""
			},
			expectedFatal:  true,
			expectedErrors: 3,
			errorFields:    []string{"app.port", "app.encryption_key", "database.host"},
		},
		{
			name: "mixed fatal and warning errors",
			modifyConfig: func(c *Config) {
				c.App.Port = ""                          // Fatal
				c.App.EncryptionKey = "short"            // Warning
				c.App.Mode = "unknown"                   // Warning
			},
			expectedFatal:  true,
			expectedErrors: 3,
			errorFields:    []string{"app.port", "app.encryption_key", "app.mode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modifyConfig(cfg)

			report := cfg.Validate()

			assert.Equal(t, tt.expectedFatal, report.HasFatal(), "HasFatal() mismatch")

			assert.Len(t, report.Errors, tt.expectedErrors, "Expected %d errors", tt.expectedErrors)

			if len(tt.errorFields) > 0 {
				actualFields := make([]string, len(report.Errors))
				for i, err := range report.Errors {
					actualFields[i] = err.Field
				}

				for _, expectedField := range tt.errorFields {
					assert.Contains(t, actualFields, expectedField,
						"Expected error for field %q", expectedField)
				}
			}
		})
	}
}

func TestValidationReport_HasFatal(t *testing.T) {
	tests := []struct {
		name     string
		errors   []ValidationError
		expected bool
	}{
		{
			name:     "no errors",
			errors:   []ValidationError{},
			expected: false,
		},
		{
			name: "only warnings",
			errors: []ValidationError{
				{Severity: SeverityWarning, Field: "field1"},
				{Severity: SeverityWarning, Field: "field2"},
			},
			expected: false,
		},
		{
			name: "has fatal error",
			errors: []ValidationError{
				{Severity: SeverityWarning, Field: "field1"},
				{Severity: SeverityFatal, Field: "field2"},
			},
			expected: true,
		},
		{
			name: "only fatal errors",
			errors: []ValidationError{
				{Severity: SeverityFatal, Field: "field1"},
				{Severity: SeverityFatal, Field: "field2"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := ValidationReport{Errors: tt.errors}
			assert.Equal(t, tt.expected, report.HasFatal())
		})
	}
}

func TestValidationReport_Log(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name   string
		errors []ValidationError
	}{
		{
			name:   "no errors",
			errors: []ValidationError{},
		},
		{
			name: "warning error",
			errors: []ValidationError{
				{Severity: SeverityWarning, Field: "field1", Message: "warning message"},
			},
		},
		{
			name: "fatal error",
			errors: []ValidationError{
				{Severity: SeverityFatal, Field: "field2", Message: "fatal message"},
			},
		},
		{
			name: "mixed errors",
			errors: []ValidationError{
				{Severity: SeverityWarning, Field: "field1", Message: "warning"},
				{Severity: SeverityFatal, Field: "field2", Message: "fatal"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := ValidationReport{Errors: tt.errors}
			require.NotPanics(t, func() {
				report.Log(&logger)
			})
		})
	}
}

func TestConfig_JWTExpiration(t *testing.T) {
	tests := []struct {
		name     string
		expiry   int
		expected time.Duration
	}{
		{
			name:     "1 day",
			expiry:   1,
			expected: 24 * time.Hour,
		},
		{
			name:     "30 days",
			expiry:   30,
			expected: 30 * 24 * time.Hour,
		},
		{
			name:     "365 days",
			expiry:   365,
			expected: 365 * 24 * time.Hour,
		},
		{
			name:     "zero (edge case)",
			expiry:   0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				JWT: JWTConfig{
					Expiry: tt.expiry,
				},
			}

			result := cfg.JWTExpiration()
			assert.Equal(t, tt.expected, result)
		})
	}
}