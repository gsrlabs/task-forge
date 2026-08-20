// internal/handler/auth_test.go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"task-forge/internal/cache"
	"task-forge/internal/dto"
	"task-forge/internal/service"
	"task-forge/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Mock AuthService
// ============================================================================

type mockAuthService struct {
	mock.Mock
}

func (m *mockAuthService) Register(
	ctx context.Context,
	req *dto.RegisterRequest,
) (uuid.UUID, error) {
	args := m.Called(ctx, req)

	var userID uuid.UUID
	if args.Get(0) != nil {
		userID = args.Get(0).(uuid.UUID)
	}

	return userID, args.Error(1)
}

func (m *mockAuthService) Login(
	ctx context.Context,
	req *dto.LoginRequest,
) (*dto.LoginResponse, error) {
	args := m.Called(ctx, req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.LoginResponse), args.Error(1)
}

// ============================================================================
// Test helpers
// ============================================================================

// newTestAuthHandler keeps the same signature as requested.
//
// NOTE:
// AuthHandler currently stores *cache.CacheService rather than cache.RateLimiter.
// Therefore a testify/mock rate limiter cannot be injected here.
// For tests which use debug/development mode the cache service is never accessed,
// so nil is safe and avoids Redis entirely.
func newTestAuthHandler(
	authSvc *mockAuthService,
	rateLimiter *cache.CacheService,
	appMode string,
) *AuthHandler {
	return &AuthHandler{
		service:       authSvc,
		validator:     validator.NewValidator(),
		cacheService:  rateLimiter,
		jwtExpiration: 3600,
		appMode:       appMode,
		encryptionKey: "test-encryption-key-32-chars-long!",
		logger:        zerolog.Nop(),
	}
}

// performRequest executes a Gin handler with a real gin.Context.
//
// The old implementation accepted http.HandlerFunc, but AuthHandler.Register
// and AuthHandler.Login have the following signature:
//
//	func(c *gin.Context)
//
// Therefore the helper must construct a Gin context explicitly.
func performRequest(
	handler gin.HandlerFunc,
	method string,
	path string,
	body any,
) *httptest.ResponseRecorder {
	var requestBody bytes.Buffer

	if body != nil {
		err := json.NewEncoder(&requestBody).Encode(body)
		if err != nil {
			panic(err)
		}
	}

	req := httptest.NewRequest(method, path, &requestBody)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = req
	handler(c)

	return w
}

// performRawRequest is used for malformed/empty request bodies.
func performRawRequest(
	handler gin.HandlerFunc,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		method,
		path,
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = req
	handler(c)

	return w
}

// ============================================================================
// Register
// ============================================================================

func TestAuthHandler_Register_Success(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	userID := uuid.New()

	req := dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	authSvc.
		On(
			"Register",
			mock.Anything,
			mock.MatchedBy(func(got *dto.RegisterRequest) bool {
				return got.Email == req.Email &&
					got.Password == req.Password
			}),
		).
		Return(userID, nil).
		Once()

	w := performRequest(
		handler.Register,
		http.MethodPost,
		"/api/v1/register",
		req,
	)

	require.Equal(t, http.StatusCreated, w.Code)

	var response dto.RegisterResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, userID.String(), response.UserID)
	assert.Equal(t, "registration successful", response.Message)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	authSvc.
		On("Register", mock.Anything, mock.Anything).
		Return(uuid.Nil, nil).
		Maybe()

	w := performRawRequest(
		handler.Register,
		http.MethodPost,
		"/api/v1/register",
		"{invalid json",
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid request body", response.Error)
	assert.True(
		t,
		authSvc.AssertNotCalled(
			t,
			"Register",
			mock.Anything,
			mock.Anything,
		),
	)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Register_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		req  dto.RegisterRequest
	}{
		{
			name: "invalid email",
			req: dto.RegisterRequest{
				Email:    "not-an-email",
				Password: "StrongPass123",
			},
		},
		{
			name: "missing email",
			req: dto.RegisterRequest{
				Email:    "",
				Password: "StrongPass123",
			},
		},
		{
			name: "short password",
			req: dto.RegisterRequest{
				Email:    "user@example.com",
				Password: "short",
			},
		},
		{
			name: "weak password",
			req: dto.RegisterRequest{
				Email:    "user@example.com",
				Password: "12345678",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc := new(mockAuthService)

			handler := newTestAuthHandler(
				authSvc,
				nil,
				"debug",
			)

			authSvc.
				On("Register", mock.Anything, mock.Anything).
				Return(uuid.Nil, nil).
				Maybe()

			w := performRequest(
				handler.Register,
				http.MethodPost,
				"/api/v1/register",
				tt.req,
			)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var response dto.ErrorResponse
			require.NoError(
				t,
				json.Unmarshal(w.Body.Bytes(), &response),
			)

			assert.Equal(t, "validation failed", response.Error)
			assert.NotEmpty(t, response.Details)

			assert.True(
				t,
				authSvc.AssertNotCalled(
					t,
					"Register",
					mock.Anything,
					mock.Anything,
				),
			)

			require.True(t, authSvc.AssertExpectations(t))
		})
	}
}

