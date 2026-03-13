package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/handler"
	"github.com/daewon/haru/internal/middleware"
	"github.com/daewon/haru/internal/model"
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

// --- Mock AuthService ---

type mockAuthService struct {
	appleLoginFn     func(ctx context.Context, code string) (*model.User, *jwt.TokenPair, error)
	kakaoLoginFn     func(ctx context.Context, code string) (*model.User, *jwt.TokenPair, error)
	refreshTokenFn   func(ctx context.Context, refreshToken string) (*jwt.TokenPair, error)
	logoutFn         func(ctx context.Context, userID uuid.UUID) error
	getCurrentUserFn func(ctx context.Context, userID uuid.UUID) (*model.User, error)
	deleteAccountFn  func(ctx context.Context, userID uuid.UUID, authCode string) error
}

func (m *mockAuthService) AppleLogin(ctx context.Context, code string) (*model.User, *jwt.TokenPair, error) {
	return m.appleLoginFn(ctx, code)
}

func (m *mockAuthService) KakaoLogin(ctx context.Context, code string) (*model.User, *jwt.TokenPair, error) {
	return m.kakaoLoginFn(ctx, code)
}

func (m *mockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	return m.refreshTokenFn(ctx, refreshToken)
}

func (m *mockAuthService) Logout(ctx context.Context, userID uuid.UUID) error {
	return m.logoutFn(ctx, userID)
}

func (m *mockAuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return m.getCurrentUserFn(ctx, userID)
}

func (m *mockAuthService) DeleteAccount(ctx context.Context, userID uuid.UUID, authCode string) error {
	return m.deleteAccountFn(ctx, userID, authCode)
}

// --- Test helpers ---

func newTestUser() *model.User {
	now := time.Now()
	email := "test@example.com"
	return &model.User{
		ID:          uuid.New(),
		Provider:    "apple",
		ProviderSub: "001234.abcdef",
		Email:       &email,
		CreatedAt:   now,
		UpdatedAt:   now,
		LastLoginAt: &now,
	}
}

func newTestTokenPair() *jwt.TokenPair {
	return &jwt.TokenPair{
		AccessToken:  "access-token-string",
		RefreshToken: "refresh-token-string",
		ExpiresIn:    3600,
	}
}

func setupPublicRouter(svc *mockAuthService) *gin.Engine {
	r := gin.New()
	h := handler.NewAuthHandler(svc)
	api := r.Group("/api")
	h.RegisterPublicRoutes(api)
	return r
}

func setupProtectedRouter(svc *mockAuthService) *gin.Engine {
	r := gin.New()
	h := handler.NewAuthHandler(svc)
	api := r.Group("/api")
	h.RegisterProtectedRoutes(api)
	return r
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	body, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(body)
}

// --- AppleLogin tests ---

