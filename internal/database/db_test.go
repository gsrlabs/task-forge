// internal/database/db_test.go
package database

import (
	"testing"

	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/assert"
)

func TestMapPgxLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected tracelog.LogLevel
	}{
		// Production modes → Warn
		{
			name:     "prod mode returns Warn",
			mode:     "prod",
			expected: tracelog.LogLevelWarn,
		},
		{
			name:     "release mode returns Warn",
			mode:     "release",
			expected: tracelog.LogLevelWarn,
		},
		{
			name:     "production mode returns Warn",
			mode:     "production",
			expected: tracelog.LogLevelWarn,
		},

		// Development modes → Info
		{
			name:     "dev mode returns Info",
			mode:     "dev",
			expected: tracelog.LogLevelInfo,
		},
		{
			name:     "debug mode returns Info",
			mode:     "debug",
			expected: tracelog.LogLevelInfo,
		},
		{
			name:     "development mode returns Info",
			mode:     "development",
			expected: tracelog.LogLevelInfo,
		},

		// Unknown modes → Error (default)
		{
			name:     "unknown mode returns Error",
			mode:     "staging",
			expected: tracelog.LogLevelError,
		},
		{
			name:     "empty string returns Error",
			mode:     "",
			expected: tracelog.LogLevelError,
		},
		{
			name:     "invalid mode returns Error",
			mode:     "invalid",
			expected: tracelog.LogLevelError,
		},
		{
			name:     "case sensitive - Prod returns Error",
			mode:     "Prod",
			expected: tracelog.LogLevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapPgxLogLevel(tt.mode)
			assert.Equal(t, tt.expected, result,
				"mapPgxLogLevel(%q) should return %v", tt.mode, tt.expected)
		})
	}
}
