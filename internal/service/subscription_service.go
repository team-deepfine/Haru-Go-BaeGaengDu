package service

import (
	"context"

	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/pkg/appstore"
	"github.com/google/uuid"
)

const defaultVoiceParseLimit = 3

// SubscriptionService defines the interface for subscription business logic.
type SubscriptionService interface {
	VerifyAndActivate(ctx context.Context, userID uuid.UUID, transactionID string) (*dto.SubscriptionResponse, error)
	GetStatus(ctx context.Context, userID uuid.UUID) (*dto.SubscriptionResponse, error)
	HandleNotification(ctx context.Context, signedPayload string) error
	CheckVoiceParseLimit(ctx context.Context, userID uuid.UUID) error
	IncrementVoiceParseCount(ctx context.Context, userID uuid.UUID) error
}

type subscriptionService struct {
	userRepo        repository.UserRepository
	appStoreClient  *appstore.Client
	voiceParseLimit int
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(userRepo repository.UserRepository, appStoreClient *appstore.Client, voiceParseLimit int) SubscriptionService {
	if voiceParseLimit <= 0 {
		voiceParseLimit = defaultVoiceParseLimit
	}
	return &subscriptionService{
		userRepo:        userRepo,
		appStoreClient:  appStoreClient,
		voiceParseLimit: voiceParseLimit,
	}
}
