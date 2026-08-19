//go:build integration
// +build integration

// internal/cache/ratelimit_integration_test.go
package cache

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"

	"task-forge/internal/config"
)

func setupRedisForRateLimit(t *testing.T) (*CacheService, context.Context) {
	t.Helper()

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

	t.Cleanup(func() {
		require.NoError(t, service.Close())
	})

	return service, ctx
}

func TestRateLimiter_Allow_FirstRequest(t *testing.T) {
	service, ctx := setupRedisForRateLimit(t)

	key := "test:ratelimit:first"
	limit := 5
	window := 1 * time.Minute

	allowed, err := service.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.True(t, allowed, "First request should be allowed")
}

func TestRateLimiter_Allow_UntilLimit(t *testing.T) {
	service, ctx := setupRedisForRateLimit(t)

	key := "test:ratelimit:until"
	limit := 5
	window := 1 * time.Minute

	for i := 1; i <= limit; i++ {
		allowed, err := service.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed, "Request %d should be allowed", i)
	}
}

func TestRateLimiter_Allow_ExceedLimit(t *testing.T) {
	service, ctx := setupRedisForRateLimit(t)

	key := "test:ratelimit:exceed"
	limit := 3
	window := 1 * time.Minute

	for i := 0; i < limit; i++ {
		allowed, err := service.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	allowed, err := service.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.False(t, allowed, "Request beyond limit should be denied")
}

func TestRateLimiter_Allow_TTLSetOnFirstRequest(t *testing.T) {
	service, ctx := setupRedisForRateLimit(t)

	key := "test:ratelimit:ttl"
	limit := 10
	window := 60 * time.Second

	_, err := service.Allow(ctx, key, limit, window)
	require.NoError(t, err)

	ttl, err := service.Client().TTL(ctx, key).Result()
	require.NoError(t, err)

	assert.InDelta(t, window.Seconds(), ttl.Seconds(), 2.0,
		"TTL should be approximately %v", window)
	assert.Greater(t, ttl, time.Duration(0), "TTL should be positive")
}

func TestRateLimiter_Allow_DifferentKeys(t *testing.T) {
	service, ctx := setupRedisForRateLimit(t)

	key1 := "test:ratelimit:key1"
	key2 := "test:ratelimit:key2"
	limit := 2
	window := 1 * time.Minute

	for i := 0; i < limit; i++ {
		allowed, err := service.Allow(ctx, key1, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	allowed, err := service.Allow(ctx, key1, limit, window)
	require.NoError(t, err)
	assert.False(t, allowed, "key1 should be rate limited")

	allowed, err = service.Allow(ctx, key2, limit, window)
	require.NoError(t, err)
	assert.True(t, allowed, "key2 should still be allowed")
}

func TestRateLimiter_Allow_ResetAfterTTL(t *testing.T) {
	service, ctx := setupRedisForRateLimit(t)

	key := "test:ratelimit:reset"
	limit := 2
	window := 2 * time.Second

	for range limit {
		allowed, err := service.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	allowed, err := service.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.False(t, allowed, "Should be rate limited")

	time.Sleep(window + 500*time.Millisecond)

	allowed, err = service.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.True(t, allowed, "Should be allowed after TTL expires")
}

func TestRateLimiter_Allow_ConcurrentRequests(t *testing.T) {
	service, ctx := setupRedisForRateLimit(t)

	key := "test:ratelimit:concurrent"
	limit := 10
	window := 1 * time.Minute
	goroutines := 20

	results := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			allowed, err := service.Allow(ctx, key, limit, window)
			if err != nil {
				results <- false
				return
			}
			results <- allowed
		}()
	}

	allowedCount := 0
	deniedCount := 0
	for i := 0; i < goroutines; i++ {
		if <-results {
			allowedCount++
		} else {
			deniedCount++
		}
	}

	assert.Equal(t, limit, allowedCount, "Exactly %d requests should be allowed", limit)
	assert.Equal(t, goroutines-limit, deniedCount, "Remaining requests should be denied")
	assert.Equal(t, goroutines, allowedCount+deniedCount, "All requests should complete")
}