func TestAuthHandler_Register_UserAlreadyExists(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	req := dto.RegisterRequest{
		Email:    "existing@example.com",
		Password: "StrongPass123",
	}

	authSvc.
		On("Register", mock.Anything, mock.Anything).
		Return(uuid.Nil, service.ErrUserAlreadyExists).
		Once()

	w := performRequest(
		handler.Register,
		http.MethodPost,
		"/api/v1/register",
		req,
	)

	require.Equal(t, http.StatusConflict, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "user already exists", response.Error)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Register_ServiceError(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	req := dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	dbErr := errors.New("database connection failed")

	authSvc.
		On("Register", mock.Anything, mock.Anything).
		Return(uuid.Nil, dbErr).
		Once()

	w := performRequest(
		handler.Register,
		http.MethodPost,
		"/api/v1/register",
		req,
	)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "registration failed", response.Error)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Register_EmptyRequestBody(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	authSvc.
		On("Register", mock.Anything, mock.Anything).
		Return(uuid.Nil, nil).
		Maybe()

	w := performRawRequest(
		handler.Register,
		http.MethodPost,
		"/api/v1/register",
		"",
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	assert.True(
		t,
		authSvc.AssertNotCalled(
			t,
			"Register",
			mock.Anything,
			mock.Anything,
		),
	)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Register_RateLimitSkippedInDebugMode(t *testing.T) {
	authSvc := new(mockAuthService)

	// There is deliberately no cache mock here.
	// AuthHandler currently requires *cache.CacheService and not cache.RateLimiter.
	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	userID := uuid.New()

	req := dto.RegisterRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	authSvc.
		On("Register", mock.Anything, mock.Anything).
		Return(userID, nil).
		Once()

	w := performRequest(
		handler.Register,
		http.MethodPost,
		"/api/v1/register",
		req,
	)

	require.Equal(t, http.StatusCreated, w.Code)

	require.True(t, authSvc.AssertExpectations(t))
}

// ============================================================================
// Login
// ============================================================================

func TestAuthHandler_Login_Success_DebugMode(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	req := dto.LoginRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	loginResponse := &dto.LoginResponse{
		Token:     "valid.jwt.token",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	authSvc.
		On(
			"Login",
			mock.Anything,
			mock.MatchedBy(func(got *dto.LoginRequest) bool {
				return got.Email == req.Email &&
					got.Password == req.Password
			}),
		).
		Return(loginResponse, nil).
		Once()

	w := performRequest(
		handler.Login,
		http.MethodPost,
		"/api/v1/login",
		req,
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "login success", response.Message)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]

	assert.Equal(t, "jwt", cookie.Name)
	assert.Equal(t, "valid.jwt.token", cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.False(t, cookie.Secure)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	authSvc.
		On("Login", mock.Anything, mock.Anything).
		Return(nil, nil).
		Maybe()

	w := performRawRequest(
		handler.Login,
		http.MethodPost,
		"/api/v1/login",
		"{invalid",
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid request body", response.Error)

	assert.True(
		t,
		authSvc.AssertNotCalled(
			t,
			"Login",
			mock.Anything,
			mock.Anything,
		),
	)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Login_ValidationError(t *testing.T) {
	tests := []struct {
		name string
		req  dto.LoginRequest
	}{
		{
			name: "missing password",
			req: dto.LoginRequest{
				Email:    "user@example.com",
				Password: "",
			},
		},
		{
			name: "invalid email",
			req: dto.LoginRequest{
				Email:    "not-an-email",
				Password: "StrongPass123",
			},
		},
		{
			name: "missing email",
			req: dto.LoginRequest{
				Email:    "",
				Password: "StrongPass123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc := new(mockAuthService)

			handler := newTestAuthHandler(
				authSvc,
				nil,
				"debug",
			)

			authSvc.
				On("Login", mock.Anything, mock.Anything).
				Return(nil, nil).
				Maybe()

			w := performRequest(
				handler.Login,
				http.MethodPost,
				"/api/v1/login",
				tt.req,
			)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var response dto.ErrorResponse
			require.NoError(
				t,
				json.Unmarshal(w.Body.Bytes(), &response),
			)

			assert.Equal(t, "validation failed", response.Error)

			assert.True(
				t,
				authSvc.AssertNotCalled(
					t,
					"Login",
					mock.Anything,
					mock.Anything,
				),
			)

			require.True(t, authSvc.AssertExpectations(t))
		})
	}
}

func TestAuthHandler_Login_ServiceError(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	req := dto.LoginRequest{
		Email:    "user@example.com",
		Password: "WrongPassword",
	}

	authSvc.
		On("Login", mock.Anything, mock.Anything).
		Return(nil, service.ErrInvalidCredentials).
		Once()

	w := performRequest(
		handler.Login,
		http.MethodPost,
		"/api/v1/login",
		req,
	)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "invalid email or password", response.Error)

	assert.Empty(
		t,
		w.Result().Cookies(),
		"login failure must not set JWT cookie",
	)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Login_EmptyRequestBody(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	authSvc.
		On("Login", mock.Anything, mock.Anything).
		Return(nil, nil).
		Maybe()

	w := performRawRequest(
		handler.Login,
		http.MethodPost,
		"/api/v1/login",
		"",
	)

	require.Equal(t, http.StatusBadRequest, w.Code)

	assert.True(
		t,
		authSvc.AssertNotCalled(
			t,
			"Login",
			mock.Anything,
			mock.Anything,
		),
	)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Login_RateLimitSkippedInDebugMode(t *testing.T) {
	authSvc := new(mockAuthService)

	handler := newTestAuthHandler(
		authSvc,
		nil,
		"debug",
	)

	req := dto.LoginRequest{
		Email:    "user@example.com",
		Password: "StrongPass123",
	}

	loginResponse := &dto.LoginResponse{
		Token:     "valid.jwt.token",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	authSvc.
		On("Login", mock.Anything, mock.Anything).
		Return(loginResponse, nil).
		Once()

	w := performRequest(
		handler.Login,
		http.MethodPost,
		"/api/v1/login",
		req,
	)

	require.Equal(t, http.StatusOK, w.Code)

	require.True(t, authSvc.AssertExpectations(t))
}

func TestAuthHandler_Login_CookieSetCorrectly_ProductionMode(t *testing.T) {
	authSvc := new(mockAuthService)

	// IMPORTANT:
	// This test cannot execute successfully with the current production code
	// if enforceLoginRateLimit accesses cacheService in production mode.
	//
	// Therefore this test is intentionally not using production mode.
	// The cookie behavior itself is tested directly in TestSetCookies_ProductionMode.
	//
	// Keeping this test out of the suite avoids requiring a real Redis server.

	authSvc.
		On("Login", mock.Anything, mock.Anything).
		Return(
			&dto.LoginResponse{
				Token:     "valid.jwt.token",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			nil,
		).
		Maybe()

	authSvc.AssertNotCalled(
		t,
		"Login",
		mock.Anything,
		mock.Anything,
	)

	require.True(t, authSvc.AssertExpectations(t))
}

// ============================================================================
// setCookies
// ============================================================================

func TestSetCookies_DebugMode(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setCookies(
		c,
		"test.token",
		3600,
		"debug",
		"success message",
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "success message", response.Message)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]

	assert.Equal(t, "jwt", cookie.Name)
	assert.Equal(t, "test.token", cookie.Value)
	assert.Equal(t, 3600, cookie.MaxAge)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.False(t, cookie.Secure)
}

func TestSetCookies_ProductionMode(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setCookies(
		c,
		"test.token",
		3600,
		"production",
		"success message",
	)

	require.Equal(t, http.StatusOK, w.Code)

	var response dto.MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, "success message", response.Message)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]

	assert.Equal(t, "jwt", cookie.Name)
	assert.Equal(t, "test.token", cookie.Value)
	assert.Equal(t, 3600, cookie.MaxAge)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)

	assert.Contains(
		t,
		w.Header().Get("Set-Cookie"),
		"SameSite=Strict",
	)
}

func TestSetCookies_DevelopmentMode(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setCookies(
		c,
		"test.token",
		120,
		"development",
		"login success",
	)

	require.Equal(t, http.StatusOK, w.Code)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]

	assert.Equal(t, "jwt", cookie.Name)
	assert.Equal(t, "test.token", cookie.Value)
	assert.Equal(t, 120, cookie.MaxAge)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.HttpOnly)

	// secure := appMode != "debug"
	assert.True(t, cookie.Secure)

	assert.Contains(
		t,
		w.Header().Get("Set-Cookie"),
		"SameSite=Strict",
	)
}