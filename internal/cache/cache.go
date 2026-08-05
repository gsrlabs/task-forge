// internal/cache/cache.go
package cache

import (
	"context"
	"fmt"
	"time"

	"task-forge/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// CacheService encapsulates the work with Redis.
type CacheService struct {
	client *redis.Client
	logger zerolog.Logger
}

// NewCacheService creates and verifies a connection to Redis.
func NewCacheService(cfg config.RedisConfig, logger zerolog.Logger) (*CacheService, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           0,
		PoolSize:     20,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	logger.Info().Str("addr", cfg.Addr).Msg("Connected to Redis")

	return &CacheService{
		client: client,
		logger: logger,
	}, nil
}

func (c *CacheService) Close() error {
	return c.client.Close()
}

func (c *CacheService) Client() *redis.Client {
	return c.client
}