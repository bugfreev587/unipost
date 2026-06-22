package quotaemail

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/loops"
)

func TestServiceSendsHighestUnsentThreshold(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			WorkspaceID:   "ws_123",
			WorkspaceName: "Acme",
			UserID:        "user_123",
			OwnerEmail:    "owner@example.com",
			OwnerName:     "Ada Lovelace",
			PlanID:        "free",
			Period:        "2026-06",
			Usage:         89,
			Limit:         100,
		},
		attempted: map[int]bool{80: true},
	}
	sender := &fakeSender{}
	svc := NewService(Config{
		Store:           store,
		Sender:          sender,
		TransactionalID: "tmpl_quota",
		PricingURL:      "https://unipost.dev/pricing",
		AppBaseURL:      "https://dev-app.unipost.dev",
	})

	if err := svc.EvaluateAndSend(context.Background(), Evaluation{WorkspaceID: "ws_123"}); err != nil {
		t.Fatalf("EvaluateAndSend returned error: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sender.sent))
	}
	if got := store.created[0].ThresholdPercent; got != 85 {
		t.Fatalf("threshold = %d, want 85", got)
	}
	if got := sender.sent[0].DataVariables["subject"]; got != "UniPost Free plan quota reached 85%" {
		t.Fatalf("subject = %#v", got)
	}
	if got := sender.sent[0].DataVariables["recipient_name"]; got != "Ada" {
		t.Fatalf("recipient_name = %#v", got)
	}
	if store.sentIDs[0] != store.created[0].ID {
		t.Fatalf("sent id = %q, want %q", store.sentIDs[0], store.created[0].ID)
	}
}

func TestServiceSkipsPaidPlans(t *testing.T) {
	store := &fakeStore{snapshot: Snapshot{
		WorkspaceID: "ws_123",
		OwnerEmail:  "owner@example.com",
		PlanID:      "basic",
		Period:      "2026-06",
		Usage:       100,
		Limit:       100,
	}}
	sender := &fakeSender{}
	svc := NewService(Config{Store: store, Sender: sender, TransactionalID: "tmpl_quota"})

	if err := svc.EvaluateAndSend(context.Background(), Evaluation{WorkspaceID: "ws_123"}); err != nil {
		t.Fatalf("EvaluateAndSend returned error: %v", err)
	}

	if len(sender.sent) != 0 {
		t.Fatalf("sent count = %d, want 0", len(sender.sent))
	}
	if len(store.created) != 0 {
		t.Fatalf("created ledger rows = %d, want 0", len(store.created))
	}
}

func TestServiceSuppressesAlreadyAttemptedThreshold(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			WorkspaceID: "ws_123",
			OwnerEmail:  "owner@example.com",
			PlanID:      "free",
			Period:      "2026-06",
			Usage:       95,
			Limit:       100,
		},
		attempted: map[int]bool{80: true, 85: true, 90: true, 95: true},
	}
	sender := &fakeSender{}
	svc := NewService(Config{Store: store, Sender: sender, TransactionalID: "tmpl_quota"})

	if err := svc.EvaluateAndSend(context.Background(), Evaluation{WorkspaceID: "ws_123"}); err != nil {
		t.Fatalf("EvaluateAndSend returned error: %v", err)
	}

	if len(sender.sent) != 0 {
		t.Fatalf("sent count = %d, want 0", len(sender.sent))
	}
}

func TestServiceDoesNotBackfillLowerThresholdsAfterHigherAttempt(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			WorkspaceID: "ws_123",
			OwnerEmail:  "owner@example.com",
			PlanID:      "free",
			Period:      "2026-06",
			Usage:       92,
			Limit:       100,
		},
		attempted: map[int]bool{90: true},
	}
	sender := &fakeSender{}
	svc := NewService(Config{Store: store, Sender: sender, TransactionalID: "tmpl_quota"})

	if err := svc.EvaluateAndSend(context.Background(), Evaluation{WorkspaceID: "ws_123"}); err != nil {
		t.Fatalf("EvaluateAndSend returned error: %v", err)
	}

	if len(sender.sent) != 0 {
		t.Fatalf("sent count = %d, want 0", len(sender.sent))
	}
}

