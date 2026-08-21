// internal/service/jwt_test.go
package service

import (
	"strings"
	"testing"
	"time"

	"task-forge/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key-at-least-32-chars-long-for-hmac-sha256"

func TestJWTManager_GenerateToken_Success(t *testing.T) {
	manager := NewJWTManager(testSecret, 24*time.Hour)

	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
	}

	before := time.Now()
	token, expiresAt, err := manager.GenerateToken(user)
	after := time.Now()

	require.NoError(t, err)
	assert.NotEmpty(t, token, "Token should not be empty")
	assert.Contains(t, token, ".", "JWT should contain dots (header.payload.signature)")

	expectedExpires := before.Add(24 * time.Hour)
	assert.WithinRange(t, expiresAt,
		expectedExpires.Add(-time.Second),
		expectedExpires.Add(time.Second),
		"ExpiresAt should be approximately now + 24h")

	assert.True(t, expiresAt.After(before))
	assert.True(t, expiresAt.After(after) || expiresAt.Equal(after))
}

func TestJWTManager_GenerateToken_RoundTrip(t *testing.T) {
	manager := NewJWTManager(testSecret, 24*time.Hour)

	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
	}

	token, _, err := manager.GenerateToken(user)
	require.NoError(t, err)

	claims, err := manager.ValidateToken(token)
	require.NoError(t, err)

	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Email, claims.Email)
}

func TestJWTManager_GenerateToken_DifferentUsersDifferentTokens(t *testing.T) {
	manager := NewJWTManager(testSecret, 24*time.Hour)

	user1 := &domain.User{ID: uuid.New(), Email: "user1@example.com"}
	user2 := &domain.User{ID: uuid.New(), Email: "user2@example.com"}

	token1, _, err := manager.GenerateToken(user1)
	require.NoError(t, err)

	token2, _, err := manager.GenerateToken(user2)
	require.NoError(t, err)

	assert.NotEqual(t, token1, token2,
		"Tokens for different users should be different")
}

func TestJWTManager_ValidateToken_Success(t *testing.T) {
	manager := NewJWTManager(testSecret, 24*time.Hour)

	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
	}

	token, _, err := manager.GenerateToken(user)
	require.NoError(t, err)

	claims, err := manager.ValidateToken(token)
	require.NoError(t, err)
	require.NotNil(t, claims)

	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Email, claims.Email)
}

func TestJWTManager_ValidateToken_InvalidFormat(t *testing.T) {
	manager := NewJWTManager(testSecret, 24*time.Hour)

	testCases := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"random string", "not-a-jwt"},
		{"only one part", "header"},
		{"two parts only", "header.payload"},
		{"four parts", "a.b.c.d"},
		{"malformed base64", "!!!.@@@.###"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := manager.ValidateToken(tc.token)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInvalidToken)
			assert.Nil(t, claims)
		})
	}
}

func TestJWTManager_ValidateToken_Expired(t *testing.T) {
	expiredTime := time.Now().Add(-1 * time.Hour)

	claims := jwt.MapClaims{
		"user_id": uuid.New().String(),
		"email":   "user@example.com",
		"exp":     expiredTime.Unix(),
		"iat":     expiredTime.Add(-24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken,
		"Expired token should return ErrInvalidToken")
	assert.Nil(t, result)
}

func TestJWTManager_ValidateToken_WrongSecret(t *testing.T) {
	manager := NewJWTManager(testSecret, 24*time.Hour)
	wrongManager := NewJWTManager("completely-different-secret-key-32chars!", 24*time.Hour)

	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
	}

	token, _, err := wrongManager.GenerateToken(user)
	require.NoError(t, err)

	claims, err := manager.ValidateToken(token)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken,
		"Token signed with wrong secret should be rejected")
	assert.Nil(t, claims)
}

func TestJWTManager_ValidateToken_WrongAlgorithm(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": uuid.New().String(),
		"email":   "user@example.com",
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, result)
}

func TestJWTManager_ValidateToken_HS256(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": uuid.New().String(),
		"email":   "user@example.com",
		"exp":     time.Now().Add(time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestJWTManager_ValidateToken_MissingUserID(t *testing.T) {
	claims := jwt.MapClaims{
		"email": "user@example.com",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, result)
}

func TestJWTManager_ValidateToken_UserIDWrongType(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": 12345,
		"email":   "user@example.com",
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, result)
}

func TestJWTManager_ValidateToken_InvalidUUIDFormat(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": "not-a-valid-uuid",
		"email":   "user@example.com",
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, result)
}

func TestJWTManager_ValidateToken_MissingEmail(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": uuid.New().String(),
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, result)
}

func TestJWTManager_ValidateToken_EmailWrongType(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": uuid.New().String(),
		"email":   12345,
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, result)
}

func TestJWTManager_ValidateToken_MissingExp(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": uuid.New().String(),
		"email":   "user@example.com",
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	manager := NewJWTManager(testSecret, 24*time.Hour)

	result, err := manager.ValidateToken(tokenString)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, result)
}

func TestJWTManager_TokenStructure(t *testing.T) {
	manager := NewJWTManager(testSecret, 24*time.Hour)

	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
	}

	token, _, err := manager.GenerateToken(user)
	require.NoError(t, err)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "JWT should have 3 parts: header.payload.signature")

	for i, part := range parts {
		assert.NotEmpty(t, part, "Part %d should not be empty", i)
	}
}

func TestJWTManager_DifferentDurations(t *testing.T) {
	testCases := []struct {
		name     string
		duration time.Duration
	}{
		{"1 hour", 1 * time.Hour},
		{"24 hours", 24 * time.Hour},
		{"7 days", 7 * 24 * time.Hour},
		{"30 days", 30 * 24 * time.Hour},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := NewJWTManager(testSecret, tc.duration)

			user := &domain.User{
				ID:    uuid.New(),
				Email: "user@example.com",
			}

			before := time.Now()
			_, expiresAt, err := manager.GenerateToken(user)
			require.NoError(t, err)

			expected := before.Add(tc.duration)
			assert.WithinRange(t, expiresAt,
				expected.Add(-time.Second),
				expected.Add(time.Second),
				"ExpiresAt should match configured duration")
		})
	}
}
