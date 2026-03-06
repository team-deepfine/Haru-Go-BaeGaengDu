package service

import (
	"context"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
)

func (s *eventService) GetEvent(ctx context.Context, id uuid.UUID) (*model.Event, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *eventService) ListEvents(ctx context.Context, start, end time.Time) ([]model.Event, error) {
	return s.repo.FindByDateRange(ctx, start.UTC(), end.UTC())
}
