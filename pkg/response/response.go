package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ProblemDetail represents an RFC 7807 error response.
type ProblemDetail struct {
	Type   string            `json:"type"`
	Title  string            `json:"title"`
	Status int               `json:"status"`
	Detail string            `json:"detail"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationError represents a field-level validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error sends an RFC 7807 error response.
func Error(c *gin.Context, status int, detail string) {
	c.JSON(status, ProblemDetail{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	})
}

// ValidationFailed sends a 400 response with field-level errors.
func ValidationFailed(c *gin.Context, errs []ValidationError) {
	c.JSON(http.StatusBadRequest, ProblemDetail{
		Type:   "about:blank",
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: "Validation failed",
		Errors: errs,
	})
}

// JSON sends a successful JSON response.
func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}
