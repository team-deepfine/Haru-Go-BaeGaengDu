package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/middleware"
	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/service"
	"github.com/daewon/haru/pkg/response"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles HTTP requests for authentication.
type AuthHandler struct {
	svc service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// RegisterPublicRoutes registers auth routes that do NOT require authentication.
func (h *AuthHandler) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/apple", h.AppleLogin)
	rg.POST("/auth/kakao", h.KakaoLogin)
	rg.POST("/auth/refresh", h.Refresh)
}

// RegisterProtectedRoutes registers auth routes that require authentication.
func (h *AuthHandler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.GET("/auth/me", h.Me)
	rg.POST("/auth/logout", h.Logout)
	rg.DELETE("/auth/account", h.DeleteAccount)
}

// AppleLogin handles POST /api/auth/apple.
func (h *AuthHandler) AppleLogin(c *gin.Context) {
	var req dto.AppleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "code is required")
		return
	}

	user, tokenPair, err := h.svc.AppleLogin(c.Request.Context(), req.Code)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		User:         dto.ToUserResponse(user),
	})
}

// KakaoLogin handles POST /api/auth/kakao.
func (h *AuthHandler) KakaoLogin(c *gin.Context) {
	var req dto.KakaoLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "code is required")
		return
	}

	user, tokenPair, err := h.svc.KakaoLogin(c.Request.Context(), req.Code)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		User:         dto.ToUserResponse(user),
	})
}

// Refresh handles POST /api/auth/refresh.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "refreshToken is required")
		return
	}

	tokenPair, err := h.svc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	})
}

// Me handles GET /api/auth/me.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.svc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.ToUserResponse(user))
}

// Logout handles POST /api/auth/logout.
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.svc.Logout(c.Request.Context(), userID); err != nil {
		slog.Error("logout failed", "error", err)
		response.Error(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.Status(http.StatusNoContent)
}

// DeleteAccount handles DELETE /api/auth/account.
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	var req dto.DeleteAccountRequest
	// body가 없어도 허용 (Apple이 아닌 사용자)
	_ = c.ShouldBindJSON(&req)

	if err := h.svc.DeleteAccount(c.Request.Context(), userID, req.Code); err != nil {
		handleAuthError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func handleAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrInvalidAuthCode):
		slog.Error("auth code exchange failed", "error", err)
		response.Error(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, model.ErrInvalidRefreshToken):
		response.Error(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, model.ErrInvalidAccessToken):
		response.Error(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, model.ErrUserNotFound):
		response.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, model.ErrOAuthProviderUnavailable):
		response.Error(c, http.StatusBadGateway, err.Error())
	default:
		slog.Error("auth error", "error", err)
		response.Error(c, http.StatusInternalServerError, "internal server error")
	}
}
