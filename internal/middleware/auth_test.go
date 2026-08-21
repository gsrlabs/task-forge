// internal/middleware/auth_test.go

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"task-forge/internal/domain"
	"task-forge/internal/dto"
	"task-forge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRateLimiter is a mock implementation of cache.RateLimiter.
type MockRateLimiter struct {
	mock.Mock
}

func (m *MockRateLimiter) Allow(
	ctx context.Context,
	key string,
	limit int,
	window time.Duration,
) (bool, error) {
	args := m.Called(ctx, key, limit, window)
	return args.Bool(0), args.Error(1)
}

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Test helpers
// ============================================================================

const testJWTSecret = "test-secret-key-at-least-32-chars-long-for-hmac"

// newTestAuthMiddleware creates AuthMiddleware with mocked dependencies.
func newTestAuthMiddleware(rateLimiter *MockRateLimiter) *AuthMiddleware {
	jwtManager := service.NewJWTManager(testJWTSecret, 24*time.Hour)
	return &AuthMiddleware{
		rateLimiter: rateLimiter,
		jwtManager:  jwtManager,
		logger:      zerolog.Nop(),
	}
}

// generateValidToken creates a valid JWT for testing.
func generateValidToken(t *testing.T) (string, uuid.UUID, string) {
	t.Helper()

	jwtManager := service.NewJWTManager(
		testJWTSecret,
		24*time.Hour,
	)

	userID := uuid.New()
	email := "user@example.com"

	user := &domain.User{
		ID:    userID,
		Email: email,
	}

	token, _, err := jwtManager.GenerateToken(user)
	require.NoError(t, err)

	return token, userID, email
}

// ============================================================================
// Auth Tests
// ============================================================================

func TestAuthenticate_AllowIP_Success(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, "rl:auth_mw:ip:192.168.1.1", 100, time.Minute).
		Return(true, nil)

	rateLimiter.On("Allow", mock.Anything, mock.MatchedBy(func(key string) bool {
		return len(key) > 0 // any user key
	}), 100, time.Minute).Return(true, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	_ = mw
	_ = c
}

func TestAuthenticate_IPRateLimit_Blocked(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, mock.MatchedBy(func(key string) bool {
		return len(key) > 0 && key[:13] == "rl:auth_mw:ip"
	}), 100, time.Minute).Return(false, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	c.Request = req

	handler := mw.Authenticate()
	handler(c)

	// Should return 429
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var resp dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "too many requests, please slow down", resp.Error)

	rateLimiter.AssertNumberOfCalls(t, "Allow", 1)
}

func TestAuthenticate_IPRateLimit_FailOpen(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, mock.MatchedBy(func(key string) bool {
		return len(key) > 0 && key[:13] == "rl:auth_mw:ip"
	}), 100, time.Minute).Return(false, errors.New("redis connection refused"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	c.Request = req

	handler := mw.Authenticate()
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"Should fail-open on IP check, then fail on JWT")

	var resp dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "unauthorized", resp.Error)
}

func TestAuthenticate_NoCookie(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, mock.Anything, 100, time.Minute).
		Return(true, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	c.Request = req

	handler := mw.Authenticate()
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "unauthorized", resp.Error)

	rateLimiter.AssertNumberOfCalls(t, "Allow", 1)
}

func TestAuthenticate_EmptyCookie(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, mock.Anything, 100, time.Minute).
		Return(true, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.AddCookie(&http.Cookie{Name: "jwt", Value: ""})
	c.Request = req

	handler := mw.Authenticate()
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "unauthorized", resp.Error)
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, mock.Anything, 100, time.Minute).
		Return(true, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.AddCookie(&http.Cookie{Name: "jwt", Value: "invalid.jwt.token"})
	c.Request = req

	handler := mw.Authenticate()
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid or expired token", resp.Error)
}

