package ws

import (
	"testing"
	"time"
)

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

func TestHubBroadcast_DropsSlowSubscriber(t *testing.T) {
	h := NewHub()
	sub := h.Subscribe("ws_1")
	defer h.Unsubscribe("ws_1", sub)

	// Overflow the buffer; Broadcast must not block.
	for i := 0; i < 1000; i++ {
		h.Broadcast("ws_1", []byte("x"))
	}
}
