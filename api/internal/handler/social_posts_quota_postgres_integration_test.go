//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/ratelimit"
)

func TestScheduledQuotaSnapshotPostgresCountsOnlyAdmissionAllowedTargets(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := context.Background()
	baseline := time.Now().UTC()
	scheduledAt := time.Date(baseline.Year(), baseline.Month()+1, 15, 12, 0, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{
		{AccountID: "account_tiktok", Caption: "restricted at admission"},
		{AccountID: "account_instagram", Caption: "allowed at admission"},
	}
	accountMap := map[string]platform.ValidateAccount{
		"account_tiktok":    {Platform: "tiktok"},
		"account_instagram": {Platform: "instagram"},
	}
	period := quota.PeriodForTime(scheduledAt)
	if _, err := pool.Exec(ctx, `
		INSERT INTO plans(id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions(workspace_id,plan_id,status) VALUES ('workspace_snapshot','free','active');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage(workspace_id,period,post_count) VALUES ('workspace_snapshot',$1,0)`, period); err != nil {
		t.Fatal(err)
	}
	evaluator := &fakePostRestrictionEvaluator{decisions: map[string]publishingrestrictions.Decision{
		"tiktok": {
			Restricted: true,
			Platform:   "tiktok",
			PlanID:     "free",
			CycleID:    "cycle_admission",
			Code:       publishingrestrictions.NormalizedCode,
		},
	}}
	queries := db.New(pool)
	h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).SetPublishingRestrictions(evaluator)
	blocked, err := h.evaluatePublishingRestrictions(ctx, "workspace_snapshot", posts, accountMap)
	if err != nil {
		t.Fatalf("evaluate admission policy: %v", err)
	}
	allowedTargets := allowedPublishingTargets(posts, blocked)
	allowedUnits := countPublishQuotaUnits(allowedTargets, accountMap)
	if allowedUnits != 1 {
		t.Fatalf("admission allowed units = %d, want 1", allowedUnits)
	}

	body := fmt.Sprintf(`{
		"scheduled_at":%q,
		"scheduled_quota_units":2,
		"scheduled_quota_account_ids":["account_tiktok"],
		"scheduled_quota_snapshot":{"version":1,"targets":[]},
		"platform_posts":[
			{"account_id":"account_tiktok","caption":"restricted at admission","media_urls":["https://cdn.example/image.jpg"]},
			{"account_id":"account_instagram","caption":"allowed at admission","media_urls":["https://cdn.example/image.jpg"]}
		]
	}`, scheduledAt.Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace_snapshot"))
	req = req.WithContext(withoutPostMediaRetentionSync(req.Context()))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create scheduled status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}

	var postID string
	var metadata []byte
	if err := pool.QueryRow(ctx, `
		SELECT id, metadata
		FROM social_posts
		WHERE workspace_id = 'workspace_snapshot'
	`).Scan(&postID, &metadata); err != nil {
		t.Fatalf("load persisted scheduled post: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(metadata, &persisted); err != nil {
		t.Fatalf("decode persisted metadata: %v", err)
	}
	if got := persisted["scheduled_quota_units"]; got != float64(1) {
		t.Fatalf("persisted scheduled_quota_units = %#v, want 1", got)
	}
	if got, ok := persisted["scheduled_quota_account_ids"].([]any); !ok || len(got) != 1 || got[0] != "account_instagram" {
		t.Fatalf("persisted scheduled_quota_account_ids = %#v, want [account_instagram]", persisted["scheduled_quota_account_ids"])
	}
	reserved, ok := scheduledQuotaReservedIndexesFromMetadata(metadata)
	if !ok || !slices.Equal(reserved, []bool{false, true}) {
		t.Fatalf("persisted scheduled target snapshot = %v/%t, want [false true]/true", reserved, ok)
	}

	countScheduled := func(workspaceID string) int32 {
		t.Helper()
		got, err := queries.CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{
			WorkspaceID: workspaceID,
			Period:      period,
		})
		if err != nil {
			t.Fatalf("count scheduled quota for %s: %v", workspaceID, err)
		}
		return got
	}
	if got := countScheduled("workspace_snapshot"); got != 1 {
		t.Fatalf("scheduled reservation before policy lift = %d, want 1", got)
	}

	// Policy is deliberately dynamic at execution time. Lifting it changes
	// the execution target count, but must not rewrite the already-committed
	// claim-before reservation snapshot.
	evaluator.decisions = map[string]publishingrestrictions.Decision{}
	blocked, err = h.evaluatePublishingRestrictions(ctx, "workspace_snapshot", posts, accountMap)
	if err != nil {
		t.Fatalf("evaluate lifted execution policy: %v", err)
	}
	if executionUnits := countPublishQuotaUnits(allowedPublishingTargets(posts, blocked), accountMap); executionUnits != 2 {
		t.Fatalf("execution units after policy lift = %d, want 2", executionUnits)
	}
	if got := countScheduled("workspace_snapshot"); got != 1 {
		t.Fatalf("scheduled reservation after policy lift = %d, want immutable snapshot 1", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE social_posts SET status = 'quota_hold' WHERE id = $1`, postID); err != nil {
		t.Fatalf("move scheduled post to quota_hold: %v", err)
	}
	holdUnits, err := queries.CountQuotaHoldUnitsByWorkspaceAndPeriod(ctx, db.CountQuotaHoldUnitsByWorkspaceAndPeriodParams{
		WorkspaceID: "workspace_snapshot",
		Period:      period,
	})
	if err != nil {
		t.Fatalf("count quota_hold units: %v", err)
	}
	if holdUnits != 1 || countScheduled("workspace_snapshot") != 1 {
		t.Fatalf("quota_hold snapshot counts = hold:%d scheduled:%d, want 1/1", holdUnits, countScheduled("workspace_snapshot"))
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, workspace_id, status, scheduled_at, created_at, metadata)
		VALUES
			('legacy_post', 'workspace_legacy', 'scheduled', $1::timestamptz, $1::timestamptz - INTERVAL '1 day',
			 '{"schema_version":2,"platform_posts":[{"account_id":"account_tiktok"},{"account_id":"account_instagram"}]}'::jsonb),
			('invalid_negative', 'workspace_invalid', 'scheduled', $1::timestamptz, $1::timestamptz - INTERVAL '1 day',
			 '{"schema_version":2,"scheduled_quota_units":-1,"platform_posts":[{"account_id":"account_tiktok"},{"account_id":"account_instagram"}]}'::jsonb),
			('invalid_large', 'workspace_invalid', 'scheduled', $1::timestamptz, $1::timestamptz - INTERVAL '1 day',
			 '{"schema_version":2,"scheduled_quota_units":999999999999999999999,"platform_posts":[{"account_id":"account_tiktok"},{"account_id":"account_instagram"}]}'::jsonb)
	`, scheduledAt); err != nil {
		t.Fatalf("insert legacy/invalid snapshots: %v", err)
	}
	if got := countScheduled("workspace_legacy"); got != 2 {
		t.Fatalf("legacy reservation = %d, want legacy target count 2", got)
	}
	if got := countScheduled("workspace_invalid"); got != 4 {
		t.Fatalf("invalid snapshot reservation = %d, want fail-safe legacy count 4", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE social_posts SET status = 'publishing' WHERE id = $1`, postID); err != nil {
		t.Fatalf("move snapshot post to publishing: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status)
		VALUES
			('result_policy_failed', $1, 'account_tiktok', 'failed'),
			('result_allowed_pending', $1, 'account_instagram', 'pending');
	`, postID); err != nil {
		t.Fatalf("seed publishing result precedence: %v", err)
	}
	if got := countScheduled("workspace_snapshot"); got != 1 {
		t.Fatalf("publishing reservation = %d, want pending result count 1", got)
	}
}

func TestScheduledQuotaTargetSnapshotRejectsMissingMalformedAndForgedBindings(t *testing.T) {
	tests := []struct {
		name     string
		metadata string
		want     []bool
		wantOK   bool
	}{
		{
			name: "complete mixed binding",
			metadata: `{"schema_version":2,"scheduled_quota_units":1,"scheduled_quota_account_ids":["facebook"],
				"scheduled_quota_snapshot":{"version":1,"targets":[
					{"post_input_index":0,"account_id":"tiktok","reserved":false},
					{"post_input_index":1,"account_id":"facebook","reserved":true}]},
				"platform_posts":[{"account_id":"tiktok"},{"account_id":"facebook"}]}`,
			want:   []bool{false, true},
			wantOK: true,
		},
		{
			name: "missing full snapshot",
			metadata: `{"schema_version":2,"scheduled_quota_units":1,"scheduled_quota_account_ids":["facebook"],
				"platform_posts":[{"account_id":"tiktok"},{"account_id":"facebook"}]}`,
		},
		{
			name: "missing target binding",
			metadata: `{"schema_version":2,"scheduled_quota_units":1,"scheduled_quota_account_ids":["facebook"],
				"scheduled_quota_snapshot":{"version":1,"targets":[{"post_input_index":1,"account_id":"facebook","reserved":true}]},
				"platform_posts":[{"account_id":"tiktok"},{"account_id":"facebook"}]}`,
		},
		{
			name: "reserved account list steals facebook unit for tiktok",
			metadata: `{"schema_version":2,"scheduled_quota_units":1,"scheduled_quota_account_ids":["tiktok"],
				"scheduled_quota_snapshot":{"version":1,"targets":[
					{"post_input_index":0,"account_id":"tiktok","reserved":false},
					{"post_input_index":1,"account_id":"facebook","reserved":true}]},
				"platform_posts":[{"account_id":"tiktok"},{"account_id":"facebook"}]}`,
		},
		{
			name: "snapshot bit steals facebook unit for tiktok",
			metadata: `{"schema_version":2,"scheduled_quota_units":1,"scheduled_quota_account_ids":["facebook"],
				"scheduled_quota_snapshot":{"version":1,"targets":[
					{"post_input_index":0,"account_id":"tiktok","reserved":true},
					{"post_input_index":1,"account_id":"facebook","reserved":false}]},
				"platform_posts":[{"account_id":"tiktok"},{"account_id":"facebook"}]}`,
		},
		{
			name: "repeated account cannot reserve only one thread entry",
			metadata: `{"schema_version":2,"scheduled_quota_units":1,"scheduled_quota_account_ids":["threaded"],
				"scheduled_quota_snapshot":{"version":1,"targets":[
					{"post_input_index":0,"account_id":"threaded","reserved":true},
					{"post_input_index":1,"account_id":"threaded","reserved":false}]},
				"platform_posts":[{"account_id":"threaded","thread_position":1},{"account_id":"threaded","thread_position":2}]}`,
		},
		{
			name: "repeated account complete reservation",
			metadata: `{"schema_version":2,"scheduled_quota_units":2,"scheduled_quota_account_ids":["threaded","threaded"],
				"scheduled_quota_snapshot":{"version":1,"targets":[
					{"post_input_index":0,"account_id":"threaded","reserved":true},
					{"post_input_index":1,"account_id":"threaded","reserved":true}]},
				"platform_posts":[{"account_id":"threaded","thread_position":1},{"account_id":"threaded","thread_position":2}]}`,
			want:   []bool{true, true},
			wantOK: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := scheduledQuotaReservedIndexesFromMetadata([]byte(test.metadata))
			if ok != test.wantOK || !slices.Equal(got, test.want) {
				t.Fatalf("snapshot=%v/%t, want %v/%t", got, ok, test.want, test.wantOK)
			}
		})
	}
}

func TestDuePostFailsClosedForMissingMalformedAndForgedQuotaSnapshots(t *testing.T) {
	tests := []struct {
		name     string
		repeated bool
		mutate   func(map[string]any)
	}{
		{name: "missing", mutate: func(object map[string]any) { delete(object, scheduledQuotaSnapshotMetadataKey) }},
		{name: "malformed", mutate: func(object map[string]any) {
			snapshot := object[scheduledQuotaSnapshotMetadataKey].(map[string]any)
			snapshot["targets"] = snapshot["targets"].([]any)[:1]
		}},
		{name: "forged reserved account", mutate: func(object map[string]any) {
			object[scheduledQuotaAccountIDsMetadataKey] = []any{"account_invalid_tiktok"}
		}},
		{name: "repeated thread undercount", repeated: true, mutate: func(object map[string]any) {
			object[scheduledQuotaUnitsMetadataKey] = float64(1)
			object[scheduledQuotaAccountIDsMetadataKey] = []any{"account_invalid_threaded"}
			snapshot := object[scheduledQuotaSnapshotMetadataKey].(map[string]any)
			targets := snapshot["targets"].([]any)
			targets[1].(map[string]any)["reserved"] = false
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pool := openRestrictedDeliveryIntegrationPool(t)
			setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
			ctx := withoutPostMediaRetentionSync(context.Background())
			now := time.Now().UTC()
			period := quota.PeriodForTime(now)
			posts := []platform.PlatformPostInput{
				{AccountID: "account_invalid_tiktok", Caption: "newly allowed"},
				{AccountID: "account_invalid_facebook", Caption: "original reservation"},
			}
			quotaAccountIDs := []string{"account_invalid_facebook"}
			if test.repeated {
				posts = []platform.PlatformPostInput{
					{AccountID: "account_invalid_threaded", Caption: "thread one", ThreadPosition: 1},
					{AccountID: "account_invalid_threaded", Caption: "thread two", ThreadPosition: 2},
				}
				quotaAccountIDs = []string{"account_invalid_threaded", "account_invalid_threaded"}
			}
			metadata, err := encodeScheduledPostMetadataWithQuotaAccounts(posts, len(quotaAccountIDs), quotaAccountIDs)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]any
			if err := json.Unmarshal(metadata, &object); err != nil {
				t.Fatal(err)
			}
			test.mutate(object)
			metadata, err = json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := scheduledQuotaReservedIndexesFromMetadata(metadata); ok {
				t.Fatal("mutated snapshot unexpectedly validated")
			}

			if _, err := pool.Exec(ctx, `
				INSERT INTO profiles VALUES ('profile_invalid','workspace_invalid_due');
				INSERT INTO social_accounts(id,profile_id,platform) VALUES
					('account_invalid_tiktok','profile_invalid','tiktok'),
					('account_invalid_facebook','profile_invalid','facebook'),
					('account_invalid_threaded','profile_invalid','linkedin');
				INSERT INTO plans(id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
				INSERT INTO subscriptions(workspace_id,plan_id,status) VALUES ('workspace_invalid_due','free','active');
			`); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO usage(workspace_id,period,post_count) VALUES ('workspace_invalid_due',$1,99)`, period); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,workspace_id,status,scheduled_at,created_at,metadata) VALUES ('post_invalid_due','workspace_invalid_due','scheduled',$1,$1,$2)`, now, metadata); err != nil {
				t.Fatal(err)
			}
			queries := db.New(pool)
			handler := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
				SetPublishingRestrictions(&fakePostRestrictionEvaluator{})
			if err := handler.ClaimAndEnqueueScheduledPost(ctx, "post_invalid_due"); err != nil {
				t.Fatal(err)
			}
			var status string
			var results, failed, jobs int
			if err := pool.QueryRow(ctx, `
				SELECT status,
				       (SELECT COUNT(*)::int FROM social_post_results WHERE post_id='post_invalid_due'),
				       (SELECT COUNT(*)::int FROM social_post_results WHERE post_id='post_invalid_due' AND status='failed'),
				       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id='post_invalid_due')
				FROM social_posts WHERE id='post_invalid_due'
			`).Scan(&status, &results, &failed, &jobs); err != nil {
				t.Fatal(err)
			}
			if status != "failed" || results != 2 || failed != 2 || jobs != 0 {
				t.Fatalf("fail-closed outcome=%q/%d/%d/%d, want failed/2/2/0", status, results, failed, jobs)
			}
		})
	}
}

