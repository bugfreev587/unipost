package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

// WebhookDeliveryWorker delivers webhook events in the background.
type WebhookDeliveryWorker struct {
	queries *db.Queries
	client  *http.Client
	ilog    *integrationlogs.Logger
}

func NewWebhookDeliveryWorker(queries *db.Queries, ilog *integrationlogs.Logger) *WebhookDeliveryWorker {
	return &WebhookDeliveryWorker{
		queries: queries,
		client:  &http.Client{Timeout: 5 * time.Second},
		ilog:    ilog,
	}
}

func (w *WebhookDeliveryWorker) logDelivery(ctx context.Context, webhook db.Webhook, delivery db.WebhookDelivery, event integrationlogs.Event) {
	if w == nil || w.ilog == nil || webhook.WorkspaceID == "" {
		return
	}
	event.WorkspaceID = webhook.WorkspaceID
	if event.Category == "" {
		event.Category = integrationlogs.CategoryWebhook
	}
	if event.Source == "" {
		event.Source = integrationlogs.SourceWebhook
	}
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}
	meta, ok := event.Metadata.(map[string]any)
	if !ok {
		meta = map[string]any{
			"details": event.Metadata,
		}
	}
	meta["webhook_id"] = webhook.ID
	meta["delivery_id"] = delivery.ID
	meta["delivery_event"] = delivery.Event
	meta["attempt"] = delivery.Attempts + 1
	event.Metadata = meta
	w.ilog.Write(ctx, event)
}

// Start runs the delivery loop every 10 seconds until ctx is cancelled.
func (w *WebhookDeliveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	slog.Info("webhook delivery worker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("webhook delivery worker stopped")
			return
		case <-ticker.C:
			w.deliverPending(ctx)
		}
	}
}

// Publish satisfies the events.EventBus interface so handler /
// scheduler can fire events without importing this package directly.
// Best-effort: panics are recovered, errors are logged, no return.
func (w *WebhookDeliveryWorker) Publish(ctx context.Context, workspaceID, event string, data any) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("webhook publish: panic recovered", "event", event, "workspace_id", workspaceID, "panic", r)
		}
	}()
	if err := w.Enqueue(ctx, workspaceID, event, data); err != nil {
		slog.Warn("webhook publish: enqueue failed", "event", event, "workspace_id", workspaceID, "error", err)
	}
}

// Enqueue creates webhook delivery records for all matching webhooks.
func (w *WebhookDeliveryWorker) Enqueue(ctx context.Context, workspaceID string, event string, data any) error {
	webhooks, err := w.queries.ListWebhooksByWorkspaceAndEvent(ctx, db.ListWebhooksByWorkspaceAndEventParams{
		WorkspaceID: workspaceID,
		Event:       event,
	})
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"event":     event,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      data,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	for _, wh := range webhooks {
		_, err := w.queries.CreateWebhookDelivery(ctx, db.CreateWebhookDeliveryParams{
			WebhookID: wh.ID,
			Event:     event,
			Payload:   payload,
		})
		if err != nil {
			slog.Error("webhook enqueue: failed to create delivery", "webhook_id", wh.ID, "error", err)
		}
	}

	return nil
}

