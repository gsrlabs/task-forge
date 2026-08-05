// internal/cache/ratelimit.go
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter describes the contract for checking request limits.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// rateLimitScript atomically increments the counter and sets the TTL only on the first access.
var rateLimitScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
`)

// Allow It checks whether the request limit for the given key has been exceeded.
func (c *CacheService) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	current, err := rateLimitScript.Run(ctx, c.client, []string{key}, int(window.Seconds())).Int()
	if err != nil {
		return false, fmt.Errorf("rate limit script failed: %w", err)
	}
	
	return current <= limit, nil
}