// internal/service/jwt.go
package service

import (
	"fmt"
	"time"

	"task-forge/internal/domain"
	"task-forge/internal/dto"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTManager is responsible for generating and validating JWT tokens.
type JWTManager struct {
	secretKey     string
	tokenDuration time.Duration
}

// NewJWTManager creates an instance of JWTManager.
func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:     secretKey,
		tokenDuration: tokenDuration,
	}
}

// GenerateToken creates a JWT token for the user.
func (m *JWTManager) GenerateToken(user *domain.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.tokenDuration)

	claims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(m.secretKey))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateToken checks and parses the JWT token.
func (m *JWTManager) ValidateToken(tokenString string) (*dto.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secretKey), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, ErrInvalidToken
	}

	email, ok := claims["email"].(string)
	if !ok {
		return nil, ErrInvalidToken
	}

	return &dto.JWTClaims{
		UserID: userID,
		Email:  email,
	}, nil
}
