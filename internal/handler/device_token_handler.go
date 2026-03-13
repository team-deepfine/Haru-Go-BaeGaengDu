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

// DeviceTokenHandler handles HTTP requests for device token management.
type DeviceTokenHandler struct {
	svc service.DeviceTokenService
}

// NewDeviceTokenHandler creates a new DeviceTokenHandler.
func NewDeviceTokenHandler(svc service.DeviceTokenService) *DeviceTokenHandler {
	return &DeviceTokenHandler{svc: svc}
}

// RegisterRoutes registers device token routes on a Gin router group.
func (h *DeviceTokenHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/devices", h.Register)
	rg.DELETE("/devices", h.Unregister)
}

// Register handles POST /api/devices.
func (h *DeviceTokenHandler) Register(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	var req dto.RegisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "token is required")
		return
	}

	dt, err := h.svc.Register(c.Request.Context(), userID, req.Token)
	if err != nil {
		handleDeviceTokenError(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, dto.ToDeviceTokenResponse(dt))
}

// Unregister handles DELETE /api/devices.
func (h *DeviceTokenHandler) Unregister(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}

	var req dto.UnregisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "token is required")
		return
	}

	if err := h.svc.Unregister(c.Request.Context(), userID, req.Token); err != nil {
		handleDeviceTokenError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func handleDeviceTokenError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrDeviceTokenRequired):
		response.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, model.ErrDeviceTokenNotFound):
		response.Error(c, http.StatusNotFound, err.Error())
	default:
		slog.Error("device token error", "error", err)
		response.Error(c, http.StatusInternalServerError, "internal server error")
	}
}
