package service

import (
	"context"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/google/uuid"
)

// NotificationScheduler defines the interface for notification scheduling.
type NotificationScheduler interface {
	ScheduleForEvent(ctx context.Context, event *model.Event) error
	CancelForEvent(ctx context.Context, eventID uuid.UUID) error
	RescheduleForEvent(ctx context.Context, event *model.Event) error
}

type notificationScheduler struct {
	repo repository.NotificationRepository
}

// NewNotificationScheduler creates a new NotificationScheduler.
func NewNotificationScheduler(repo repository.NotificationRepository) NotificationScheduler {
	return &notificationScheduler{repo: repo}
}

// ScheduleForEvent creates notification rows for each reminderOffset of the event.
// Skips offsets that produce a notify_at time in the past.
func (s *notificationScheduler) ScheduleForEvent(ctx context.Context, event *model.Event) error {
	now := time.Now().UTC()
	var notifications []*model.Notification

	for _, offsetMin := range event.ReminderOffsets {
		notifyAt := event.StartAt.Add(-time.Duration(offsetMin) * time.Minute)
		if notifyAt.Before(now) {
			continue
		}
		notifications = append(notifications, &model.Notification{
			ID:        uuid.Must(uuid.NewV7()),
			EventID:   event.ID,
			UserID:    event.UserID,
			NotifyAt:  notifyAt,
			OffsetMin: int(offsetMin),
		})
	}

	return s.repo.CreateBatch(ctx, notifications)
}

// CancelForEvent deletes all notifications for the given event.
func (s *notificationScheduler) CancelForEvent(ctx context.Context, eventID uuid.UUID) error {
	return s.repo.DeleteByEventID(ctx, eventID)
}

// RescheduleForEvent cancels existing notifications then creates new ones.
func (s *notificationScheduler) RescheduleForEvent(ctx context.Context, event *model.Event) error {
	if err := s.CancelForEvent(ctx, event.ID); err != nil {
		return err
	}
	return s.ScheduleForEvent(ctx, event)
}
