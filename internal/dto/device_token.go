package dto

import (
	"time"

	"github.com/daewon/haru/internal/model"
)

// RegisterDeviceTokenRequest is the request body for device token registration.
type RegisterDeviceTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// UnregisterDeviceTokenRequest is the request body for device token removal.
type UnregisterDeviceTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// DeviceTokenResponse is the API response for a device token.
type DeviceTokenResponse struct {
	ID        string `json:"id"`
	Token     string `json:"token"`
	CreatedAt string `json:"createdAt"`
}

// ToDeviceTokenResponse converts a domain model to an API response DTO.
func ToDeviceTokenResponse(dt *model.DeviceToken) DeviceTokenResponse {
	return DeviceTokenResponse{
		ID:        dt.ID.String(),
		Token:     dt.Token,
		CreatedAt: dt.CreatedAt.Format(time.RFC3339),
	}
}
