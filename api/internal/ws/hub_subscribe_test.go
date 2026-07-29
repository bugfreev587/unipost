package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
	"github.com/xiaoboyu/unipost-api/internal/requestevents"
)

func TestHubV2LogConnectionReceivesExactlyOneCanonicalRequestEvent(t *testing.T) {
	h := NewHub()
	h.WithLogReadSelector(staticLogReadSelector(true))
	connection := &Conn{send: make(chan []byte, 4)}
	h.RegisterLog("ws_1", connection)
	defer h.Unregister("ws_1", connection)

	legacy, _ := json.Marshal(LogEnvelope(db.IntegrationLog{ID: 42, WorkspaceID: "ws_1", Category: "api_request"}))
	canonical, _ := json.Marshal(RequestEventLogEnvelope(requestevents.Event{ID: "event-1", WorkspaceID: "ws_1"}))
	h.Broadcast("ws_1", legacy)
	h.Broadcast("ws_1", canonical)

	select {
	case payload := <-connection.send:
		var envelope struct {
			Log struct {
				ID string `json:"id"`
			} `json:"log"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Log.ID != "request:event-1" {
			t.Fatalf("id = %q", envelope.Log.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for canonical request event")
	}
	select {
	case payload := <-connection.send:
		t.Fatalf("v2 connection received duplicate request event: %s", payload)
	case <-time.After(25 * time.Millisecond):
	}
}

type staticLogReadSelector bool

func (s staticLogReadSelector) UseV2(context.Context) bool { return bool(s) }

type mutableCountingLogReadSelector struct {
	enabled atomic.Bool
	calls   atomic.Int64
}

func (s *mutableCountingLogReadSelector) UseV2(context.Context) bool {
	s.calls.Add(1)
	return s.enabled.Load()
}

func TestHubOpenLogConnectionsUseOneDynamicModeReadPerEnvelope(t *testing.T) {
	selector := &mutableCountingLogReadSelector{}
	selector.enabled.Store(true)
	h := NewHub().WithLogReadSelector(selector)
	first := &Conn{send: make(chan []byte, 2)}
	second := &Conn{send: make(chan []byte, 2)}
	h.RegisterLog("ws_1", first)
	h.RegisterLog("ws_1", second)
	defer h.Unregister("ws_1", first)
	defer h.Unregister("ws_1", second)

	canonical, _ := json.Marshal(RequestEventLogEnvelope(requestevents.Event{ID: "event-1", WorkspaceID: "ws_1"}))
	h.Broadcast("ws_1", canonical)
	assertHubLogID(t, first.send, "request:event-1")
	assertHubLogID(t, second.send, "request:event-1")
	if got := selector.calls.Load(); got != 1 {
		t.Fatalf("mode reads after first envelope = %d, want 1", got)
	}

	selector.enabled.Store(false)
	legacyAPI, _ := json.Marshal(LogEnvelope(db.IntegrationLog{ID: 42, WorkspaceID: "ws_1", Category: "api_request"}))
	h.Broadcast("ws_1", legacyAPI)
	assertHubLogID(t, first.send, int64(42))
	assertHubLogID(t, second.send, int64(42))
	if got := selector.calls.Load(); got != 2 {
		t.Fatalf("mode reads after second envelope = %d, want 2", got)
	}
}

func assertHubLogID(t *testing.T, messages <-chan []byte, want any) {
	t.Helper()
	select {
	case payload := <-messages:
		var envelope struct {
			Log map[string]any `json:"log"`
		}
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.UseNumber()
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		got := envelope.Log["id"]
		if number, ok := got.(json.Number); ok {
			got, _ = number.Int64()
		}
		if got != want {
			t.Fatalf("log id = %#v, want %#v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log envelope")
	}
}

func TestHubSubscribe_ReceivesBroadcast(t *testing.T) {
	h := NewHub()
	sub := h.Subscribe("ws_1")
	defer h.Unsubscribe("ws_1", sub)

	h.Broadcast("ws_1", []byte("hello"))

	select {
	case msg := <-sub.C():
		if string(msg) != "hello" {
			t.Fatalf("got %q, want hello", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
}

func TestHubSubscribe_IsolatedByWorkspace(t *testing.T) {
	h := NewHub()
	sub := h.Subscribe("ws_1")
	defer h.Unsubscribe("ws_1", sub)

	h.Broadcast("ws_2", []byte("other-workspace"))

	select {
	case msg := <-sub.C():
		t.Fatalf("received cross-workspace message: %q", msg)
	case <-time.After(100 * time.Millisecond):
		// expected: no delivery
	}
}

func TestHubUnsubscribe_ClosesChannelAndStopsDelivery(t *testing.T) {
	h := NewHub()
	sub := h.Subscribe("ws_1")
	h.Unsubscribe("ws_1", sub)

	// Channel must be closed.
	if _, open := <-sub.C(); open {
		t.Fatal("expected channel to be closed after Unsubscribe")
	}

	// A second Unsubscribe must not panic.
	h.Unsubscribe("ws_1", sub)
}

// TestHubBroadcast_ConcurrentChurn races Broadcast against Subscribe and
// Unsubscribe. Run with -race, it guards against concurrent map
// access and send-on-closed-channel panics.
func TestHubBroadcast_ConcurrentChurn(t *testing.T) {
	h := NewHub()
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				h.Broadcast("ws_1", []byte("x"))
			}
		}
	}()

	for i := 0; i < 200; i++ {
		sub := h.Subscribe("ws_1")
		// Drain a little so the buffer doesn't merely fill.
		select {
		case <-sub.C():
		default:
		}
		h.Unsubscribe("ws_1", sub)
	}
	close(done)
}

func TestHubBroadcast_DropsSlowSubscriber(t *testing.T) {
	h := NewHub()
	sub := h.Subscribe("ws_1")
	defer h.Unsubscribe("ws_1", sub)

	// Overflow the buffer; Broadcast must not block.
	for i := 0; i < 1000; i++ {
		h.Broadcast("ws_1", []byte("x"))
	}
}

func TestHubBroadcastInboxPartitionsManagedUsers(t *testing.T) {
	h := NewHub()
	workspaceOne := mustSubscribeScope(t, h, inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeWorkspace})
	managedA := mustSubscribeScope(t, h, inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed_a"})
	managedB := mustSubscribeScope(t, h, inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed_b"})
	workspaceTwo := mustSubscribeScope(t, h, inboxaccess.Scope{WorkspaceID: "ws_2", Mode: inboxaccess.ModeWorkspace})
	workspaceTwoA := mustSubscribeScope(t, h, inboxaccess.Scope{WorkspaceID: "ws_2", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed_a"})

	h.BroadcastInbox("ws_1", "managed_a", []byte("a"))
	assertSubscriptionMessage(t, workspaceOne, "a")
	assertSubscriptionMessage(t, managedA, "a")
	assertSubscriptionEmpty(t, managedB)
	assertSubscriptionEmpty(t, workspaceTwo)
	assertSubscriptionEmpty(t, workspaceTwoA)

	h.BroadcastInbox("ws_1", "managed_b", []byte("b"))
	assertSubscriptionMessage(t, workspaceOne, "b")
	assertSubscriptionEmpty(t, managedA)
	assertSubscriptionMessage(t, managedB, "b")
	assertSubscriptionEmpty(t, workspaceTwo)
	assertSubscriptionEmpty(t, workspaceTwoA)

	h.BroadcastInbox("ws_1", "", []byte("byo"))
	assertSubscriptionMessage(t, workspaceOne, "byo")
	assertSubscriptionEmpty(t, managedA)
	assertSubscriptionEmpty(t, managedB)
	assertSubscriptionEmpty(t, workspaceTwo)
	assertSubscriptionEmpty(t, workspaceTwoA)
}

func TestHubBroadcastInboxDoesNotCrossLegacyBoundary(t *testing.T) {
	h := NewHub()
	legacy := h.Subscribe("ws_1")
	defer h.Unsubscribe("ws_1", legacy)
	scoped := mustSubscribeScope(t, h, inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeWorkspace})

	h.Broadcast("ws_1", []byte("legacy"))
	assertSubscriptionMessage(t, legacy, "legacy")
	assertSubscriptionEmpty(t, scoped)

	h.BroadcastInbox("ws_1", "managed_a", []byte("inbox"))
	assertSubscriptionMessage(t, scoped, "inbox")
	assertSubscriptionEmpty(t, legacy)
}

func TestHubScopedAPIsRejectMalformedScopes(t *testing.T) {
	h := NewHub()
	invalid := []inboxaccess.Scope{
		{Mode: inboxaccess.ModeWorkspace},
		{WorkspaceID: " ", Mode: inboxaccess.ModeWorkspace},
		{WorkspaceID: "ws_1", Mode: inboxaccess.Mode("unknown")},
		{WorkspaceID: "ws_1", Mode: inboxaccess.ModeWorkspace, ExternalUserID: "managed_a"},
		{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser},
		{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: " "},
		{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: " managed_a"},
	}

	for _, scope := range invalid {
		if sub, ok := h.SubscribeScope(scope); ok || sub != nil {
			t.Fatalf("SubscribeScope(%#v) = %#v, %v; want nil, false", scope, sub, ok)
		}
		conn := &Conn{send: make(chan []byte, 1)}
		if h.RegisterScope(scope, conn) {
			t.Fatalf("RegisterScope(%#v) = true, want false", scope)
		}
	}

	if len(h.scopedSubs) != 0 || len(h.scopedConns) != 0 {
		t.Fatalf("invalid scopes registered: subs=%d conns=%d", len(h.scopedSubs), len(h.scopedConns))
	}
	h.BroadcastInbox("", "managed_a", []byte("invalid"))
}

func TestHubScopedRegisterAndUnregister(t *testing.T) {
	h := NewHub()
	scope := inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed_a"}
	conn := &Conn{send: make(chan []byte, 1)}
	if !h.RegisterScope(scope, conn) {
		t.Fatal("RegisterScope(valid) = false, want true")
	}

	h.BroadcastInbox("ws_1", "managed_a", []byte("hello"))
	select {
	case msg := <-conn.send:
		if string(msg) != "hello" {
			t.Fatalf("connection got %q, want hello", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for scoped connection broadcast")
	}

	if !h.UnregisterScope(scope, conn) {
		t.Fatal("UnregisterScope(registered) = false, want true")
	}
	if _, open := <-conn.send; open {
		t.Fatal("connection send channel remains open after UnregisterScope")
	}
	if h.UnregisterScope(scope, conn) {
		t.Fatal("second UnregisterScope = true, want false")
	}
}

func TestHubScopedUnsubscribeClosesAndStops(t *testing.T) {
	h := NewHub()
	scope := inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed_a"}
	sub := mustSubscribeScope(t, h, scope)
	if !h.UnsubscribeScope(scope, sub) {
		t.Fatal("UnsubscribeScope(registered) = false, want true")
	}
	if _, open := <-sub.C(); open {
		t.Fatal("subscription remains open after UnsubscribeScope")
	}
	if h.UnsubscribeScope(scope, sub) {
		t.Fatal("second UnsubscribeScope = true, want false")
	}
	h.BroadcastInbox("ws_1", "managed_a", []byte("after unsubscribe"))
}

func TestHubBroadcastInboxDropsSlowSubscriber(t *testing.T) {
	h := NewHub()
	sub := mustSubscribeScope(t, h, inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeWorkspace})
	defer h.UnsubscribeScope(inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeWorkspace}, sub)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			h.BroadcastInbox("ws_1", "managed_a", []byte("x"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("BroadcastInbox blocked on a slow subscriber")
	}
}

func TestHubBroadcastInboxConcurrentChurn(t *testing.T) {
	h := NewHub()
	scope := inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "managed_a"}
	done := make(chan struct{})
	broadcastStopped := make(chan struct{})

	go func() {
		defer close(broadcastStopped)
		for {
			select {
			case <-done:
				return
			default:
				h.BroadcastInbox("ws_1", "managed_a", []byte("x"))
			}
		}
	}()

	for i := 0; i < 200; i++ {
		sub := mustSubscribeScope(t, h, scope)
		select {
		case <-sub.C():
		default:
		}
		if !h.UnsubscribeScope(scope, sub) {
			t.Fatal("UnsubscribeScope failed during concurrent churn")
		}
	}
	close(done)
	<-broadcastStopped
}

func TestHubServeScopedConnRejectsMalformedScope(t *testing.T) {
	h := NewHub()
	h.ServeScopedConn(context.Background(), inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser}, nil)
	if len(h.scopedConns) != 0 {
		t.Fatalf("malformed ServeScopedConn registered %d scope keys", len(h.scopedConns))
	}
}

func mustSubscribeScope(t *testing.T, h *Hub, scope inboxaccess.Scope) *Subscription {
	t.Helper()
	sub, ok := h.SubscribeScope(scope)
	if !ok || sub == nil {
		t.Fatalf("SubscribeScope(%#v) = %#v, %v; want subscription, true", scope, sub, ok)
	}
	t.Cleanup(func() { h.UnsubscribeScope(scope, sub) })
	return sub
}

func assertSubscriptionMessage(t *testing.T, sub *Subscription, want string) {
	t.Helper()
	select {
	case msg, open := <-sub.C():
		if !open {
			t.Fatal("subscription closed before receiving message")
		}
		if string(msg) != want {
			t.Fatalf("got %q, want %q", msg, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %q", want)
	}
}

func assertSubscriptionEmpty(t *testing.T, sub *Subscription) {
	t.Helper()
	select {
	case msg, open := <-sub.C():
		if open {
			t.Fatalf("unexpected message %q", msg)
		}
		t.Fatal("subscription unexpectedly closed")
	default:
	}
}
