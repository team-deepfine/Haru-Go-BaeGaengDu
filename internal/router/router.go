package router

import (
	"github.com/daewon/haru/internal/handler"
	"github.com/daewon/haru/internal/middleware"
	"github.com/daewon/haru/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// New creates a configured Gin engine with all routes registered.
func New(
	jwtManager *jwt.Manager,
	authHandler *handler.AuthHandler,
	eventHandler *handler.EventHandler,
	voiceHandler *handler.VoiceHandler,
	deviceTokenHandler *handler.DeviceTokenHandler,
) *gin.Engine {
	r := gin.New()

	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")

	// Public routes (no auth required)
	authHandler.RegisterPublicRoutes(api)

	// Protected routes (auth required)
	protected := api.Group("")
	protected.Use(middleware.AuthRequired(jwtManager))
	authHandler.RegisterProtectedRoutes(protected)
	eventHandler.RegisterRoutes(protected)
	voiceHandler.RegisterRoutes(protected)
	deviceTokenHandler.RegisterRoutes(protected)

	return r
}
