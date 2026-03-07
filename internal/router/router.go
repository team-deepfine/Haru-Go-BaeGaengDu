package router

import (
	"github.com/daewon/haru/internal/handler"
	"github.com/daewon/haru/internal/middleware"
	"github.com/gin-gonic/gin"
)

// New creates a configured Gin engine with all routes registered.
func New(eventHandler *handler.EventHandler, voiceHandler *handler.VoiceHandler) *gin.Engine {
	r := gin.New()

	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	api := r.Group("/api")
	eventHandler.RegisterRoutes(api)
	voiceHandler.RegisterRoutes(api)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
