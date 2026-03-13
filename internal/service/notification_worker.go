package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/daewon/haru/internal/model"
	"github.com/daewon/haru/internal/repository"
	"github.com/daewon/haru/pkg/fcm"
	"github.com/google/uuid"
)

const (
	workerPollInterval = 1 * time.Minute
	workerBatchSize    = 100
)

// NotificationWorker polls for pending notifications and sends them via FCM.
type NotificationWorker struct {
	notifRepo  repository.NotificationRepository
	deviceRepo repository.DeviceTokenRepository
	fcmClient  *fcm.Client
}

// NewNotificationWorker creates a new background notification worker.
func NewNotificationWorker(
	notifRepo repository.NotificationRepository,
	deviceRepo repository.DeviceTokenRepository,
	fcmClient *fcm.Client,
) *NotificationWorker {
	return &NotificationWorker{
		notifRepo:  notifRepo,
		deviceRepo: deviceRepo,
		fcmClient:  fcmClient,
	}
}

// Start begins the background polling loop. It blocks until the context is cancelled.
func (w *NotificationWorker) Start(ctx context.Context) {
	slog.Info("notification worker started", "interval", workerPollInterval)
	ticker := time.NewTicker(workerPollInterval)
	defer ticker.Stop()

	// Run once immediately on startup to catch missed notifications from downtime.
	w.processPending(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("notification worker stopped")
			return
		case <-ticker.C:
			w.processPending(ctx)
		}
	}
}

func (w *NotificationWorker) processPending(ctx context.Context) {
	now := time.Now().UTC()
	notifications, err := w.notifRepo.FindPending(ctx, now, workerBatchSize)
	if err != nil {
		slog.Error("failed to fetch pending notifications", "error", err)
		return
	}

	if len(notifications) == 0 {
		return
	}

	slog.Info("processing pending notifications", "count", len(notifications))

	// Group notifications by user to batch device token lookups.
	byUser := make(map[uuid.UUID][]model.Notification)
	for _, n := range notifications {
		byUser[n.UserID] = append(byUser[n.UserID], n)
	}

	for userID, userNotifs := range byUser {
		tokens, err := w.deviceRepo.FindByUserID(ctx, userID)
		if err != nil {
			slog.Error("failed to fetch device tokens", "userID", userID, "error", err)
			continue
		}

		if len(tokens) == 0 {
			// No registered devices — mark as sent to avoid re-processing.
			for _, n := range userNotifs {
				if err := w.notifRepo.MarkSent(ctx, n.ID); err != nil {
					slog.Error("failed to mark notification sent (no devices)", "id", n.ID, "error", err)
				}
			}
			continue
		}

		tokenStrings := make([]string, len(tokens))
		for i, t := range tokens {
			tokenStrings[i] = t.Token
		}

		for _, n := range userNotifs {
			w.sendNotification(ctx, n, tokenStrings)
		}
	}
}

func (w *NotificationWorker) sendNotification(ctx context.Context, notif model.Notification, tokens []string) {
	title := "Haru"
	body := "일정이 곧 시작됩니다"
	if notif.Event.ID != uuid.Nil && notif.Event.Title != "" {
		title = notif.Event.Title
		body = formatNotificationBody(notif.OffsetMin)
	}

	data := map[string]string{
		"eventId": notif.EventID.String(),
		"type":    "event_reminder",
	}

	invalidTokens := w.fcmClient.SendMulticast(ctx, tokens, title, body, data)

	// Remove invalid tokens.
	for _, token := range invalidTokens {
		slog.Info("removing invalid FCM token", "token", token[:min(10, len(token))]+"...")
		if err := w.deviceRepo.DeleteByToken(ctx, token); err != nil {
			slog.Error("failed to delete stale token", "error", err)
		}
	}

	// If there were no invalid tokens and no errors, mark as sent.
	// If all tokens are invalid, also mark as sent (no valid devices left).
	if len(invalidTokens) == len(tokens) || len(invalidTokens) == 0 {
		if err := w.notifRepo.MarkSent(ctx, notif.ID); err != nil {
			slog.Error("failed to mark notification sent", "id", notif.ID, "error", err)
		}
	} else {
		// Partial failure — increment retries for next poll.
		if err := w.notifRepo.IncrementRetries(ctx, notif.ID); err != nil {
			slog.Error("failed to increment retries", "id", notif.ID, "error", err)
		}
	}
}

func formatNotificationBody(offsetMin int) string {
	switch {
	case offsetMin == 0:
		return "일정이 지금 시작됩니다"
	case offsetMin < 60:
		return fmt.Sprintf("%d분 후 일정이 시작됩니다", offsetMin)
	case offsetMin == 60:
		return "1시간 후 일정이 시작됩니다"
	case offsetMin < 1440:
		return fmt.Sprintf("%d시간 후 일정이 시작됩니다", offsetMin/60)
	default:
		return fmt.Sprintf("%d일 후 일정이 시작됩니다", offsetMin/1440)
	}
}
