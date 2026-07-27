package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/loops"
)

var _ events.DurableEventSink = (*NotificationDispatcher)(nil)
var _ events.DurableEventSink = (*LoopsNotificationEmailBus)(nil)

type fakeDurableNotificationStore struct {
	targets    []db.ResolveNotificationTargetsRow
	resolveErr error
	insertErr  error
	created    map[string]db.CreateNotificationDeliveryParams
}

func (s *fakeDurableNotificationStore) ResolveNotificationTargets(context.Context, db.ResolveNotificationTargetsParams) ([]db.ResolveNotificationTargetsRow, error) {
	return s.targets, s.resolveErr
}

func (s *fakeDurableNotificationStore) CreateNotificationDelivery(_ context.Context, arg db.CreateNotificationDeliveryParams) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	if s.created == nil {
		s.created = make(map[string]db.CreateNotificationDeliveryParams)
	}
	key := arg.EventID + ":" + arg.ChannelID
	if _, exists := s.created[key]; !exists {
		s.created[key] = arg
	}
	return nil
}

func (s *fakeDurableNotificationStore) CreateSkippedNotificationDelivery(_ context.Context, arg db.CreateSkippedNotificationDeliveryParams) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	if s.created == nil {
		s.created = make(map[string]db.CreateNotificationDeliveryParams)
	}
	key := arg.EventID + ":" + arg.ChannelID
	if _, exists := s.created[key]; !exists {
		s.created[key] = db.CreateNotificationDeliveryParams{
			SubscriptionID: arg.SubscriptionID,
			ChannelID:      arg.ChannelID,
			EventType:      arg.EventType,
			EventID:        arg.EventID,
			Payload:        arg.Payload,
		}
	}
	return nil
}

func TestNotificationPublishDurableUsesStableDeliveryLedgerIdentity(t *testing.T) {
	store := &fakeDurableNotificationStore{targets: []db.ResolveNotificationTargetsRow{{
		SubscriptionID: "subscription_1",
		ChannelID:      "channel_1",
		EventType:      events.EventPostPublished,
		ChannelKind:    "slack_webhook",
	}}}
	dispatcher := &NotificationDispatcher{queries: store}
	payload := []byte(`{"id":"post_1"}`)

	for attempt := 0; attempt < 2; attempt++ {
		if err := dispatcher.PublishDurable(context.Background(), "event_1", "workspace_1", events.EventPostPublished, payload); err != nil {
			t.Fatalf("PublishDurable attempt %d: %v", attempt+1, err)
		}
	}

	if len(store.created) != 1 {
		t.Fatalf("durable delivery rows = %d, want 1", len(store.created))
	}
	created := store.created["event_1:channel_1"]
	if created.EventID != "event_1" || created.ChannelID != "channel_1" {
		t.Fatalf("durable delivery identity = %q/%q", created.EventID, created.ChannelID)
	}
}

func TestNotificationPublishDurableReturnsLedgerFailures(t *testing.T) {
	wantErr := errors.New("notification ledger unavailable")
	store := &fakeDurableNotificationStore{
		targets: []db.ResolveNotificationTargetsRow{{
			SubscriptionID: "subscription_1",
			ChannelID:      "channel_1",
			EventType:      events.EventPostFailed,
			ChannelKind:    "slack_webhook",
		}},
		insertErr: wantErr,
	}
	dispatcher := &NotificationDispatcher{queries: store}

	err := dispatcher.PublishDurable(context.Background(), "event_1", "workspace_1", events.EventPostFailed, []byte(`{}`))
	if !errors.Is(err, wantErr) {
		t.Fatalf("PublishDurable error = %v, want %v", err, wantErr)
	}
}

type fakeDurableLoopsStore struct {
	workspace db.Workspace
	owner     db.User
	count     int32
	countErr  error
	spaceErr  error
	ownerErr  error
}

func (s *fakeDurableLoopsStore) CountPublishedPostsByWorkspace(context.Context, string) (int32, error) {
	return s.count, s.countErr
}

func (s *fakeDurableLoopsStore) GetWorkspace(context.Context, string) (db.Workspace, error) {
	return s.workspace, s.spaceErr
}

func (s *fakeDurableLoopsStore) GetUser(context.Context, string) (db.User, error) {
	return s.owner, s.ownerErr
}

func (s *fakeDurableLoopsStore) ListSocialAccountsByWorkspace(context.Context, string) ([]db.SocialAccount, error) {
	return nil, nil
}

type deduplicatingLifecycleSender struct {
	err       error
	attempts  []loops.LifecycleEvent
	accepted  map[string]bool
	realSends int
}

type strictLifecycleSender struct {
	strictErr   error
	legacyCalls int
	strictCalls int
}

func (s *strictLifecycleSender) SendLifecycleEvent(context.Context, loops.LifecycleEvent) error {
	s.legacyCalls++
	return nil
}

