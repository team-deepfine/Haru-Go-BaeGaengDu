package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockSubUserRepository struct {
	user     *model.User
	updateFn func(ctx context.Context, user *model.User) error
}

var _ repository.UserRepository = (*mockSubUserRepository)(nil)

func (m *mockSubUserRepository) Create(_ context.Context, _ *model.User) error { return nil }
func (m *mockSubUserRepository) FindByID(_ context.Context, _ uuid.UUID) (*model.User, error) {
	if m.user == nil {
		return nil, model.ErrUserNotFound
	}
	return m.user, nil
}
func (m *mockSubUserRepository) FindByProviderSub(_ context.Context, _, _ string) (*model.User, error) {
	return nil, model.ErrUserNotFound
}
func (m *mockSubUserRepository) Update(ctx context.Context, user *model.User) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, user)
	}
	m.user = user
	return nil
}
func (m *mockSubUserRepository) FindByOriginalTransactionID(_ context.Context, _ string) (*model.User, error) {
	return nil, model.ErrUserNotFound
}
func (m *mockSubUserRepository) Delete(_ context.Context, _ uuid.UUID) error { return nil }

// ---------------------------------------------------------------------------
// GetStatus
// ---------------------------------------------------------------------------

func TestSubscriptionService_GetStatus(t *testing.T) {
	t.Run("returns free status for new user", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "free",
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		resp, err := svc.GetStatus(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, "free", resp.SubscriptionStatus)
		assert.Equal(t, 3, resp.VoiceParseLimit)
		assert.Equal(t, 3, resp.VoiceParseRemaining)
		assert.Equal(t, 0, resp.VoiceParseCount)
	})

	t.Run("returns premium status for active subscription", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		expiry := time.Now().Add(30 * 24 * time.Hour)
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "premium",
				SubscriptionExpiry: &expiry,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		resp, err := svc.GetStatus(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, "premium", resp.SubscriptionStatus)
		assert.NotNil(t, resp.ExpiresAt)
	})

	t.Run("expired premium stays premium until webhook downgrades", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		expired := time.Now().Add(-1 * time.Hour)
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "premium",
				SubscriptionExpiry: &expired,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		resp, err := svc.GetStatus(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, "premium", resp.SubscriptionStatus)
		// Status should NOT be changed by GetStatus — only webhook changes it
		assert.Equal(t, "premium", repo.user.SubscriptionStatus)
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		repo := &mockSubUserRepository{user: nil}
		svc := service.NewSubscriptionService(repo, nil, 3)

		_, err := svc.GetStatus(context.Background(), uuid.Must(uuid.NewV7()))
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// CheckVoiceParseLimit
// ---------------------------------------------------------------------------

func TestSubscriptionService_CheckVoiceParseLimit(t *testing.T) {
	t.Run("premium user passes limit check", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		expiry := time.Now().Add(30 * 24 * time.Hour)
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "premium",
				SubscriptionExpiry: &expiry,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		err := svc.CheckVoiceParseLimit(context.Background(), userID)
		require.NoError(t, err)
	})

	t.Run("free user with count below limit passes", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		now := time.Now().UTC()
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "free",
				VoiceParseCount:    2,
				VoiceParseDate:     &now,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		err := svc.CheckVoiceParseLimit(context.Background(), userID)
		require.NoError(t, err)
	})

	t.Run("free user at limit returns ErrVoiceParseLimit", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		now := time.Now().UTC()
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "free",
				VoiceParseCount:    3,
				VoiceParseDate:     &now,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		err := svc.CheckVoiceParseLimit(context.Background(), userID)
		require.ErrorIs(t, err, model.ErrVoiceParseLimit)
	})

	t.Run("free user count resets on new day", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		yesterday := time.Now().Add(-24 * time.Hour).UTC()
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "free",
				VoiceParseCount:    3,
				VoiceParseDate:     &yesterday,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		err := svc.CheckVoiceParseLimit(context.Background(), userID)
		require.NoError(t, err)
	})

	t.Run("expired premium user still has unlimited access until webhook downgrades", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		expired := time.Now().Add(-1 * time.Hour)
		now := time.Now().UTC()
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "premium",
				SubscriptionExpiry: &expired,
				VoiceParseCount:    3,
				VoiceParseDate:     &now,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		err := svc.CheckVoiceParseLimit(context.Background(), userID)
		require.NoError(t, err) // premium status is trusted until webhook changes it
	})
}

// ---------------------------------------------------------------------------
// IncrementVoiceParseCount
// ---------------------------------------------------------------------------

func TestSubscriptionService_IncrementVoiceParseCount(t *testing.T) {
	t.Run("increments count for free user", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		now := time.Now().UTC()
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "free",
				VoiceParseCount:    1,
				VoiceParseDate:     &now,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		err := svc.IncrementVoiceParseCount(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, 2, repo.user.VoiceParseCount)
	})

	t.Run("does not increment for premium user", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		expiry := time.Now().Add(30 * 24 * time.Hour)
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "premium",
				SubscriptionExpiry: &expiry,
				VoiceParseCount:    0,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		err := svc.IncrementVoiceParseCount(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, 0, repo.user.VoiceParseCount)
	})

	t.Run("resets count on new day and starts at 1", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		yesterday := time.Now().Add(-24 * time.Hour).UTC()
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "free",
				VoiceParseCount:    3,
				VoiceParseDate:     &yesterday,
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		err := svc.IncrementVoiceParseCount(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, 1, repo.user.VoiceParseCount)
	})
}

// ---------------------------------------------------------------------------
// VerifyAndActivate
// ---------------------------------------------------------------------------

func TestSubscriptionService_VerifyAndActivate(t *testing.T) {
	t.Run("returns error when app store client is nil", func(t *testing.T) {
		userID := uuid.Must(uuid.NewV7())
		repo := &mockSubUserRepository{
			user: &model.User{
				ID:                 userID,
				SubscriptionStatus: "free",
			},
		}
		svc := service.NewSubscriptionService(repo, nil, 3)

		_, err := svc.VerifyAndActivate(context.Background(), userID, "tx-123")
		require.Error(t, err)
		require.ErrorIs(t, err, model.ErrStoreAPIFailed)
	})
}
