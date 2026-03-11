package service

import (
	"context"
	"fmt"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
)

func applyDefaults(req *CreateEventInput) {
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}
	if req.ReminderOffsets == nil {
		req.ReminderOffsets = []int64{180}
	}
}

func (s *eventService) CreateEvent(ctx context.Context, userID uuid.UUID, req CreateEventInput) (*model.Event, error) {
	applyDefaults(&req)

	event := &model.Event{
		ID:              uuid.Must(uuid.NewV7()),
		UserID:          userID,
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

	if err := event.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, event); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	return event, nil
}

func (s *eventService) UpdateEvent(ctx context.Context, userID, id uuid.UUID, req CreateEventInput) (*model.Event, error) {
	applyDefaults(&req)

	event, err := s.repo.FindByID(ctx, userID, id)
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

	if err := event.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *eventService) DeleteEvent(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}