func TestDueRepeatedAccountSnapshotPreservesEveryThreadIndex(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := withoutPostMediaRetentionSync(context.Background())
	now := time.Now().UTC()
	period := quota.PeriodForTime(now)
	posts := []platform.PlatformPostInput{
		{AccountID: "account_threaded_due", Caption: "thread one", ThreadPosition: 1},
		{AccountID: "account_threaded_due", Caption: "thread two", ThreadPosition: 2},
	}
	metadata, err := encodeScheduledPostMetadataWithQuotaAccounts(posts, 2, []string{"account_threaded_due", "account_threaded_due"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles VALUES ('profile_threaded_due','workspace_threaded_due');
		INSERT INTO social_accounts(id,profile_id,platform) VALUES ('account_threaded_due','profile_threaded_due','linkedin');
		INSERT INTO plans(id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions(workspace_id,plan_id,status) VALUES ('workspace_threaded_due','free','active');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage(workspace_id,period,post_count) VALUES ('workspace_threaded_due',$1,98)`, period); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,workspace_id,status,scheduled_at,created_at,metadata) VALUES ('post_threaded_due','workspace_threaded_due','scheduled',$1,$1,$2)`, now, metadata); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	if err := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
		ClaimAndEnqueueScheduledPost(ctx, "post_threaded_due"); err != nil {
		t.Fatal(err)
	}
	var status string
	var results, jobs int
	var indexes []int32
	if err := pool.QueryRow(ctx, `
		SELECT status,
		       (SELECT COUNT(*)::int FROM social_post_results WHERE post_id='post_threaded_due'),
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id='post_threaded_due'),
		       (SELECT ARRAY_AGG(post_input_index ORDER BY post_input_index) FROM post_delivery_jobs WHERE post_id='post_threaded_due')
		FROM social_posts WHERE id='post_threaded_due'
	`).Scan(&status, &results, &jobs, &indexes); err != nil {
		t.Fatal(err)
	}
	if status != "publishing" || results != 2 || jobs != 2 || !slices.Equal(indexes, []int32{0, 1}) {
		t.Fatalf("threaded due outcome=%q/%d/%d/%v, want publishing/2/2/[0 1]", status, results, jobs, indexes)
	}
}

func TestConcurrentDueMixedPostKeepsAdmissionReservedTargetWhenRestrictionLiftExceedsQuota(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(withoutPostMediaRetentionSync(context.Background()), 10*time.Second)
	defer cancel()

	now := time.Now().UTC()
	period := quota.PeriodForTime(now)
	metadata, err := encodeScheduledPostMetadataWithQuotaAccounts([]platform.PlatformPostInput{
		{AccountID: "account_mixed_tiktok", Caption: "newly allowed at execution"},
		{AccountID: "account_mixed_facebook", Caption: "reserved at admission"},
	}, 1, []string{"account_mixed_facebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, workspace_id) VALUES ('profile_mixed_growth', 'workspace_mixed_growth');
		INSERT INTO social_accounts (id, profile_id, platform) VALUES
			('account_mixed_tiktok', 'profile_mixed_growth', 'tiktok'),
			('account_mixed_facebook', 'profile_mixed_growth', 'facebook');
		INSERT INTO plans (id, name, price_cents, post_limit) VALUES ('free', 'Free', 0, 100);
		INSERT INTO subscriptions (workspace_id, plan_id, status) VALUES ('workspace_mixed_growth', 'free', 'active');
	`); err != nil {
		t.Fatalf("seed mixed due quota identities: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id, period, post_count) VALUES ('workspace_mixed_growth', $1, 99)`, period); err != nil {
		t.Fatalf("seed mixed due quota usage: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, workspace_id, status, scheduled_at, created_at, metadata)
		VALUES ('post_mixed_growth', 'workspace_mixed_growth', 'scheduled', $1::timestamptz, $1::timestamptz - INTERVAL '1 hour', $2::jsonb)
	`, now, metadata); err != nil {
		t.Fatalf("seed mixed due quota fixture: %v", err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			queries := db.New(pool)
			handler := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
				SetPublishingRestrictions(&fakePostRestrictionEvaluator{})
			errs <- handler.ClaimAndEnqueueScheduledPost(ctx, "post_mixed_growth")
		}()
	}
	close(start)

	var successes, lostClaims int
	for range 2 {
		err := <-errs
		switch {
		case err == nil:
			successes++
		case errors.Is(err, pgx.ErrNoRows):
			lostClaims++
		default:
			t.Fatalf("concurrent mixed claim: %v", err)
		}
	}
	if successes != 1 || lostClaims != 1 {
		t.Fatalf("concurrent claims = success:%d lost:%d, want 1/1", successes, lostClaims)
	}

	var parentStatus, facebookStatus, tiktokStatus string
	var tiktokError pgtype.Text
	var facebookJobs, facebookPostInputIndex, tiktokJobs, quotaFailures, usage int
	if err := pool.QueryRow(ctx, `
		SELECT sp.status,
		       facebook.status,
		       tiktok.status,
		       tiktok.error_message,
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=sp.id AND social_account_id='account_mixed_facebook'),
		       (SELECT post_input_index FROM post_delivery_jobs WHERE post_id=sp.id AND social_account_id='account_mixed_facebook'),
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=sp.id AND social_account_id='account_mixed_tiktok'),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id=sp.id AND social_account_id='account_mixed_tiktok' AND failure_stage='quota'),
		       (SELECT post_count FROM usage WHERE workspace_id=sp.workspace_id AND period=$1)
		FROM social_posts sp
		JOIN social_post_results facebook ON facebook.post_id=sp.id AND facebook.social_account_id='account_mixed_facebook'
		JOIN social_post_results tiktok ON tiktok.post_id=sp.id AND tiktok.social_account_id='account_mixed_tiktok'
		WHERE sp.id='post_mixed_growth'
	`, period).Scan(
		&parentStatus, &facebookStatus, &tiktokStatus, &tiktokError,
		&facebookJobs, &facebookPostInputIndex, &tiktokJobs, &quotaFailures, &usage,
	); err != nil {
		t.Fatalf("load mixed due outcome: %v", err)
	}
	if parentStatus != "publishing" || facebookStatus != "pending" || facebookJobs != 1 || facebookPostInputIndex != 1 {
		t.Fatalf("reserved Facebook outcome = parent:%q result:%q jobs:%d index:%d, want publishing/pending/1/1", parentStatus, facebookStatus, facebookJobs, facebookPostInputIndex)
	}
	if tiktokStatus != "failed" || tiktokJobs != 0 || quotaFailures != 1 || !strings.Contains(tiktokError.String, "quota exceeded") {
		t.Fatalf("new TikTok outcome = result:%q jobs:%d failures:%d error:%q, want quota-failed/0/1", tiktokStatus, tiktokJobs, quotaFailures, tiktokError.String)
	}
	if usage != 99 {
		t.Fatalf("usage after enqueue = %d, want 99 until Facebook publishes", usage)
	}
	reservedUnits, err := db.New(pool).CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{
		WorkspaceID: "workspace_mixed_growth",
		Period:      period,
	})
	if err != nil || reservedUnits != 1 {
		t.Fatalf("mixed execution reservation = %d, err=%v, want 1", reservedUnits, err)
	}

	var facebookResultID string
	if err := pool.QueryRow(ctx, `SELECT id FROM social_post_results WHERE post_id='post_mixed_growth' AND social_account_id='account_mixed_facebook'`).Scan(&facebookResultID); err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Now().UTC()
	if _, err := db.New(pool).UpdateSocialPostResultAfterRetryAndIncrementUsage(ctx, db.UpdateSocialPostResultAfterRetryAndIncrementUsageParams{
		ID:          facebookResultID,
		Status:      "published",
		ExternalID:  pgtype.Text{String: "facebook_external", Valid: true},
		PublishedAt: pgtype.Timestamptz{Time: publishedAt, Valid: true},
		WorkspaceID: "workspace_mixed_growth",
		Period:      period,
		PostCount:   1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE post_delivery_jobs SET state='succeeded', finished_at=$1 WHERE post_id='post_mixed_growth' AND social_account_id='account_mixed_facebook'`, publishedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE TABLE terminal_post_event_outbox (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			sequence_number BIGSERIAL NOT NULL UNIQUE,
			post_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			parent_status TEXT NOT NULL,
			parent_version TEXT NOT NULL,
			event_type TEXT NOT NULL DEFAULT '',
			payload JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(post_id,parent_version)
		);
		CREATE TABLE terminal_post_event_deliveries (
			event_id TEXT NOT NULL,
			sink TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			lease_expires_at TIMESTAMPTZ,
			last_error TEXT,
			enqueued_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY(event_id,sink)
		);
	`); err != nil {
		t.Fatal(err)
	}
	post, err := db.New(pool).GetSocialPostByID(ctx, "post_mixed_growth")
	if err != nil {
		t.Fatal(err)
	}
	results, err := db.New(pool).ListSocialPostResultsByPost(ctx, "post_mixed_growth")
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSocialPostHandler(db.New(pool), nil, quota.NewChecker(db.New(pool)), nil, nil, nil, nil).
		refreshParentPostStatusContext(ctx, post, results); err != nil {
		t.Fatal(err)
	}
	var finalParent, finalFacebook, finalTikTok string
	var finalUsage, finalEvents int
	if err := pool.QueryRow(ctx, `
		SELECT post.status, facebook.status, tiktok.status,
		       (SELECT post_count FROM usage WHERE workspace_id=post.workspace_id AND period=$1),
		       (SELECT COUNT(*)::int FROM terminal_post_event_outbox WHERE post_id=post.id AND parent_status='partial')
		FROM social_posts post
		JOIN social_post_results facebook ON facebook.post_id=post.id AND facebook.social_account_id='account_mixed_facebook'
		JOIN social_post_results tiktok ON tiktok.post_id=post.id AND tiktok.social_account_id='account_mixed_tiktok'
		WHERE post.id='post_mixed_growth'
	`, period).Scan(&finalParent, &finalFacebook, &finalTikTok, &finalUsage, &finalEvents); err != nil {
		t.Fatal(err)
	}
	if finalParent != "partial" || finalFacebook != "published" || finalTikTok != "failed" || finalUsage != 100 || finalEvents != 1 {
		t.Fatalf("Facebook terminal aggregation=%q/%q/%q usage=%d events=%d, want partial/published/failed/100/1", finalParent, finalFacebook, finalTikTok, finalUsage, finalEvents)
	}
	reservedUnits, err = db.New(pool).CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_mixed_growth", Period: period})
	if err != nil || reservedUnits != 0 {
		t.Fatalf("terminal mixed reservation=%d err=%v, want 0", reservedUnits, err)
	}
}

func TestDueMixedPostUsesPartialHeadroomDeterministically(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := withoutPostMediaRetentionSync(context.Background())
	now := time.Now().UTC()
	period := quota.PeriodForTime(now)
	metadata, err := encodeScheduledPostMetadataWithQuotaAccounts([]platform.PlatformPostInput{
		{AccountID: "account_headroom_tiktok", Caption: "first newly allowed"},
		{AccountID: "account_headroom_facebook", Caption: "reserved"},
		{AccountID: "account_headroom_instagram", Caption: "second newly allowed"},
	}, 1, []string{"account_headroom_facebook"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles VALUES ('profile_headroom','workspace_headroom');
		INSERT INTO social_accounts (id,profile_id,platform) VALUES
			('account_headroom_tiktok','profile_headroom','tiktok'),
			('account_headroom_facebook','profile_headroom','facebook'),
			('account_headroom_instagram','profile_headroom','instagram');
		INSERT INTO plans (id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions (workspace_id,plan_id,status) VALUES ('workspace_headroom','free','active');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage(workspace_id,period,post_count) VALUES ('workspace_headroom',$1,98)`, period); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,workspace_id,status,scheduled_at,created_at,metadata) VALUES ('post_headroom','workspace_headroom','scheduled',$1,$1,$2::jsonb)`, now, metadata); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	handler := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
		SetPublishingRestrictions(&fakePostRestrictionEvaluator{})
	if err := handler.ClaimAndEnqueueScheduledPost(ctx, "post_headroom"); err != nil {
		t.Fatal(err)
	}

	type targetOutcome struct {
		accountID string
		status    string
		jobIndex  pgtype.Int4
		error     pgtype.Text
	}
	rows, err := pool.Query(ctx, `
		SELECT result.social_account_id, result.status, job.post_input_index, result.error_message
		FROM social_post_results result
		LEFT JOIN post_delivery_jobs job ON job.social_post_result_id=result.id
		WHERE result.post_id='post_headroom'
		ORDER BY CASE result.social_account_id
			WHEN 'account_headroom_tiktok' THEN 0
			WHEN 'account_headroom_facebook' THEN 1
			ELSE 2 END
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var outcomes []targetOutcome
	for rows.Next() {
		var outcome targetOutcome
		if err := rows.Scan(&outcome.accountID, &outcome.status, &outcome.jobIndex, &outcome.error); err != nil {
			t.Fatal(err)
		}
		outcomes = append(outcomes, outcome)
	}
	if len(outcomes) != 3 {
		t.Fatalf("outcomes=%v, want 3", outcomes)
	}
	if outcomes[0].status != "pending" || !outcomes[0].jobIndex.Valid || outcomes[0].jobIndex.Int32 != 0 {
		t.Fatalf("first newly allowed outcome=%+v, want pending index 0", outcomes[0])
	}
	if outcomes[1].status != "pending" || !outcomes[1].jobIndex.Valid || outcomes[1].jobIndex.Int32 != 1 {
		t.Fatalf("reserved outcome=%+v, want pending index 1", outcomes[1])
	}
	if outcomes[2].status != "failed" || outcomes[2].jobIndex.Valid || !strings.Contains(outcomes[2].error.String, "already have 2 scheduled posts reserved") {
		t.Fatalf("overflow outcome=%+v, want quota failed without job", outcomes[2])
	}
	var parentStatus string
	var failures, usage int
	if err := pool.QueryRow(ctx, `
		SELECT status,
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id='post_headroom' AND failure_stage='quota'),
		       (SELECT post_count FROM usage WHERE workspace_id='workspace_headroom' AND period=$1)
		FROM social_posts WHERE id='post_headroom'
	`, period).Scan(&parentStatus, &failures, &usage); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "publishing" || failures != 1 || usage != 98 {
		t.Fatalf("partial headroom parent/failures/usage=%q/%d/%d, want publishing/1/98", parentStatus, failures, usage)
	}
	reserved, err := queries.CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_headroom", Period: period})
	if err != nil || reserved != 2 {
		t.Fatalf("partial headroom reservation=%d err=%v, want 2", reserved, err)
	}
}

