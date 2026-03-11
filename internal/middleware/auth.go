package middleware

import (
	"net/http"
	"strings"

	"github.com/daewon/haru/pkg/jwt"
	"github.com/daewon/haru/pkg/response"
	"github.com/gin-gonic/gin"
)

// AuthRequired returns a middleware that validates JWT Bearer tokens.
func AuthRequired(jwtManager *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "authorization header required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Error(c, http.StatusUnauthorized, "invalid authorization header format")
			c.Abort()
			return
		}

		userID, err := jwtManager.ValidateToken(parts[1])
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid or expired access token")
			c.Abort()
			return
		}

		SetUserID(c, userID)
		c.Next()
	}
}
