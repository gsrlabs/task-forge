// internal/handler/handler.go
package handler

import (
	"task-forge/internal/cache"
	"task-forge/internal/middleware"
	"task-forge/internal/service"
	"task-forge/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// Handlers aggregates all HTTP handlers.
type Handlers struct {
	Auth  *AuthHandler
	Teams *TeamsHandler
	Tasks *TasksHandler
}

// NewHandlers creates a container with all the handlers.
func NewHandlers(
	services *service.Services,
	validator *validator.Validator,
	cacheService *cache.CacheService,
	jwtManager *service.JWTManager,
	jwtExpiration int,
	appMode string,
	appSecret string,
	logger zerolog.Logger,
) *Handlers {
	return &Handlers{
		Auth: NewAuthHandler(
			services.Auth,
			validator,
			cacheService,
			jwtExpiration,
			appMode,
			appSecret,
			logger,
		),
		Teams: NewTeamsHandler(
			services.Teams,
			validator,
			logger,
		),
		Tasks: NewTasksHandler(
			services.Tasks,
			validator,
			logger,
		),
	}
}

// RegisterRoutes registers all routes in the Gin router.
func (h *Handlers) RegisterRoutes(router *gin.Engine, middlewares *middleware.Middlewares) {
	api := router.Group("/api/v1")
	{

		// Public routes
		api.POST("/register", h.Auth.Register)
		api.POST("/login", h.Auth.Login)

		// Protected routes
		protected := api.Group("")
		protected.Use(middlewares.Auth.Authenticate())
		
		// Teams
		protected.POST("/teams", h.Teams.Create)
		protected.GET("/teams", h.Teams.List)
		protected.POST("/teams/:id/invite", h.Teams.Invite)

		// Tasks
		protected.POST("/tasks", h.Tasks.Create)
		protected.GET("/tasks", h.Tasks.List)
		protected.PUT("/tasks/:id", h.Tasks.Update)
		protected.GET("/tasks/:id/history", h.Tasks.GetHistory)
	}
}