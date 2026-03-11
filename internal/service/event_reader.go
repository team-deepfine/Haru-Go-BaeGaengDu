package service

import (
	"context"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
)

func (s *eventService) GetEvent(ctx context.Context, userID, id uuid.UUID) (*model.Event, error) {
	return s.repo.FindByID(ctx, userID, id)
}

func (s *eventService) ListEvents(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]model.Event, error) {
	return s.repo.FindByDateRange(ctx, userID, start.UTC(), end.UTC())
}