func TestDueMixedPostReusesReservationReleasedByCurrentRestriction(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := withoutPostMediaRetentionSync(context.Background())
	now := time.Now().UTC()
	period := quota.PeriodForTime(now)
	posts := []platform.PlatformPostInput{
		{AccountID: "account_release_facebook", Caption: "reserved and still allowed"},
		{AccountID: "account_release_instagram", Caption: "reserved then restricted"},
		{AccountID: "account_release_tiktok", Caption: "first newly allowed"},
		{AccountID: "account_release_threads", Caption: "second newly allowed"},
	}
	metadata, err := encodeScheduledPostMetadataWithQuotaAccounts(posts, 2, []string{"account_release_facebook", "account_release_instagram"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles VALUES ('profile_release','workspace_release');
		INSERT INTO social_accounts(id,profile_id,platform) VALUES
			('account_release_facebook','profile_release','facebook'),
			('account_release_instagram','profile_release','instagram'),
			('account_release_tiktok','profile_release','tiktok'),
			('account_release_threads','profile_release','threads');
		INSERT INTO plans(id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions(workspace_id,plan_id,status) VALUES ('workspace_release','free','active');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage(workspace_id,period,post_count) VALUES ('workspace_release',$1,98)`, period); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,workspace_id,status,scheduled_at,created_at,metadata) VALUES ('post_release','workspace_release','scheduled',$1,$1,$2)`, now, metadata); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	handler := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
		SetPublishingRestrictions(&fakePostRestrictionEvaluator{decisions: map[string]publishingrestrictions.Decision{
			"instagram": {Restricted: true, Platform: "instagram", PlanID: "free", CycleID: "cycle_release"},
		}})
	if err := handler.ClaimAndEnqueueScheduledPost(ctx, "post_release"); err != nil {
		t.Fatal(err)
	}
	var parent string
	var jobIndexes []int32
	var policyFailures, quotaFailures, usage int
	if err := pool.QueryRow(ctx, `
		SELECT status,
		       (SELECT ARRAY_AGG(post_input_index ORDER BY post_input_index) FROM post_delivery_jobs WHERE post_id='post_release'),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id='post_release' AND failure_stage='publishing_policy'),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id='post_release' AND failure_stage='quota'),
		       (SELECT post_count FROM usage WHERE workspace_id='workspace_release' AND period=$1)
		FROM social_posts WHERE id='post_release'
	`, period).Scan(&parent, &jobIndexes, &policyFailures, &quotaFailures, &usage); err != nil {
		t.Fatal(err)
	}
	if parent != "publishing" || !slices.Equal(jobIndexes, []int32{0, 2}) || policyFailures != 1 || quotaFailures != 1 || usage != 98 {
		t.Fatalf("released reservation outcome=%q jobs=%v policy=%d quota=%d usage=%d, want publishing/[0 2]/1/1/98", parent, jobIndexes, policyFailures, quotaFailures, usage)
	}
	reserved, err := queries.CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_release", Period: period})
	if err != nil || reserved != 2 {
		t.Fatalf("released reservation final units=%d err=%v, want 2", reserved, err)
	}
}

func TestDueCrossMonthPostUsesOnlyCurrentPeriodPartialHeadroom(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := withoutPostMediaRetentionSync(context.Background())
	now := time.Now().UTC()
	period := quota.PeriodForTime(now)
	priorScheduledAt := now.AddDate(0, -1, 0)
	posts := []platform.PlatformPostInput{
		{AccountID: "account_cross_headroom_0", Caption: "first current-month slot"},
		{AccountID: "account_cross_headroom_1", Caption: "second current-month slot"},
		{AccountID: "account_cross_headroom_2", Caption: "overflow"},
	}
	metadata, err := encodeScheduledPostMetadataWithQuotaAccounts(posts, 3, []string{
		"account_cross_headroom_0", "account_cross_headroom_1", "account_cross_headroom_2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles VALUES ('profile_cross_headroom','workspace_cross_headroom');
		INSERT INTO social_accounts(id,profile_id,platform) VALUES
			('account_cross_headroom_0','profile_cross_headroom','linkedin'),
			('account_cross_headroom_1','profile_cross_headroom','facebook'),
			('account_cross_headroom_2','profile_cross_headroom','instagram');
		INSERT INTO plans(id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions(workspace_id,plan_id,status) VALUES ('workspace_cross_headroom','free','active');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage(workspace_id,period,post_count) VALUES ('workspace_cross_headroom',$1,98)`, period); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,workspace_id,status,scheduled_at,created_at,metadata) VALUES ('post_cross_headroom','workspace_cross_headroom','scheduled',$1,$1,$2)`, priorScheduledAt, metadata); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	if err := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
		ClaimAndEnqueueScheduledPost(ctx, "post_cross_headroom"); err != nil {
		t.Fatal(err)
	}
	var parent string
	var jobIndexes []int32
	var quotaFailures, usage int
	if err := pool.QueryRow(ctx, `
		SELECT status,
		       (SELECT ARRAY_AGG(post_input_index ORDER BY post_input_index) FROM post_delivery_jobs WHERE post_id='post_cross_headroom'),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id='post_cross_headroom' AND failure_stage='quota'),
		       (SELECT post_count FROM usage WHERE workspace_id='workspace_cross_headroom' AND period=$1)
		FROM social_posts WHERE id='post_cross_headroom'
	`, period).Scan(&parent, &jobIndexes, &quotaFailures, &usage); err != nil {
		t.Fatal(err)
	}
	if parent != "publishing" || !slices.Equal(jobIndexes, []int32{0, 1}) || quotaFailures != 1 || usage != 98 {
		t.Fatalf("cross-month headroom outcome=%q jobs=%v failures=%d usage=%d, want publishing/[0 1]/1/98", parent, jobIndexes, quotaFailures, usage)
	}
	reserved, err := queries.CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_cross_headroom", Period: period})
	if err != nil || reserved != 2 {
		t.Fatalf("cross-month current reservation=%d err=%v, want 2", reserved, err)
	}
}

func TestClaimScheduledPostDoesNotDoubleCountOwnQuotaReservation(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := withoutPostMediaRetentionSync(context.Background())
	now := time.Now().UTC()
	period := quota.PeriodForTime(now)
	metadata, err := encodeScheduledPostMetadata([]platform.PlatformPostInput{{
		AccountID: "account_due_instagram",
		Caption:   "claim-before reservation",
	}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	seedStatements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO profiles (id, workspace_id) VALUES ('profile_due', 'workspace_due')`, nil},
		{`INSERT INTO social_accounts (id, profile_id, platform) VALUES ('account_due_instagram', 'profile_due', 'instagram')`, nil},
		{`INSERT INTO plans (id, name, price_cents, post_limit) VALUES ('free', 'Free', 0, 100)`, nil},
		{`INSERT INTO subscriptions (workspace_id, plan_id, status) VALUES ('workspace_due', 'free', 'active')`, nil},
		{`INSERT INTO usage (workspace_id, period, post_count) VALUES ('workspace_due', $1, 99)`, []any{period}},
		{`
			INSERT INTO social_posts (id, workspace_id, status, scheduled_at, created_at, metadata)
			VALUES ('post_due_boundary', 'workspace_due', 'scheduled', $1::timestamptz, $1::timestamptz - INTERVAL '1 hour', $2)
		`, []any{now, metadata}},
	}
	for _, statement := range seedStatements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed due-time quota fixture: %v", err)
		}
	}

	queries := db.New(pool)
	h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil)
	if err := h.ClaimAndEnqueueScheduledPost(ctx, "post_due_boundary"); err != nil {
		t.Fatalf("claim and enqueue scheduled post: %v", err)
	}

	var status string
	var results, jobs int
	if err := pool.QueryRow(ctx, `
		SELECT sp.status,
		       (SELECT COUNT(*)::int FROM social_post_results WHERE post_id = sp.id),
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id = sp.id)
		FROM social_posts sp WHERE sp.id = 'post_due_boundary'
	`).Scan(&status, &results, &jobs); err != nil {
		t.Fatalf("load due-time outcome: %v", err)
	}
	if status != "publishing" || results != 1 || jobs != 1 {
		t.Fatalf("due-time outcome = status:%q results:%d jobs:%d, want publishing/1/1", status, results, jobs)
	}
}

func TestUpdateScheduledPostEnforcesFreeQuotaDeltaAtomically(t *testing.T) {
	tests := []struct {
		name       string
		newTargets []string
		wantStatus int
		wantUnits  int
	}{
		{
			name:       "one to two rejects projected overage",
			newTargets: []string{"account_edit_one", "account_edit_two"},
			wantStatus: http.StatusPaymentRequired,
			wantUnits:  1,
		},
		{
			name:       "one to one preserves replacement capacity",
			newTargets: []string{"account_edit_two"},
			wantStatus: http.StatusOK,
			wantUnits:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := openRestrictedDeliveryIntegrationPool(t)
			setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
			ctx := context.Background()
			now := time.Now().UTC()
			period := quota.PeriodForTime(now)
			scheduledAt := now.Add(24 * time.Hour)
			oldMetadata, err := encodeScheduledPostMetadataWithQuotaAccounts([]platform.PlatformPostInput{{
				AccountID: "account_edit_one",
				Caption:   "existing reservation",
			}}, 1, []string{"account_edit_one"})
			if err != nil {
				t.Fatal(err)
			}
			seedStatements := []struct {
				sql  string
				args []any
			}{
				{`INSERT INTO profiles (id, workspace_id) VALUES ('profile_edit', 'workspace_edit')`, nil},
				{`INSERT INTO social_accounts (id, profile_id, platform) VALUES ('account_edit_one', 'profile_edit', 'instagram'), ('account_edit_two', 'profile_edit', 'instagram')`, nil},
				{`INSERT INTO plans (id, name, price_cents, post_limit) VALUES ('free', 'Free', 0, 100)`, nil},
				{`INSERT INTO subscriptions (workspace_id, plan_id, status) VALUES ('workspace_edit', 'free', 'active')`, nil},
				{`INSERT INTO usage (workspace_id, period, post_count) VALUES ('workspace_edit', $1, 99)`, []any{period}},
				{`
					INSERT INTO social_posts (id, workspace_id, status, scheduled_at, created_at, metadata)
					VALUES ('post_edit_boundary', 'workspace_edit', 'scheduled', $1, $2, $3)
				`, []any{scheduledAt, now.Add(-time.Hour), oldMetadata}},
			}
			for _, statement := range seedStatements {
				if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
					t.Fatalf("seed scheduled edit fixture: %v", err)
				}
			}

			var posts []string
			for _, accountID := range tt.newTargets {
				posts = append(posts, fmt.Sprintf(`{"account_id":%q,"caption":"updated"}`, accountID))
			}
			body := fmt.Sprintf(`{
				"scheduled_quota_units":0,
				"scheduled_quota_account_ids":[],
				"scheduled_quota_snapshot":{"version":1,"targets":[]},
				"platform_posts":[%s]
			}`, strings.Join(posts, ","))
			req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post_edit_boundary", strings.NewReader(body))
			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", "post_edit_boundary")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace_edit"))
			req = req.WithContext(withoutPostMediaRetentionSync(req.Context()))
			rr := httptest.NewRecorder()
			queries := db.New(pool)
			h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
				SetPaidScheduleCoordinator(paidquota.NewPostgresCoordinator(pool))

			h.UpdateDraft(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("update status = %d, want %d; body=%s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			var metadata []byte
			if err := pool.QueryRow(ctx, `SELECT metadata FROM social_posts WHERE id = 'post_edit_boundary'`).Scan(&metadata); err != nil {
				t.Fatalf("load edited metadata: %v", err)
			}
			units, ok := scheduledQuotaUnitsFromMetadata(metadata)
			if !ok || units != tt.wantUnits {
				t.Fatalf("persisted snapshot = %d/%t, want %d/true", units, ok, tt.wantUnits)
			}
			if tt.wantStatus == http.StatusOK {
				reserved, ok := scheduledQuotaReservedIndexesFromMetadata(metadata)
				wantReserved := make([]bool, len(tt.newTargets))
				for index := range wantReserved {
					wantReserved[index] = true
				}
				if !ok || !slices.Equal(reserved, wantReserved) {
					t.Fatalf("updated target snapshot = %v/%t, want %v/true", reserved, ok, wantReserved)
				}
			}
			parsed, err := platform.DecodePostMetadata(metadata, "")
			if err != nil {
				t.Fatalf("decode persisted targets: %v", err)
			}
			wantTargets := len(tt.newTargets)
			if tt.wantStatus == http.StatusPaymentRequired {
				wantTargets = 1
			}
			if len(parsed) != wantTargets {
				t.Fatalf("persisted targets = %d, want %d", len(parsed), wantTargets)
			}
		})
	}
}

func TestConcurrentFreeScheduledCreatesSerializeQuotaAdmission(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := context.Background()
	now := time.Now().UTC()
	period := quota.PeriodForTime(now)
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles VALUES ('profile_concurrent_create', 'workspace_concurrent_create');
		INSERT INTO social_accounts (id, profile_id, platform) VALUES ('account_concurrent_create', 'profile_concurrent_create', 'linkedin');
		INSERT INTO plans (id, name, price_cents, post_limit) VALUES ('free', 'Free', 0, 100);
		INSERT INTO subscriptions (workspace_id, plan_id, status) VALUES ('workspace_concurrent_create', 'free', 'active');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id, period, post_count) VALUES ('workspace_concurrent_create', $1, 99)`, period); err != nil {
		t.Fatal(err)
	}

	statuses := make(chan int, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func(index int) {
			<-start
			queries := db.New(pool)
			h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
				SetPaidScheduleCoordinator(paidquota.NewPostgresCoordinator(pool))
			scheduledAt := now.Add(24 * time.Hour)
			req := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
			req = req.WithContext(withoutPostMediaRetentionSync(req.Context()))
			rr := httptest.NewRecorder()
			h.createScheduledPost(rr, req, "workspace_concurrent_create", parsedRequest{
				Posts:       []platform.PlatformPostInput{{AccountID: "account_concurrent_create", Caption: fmt.Sprintf("post %d", index)}},
				ScheduledAt: &scheduledAt,
			}, 1)
			statuses <- rr.Code
		}(i)
	}
	close(start)
	got := []int{<-statuses, <-statuses}
	slices.Sort(got)
	if !slices.Equal(got, []int{http.StatusCreated, http.StatusPaymentRequired}) {
		t.Fatalf("concurrent create statuses = %v, want [201 402]", got)
	}
	var posts int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM social_posts WHERE workspace_id='workspace_concurrent_create'`).Scan(&posts); err != nil || posts != 1 {
		t.Fatalf("persisted concurrent scheduled posts = %d, err=%v, want 1", posts, err)
	}
}

func TestConcurrentScheduledIdempotencyReplayPrecedesMutableCapacityGates(t *testing.T) {
	tests := []struct {
		name        string
		planID      string
		limit       int
		usage       int
		activeLimit int
	}{
		{name: "free monthly final unit", planID: "free", limit: 100, usage: 99, activeLimit: 10},
		{name: "paid monthly final unit", planID: "basic", limit: 2500, usage: 2499},
		{name: "free active scheduled final slot", planID: "free", limit: 100, usage: 0, activeLimit: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := openRestrictedDeliveryIntegrationPool(t)
			setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
			ctx := context.Background()
			now := time.Now().UTC()
			scheduledAt := now.Add(24 * time.Hour).Truncate(time.Second)
			period := quota.PeriodForTime(scheduledAt)
			workspaceID := "workspace_idempotent_capacity"
			accountID := "account_idempotent_capacity"

			seed := []struct {
				query string
				args  []any
			}{
				{`INSERT INTO profiles VALUES ('profile_idempotent_capacity', $1)`, []any{workspaceID}},
				{`INSERT INTO social_accounts (id, profile_id, platform) VALUES ($1, 'profile_idempotent_capacity', 'linkedin')`, []any{accountID}},
				{`INSERT INTO plans (id, name, price_cents, post_limit) VALUES ($1, $1, 0, $2)`, []any{tt.planID, tt.limit}},
				{`INSERT INTO subscriptions (workspace_id, plan_id, status) VALUES ($1, $2, 'active')`, []any{workspaceID, tt.planID}},
				{`INSERT INTO usage (workspace_id, period, post_count) VALUES ($1, $2, $3)`, []any{workspaceID, period, tt.usage}},
			}
			for _, statement := range seed {
				if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
					t.Fatal(err)
				}
			}
			if tt.activeLimit > 0 {
				if _, err := pool.Exec(ctx, `INSERT INTO workspace_active_scheduled_limits (workspace_id, limit_count) VALUES ($1, $2)`, workspaceID, tt.activeLimit); err != nil {
					t.Fatal(err)
				}
			}

			responses := runConcurrentScheduledCreatesBehindPeriodLock(t, pool, workspaceID, accountID, "same-key", []string{"same payload", "same payload"}, scheduledAt)
			assertConcurrentScheduledReplayOutcomes(t, pool, workspaceID, accountID, period, responses)
		})
	}
}

func TestConcurrentScheduledIdempotencyDifferentPayloadConflicts(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := context.Background()
	now := time.Now().UTC()
	scheduledAt := now.Add(24 * time.Hour).Truncate(time.Second)
	period := quota.PeriodForTime(scheduledAt)
	workspaceID := "workspace_idempotent_conflict"
	accountID := "account_idempotent_conflict"
	seed := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO profiles VALUES ('profile_idempotent_conflict', $1)`, []any{workspaceID}},
		{`INSERT INTO social_accounts (id, profile_id, platform) VALUES ($1, 'profile_idempotent_conflict', 'linkedin')`, []any{accountID}},
		{`INSERT INTO plans (id, name, price_cents, post_limit) VALUES ('free', 'Free', 0, 100)`, nil},
		{`INSERT INTO subscriptions (workspace_id, plan_id, status) VALUES ($1, 'free', 'active')`, []any{workspaceID}},
		{`INSERT INTO usage (workspace_id, period, post_count) VALUES ($1, $2, 0)`, []any{workspaceID, period}},
		{`INSERT INTO workspace_active_scheduled_limits (workspace_id, limit_count) VALUES ($1, 10)`, []any{workspaceID}},
	}
	for _, statement := range seed {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}

	responses := runConcurrentScheduledCreatesBehindPeriodLock(t, pool, workspaceID, accountID, "conflicting-key", []string{"first payload", "different payload"}, scheduledAt)
	statuses := []int{responses[0].status, responses[1].status}
	slices.Sort(statuses)
	if !slices.Equal(statuses, []int{http.StatusCreated, http.StatusConflict}) {
		t.Fatalf("concurrent conflict statuses = %v, want [201 409]; bodies=%q / %q", statuses, responses[0].body, responses[1].body)
	}
	var posts int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM social_posts WHERE workspace_id=$1`, workspaceID).Scan(&posts); err != nil || posts != 1 {
		t.Fatalf("persisted posts = %d, err=%v, want 1", posts, err)
	}
}

type blockingIdempotencyPolicyEvaluator struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
}

func (e *blockingIdempotencyPolicyEvaluator) Evaluate(context.Context, string, string) (publishingrestrictions.Decision, error) {
	e.mu.Lock()
	e.calls++
	call := e.calls
	e.mu.Unlock()
	if call == 1 {
		close(e.entered)
		<-e.release
		return publishingrestrictions.Decision{}, nil
	}
	return publishingrestrictions.Decision{}, errors.New("policy changed after first acceptance")
}

func (e *blockingIdempotencyPolicyEvaluator) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

type oneTokenEnqueueLimiter struct {
	mu           sync.Mutex
	enqueueCalls int
}

func (l *oneTokenEnqueueLimiter) AllowRequest(context.Context, ratelimit.RequestScope) (ratelimit.Decision, error) {
	return ratelimit.Decision{Allowed: true}, nil
}

func (l *oneTokenEnqueueLimiter) AllowEnqueue(context.Context, ratelimit.EnqueueScope, int) (ratelimit.Decision, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enqueueCalls++
	return ratelimit.Decision{Allowed: l.enqueueCalls == 1, Reason: "one_token"}, nil
}

func (l *oneTokenEnqueueLimiter) CheckQueueDepth(context.Context, ratelimit.QueueScope, int) (ratelimit.Decision, error) {
	return ratelimit.Decision{Allowed: true}, nil
}

func (l *oneTokenEnqueueLimiter) calls() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enqueueCalls
}

func TestScheduledIdempotencySerializesPolicyAndEnqueueBehindKeyLock(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := context.Background()
	scheduledAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	period := quota.PeriodForTime(scheduledAt)
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles VALUES ('profile_mutable_gates','workspace_mutable_gates');
		INSERT INTO social_accounts (id,profile_id,platform) VALUES ('account_mutable_gates','profile_mutable_gates','linkedin');
		INSERT INTO plans (id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions (workspace_id,plan_id,status) VALUES ('workspace_mutable_gates','free','active');
		INSERT INTO workspace_active_scheduled_limits (workspace_id,limit_count) VALUES ('workspace_mutable_gates',10);
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id,period,post_count) VALUES ('workspace_mutable_gates',$1,99)`, period); err != nil {
		t.Fatal(err)
	}

	evaluator := &blockingIdempotencyPolicyEvaluator{entered: make(chan struct{}), release: make(chan struct{})}
	limiter := &oneTokenEnqueueLimiter{}
	queries := db.New(pool)
	h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, limiter, nil).
		SetPaidScheduleCoordinator(paidquota.NewPostgresCoordinator(pool)).
		SetPublishingRestrictions(evaluator)
	request := func() *http.Request {
		body := fmt.Sprintf(`{"scheduled_at":%q,"idempotency_key":"mutable-key","platform_posts":[{"account_id":"account_mutable_gates","caption":"same payload"}]}`, scheduledAt.Format(time.RFC3339Nano))
		req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
		req = req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace_mutable_gates"))
		return req.WithContext(withoutPostMediaRetentionSync(req.Context()))
	}

	responses := make(chan scheduledCreateIntegrationResponse, 2)
	run := func() {
		rr := httptest.NewRecorder()
		h.Create(rr, request())
		responses <- scheduledCreateIntegrationResponse{status: rr.Code, body: rr.Body.String(), replay: rr.Header().Get("X-UniPost-Idempotent-Replay") == "true"}
	}
	go run()
	select {
	case <-evaluator.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first request did not reach policy while holding the idempotency lock")
	}
	baselineWaiting := countWaitingAdvisoryLocks(t, pool)
	go run()
	deadline := time.Now().Add(3 * time.Second)
	for countWaitingAdvisoryLocks(t, pool) < baselineWaiting+1 {
		if time.Now().After(deadline) {
			t.Fatal("second request did not wait on the idempotency lock before policy")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(evaluator.release)
	got := []scheduledCreateIntegrationResponse{<-responses, <-responses}
	if got[0].status != http.StatusCreated || got[1].status != http.StatusCreated {
		t.Fatalf("statuses=%d/%d want 201/201; bodies=%q/%q", got[0].status, got[1].status, got[0].body, got[1].body)
	}
	replays := 0
	for _, response := range got {
		if response.replay {
			replays++
		}
	}
	if replays != 1 || evaluator.callCount() != 1 || limiter.calls() != 1 {
		t.Fatalf("replays=%d policy_calls=%d enqueue_calls=%d, want 1/1/1", replays, evaluator.callCount(), limiter.calls())
	}
}

func TestScheduledIdempotencyPolicyUsesTransactionAtMaxConnsOne(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE platform_publishing_restrictions (
			id TEXT PRIMARY KEY, platform TEXT NOT NULL UNIQUE, enabled BOOLEAN NOT NULL,
			restricted_plan_ids TEXT[] NOT NULL DEFAULT '{}', reason_code TEXT NOT NULL DEFAULT '',
			user_message TEXT NOT NULL DEFAULT '', cycle_id TEXT, version BIGINT NOT NULL DEFAULT 1,
			enabled_at TIMESTAMPTZ, disabled_at TIMESTAMPTZ, updated_by_user_id TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO platform_publishing_restrictions (id,platform,enabled) VALUES ('restriction_linkedin','linkedin',FALSE);
		INSERT INTO profiles VALUES ('profile_one_conn','workspace_one_conn');
		INSERT INTO social_accounts (id,profile_id,platform) VALUES ('account_one_conn','profile_one_conn','linkedin');
		INSERT INTO plans (id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions (workspace_id,plan_id,status) VALUES ('workspace_one_conn','free','active');
	`); err != nil {
		t.Fatal(err)
	}

	config := pool.Config()
	config.MaxConns = 1
	oneConnectionPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer oneConnectionPool.Close()
	queries := db.New(oneConnectionPool)
	policy := publishingrestrictions.NewService(publishingrestrictions.NewPostgresStore(oneConnectionPool))
	h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
		SetPaidScheduleCoordinator(paidquota.NewPostgresCoordinator(oneConnectionPool)).
		SetPublishingRestrictions(policy)
	scheduledAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	body := fmt.Sprintf(`{"scheduled_at":%q,"idempotency_key":"one-connection-key","platform_posts":[{"account_id":"account_one_conn","caption":"same transaction"}]}`, scheduledAt.Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace_one_conn"))
	req = req.WithContext(withoutPostMediaRetentionSync(req.Context()))
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		done <- rr
	}()
	select {
	case rr := <-done:
		if rr.Code != http.StatusCreated {
			t.Fatalf("status=%d want201 body=%s", rr.Code, rr.Body.String())
		}
	case <-time.After(4 * time.Second):
		t.Fatal("scheduled create self-deadlocked by checking out a second policy connection")
	}
}

func TestScheduledQuotaHoldIdempotencyPhaseAUsesLegacyIndexAndNewLookup(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := context.Background()
	scheduledAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	metadata, err := encodeScheduledPostMetadata([]platform.PlatformPostInput{{AccountID: "account_phase_a", Caption: "same payload"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles VALUES ('profile_phase_a','workspace_phase_a');
		INSERT INTO social_accounts (id,profile_id,platform) VALUES ('account_phase_a','profile_phase_a','linkedin');
		INSERT INTO plans (id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions (workspace_id,plan_id,status) VALUES ('workspace_phase_a','free','active');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id,workspace_id,status,scheduled_at,created_at,metadata,idempotency_key)
		VALUES ('post_phase_a','workspace_phase_a','quota_hold',$1,$2,$3,'phase-a-key')
	`, scheduledAt, scheduledAt.Add(-time.Hour), metadata); err != nil {
		t.Fatal(err)
	}

	var predicate string
	if err := pool.QueryRow(ctx, `
		SELECT pg_get_expr(index.indpred, index.indrelid)
		FROM pg_index index
		JOIN pg_class relation ON relation.oid=index.indexrelid
		JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
		WHERE relation.relname='social_posts_workspace_scheduled_idempotency_uniq'
		  AND namespace.nspname=current_schema()
	`).Scan(&predicate); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(predicate, "scheduled") || strings.Contains(predicate, "quota_hold") {
		t.Fatalf("Phase A index predicate=%q, want legacy scheduled-only semantics", predicate)
	}

	request := func(caption string) *http.Request {
		body := fmt.Sprintf(`{"scheduled_at":%q,"idempotency_key":"phase-a-key","platform_posts":[{"account_id":"account_phase_a","caption":%q}]}`, scheduledAt.Format(time.RFC3339Nano), caption)
		req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
		return req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace_phase_a"))
	}
	queries := db.New(pool)
	h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
		SetPaidScheduleCoordinator(paidquota.NewPostgresCoordinator(pool))
	replay := httptest.NewRecorder()
	h.Create(replay, request("same payload"))
	if replay.Code != http.StatusCreated || replay.Header().Get("X-UniPost-Idempotent-Replay") != "true" {
		t.Fatalf("quota-hold replay status=%d replay=%q body=%s", replay.Code, replay.Header().Get("X-UniPost-Idempotent-Replay"), replay.Body.String())
	}
	conflict := httptest.NewRecorder()
	h.Create(conflict, request("different payload"))
	if conflict.Code != http.StatusConflict {
		t.Fatalf("quota-hold conflict status=%d want409 body=%s", conflict.Code, conflict.Body.String())
	}
}

type scheduledCreateIntegrationResponse struct {
	status int
	body   string
	replay bool
}

func runConcurrentScheduledCreatesBehindPeriodLock(
	t *testing.T,
	pool *pgxpool.Pool,
	workspaceID string,
	accountID string,
	idempotencyKey string,
	captions []string,
	scheduledAt time.Time,
) []scheduledCreateIntegrationResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	period := quota.PeriodForTime(scheduledAt)
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback(context.Background()) }()
	if _, err := blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, workspaceID, "paid_schedule_quota:"+period); err != nil {
		t.Fatal(err)
	}

	baselineWaiting := countWaitingAdvisoryLocks(t, pool)
	responses := make(chan scheduledCreateIntegrationResponse, len(captions))
	start := make(chan struct{})
	for _, caption := range captions {
		caption := caption
		go func() {
			<-start
			body := fmt.Sprintf(`{
				"scheduled_at":%q,
				"idempotency_key":%q,
				"scheduled_quota_units":0,
				"scheduled_quota_account_ids":[],
				"scheduled_quota_snapshot":{"version":1,"targets":[]},
				"platform_posts":[{"account_id":%q,"caption":%q}]
			}`, scheduledAt.Format(time.RFC3339Nano), idempotencyKey, accountID, caption)
			req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(body))
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), workspaceID))
			req = req.WithContext(withoutPostMediaRetentionSync(req.Context()))
			rr := httptest.NewRecorder()
			queries := db.New(pool)
			h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
				SetPaidScheduleCoordinator(paidquota.NewPostgresCoordinator(pool))
			h.Create(rr, req)
			var envelope struct {
				Data struct {
					IdempotencyReplay bool `json:"idempotency_replay"`
				} `json:"data"`
			}
			_ = json.Unmarshal(rr.Body.Bytes(), &envelope)
			responses <- scheduledCreateIntegrationResponse{status: rr.Code, body: rr.Body.String(), replay: envelope.Data.IdempotencyReplay}
		}()
	}
	close(start)

	deadline := time.Now().Add(5 * time.Second)
	for countWaitingAdvisoryLocks(t, pool) < baselineWaiting+len(captions) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for both scheduled creates to pass their initial idempotency miss and block on the quota period lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	got := make([]scheduledCreateIntegrationResponse, 0, len(captions))
	for range captions {
		select {
		case response := <-responses:
			got = append(got, response)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for concurrent scheduled create responses: %v", ctx.Err())
		}
	}
	return got
}

