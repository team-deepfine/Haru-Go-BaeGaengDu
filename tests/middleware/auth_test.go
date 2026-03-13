package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"github.com/daewon/haru/internal/middleware"
	"github.com/daewon/haru/pkg/jwt"
	"github.com/daewon/haru/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestJWTManager() *jwt.Manager {
	return jwt.NewManager("test-secret", time.Hour, 720*time.Hour)
}

func setupAuthRouter(jwtManager *jwt.Manager) *gin.Engine {
	r := gin.New()
	r.GET("/protected", middleware.AuthRequired(jwtManager), func(c *gin.Context) {
		userID, ok := middleware.GetUserID(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "userID not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"userID": userID.String()})
	})
	return r
}

func TestAuthRequired_ValidBearerToken(t *testing.T) {
	jwtManager := newTestJWTManager()
	router := setupAuthRouter(jwtManager)
	userID := uuid.New()

	tokenPair, err := jwtManager.GenerateTokenPair(userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, userID.String(), body["userID"])
}

func TestAuthRequired_MissingAuthorizationHeader(t *testing.T) {
	jwtManager := newTestJWTManager()
	router := setupAuthRouter(jwtManager)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var problem response.ProblemDetail
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.Status)
	assert.Equal(t, "authorization header required", problem.Detail)
}

func TestAuthRequired_InvalidFormat_NoBearerPrefix(t *testing.T) {
	jwtManager := newTestJWTManager()
	router := setupAuthRouter(jwtManager)

	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "token without Bearer prefix",
			header: "some-token-value",
		},
		{
			name:   "Basic auth instead of Bearer",
			header: "Basic dXNlcjpwYXNz",
		},
		{
			name:   "lowercase bearer",
			header: "bearer some-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var problem response.ProblemDetail
			err := json.Unmarshal(w.Body.Bytes(), &problem)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, problem.Status)
			assert.Equal(t, "invalid authorization header format", problem.Detail)
		})
	}
}

func TestAuthRequired_ExpiredToken(t *testing.T) {
	jwtManager := newTestJWTManager()
	router := setupAuthRouter(jwtManager)
	userID := uuid.New()

	// Create a token that is expired well beyond the 30s leeway.
	claims := jwtlib.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwtlib.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-1 * time.Hour)),
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var problem response.ProblemDetail
	err = json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.Status)
	assert.Equal(t, "invalid or expired access token", problem.Detail)
}

func TestAuthRequired_MalformedToken(t *testing.T) {
	jwtManager := newTestJWTManager()
	router := setupAuthRouter(jwtManager)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "completely invalid string",
			token: "not-a-jwt-token-at-all",
		},
		{
			name:  "empty token after Bearer",
			token: "",
		},
		{
			name:  "partial JWT format",
			token: "eyJhbGciOiJIUzI1NiJ9.tampered.payload",
		},
		{
			name: "signed with different secret",
			token: func() string {
				otherManager := jwt.NewManager("different-secret", time.Hour, 720*time.Hour)
				pair, _ := otherManager.GenerateTokenPair(uuid.New())
				return pair.AccessToken
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var problem response.ProblemDetail
			err := json.Unmarshal(w.Body.Bytes(), &problem)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, problem.Status)
			assert.Equal(t, "invalid or expired access token", problem.Detail)
		})
	}
}
