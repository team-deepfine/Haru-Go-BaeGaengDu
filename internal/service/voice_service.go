package service

import (
	"context"

	"github.com/daewon/haru/internal/dto"
)

// VoiceParsingService defines the interface for voice text parsing.
type VoiceParsingService interface {
	ParseVoice(ctx context.Context, input ParseVoiceInput) (*dto.ParseVoiceResponse, error)
}

// ParseVoiceInput is the service-layer input for voice parsing.
type ParseVoiceInput struct {
	Text string
}
