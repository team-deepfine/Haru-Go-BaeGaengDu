package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/internal/service"
	"github.com/daewon/haru/pkg/jwt"
	"github.com/daewon/haru/pkg/oauth"
	jwtlib "github.com/golang-jwt/jwt/v5"
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

var _ repository.UserRepository = (*mockUserRepository)(nil)

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

func (m *mockUserRepository) FindByOriginalTransactionID(_ context.Context, _ string) (*model.User, error) {
	return nil, model.ErrUserNotFound
}

func (m *mockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

// mockTokenRepository implements repository.TokenRepository for testing.
type mockTokenRepository struct {
	createFn         func(ctx context.Context, token *model.RefreshToken) error
	findByTokenFn    func(ctx context.Context, token string) (*model.RefreshToken, error)
	deleteByTokenFn  func(ctx context.Context, token string) error
	deleteByUserIDFn func(ctx context.Context, userID uuid.UUID) error
	deleteExpiredFn  func(ctx context.Context) error
}

var _ repository.TokenRepository = (*mockTokenRepository)(nil)

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

// mockDeviceTokenRepository implements repository.DeviceTokenRepository for testing.
type mockDeviceTokenRepository struct {
	upsertFn             func(ctx context.Context, token *model.DeviceToken) error
	deleteByUserAndTokenFn func(ctx context.Context, userID uuid.UUID, token string) error
	deleteByUserIDFn     func(ctx context.Context, userID uuid.UUID) error
	findByUserIDFn       func(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error)
	deleteByTokenFn      func(ctx context.Context, token string) error
}

var _ repository.DeviceTokenRepository = (*mockDeviceTokenRepository)(nil)

func (m *mockDeviceTokenRepository) Upsert(ctx context.Context, token *model.DeviceToken) error {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, token)
	}
	return nil
}
func (m *mockDeviceTokenRepository) DeleteByUserAndToken(ctx context.Context, userID uuid.UUID, token string) error {
	if m.deleteByUserAndTokenFn != nil {
		return m.deleteByUserAndTokenFn(ctx, userID, token)
	}
	return nil
}
func (m *mockDeviceTokenRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if m.deleteByUserIDFn != nil {
		return m.deleteByUserIDFn(ctx, userID)
	}
	return nil
}
func (m *mockDeviceTokenRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}
func (m *mockDeviceTokenRepository) FindByUserIDs(_ context.Context, _ []uuid.UUID) ([]model.DeviceToken, error) {
	return nil, nil
}
func (m *mockDeviceTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	if m.deleteByTokenFn != nil {
		return m.deleteByTokenFn(ctx, token)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestJWTManager() *jwt.Manager {
	return jwt.NewManager("test-secret", time.Hour, 720*time.Hour)
}

// testApplePrivateKeyPEM is a test-only ES256 private key (not used in production).
const testApplePrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQggNPpkAwDVkvZw6H1
sOZ0HU1bCMeoQ1b1+OnuG8AxbU6hRANCAAQ85HzhTQNWkkyZXtdZMV2B96l+paFE
XM+YSpZkqDqkJoOYd5TYGa7iDC5iDFxWYeyIpt80lnMX/0KVH0ipb2Sf
-----END PRIVATE KEY-----`

func newTestAppleClient() *oauth.AppleClient {
	client, err := oauth.NewAppleClient("test-client-id", "test-team-id", "test-key-id", testApplePrivateKeyPEM, "")
	if err != nil {
		panic("failed to create test apple client: " + err.Error())
	}
	client.SetSkipJWKS(true)
	return client
}

func newTestKakaoClient() *oauth.KakaoClient {
	return oauth.NewKakaoClient()
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

	svc := service.NewAuthService(&mockUserRepository{}, tokenRepo, &mockDeviceTokenRepository{}, jwtMgr, newTestAppleClient(), newTestKakaoClient())

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

	svc := service.NewAuthService(&mockUserRepository{}, tokenRepo, &mockDeviceTokenRepository{}, newTestJWTManager(), newTestAppleClient(), newTestKakaoClient())

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

	svc := service.NewAuthService(&mockUserRepository{}, tokenRepo, &mockDeviceTokenRepository{}, newTestJWTManager(), newTestAppleClient(), newTestKakaoClient())

	err := svc.Logout(context.Background(), userID, "")
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

	svc := service.NewAuthService(userRepo, &mockTokenRepository{}, &mockDeviceTokenRepository{}, newTestJWTManager(), newTestAppleClient(), newTestKakaoClient())

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

	svc := service.NewAuthService(userRepo, &mockTokenRepository{}, &mockDeviceTokenRepository{}, newTestJWTManager(), newTestAppleClient(), newTestKakaoClient())

	_, err := svc.GetCurrentUser(context.Background(), uuid.Must(uuid.NewV7()))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestDeleteAccount_NonAppleUser(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	deleteTokensCalled := false
	deleteUserCalled := false

	userRepo := &mockUserRepository{
		findByIDFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
			return &model.User{ID: userID, Provider: "kakao", ProviderSub: "kakao-123"}, nil
		},
		deleteFn: func(_ context.Context, id uuid.UUID) error {
			deleteUserCalled = true
			if id != userID {
				t.Errorf("expected userID %s, got %s", userID, id)
			}
			return nil
		},
	}

	tokenRepo := &mockTokenRepository{
		deleteByUserIDFn: func(_ context.Context, id uuid.UUID) error {
			deleteTokensCalled = true
			if id != userID {
				t.Errorf("expected userID %s, got %s", userID, id)
			}
			return nil
		},
	}

	svc := service.NewAuthService(userRepo, tokenRepo, &mockDeviceTokenRepository{}, newTestJWTManager(), newTestAppleClient(), newTestKakaoClient())

	err := svc.DeleteAccount(context.Background(), userID, "")
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

func TestDeleteAccount_AppleUser_MissingCode(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())

	userRepo := &mockUserRepository{
		findByIDFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
			return &model.User{ID: userID, Provider: "apple", ProviderSub: "apple-123"}, nil
		},
	}

	svc := service.NewAuthService(userRepo, &mockTokenRepository{}, &mockDeviceTokenRepository{}, newTestJWTManager(), newTestAppleClient(), newTestKakaoClient())

	err := svc.DeleteAccount(context.Background(), userID, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, model.ErrInvalidAuthCode) {
		t.Fatalf("expected ErrInvalidAuthCode, got: %v", err)
	}
}

func TestDeleteAccount_AppleUser_RevokeSuccess(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	idToken := createTestIDToken(t, "apple-user-123", "test@apple.com")

	// Mock Apple token endpoint (exchange) + revoke endpoint
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		grantType := r.FormValue("grant_type")

		if grantType == "authorization_code" {
			// Token exchange response
			json.NewEncoder(w).Encode(map[string]string{
				"access_token":  "mock-access-token",
				"refresh_token": "mock-refresh-token",
				"id_token":      idToken,
				"token_type":    "Bearer",
			})
			return
		}

		// Revoke endpoint - check token was passed
		token := r.FormValue("token")
		if token != "mock-refresh-token" {
			t.Errorf("expected revoke token 'mock-refresh-token', got '%s'", token)
		}
		tokenTypeHint := r.FormValue("token_type_hint")
		if tokenTypeHint != "refresh_token" {
			t.Errorf("expected token_type_hint 'refresh_token', got '%s'", tokenTypeHint)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	appleClient := newTestAppleClient()
	appleClient.SetTokenURL(mockServer.URL)
	appleClient.SetRevokeURL(mockServer.URL)

	deleteTokensCalled := false
	deleteUserCalled := false

	userRepo := &mockUserRepository{
		findByIDFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
			return &model.User{ID: userID, Provider: "apple", ProviderSub: "apple-123"}, nil
		},
		deleteFn: func(_ context.Context, id uuid.UUID) error {
			deleteUserCalled = true
			return nil
		},
	}

	tokenRepo := &mockTokenRepository{
		deleteByUserIDFn: func(_ context.Context, id uuid.UUID) error {
			deleteTokensCalled = true
			return nil
		},
	}

	svc := service.NewAuthService(userRepo, tokenRepo, &mockDeviceTokenRepository{}, newTestJWTManager(), appleClient, newTestKakaoClient())

	err := svc.DeleteAccount(context.Background(), userID, "valid-auth-code")
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

func TestAppleLogin_InvalidCode(t *testing.T) {
	// Mock Apple token endpoint that returns 400 (invalid grant)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
	}))
	defer mockServer.Close()

	appleClient := newTestAppleClient()
	appleClient.SetTokenURL(mockServer.URL)

	svc := service.NewAuthService(
		&mockUserRepository{},
		&mockTokenRepository{},
		&mockDeviceTokenRepository{},
		newTestJWTManager(),
		appleClient,
		newTestKakaoClient(),
	)

	_, _, err := svc.AppleLogin(context.Background(), "invalid-auth-code")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, model.ErrInvalidAuthCode) {
		t.Fatalf("expected ErrInvalidAuthCode, got: %v", err)
	}
}

func TestAppleLogin_Success(t *testing.T) {
	// Create a mock id_token JWT with sub and email claims
	idToken := createTestIDToken(t, "apple-user-123", "test@apple.com")

	// Mock Apple token endpoint that returns a valid response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "mock-access-token",
			"id_token":     idToken,
			"token_type":   "Bearer",
		})
	}))
	defer mockServer.Close()

	appleClient := newTestAppleClient()
	appleClient.SetTokenURL(mockServer.URL)

	createCalled := false
	userRepo := &mockUserRepository{
		findByProviderSub: func(_ context.Context, provider, providerSub string) (*model.User, error) {
			if provider == "apple" && providerSub == "apple-user-123" {
				return nil, nil // new user
			}
			return nil, model.ErrUserNotFound
		},
		createFn: func(_ context.Context, user *model.User) error {
			createCalled = true
			if user.Provider != "apple" {
				t.Errorf("expected provider 'apple', got '%s'", user.Provider)
			}
			if user.ProviderSub != "apple-user-123" {
				t.Errorf("expected providerSub 'apple-user-123', got '%s'", user.ProviderSub)
			}
			if user.Email == nil || *user.Email != "test@apple.com" {
				t.Errorf("expected email 'test@apple.com', got %v", user.Email)
			}
			return nil
		},
	}

	tokenRepo := &mockTokenRepository{
		createFn: func(_ context.Context, rt *model.RefreshToken) error {
			return nil
		},
	}

	svc := service.NewAuthService(userRepo, tokenRepo, &mockDeviceTokenRepository{}, newTestJWTManager(), appleClient, newTestKakaoClient())

	user, tokenPair, err := svc.AppleLogin(context.Background(), "valid-auth-code")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if tokenPair == nil {
		t.Fatal("expected non-nil token pair")
	}
	if tokenPair.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if !createCalled {
		t.Error("expected Create to be called for new user")
	}
}

// createTestIDToken creates a minimal unsigned JWT with sub and email claims for testing.
func createTestIDToken(t *testing.T, sub, email string) string {
	t.Helper()
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.MapClaims{
		"sub":   sub,
		"email": email,
		"iss":   "https://appleid.apple.com",
		"aud":   "test-client-id",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	signed, err := token.SignedString([]byte("test-secret-for-id-token"))
	if err != nil {
		t.Fatalf("failed to create test id_token: %v", err)
	}
	return signed
}
