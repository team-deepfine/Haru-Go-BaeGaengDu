package repository

import (
	"context"
	"fmt"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeviceTokenRepository defines the interface for device token data access.
type DeviceTokenRepository interface {
	Upsert(ctx context.Context, token *model.DeviceToken) error
	DeleteByUserAndToken(ctx context.Context, userID uuid.UUID, token string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error)
	DeleteByToken(ctx context.Context, token string) error
}

type deviceTokenRepository struct {
	db *gorm.DB
}

// NewDeviceTokenRepository creates a new GORM-backed DeviceTokenRepository.
func NewDeviceTokenRepository(db *gorm.DB) DeviceTokenRepository {
	return &deviceTokenRepository{db: db}
}

func (r *deviceTokenRepository) Upsert(ctx context.Context, token *model.DeviceToken) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "updated_at"}),
	}).Create(token).Error; err != nil {
		return fmt.Errorf("upsert device token: %w", err)
	}
	return nil
}

func (r *deviceTokenRepository) DeleteByUserAndToken(ctx context.Context, userID uuid.UUID, token string) error {
	result := r.db.WithContext(ctx).Delete(&model.DeviceToken{}, "user_id = ? AND token = ?", userID, token)
	if result.Error != nil {
		return fmt.Errorf("delete device token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrDeviceTokenNotFound
	}
	return nil
}

func (r *deviceTokenRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&model.DeviceToken{}, "user_id = ?", userID).Error; err != nil {
		return fmt.Errorf("delete device tokens by user: %w", err)
	}
	return nil
}

func (r *deviceTokenRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error) {
	var tokens []model.DeviceToken
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("find device tokens: %w", err)
	}
	return tokens, nil
}

func (r *deviceTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	if err := r.db.WithContext(ctx).Delete(&model.DeviceToken{}, "token = ?", token).Error; err != nil {
		return fmt.Errorf("delete device token by value: %w", err)
	}
	return nil
}
