package handler

import (
	"task-forge/internal/cache"
	"task-forge/internal/service"
	"task-forge/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// Handlers aggregates all HTTP handlers
type Handlers struct {
	Auth *AuthHandler
}

// NewHandlers creates a container with all handlers.
func NewHandlers(
	services *service.Services,
	validator *validator.Validator,
	cacheService *cache.CacheService,
	jwtExpiration int,
	appMode string,
	encryptionKey string,
	logger zerolog.Logger,
) *Handlers {
	return &Handlers{
		Auth: NewAuthHandler(
			services.Auth,
			validator,
			cacheService,
			jwtExpiration,
			appMode,
			encryptionKey,
			logger,
		),
	}
}

// RegisterRoutes registers all routes in the Gin router.
func (h *Handlers) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		// Auth routes
		api.POST("/register", h.Auth.Register)
		api.POST("/login", h.Auth.Login)
	}
}