func (w *WebhookDeliveryWorker) deliverPending(ctx context.Context) {
	deliveries, err := w.queries.GetPendingWebhookDeliveries(ctx)
	if err != nil {
		slog.Error("webhook delivery: failed to get pending", "error", err)
		return
	}

	for _, d := range deliveries {
		webhook, err := w.queries.GetWebhook(ctx, d.WebhookID)
		if err != nil {
			slog.Error("webhook delivery: failed to get webhook", "webhook_id", d.WebhookID, "error", err)
			continue
		}

		w.logDelivery(ctx, webhook, d, integrationlogs.Event{
			Level:   integrationlogs.LevelInfo,
			Status:  integrationlogs.StatusSuccess,
			Action:  integrationlogs.ActionWebhookDeliveryStarted,
			Message: "Started webhook delivery attempt.",
			Metadata: map[string]any{
				"target_url": webhook.Url,
			},
		})

		statusCode, err := w.deliver(ctx, webhook, d)
		attempts := d.Attempts + 1

		if err != nil || statusCode < 200 || statusCode >= 300 {
			// Schedule retry with exponential backoff (max 3 attempts)
			if attempts >= 3 {
				slog.Warn("webhook delivery: giving up", "delivery_id", d.ID, "attempts", attempts)
				w.logDelivery(ctx, webhook, d, integrationlogs.Event{
					Level:            integrationlogs.LevelError,
					Status:           integrationlogs.StatusError,
					Action:           integrationlogs.ActionWebhookDeliveryFailed,
					Message:          "Webhook delivery failed permanently.",
					ErrorCode:        "webhook_delivery_failed",
					RemoteStatusCode: intPtr(statusCode),
					Metadata: map[string]any{
						"target_url":   webhook.Url,
						"terminal":     true,
						"max_attempts": 3,
					},
					ResponsePayload: map[string]any{
						"error": errString(err),
					},
				})
				// Mark delivered_at so the pending query stops picking it up.
				// The delivery is considered terminal even though it failed.
				w.queries.UpdateWebhookDelivery(ctx, db.UpdateWebhookDeliveryParams{
					ID:          d.ID,
					StatusCode:  pgtype.Int4{Int32: int32(statusCode), Valid: statusCode > 0},
					Attempts:    attempts,
					NextRetryAt: pgtype.Timestamptz{},
					DeliveredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
				})
			} else {
				delays := []time.Duration{1 * time.Minute, 5 * time.Minute, 30 * time.Minute}
				nextRetry := time.Now().Add(delays[attempts-1])
				w.logDelivery(ctx, webhook, d, integrationlogs.Event{
					Level:            integrationlogs.LevelWarn,
					Status:           integrationlogs.StatusError,
					Action:           integrationlogs.ActionWebhookRetryScheduled,
					Message:          "Webhook delivery failed and was scheduled for retry.",
					ErrorCode:        "webhook_delivery_retry_scheduled",
					RemoteStatusCode: intPtr(statusCode),
					Metadata: map[string]any{
						"target_url":    webhook.Url,
						"next_retry_at": nextRetry,
						"max_attempts":  3,
					},
					ResponsePayload: map[string]any{
						"error": errString(err),
					},
				})
				w.queries.UpdateWebhookDelivery(ctx, db.UpdateWebhookDeliveryParams{
					ID:          d.ID,
					StatusCode:  pgtype.Int4{Int32: int32(statusCode), Valid: statusCode > 0},
					Attempts:    attempts,
					NextRetryAt: pgtype.Timestamptz{Time: nextRetry, Valid: true},
					DeliveredAt: pgtype.Timestamptz{},
				})
			}
		} else {
			// Success
			w.logDelivery(ctx, webhook, d, integrationlogs.Event{
				Level:            integrationlogs.LevelInfo,
				Status:           integrationlogs.StatusSuccess,
				Action:           integrationlogs.ActionWebhookDeliverySucceeded,
				Message:          "Webhook delivery succeeded.",
				RemoteStatusCode: intPtr(statusCode),
				Metadata: map[string]any{
					"target_url": webhook.Url,
				},
			})
			w.queries.UpdateWebhookDelivery(ctx, db.UpdateWebhookDeliveryParams{
				ID:          d.ID,
				StatusCode:  pgtype.Int4{Int32: int32(statusCode), Valid: true},
				Attempts:    attempts,
				NextRetryAt: pgtype.Timestamptz{},
				DeliveredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
		}
	}
}

func (w *WebhookDeliveryWorker) deliver(ctx context.Context, webhook db.Webhook, delivery db.WebhookDelivery) (int, error) {
	payload := delivery.Payload

	// Sign with HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(webhook.Secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, "POST", webhook.Url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UniPost-Signature", signature)
	req.Header.Set("X-UniPost-Event", delivery.Event)

	resp, err := w.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to deliver webhook: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

func intPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
