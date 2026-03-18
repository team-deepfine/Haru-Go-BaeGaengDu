package service

import (
	"context"
	"errors"
	"testing"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockDeviceTokenRepository struct {
	upsertFn             func(ctx context.Context, token *model.DeviceToken) error
	deleteByUserAndToken func(ctx context.Context, userID uuid.UUID, token string) error
	findByUserIDFn       func(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error)
	deleteByTokenFn      func(ctx context.Context, token string) error
}

func (m *mockDeviceTokenRepository) Upsert(ctx context.Context, token *model.DeviceToken) error {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, token)
	}
	return nil
}

func (m *mockDeviceTokenRepository) DeleteByUserAndToken(ctx context.Context, userID uuid.UUID, token string) error {
	if m.deleteByUserAndToken != nil {
		return m.deleteByUserAndToken(ctx, userID, token)
	}
	return nil
}

func (m *mockDeviceTokenRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockDeviceTokenRepository) DeleteByUserID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockDeviceTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	if m.deleteByTokenFn != nil {
		return m.deleteByTokenFn(ctx, token)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDeviceTokenService_Register(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())

	t.Run("success", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{}
		svc := NewDeviceTokenService(repo)

		dt, err := svc.Register(context.Background(), userID, "fcm-token-123")
		require.NoError(t, err)
		require.NotNil(t, dt)
		assert.Equal(t, userID, dt.UserID)
		assert.Equal(t, "fcm-token-123", dt.Token)
		assert.NotEqual(t, uuid.Nil, dt.ID)
	})

	t.Run("empty token returns error", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{}
		svc := NewDeviceTokenService(repo)

		_, err := svc.Register(context.Background(), userID, "")
		require.ErrorIs(t, err, model.ErrDeviceTokenRequired)
	})

	t.Run("whitespace-only token returns error", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{}
		svc := NewDeviceTokenService(repo)

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
		svc := NewDeviceTokenService(repo)

		_, err := svc.Register(context.Background(), userID, "valid-token")
		require.ErrorIs(t, err, repoErr)
	})
}

func TestDeviceTokenService_Unregister(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())

	t.Run("success", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{}
		svc := NewDeviceTokenService(repo)

		err := svc.Unregister(context.Background(), userID, "fcm-token-123")
		require.NoError(t, err)
	})

	t.Run("not found error is propagated", func(t *testing.T) {
		repo := &mockDeviceTokenRepository{
			deleteByUserAndToken: func(_ context.Context, _ uuid.UUID, _ string) error {
				return model.ErrDeviceTokenNotFound
			},
		}
		svc := NewDeviceTokenService(repo)

		err := svc.Unregister(context.Background(), userID, "nonexistent")
		require.ErrorIs(t, err, model.ErrDeviceTokenNotFound)
	})
}
