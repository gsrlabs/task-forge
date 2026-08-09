package handler

import (
	"net/http"
	"strings"
	"time"
	"github.com/google/uuid"

	"task-forge/internal/dto"
	"task-forge/internal/middleware"
	"task-forge/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
)

type ValidationScope string

const (
	ValidationScopeLogin    ValidationScope = "login"
	ValidationScopeRegister ValidationScope = "register"
)

// Rate Limit Constants — Registration
const (
	// Maximum number of registrations from a single IP within a time window
	regIPRateLimit = 10
	regIPWindow    = 2 * time.Minute

	// Maximum registrations per email within a time window
	regEmailRateLimit = 5
	regEmailWindow    = 5 * time.Minute
)

// Rate Limit Constants — Login
const (
	// Maximum login attempts from one IP address within a time window
	loginIPRateLimit = 10
	loginIPWindow    = 30 * time.Minute

	// Maximum login attempts per email within a time window
	loginEmailRateLimit = 5
	loginEmailWindow    = 15 * time.Minute
)

// ValidationErrorResponse formats validation errors into a structured response.
func ValidationErrorResponse(
	err error,
	scope ValidationScope,
) dto.ErrorResponse {
	resp := dto.ErrorResponse{
		Error: "validation failed",
	}

	ve, ok := err.(validator.ValidationErrors)
	if !ok || len(ve) == 0 {
		return resp
	}

	fe := ve[0]
	field := strings.ToLower(fe.Field())
	field = normalizeFieldName(field)

	switch scope {
	// ---------- LOGIN ----------
	case ValidationScopeLogin:
		switch fe.Tag() {
		case "required":
			resp.Details = field + " is required"
		case "strict_email":
			resp.Details = "invalid email format"
		case "min":
			if field == "password" {
				resp.Details = "password is too short (minimum 8 characters)"
			} else {
				resp.Details = "invalid request"
			}
		case "max":
			if field == "password" {
				resp.Details = "password is too long (maximum 72 characters)"
			} else {
				resp.Details = "invalid request"
			}
		default:
			resp.Details = "invalid request"
		}

	// ---------- REGISTRATION ----------
	case ValidationScopeRegister:
		switch fe.Tag() {
		case "required":
			resp.Details = field + " is required"
		case "strict_email":
			resp.Details = "invalid email format"
		case "min":
			if field == "password" {
				resp.Details = "password is too short (minimum 8 characters)"
			} else {
				resp.Details = field + " is too short"
			}
		case "max":
			if field == "password" {
				resp.Details = "password is too long (maximum 72 characters)"
			} else {
				resp.Details = field + " is too long"
			}
		case "len":
			resp.Details = field + " has invalid length"
		case "numeric":
			resp.Details = field + " must be numeric"
		case "strong_password":
			resp.Details = "password must contain at least one letter"
		default:
			resp.Details = field + " is invalid"
		}
	}

	return resp
}

// normalizeFieldName converts internal field names to custom ones..
func normalizeFieldName(fieldName string) string {
	switch fieldName {
	case "newpassword", "oldpassword":
		return "password"
	case "newemail":
		return "email"
	case "userid":
		return "user_id"
	default:
		return fieldName
	}
}

// rateLimitIP generates a key for rate limiting by IP address.
func (h *AuthHandler) rateLimitIP(ip string) string {
	return "rl:auth:ip:" + ip
}

// rateLimitEmail generates a key for rate limiting by email (hashed)
func (h *AuthHandler) rateLimitEmail(email string) string {
	return "rl:auth:email:" + utils.HashIdentifierWithKey(email, h.encryptionKey)
}

// enforceRegisterRateLimit checks rate limiting for registration.
func (h *AuthHandler) enforceRegisterRateLimit(c *gin.Context, email string) bool {
	ctx := c.Request.Context()
	ip := c.ClientIP()
	normalizedEmail := strings.ToLower(email)

	// Rate limit by IP: protection against mass registrations from a single IP
	allowed, err := h.cacheService.Allow(
		ctx,
		h.rateLimitIP(ip),
		regIPRateLimit,
		regIPWindow,
	)
	if err != nil {
		h.logger.Error().Err(err).Msg("Rate limit check failed (Redis down?), failing OPEN")
		// Failing OPEN: if Redis is unavailable, allow the request
		return true
	}

	if !allowed {
		h.logger.Warn().
			Str("ip", ip).
			Msg("Rate limit exceeded (registration by IP)")
		c.JSON(http.StatusTooManyRequests, dto.ErrorResponse{
			Error: "too many requests from your network",
		})
		return false
	}

	// Rate limit by email: spam protection for a single email
	allowed, err = h.cacheService.Allow(
		ctx,
		h.rateLimitEmail(normalizedEmail),
		regEmailRateLimit,
		regEmailWindow,
	)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("email", normalizedEmail).
			Msg("Rate limit error (registration by email)")
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal server error",
		})
		return false
	}

	if !allowed {
		h.logger.Warn().
			Str("email", normalizedEmail).
			Msg("Rate limit exceeded (registration by email)")
		c.JSON(http.StatusTooManyRequests, dto.ErrorResponse{
			Error: "too many registration attempts for this email",
		})
		return false
	}

	return true
}

// enforceLoginRateLimit checks rate limiting for login.
func (h *AuthHandler) enforceLoginRateLimit(c *gin.Context, email string) bool {
	ctx := c.Request.Context()
	ip := c.ClientIP()
	normalizedEmail := strings.ToLower(email)

	// IP Limit: protection against brute‑force attacks from a single IP address
	ipKey := "rl:login:ip:" + ip
	allowed, err := h.cacheService.Allow(
		ctx,
		ipKey,
		loginIPRateLimit,
		loginIPWindow,
	)
	if err != nil {
		h.logger.Error().Err(err).Msg("Login rate limit check failed (Redis down?), failing OPEN")
		return true
	}

	if !allowed {
		h.logger.Warn().
			Str("ip", ip).
			Msg("Login rate limit exceeded (by IP)")
		c.JSON(http.StatusTooManyRequests, dto.ErrorResponse{
			Error: "too many login attempts from your network",
		})
		return false
	}

	// Email Limit: protection against password guessing for a specific account
	emailKey := "rl:login:email:" + utils.HashIdentifierWithKey(normalizedEmail, h.encryptionKey)
	allowed, err = h.cacheService.Allow(
		ctx,
		emailKey,
		loginEmailRateLimit,
		loginEmailWindow,
	)

	if err != nil {
		h.logger.Error().
			Err(err).
			Str("email", normalizedEmail).
			Msg("Login rate limit error (by email)")
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal server error",
		})
		return false
	}

	if !allowed {
		// Return 401 instead of 429 to avoid revealing the fact that a specific email has been blocked.
		h.logger.Warn().
			Str("email", normalizedEmail).
			Msg("Login rate limit exceeded (by email)")
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
			Error: "invalid email or password",
		})
		return false
	}

	return true
}

func getAuthenticatedUserID(
	c *gin.Context,
	logger zerolog.Logger,
) (uuid.UUID, bool) {
	userID, err := middleware.GetUserID(c)
	if err == nil {
		return userID, true
	}

	logger.Error().
		Err(err).
		Msg("Failed to get authenticated user ID")

	c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
		Error:   "internal server error",
		Details: "failed to extract user identity",
	})

	return uuid.Nil, false
}
