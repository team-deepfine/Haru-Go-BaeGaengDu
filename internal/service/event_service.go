package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/google/uuid"
)

// EventService defines the interface for event business logic.
type EventService interface {
	CreateEvent(ctx context.Context, req CreateEventInput) (*model.Event, error)
	GetEvent(ctx context.Context, id uuid.UUID) (*model.Event, error)
	ListEvents(ctx context.Context, start, end time.Time) ([]model.Event, error)
	UpdateEvent(ctx context.Context, id uuid.UUID, req CreateEventInput) (*model.Event, error)
	DeleteEvent(ctx context.Context, id uuid.UUID) error
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
	repo repository.EventRepository
}

// NewEventService creates a new EventService.
func NewEventService(repo repository.EventRepository) EventService {
	return &eventService{repo: repo}
}

func (s *eventService) CreateEvent(ctx context.Context, req CreateEventInput) (*model.Event, error) {
	if err := s.validate(req); err != nil {
		return nil, err
	}

	event := &model.Event{
		ID:              uuid.New(),
		Title:           req.Title,
		StartAt:         req.StartAt.UTC(),
		EndAt:           req.EndAt.UTC(),
		AllDay:          req.AllDay,
		Timezone:        req.Timezone,
		LocationName:    req.LocationName,
		LocationAddress: req.LocationAddress,
		LocationLat:     req.LocationLat,
		LocationLng:     req.LocationLng,
		ReminderOffsets: model.Int64Array(req.ReminderOffsets),
		Notes:           req.Notes,
	}

	if err := s.repo.Create(ctx, event); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	return event, nil
}

func (s *eventService) GetEvent(ctx context.Context, id uuid.UUID) (*model.Event, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *eventService) ListEvents(ctx context.Context, start, end time.Time) ([]model.Event, error) {
	return s.repo.FindByDateRange(ctx, start.UTC(), end.UTC())
}

func (s *eventService) UpdateEvent(ctx context.Context, id uuid.UUID, req CreateEventInput) (*model.Event, error) {
	if err := s.validate(req); err != nil {
		return nil, err
	}

	event, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	event.Title = req.Title
	event.StartAt = req.StartAt.UTC()
	event.EndAt = req.EndAt.UTC()
	event.AllDay = req.AllDay
	event.Timezone = req.Timezone
	event.LocationName = req.LocationName
	event.LocationAddress = req.LocationAddress
	event.LocationLat = req.LocationLat
	event.LocationLng = req.LocationLng
	event.ReminderOffsets = model.Int64Array(req.ReminderOffsets)
	event.Notes = req.Notes

	if err := s.repo.Update(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *eventService) DeleteEvent(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *eventService) validate(req CreateEventInput) error {
	if strings.TrimSpace(req.Title) == "" {
		return model.ErrTitleRequired
	}
	if req.EndAt.Before(req.StartAt) {
		return model.ErrInvalidTimeRange
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		return model.ErrInvalidTimezone
	}
	return nil
}
