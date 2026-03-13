package service

import (
	"context"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/google/uuid"
)

// EventService defines the interface for event business logic.
type EventService interface {
	CreateEvent(ctx context.Context, userID uuid.UUID, req CreateEventInput) (*model.Event, error)
	GetEvent(ctx context.Context, userID, id uuid.UUID) (*model.Event, error)
	ListEvents(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]model.Event, error)
	UpdateEvent(ctx context.Context, userID, id uuid.UUID, req CreateEventInput) (*model.Event, error)
	DeleteEvent(ctx context.Context, userID, id uuid.UUID) error
}

// CreateEventInput is the service-layer input for creating/updating an event.
type CreateEventInput struct {
	Title           string
	StartAt         time.Time
	EndAt           time.Time
	AllDay          bool
	Timezone        string
	LocationName    *string
	LocationAddress *string
	LocationLat     *float64
	LocationLng     *float64
	ReminderOffsets []int64
	Notes           *string
}

type eventService struct {
	repo     repository.EventRepository
	notifier NotificationScheduler
}

// EventServiceOption configures optional dependencies on EventService.
type EventServiceOption func(*eventService)

// WithNotificationScheduler sets the notification scheduler for automatic scheduling.
func WithNotificationScheduler(ns NotificationScheduler) EventServiceOption {
	return func(s *eventService) {
		s.notifier = ns
	}
}

// NewEventService creates a new EventService.
func NewEventService(repo repository.EventRepository, opts ...EventServiceOption) EventService {
	svc := &eventService{repo: repo}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}