func TestServiceMarksProviderFailureRetryable(t *testing.T) {
	store := &fakeStore{snapshot: Snapshot{
		WorkspaceID: "ws_123",
		OwnerEmail:  "owner@example.com",
		PlanID:      "free",
		Period:      "2026-06",
		Usage:       80,
		Limit:       100,
	}}
	sender := &fakeSender{err: errors.New("loops down")}
	svc := NewService(Config{Store: store, Sender: sender, TransactionalID: "tmpl_quota"})

	if err := svc.EvaluateAndSend(context.Background(), Evaluation{WorkspaceID: "ws_123"}); err != nil {
		t.Fatalf("EvaluateAndSend returned error: %v", err)
	}

	if len(store.failed) != 1 {
		t.Fatalf("failed marks = %d, want 1", len(store.failed))
	}
	if store.failed[0].reason != "loops down" {
		t.Fatalf("failure reason = %q", store.failed[0].reason)
	}
}

func TestServiceSendsBlockedWarningAt100Percent(t *testing.T) {
	store := &fakeStore{snapshot: Snapshot{
		WorkspaceID:   "ws_123",
		WorkspaceName: "Acme",
		UserID:        "user_123",
		OwnerEmail:    "owner@example.com",
		PlanID:        "free",
		Period:        "2026-06",
		Usage:         122,
		Reserved:      3,
		Limit:         100,
	}}
	sender := &fakeSender{}
	svc := NewService(Config{
		Store:           store,
		Sender:          sender,
		TransactionalID: "tmpl_quota",
		PricingURL:      "https://unipost.dev/pricing",
		AppBaseURL:      "https://dev-app.unipost.dev",
	})

	if err := svc.EvaluateAndSend(context.Background(), Evaluation{WorkspaceID: "ws_123", Blocked: true}); err != nil {
		t.Fatalf("EvaluateAndSend returned error: %v", err)
	}

	vars := sender.sent[0].DataVariables
	if got := store.created[0].ThresholdPercent; got != 100 {
		t.Fatalf("threshold = %d, want 100", got)
	}
	if got := vars["subject"]; got != "Warning: UniPost Free plan quota reached 100%" {
		t.Fatalf("subject = %#v", got)
	}
	if got := vars["usage_percent"]; got != "100" {
		t.Fatalf("usage_percent = %#v, want 100", got)
	}
	if got := vars["posts_used_or_reserved"]; got != "100" {
		t.Fatalf("posts_used_or_reserved = %#v, want clamped 100", got)
	}
	if got := vars["remaining_posts"]; got != "0" {
		t.Fatalf("remaining_posts = %#v, want 0", got)
	}
	if got := vars["reset_message"]; got != "Your Free plan quota resets on the first day of the next month. Until then, new publish requests remain blocked unless you upgrade." {
		t.Fatalf("reset_message = %#v", got)
	}
}

type fakeStore struct {
	snapshot  Snapshot
	attempted map[int]bool
	created   []Reminder
	sentIDs   []string
	failed    []struct {
		id     string
		reason string
	}
}

func (f *fakeStore) Snapshot(context.Context, string, string) (Snapshot, error) {
	return f.snapshot, nil
}

func (f *fakeStore) AttemptedThresholds(context.Context, string, string) (map[int]bool, error) {
	if f.attempted == nil {
		return map[int]bool{}, nil
	}
	out := make(map[int]bool, len(f.attempted))
	for threshold, attempted := range f.attempted {
		out[threshold] = attempted
	}
	return out, nil
}

func (f *fakeStore) CreatePending(_ context.Context, reminder Reminder) (Reminder, bool, error) {
	reminder.ID = "reminder_1"
	reminder.CreatedAt = time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	f.created = append(f.created, reminder)
	return reminder, true, nil
}

func (f *fakeStore) MarkSent(_ context.Context, id string) error {
	f.sentIDs = append(f.sentIDs, id)
	return nil
}

func (f *fakeStore) MarkFailed(_ context.Context, id, reason string) error {
	f.failed = append(f.failed, struct {
		id     string
		reason string
	}{id: id, reason: reason})
	return nil
}

type fakeSender struct {
	sent []loops.TransactionalEmail
	err  error
}

func (f *fakeSender) SendTransactional(_ context.Context, email loops.TransactionalEmail) error {
	f.sent = append(f.sent, email)
	return f.err
}
