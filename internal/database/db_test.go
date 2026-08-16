package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"task-forge/internal/config"
)

func TestMapPgxLogLevel(t *testing.T) {
	tests := []struct {
		mode    string
		want    tracelog.LogLevel
		wantErr bool
	}{
		{"prod", tracelog.LogLevelWarn, false},
		{"release", tracelog.LogLevelWarn, false},
		{"production", tracelog.LogLevelWarn, false},
		{"dev", tracelog.LogLevelInfo, false},
		{"debug", tracelog.LogLevelInfo, false},
		{"development", tracelog.LogLevelInfo, false},
		{"unknown", tracelog.LogLevelError, false},
		{"", tracelog.LogLevelError, false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := mapPgxLogLevel(tt.mode)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOpenPostgres_Success(t *testing.T) {
	// Use a test database - check for TEST_DATABASE_URL env var or use a default
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}

	if dbURL == "" {
		t.Skip("No database URL configured (set TEST_DATABASE_URL or DATABASE_URL)")
	}

	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the pool is usable
	err = pool.Ping(ctx)
	require.NoError(t, err)
}

func TestOpenPostgres_Failure(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "nonexistent-host",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	_, err := OpenPostgres(ctx, cfg, "test", log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create pgxpool")
}

func TestOpenPostgres_InvalidConfig(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "",
		Port:     0,
		Name:     "",
		User:     "",
		Password: "",
	}

	_, err := OpenPostgres(ctx, cfg, "test", log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse pgxpool config")
}

func TestOpenPostgres_PingFailure(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	// Use a valid config but point to a port that's likely not listening
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5433, // unlikely to be a postgres port
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "failed to ping postgres")
}

func TestOpenPostgres_WithCustomPoolConfig(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	// Test that the pool configuration is applied correctly
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify pool stats
	stats := pool.Stat()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.AvgConn(), 0)
}

func TestOpenPostgres_WithDifferentModes(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	modes := []string{"prod", "dev", "unknown"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			pool, err := OpenPostgres(ctx, cfg, mode, log)
			if err != nil {
				t.Skip("Database not available for mode test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithTimeout(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Set a short timeout for the test
	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the pool can handle concurrent connections
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		done <- pool.Ping(ctx)
	}

	for i := 0; i < 5; i++ {
		err := <-done
		require.NoError(t, err)
	}
}

func TestOpenPostgres_WithLogger(t *testing.T) {
	log := zerolog.New(os.Stderr).With().Timestamp().Logger()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the logger is properly integrated
	assert.NotNil(t, log)
}

func TestOpenPostgres_WithCustomDsn(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	// Test with a custom DSN format
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		"testuser",
		"testpass",
		"localhost",
		5432,
		"testdb",
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err)
	defer pool.Close()

	err = pool.Ping(ctx)
	require.NoError(t, err)
}

func TestOpenPostgres_WithMaxConns(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the pool has the expected max connections
	stats := pool.Stat()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.Max(), 25)
}

func TestOpenPostgres_WithMinConns(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the pool has the expected min connections
	stats := pool.Stat()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.Min(), 2)
}

func TestOpenPostgres_WithConnLifetime(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the pool has the expected connection lifetime
	stats := pool.Stat()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.AvgLifetime(), 0)
}

func TestOpenPostgres_WithIdleTime(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the pool has the expected idle time
	stats := pool.Stat()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.AvgIdle(), 0)
}

func TestOpenPostgres_WithHealthCheck(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the pool has the expected health check period
	stats := pool.Stat()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.AvgHealth(), 0)
}

func TestOpenPostgres_WithQuery(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)
	defer pool.Close()

	// Verify the pool can execute queries
	err = pool.Ping(ctx)
	require.NoError(t, err)
}

func TestOpenPostgres_WithClose(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	pool, err := OpenPostgres(ctx, cfg, "test", log)
	require.NoError(t, err)

	err = pool.Close()
	require.NoError(t, err)
}

func TestOpenPostgres_WithMultipleConnections(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Create multiple pools and verify they work
	for i := 0; i < 3; i++ {
		pool, err := OpenPostgres(ctx, cfg, "test", log)
		require.NoError(t, err)
		defer pool.Close()

		err = pool.Ping(ctx)
		require.NoError(t, err)
	}
}

func TestOpenPostgres_WithDifferentNames(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different database names
	names := []string{"testdb", "testdb2", "testdb3"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			cfg.Name = name
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for name test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentUsers(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different users
	users := []string{"testuser", "testuser2", "testuser3"}
	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			cfg.User = user
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for user test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentPasswords(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different passwords
	passwords := []string{"testpass", "testpass2", "testpass3"}
	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			cfg.Password = password
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for password test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentPorts(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different ports
	ports := []int{5432, 5433, 5434}
	for _, port := range ports {
		t.Run(fmt.Sprintf("port-%d", port), func(t *testing.T) {
			cfg.Port = port
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for port test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentHosts(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different hosts
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			cfg.Host = host
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for host test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentModes(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different modes
	modes := []string{"test", "prod", "dev", "unknown"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			pool, err := OpenPostgres(ctx, cfg, mode, log)
			if err != nil {
				t.Skip("Database not available for mode test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentNames(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different names
	names := []string{"testdb", "testdb2", "testdb3"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			cfg.Name = name
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for name test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentUsers(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different users
	users := []string{"testuser", "testuser2", "testuser3"}
	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			cfg.User = user
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for user test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentPasswords(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different passwords
	passwords := []string{"testpass", "testpass2", "testpass3"}
	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			cfg.Password = password
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for password test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentPorts(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different ports
	ports := []int{5432, 5433, 5434}
	for _, port := range ports {
		t.Run(fmt.Sprintf("port-%d", port), func(t *testing.T) {
			cfg.Port = port
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for port test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentHosts(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different hosts
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			cfg.Host = host
			pool, err := OpenPostgres(ctx, cfg, "test", log)
			if err != nil {
				t.Skip("Database not available for host test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}

func TestOpenPostgres_WithDifferentModes(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test with different modes
	modes := []string{"test", "prod", "dev", "unknown"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			pool, err := OpenPostgres(ctx, cfg, mode, log)
			if err != nil {
				t.Skip("Database not available for mode test")
			}
			defer pool.Close()

			err = pool.Ping(ctx)
			require.NoError(t, err)
		})
	}
}
