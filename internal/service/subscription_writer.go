package service

import (
	"context"
	"fmt"
	"time"

	"github.com/daewon/haru/internal/dto"
	"github.com/daewon/haru/internal/model"

	"github.com/google/uuid"
)

func (s *subscriptionService) VerifyAndActivate(ctx context.Context, userID uuid.UUID, transactionID string) (*dto.SubscriptionResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	if s.appStoreClient == nil {
		return nil, fmt.Errorf("%w: app store client not configured", model.ErrStoreAPIFailed)
	}

	// Verify transaction with App Store Server API v2
	tx, err := s.appStoreClient.VerifyTransaction(transactionID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrStoreAPIFailed, err)
	}

	if tx.IsRevoked {
		return nil, model.ErrInvalidTransaction
	}

	user.SubscriptionStatus = "premium"
	user.OriginalTransactionID = &tx.OriginalTransactionID
	if tx.ExpiresAt != nil {
		user.SubscriptionExpiry = tx.ExpiresAt
	} else {
		// Non-expiring product (lifetime): set far future expiry
		farFuture := time.Now().UTC().AddDate(100, 0, 0)
		user.SubscriptionExpiry = &farFuture
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update subscription: %w", err)
	}

	return dto.ToSubscriptionResponse(user, s.voiceParseLimit), nil
}

func (s *subscriptionService) CheckVoiceParseLimit(ctx context.Context, userID uuid.UUID) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	// Premium users have unlimited access
	if user.IsPremium() {
		return nil
	}

	// Check daily limit for free users
	count := s.todayParseCount(user)
	if count >= s.voiceParseLimit {
		return model.ErrVoiceParseLimit
	}

	return nil
}

func (s *subscriptionService) IncrementVoiceParseCount(ctx context.Context, userID uuid.UUID) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	if user.IsPremium() {
		return nil
	}

	count := s.todayParseCount(user)
	user.VoiceParseCount = count + 1
	now := time.Now().UTC()
	user.VoiceParseDate = &now

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("update voice parse count: %w", err)
	}

	return nil
}

func (s *subscriptionService) todayParseCount(user *model.User) int {
	kst := time.FixedZone("KST", 9*60*60)
	today := time.Now().In(kst).Truncate(24 * time.Hour)

	if user.VoiceParseDate == nil || !user.VoiceParseDate.In(kst).Truncate(24*time.Hour).Equal(today) {
		return 0
	}
	return user.VoiceParseCount
}
