// internal/middleware/middleware.go
package middleware

import (
	"task-forge/internal/cache"
	"task-forge/internal/service"

	"github.com/rs/zerolog"
)

type Middlewares struct {
	Auth *AuthMiddleware
}

func NewMiddlewares(
	cacheService *cache.CacheService,
	jwtManager *service.JWTManager,
	logger zerolog.Logger,
) *Middlewares {
	return &Middlewares{
		Auth: NewAuthMiddleware(
			cacheService,
			jwtManager,
			logger,
		),
	}
}