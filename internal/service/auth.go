package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"task-forge/internal/domain"
	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

type authService struct {
	userRepo   repository.UserRepository
	jwtManager *JWTManager
	logger     zerolog.Logger
}

// NewAuthService creates an instance of AuthService.
func NewAuthService(
	userRepo repository.UserRepository,
	jwtManager *JWTManager,
	logger zerolog.Logger,
) AuthService {
	return &authService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

// Register registers a new user.
func (s *authService) Register(ctx context.Context, req *dto.RegisterRequest) (uuid.UUID, error) {
	// Checking if there is a user with this email address
	existingUser, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		s.logger.Error().Err(err).Str("email", req.Email).Msg("Failed to check existing user")
		return uuid.Nil, fmt.Errorf("check existing user: %w", err)
	}

	if existingUser != nil {
		s.logger.Warn().Str("email", req.Email).Msg("Attempt to register existing user")
		return uuid.Nil, ErrUserAlreadyExists
	}

	// Hashing the password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to hash password")
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	// Creating a user
	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(passwordHash),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return uuid.Nil, ErrUserAlreadyExists
		}
		s.logger.Error().Err(err).Str("email", req.Email).Msg("Failed to create user")
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}

	s.logger.Info().
		Str("user_id", user.ID.String()).
		Str("email", user.Email).
		Msg("User registered successfully")

	return user.ID, nil
}

// Login authenticates the user.
func (s *authService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	// We are looking for a user by email
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.logger.Warn().Str("email", req.Email).Msg("Login attempt with non-existent email")
			return nil, ErrInvalidCredentials
		}
		s.logger.Error().Err(err).Str("email", req.Email).Msg("Failed to find user")
		return nil, fmt.Errorf("find user: %w", err)
	}

	// Comparing passwords
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.logger.Warn().
			Str("user_id", user.ID.String()).
			Str("email", user.Email).
			Msg("Login attempt with invalid password")
		return nil, ErrInvalidCredentials
	}

	s.logger.Info().
		Str("user_id", user.ID.String()).
		Str("email", user.Email).
		Msg("User logged in successfully")

	// Generating a JWT token
	token, expiresAt, err := s.jwtManager.GenerateToken(user)
	if err != nil {
		s.logger.Error().Err(err).Str("user_id", user.ID.String()).Msg("Failed to generate token")
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &dto.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}
