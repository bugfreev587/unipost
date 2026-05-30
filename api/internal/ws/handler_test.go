package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type staticInboxPlanGate struct {
	allow bool
}

func (s staticInboxPlanGate) PlanAllowsInbox(context.Context, string) bool {
	return s.allow
}

func TestInboxWebSocketPlanGateBlocksUnavailablePlans(t *testing.T) {
	handler := NewHandler(NewHub(), nil).WithInboxPlanGate(staticInboxPlanGate{allow: false})
	req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws", nil)
	rr := httptest.NewRecorder()

	if handler.ensureInboxPlanAllowed(rr, req, "workspace_123") {
		t.Fatal("expected inbox websocket plan gate to block unavailable plan")
	}
	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status 402, got %d", rr.Code)
	}
}

func TestInboxWebSocketPlanGateAllowsUnlockedPlans(t *testing.T) {
	handler := NewHandler(NewHub(), nil).WithInboxPlanGate(staticInboxPlanGate{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws", nil)
	rr := httptest.NewRecorder()

	if !handler.ensureInboxPlanAllowed(rr, req, "workspace_123") {
		t.Fatal("expected inbox websocket plan gate to allow unlocked plan")
	}
}
