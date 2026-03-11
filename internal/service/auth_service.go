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
	AppleLogin(ctx context.Context, idToken string) (*model.User, *jwt.TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*jwt.TokenPair, error)
	Logout(ctx context.Context, userID uuid.UUID) error
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.User, error)
	DeleteAccount(ctx context.Context, userID uuid.UUID) error
}

type authService struct {
	userRepo      repository.UserRepository
	tokenRepo     repository.TokenRepository
	jwtManager    *jwt.Manager
	appleVerifier *oauth.AppleVerifier
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	jwtManager *jwt.Manager,
	appleVerifier *oauth.AppleVerifier,
) AuthService {
	return &authService{
		userRepo:      userRepo,
		tokenRepo:     tokenRepo,
		jwtManager:    jwtManager,
		appleVerifier: appleVerifier,
	}
}

func (s *authService) AppleLogin(ctx context.Context, idToken string) (*model.User, *jwt.TokenPair, error) {
	claims, err := s.appleVerifier.Verify(ctx, idToken)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", model.ErrInvalidIDToken, err)
	}

	user, err := s.userRepo.FindByProviderSub(ctx, "apple", claims.Sub)
	if err != nil {
		return nil, nil, fmt.Errorf("find user: %w", err)
	}

	now := time.Now()
	if user == nil {
		// New user - auto register
		user = &model.User{
			ID:          uuid.Must(uuid.NewV7()),
			Provider:    "apple",
			ProviderSub: claims.Sub,
			Email:       claims.Email,
			LastLoginAt: &now,
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("create user: %w", err)
		}
	} else {
		// Existing user - update login time and email if provided
		user.LastLoginAt = &now
		if claims.Email != nil {
			user.Email = claims.Email
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
	return s.tokenRepo.DeleteByUserID(ctx, userID)
}

func (s *authService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

func (s *authService) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	if err := s.tokenRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete tokens: %w", err)
	}
	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
