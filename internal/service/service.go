package service

import (
	"context"

	"task-forge/internal/dto"
	"task-forge/internal/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Services агрегирует все сервисы приложения.
type Services struct {
	Auth AuthService
}

// AuthService описывает контракт для аутентификации.
type AuthService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (uuid.UUID, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
}

// NewServices создает контейнер со всеми сервисами.
func NewServices(repos *repository.Repositories, jwtManager *JWTManager, logger zerolog.Logger) *Services {
	return &Services{
		Auth: NewAuthService(repos.Users, jwtManager, logger),
	}
}