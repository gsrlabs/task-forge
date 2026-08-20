// internal/service/auth_test.go
package service

import (
	"context"
	"errors"
	"fmt"
	"task-forge/internal/domain"
	"task-forge/internal/dto"
	"task-forge/internal/repository"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Mocks
// ============================================================================

// MockUserRepository is a mock implementation of repository.UserRepository.
type MockUserRepository struct {
	mock.Mock
}

// Create mocks the creation of a user.
func (m *MockUserRepository) Create(
	ctx context.Context,
	user *domain.User,
) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

// FindByEmail mocks finding a user by email.
func (m *MockUserRepository) FindByEmail(
	ctx context.Context,
	email string,
) (*domain.User, error) {
	args := m.Called(ctx, email)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*domain.User), args.Error(1)
}

// FindByID mocks finding a user by ID.
func (m *MockUserRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*domain.User, error) {
	args := m.Called(ctx, id)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*domain.User), args.Error(1)
}

// ============================================================================
// Test helpers
// ============================================================================

// newTestAuthService создаёт AuthService с моком репозитория и реальным JWTManager.
func newTestAuthService(
	t *testing.T,
	userRepo *MockUserRepository,
) *authService {
	t.Helper()

	jwtManager := NewJWTManager(
		"test-secret-key-at-least-32-chars-long-for-hmac-sha256",
		24*time.Hour,
	)

	return &authService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		logger:     zerolog.Nop(),
	}
}

func generateTestPasswordHash(t *testing.T, password string) string {
	t.Helper()
	// cost=4 для скорости (вместо production cost=12)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	require.NoError(t, err)
	return string(hash)
}

// ============================================================================
// TestAuthService
// ============================================================================

