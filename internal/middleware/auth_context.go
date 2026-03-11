package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const userIDKey = "userID"

// SetUserID stores the authenticated user's ID in the gin context.
func SetUserID(c *gin.Context, userID uuid.UUID) {
	c.Set(userIDKey, userID)
}

// GetUserID retrieves the authenticated user's ID from the gin context.
// Returns the userID and true if present, or uuid.Nil and false otherwise.
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	v, exists := c.Get(userIDKey)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		return uuid.Nil, false
	}
	return id, true
}
