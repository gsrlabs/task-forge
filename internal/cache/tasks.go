// internal/cache/tasks.go
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

func teamTasksKey(
	teamID string,
	status *string,
	assigneeID *string,
	limit, offset int,
) string {
	statusValue := ""
	if status != nil {
		statusValue = *status
	}

	assigneeValue := ""
	if assigneeID != nil {
		assigneeValue = *assigneeID
	}

	return fmt.Sprintf(
		"team:%s:tasks:status=%s:assignee=%s:limit=%d:offset=%d",
		teamID,
		statusValue,
		assigneeValue,
		limit,
		offset,
	)
}

// SetTeamTasks saves the team's task list in Redis.
func (c *CacheService) SetTeamTasks(
	ctx context.Context,
	teamID string,
	status *string,
	assigneeID *string,
	limit, offset int,
	tasks any,
) error {
	data, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("marshal tasks for cache: %w", err)
	}

	key := teamTasksKey(teamID, status, assigneeID, limit, offset)

	if err := c.client.Set(ctx, key, data, TeamTasksTTL).Err(); err != nil {
		return fmt.Errorf("set tasks to cache: %w", err)
	}

	c.logger.Debug().
		Str("team_id", teamID).
		Msg("Tasks cached successfully")

	return nil
}

// GetTeamTasks extracts the list of team tasks from Redis.
// Returns redis.Nil if the data is missing from the cache (cache miss).
func (c *CacheService) GetTeamTasks(
	ctx context.Context,
	teamID string,
	status *string,
	assigneeID *string,
	limit, offset int,
	dest any,
) error {
	key := teamTasksKey(teamID, status, assigneeID, limit, offset)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return redis.Nil
		}

		return fmt.Errorf("get tasks from cache: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshal tasks from cache: %w", err)
	}

	c.logger.Debug().
		Str("team_id", teamID).
		Msg("Tasks retrieved from cache")

	return nil
}

// InvalidateTeamTasks removes the team task cache (triggered when a task is created/updated).
func (c *CacheService) InvalidateTeamTasks(
	ctx context.Context,
	teamID string,
) error {
	pattern := fmt.Sprintf("team:%s:tasks:*", teamID)

	var cursor uint64

	for {
		keys, nextCursor, err := c.client.Scan(
			ctx,
			cursor,
			pattern,
			100,
		).Result()
		if err != nil {
			return fmt.Errorf("scan team task cache keys: %w", err)
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("delete team task cache keys: %w", err)
			}
		}

		cursor = nextCursor

		if cursor == 0 {
			break
		}
	}

	c.logger.Debug().
		Str("team_id", teamID).
		Msg("Tasks cache invalidated")

	return nil
}
