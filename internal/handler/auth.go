// internal/handler/auth.go
package handler

import (
	"errors"
	"net/http"

	"task-forge/internal/cache"
	"task-forge/internal/dto"
	"task-forge/internal/service"
	"task-forge/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// AuthHandler processes authentication requests.
type AuthHandler struct {
	service       service.AuthService
	validator     *validator.Validator
	cacheService  *cache.CacheService
	jwtExpiration int
	appMode       string
	encryptionKey string
	logger        zerolog.Logger
}

// NewAuthHandler creates an instance of AuthHandler.
func NewAuthHandler(
	service service.AuthService,
	validator *validator.Validator,
	cacheService *cache.CacheService,
	jwtExpiration int,
	appMode string,
	encryptionKey string,
	logger zerolog.Logger,
) *AuthHandler {
	return &AuthHandler{
		service:       service,
		validator:     validator,
		cacheService:  cacheService,
		jwtExpiration: jwtExpiration,
		appMode:       appMode,
		encryptionKey: encryptionKey,
		logger:        logger,
	}
}

// Register processes POST /api/v1/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Invalid registration request")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	if err := h.validator.ValidateStruct(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Registration validation failed")
		c.JSON(
			http.StatusBadRequest,
			ValidationErrorResponse(err, ValidationScopeRegister),
		)
		return
	}

	if h.appMode != "debug" && h.appMode != "development" {
		if !h.enforceRegisterRateLimit(c, req.Email) {
			return
		}
	}

	userID, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserAlreadyExists):
			h.logger.Warn().
				Str("email", req.Email).
				Msg("Attempt to register existing user")
			c.JSON(http.StatusConflict, dto.ErrorResponse{
				Error: "user already exists",
			})
			return

		default:
			h.logger.Error().
				Err(err).
				Str("email", req.Email).
				Msg("Failed to register user")
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
				Error: "registration failed",
			})
			return
		}
	}

	h.logger.Info().
		Str("user_id", userID.String()).
		Str("email", req.Email).
		Msg("User registered successfully")

	c.JSON(http.StatusCreated, dto.RegisterResponse{
		UserID:  userID.String(),
		Message: "registration successful",
	})
}

// Login processes POST /api/v1/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Invalid login request")
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	if err := h.validator.ValidateStruct(&req); err != nil {
		h.logger.Warn().Err(err).Msg("Login validation failed")
		c.JSON(
			http.StatusBadRequest,
			ValidationErrorResponse(err, ValidationScopeLogin),
		)
		return
	}

	if h.appMode != "debug" {
		if !h.enforceLoginRateLimit(c, req.Email) {
			return
		}
	}

	loginResponse, err := h.service.Login(c.Request.Context(), &req)
	if err != nil {
		h.logger.Warn().Str("email", req.Email).Err(err).Msg("Login failed")
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid email or password"})
		return
	}

	setCookies(
		c,
		loginResponse.Token,
		h.jwtExpiration,
		h.appMode,
		"login success",
	)
}

// setCookies sets the JWT token in an HttpOnly cookie.
func setCookies(
	c *gin.Context,
	token string,
	expirationSeconds int,
	appMode string,
	message string,
) {
	secure := appMode != "debug"

	c.SetCookie(
		"jwt",
		token,
		expirationSeconds,
		"/",
		"",
		secure,
		true,
	)

	if secure {
		c.Writer.Header().Set("Set-Cookie",
			c.Writer.Header().Get("Set-Cookie")+"; SameSite=Strict")
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: message,
	})
}


