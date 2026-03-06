package middleware

import (
	"log/slog"
	"net/http"

	"github.com/daewon/haru/pkg/response"
	"github.com/gin-gonic/gin"
)

// Recovery returns a Gin middleware that recovers from panics.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err)
				response.Error(c, http.StatusInternalServerError, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
