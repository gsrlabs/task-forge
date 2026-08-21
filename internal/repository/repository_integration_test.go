//go:build integration
// +build integration

// internal/repository/repository_integration_test.go
package repository

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"task-forge/internal/config"
	"task-forge/internal/database"
	"task-forge/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testDBName     = "testdb"
	testDBUser     = "testuser"
	testDBPassword = "testpass"

	testMigratorUser     = "taskforge_migrator"
	testMigratorPassword = "migrator_dev_password"

	testAppUser     = "taskforge_app"
	testAppPassword = "postgres_dev_password"
)

type testEnv struct {
	pool          *pgxpool.Pool
	userRepo      UserRepository
	teamRepo      TeamRepository
	taskRepo      TaskRepository
	analyticsRepo AnalyticsRepository
	ctx           context.Context
	logger        zerolog.Logger
}

func setupPostgres(t *testing.T) *testEnv {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := t.Context()
	logger := zerolog.Nop()

	pgContainer, err := postgres.Run(
		ctx,
		"postgres:18-alpine",
		postgres.WithDatabase(testDBName),
		postgres.WithUsername(testDBUser),
		postgres.WithPassword(testDBPassword),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections"),
				wait.ForListeningPort("5432/tcp"),
			),
		),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	t.Cleanup(func() {
		require.NoError(
			t,
			pgContainer.Terminate(context.Background()),
		)
	})

	createDatabaseRoles(t, ctx, pgContainer)

	host, err := pgContainer.Host(ctx)
	require.NoError(t, err)

	port, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dbCfg := config.DatabaseConfig{
		Host: host,
		Port: int(port.Num()),
		Name: testDBName,
	}

	migrationsCfg := config.MigrationConfig{
		Auto:     true,
		User:     testMigratorUser,
		Password: testMigratorPassword,
		Path:     migrationsPath(t),
	}

	require.NoError(
		t,
		database.RunMigrations(
			dbCfg,
			migrationsCfg,
			logger,
		),
		"Failed to run migrations",
	)

	appDBCfg := dbCfg
	appDBCfg.User = testAppUser
	appDBCfg.Password = testAppPassword

	pool, err := database.OpenPostgres(
		ctx,
		appDBCfg,
		"test",
		logger,
	)
	require.NoError(t, err, "Failed to open postgres pool")

	t.Cleanup(func() {
		pool.Close()
	})

	userRepo := NewUserRepository(pool, logger)
	teamRepo := NewTeamRepository(pool, logger)
	taskRepo := NewTaskRepository(pool, logger)
	analyticsRepo := NewAnalyticsRepository(pool, logger)

	return &testEnv{
		pool:          pool,
		userRepo:      userRepo,
		teamRepo:      teamRepo,
		taskRepo:      taskRepo,
		analyticsRepo: analyticsRepo,
		ctx:           ctx,
		logger:        logger,
	}
}

func createDatabaseRoles(
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

	require.NoError(t, err, "Failed to prepare database roles")
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

// ============================================================================
// Common helpers
// ============================================================================

func randomEmail() string {
	return uuid.New().String() + "@test.local"
}

func (env *testEnv) createTestUser(t *testing.T, email string) *domain.User {
	t.Helper()

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "$2a$12$test_hash",
	}
	require.NoError(t, env.userRepo.Create(env.ctx, user))
	return user
}

func (env *testEnv) createTeam(
	t *testing.T,
	ownerID uuid.UUID,
	name string,
) *domain.Team {
	t.Helper()

	team := &domain.Team{
		ID:   uuid.New(),
		Name: name,
	}

	require.NoError(t, env.teamRepo.Create(env.ctx, ownerID, team))
	return team
}

func (env *testEnv) addMember(
	t *testing.T,
	teamID, userID uuid.UUID,
	role domain.TeamRole,
) {
	t.Helper()

	member := &domain.TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}

	require.NoError(t, env.teamRepo.AddMember(env.ctx, member))
}

