//go:build integration
// +build integration

// internal/cache/tasks_integration_test.go
package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"

	"task-forge/internal/config"
)

func setupRedisContainer(t *testing.T) (*CacheService, context.Context) {
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

func TestTeamTasks_SetAndGet_Success(t *testing.T) {
	service, ctx := setupRedisContainer(t)

	type Task struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}

	teamID := "550e8400-e29b-41d4-a716-446655440000"
	status := "todo"
	assigneeID := "770e8400-e29b-41d4-a716-446655440002"
	limit := 20
	offset := 0

	originalTasks := []Task{
		{ID: "task-1", Title: "Task 1", Status: "todo"},
		{ID: "task-2", Title: "Task 2", Status: "todo"},
	}

	err := service.SetTeamTasks(ctx, teamID, &status, &assigneeID, limit, offset, originalTasks)
	require.NoError(t, err)

	var retrievedTasks []Task
	err = service.GetTeamTasks(ctx, teamID, &status, &assigneeID, limit, offset, &retrievedTasks)
	require.NoError(t, err)

	assert.Equal(t, originalTasks, retrievedTasks, "Retrieved tasks should match original")
}

func TestTeamTasks_GetCacheMiss(t *testing.T) {
	service, ctx := setupRedisContainer(t)

	teamID := "non-existent-team"
	status := "todo"
	limit := 20
	offset := 0

	var tasks []interface{}
	err := service.GetTeamTasks(ctx, teamID, &status, nil, limit, offset, &tasks)

	require.ErrorIs(t, err, redis.Nil, "Should return redis.Nil on cache miss")
	assert.Nil(t, tasks, "Tasks should be nil on cache miss")
}

func TestTeamTasks_NullableParameters(t *testing.T) {
	service, ctx := setupRedisContainer(t)

	teamID := "550e8400-e29b-41d4-a716-446655440000"
	limit := 10
	offset := 0

	originalTasks := []string{"task-1", "task-2"}

	err := service.SetTeamTasks(ctx, teamID, nil, nil, limit, offset, originalTasks)
	require.NoError(t, err)

	var retrievedTasks []string
	err = service.GetTeamTasks(ctx, teamID, nil, nil, limit, offset, &retrievedTasks)
	require.NoError(t, err)

	assert.Equal(t, originalTasks, retrievedTasks)

	var otherTasks []string
	status := "in_progress"
	err = service.GetTeamTasks(ctx, teamID, &status, nil, limit, offset, &otherTasks)
	require.ErrorIs(t, err, redis.Nil, "Different filter should result in cache miss")
}

func TestTeamTasks_Invalidate(t *testing.T) {
	service, ctx := setupRedisContainer(t)

	teamID := "550e8400-e29b-41d4-a716-446655440000"
	tasks := []string{"task-1"}

	statuses := []string{"todo", "in_progress", "done"}
	for _, status := range statuses {
		s := status
		err := service.SetTeamTasks(ctx, teamID, &s, nil, 20, 0, tasks)
		require.NoError(t, err)
	}

	for _, status := range statuses {
		s := status
		var retrieved []string
		err := service.GetTeamTasks(ctx, teamID, &s, nil, 20, 0, &retrieved)
		require.NoError(t, err, "Should retrieve task with status=%s", s)
		assert.Equal(t, tasks, retrieved)
	}

	err := service.InvalidateTeamTasks(ctx, teamID)
	require.NoError(t, err)

	for _, status := range statuses {
		s := status
		var retrieved []string
		err := service.GetTeamTasks(ctx, teamID, &s, nil, 20, 0, &retrieved)
		require.ErrorIs(t, err, redis.Nil, "Cache should be empty after invalidation for status=%s", s)
	}
}

func TestTeamTasks_Invalidate_DoesNotAffectOtherTeams(t *testing.T) {
	service, ctx := setupRedisContainer(t)

	team1 := "team-1"
	team2 := "team-2"
	status := "todo"
	tasks := []string{"task-1"}

	require.NoError(t, service.SetTeamTasks(ctx, team1, &status, nil, 20, 0, tasks))
	require.NoError(t, service.SetTeamTasks(ctx, team2, &status, nil, 20, 0, tasks))


	require.NoError(t, service.InvalidateTeamTasks(ctx, team1))

	var retrieved1 []string
	err := service.GetTeamTasks(ctx, team1, &status, nil, 20, 0, &retrieved1)
	require.ErrorIs(t, err, redis.Nil)

	var retrieved2 []string
	err = service.GetTeamTasks(ctx, team2, &status, nil, 20, 0, &retrieved2)
	require.NoError(t, err)
	assert.Equal(t, tasks, retrieved2, "Other team's cache should not be affected")
}


func TestTeamTasks_TTL(t *testing.T) {
	service, ctx := setupRedisContainer(t)

	teamID := "test-team"
	status := "todo"
	tasks := []string{"task-1"}

	require.NoError(t, service.SetTeamTasks(ctx, teamID, &status, nil, 20, 0, tasks))
	
	key := teamTasksKey(teamID, &status, nil, 20, 0)
	ttl, err := service.Client().TTL(ctx, key).Result()
	require.NoError(t, err)

	assert.InDelta(t, TeamTasksTTL.Seconds(), ttl.Seconds(), 2.0,
		"TTL should be approximately %v", TeamTasksTTL)
	assert.Greater(t, ttl, time.Duration(0), "TTL should be positive")
}