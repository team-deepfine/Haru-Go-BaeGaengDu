package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceTokenService_Register(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())

	t.Run("success", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{}
		svc := service.NewDeviceTokenService(repo)

		dt, err := svc.Register(context.Background(), userID, "fcm-token-123")
		require.NoError(t, err)
		require.NotNil(t, dt)
		assert.Equal(t, userID, dt.UserID)
		assert.Equal(t, "fcm-token-123", dt.Token)
		assert.NotEqual(t, uuid.Nil, dt.ID)
	})

	t.Run("empty token returns error", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{}
		svc := service.NewDeviceTokenService(repo)

		_, err := svc.Register(context.Background(), userID, "")
		require.ErrorIs(t, err, model.ErrDeviceTokenRequired)
	})

	t.Run("whitespace-only token returns error", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{}
		svc := service.NewDeviceTokenService(repo)

		_, err := svc.Register(context.Background(), userID, "   ")
		require.ErrorIs(t, err, model.ErrDeviceTokenRequired)
	})

	t.Run("repo error is propagated", func(t *testing.T) {
		repoErr := errors.New("db connection failed")
		repo := &mockDeviceTokenRepository{
			upsertFn: func(_ context.Context, _ *model.DeviceToken) error {
				return repoErr
			},
		}
		svc := service.NewDeviceTokenService(repo)

		_, err := svc.Register(context.Background(), userID, "valid-token")
		require.ErrorIs(t, err, repoErr)
	})
}

func TestDeviceTokenService_Unregister(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())

	t.Run("success", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{}
		svc := service.NewDeviceTokenService(repo)

		err := svc.Unregister(context.Background(), userID, "fcm-token-123")
		require.NoError(t, err)
	})

	t.Run("not found error is propagated", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{
			deleteByUserAndTokenFn: func(_ context.Context, _ uuid.UUID, _ string) error {
				return model.ErrDeviceTokenNotFound
			},
		}
		svc := service.NewDeviceTokenService(repo)

		err := svc.Unregister(context.Background(), userID, "nonexistent")
		require.ErrorIs(t, err, model.ErrDeviceTokenNotFound)
	})
}
