package fcm

import (
	"context"
	"fmt"
	"log/slog"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Client wraps Firebase Cloud Messaging for push notification delivery.
type Client struct {
	messaging *messaging.Client
}

// NewClient creates an FCM client authenticated with a service account JSON file.
func NewClient(ctx context.Context, credFile string) (*Client, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credFile))
	if err != nil {
		return nil, fmt.Errorf("create firebase app: %w", err)
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("create messaging client: %w", err)
	}

	return &Client{messaging: msgClient}, nil
}

// SendMulticast sends a push notification to multiple device tokens.
// Returns the list of invalid tokens and an error if the entire call failed.
func (c *Client) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{Sound: "default"},
			},
		},
	}

	resp, err := c.messaging.SendEachForMulticast(ctx, msg)
	if err != nil {
		slog.Error("FCM SendEachForMulticast failed", "error", err)
		return nil, fmt.Errorf("fcm multicast: %w", err)
	}

	var invalidTokens []string
	for i, r := range resp.Responses {
		if r.Success {
			continue
		}
		if messaging.IsUnregistered(r.Error) {
			invalidTokens = append(invalidTokens, tokens[i])
		} else {
			slog.Error("FCM send failed for token",
				"token", tokens[i][:min(10, len(tokens[i]))]+"...",
				"error", r.Error,
			)
		}
	}

	slog.Info("FCM multicast result",
		"total", len(tokens),
		"success", resp.SuccessCount,
		"failure", resp.FailureCount,
	)

	return invalidTokens, nil
}