func countWaitingAdvisoryLocks(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*)::int FROM pg_locks WHERE locktype='advisory' AND NOT granted`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func assertConcurrentScheduledReplayOutcomes(t *testing.T, pool *pgxpool.Pool, workspaceID, accountID, period string, responses []scheduledCreateIntegrationResponse) {
	t.Helper()
	statuses := []int{responses[0].status, responses[1].status}
	slices.Sort(statuses)
	if !slices.Equal(statuses, []int{http.StatusCreated, http.StatusCreated}) {
		t.Fatalf("concurrent replay statuses = %v, want [201 201]; bodies=%q / %q", statuses, responses[0].body, responses[1].body)
	}
	replays := 0
	for _, response := range responses {
		if response.replay {
			replays++
		}
	}
	if replays != 1 {
		t.Fatalf("idempotency replay responses = %d, want exactly 1", replays)
	}
	var posts int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*)::int FROM social_posts WHERE workspace_id=$1`, workspaceID).Scan(&posts); err != nil || posts != 1 {
		t.Fatalf("persisted posts = %d, err=%v, want 1", posts, err)
	}
	var metadata []byte
	if err := pool.QueryRow(context.Background(), `SELECT metadata FROM social_posts WHERE workspace_id=$1`, workspaceID).Scan(&metadata); err != nil {
		t.Fatal(err)
	}
	reservedIndexes, ok := scheduledQuotaReservedIndexesFromMetadata(metadata)
	if !ok || !slices.Equal(reservedIndexes, []bool{true}) {
		t.Fatalf("idempotent replay snapshot for %s = %v/%t, want [true]/true", accountID, reservedIndexes, ok)
	}
	reserved, err := db.New(pool).CountScheduledQuotaUnitsByWorkspaceAndPeriod(context.Background(), db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: workspaceID, Period: period})
	if err != nil || reserved != 1 {
		t.Fatalf("scheduled reservation = %d, err=%v, want 1", reserved, err)
	}
}

