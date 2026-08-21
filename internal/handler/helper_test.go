package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-forge/internal/dto"
	"task-forge/internal/middleware"
	"task-forge/internal/utils"
	"task-forge/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// ValidationErrorResponse
// ============================================================================

func TestValidationErrorResponse_LoginScope(t *testing.T) {
	// ✅ Используем реальный валидатор с production-валидаторами
	v := validator.NewValidator()

	tests := []struct {
		name            string
		req             dto.LoginRequest
		expectedDetails string
	}{
		{
			name: "missing email",
			req: dto.LoginRequest{
				Email:    "",
				Password: "StrongPass123",
			},
			expectedDetails: "email is required",
		},
		{
			name: "missing password",
			req: dto.LoginRequest{
				Email:    "user@example.com",
				Password: "",
			},
			expectedDetails: "password is required",
		},
		{
			name: "invalid email format",
			req: dto.LoginRequest{
				Email:    "not-an-email",
				Password: "StrongPass123",
			},
			expectedDetails: "invalid email format",
		},
		{
			name: "password too short",
			req: dto.LoginRequest{
				Email:    "user@example.com",
				Password: "short",
			},
			expectedDetails: "password is too short (minimum 8 characters)",
		},
		{
			name: "password too long",
			req: dto.LoginRequest{
				Email:    "user@example.com",
				Password: string(make([]byte, 73)),
			},
			expectedDetails: "password is too long (maximum 72 characters)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateStruct(tt.req) // ← Используем ValidateStruct
			require.Error(t, err)

			resp := ValidationErrorResponse(err, ValidationScopeLogin)

			assert.Equal(t, "validation failed", resp.Error)
			assert.Equal(t, tt.expectedDetails, resp.Details)
		})
	}
}

func TestValidationErrorResponse_RegisterScope(t *testing.T) {
	// ✅ Используем реальный валидатор
	v := validator.NewValidator()

	tests := []struct {
		name            string
		req             dto.RegisterRequest
		expectedDetails string
	}{
		{
			name: "missing email",
			req: dto.RegisterRequest{
				Email:    "",
				Password: "StrongPass123",
			},
			expectedDetails: "email is required",
		},
		{
			name: "invalid email format",
			req: dto.RegisterRequest{
				Email:    "not-an-email",
				Password: "StrongPass123",
			},
			expectedDetails: "invalid email format",
		},
		{
			name: "password too short",
			req: dto.RegisterRequest{
				Email:    "user@example.com",
				Password: "short",
			},
			expectedDetails: "password is too short (minimum 8 characters)",
		},
		{
			name: "password too long",
			req: dto.RegisterRequest{
				Email:    "user@example.com",
				Password: string(make([]byte, 73)),
			},
			expectedDetails: "password is too long (maximum 72 characters)",
		},
		{
			name: "weak password - no letters",
			req: dto.RegisterRequest{
				Email:    "user@example.com",
				Password: "12345678",
			},
			expectedDetails: "password must contain at least one letter",
		},
		{
			name: "email with double dots in local part",
			req: dto.RegisterRequest{
				Email:    "user..name@example.com",
				Password: "StrongPass123",
			},
			expectedDetails: "invalid email format",
		},
		{
			name: "email starting with dot",
			req: dto.RegisterRequest{
				Email:    ".user@example.com",
				Password: "StrongPass123",
			},
			expectedDetails: "invalid email format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateStruct(tt.req)
			require.Error(t, err)

			resp := ValidationErrorResponse(err, ValidationScopeRegister)

			assert.Equal(t, "validation failed", resp.Error)
			assert.Equal(t, tt.expectedDetails, resp.Details)
		})
	}
}

// ============================================================================
// Остальные тесты без изменений
// ============================================================================

func TestValidationErrorResponse_NonValidationError(t *testing.T) {
	err := errors.New("some random error")

	resp := ValidationErrorResponse(err, ValidationScopeLogin)

	assert.Equal(t, "validation failed", resp.Error)
	assert.Empty(t, resp.Details, "Details should be empty for non-validation errors")
}

func TestNormalizeFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"newpassword", "password"},
		{"oldpassword", "password"},
		{"newemail", "email"},
		{"userid", "user_id"},
		{"email", "email"},
		{"password", "password"},
		{"username", "username"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeFieldName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAuthHandler_rateLimitIP(t *testing.T) {
	handler := &AuthHandler{
		encryptionKey: "test-key",
	}

	ip := "192.168.1.1"
	result := handler.rateLimitIP(ip)

	assert.Equal(t, "rl:auth:ip:192.168.1.1", result)
}

func TestAuthHandler_rateLimitEmail(t *testing.T) {
	handler := &AuthHandler{
		encryptionKey: "test-encryption-key",
	}

	email := "user@example.com"
	result := handler.rateLimitEmail(email)

	assert.Contains(t, result, "rl:auth:email:")

	expectedHash := utils.HashIdentifierWithKey(email, handler.encryptionKey)
	assert.Contains(t, result, expectedHash)
}

func TestGetAuthenticatedUserID_Success(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	userID := uuid.New()
	email := "user@example.com"

	middleware.SetUserIdentity(c, userID, email)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	c.Request = req

	resultID, ok := getAuthenticatedUserID(c, zerolog.Nop())

	assert.True(t, ok)
	assert.Equal(t, userID, resultID)
}

func TestGetAuthenticatedUserID_ContextAborted(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Abort()

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	c.Request = req

	resultID, ok := getAuthenticatedUserID(c, zerolog.Nop())

	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, resultID)
}

func TestGetAuthenticatedUserID_NotInContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	c.Request = req

	resultID, ok := getAuthenticatedUserID(c, zerolog.Nop())

	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, resultID)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "internal server error", resp.Error)
	assert.Contains(t, resp.Details, "middleware was not applied")
}

func TestGetAuthenticatedUserID_InvalidType(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("user_id", "not-a-uuid")

	req := httptest.NewRequest("GET", "/api/v1/teams", nil)
	c.Request = req

	resultID, ok := getAuthenticatedUserID(c, zerolog.Nop())

	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, resultID)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "internal server error", resp.Error)
	assert.Contains(t, resp.Details, "failed to extract user identity")
}
