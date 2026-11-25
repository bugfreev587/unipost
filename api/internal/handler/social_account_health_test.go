package handler

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"testing"
	"time"
)

func TestAccountHealthLatestTerminalOutcome(t *testing.T) {
	now := time.Date(2026, 11, 23, 19, 40, 24, 0, time.UTC)
	stamp := func(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
	published := db.ListAccountHealthResultsRow{Status: "published", PublishedAt: stamp(now), OccurredAt: stamp(now)}
	failed := db.ListAccountHealthResultsRow{Status: "failed", ErrorMessage: pgtype.Text{String: "token expired", Valid: true}, OccurredAt: stamp(now.Add(-time.Hour))}
	for _, tc := range []struct {
		name      string
		rows      []db.ListAccountHealthResultsRow
		want      string
		wantError bool
	}{
		{"success clears older failure", []db.ListAccountHealthResultsRow{published, failed}, "ok", false},
		{"latest failure remains degraded", []db.ListAccountHealthResultsRow{failed, published}, "degraded", true},
		{"no terminal result", nil, "ok", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := accountHealthResponse{Status: "ok"}
			applyAccountHealthResults(&resp, tc.rows)
			if resp.Status != tc.want || (resp.LastError != nil) != tc.wantError {
				t.Fatalf("health = %+v", resp)
			}
			if len(tc.rows) > 0 && (resp.LastSuccessfulPostAt == nil || !resp.LastSuccessfulPostAt.Equal(now)) {
				t.Fatalf("missing successful timestamp: %+v", resp)
			}
			if tc.wantError && !resp.LastError.OccurredAt.Equal(failed.OccurredAt.Time) {
				t.Fatal("failure timestamp lost")
			}
		})
	}
}
