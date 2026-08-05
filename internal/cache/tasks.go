package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// TeamTasksTTL the cache lifetime of the command task list.
	TeamTasksTTL = 5 * time.Minute
)

// SetTeamTasks saves the team's task list in Redis.
func (c *CacheService) SetTeamTasks(ctx context.Context, teamID string, tasks any) error {
	data, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("marshal tasks for cache: %w", err)
	}

	key := fmt.Sprintf("team:%s:tasks", teamID)
	
	if err := c.client.Set(ctx, key, data, TeamTasksTTL).Err(); err != nil {
		return fmt.Errorf("set tasks to cache: %w", err)
	}

	c.logger.Debug().Str("team_id", teamID).Msg("Tasks cached successfully")
	return nil
}

// GetTeamTasks extracts the list of team tasks from Redis.
// Returns redis.Nil if the data is missing from the cache (cache miss).
func (c *CacheService) GetTeamTasks(ctx context.Context, teamID string, dest any) error {
	key := fmt.Sprintf("team:%s:tasks", teamID)
	
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return redis.Nil // Cache miss - это нормальная ситуация
		}
		return fmt.Errorf("get tasks from cache: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshal tasks from cache: %w", err)
	}

	c.logger.Debug().Str("team_id", teamID).Msg("Tasks retrieved from cache")
	return nil
}

// InvalidateTeamTasks removes the team task cache (triggered when a task is created/updated).
func (c *CacheService) InvalidateTeamTasks(ctx context.Context, teamID string) error {
	key := fmt.Sprintf("team:%s:tasks", teamID)
	
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("invalidate tasks cache: %w", err)
	}
	
	c.logger.Debug().Str("team_id", teamID).Msg("Tasks cache invalidated")
	return nil
}