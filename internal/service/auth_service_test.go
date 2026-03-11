package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/pkg/jwt"
	"github.com/daewon/haru/pkg/oauth"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Manual mocks
// ---------------------------------------------------------------------------

// mockUserRepository implements repository.UserRepository for testing.
type mockUserRepository struct {
	createFn          func(ctx context.Context, user *model.User) error
	findByIDFn        func(ctx context.Context, id uuid.UUID) (*model.User, error)
	findByProviderSub func(ctx context.Context, provider, providerSub string) (*model.User, error)
	updateFn          func(ctx context.Context, user *model.User) error
	deleteFn          func(ctx context.Context, id uuid.UUID) error
}

func (m *mockUserRepository) Create(ctx context.Context, user *model.User) error {
	if m.createFn != nil {
		return m.createFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, model.ErrUserNotFound
}

func (m *mockUserRepository) FindByProviderSub(ctx context.Context, provider, providerSub string) (*model.User, error) {
	if m.findByProviderSub != nil {
		return m.findByProviderSub(ctx, provider, providerSub)
	}
	return nil, nil
}

func (m *mockUserRepository) Update(ctx context.Context, user *model.User) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

// mockTokenRepository implements repository.TokenRepository for testing.
type mockTokenRepository struct {
	createFn       func(ctx context.Context, token *model.RefreshToken) error
	findByTokenFn  func(ctx context.Context, token string) (*model.RefreshToken, error)
	deleteByTokenFn  func(ctx context.Context, token string) error
	deleteByUserIDFn func(ctx context.Context, userID uuid.UUID) error
	deleteExpiredFn  func(ctx context.Context) error
}

func (m *mockTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	if m.createFn != nil {
		return m.createFn(ctx, token)
	}
	return nil
}

func (m *mockTokenRepository) FindByToken(ctx context.Context, token string) (*model.RefreshToken, error) {
	if m.findByTokenFn != nil {
		return m.findByTokenFn(ctx, token)
	}
	return nil, model.ErrInvalidRefreshToken
}

func (m *mockTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	if m.deleteByTokenFn != nil {
		return m.deleteByTokenFn(ctx, token)
	}
	return nil
}

func (m *mockTokenRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if m.deleteByUserIDFn != nil {
		return m.deleteByUserIDFn(ctx, userID)
	}
	return nil
}

func (m *mockTokenRepository) DeleteExpired(ctx context.Context) error {
	if m.deleteExpiredFn != nil {
		return m.deleteExpiredFn(ctx)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestJWTManager() *jwt.Manager {
	return jwt.NewManager("test-secret", time.Hour, 720*time.Hour)
}

func newTestAppleVerifier() *oauth.AppleVerifier {
	return oauth.NewAppleVerifier("test-client-id")
}

func ptrString(s string) *string { return &s }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRefreshToken_Success(t *testing.T) {
	jwtMgr := newTestJWTManager()
	userID := uuid.Must(uuid.NewV7())

	// Generate a real token pair so that jwtManager.ValidateToken succeeds.
	originalPair, err := jwtMgr.GenerateTokenPair(userID)
	if err != nil {
		t.Fatalf("failed to generate initial token pair: %v", err)
	}

	storedRT := &model.RefreshToken{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    userID,
		Token:     originalPair.RefreshToken,
		ExpiresAt: time.Now().Add(720 * time.Hour),
	}

	tokenRepo := &mockTokenRepository{
		findByTokenFn: func(_ context.Context, token string) (*model.RefreshToken, error) {
			if token == originalPair.RefreshToken {
				return storedRT, nil
			}
			return nil, model.ErrInvalidRefreshToken
		},
		deleteByTokenFn: func(_ context.Context, token string) error {
			return nil
		},
		createFn: func(_ context.Context, rt *model.RefreshToken) error {
			return nil
		},
	}

	svc := NewAuthService(&mockUserRepository{}, tokenRepo, jwtMgr, newTestAppleVerifier())

	newPair, err := svc.RefreshToken(context.Background(), originalPair.RefreshToken)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newPair == nil {
		t.Fatal("expected non-nil token pair")
	}
	if newPair.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if newPair.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	// The new refresh token should differ from the old one (rotation).
	if newPair.RefreshToken == originalPair.RefreshToken {
		t.Error("expected rotated refresh token to differ from original")
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	tokenRepo := &mockTokenRepository{
		findByTokenFn: func(_ context.Context, _ string) (*model.RefreshToken, error) {
			return nil, model.ErrInvalidRefreshToken
		},
	}

	svc := NewAuthService(&mockUserRepository{}, tokenRepo, newTestJWTManager(), newTestAppleVerifier())

	_, err := svc.RefreshToken(context.Background(), "totally-invalid-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, model.ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got: %v", err)
	}
}

func TestLogout_Success(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	deleteByUserIDCalled := false

	tokenRepo := &mockTokenRepository{
		deleteByUserIDFn: func(_ context.Context, id uuid.UUID) error {
			deleteByUserIDCalled = true
			if id != userID {
				t.Errorf("expected userID %s, got %s", userID, id)
			}
			return nil
		},
	}

	svc := NewAuthService(&mockUserRepository{}, tokenRepo, newTestJWTManager(), newTestAppleVerifier())

	err := svc.Logout(context.Background(), userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !deleteByUserIDCalled {
		t.Error("expected DeleteByUserID to be called")
	}
}

func TestGetCurrentUser_Success(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	expectedUser := &model.User{
		ID:          userID,
		Provider:    "apple",
		ProviderSub: "sub123",
		Email:       ptrString("test@example.com"),
	}

	userRepo := &mockUserRepository{
		findByIDFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
			if id == userID {
				return expectedUser, nil
			}
			return nil, model.ErrUserNotFound
		},
	}

	svc := NewAuthService(userRepo, &mockTokenRepository{}, newTestJWTManager(), newTestAppleVerifier())

	user, err := svc.GetCurrentUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, user.ID)
	}
	if user.Provider != "apple" {
		t.Errorf("expected provider 'apple', got '%s'", user.Provider)
	}
	if user.Email == nil || *user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %v", user.Email)
	}
}

