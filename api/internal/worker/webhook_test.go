package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
)

var _ events.DurableEventSink = (*WebhookDeliveryWorker)(nil)

func TestWebhookPublishDurableCreatesOneDeliveryPerSubscriptionAndReplayIsIdempotent(t *testing.T) {
	fake := newWebhookSinkDB(
		db.Webhook{ID: "wh-1", WorkspaceID: "workspace-1", Active: true},
		db.Webhook{ID: "wh-2", WorkspaceID: "workspace-1", Active: true},
	)
	worker := NewWebhookDeliveryWorker(db.New(fake), nil)
	payload := []byte(`{"post_id":"post-1","status":"published"}`)

	for attempt := 0; attempt < 2; attempt++ {
		if err := worker.PublishDurable(context.Background(), "event-1", "workspace-1", "post.published", payload); err != nil {
			t.Fatalf("PublishDurable attempt %d: %v", attempt+1, err)
		}
	}

	if got := len(fake.durableDeliveries); got != 2 {
		t.Fatalf("durable deliveries = %d, want one per subscription after replay (2)", got)
	}
	for _, webhookID := range []string{"wh-1", "wh-2"} {
		stored, ok := fake.durableDeliveries[webhookID+"/event-1"]
		if !ok {
			t.Fatalf("missing durable delivery for %s", webhookID)
		}
		var envelope struct {
			Event   string          `json:"event"`
			EventID string          `json:"event_id"`
			Data    json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(stored, &envelope); err != nil {
			t.Fatalf("decode delivery for %s: %v", webhookID, err)
		}
		if envelope.Event != "post.published" || envelope.EventID != "event-1" {
			t.Fatalf("delivery identity = (%q, %q), want (post.published, event-1)", envelope.Event, envelope.EventID)
		}
		if string(envelope.Data) != string(payload) {
			t.Fatalf("delivery data = %s, want %s", envelope.Data, payload)
		}
	}
}

func TestWebhookPublishDurableReturnsInsertErrorAndReplayCompletesMissingDelivery(t *testing.T) {
	insertErr := errors.New("database unavailable")
	fake := newWebhookSinkDB(
		db.Webhook{ID: "wh-fail", WorkspaceID: "workspace-1", Active: true},
		db.Webhook{ID: "wh-ok", WorkspaceID: "workspace-1", Active: true},
	)
	fake.failures["wh-fail"] = []error{insertErr}
	worker := NewWebhookDeliveryWorker(db.New(fake), nil)

	err := worker.PublishDurable(context.Background(), "event-1", "workspace-1", "post.failed", []byte(`{"post_id":"post-1"}`))
	if !errors.Is(err, insertErr) {
		t.Fatalf("first PublishDurable error = %v, want database error", err)
	}
	if !strings.Contains(err.Error(), "wh-fail") {
		t.Fatalf("first PublishDurable error = %q, want failing webhook id for retry diagnostics", err)
	}
	if _, ok := fake.durableDeliveries["wh-ok/event-1"]; !ok {
		t.Fatal("a failed subscription must not prevent later subscriptions from being enqueued")
	}

	if err := worker.PublishDurable(context.Background(), "event-1", "workspace-1", "post.failed", []byte(`{"post_id":"post-1"}`)); err != nil {
		t.Fatalf("replay PublishDurable: %v", err)
	}
	if got := len(fake.durableDeliveries); got != 2 {
		t.Fatalf("durable deliveries after replay = %d, want 2", got)
	}
}

func TestWebhookPublishKeepsLegacyNonDurableEnqueueSemantics(t *testing.T) {
	fake := newWebhookSinkDB(db.Webhook{ID: "wh-legacy", WorkspaceID: "workspace-1", Active: true})
	worker := NewWebhookDeliveryWorker(db.New(fake), nil)

	worker.Publish(context.Background(), "workspace-1", "account.connected", map[string]any{"account_id": "account-1"})

	if got := len(fake.legacyDeliveries); got != 1 {
		t.Fatalf("legacy deliveries = %d, want 1", got)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(fake.legacyDeliveries[0], &envelope); err != nil {
		t.Fatalf("decode legacy delivery: %v", err)
	}
	if _, exists := envelope["event_id"]; exists {
		t.Fatal("legacy Publish must not add durable event_id semantics")
	}
	if got := string(envelope["event"]); got != `"account.connected"` {
		t.Fatalf("legacy event = %s, want account.connected", got)
	}
}

type webhookSinkDB struct {
	webhooks          []db.Webhook
	durableDeliveries map[string][]byte
	legacyDeliveries  [][]byte
	failures          map[string][]error
}

func newWebhookSinkDB(webhooks ...db.Webhook) *webhookSinkDB {
	return &webhookSinkDB{
		webhooks:          webhooks,
		durableDeliveries: make(map[string][]byte),
		failures:          make(map[string][]error),
	}
}

func (f *webhookSinkDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	if !strings.Contains(query, "INSERT INTO webhook_deliveries") || !strings.Contains(query, "event_id") {
		return pgconn.CommandTag{}, fmt.Errorf("unexpected Exec query: %s", query)
	}
	webhookID, ok := args[0].(string)
	if !ok {
		return pgconn.CommandTag{}, fmt.Errorf("webhook id type = %T", args[0])
	}
	if failures := f.failures[webhookID]; len(failures) > 0 {
		f.failures[webhookID] = failures[1:]
		return pgconn.CommandTag{}, failures[0]
	}
	eventID, ok := args[3].(pgtype.Text)
	if !ok || !eventID.Valid {
		return pgconn.CommandTag{}, fmt.Errorf("event id = %#v, want valid pgtype.Text", args[3])
	}
	key := webhookID + "/" + eventID.String
	if _, exists := f.durableDeliveries[key]; !exists {
		f.durableDeliveries[key] = append([]byte(nil), args[2].([]byte)...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (f *webhookSinkDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if !strings.Contains(query, "FROM webhooks") {
		return nil, fmt.Errorf("unexpected Query: %s", query)
	}
	rows := make([][]any, 0, len(f.webhooks))
	for _, webhook := range f.webhooks {
		rows = append(rows, []any{
			webhook.ID,
			webhook.Url,
			webhook.Secret,
			webhook.Events,
			webhook.Active,
			webhook.CreatedAt,
			webhook.WorkspaceID,
			webhook.Name,
		})
	}
	return &webhookRows{rows: rows, index: -1}, nil
}

func (f *webhookSinkDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	if !strings.Contains(query, "INSERT INTO webhook_deliveries") || strings.Contains(query, "event_id)") {
		return webhookRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
	payload := append([]byte(nil), args[2].([]byte)...)
	f.legacyDeliveries = append(f.legacyDeliveries, payload)
	return webhookRow{values: []any{
		"delivery-legacy",
		args[0].(string),
		args[1].(string),
		payload,
		pgtype.Int4{},
		int32(0),
		pgtype.Timestamptz{},
		pgtype.Timestamptz{},
		pgtype.Timestamptz{},
		pgtype.Text{},
	}}
}

type webhookRow struct {
	values []any
	err    error
}

func (r webhookRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignWebhookValues(dest, r.values)
}

type webhookRows struct {
	rows   [][]any
	index  int
	closed bool
	err    error
}

func (r *webhookRows) Close()                                       { r.closed = true }
func (r *webhookRows) Err() error                                   { return r.err }
func (r *webhookRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *webhookRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *webhookRows) Next() bool {
	if r.closed {
		return false
	}
	r.index++
	if r.index >= len(r.rows) {
		r.Close()
		return false
	}
	return true
}
func (r *webhookRows) Scan(dest ...any) error { return assignWebhookValues(dest, r.rows[r.index]) }
func (r *webhookRows) Values() ([]any, error) { return r.rows[r.index], nil }
func (r *webhookRows) RawValues() [][]byte    { return nil }
func (r *webhookRows) Conn() *pgx.Conn        { return nil }

func assignWebhookValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("scan destinations = %d, values = %d", len(dest), len(values))
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return fmt.Errorf("scan destination %d is %T, want non-nil pointer", i, dest[i])
		}
		target.Elem().Set(reflect.ValueOf(values[i]))
	}
	return nil
}