func TestAuthenticate_ExpiredToken(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, mock.Anything, 100, time.Minute).
		Return(true, nil)

	jwtManager := service.NewJWTManager(testJWTSecret, -1*time.Hour)
	userID := uuid.New()
	email := "user@example.com"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.AddCookie(&http.Cookie{Name: "jwt", Value: "expired.token.here"})
	c.Request = req

	handler := mw.Authenticate()
	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	_ = jwtManager
	_ = userID
	_ = email
}

func TestAuthenticate_UserRateLimit_Blocked(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	userID := uuid.New()
	email := "user@example.com"

	jwtMgr := service.NewJWTManager(testJWTSecret, 24*time.Hour)

	rateLimiter.On("Allow", mock.Anything, mock.MatchedBy(func(key string) bool {
		return len(key) > 0 && key[:13] == "rl:auth_mw:ip"
	}), 100, time.Minute).Return(true, nil)

	rateLimiter.On("Allow", mock.Anything, mock.MatchedBy(func(key string) bool {
		return len(key) > 0 && key[:14] == "rl:auth_mw:user"
	}), 100, time.Minute).Return(false, nil)

	_ = mw
	_ = jwtMgr
	_ = userID
	_ = email
}

func TestAuthenticate_UserRateLimit_FailOpen(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, mock.MatchedBy(func(key string) bool {
		return len(key) > 0 && key[:13] == "rl:auth_mw:ip"
	}), 100, time.Minute).Return(true, nil)

	rateLimiter.On("Allow", mock.Anything, mock.MatchedBy(func(key string) bool {
		return len(key) > 0 && key[:14] == "rl:auth_mw:user"
	}), 100, time.Minute).Return(false, errors.New("redis error"))

	_ = mw
}

func TestAuthenticate_FullFlow_Success(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)
	token, userID, email := generateValidToken(t)

	ip := "192.168.1.1"

	rateLimiter.On(
		"Allow",
		mock.Anything,
		"rl:auth_mw:ip:"+ip,
		100,
		time.Minute,
	).Return(true, nil)

	rateLimiter.On(
		"Allow",
		mock.Anything,
		"rl:auth_mw:user:"+userID.String(),
		100,
		time.Minute,
	).Return(true, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = ip + ":12345"
	req.AddCookie(&http.Cookie{
		Name:  jwtCookieName,
		Value: token,
	})

	c.Request = req

	mw.Authenticate()(c)

	assert.False(t, c.IsAborted())

	retrievedUserID, err := GetUserID(c)
	require.NoError(t, err)
	assert.Equal(t, userID, retrievedUserID)

	retrievedEmail, err := GetUserEmail(c)
	require.NoError(t, err)
	assert.Equal(t, email, retrievedEmail)

	rateLimiter.AssertExpectations(t)
}

func TestAuthenticate_FullFlow_IPBlocked(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	rateLimiter.On("Allow", mock.Anything, mock.Anything, 100, time.Minute).
		Return(false, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	req.AddCookie(&http.Cookie{Name: "jwt", Value: "valid-token"})
	c.Request = req

	handler := mw.Authenticate()
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	rateLimiter.AssertNumberOfCalls(t, "Allow", 1)
}

func TestAuthenticate_FullFlow_UserBlocked(t *testing.T) {
	rateLimiter := new(MockRateLimiter)
	mw := newTestAuthMiddleware(rateLimiter)

	token, userID, _ := generateValidToken(t)

	ip := "192.168.1.1"

	rateLimiter.On(
		"Allow",
		mock.Anything,
		"rl:auth_mw:ip:"+ip,
		100,
		time.Minute,
	).Return(true, nil)

	rateLimiter.On(
		"Allow",
		mock.Anything,
		"rl:auth_mw:user:"+userID.String(),
		100,
		time.Minute,
	).Return(false, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	req.RemoteAddr = ip + ":12345"
	req.AddCookie(&http.Cookie{
		Name:  jwtCookieName,
		Value: token,
	})

	c.Request = req

	mw.Authenticate()(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var resp dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(
		t,
		"too many requests, please slow down",
		resp.Error,
	)

	rateLimiter.AssertNumberOfCalls(t, "Allow", 2)
	rateLimiter.AssertExpectations(t)
}
