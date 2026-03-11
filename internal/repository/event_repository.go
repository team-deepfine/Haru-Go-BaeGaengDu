package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EventRepository defines the interface for event data access.
type EventRepository interface {
	Create(ctx context.Context, event *model.Event) error
	FindByID(ctx context.Context, userID, id uuid.UUID) (*model.Event, error)
	FindByDateRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]model.Event, error)
	Update(ctx context.Context, event *model.Event) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type eventRepository struct {
	db *gorm.DB
}

// NewEventRepository creates a new GORM-backed EventRepository.
func NewEventRepository(db *gorm.DB) EventRepository {
	return &eventRepository{db: db}
}

func (r *eventRepository) Create(ctx context.Context, event *model.Event) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	return nil
}

func (r *eventRepository) FindByID(ctx context.Context, userID, id uuid.UUID) (*model.Event, error) {
	var event model.Event
	err := r.db.WithContext(ctx).First(&event, "id = ? AND user_id = ?", id, userID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, model.ErrEventNotFound
		}
		return nil, fmt.Errorf("find event by id: %w", err)
	}
	return &event, nil
}

func (r *eventRepository) FindByDateRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]model.Event, error) {
	var events []model.Event
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND start_at < ? AND end_at > ?", userID, end, start).
		Order("all_day DESC, start_at ASC").
		Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("find events by date range: %w", err)
	}
	return events, nil
}

func (r *eventRepository) Update(ctx context.Context, event *model.Event) error {
	if err := r.db.WithContext(ctx).Save(event).Error; err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

func (r *eventRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&model.Event{}, "id = ? AND user_id = ?", id, userID)
	if result.Error != nil {
		return fmt.Errorf("delete event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrEventNotFound
	}
	return nil
}