func TestConcurrentDueReservationGrowthSerializesAndCrossMonthUsesFullUnits(t *testing.T) {
	t.Run("concurrent snapshot growth", func(t *testing.T) {
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
		ctx := withoutPostMediaRetentionSync(context.Background())
		now := time.Now().UTC()
		period := quota.PeriodForTime(now)
		if _, err := pool.Exec(ctx, `
			INSERT INTO profiles VALUES ('profile_due_growth', 'workspace_due_growth');
			INSERT INTO social_accounts (id, profile_id, platform) VALUES
			 ('account_due_growth_1', 'profile_due_growth', 'linkedin'), ('account_due_growth_2', 'profile_due_growth', 'linkedin');
			INSERT INTO plans (id, name, price_cents, post_limit) VALUES ('free', 'Free', 0, 100);
			INSERT INTO subscriptions (workspace_id, plan_id, status) VALUES ('workspace_due_growth', 'free', 'active');
		`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id, period, post_count) VALUES ('workspace_due_growth', $1, 99)`, period); err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 2; i++ {
			metadata, _ := encodeScheduledPostMetadata([]platform.PlatformPostInput{{AccountID: fmt.Sprintf("account_due_growth_%d", i)}}, 0)
			if _, err := pool.Exec(ctx, `INSERT INTO social_posts (id,workspace_id,status,scheduled_at,created_at,metadata) VALUES ($1,'workspace_due_growth','scheduled',$2,$2,$3)`, fmt.Sprintf("post_due_growth_%d", i), now, metadata); err != nil {
				t.Fatal(err)
			}
		}
		errs := make(chan error, 2)
		start := make(chan struct{})
		for i := 1; i <= 2; i++ {
			go func(i int) {
				<-start
				q := db.New(pool)
				h := NewSocialPostHandler(q, nil, quota.NewChecker(q), nil, nil, nil, nil)
				errs <- h.ClaimAndEnqueueScheduledPost(ctx, fmt.Sprintf("post_due_growth_%d", i))
			}(i)
		}
		close(start)
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		var publishing, failed, jobs int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE status='publishing')::int, COUNT(*) FILTER (WHERE status='failed')::int, (SELECT COUNT(*)::int FROM post_delivery_jobs) FROM social_posts WHERE workspace_id='workspace_due_growth'`).Scan(&publishing, &failed, &jobs); err != nil {
			t.Fatal(err)
		}
		if publishing != 1 || failed != 1 || jobs != 1 {
			t.Fatalf("concurrent due = publishing:%d failed:%d jobs:%d, want 1/1/1", publishing, failed, jobs)
		}
		reserved, err := db.New(pool).CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_due_growth", Period: period})
		if err != nil || reserved != 1 {
			t.Fatalf("same-period execution reservation = %d, err=%v, want 1", reserved, err)
		}
	})

	t.Run("concurrent prior month execution reservations move to current period", func(t *testing.T) {
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
		ctx := withoutPostMediaRetentionSync(context.Background())
		now := time.Now().UTC()
		period := quota.PeriodForTime(now)
		priorScheduledAt := now.AddDate(0, -1, 0)
		if _, err := pool.Exec(ctx, `
			INSERT INTO profiles VALUES ('profile_due_cross_growth', 'workspace_due_cross_growth');
			INSERT INTO social_accounts (id, profile_id, platform) VALUES
			 ('account_due_cross_growth_1', 'profile_due_cross_growth', 'linkedin'),
			 ('account_due_cross_growth_2', 'profile_due_cross_growth', 'linkedin');
			INSERT INTO plans (id, name, price_cents, post_limit) VALUES ('free', 'Free', 0, 100);
			INSERT INTO subscriptions (workspace_id, plan_id, status) VALUES ('workspace_due_cross_growth', 'free', 'active');
		`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id, period, post_count) VALUES ('workspace_due_cross_growth', $1, 99)`, period); err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 2; i++ {
			metadata, _ := encodeScheduledPostMetadata([]platform.PlatformPostInput{{AccountID: fmt.Sprintf("account_due_cross_growth_%d", i)}}, 1)
			var object map[string]any
			if err := json.Unmarshal(metadata, &object); err != nil {
				t.Fatal(err)
			}
			// Simulates untrusted/legacy metadata. The claim path must overwrite
			// this with the server-selected execution period.
			object["scheduled_execution_reservation_period"] = "2099-12"
			metadata, _ = json.Marshal(object)
			if _, err := pool.Exec(ctx, `INSERT INTO social_posts (id,workspace_id,status,scheduled_at,created_at,metadata) VALUES ($1,'workspace_due_cross_growth','scheduled',$2,$2,$3)`, fmt.Sprintf("post_due_cross_growth_%d", i), priorScheduledAt, metadata); err != nil {
				t.Fatal(err)
			}
		}

		errs := make(chan error, 2)
		start := make(chan struct{})
		for i := 1; i <= 2; i++ {
			go func(i int) {
				<-start
				queries := db.New(pool)
				h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil)
				errs <- h.ClaimAndEnqueueScheduledPost(ctx, fmt.Sprintf("post_due_cross_growth_%d", i))
			}(i)
		}
		close(start)
		for range 2 {
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
		}

		queries := db.New(pool)
		reserved, err := queries.CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_due_cross_growth", Period: period})
		if err != nil || reserved != 1 {
			t.Fatalf("current-period execution reservation = %d, err=%v, want 1", reserved, err)
		}
		var publishing, failed, jobs int
		var publishedResultID string
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FILTER (WHERE sp.status='publishing')::int,
			       COUNT(*) FILTER (WHERE sp.status='failed')::int,
			       (SELECT COUNT(*)::int FROM post_delivery_jobs),
			       (SELECT id FROM social_post_results WHERE status='pending' LIMIT 1)
			FROM social_posts sp WHERE sp.workspace_id='workspace_due_cross_growth'
		`).Scan(&publishing, &failed, &jobs, &publishedResultID); err != nil {
			t.Fatal(err)
		}
		if publishing != 1 || failed != 1 || jobs != 1 {
			t.Fatalf("cross-period concurrent due = publishing:%d failed:%d jobs:%d, want 1/1/1", publishing, failed, jobs)
		}

		if _, err := queries.UpdateSocialPostResultAfterRetryAndIncrementUsage(ctx, db.UpdateSocialPostResultAfterRetryAndIncrementUsageParams{
			ID: publishedResultID, Status: "published",
			ExternalID:  pgtype.Text{String: "external_cross_period", Valid: true},
			PublishedAt: pgtype.Timestamptz{Time: now, Valid: true},
			WorkspaceID: "workspace_due_cross_growth", Period: period, PostCount: 1,
		}); err != nil {
			t.Fatal(err)
		}
		reserved, err = queries.CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_due_cross_growth", Period: period})
		if err != nil || reserved != 0 {
			t.Fatalf("terminal execution reservation = %d, err=%v, want 0", reserved, err)
		}
		var usage int
		if err := pool.QueryRow(ctx, `SELECT post_count FROM usage WHERE workspace_id='workspace_due_cross_growth' AND period=$1`, period).Scan(&usage); err != nil || usage != 100 {
			t.Fatalf("terminal usage = %d, err=%v, want 100", usage, err)
		}
	})

	t.Run("prior month reservation is not deducted", func(t *testing.T) {
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
		ctx := withoutPostMediaRetentionSync(context.Background())
		now := time.Now().UTC()
		period := quota.PeriodForTime(now)
		metadata, _ := encodeScheduledPostMetadataWithQuotaAccounts([]platform.PlatformPostInput{
			{AccountID: "account_cross_month_reserved"},
			{AccountID: "account_cross_month_new"},
		}, 1, []string{"account_cross_month_reserved"})
		if _, err := pool.Exec(ctx, `INSERT INTO profiles VALUES ('profile_cross_month','workspace_cross_month'); INSERT INTO social_accounts (id,profile_id,platform) VALUES ('account_cross_month_reserved','profile_cross_month','linkedin'), ('account_cross_month_new','profile_cross_month','linkedin'); INSERT INTO plans (id,name,price_cents,post_limit) VALUES ('free','Free',0,100); INSERT INTO subscriptions (workspace_id,plan_id,status) VALUES ('workspace_cross_month','free','active');`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id,period,post_count) VALUES ('workspace_cross_month',$1,100)`, period); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO social_posts (id,workspace_id,status,scheduled_at,created_at,metadata) VALUES ('post_cross_month','workspace_cross_month','scheduled',$1,$1,$2)`, now.AddDate(0, -1, 0), metadata); err != nil {
			t.Fatal(err)
		}
		q := db.New(pool)
		h := NewSocialPostHandler(q, nil, quota.NewChecker(q), nil, nil, nil, nil)
		if err := h.ClaimAndEnqueueScheduledPost(ctx, "post_cross_month"); err != nil {
			t.Fatal(err)
		}
		var status string
		var results, jobs int
		if err := pool.QueryRow(ctx, `
			SELECT status,
			       (SELECT COUNT(*)::int FROM social_post_results WHERE post_id='post_cross_month'),
			       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id='post_cross_month')
			FROM social_posts WHERE id='post_cross_month'
		`).Scan(&status, &results, &jobs); err != nil {
			t.Fatal(err)
		}
		if status != "failed" || results != 2 || jobs != 0 {
			t.Fatalf("cross-month outcome=%q/%d/%d, want failed/2/0", status, results, jobs)
		}
	})
}

