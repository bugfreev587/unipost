package loops

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
)

func TestSendLifecycleEventDurableReturnsProviderFailures(t *testing.T) {
	event := LifecycleEvent{
		UserID: "user_1", Email: "owner@example.com", EventName: "first_post_published",
		IdempotencyKey: "terminal_post_event:loops:event_1:user_1",
	}

	t.Run("contact", func(t *testing.T) {
		wantErr := errors.New("contact provider unavailable")
		syncer := NewSyncer(&fakeLifecycleClient{contactErr: wantErr}, Options{})
		if err := syncer.SendLifecycleEventDurable(context.Background(), event); !errors.Is(err, wantErr) {
			t.Fatalf("durable contact error = %v, want %v", err, wantErr)
		}
	})

	t.Run("event", func(t *testing.T) {
		wantErr := errors.New("event provider unavailable")
		syncer := NewSyncer(&fakeLifecycleClient{eventErr: wantErr}, Options{})
		if err := syncer.SendLifecycleEventDurable(context.Background(), event); !errors.Is(err, wantErr) {
			t.Fatalf("durable event error = %v, want %v", err, wantErr)
		}
	})

	t.Run("transactional", func(t *testing.T) {
		wantErr := errors.New("transactional provider unavailable")
		syncer := NewSyncer(&fakeLifecycleClient{transactionalErr: wantErr}, Options{
			TransactionalIDs: TransactionalIDs{PostFailed: "template_post_failed"},
		})
		transactionalEvent := event
		transactionalEvent.EventName = "post_failed"
		if err := syncer.SendLifecycleEventDurable(context.Background(), transactionalEvent); !errors.Is(err, wantErr) {
			t.Fatalf("durable transactional error = %v, want %v", err, wantErr)
		}
	})
}

func TestSendLifecycleEventDurableReturnsPolicyAndAuditFailures(t *testing.T) {
	event := LifecycleEvent{
		UserID: "user_1", Email: "owner@example.com", EventName: "post_failed",
		IdempotencyKey: "terminal_post_event:loops:event_1:user_1",
	}

	t.Run("policy", func(t *testing.T) {
		wantErr := errors.New("email policy ledger unavailable")
		syncer := NewSyncer(&fakeLifecycleClient{}, Options{
			TransactionalIDs: TransactionalIDs{PostFailed: "template_post_failed"},
			EmailPolicy:      &fakeEmailPolicy{err: wantErr},
		})
		if err := syncer.SendLifecycleEventDurable(context.Background(), event); !errors.Is(err, wantErr) {
			t.Fatalf("durable policy error = %v, want %v", err, wantErr)
		}
	})

	t.Run("audit", func(t *testing.T) {
		wantErr := errors.New("email audit ledger unavailable")
		syncer := NewSyncer(&fakeLifecycleClient{}, Options{
			TransactionalIDs: TransactionalIDs{PostFailed: "template_post_failed"},
			EmailAuditStore:  &fakeEmailAuditStore{createErr: wantErr},
			EmailPolicy: &fakeEmailPolicy{decision: emailpolicy.Decision{
				ShouldSend: true,
			}},
		})
		if err := syncer.SendLifecycleEventDurable(context.Background(), event); !errors.Is(err, wantErr) {
			t.Fatalf("durable audit error = %v, want %v", err, wantErr)
		}
	})
}

func TestSendLifecycleEventKeepsLegacyBestEffortContract(t *testing.T) {
	syncer := NewSyncer(&fakeLifecycleClient{eventErr: errors.New("provider unavailable")}, Options{})
	err := syncer.SendLifecycleEvent(context.Background(), LifecycleEvent{
		UserID: "user_1", Email: "owner@example.com", EventName: "first_post_published",
		IdempotencyKey: "first_post_published:workspace_1",
	})
	if err != nil {
		t.Fatalf("legacy SendLifecycleEvent error = %v, want nil", err)
	}
}
