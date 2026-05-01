package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRoleLevel locks the numeric role ladder. Adding new roles must
// keep the existing levels stable so existing RequireRole(min) call
// sites don't silently allow more / less than they used to.
func TestRoleLevel(t *testing.T) {
	cases := map[string]int{
		"owner":   3,
		"admin":   2,
		"editor":  1,
		"":        0,
		"viewer":  0, // not a real role yet — falls through to 0
		"garbage": 0,
	}
	for role, want := range cases {
		if got := RoleLevel(role); got != want {
			t.Errorf("RoleLevel(%q) = %d, want %d", role, got, want)
		}
	}
}

// TestRoleLevel_StrictlyOrdered — owner > admin > editor > unknown.
// Catches a class of bug where two roles silently end up at the same
// level (which would let editors take admin actions, etc).
func TestRoleLevel_StrictlyOrdered(t *testing.T) {
	if !(RoleLevel(RoleOwner) > RoleLevel(RoleAdmin)) {
		t.Error("expected owner > admin")
	}
	if !(RoleLevel(RoleAdmin) > RoleLevel(RoleEditor)) {
		t.Error("expected admin > editor")
	}
	if !(RoleLevel(RoleEditor) > RoleLevel("")) {
		t.Error("expected editor > unknown")
	}
}

// TestRequireRole_AllowsAtOrAboveMin — every role at or above the
// minimum passes. The middleware must call next.ServeHTTP exactly once.
func TestRequireRole_AllowsAtOrAboveMin(t *testing.T) {
	cases := []struct {
		role string
		min  string
	}{
		{RoleOwner, RoleEditor},
		{RoleOwner, RoleAdmin},
		{RoleOwner, RoleOwner},
		{RoleAdmin, RoleEditor},
		{RoleAdmin, RoleAdmin},
		{RoleEditor, RoleEditor},
	}
	for _, tc := range cases {
		t.Run(tc.role+"_>="+tc.min, func(t *testing.T) {
			calls := 0
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ })
			h := RequireRole(tc.min)(next)
			req := httptest.NewRequest(http.MethodGet, "/", nil).
				WithContext(SetRole(context.Background(), tc.role))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if calls != 1 {
				t.Errorf("expected next handler to run once, got %d", calls)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rr.Code)
			}
		})
	}
}

// TestRequireRole_DeniesBelowMin — every role below the minimum gets
// 403 INSUFFICIENT_ROLE without invoking the wrapped handler.
func TestRequireRole_DeniesBelowMin(t *testing.T) {
	cases := []struct {
		role string
		min  string
	}{
		{RoleEditor, RoleAdmin},
		{RoleEditor, RoleOwner},
		{RoleAdmin, RoleOwner},
		{"", RoleEditor},
		{"garbage", RoleEditor},
	}
	for _, tc := range cases {
		t.Run(tc.role+"_<"+tc.min, func(t *testing.T) {
			calls := 0
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ })
			h := RequireRole(tc.min)(next)
			req := httptest.NewRequest(http.MethodGet, "/", nil).
				WithContext(SetRole(context.Background(), tc.role))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if calls != 0 {
				t.Errorf("expected next handler NOT to run, got %d calls", calls)
			}
			if rr.Code != http.StatusForbidden {
				t.Errorf("expected 403, got %d", rr.Code)
			}
		})
	}
}

// TestRequireRole_MissingRoleIs403 — a request that reached the
// gate without a role context value (auth middleware misconfigured)
// is treated as 403, not 500. Same as "below minimum" semantically.
func TestRequireRole_MissingRoleIs403(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ })
	h := RequireRole(RoleEditor)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if calls != 0 {
		t.Error("expected next handler NOT to run when role is missing")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