func (env *testEnv) createTestTaskWithHistory(
	t *testing.T,
	teamID uuid.UUID,
	creatorID uuid.UUID,
	title string,
	status domain.TaskStatus,
	assigneeID *uuid.UUID,
) *domain.Task {
	t.Helper()

	task := &domain.Task{
		ID:         uuid.New(),
		TeamID:     teamID,
		Title:      title,
		Status:     status,
		AssigneeID: assigneeID,
		CreatedBy:  creatorID,
	}

	changesJSON, err := json.Marshal(map[string]any{
		"title":  task.Title,
		"status": string(task.Status),
	})
	require.NoError(t, err)

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: creatorID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   changesJSON,
	}

	require.NoError(t, env.taskRepo.Create(env.ctx, task, history))

	return task
}

func (env *testEnv) updateTaskStatus(
	t *testing.T,
	taskID uuid.UUID,
	changedByID uuid.UUID,
	newStatus domain.TaskStatus,
) {
	t.Helper()

	newStatusCopy := newStatus
	update := domain.TaskUpdate{
		Status:    &newStatusCopy,
		StatusSet: true,
	}

	changesJSON, err := json.Marshal(map[string]any{
		"status": map[string]any{
			"to": string(newStatus),
		},
	})
	require.NoError(t, err)

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    taskID,
		ChangedBy: changedByID,
		Action:    domain.TaskHistoryActionUpdated,
		Changes:   changesJSON,
	}

	_, err = env.taskRepo.Update(env.ctx, taskID, changedByID, update, history)
	require.NoError(t, err)
}

func (env *testEnv) createTestTeam(t *testing.T, ownerID uuid.UUID, name string) *domain.Team {
	t.Helper()

	team := &domain.Team{
		ID:   uuid.New(),
		Name: name,
	}
	require.NoError(t, env.teamRepo.Create(env.ctx, ownerID, team))
	return team
}

func (env *testEnv) addTeamMember(t *testing.T, teamID, userID uuid.UUID, role domain.TeamRole) {
	t.Helper()

	member := &domain.TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}
	require.NoError(t, env.teamRepo.AddMember(env.ctx, member))
}

func (env *testEnv) createTestTaskWithHistoryAndAssignee(
	t *testing.T,
	teamID uuid.UUID,
	creatorID uuid.UUID,
	title string,
	status domain.TaskStatus,
	assigneeID *uuid.UUID,
) (*domain.Task, *domain.TaskHistory) {
	t.Helper()

	task := &domain.Task{
		ID:          uuid.New(),
		TeamID:      teamID,
		Title:       title,
		Description: nil,
		Status:      status,
		AssigneeID:  assigneeID,
		CreatedBy:   creatorID,
	}

	changesMap := map[string]any{
		"title":       task.Title,
		"status":      string(task.Status),
		"assignee_id": task.AssigneeID,
	}
	changesJSON, err := json.Marshal(changesMap)
	require.NoError(t, err)

	history := domain.TaskHistory{
		ID:        uuid.New(),
		TaskID:    task.ID,
		ChangedBy: creatorID,
		Action:    domain.TaskHistoryActionCreated,
		Changes:   changesJSON,
	}

	require.NoError(t, env.taskRepo.Create(env.ctx, task, history))

	return task, &history
}

func (env *testEnv) createOldDoneTask(
	t *testing.T,
	teamID uuid.UUID,
	creatorID uuid.UUID,
	title string,
	updatedAt time.Time,
) *domain.Task {
	t.Helper()

	task := &domain.Task{
		ID:        uuid.New(),
		TeamID:    teamID,
		Title:     title,
		Status:    domain.TaskStatusDone,
		CreatedBy: creatorID,
	}

	_, err := env.pool.Exec(
		env.ctx,
		`
		INSERT INTO tasks (
			id,
			team_id,
			title,
			status,
			created_by,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		`,
		task.ID,
		task.TeamID,
		task.Title,
		task.Status,
		task.CreatedBy,
		updatedAt,
		updatedAt,
	)

	require.NoError(t, err)

	return task
}
