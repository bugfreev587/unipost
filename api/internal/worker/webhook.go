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
)

// WebhookDeliveryWorker delivers webhook events in the background.
type WebhookDeliveryWorker struct {
	queries *db.Queries
	client  *http.Client
}

func NewWebhookDeliveryWorker(queries *db.Queries) *WebhookDeliveryWorker {
	return &WebhookDeliveryWorker{
		queries: queries,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
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

// Enqueue creates webhook delivery records for all matching webhooks.
func (w *WebhookDeliveryWorker) Enqueue(ctx context.Context, projectID string, event string, data any) error {
	webhooks, err := w.queries.ListWebhooksByProjectAndEvent(ctx, db.ListWebhooksByProjectAndEventParams{
		ProjectID: projectID,
		Event:     event,
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

		statusCode, err := w.deliver(ctx, webhook, d)
		attempts := d.Attempts + 1

		if err != nil || statusCode < 200 || statusCode >= 300 {
			// Schedule retry with exponential backoff (max 3 attempts)
			if attempts >= 3 {
				slog.Warn("webhook delivery: giving up", "delivery_id", d.ID, "attempts", attempts)
				w.queries.UpdateWebhookDelivery(ctx, db.UpdateWebhookDeliveryParams{
					ID:          d.ID,
					StatusCode:  pgtype.Int4{Int32: int32(statusCode), Valid: statusCode > 0},
					Attempts:    attempts,
					NextRetryAt: pgtype.Timestamptz{},
					DeliveredAt: pgtype.Timestamptz{},
				})
			} else {
				delays := []time.Duration{1 * time.Minute, 5 * time.Minute, 30 * time.Minute}
				nextRetry := time.Now().Add(delays[attempts-1])
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
