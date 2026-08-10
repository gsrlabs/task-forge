package middleware

import (
	"context"
	"net/http"
	"time"

	"task-forge/internal/cache"
	"task-forge/internal/dto"
	"task-forge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	ipRateLimit    = 100
	ipRateWindow   = time.Minute
	userRateLimit  = 100
	userRateWindow = time.Minute

	jwtCookieName = "jwt"
)

type AuthMiddleware struct {
	cache      *cache.CacheService
	jwtManager *service.JWTManager
	logger     zerolog.Logger
}

func NewAuthMiddleware(
	cacheService *cache.CacheService,
	jwtManager *service.JWTManager,
	logger zerolog.Logger,
) *AuthMiddleware {
	return &AuthMiddleware{
		cache:      cacheService,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ip := c.ClientIP()

		if !m.allowIP(ctx, c, ip) {
			return
		}

		claims, ok := m.authenticate(c, ip)
		if !ok {
			return
		}

		userID := claims.UserID.String()

		if !m.allowUser(ctx, c, userID, ip) {
			return
		}

		SetUserIdentity(c, claims.UserID, claims.Email)

		c.Next()
	}
}

func (m *AuthMiddleware) allowIP(
	ctx context.Context,
	c *gin.Context,
	ip string,
) bool {
	key := "rl:auth_mw:ip:" + ip

	allowed, err := m.cache.Allow(
		ctx,
		key,
		ipRateLimit,
		ipRateWindow,
	)
	if err != nil {
		m.logger.Error().
			Err(err).
			Str("ip", ip).
			Msg("IP rate limit check failed; failing open")

		return true
	}

	if allowed {
		return true
	}

	m.logger.Warn().
		Str("ip", ip).
		Str("path", c.Request.URL.Path).
		Msg("IP rate limit exceeded")

	tooManyRequests(c)
	return false
}

func (m *AuthMiddleware) authenticate(
	c *gin.Context,
	ip string,
) (*dto.JWTClaims, bool) {
	token, err := c.Cookie(jwtCookieName)
	if err != nil || token == "" {
		m.logger.Debug().
			Str("ip", ip).
			Str("path", c.Request.URL.Path).
			Msg("JWT cookie not found")

		unauthorized(c, "unauthorized")
		return nil, false
	}

	claims, err := m.jwtManager.ValidateToken(token)
	if err != nil {
		m.logger.Warn().
			Err(err).
			Str("ip", ip).
			Str("path", c.Request.URL.Path).
			Msg("JWT validation failed")

		unauthorized(c, "invalid or expired token")
		return nil, false
	}

	return claims, true
}

func (m *AuthMiddleware) allowUser(
	ctx context.Context,
	c *gin.Context,
	userID string,
	ip string,
) bool {
	key := "rl:auth_mw:user:" + userID

	allowed, err := m.cache.Allow(
		ctx,
		key,
		userRateLimit,
		userRateWindow,
	)
	if err != nil {
		m.logger.Error().
			Err(err).
			Str("user_id", userID).
			Str("ip", ip).
			Msg("User rate limit check failed; failing open")

		return true
	}

	if allowed {
		return true
	}

	m.logger.Warn().
		Str("user_id", userID).
		Str("ip", ip).
		Str("path", c.Request.URL.Path).
		Msg("User rate limit exceeded")

	tooManyRequests(c)
	return false
}

func unauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
		Error: message,
	})
}

func tooManyRequests(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.ErrorResponse{
		Error: "too many requests, please slow down",
	})
}