func TestAtomicMonthlySnapshotRejectsDuringTerminalReservationConversion(t *testing.T) {
	for _, test := range []struct {
		name        string
		scheduledAt func(time.Time) time.Time
	}{
		{name: "same month", scheduledAt: func(now time.Time) time.Time { return now.Add(24 * time.Hour) }},
		{name: "cross month execution reservation", scheduledAt: func(now time.Time) time.Time { return now.AddDate(0, -1, 0) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			pool := openRestrictedDeliveryIntegrationPool(t)
			setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			now := time.Now().UTC()
			period := quota.PeriodForTime(now)
			scheduledAt := test.scheduledAt(now)
			metadata := fmt.Sprintf(`{"scheduled_execution_reservation_period":%q,"scheduled_quota_units":1,"platform_posts":[{"account_id":"account_terminal_snapshot"}]}`, period)
			if _, err := pool.Exec(ctx, `
				INSERT INTO profiles VALUES ('profile_terminal_snapshot','workspace_terminal_snapshot');
				INSERT INTO social_accounts (id,profile_id,platform) VALUES ('account_terminal_snapshot','profile_terminal_snapshot','linkedin');
				INSERT INTO plans (id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
				INSERT INTO subscriptions (workspace_id,plan_id,status) VALUES ('workspace_terminal_snapshot','free','active');
			`); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id,period,post_count) VALUES ('workspace_terminal_snapshot',$1,99)`, period); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `
				INSERT INTO social_posts (id,workspace_id,status,scheduled_at,created_at,metadata)
				VALUES ('post_terminal_snapshot','workspace_terminal_snapshot','publishing',$1,$2,$3::jsonb)
			`, scheduledAt, now.Add(-time.Hour), metadata); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO social_post_results (id,post_id,social_account_id,status) VALUES ('result_terminal_snapshot','post_terminal_snapshot','account_terminal_snapshot','pending')`); err != nil {
				t.Fatal(err)
			}

			terminalTx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = terminalTx.Rollback(context.Background()) }()
			if _, err := db.New(terminalTx).UpdateSocialPostResultAfterRetryAndIncrementUsage(ctx, db.UpdateSocialPostResultAfterRetryAndIncrementUsageParams{
				ID: "result_terminal_snapshot", Status: "published",
				ExternalID:  pgtype.Text{String: "external_terminal_snapshot", Valid: true},
				PublishedAt: pgtype.Timestamptz{Time: now, Valid: true},
				WorkspaceID: "workspace_terminal_snapshot", Period: period, PostCount: 1,
			}); err != nil {
				t.Fatal(err)
			}

			mutated := false
			admissionDone := make(chan error, 1)
			go func() {
				admissionDone <- paidquota.NewPostgresCoordinator(pool).Mutate(ctx, "workspace_terminal_snapshot", []paidquota.PeriodDelta{{Period: period, RequestedUnits: 1}}, func(*db.Queries) error {
					mutated = true
					return nil
				})
			}()
			select {
			case admissionErr := <-admissionDone:
				var quotaErr *paidquota.AdmissionError
				if !errors.As(admissionErr, &quotaErr) {
					t.Fatalf("admission error=%v, want quota rejection against pre-conversion 99+1 snapshot", admissionErr)
				}
			case <-ctx.Done():
				t.Fatalf("admission snapshot blocked during terminal conversion: %v", ctx.Err())
			}
			if mutated {
				t.Fatal("admission mutation ran while effective usage was already 100")
			}
			if err := terminalTx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			final, err := quota.NewChecker(db.New(pool)).MonthlySnapshotForPeriod(ctx, "workspace_terminal_snapshot", period)
			if err != nil {
				t.Fatal(err)
			}
			if final.Completed != 100 || final.Scheduled != 0 || final.EffectiveUsage() != 100 {
				t.Fatalf("final snapshot=%#v, want completed=100 scheduled=0 effective=100", final)
			}
		})
	}
}

