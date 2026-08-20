// internal/middleware/metrics_test.go
package middleware

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSetUserIdentity_And_GetUserID(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)

	userID := uuid.New()
	email := "user@example.com"

	SetUserIdentity(c, userID, email)

	retrievedID, err := GetUserID(c)
	require.NoError(t, err)
	assert.Equal(t, userID, retrievedID)

	retrievedEmail, err := GetUserEmail(c)
	require.NoError(t, err)
	assert.Equal(t, email, retrievedEmail)
}

func TestGetUserID_NotInContext(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)

	userID, err := GetUserID(c)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrUserIDNotInContext)
	assert.Equal(t, uuid.Nil, userID)
}

func TestGetUserID_InvalidType(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)

	c.Set(string(contextKeyUserID), "not-a-uuid")

	userID, err := GetUserID(c)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidUserIDType)
	assert.Equal(t, uuid.Nil, userID)
}

func TestGetUserEmail_Success(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)

	email := "test@example.com"
	c.Set(string(contextKeyUserEmail), email)

	retrieved, err := GetUserEmail(c)
	require.NoError(t, err)
	assert.Equal(t, email, retrieved)
}

func TestGetUserEmail_NotInContext(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)

	email, err := GetUserEmail(c)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user email not found")
	assert.Equal(t, "", email)
}

func TestGetUserEmail_InvalidType(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)

	c.Set(string(contextKeyUserEmail), 12345) // wrong type

	email, err := GetUserEmail(c)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user email type")
	assert.Equal(t, "", email)
}