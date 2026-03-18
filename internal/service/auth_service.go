package service

import (
	"context"
	"fmt"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/pkg/jwt"
	"github.com/daewon/haru/pkg/oauth"
	"github.com/google/uuid"
)

// AuthService defines the interface for authentication business logic.
type AuthService interface {
	AppleLogin(ctx context.Context, code string) (*model.User, *jwt.TokenPair, error)
	KakaoLogin(ctx context.Context, code string) (*model.User, *jwt.TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*jwt.TokenPair, error)
	Logout(ctx context.Context, userID uuid.UUID) error
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.User, error)
	DeleteAccount(ctx context.Context, userID uuid.UUID, authCode string) error
}

type authService struct {
	userRepo       repository.UserRepository
	tokenRepo      repository.TokenRepository
	deviceTokenRepo repository.DeviceTokenRepository
	jwtManager     *jwt.Manager
	appleClient    *oauth.AppleClient
	kakaoClient    *oauth.KakaoClient
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	deviceTokenRepo repository.DeviceTokenRepository,
	jwtManager *jwt.Manager,
	appleClient *oauth.AppleClient,
	kakaoClient *oauth.KakaoClient,
) AuthService {
	return &authService{
		userRepo:       userRepo,
		tokenRepo:      tokenRepo,
		deviceTokenRepo: deviceTokenRepo,
		jwtManager:     jwtManager,
		appleClient:    appleClient,
		kakaoClient:    kakaoClient,
	}
}

func (s *authService) AppleLogin(ctx context.Context, code string) (*model.User, *jwt.TokenPair, error) {
	info, err := s.appleClient.ExchangeAndGetUser(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", model.ErrInvalidAuthCode, err)
	}

	user, err := s.userRepo.FindByProviderSub(ctx, "apple", info.Sub)
	if err != nil {
		return nil, nil, fmt.Errorf("find user: %w", err)
	}

	now := time.Now()
	if user == nil {
		// New user - auto register
		user = &model.User{
			ID:          uuid.Must(uuid.NewV7()),
			Provider:    "apple",
			ProviderSub: info.Sub,
			Email:       info.Email,
			LastLoginAt: &now,
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("create user: %w", err)
		}
	} else {
		// Existing user - update login time and email if provided
		user.LastLoginAt = &now
		if info.Email != nil {
			user.Email = info.Email
		}
		if err := s.userRepo.Update(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("update user: %w", err)
		}
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	// Store refresh token in DB
	rt := &model.RefreshToken{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user.ID,
		Token:     tokenPair.RefreshToken,
		ExpiresAt: now.Add(720 * time.Hour),
	}
	if err := s.tokenRepo.Create(ctx, rt); err != nil {
		return nil, nil, fmt.Errorf("store refresh token: %w", err)
	}

	return user, tokenPair, nil
}

func (s *authService) KakaoLogin(ctx context.Context, code string) (*model.User, *jwt.TokenPair, error) {
	info, err := s.kakaoClient.ExchangeAndGetUser(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", model.ErrInvalidAuthCode, err)
	}

	user, err := s.userRepo.FindByProviderSub(ctx, "kakao", info.Sub)
	if err != nil {
		return nil, nil, fmt.Errorf("find user: %w", err)
	}

	now := time.Now()
	if user == nil {
		user = &model.User{
			ID:           uuid.Must(uuid.NewV7()),
			Provider:     "kakao",
			ProviderSub:  info.Sub,
			Email:        info.Email,
			Nickname:     info.Nickname,
			ProfileImage: info.ProfileImage,
			LastLoginAt:  &now,
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("create user: %w", err)
		}
	} else {
		user.LastLoginAt = &now
		if info.Email != nil {
			user.Email = info.Email
		}
		if info.Nickname != nil {
			user.Nickname = info.Nickname
		}
		if info.ProfileImage != nil {
			user.ProfileImage = info.ProfileImage
		}
		if err := s.userRepo.Update(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("update user: %w", err)
		}
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	rt := &model.RefreshToken{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user.ID,
		Token:     tokenPair.RefreshToken,
		ExpiresAt: now.Add(720 * time.Hour),
	}
	if err := s.tokenRepo.Create(ctx, rt); err != nil {
		return nil, nil, fmt.Errorf("store refresh token: %w", err)
	}

	return user, tokenPair, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	// Validate the refresh token exists and is not expired
	rt, err := s.tokenRepo.FindByToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	// Validate the JWT signature
	userID, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		return nil, model.ErrInvalidRefreshToken
	}

	// Delete old refresh token (Rotation)
	if err := s.tokenRepo.DeleteByToken(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("delete old refresh token: %w", err)
	}

	// Generate new token pair
	tokenPair, err := s.jwtManager.GenerateTokenPair(userID)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	// Store new refresh token
	newRT := &model.RefreshToken{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    rt.UserID,
		Token:     tokenPair.RefreshToken,
		ExpiresAt: time.Now().Add(720 * time.Hour),
	}
	if err := s.tokenRepo.Create(ctx, newRT); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return tokenPair, nil
}

func (s *authService) Logout(ctx context.Context, userID uuid.UUID) error {
	if err := s.deviceTokenRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete device tokens: %w", err)
	}
	return s.tokenRepo.DeleteByUserID(ctx, userID)
}

func (s *authService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

func (s *authService) DeleteAccount(ctx context.Context, userID uuid.UUID, authCode string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		return model.ErrUserNotFound
	}

	// Apple 사용자: authorization code로 token revoke 호출
	if user.Provider == "apple" {
		if authCode == "" {
			return fmt.Errorf("%w: apple users must provide authorization code for account deletion", model.ErrInvalidAuthCode)
		}
		if err := s.appleClient.RevokeByAuthCode(ctx, authCode); err != nil {
			return fmt.Errorf("apple token revoke: %w", err)
		}
	}

	if err := s.deviceTokenRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete device tokens: %w", err)
	}
	if err := s.tokenRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete tokens: %w", err)
	}
	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
