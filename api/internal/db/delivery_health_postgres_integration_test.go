//go:build integration

package db

import (
	"context"
	"testing"
	"time"
)

func TestDeliveryHealthPostgresCancellationOrderingAndQueueAge(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
 CREATE TABLE social_posts (id text PRIMARY KEY, created_at timestamptz NOT NULL);
 CREATE TABLE social_post_results (id text PRIMARY KEY, post_id text, social_account_id text, status text, error_message text, error_code text, published_at timestamptz);
 CREATE TABLE post_delivery_jobs (id text PRIMARY KEY, social_post_result_id text, workspace_id text, kind text, state text, created_at timestamptz, next_run_at timestamptz, finished_at timestamptz, dismissed_at timestamptz);
 INSERT INTO social_posts VALUES ('post',now()-interval '1 day');
 INSERT INTO social_post_results VALUES
 ('success','post','account','published',NULL,NULL,now()-interval '5 minutes'),
 ('cancel','post','account','failed','Delivery cancelled before provider dispatch.',NULL,NULL),
 ('old-failure','post','account','failed','token expired','token_expired',NULL),
 ('other','post','other-account','failed','other failure',NULL,NULL);
 INSERT INTO post_delivery_jobs VALUES
 ('old-job','old-failure','workspace','dispatch','dead',now()-interval '1 hour',NULL,now()-interval '1 hour',NULL),
 ('cancel-job','cancel','workspace','dispatch','cancelled',now(),NULL,now(),NULL),
 ('pending',NULL,'workspace','dispatch','pending',now()-interval '38 minutes',NULL,NULL,NULL),
 ('future',NULL,'workspace','retry','pending',now()-interval '1 day',now()+interval '1 hour',NULL,NULL),
 ('dismissed',NULL,'workspace','dispatch','pending',now()-interval '2 days',NULL,NULL,now()),
 ('other-pending',NULL,'other-workspace','dispatch','pending',now()-interval '3 days',NULL,NULL,NULL);
 `)
	if err != nil {
		t.Fatal(err)
	}
	q := New(pool)
	rows, err := q.ListAccountHealthResults(ctx, ListAccountHealthResultsParams{SocialAccountID: "account", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Status != "published" || rows[1].ErrorMessage.String != "token expired" {
		t.Fatalf("cancel leaked or wrong terminal ordering: %+v", rows)
	}
	if !rows[1].OccurredAt.Valid || time.Since(rows[1].OccurredAt.Time) < 59*time.Minute {
		t.Fatal("failed delivery completion timestamp missing")
	}
	_, err = pool.Exec(ctx, "UPDATE post_delivery_jobs SET finished_at=now() WHERE id='old-job'")
	if err != nil {
		t.Fatal(err)
	}
	rows, err = q.ListAccountHealthResults(ctx, ListAccountHealthResultsParams{SocialAccountID: "account", Limit: 10})
	if err != nil || len(rows) != 2 || rows[0].Status != "failed" {
		t.Fatalf("newer real failure must win: %+v %v", rows, err)
	}
	summary, err := q.GetPostDeliveryJobsSummaryByWorkspace(ctx, "workspace")
	if err != nil {
		t.Fatal(err)
	}
	if summary.PendingCount != 2 || !summary.OldestPendingAt.Valid || summary.OldestPendingAgeMs < 2279000 || summary.OldestPendingAgeMs > 2285000 {
		t.Fatalf("wrong scoped ready queue age: %+v", summary)
	}
	empty, err := q.GetPostDeliveryJobsSummaryByWorkspace(ctx, "empty")
	if err != nil || empty.OldestPendingAt.Valid || empty.OldestPendingAgeMs != 0 {
		t.Fatalf("empty queue = %+v %v", empty, err)
	}
	// The latest success must survive the history limit after many newer failures.
	_, err = pool.Exec(ctx, `INSERT INTO social_post_results
      SELECT 'failure-' || n, 'post', 'account', 'failed', 'new failure', NULL, NULL
      FROM generate_series(1, 12) n;
      INSERT INTO post_delivery_jobs
      SELECT 'failure-job-' || n, 'failure-' || n, 'workspace', 'dispatch', 'dead', now(), NULL, now(), NULL
      FROM generate_series(1, 12) n;`)
	if err != nil {
		t.Fatal(err)
	}
	limited, err := q.ListAccountHealthResults(ctx, ListAccountHealthResultsParams{SocialAccountID: "account", Limit: 10})
	if err != nil || len(limited) != 10 || !limited[0].LastSuccessfulPostAt.Valid {
		t.Fatalf("latest success lost behind failed history: %+v %v", limited, err)
	}
	// A future retry is pending but not overdue.
	_, err = pool.Exec(ctx, "DELETE FROM post_delivery_jobs WHERE id='pending'")
	if err != nil {
		t.Fatal(err)
	}
	future, err := q.GetPostDeliveryJobsSummaryByWorkspace(ctx, "workspace")
	if err != nil || future.OldestPendingAt.Valid || future.OldestPendingAgeMs != 0 {
		t.Fatalf("future retry counted as overdue: %+v %v", future, err)
	}
}