func TestAppleLogin_ValidRequest(t *testing.T) {
	user := newTestUser()
	tokenPair := newTestTokenPair()

	svc := &mockAuthService{
		appleLoginFn: func(_ context.Context, code string) (*model.User, *jwt.TokenPair, error) {
			assert.Equal(t, "valid-auth-code", code)
			return user, tokenPair, nil
		},
	}

	router := setupPublicRouter(svc)
	reqBody := jsonBody(t, dto.AppleLoginRequest{Code: "valid-auth-code"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/apple", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp dto.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, tokenPair.AccessToken, resp.AccessToken)
	assert.Equal(t, tokenPair.RefreshToken, resp.RefreshToken)
	assert.Equal(t, tokenPair.ExpiresIn, resp.ExpiresIn)
	require.NotNil(t, resp.User)
	assert.Equal(t, user.ID.String(), resp.User.ID)
	assert.Equal(t, "apple", resp.User.Provider)
	assert.Equal(t, user.Email, resp.User.Email)
}

func TestAppleLogin_MissingCode(t *testing.T) {
	svc := &mockAuthService{}
	router := setupPublicRouter(svc)

	// Send empty JSON body (no code field).
	reqBody := jsonBody(t, map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/apple", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var problem response.ProblemDetail
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, problem.Status)
	assert.Equal(t, "code is required", problem.Detail)
}

func TestAppleLogin_ServiceReturnsErrInvalidAuthCode(t *testing.T) {
	svc := &mockAuthService{
		appleLoginFn: func(_ context.Context, _ string) (*model.User, *jwt.TokenPair, error) {
			return nil, nil, model.ErrInvalidAuthCode
		},
	}

	router := setupPublicRouter(svc)
	reqBody := jsonBody(t, dto.AppleLoginRequest{Code: "bad-code"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/apple", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var problem response.ProblemDetail
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.Status)
	assert.Contains(t, problem.Detail, "invalid authorization code")
}

// --- Refresh tests ---

func TestRefresh_ValidRequest(t *testing.T) {
	newTokenPair := &jwt.TokenPair{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresIn:    3600,
	}

	svc := &mockAuthService{
		refreshTokenFn: func(_ context.Context, refreshToken string) (*jwt.TokenPair, error) {
			assert.Equal(t, "valid-refresh-token", refreshToken)
			return newTokenPair, nil
		},
	}

	router := setupPublicRouter(svc)
	reqBody := jsonBody(t, dto.RefreshRequest{RefreshToken: "valid-refresh-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp dto.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, newTokenPair.AccessToken, resp.AccessToken)
	assert.Equal(t, newTokenPair.RefreshToken, resp.RefreshToken)
	assert.Equal(t, newTokenPair.ExpiresIn, resp.ExpiresIn)
	assert.Nil(t, resp.User) // Refresh does not return user data.
}

func TestRefresh_MissingRefreshToken(t *testing.T) {
	svc := &mockAuthService{}
	router := setupPublicRouter(svc)

	reqBody := jsonBody(t, map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var problem response.ProblemDetail
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, problem.Status)
	assert.Equal(t, "refreshToken is required", problem.Detail)
}

func TestRefresh_ServiceReturnsErrInvalidRefreshToken(t *testing.T) {
	svc := &mockAuthService{
		refreshTokenFn: func(_ context.Context, _ string) (*jwt.TokenPair, error) {
			return nil, model.ErrInvalidRefreshToken
		},
	}

	router := setupPublicRouter(svc)
	reqBody := jsonBody(t, dto.RefreshRequest{RefreshToken: "expired-refresh-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var problem response.ProblemDetail
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.Status)
	assert.Contains(t, problem.Detail, "invalid or expired refresh token")
}

// --- Me tests ---

func TestMe_ValidUser(t *testing.T) {
	user := newTestUser()
	userID := user.ID

	svc := &mockAuthService{
		getCurrentUserFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
			assert.Equal(t, userID, id)
			return user, nil
		},
	}

	router := gin.New()
	h := handler.NewAuthHandler(svc)
	router.GET("/api/auth/me", func(c *gin.Context) {
		middleware.SetUserID(c, userID)
		c.Next()
	}, h.Me)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp dto.UserResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, userID.String(), resp.ID)
	assert.Equal(t, "apple", resp.Provider)
	assert.Equal(t, user.Email, resp.Email)
}

func TestMe_NoUserIDInContext(t *testing.T) {
	svc := &mockAuthService{}

	router := setupProtectedRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var problem response.ProblemDetail
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.Status)
	assert.Equal(t, "authentication required", problem.Detail)
}

// --- Logout tests ---

func TestLogout_Valid(t *testing.T) {
	userID := uuid.New()

	svc := &mockAuthService{
		logoutFn: func(_ context.Context, id uuid.UUID) error {
			assert.Equal(t, userID, id)
			return nil
		},
	}

	router := gin.New()
	h := handler.NewAuthHandler(svc)
	router.POST("/api/auth/logout", func(c *gin.Context) {
		middleware.SetUserID(c, userID)
		c.Next()
	}, h.Logout)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.Bytes())
}

// --- DeleteAccount tests ---

func TestDeleteAccount_Valid(t *testing.T) {
	userID := uuid.New()

	svc := &mockAuthService{
		deleteAccountFn: func(_ context.Context, id uuid.UUID, authCode string) error {
			assert.Equal(t, userID, id)
			assert.Empty(t, authCode)
			return nil
		},
	}

	router := gin.New()
	h := handler.NewAuthHandler(svc)
	router.DELETE("/api/auth/account", func(c *gin.Context) {
		middleware.SetUserID(c, userID)
		c.Next()
	}, h.DeleteAccount)

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.Bytes())
}

func TestDeleteAccount_WithAppleCode(t *testing.T) {
	userID := uuid.New()

	svc := &mockAuthService{
		deleteAccountFn: func(_ context.Context, id uuid.UUID, authCode string) error {
			assert.Equal(t, userID, id)
			assert.Equal(t, "apple-auth-code", authCode)
			return nil
		},
	}

	router := gin.New()
	h := handler.NewAuthHandler(svc)
	router.DELETE("/api/auth/account", func(c *gin.Context) {
		middleware.SetUserID(c, userID)
		c.Next()
	}, h.DeleteAccount)

	reqBody := jsonBody(t, dto.DeleteAccountRequest{Code: "apple-auth-code"})
	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.Bytes())
}