func TestGetCurrentUser_NotFound(t *testing.T) {
	userRepo := &mockUserRepository{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (*model.User, error) {
			return nil, model.ErrUserNotFound
		},
	}

	svc := NewAuthService(userRepo, &mockTokenRepository{}, newTestJWTManager(), newTestAppleVerifier())

	_, err := svc.GetCurrentUser(context.Background(), uuid.Must(uuid.NewV7()))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestDeleteAccount_Success(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	deleteTokensCalled := false
	deleteUserCalled := false

	tokenRepo := &mockTokenRepository{
		deleteByUserIDFn: func(_ context.Context, id uuid.UUID) error {
			deleteTokensCalled = true
			if id != userID {
				t.Errorf("expected userID %s, got %s", userID, id)
			}
			return nil
		},
	}

	userRepo := &mockUserRepository{
		deleteFn: func(_ context.Context, id uuid.UUID) error {
			deleteUserCalled = true
			if id != userID {
				t.Errorf("expected userID %s, got %s", userID, id)
			}
			return nil
		},
	}

	svc := NewAuthService(userRepo, tokenRepo, newTestJWTManager(), newTestAppleVerifier())

	err := svc.DeleteAccount(context.Background(), userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !deleteTokensCalled {
		t.Error("expected DeleteByUserID on token repo to be called")
	}
	if !deleteUserCalled {
		t.Error("expected Delete on user repo to be called")
	}
}

func TestAppleLogin_InvalidToken(t *testing.T) {
	svc := NewAuthService(
		&mockUserRepository{},
		&mockTokenRepository{},
		newTestJWTManager(),
		newTestAppleVerifier(),
	)

	_, _, err := svc.AppleLogin(context.Background(), "garbage-not-a-jwt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, model.ErrInvalidIDToken) {
		t.Fatalf("expected ErrInvalidIDToken, got: %v", err)
	}
}
