package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestWorkspaceUpdateCustomPlatformSlotNormalizesAndReturnsSlot(t *testing.T) {
	store := &workspaceTestDB{}
	h := NewWorkspaceHandler(db.New(store))
	req := httptest.NewRequest(http.MethodPatch, "/v1/workspace", strings.NewReader(`{"custom_platform_slot":" TikTok "}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.customPlatformSlot != "tiktok" {
		t.Fatalf("customPlatformSlot = %q, want tiktok", store.customPlatformSlot)
	}
	if !strings.Contains(rec.Body.String(), `"custom_platform_slot":"tiktok"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestWorkspaceUpdateCustomPlatformSlotRejectsUnknownPlatform(t *testing.T) {
	store := &workspaceTestDB{}
	h := NewWorkspaceHandler(db.New(store))
	req := httptest.NewRequest(http.MethodPatch, "/v1/workspace", strings.NewReader(`{"custom_platform_slot":"mastodon"}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.customPlatformSlot != "" {
		t.Fatalf("customPlatformSlot = %q, want empty", store.customPlatformSlot)
	}
}

type workspaceTestDB struct {
	customPlatformSlot string
}

func (f *workspaceTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *workspaceTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}

func (f *workspaceTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: UpdateWorkspaceCustomPlatformSlot"):
		slot, _ := args[1].(string)
		f.customPlatformSlot = slot
		return f.workspaceRow()
	case strings.Contains(query, "-- name: GetWorkspace"):
		return f.workspaceRow()
	default:
		return scanRow{err: pgx.ErrNoRows}
	}
}

func (f *workspaceTestDB) workspaceRow() scanRow {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	customPlatformSlot := pgtype.Text{}
	if f.customPlatformSlot != "" {
		customPlatformSlot = pgtype.Text{String: f.customPlatformSlot, Valid: true}
	}
	return scanRow{values: []any{
		"ws_1",
		"user_1",
		"Workspace",
		pgtype.Int4{},
		now,
		now,
		[]string{"publishing"},
		customPlatformSlot,
	}}
}