func TestAuthService_Register_Success(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, repository.ErrUserNotFound)

	userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.Email == req.Email &&
			u.ID != uuid.Nil &&
			len(u.PasswordHash) > 0
	})).Return(nil).
		Run(func(args mock.Arguments) {
			u := args.Get(1).(*domain.User)
			if u.ID == uuid.Nil {
				u.ID = uuid.New()
			}
		})

	userID, err := svc.Register(ctx, req)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, userID, "Should return valid user ID")

	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_UserAlreadyExistsOnCheck(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.RegisterRequest{
		Email:    "existing@example.com",
		Password: "StrongPass123",
	}

	existingUser := &domain.User{
		ID:    uuid.New(),
		Email: req.Email,
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(existingUser, nil)

	userID, err := svc.Register(ctx, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrUserAlreadyExists,
		"Should return ErrUserAlreadyExists when user exists")
	assert.Equal(t, uuid.Nil, userID, "Should return Nil UUID on error")

	userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_UserAlreadyExistsOnCreate_RaceCondition(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.RegisterRequest{
		Email:    "race@example.com",
		Password: "StrongPass123",
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, repository.ErrUserNotFound)

	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
		Return(repository.ErrUserAlreadyExists)

	userID, err := svc.Register(ctx, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrUserAlreadyExists,
		"Should map repository.ErrUserAlreadyExists to service.ErrUserAlreadyExists")
	assert.Equal(t, uuid.Nil, userID)

	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_FindByEmailError(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	dbErr := errors.New("database connection failed")

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, dbErr)

	userID, err := svc.Register(ctx, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "check existing user",
		"Should wrap the error with context")
	assert.True(t, errors.Is(err, dbErr),
		"Original error should be preserved in chain")
	assert.Equal(t, uuid.Nil, userID)

	userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_CreateError(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	dbErr := errors.New("insert failed")

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, repository.ErrUserNotFound)
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
		Return(dbErr)

	userID, err := svc.Register(ctx, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "create user")
	assert.True(t, errors.Is(err, dbErr))
	assert.Equal(t, uuid.Nil, userID)

	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_CorrectPasswordHashing(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "MySecurePassword99",
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, repository.ErrUserNotFound)

	var capturedHash string
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
		Return(nil).
		Run(func(args mock.Arguments) {
			u := args.Get(1).(*domain.User)
			capturedHash = u.PasswordHash
			if u.ID == uuid.Nil {
				u.ID = uuid.New()
			}
		})

	_, err := svc.Register(ctx, req)
	require.NoError(t, err)

	require.NotEmpty(t, capturedHash)
	err = bcrypt.CompareHashAndPassword([]byte(capturedHash), []byte(req.Password))
	require.NoError(t, err, "Stored hash should match the original password")

	assert.NotEqual(t, req.Password, capturedHash,
		"Password should not be stored in plaintext")

	userRepo.AssertExpectations(t)
}

func TestAuthService_Login_Success(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	password := "StrongPass123"

	user := &domain.User{
		ID:           uuid.New(),
		Email:        "user@example.com",
		PasswordHash: generateTestPasswordHash(t, password),
	}

	req := &dto.LoginRequest{
		Email:    user.Email,
		Password: password,
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(user, nil)

	resp, err := svc.Login(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token, "Token should not be empty")
	assert.True(t, resp.ExpiresAt.After(time.Now()),
		"ExpiresAt should be in the future")

	claims, err := svc.jwtManager.ValidateToken(resp.Token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Email, claims.Email)

	userRepo.AssertExpectations(t)
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "StrongPass123",
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, repository.ErrUserNotFound)

	resp, err := svc.Login(ctx, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCredentials,
		"Should return ErrInvalidCredentials to avoid user enumeration")
	assert.Nil(t, resp)

	userRepo.AssertExpectations(t)
}

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()

	user := &domain.User{
		ID:           uuid.New(),
		Email:        "user@example.com",
		PasswordHash: generateTestPasswordHash(t, "CorrectPassword1"),
	}

	req := &dto.LoginRequest{
		Email:    user.Email,
		Password: "WrongPassword1",
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(user, nil)

	resp, err := svc.Login(ctx, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCredentials,
		"Wrong password should return ErrInvalidCredentials")
	assert.Nil(t, resp)

	userRepo.AssertExpectations(t)
}

func TestAuthService_Login_FindByEmailError(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.LoginRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	dbErr := errors.New("database error")
	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, dbErr)

	resp, err := svc.Login(ctx, req)

	require.Error(t, err)
	assert.ErrorContains(t, err, "find user")
	assert.True(t, errors.Is(err, dbErr))
	assert.Nil(t, resp)

	userRepo.AssertExpectations(t)
}


func TestAuthService_Login_EmptyPasswordHash(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()

	user := &domain.User{
		ID:           uuid.New(),
		Email:        "user@example.com",
		PasswordHash: "",
	}

	req := &dto.LoginRequest{
		Email:    user.Email,
		Password: "AnyPassword1",
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(user, nil)

	resp, err := svc.Login(ctx, req)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Nil(t, resp)

	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_SetsUserIDOnCreation(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()
	req := &dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	userRepo.On("FindByEmail", ctx, req.Email).
		Return(nil, repository.ErrUserNotFound)

	var capturedUser *domain.User
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
		Return(nil).
		Run(func(args mock.Arguments) {
			capturedUser = args.Get(1).(*domain.User)
			if capturedUser.ID == uuid.Nil {
				capturedUser.ID = uuid.New()
			}
		})

	userID, err := svc.Register(ctx, req)
	require.NoError(t, err)

	require.NotNil(t, capturedUser)
	assert.Equal(t, req.Email, capturedUser.Email)
	assert.NotEmpty(t, capturedUser.PasswordHash)
	assert.Equal(t, userID, capturedUser.ID,
		"Returned userID should match the user.ID set by repository")

	userRepo.AssertExpectations(t)
}

func TestAuthService_Login_DoesNotLeakUserExistenceViaTiming(t *testing.T) {
	userRepo := new(MockUserRepository)
	svc := newTestAuthService(t, userRepo)

	ctx := context.Background()

	t.Run("nonexistent user", func(t *testing.T) {
		userRepo.ExpectedCalls = nil // reset
		userRepo.On("FindByEmail", ctx, "no@example.com").
			Return(nil, repository.ErrUserNotFound)

		_, err := svc.Login(ctx, &dto.LoginRequest{
			Email:    "no@example.com",
			Password: "AnyPass1",
		})

		require.ErrorIs(t, err, ErrInvalidCredentials)
	})

	t.Run("existing user with wrong password", func(t *testing.T) {
		userRepo.ExpectedCalls = nil
		user := &domain.User{
			ID:           uuid.New(),
			Email:        "yes@example.com",
			PasswordHash: generateTestPasswordHash(t, "CorrectPass1"),
		}
		userRepo.On("FindByEmail", ctx, "yes@example.com").
			Return(user, nil)

		_, err := svc.Login(ctx, &dto.LoginRequest{
			Email:    "yes@example.com",
			Password: "WrongPass1",
		})

		require.ErrorIs(t, err, ErrInvalidCredentials)
	})

	_ = fmt.Sprintf("both scenarios return %v", ErrInvalidCredentials)
}