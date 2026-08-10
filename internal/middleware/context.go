package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type contextKey string

const (
	contextKeyUserID    contextKey = "user_id"
	contextKeyUserEmail contextKey = "user_email"
)

var (
	ErrUserIDNotInContext = errors.New("user ID not found in context")
	ErrInvalidUserIDType  = errors.New("invalid user ID type in context")
)

// SetUserIdentity stores authenticated user information in Gin context.
func SetUserIdentity(c *gin.Context, userID uuid.UUID, email string) {
	c.Set(string(contextKeyUserID), userID)
	c.Set(string(contextKeyUserEmail), email)
}

// GetUserID returns the authenticated user's ID.
func GetUserID(c *gin.Context) (uuid.UUID, error) {
	value, exists := c.Get(string(contextKeyUserID))
	if !exists {
		return uuid.Nil, ErrUserIDNotInContext
	}

	userID, ok := value.(uuid.UUID)
	if !ok {
		return uuid.Nil, ErrInvalidUserIDType
	}

	return userID, nil
}

// GetUserEmail returns the authenticated user's email.
func GetUserEmail(c *gin.Context) (string, error) {
	value, exists := c.Get(string(contextKeyUserEmail))
	if !exists {
		return "", errors.New("user email not found in context")
	}

	email, ok := value.(string)
	if !ok {
		return "", errors.New("invalid user email type in context")
	}

	return email, nil
}