func TestPublishQuotaHoldUsesSamePeriodDeltaAndCrossPeriodFullUnits(t *testing.T) {
	for _, tc := range []struct {
		name       string
		priorMonth bool
		usage      int
		wantCode   int
		wantStatus string
		wantJobs   int
	}{
		{name: "same period replaces reservation", usage: 99, wantCode: http.StatusAccepted, wantStatus: "publishing", wantJobs: 1},
		{name: "cross period checks full execution", priorMonth: true, usage: 100, wantCode: http.StatusPaymentRequired, wantStatus: "quota_hold", wantJobs: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := openRestrictedDeliveryIntegrationPool(t)
			setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
			ctx := context.Background()
			now := time.Now().UTC()
			scheduledAt := now
			if tc.priorMonth {
				scheduledAt = now.AddDate(0, -1, 0)
			}
			period := quota.PeriodForTime(now)
			metadata, _ := encodeScheduledPostMetadata([]platform.PlatformPostInput{{AccountID: "account_hold_publish", Caption: "publish hold"}}, 1)
			if _, err := pool.Exec(ctx, `INSERT INTO profiles VALUES ('profile_hold_publish','workspace_hold_publish'); INSERT INTO social_accounts (id,profile_id,platform) VALUES ('account_hold_publish','profile_hold_publish','linkedin'); INSERT INTO plans (id,name,price_cents,post_limit) VALUES ('free','Free',0,100); INSERT INTO subscriptions (workspace_id,plan_id,status) VALUES ('workspace_hold_publish','free','active');`); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id,period,post_count) VALUES ('workspace_hold_publish',$1,$2)`, period, tc.usage); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO social_posts (id,workspace_id,status,scheduled_at,created_at,metadata,quota_hold_reason,quota_hold_at,quota_hold_original_scheduled_at) VALUES ('post_hold_publish','workspace_hold_publish','quota_hold',$1,$1,$2,'quota',$3,$1)`, scheduledAt, metadata, now); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/posts/post_hold_publish/publish", nil)
			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", "post_hold_publish")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace_hold_publish"))
			req = req.WithContext(withoutPostMediaRetentionSync(req.Context()))
			rr := httptest.NewRecorder()
			q := db.New(pool)
			h := NewSocialPostHandler(q, nil, quota.NewChecker(q), nil, nil, nil, nil).SetPaidScheduleCoordinator(paidquota.NewPostgresCoordinator(pool))
			h.PublishDraft(rr, req)
			if rr.Code != tc.wantCode {
				t.Fatalf("status=%d want=%d body=%s", rr.Code, tc.wantCode, rr.Body.String())
			}
			var status string
			var jobs int
			if err := pool.QueryRow(ctx, `SELECT status,(SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id='post_hold_publish') FROM social_posts WHERE id='post_hold_publish'`).Scan(&status, &jobs); err != nil {
				t.Fatal(err)
			}
			if status != tc.wantStatus || jobs != tc.wantJobs {
				t.Fatalf("outcome=%s/%d want=%s/%d", status, jobs, tc.wantStatus, tc.wantJobs)
			}
			reserved, err := q.CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_hold_publish", Period: period})
			wantReserved := int32(tc.wantJobs)
			if err != nil || reserved != wantReserved {
				t.Fatalf("execution reservation=%d err=%v want=%d", reserved, err, wantReserved)
			}
		})
	}
}

