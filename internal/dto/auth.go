// internal/dto/auth.go
package dto

import (
	"task-forge/internal/domain"
	"time"

	"github.com/google/uuid"
)

// JWTClaims — the structure of a JWT token.
type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
}

// =========================================================================
// Request
// =========================================================================

// RegisterRequest DTO for registration.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,strict_email"`
	Password string `json:"password" validate:"required,min=8,max=72,strong_password"`
}

// LoginRequest DTO for authentication.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,strict_email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// RegisterResponse DTO for the registration response.
type RegisterResponse struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// =========================================================================
// Response
// =========================================================================

// LoginResponse DTO для ответа на запрос авторизации.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DTO AuthResponse with a JWT token.
type AuthResponse struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      domain.User `json:"user"`
}
