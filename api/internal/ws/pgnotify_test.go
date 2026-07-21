package ws

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
)

type capturedPGNotify struct {
	channel string
	payload string
}

type captureNotifyExecutor struct {
	calls []capturedPGNotify
}

func (e *captureNotifyExecutor) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	e.calls = append(e.calls, capturedPGNotify{channel: args[0].(string), payload: args[1].(string)})
	return pgconn.CommandTag{}, nil
}

func TestPGListenerScopedNotificationRoutesManagedOwnerAndAggregateOnly(t *testing.T) {
	hub := NewHub()
	aggregate := mustSubscribeInboxScope(t, hub, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeWorkspace})
	managedA := mustSubscribeInboxScope(t, hub, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed-a"})
	managedB := mustSubscribeInboxScope(t, hub, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed-b"})
	legacy := hub.Subscribe("workspace-1")

	payload := []byte(`{"type":"inbox.new_item","workspace_id":"workspace-1","external_user_id":"managed-a","item":{"id":"item-a"}}`)
	listener := NewPGListener(hub, nil, nil)
	if err := listener.forwardNotification(inboxChannel, payload); err != nil {
		t.Fatalf("forward managed notification: %v", err)
	}

	assertInboxNotificationReceived(t, aggregate, payload)
	assertInboxNotificationReceived(t, managedA, payload)
	assertInboxNotificationNotReceived(t, managedB)
	assertInboxNotificationNotReceived(t, legacy)
}

func TestPGListenerScopedNotificationBYOAndMalformedFailClosed(t *testing.T) {
	hub := NewHub()
	aggregate := mustSubscribeInboxScope(t, hub, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeWorkspace})
	managedA := mustSubscribeInboxScope(t, hub, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed-a"})
	listener := NewPGListener(hub, nil, nil)

	payload := []byte(`{"type":"inbox.sync_complete","workspace_id":"workspace-1","new_items":2}`)
	if err := listener.forwardNotification(inboxChannel, payload); err != nil {
		t.Fatalf("forward BYO notification: %v", err)
	}
	assertInboxNotificationReceived(t, aggregate, payload)
	assertInboxNotificationNotReceived(t, managedA)

	for _, malformed := range [][]byte{
		[]byte(`not-json`),
		[]byte(`{"type":"inbox.new_item","workspace_id":""}`),
		[]byte(`{"type":"inbox.new_item","workspace_id":"workspace-1","external_user_id":""}`),
		[]byte(`{"type":"inbox.new_item","workspace_id":"workspace-1","external_user_id":" managed-a "}`),
	} {
		if err := listener.forwardNotification(inboxChannel, malformed); err == nil {
			t.Fatalf("malformed payload was accepted: %s", malformed)
		}
	}
	assertInboxNotificationNotReceived(t, aggregate)
	assertInboxNotificationNotReceived(t, managedA)
}

func TestNotifyEventScopedNotificationOverridesRoutingFields(t *testing.T) {
	executor := &captureNotifyExecutor{}
	notifyEventWithExecutor(context.Background(), executor, "workspace-a", "managed-a", map[string]any{
		"type":             "inbox.sync_complete",
		"workspace_id":     "workspace-b",
		"external_user_id": "managed-b",
		"new_items":        3,
	})
	notifyWorkspaceEventWithExecutor(context.Background(), executor, "workspace-a", map[string]any{
		"type":             "inbox.sync_complete",
		"workspace_id":     "workspace-b",
		"external_user_id": "managed-b",
		"new_items":        5,
	})

	if len(executor.calls) != 2 {
		t.Fatalf("pg_notify calls = %d, want 2", len(executor.calls))
	}
	managed := decodeCapturedPGNotify(t, executor.calls[0])
	wantManaged := map[string]any{
		"type": "inbox.sync_complete", "workspace_id": "workspace-a",
		"external_user_id": "managed-a", "new_items": float64(3),
	}
	if !reflect.DeepEqual(managed, wantManaged) {
		t.Fatalf("managed envelope = %#v, want %#v", managed, wantManaged)
	}
	aggregate := decodeCapturedPGNotify(t, executor.calls[1])
	wantAggregate := map[string]any{
		"type": "inbox.sync_complete", "workspace_id": "workspace-a", "new_items": float64(5),
	}
	if !reflect.DeepEqual(aggregate, wantAggregate) {
		t.Fatalf("aggregate envelope = %#v, want %#v", aggregate, wantAggregate)
	}
}

func TestNotifyItemWithExecutorBuildsScopedNewItemEnvelope(t *testing.T) {
	tests := []struct {
		name           string
		externalUserID string
		wantOwner      string
		wantOwnerKey   bool
	}{
		{name: "managed owner", externalUserID: "managed-a", wantOwner: "managed-a", wantOwnerKey: true},
		{name: "BYO owner omitted"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &captureNotifyExecutor{}
			item := map[string]any{
				"id":               "item-a",
				"workspace_id":     "payload-workspace",
				"external_user_id": "payload-managed-b",
			}

			notifyItemWithExecutor(context.Background(), executor, "workspace-a", test.externalUserID, item)

			if len(executor.calls) != 1 {
				t.Fatalf("pg_notify calls = %d, want 1", len(executor.calls))
			}
			payload := decodeCapturedPGNotify(t, executor.calls[0])
			if payload["type"] != "inbox.new_item" || payload["workspace_id"] != "workspace-a" {
				t.Fatalf("routing envelope = %#v", payload)
			}
			owner, hasOwner := payload["external_user_id"]
			if hasOwner != test.wantOwnerKey {
				t.Fatalf("external_user_id presence = %t, want %t in %#v", hasOwner, test.wantOwnerKey, payload)
			}
			if test.wantOwnerKey && owner != test.wantOwner {
				t.Fatalf("external_user_id = %#v, want DB owner %q", owner, test.wantOwner)
			}
			gotItem, ok := payload["item"].(map[string]any)
			if !ok || !reflect.DeepEqual(gotItem, item) {
				t.Fatalf("item = %#v, want preserved %#v", payload["item"], item)
			}
		})
	}
}

