package service

import (
	"context"
	"strings"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/google/uuid"
)

// DeviceTokenService defines the interface for device token management.
type DeviceTokenService interface {
	Register(ctx context.Context, userID uuid.UUID, token string) (*model.DeviceToken, error)
	Unregister(ctx context.Context, userID uuid.UUID, token string) error
}

type deviceTokenService struct {
	repo repository.DeviceTokenRepository
}

// NewDeviceTokenService creates a new DeviceTokenService.
func NewDeviceTokenService(repo repository.DeviceTokenRepository) DeviceTokenService {
	return &deviceTokenService{repo: repo}
}

func (s *deviceTokenService) Register(ctx context.Context, userID uuid.UUID, token string) (*model.DeviceToken, error) {
	if strings.TrimSpace(token) == "" {
		return nil, model.ErrDeviceTokenRequired
	}

	dt := &model.DeviceToken{
		ID:     uuid.Must(uuid.NewV7()),
		UserID: userID,
		Token:  token,
	}
	if err := s.repo.Upsert(ctx, dt); err != nil {
		return nil, err
	}
	return dt, nil
}

func (s *deviceTokenService) Unregister(ctx context.Context, userID uuid.UUID, token string) error {
	return s.repo.DeleteByUserAndToken(ctx, userID, token)
}
