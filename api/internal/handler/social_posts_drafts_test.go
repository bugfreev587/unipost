package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestRollbackDraftAndWriteFreePlanQuotaErrorReportsRollbackFailure(t *testing.T) {
	h := &SocialPostHandler{
		queries: db.New(&draftRollbackDB{execErr: errors.New("db unavailable")}),
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post_123/publish", nil)
	rr := httptest.NewRecorder()

	h.rollbackDraftAndWriteFreePlanQuotaError(rr, req, "post_123", quota.QuotaStatus{Usage: 100, Limit: 100}, 1)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "PLAN_POST_QUOTA_EXCEEDED") {
		t.Fatalf("should not return quota error when rollback failed: %s", rr.Body.String())
	}
}

func TestRollbackDraftAndWriteFreePlanQuotaErrorReturnsQuotaAfterRollback(t *testing.T) {
	h := &SocialPostHandler{
		queries: db.New(&draftRollbackDB{}),
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post_123/publish", nil)
	rr := httptest.NewRecorder()

	h.rollbackDraftAndWriteFreePlanQuotaError(rr, req, "post_123", quota.QuotaStatus{Usage: 100, Limit: 100}, 1)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rr.Code)
	}
	if got := rr.Header().Get("X-UniPost-Warning"); got != "over_limit" {
		t.Fatalf("X-UniPost-Warning = %q, want over_limit", got)
	}
	if !strings.Contains(rr.Body.String(), "PLAN_POST_QUOTA_EXCEEDED") {
		t.Fatalf("expected quota error body, got: %s", rr.Body.String())
	}
}

type draftRollbackDB struct {
	execErr error
}

func (d *draftRollbackDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, d.execErr
}

func (d *draftRollbackDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (d *draftRollbackDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return draftRollbackRow{}
}

type draftRollbackRow struct{}

func (draftRollbackRow) Scan(...any) error {
	return errors.New("unexpected query row")
}
