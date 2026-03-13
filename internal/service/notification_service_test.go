package service

import (
	"context"
	"testing"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockNotificationRepository struct {
	createBatchFn      func(ctx context.Context, notifications []*model.Notification) error
	deleteByEventIDFn  func(ctx context.Context, eventID uuid.UUID) error
	findPendingFn      func(ctx context.Context, before time.Time, limit int) ([]model.Notification, error)
	markSentFn         func(ctx context.Context, id uuid.UUID) error
	incrementRetriesFn func(ctx context.Context, id uuid.UUID) error
	findByEventIDFn    func(ctx context.Context, eventID uuid.UUID) ([]model.Notification, error)

	createdNotifications []*model.Notification
	deletedEventIDs      []uuid.UUID
}

func (m *mockNotificationRepository) CreateBatch(ctx context.Context, notifications []*model.Notification) error {
	m.createdNotifications = append(m.createdNotifications, notifications...)
	if m.createBatchFn != nil {
		return m.createBatchFn(ctx, notifications)
	}
	return nil
}

func (m *mockNotificationRepository) DeleteByEventID(ctx context.Context, eventID uuid.UUID) error {
	m.deletedEventIDs = append(m.deletedEventIDs, eventID)
	if m.deleteByEventIDFn != nil {
		return m.deleteByEventIDFn(ctx, eventID)
	}
	return nil
}

func (m *mockNotificationRepository) FindPending(ctx context.Context, before time.Time, limit int) ([]model.Notification, error) {
	if m.findPendingFn != nil {
		return m.findPendingFn(ctx, before, limit)
	}
	return nil, nil
}

func (m *mockNotificationRepository) MarkSent(ctx context.Context, id uuid.UUID) error {
	if m.markSentFn != nil {
		return m.markSentFn(ctx, id)
	}
	return nil
}

func (m *mockNotificationRepository) IncrementRetries(ctx context.Context, id uuid.UUID) error {
	if m.incrementRetriesFn != nil {
		return m.incrementRetriesFn(ctx, id)
	}
	return nil
}

func (m *mockNotificationRepository) FindByEventID(ctx context.Context, eventID uuid.UUID) ([]model.Notification, error) {
	if m.findByEventIDFn != nil {
		return m.findByEventIDFn(ctx, eventID)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Tests: NotificationScheduler
// ---------------------------------------------------------------------------

func TestNotificationScheduler_ScheduleForEvent(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	eventID := uuid.Must(uuid.NewV7())

	t.Run("creates notifications for future offsets", func(t *testing.T) {
		repo := &mockNotificationRepository{}
		scheduler := NewNotificationScheduler(repo)

		event := &model.Event{
			ID:              eventID,
			UserID:          userID,
			StartAt:         time.Now().Add(24 * time.Hour).UTC(),
			ReminderOffsets: model.Int64Array{10, 60, 1440},
		}

		err := scheduler.ScheduleForEvent(context.Background(), event)
		require.NoError(t, err)
		assert.Equal(t, 3, len(repo.createdNotifications))

		offsets := map[int]bool{}
		for _, n := range repo.createdNotifications {
			assert.Equal(t, eventID, n.EventID)
			assert.Equal(t, userID, n.UserID)
			assert.False(t, n.Sent)
			assert.True(t, n.NotifyAt.Before(event.StartAt))
			offsets[n.OffsetMin] = true
		}
		assert.True(t, offsets[10])
		assert.True(t, offsets[60])
		assert.True(t, offsets[1440])
	})

	t.Run("skips past notifications", func(t *testing.T) {
		repo := &mockNotificationRepository{}
		scheduler := NewNotificationScheduler(repo)

		// Event starts in 5 minutes
		event := &model.Event{
			ID:              eventID,
			UserID:          userID,
			StartAt:         time.Now().Add(5 * time.Minute).UTC(),
			ReminderOffsets: model.Int64Array{0, 10, 60}, // 10 and 60 would be in the past
		}

		err := scheduler.ScheduleForEvent(context.Background(), event)
		require.NoError(t, err)

		// Only offset=0 should survive (notify at start time, 5min from now)
		for _, n := range repo.createdNotifications {
			assert.Equal(t, 0, n.OffsetMin, "only zero offset should be scheduled")
		}
	})

	t.Run("empty reminder offsets creates no notifications", func(t *testing.T) {
		repo := &mockNotificationRepository{}
		scheduler := NewNotificationScheduler(repo)

		event := &model.Event{
			ID:              eventID,
			UserID:          userID,
			StartAt:         time.Now().Add(24 * time.Hour).UTC(),
			ReminderOffsets: model.Int64Array{},
		}

		err := scheduler.ScheduleForEvent(context.Background(), event)
		require.NoError(t, err)
		assert.Equal(t, 0, len(repo.createdNotifications))
	})
}

func TestNotificationScheduler_CancelForEvent(t *testing.T) {
	repo := &mockNotificationRepository{}
	scheduler := NewNotificationScheduler(repo)
	eventID := uuid.Must(uuid.NewV7())

	err := scheduler.CancelForEvent(context.Background(), eventID)
	require.NoError(t, err)
	require.Equal(t, 1, len(repo.deletedEventIDs))
	assert.Equal(t, eventID, repo.deletedEventIDs[0])
}

func TestNotificationScheduler_RescheduleForEvent(t *testing.T) {
	repo := &mockNotificationRepository{}
	scheduler := NewNotificationScheduler(repo)
	eventID := uuid.Must(uuid.NewV7())

	event := &model.Event{
		ID:              eventID,
		UserID:          uuid.Must(uuid.NewV7()),
		StartAt:         time.Now().Add(24 * time.Hour).UTC(),
		ReminderOffsets: model.Int64Array{30},
	}

	err := scheduler.RescheduleForEvent(context.Background(), event)
	require.NoError(t, err)

	// Should have cancelled first, then scheduled
	require.Equal(t, 1, len(repo.deletedEventIDs))
	assert.Equal(t, eventID, repo.deletedEventIDs[0])
	assert.Equal(t, 1, len(repo.createdNotifications))
}

// ---------------------------------------------------------------------------
// Tests: formatNotificationBody
// ---------------------------------------------------------------------------

func TestFormatNotificationBody(t *testing.T) {
	tests := []struct {
		offsetMin int
		want      string
	}{
		{0, "일정이 지금 시작됩니다"},
		{5, "5분 후 일정이 시작됩니다"},
		{10, "10분 후 일정이 시작됩니다"},
		{30, "30분 후 일정이 시작됩니다"},
		{59, "59분 후 일정이 시작됩니다"},
		{60, "1시간 후 일정이 시작됩니다"},
		{120, "2시간 후 일정이 시작됩니다"},
		{180, "3시간 후 일정이 시작됩니다"},
		{1439, "23시간 후 일정이 시작됩니다"},
		{1440, "1일 후 일정이 시작됩니다"},
		{2880, "2일 후 일정이 시작됩니다"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatNotificationBody(tt.offsetMin)
			assert.Equal(t, tt.want, got)
		})
	}
}