func TestConcurrentCrossPeriodQuotaHoldPublishReservesCurrentExecutionCapacity(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupScheduledQuotaSnapshotIntegrationSchema(t, pool)
	ctx := context.Background()
	now := time.Now().UTC()
	period := quota.PeriodForTime(now)
	priorScheduledAt := now.AddDate(0, -1, 0)
	if _, err := pool.Exec(ctx, `
		INSERT INTO profiles VALUES ('profile_hold_cross_concurrent','workspace_hold_cross_concurrent');
		INSERT INTO social_accounts (id,profile_id,platform) VALUES
		 ('account_hold_cross_1','profile_hold_cross_concurrent','linkedin'),
		 ('account_hold_cross_2','profile_hold_cross_concurrent','linkedin');
		INSERT INTO plans (id,name,price_cents,post_limit) VALUES ('free','Free',0,100);
		INSERT INTO subscriptions (workspace_id,plan_id,status) VALUES ('workspace_hold_cross_concurrent','free','active');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usage (workspace_id,period,post_count) VALUES ('workspace_hold_cross_concurrent',$1,99)`, period); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		metadata, _ := encodeScheduledPostMetadata([]platform.PlatformPostInput{{AccountID: fmt.Sprintf("account_hold_cross_%d", i), Caption: "publish hold"}}, 1)
		if _, err := pool.Exec(ctx, `INSERT INTO social_posts (id,workspace_id,status,scheduled_at,created_at,metadata,quota_hold_reason,quota_hold_at,quota_hold_original_scheduled_at) VALUES ($1,'workspace_hold_cross_concurrent','quota_hold',$2,$2,$3,'quota',$4,$2)`, fmt.Sprintf("post_hold_cross_%d", i), priorScheduledAt, metadata, now); err != nil {
			t.Fatal(err)
		}
	}

	statuses := make(chan int, 2)
	start := make(chan struct{})
	for i := 1; i <= 2; i++ {
		go func(i int) {
			<-start
			postID := fmt.Sprintf("post_hold_cross_%d", i)
			req := httptest.NewRequest(http.MethodPost, "/v1/posts/"+postID+"/publish", nil)
			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", postID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace_hold_cross_concurrent"))
			req = req.WithContext(withoutPostMediaRetentionSync(req.Context()))
			rr := httptest.NewRecorder()
			queries := db.New(pool)
			h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil).
				SetPaidScheduleCoordinator(paidquota.NewPostgresCoordinator(pool))
			h.PublishDraft(rr, req)
			statuses <- rr.Code
		}(i)
	}
	close(start)
	got := []int{<-statuses, <-statuses}
	slices.Sort(got)
	if !slices.Equal(got, []int{http.StatusAccepted, http.StatusPaymentRequired}) {
		t.Fatalf("cross-period quota-hold publish statuses=%v want [202 402]", got)
	}
	var publishing, held, jobs int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE status='publishing')::int, COUNT(*) FILTER (WHERE status='quota_hold')::int, (SELECT COUNT(*)::int FROM post_delivery_jobs) FROM social_posts WHERE workspace_id='workspace_hold_cross_concurrent'`).Scan(&publishing, &held, &jobs); err != nil {
		t.Fatal(err)
	}
	if publishing != 1 || held != 1 || jobs != 1 {
		t.Fatalf("cross-period quota-hold outcome=%d/%d/%d want 1/1/1", publishing, held, jobs)
	}
	reserved, err := db.New(pool).CountScheduledQuotaUnitsByWorkspaceAndPeriod(ctx, db.CountScheduledQuotaUnitsByWorkspaceAndPeriodParams{WorkspaceID: "workspace_hold_cross_concurrent", Period: period})
	if err != nil || reserved != 1 {
		t.Fatalf("cross-period quota-hold reservation=%d err=%v want 1", reserved, err)
	}
}

func setupScheduledQuotaSnapshotIntegrationSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx, `
		CREATE TABLE profiles (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL
		);
		CREATE TABLE social_accounts (
			id TEXT PRIMARY KEY,
			profile_id TEXT NOT NULL,
			platform TEXT NOT NULL DEFAULT 'instagram',
			access_token TEXT NOT NULL DEFAULT '',
			refresh_token TEXT,
			token_expires_at TIMESTAMPTZ,
			external_account_id TEXT NOT NULL DEFAULT '',
			account_name TEXT,
			account_avatar_url TEXT,
			connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			disconnected_at TIMESTAMPTZ,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			scope TEXT[] NOT NULL DEFAULT '{}',
			status TEXT NOT NULL DEFAULT 'active',
			connection_type TEXT NOT NULL DEFAULT 'byo',
			connect_session_id TEXT,
			external_user_id TEXT,
			external_user_email TEXT,
			last_refreshed_at TIMESTAMPTZ,
			x_app_mode TEXT,
			connection_id TEXT,
			binding_version BIGINT NOT NULL DEFAULT 1,
			binding_status TEXT NOT NULL DEFAULT 'active'
		);
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			caption TEXT,
			media_urls TEXT[] NOT NULL DEFAULT '{}',
			status TEXT NOT NULL,
			scheduled_at TIMESTAMPTZ,
			published_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			idempotency_key TEXT,
			workspace_id TEXT NOT NULL,
			archived_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			source TEXT NOT NULL DEFAULT 'api',
			profile_ids TEXT[] NOT NULL DEFAULT '{}',
			quota_hold_reason TEXT,
			quota_hold_at TIMESTAMPTZ,
			quota_hold_original_scheduled_at TIMESTAMPTZ
		);
		CREATE TABLE social_post_results (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			post_id TEXT NOT NULL,
			social_account_id TEXT NOT NULL,
			status TEXT NOT NULL,
			external_id TEXT,
			error_message TEXT,
			published_at TIMESTAMPTZ,
			caption TEXT NOT NULL DEFAULT '',
			url TEXT,
			debug_curl TEXT,
			fb_media_type TEXT,
			remotely_deleted_at TIMESTAMPTZ,
			error_code TEXT,
			failure_stage TEXT,
			platform_error_code TEXT,
			is_retriable BOOLEAN,
			next_action TEXT,
			error_source TEXT,
			error_temporality TEXT,
			provider_error JSONB,
			publish_token TEXT,
			x_credits_counted BIGINT NOT NULL DEFAULT 0,
			x_credit_operation TEXT,
			x_credit_catalog_version TEXT,
			x_credit_billing_mode TEXT,
			daily_reservation_operation_key TEXT,
			daily_reservation_release_pending BOOLEAN NOT NULL DEFAULT FALSE
		);
		CREATE TABLE post_delivery_jobs (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			post_id TEXT NOT NULL,
			social_post_result_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			social_account_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			post_input_index INTEGER NOT NULL DEFAULT 0,
			kind TEXT NOT NULL,
			state TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 5,
			failure_stage TEXT,
			error_code TEXT,
			platform_error_code TEXT,
			last_error TEXT,
			next_run_at TIMESTAMPTZ,
			last_attempt_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			finished_at TIMESTAMPTZ,
			dismissed_at TIMESTAMPTZ,
			lease_expires_at TIMESTAMPTZ,
			lease_owner TEXT,
			first_claimed_at TIMESTAMPTZ,
			platform_started_at TIMESTAMPTZ,
			connection_id TEXT,
			binding_version BIGINT
		);
		CREATE TABLE post_failures (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			post_id TEXT NOT NULL,
			social_post_result_id TEXT,
			workspace_id TEXT NOT NULL,
			social_account_id TEXT,
			platform TEXT NOT NULL,
			failure_stage TEXT NOT NULL,
			error_code TEXT NOT NULL,
			platform_error_code TEXT,
			message TEXT NOT NULL,
			raw_error TEXT,
			is_retriable BOOLEAN NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			error_source TEXT,
			error_temporality TEXT,
			provider_error JSONB,
			restriction_cycle_id TEXT
		);
		CREATE TABLE media_post_usages (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			workspace_id TEXT NOT NULL,
			media_id TEXT NOT NULL,
			post_id TEXT NOT NULL,
			post_status TEXT NOT NULL,
			cleanup_after_at TIMESTAMPTZ,
			retention_reason TEXT NOT NULL DEFAULT 'plan_status',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (media_id, post_id)
		);
		CREATE TABLE admin_post_quota_resets (
			workspace_id TEXT NOT NULL,
			period TEXT NOT NULL,
			quota_kind TEXT NOT NULL,
			reset_at TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE workspace_active_scheduled_limits (
			workspace_id TEXT PRIMARY KEY,
			limit_count INTEGER NOT NULL,
			expires_at TIMESTAMPTZ
		);
		CREATE TABLE plans (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			price_cents INTEGER NOT NULL,
			post_limit INTEGER NOT NULL,
			stripe_price_id TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			white_label BOOLEAN NOT NULL DEFAULT FALSE,
			allow_twitter BOOLEAN NOT NULL DEFAULT TRUE,
			allow_inbox BOOLEAN NOT NULL DEFAULT TRUE,
			allow_analytics BOOLEAN NOT NULL DEFAULT TRUE,
			max_profiles INTEGER,
			max_members INTEGER
		);
		CREATE TABLE subscriptions (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			plan_id TEXT NOT NULL,
			stripe_customer_id TEXT,
			stripe_subscription_id TEXT,
			status TEXT NOT NULL,
			current_period_start TIMESTAMPTZ,
			current_period_end TIMESTAMPTZ,
			cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			trial_used BOOLEAN NOT NULL DEFAULT FALSE,
			workspace_id TEXT NOT NULL UNIQUE
		);
		CREATE TABLE usage (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			period TEXT NOT NULL,
			post_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			workspace_id TEXT NOT NULL,
			UNIQUE (workspace_id, period)
		);
		CREATE UNIQUE INDEX social_posts_workspace_scheduled_idempotency_uniq
			ON social_posts (workspace_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL
			  AND status = 'scheduled';
		INSERT INTO profiles (id, workspace_id) VALUES ('profile_1', 'workspace_snapshot');
		INSERT INTO social_accounts (id, profile_id, platform) VALUES
			('account_tiktok', 'profile_1', 'tiktok'),
			('account_instagram', 'profile_1', 'instagram');
	`)
	if err != nil {
		t.Fatalf("setup scheduled quota snapshot schema: %v", err)
	}
	setupSocialConnectionPublishingFixtureSchema(t, pool)
}
