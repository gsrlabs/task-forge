//go:build integration
// +build integration

// internal/database/migrate_test.go
package database

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"task-forge/internal/config"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRunMigrations_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := t.Context()

	postgresContainer, err := postgres.Run(
		ctx,
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
	
	createTestRoles(t, ctx, postgresContainer)
	
	host, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	
	port, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)
	
	cfg := config.DatabaseConfig{
		Host: host,
		Port: int(port.Num()),
		Name: "testdb",
	}
	
	migrationsCfg := config.MigrationConfig{
		User:     "taskforge_migrator",
		Password: "migrator_dev_password",
		Path:     migrationsPath(t),
	}
	
	require.NoError(t, RunMigrations(
		cfg,
		migrationsCfg,
		zerolog.Nop(),
	))
}

func createTestRoles(
	t *testing.T,
	ctx context.Context,
	container *postgres.PostgresContainer,
) {
	t.Helper()

	_, _, err := container.Exec(
		ctx,
		[]string{
			"psql",
			"-U", "testuser",
			"-d", "testdb",
			"-c", `
				CREATE ROLE taskforge_migrator
					WITH LOGIN
					PASSWORD 'migrator_dev_password';

				CREATE ROLE taskforge_app
					WITH LOGIN
					PASSWORD 'postgres_dev_password';

				GRANT USAGE, CREATE
					ON SCHEMA public
					TO taskforge_migrator;

				CREATE EXTENSION IF NOT EXISTS citext;
			`,
		},
	)

	require.NoError(t, err)
}

func migrationsPath(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "Failed to get current file path")

	return filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"migrations",
	)
}