func TestBroadcastInboxItemScopedNotification(t *testing.T) {
	hub := NewHub()
	aggregate := mustSubscribeInboxScope(t, hub, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeWorkspace})
	managedA := mustSubscribeInboxScope(t, hub, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed-a"})
	managedB := mustSubscribeInboxScope(t, hub, inboxaccess.Scope{WorkspaceID: "workspace-1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed-b"})

	hub.BroadcastInboxItem("workspace-1", "managed-a", map[string]any{"id": "item-a"})

	assertInboxNotificationReceivedType(t, aggregate, "inbox.new_item")
	assertInboxNotificationReceivedType(t, managedA, "inbox.new_item")
	assertInboxNotificationNotReceived(t, managedB)
}

func mustSubscribeInboxScope(t *testing.T, hub *Hub, scope inboxaccess.Scope) *Subscription {
	t.Helper()
	subscription, ok := hub.SubscribeScope(scope)
	if !ok {
		t.Fatalf("subscribe scope: %+v", scope)
	}
	return subscription
}

func assertInboxNotificationReceived(t *testing.T, subscription *Subscription, want []byte) {
	t.Helper()
	select {
	case got := <-subscription.C():
		if string(got) != string(want) {
			t.Fatalf("notification = %s, want exact %s", got, want)
		}
	default:
		t.Fatal("expected notification was not delivered")
	}
}

func assertInboxNotificationNotReceived(t *testing.T, subscription *Subscription) {
	t.Helper()
	select {
	case got := <-subscription.C():
		t.Fatalf("unexpected notification delivered: %s", got)
	default:
	}
}

func assertInboxNotificationReceivedType(t *testing.T, subscription *Subscription, wantType string) {
	t.Helper()
	select {
	case raw := <-subscription.C():
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("decode notification: %v", err)
		}
		if payload["type"] != wantType {
			t.Fatalf("notification type = %#v, want %q", payload["type"], wantType)
		}
	default:
		t.Fatal("expected notification was not delivered")
	}
}

func decodeCapturedPGNotify(t *testing.T, call capturedPGNotify) map[string]any {
	t.Helper()
	if call.channel != inboxChannel {
		t.Fatalf("channel = %q, want %q", call.channel, inboxChannel)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(call.payload), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}
