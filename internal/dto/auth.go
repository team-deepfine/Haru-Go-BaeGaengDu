package dto

import (
	"time"

	"github.com/daewon/haru/internal/model"
)

// AppleLoginRequest is the request body for Apple Sign In.
type AppleLoginRequest struct {
	Code string `json:"code" binding:"required"`
}

// KakaoLoginRequest is the request body for Kakao login.
type KakaoLoginRequest struct {
	AccessToken string `json:"accessToken" binding:"required"`
}

// LogoutRequest is the request body for logout.
type LogoutRequest struct {
	DeviceToken string `json:"deviceToken"`
}

// DeleteAccountRequest is the request body for account deletion.
// Apple users must provide a fresh authorization code for token revocation.
type DeleteAccountRequest struct {
	Code string `json:"code"`
}

// RefreshRequest is the request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// AuthResponse is the response body for login and token refresh.
type AuthResponse struct {
	AccessToken  string        `json:"accessToken"`
	RefreshToken string        `json:"refreshToken"`
	ExpiresIn    int           `json:"expiresIn"`
	User         *UserResponse `json:"user,omitempty"`
}

// UserResponse is the API response representation of a user.
type UserResponse struct {
	ID                 string  `json:"id"`
	Provider           string  `json:"provider"`
	Email              *string `json:"email,omitempty"`
	Nickname           *string `json:"nickname,omitempty"`
	ProfileImage       *string `json:"profileImage,omitempty"`
	SubscriptionStatus string  `json:"subscriptionStatus"`
	SubscriptionExpiry *string `json:"subscriptionExpiry,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	LastLoginAt        *string `json:"lastLoginAt,omitempty"`
}

// ToUserResponse converts a domain User model to an API response DTO.
func ToUserResponse(u *model.User) *UserResponse {
	resp := &UserResponse{
		ID:                 u.ID.String(),
		Provider:           u.Provider,
		Email:              u.Email,
		Nickname:           u.Nickname,
		ProfileImage:       u.ProfileImage,
		SubscriptionStatus: u.SubscriptionStatus,
		CreatedAt:          u.CreatedAt.Format(time.RFC3339),
	}
	if u.SubscriptionExpiry != nil {
		s := u.SubscriptionExpiry.Format(time.RFC3339)
		resp.SubscriptionExpiry = &s
	}
	if u.LastLoginAt != nil {
		s := u.LastLoginAt.Format(time.RFC3339)
		resp.LastLoginAt = &s
	}
	return resp
}
