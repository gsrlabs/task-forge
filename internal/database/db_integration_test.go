//go:build integration
// +build integration

// internal/database/db_integration_test.go
package database

import (
	"context"
	"testing"

	"task-forge/internal/config"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestOpenPostgres_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := t.Context()
	logger := zerolog.Nop()

	// Setup PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections"),
				wait.ForListeningPort("5432/tcp"),
			),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, postgresContainer.Terminate(context.Background()))
	})

	// Get connection details
	host, err := postgresContainer.Host(ctx)
	require.NoError(t, err)

	port, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	// Test configuration
	cfg := config.DatabaseConfig{
		Host:     host,
		Port:     int(port.Num()),
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	// Test OpenPostgres
	pool, err := OpenPostgres(ctx, cfg, "debug", logger)
	require.NoError(t, err, "OpenPostgres should succeed with valid credentials")
	require.NotNil(t, pool, "Pool should not be nil")
	defer pool.Close()

	// Verify connection works
	err = pool.Ping(ctx)
	require.NoError(t, err, "Pool should be able to ping database")
}

func TestOpenPostgres_InvalidCredentials_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := t.Context()
	logger := zerolog.Nop()

	// Setup PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections"),
				wait.ForListeningPort("5432/tcp"),
			),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, postgresContainer.Terminate(context.Background()))
	})

	host, err := postgresContainer.Host(ctx)
	require.NoError(t, err)

	port, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	// Invalid credentials
	cfg := config.DatabaseConfig{
		Host:     host,
		Port:     int(port.Num()),
		Name:     "testdb",
		User:     "wronguser",
		Password: "wrongpass",
	}

	// Should fail
	pool, err := OpenPostgres(ctx, cfg, "debug", logger)
	require.Error(t, err, "OpenPostgres should fail with invalid credentials")
	require.Nil(t, pool, "Pool should be nil on error")
	require.Contains(t, err.Error(), "failed to ping postgres")
}