func (s *strictLifecycleSender) SendLifecycleEventDurable(context.Context, loops.LifecycleEvent) error {
	s.strictCalls++
	return s.strictErr
}

func (s *deduplicatingLifecycleSender) SendLifecycleEvent(_ context.Context, event loops.LifecycleEvent) error {
	s.attempts = append(s.attempts, event)
	if s.err != nil {
		return s.err
	}
	if s.accepted == nil {
		s.accepted = make(map[string]bool)
	}
	if !s.accepted[event.IdempotencyKey] {
		s.accepted[event.IdempotencyKey] = true
		s.realSends++
	}
	return nil
}

func TestLoopsPublishDurableUsesWorkspaceProviderIdentity(t *testing.T) {
	store := &fakeDurableLoopsStore{
		workspace: db.Workspace{ID: "workspace_1", UserID: "user_1", Name: "Workspace"},
		owner:     db.User{ID: "user_1", Email: "owner@example.com"},
	}
	sender := &deduplicatingLifecycleSender{}
	bus := &LoopsNotificationEmailBus{queries: store, syncer: sender, appBaseURL: "https://app.unipost.dev"}
	payload := []byte(`{"id":"post_1","first_post_published":true}`)

	for attempt := 0; attempt < 2; attempt++ {
		if err := bus.PublishDurable(context.Background(), "event_1", "workspace_1", events.EventPostPublished, payload); err != nil {
			t.Fatalf("PublishDurable attempt %d: %v", attempt+1, err)
		}
	}

	if len(sender.attempts) != 2 {
		t.Fatalf("provider attempts = %d, want 2 outbox attempts", len(sender.attempts))
	}
	wantKey := "first_post_published:workspace_1"
	for i, attempt := range sender.attempts {
		if attempt.IdempotencyKey != wantKey {
			t.Fatalf("attempt %d provider key = %q, want %q", i+1, attempt.IdempotencyKey, wantKey)
		}
	}
	if sender.realSends != 1 {
		t.Fatalf("real provider sends = %d, want 1", sender.realSends)
	}

	if err := bus.PublishDurable(context.Background(), "event_2", "workspace_1", events.EventPostPublished, payload); err != nil {
		t.Fatalf("second logical event: %v", err)
	}
	if got := sender.attempts[2].IdempotencyKey; got != wantKey {
		t.Fatalf("second event provider key = %q", got)
	}
}

func TestLoopsPublishDurableReturnsProviderAndLedgerFailures(t *testing.T) {
	payload := []byte(`{"id":"post_1","first_post_published":true}`)
	wantProviderErr := errors.New("loops unavailable")
	store := &fakeDurableLoopsStore{
		workspace: db.Workspace{ID: "workspace_1", UserID: "user_1"},
		owner:     db.User{ID: "user_1", Email: "owner@example.com"},
	}
	providerBus := &LoopsNotificationEmailBus{
		queries: store,
		syncer:  &deduplicatingLifecycleSender{err: wantProviderErr},
	}
	if err := providerBus.PublishDurable(context.Background(), "event_1", "workspace_1", events.EventPostPublished, payload); !errors.Is(err, wantProviderErr) {
		t.Fatalf("provider error = %v, want %v", err, wantProviderErr)
	}

	wantLedgerErr := errors.New("workspace ledger unavailable")
	ledgerBus := &LoopsNotificationEmailBus{
		queries: &fakeDurableLoopsStore{spaceErr: wantLedgerErr},
		syncer:  &deduplicatingLifecycleSender{},
	}
	if err := ledgerBus.PublishDurable(context.Background(), "event_1", "workspace_1", events.EventPostPublished, payload); !errors.Is(err, wantLedgerErr) {
		t.Fatalf("ledger error = %v, want %v", err, wantLedgerErr)
	}
}

func TestLoopsPublishDurableUsesStrictLifecycleSender(t *testing.T) {
	wantErr := errors.New("strict provider failure")
	sender := &strictLifecycleSender{strictErr: wantErr}
	bus := &LoopsNotificationEmailBus{
		queries: &fakeDurableLoopsStore{
			workspace: db.Workspace{ID: "workspace_1", UserID: "user_1"},
			owner:     db.User{ID: "user_1", Email: "owner@example.com"},
		},
		syncer: sender,
	}
	err := bus.PublishDurable(
		context.Background(), "event_1", "workspace_1", events.EventPostPublished,
		[]byte(`{"id":"post_1","first_post_published":true}`),
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("strict sender error = %v, want %v", err, wantErr)
	}
	if sender.strictCalls != 1 || sender.legacyCalls != 0 {
		t.Fatalf("sender calls strict=%d legacy=%d, want 1/0", sender.strictCalls, sender.legacyCalls)
	}
}
