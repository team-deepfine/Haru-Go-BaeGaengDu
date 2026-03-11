package model

import "errors"

var (
	ErrEventNotFound    = errors.New("event not found")
	ErrInvalidTimeRange = errors.New("end time must be after start time")
	ErrTitleRequired    = errors.New("title is required")
	ErrInvalidTimezone  = errors.New("invalid timezone")

	ErrTextRequired         = errors.New("text is required")
	ErrParsingFailed        = errors.New("failed to parse event from text")
	ErrAIServiceUnavailable = errors.New("AI service unavailable")

	// Auth errors
	ErrInvalidAuthCode          = errors.New("invalid authorization code")
	ErrInvalidIDToken           = errors.New("invalid id token")
	ErrInvalidAccessToken       = errors.New("invalid or expired access token")
	ErrInvalidRefreshToken      = errors.New("invalid or expired refresh token")
	ErrUserNotFound             = errors.New("user not found")
	ErrOAuthProviderUnavailable = errors.New("oauth provider unavailable")
)
