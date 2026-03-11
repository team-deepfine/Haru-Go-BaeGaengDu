package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TokenRepository defines the interface for refresh token data access.
type TokenRepository interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	FindByToken(ctx context.Context, token string) (*model.RefreshToken, error)
	DeleteByToken(ctx context.Context, token string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type tokenRepository struct {
	db *gorm.DB
}

// NewTokenRepository creates a new GORM-backed TokenRepository.
func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *tokenRepository) FindByToken(ctx context.Context, token string) (*model.RefreshToken, error) {
	var rt model.RefreshToken
	err := r.db.WithContext(ctx).First(&rt, "token = ?", token).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, model.ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	if rt.ExpiresAt.Before(time.Now()) {
		return nil, model.ErrInvalidRefreshToken
	}
	return &rt, nil
}

func (r *tokenRepository) DeleteByToken(ctx context.Context, token string) error {
	if err := r.db.WithContext(ctx).Delete(&model.RefreshToken{}, "token = ?", token).Error; err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

func (r *tokenRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&model.RefreshToken{}, "user_id = ?", userID).Error; err != nil {
		return fmt.Errorf("delete refresh tokens by user: %w", err)
	}
	return nil
}

func (r *tokenRepository) DeleteExpired(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Delete(&model.RefreshToken{}, "expires_at < ?", time.Now()).Error; err != nil {
		return fmt.Errorf("delete expired tokens: %w", err)
	}
	return nil
}
