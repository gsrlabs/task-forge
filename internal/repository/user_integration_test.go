//go:build integration
// +build integration

// internal/repository/user_integration_test.go
package repository

import (
	"task-forge/internal/domain"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Create_Success(t *testing.T) {
	env := setupPostgres(t)

	email := randomEmail()
	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "$2a$12$hashed_password",
	}

	assert.True(t, user.CreatedAt.IsZero())
	assert.True(t, user.UpdatedAt.IsZero())

	err := env.userRepo.Create(env.ctx, user)
	require.NoError(t, err, "Create should succeed")

	assert.False(t, user.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, user.UpdatedAt.IsZero(), "UpdatedAt should be set")

	found, err := env.userRepo.FindByEmail(env.ctx, email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, email, found.Email)
	assert.Equal(t, "$2a$12$hashed_password", found.PasswordHash)
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	env := setupPostgres(t)

	email := randomEmail()

	user1 := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "hash1",
	}
	require.NoError(t, env.userRepo.Create(env.ctx, user1))

	user2 := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "hash2",
	}

	err := env.userRepo.Create(env.ctx, user2)
	require.Error(t, err, "Should return error on duplicate email")
	require.ErrorIs(t, err, ErrUserAlreadyExists,
		"Error should be ErrUserAlreadyExists")
}

func TestUserRepository_FindByEmail_Success(t *testing.T) {
	env := setupPostgres(t)

	email := randomEmail()
	created := env.createTestUser(t, email)

	found, err := env.userRepo.FindByEmail(env.ctx, email)
	require.NoError(t, err, "Should find existing user")
	require.NotNil(t, found)

	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, email, found.Email)
	assert.Equal(t, "$2a$12$test_hash", found.PasswordHash)
	assert.False(t, found.CreatedAt.IsZero())
	assert.False(t, found.UpdatedAt.IsZero())
}

func TestUserRepository_FindByEmail_NotFound(t *testing.T) {
	env := setupPostgres(t)

	found, err := env.userRepo.FindByEmail(env.ctx, "nonexistent@example.com")
	require.Error(t, err, "Should return error for non-existent user")
	require.ErrorIs(t, err, ErrUserNotFound,
		"Error should be ErrUserNotFound")
	assert.Nil(t, found, "Result should be nil")
}

func TestUserRepository_FindByEmail_CaseInsensitive(t *testing.T) {
	env := setupPostgres(t)

	originalEmail := "User.Name@Example.COM"
	created := env.createTestUser(t, originalEmail)

	testCases := []struct {
		name  string
		email string
	}{
		{"lowercase", "user.name@example.com"},
		{"uppercase", "USER.NAME@EXAMPLE.COM"},
		{"mixed case", "User.Name@Example.Com"},
		{"original case", originalEmail},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			found, err := env.userRepo.FindByEmail(env.ctx, tc.email)
			require.NoError(t, err, "Should find user with %s email", tc.name)
			require.NotNil(t, found)
			assert.Equal(t, created.ID, found.ID,
				"Should return the same user regardless of case")
		})
	}
}

func TestUserRepository_FindByID_Success(t *testing.T) {
	env := setupPostgres(t)

	email := randomEmail()
	created := env.createTestUser(t, email)

	found, err := env.userRepo.FindByID(env.ctx, created.ID)
	require.NoError(t, err, "Should find existing user by ID")
	require.NotNil(t, found)

	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, email, found.Email)
	assert.Equal(t, "$2a$12$test_hash", found.PasswordHash)
	assert.False(t, found.CreatedAt.IsZero())
	assert.False(t, found.UpdatedAt.IsZero())
}

func TestUserRepository_FindByID_NotFound(t *testing.T) {
	env := setupPostgres(t)

	nonExistentID := uuid.New()

	found, err := env.userRepo.FindByID(env.ctx, nonExistentID)
	require.Error(t, err, "Should return error for non-existent ID")
	require.ErrorIs(t, err, ErrUserNotFound,
		"Error should be ErrUserNotFound")
	assert.Nil(t, found, "Result should be nil")
}

func TestUserRepository_FindByID_NilUUID(t *testing.T) {
	env := setupPostgres(t)

	found, err := env.userRepo.FindByID(env.ctx, uuid.Nil)
	require.Error(t, err, "Should return error for nil UUID")
	require.ErrorIs(t, err, ErrUserNotFound)
	assert.Nil(t, found)
}

func TestUserRepository_FindMethodsReturnSameUser(t *testing.T) {
	env := setupPostgres(t)

	email := randomEmail()
	created := env.createTestUser(t, email)

	byEmail, err := env.userRepo.FindByEmail(env.ctx, email)
	require.NoError(t, err)

	byID, err := env.userRepo.FindByID(env.ctx, created.ID)
	require.NoError(t, err)

	assert.Equal(t, byEmail.ID, byID.ID)
	assert.Equal(t, byEmail.Email, byID.Email)
	assert.Equal(t, byEmail.PasswordHash, byID.PasswordHash)
	assert.Equal(t, byEmail.CreatedAt.Unix(), byID.CreatedAt.Unix())
	assert.Equal(t, byEmail.UpdatedAt.Unix(), byID.UpdatedAt.Unix())
}
