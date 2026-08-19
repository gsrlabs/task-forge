//go:build integration
// +build integration

// internal/cache/cache_integration_test.go
package cache

import (
	"context"
	"testing"

	"task-forge/internal/config"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestNewCacheService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := t.Context()

	container, err := rediscontainer.Run(ctx, "redis:8-alpine")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, container.Terminate(context.Background()))
	})

	endpoint, err := container.PortEndpoint(ctx, "6379/tcp", "")
	require.NoError(t, err)

	cfg := config.RedisConfig{
		Addr: endpoint,
	}

	service, err := NewCacheService(cfg, zerolog.Nop())
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Cleanup(func() {
		require.NoError(t, service.Close())
	})

	require.NotNil(t, service.Client())
	require.NoError(t, service.Client().Ping(ctx).Err())
}

func TestNewCacheService_InvalidAddress_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.RedisConfig{
		Addr: "localhost:59999",
	}

	service, err := NewCacheService(cfg, zerolog.Nop())
	require.Error(t, err)
	require.Nil(t, service)
	require.Contains(t, err.Error(), "redis ping failed")
}

func TestCacheService_Close_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := t.Context()

	container, err := rediscontainer.Run(ctx, "redis:8-alpine")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, container.Terminate(context.Background()))
	})

	endpoint, err := container.PortEndpoint(ctx, "6379/tcp", "")
	require.NoError(t, err)

	cfg := config.RedisConfig{
		Addr: endpoint,
	}

	service, err := NewCacheService(cfg, zerolog.Nop())
	require.NoError(t, err)

	require.NoError(t, service.Close())

	err = service.Client().Ping(ctx).Err()
	require.Error(t, err)
}