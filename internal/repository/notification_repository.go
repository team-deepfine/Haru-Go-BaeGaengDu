package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationRepository defines the interface for notification data access.
type NotificationRepository interface {
	CreateBatch(ctx context.Context, notifications []*model.Notification) error
	DeleteByEventID(ctx context.Context, eventID uuid.UUID) error
	FindPending(ctx context.Context, before time.Time, limit int) ([]model.Notification, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	IncrementRetries(ctx context.Context, id uuid.UUID) error
	FindByEventID(ctx context.Context, eventID uuid.UUID) ([]model.Notification, error)
}

type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new GORM-backed NotificationRepository.
func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) CreateBatch(ctx context.Context, notifications []*model.Notification) error {
	if len(notifications) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&notifications).Error; err != nil {
		return fmt.Errorf("create notifications: %w", err)
	}
	return nil
}

func (r *notificationRepository) DeleteByEventID(ctx context.Context, eventID uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&model.Notification{}, "event_id = ?", eventID).Error; err != nil {
		return fmt.Errorf("delete notifications by event: %w", err)
	}
	return nil
}

func (r *notificationRepository) FindPending(ctx context.Context, before time.Time, limit int) ([]model.Notification, error) {
	var notifications []model.Notification
	err := r.db.WithContext(ctx).
		Preload("Event").
		Where("sent = ? AND notify_at <= ? AND retries < ?", false, before, 3).
		Order("notify_at ASC").
		Limit(limit).
		Find(&notifications).Error
	if err != nil {
		return nil, fmt.Errorf("find pending notifications: %w", err)
	}
	return notifications, nil
}

func (r *notificationRepository) MarkSent(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Model(&model.Notification{}).Where("id = ?", id).Update("sent", true).Error; err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (r *notificationRepository) IncrementRetries(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Model(&model.Notification{}).Where("id = ?", id).
		UpdateColumn("retries", gorm.Expr("retries + 1")).Error; err != nil {
		return fmt.Errorf("increment notification retries: %w", err)
	}
	return nil
}

func (r *notificationRepository) FindByEventID(ctx context.Context, eventID uuid.UUID) ([]model.Notification, error) {
	var notifications []model.Notification
	err := r.db.WithContext(ctx).Where("event_id = ?", eventID).Order("notify_at ASC").Find(&notifications).Error
	if err != nil {
		return nil, fmt.Errorf("find notifications by event: %w", err)
	}
	return notifications, nil
}
