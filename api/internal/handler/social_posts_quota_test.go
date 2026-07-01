package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/quotaemail"
)

func TestCreateScheduledPostReturnsQuotaExceededWhenFreePlanCapIncludesReservations(t *testing.T) {
	dbtx := &scheduledQuotaHTTPTestDB{}
	handler := NewSocialPostHandler(db.New(dbtx), nil, quota.NewChecker(db.New(dbtx)), nil, nil, nil, nil)
	scheduledAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	body := strings.NewReader(fmt.Sprintf(`{
			"scheduled_at": %q,
			"platform_posts": [
				{
					"account_id": "acct_linkedin",
					"caption": "Codex regression coverage for free quota hard cap."
				}
			]
		}`, scheduledAt.Format(time.RFC3339)))
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", body)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402; body: %s", rr.Code, rr.Body.String())
	}
	if dbtx.createSocialPostCalls != 0 {
		t.Fatalf("CreateSocialPost calls = %d, want 0", dbtx.createSocialPostCalls)
	}
	if got := rr.Header().Get("X-UniPost-Usage"); got != "122/100" {
		t.Fatalf("X-UniPost-Usage = %q, want 122/100", got)
	}
	if got := rr.Header().Get("X-UniPost-Warning"); got != "over_limit" {
		t.Fatalf("X-UniPost-Warning = %q, want over_limit", got)
	}

	var envelope struct {
		Error struct {
			Code           string `json:"code"`
			NormalizedCode string `json:"normalized_code"`
			Message        string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Error.Code != "PLAN_POST_QUOTA_EXCEEDED" {
		t.Fatalf("error.code = %q", envelope.Error.Code)
	}
	if envelope.Error.NormalizedCode != "plan_post_quota_exceeded" {
		t.Fatalf("error.normalized_code = %q", envelope.Error.NormalizedCode)
	}
	for _, want := range []string{
		"122 of 100 posts",
		"1 scheduled post reserved",
		"needs 1 more",
	} {
		if !strings.Contains(envelope.Error.Message, want) {
			t.Fatalf("error.message = %q, want substring %q", envelope.Error.Message, want)
		}
	}
}

func TestCreateScheduledPostTriggersFreePlanQuotaEmailEvaluation(t *testing.T) {
	dbtx := &scheduledQuotaHTTPTestDB{
		usage:       79,
		reserved:    0,
		reservedSet: true,
		createPost:  true,
	}
	scheduledAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	dbtx.createdPost = scheduledQuotaCreatedPost(t, scheduledAt)
	quotaEmail := &recordingQuotaEmailService{}
	handler := NewSocialPostHandler(db.New(dbtx), nil, quota.NewChecker(db.New(dbtx)), nil, nil, nil, nil).
		SetQuotaEmailService(quotaEmail)
	body := strings.NewReader(fmt.Sprintf(`{
			"scheduled_at": %q,
			"platform_posts": [
				{
					"account_id": "acct_linkedin",
					"caption": "Codex regression coverage for free quota email trigger."
				}
			]
		}`, scheduledAt.Format(time.RFC3339)))
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", body)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}
	if dbtx.createSocialPostCalls != 1 {
		t.Fatalf("CreateSocialPost calls = %d, want 1", dbtx.createSocialPostCalls)
	}
	if len(quotaEmail.evals) != 1 {
		t.Fatalf("quota email evaluations = %d, want 1", len(quotaEmail.evals))
	}
	got := quotaEmail.evals[0]
	if got.WorkspaceID != "ws_1" {
		t.Fatalf("workspace id = %q, want ws_1", got.WorkspaceID)
	}
	if got.Period != scheduledAt.Format("2006-01") {
		t.Fatalf("period = %q, want %s", got.Period, scheduledAt.Format("2006-01"))
	}
	if got.Blocked {
		t.Fatal("blocked = true, want false for an accepted scheduled post")
	}
}

func TestEnqueueScheduledPostBlocksFreePlanQuotaAtExecution(t *testing.T) {
	dbtx := &scheduledExecutionQuotaTestDB{}
	handler := NewSocialPostHandler(db.New(dbtx), nil, quota.NewChecker(db.New(dbtx)), nil, nil, nil, nil)

	err := handler.EnqueueScheduledPost(context.Background(), scheduledExecutionQuotaPost(t))

	if err != nil {
		t.Fatalf("EnqueueScheduledPost returned error: %v", err)
	}
	if dbtx.deliveryJobCalls != 0 {
		t.Fatalf("CreatePostDeliveryJob calls = %d, want 0", dbtx.deliveryJobCalls)
	}
	if dbtx.updatedPostStatus != "failed" {
		t.Fatalf("updated post status = %q, want failed", dbtx.updatedPostStatus)
	}
	if len(dbtx.createdResultStatuses) != 2 {
		t.Fatalf("created results = %d, want 2", len(dbtx.createdResultStatuses))
	}
	for idx, status := range dbtx.createdResultStatuses {
		if status != "failed" {
			t.Fatalf("result[%d] status = %q, want failed", idx, status)
		}
	}
	if !strings.Contains(dbtx.errorSummary, "Free plan monthly post quota exceeded") {
		t.Fatalf("error summary = %q, want quota message", dbtx.errorSummary)
	}
	if dbtx.postFailureCalls != 2 {
		t.Fatalf("post failure calls = %d, want 2", dbtx.postFailureCalls)
	}
}

type scheduledQuotaHTTPTestDB struct {
	createSocialPostCalls int
	usage                 int32
	reserved              int32
	reservedSet           bool
	createPost            bool
	createdPost           db.SocialPost
}

func (f *scheduledQuotaHTTPTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *scheduledQuotaHTTPTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListSocialAccountsByWorkspace"):
		return &scheduledQuotaRows{values: [][]any{socialAccountValues(db.SocialAccount{
			ID:                "acct_linkedin",
			ProfileID:         "prof_1",
			Platform:          "linkedin",
			AccessToken:       "token",
			ExternalAccountID: "linkedin-page",
			AccountName:       pgtype.Text{String: "LinkedIn Page", Valid: true},
			ConnectedAt:       pgtype.Timestamptz{Time: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), Valid: true},
			Status:            "connected",
		})}}, nil
	default:
		return nil, errors.New("unexpected Query: " + query)
	}
}

func (f *scheduledQuotaHTTPTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		return scanRow{values: []any{
			"sub_1",
			"free",
			pgtype.Text{},
			pgtype.Text{},
			"active",
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			pgtype.Bool{},
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			false,
			"ws_1",
		}}
	case strings.Contains(query, "-- name: GetPlan"):
		return scanRow{values: []any{
			"free",
			"Free",
			int32(0),
			int32(100),
			pgtype.Text{},
			pgtype.Timestamptz{},
			false,
			false,
			false,
			false,
			pgtype.Int4{},
			pgtype.Int4{},
		}}
	case strings.Contains(query, "-- name: GetUsage"):
		usage := f.usage
		if usage == 0 {
			usage = 122
		}
		return scanRow{values: []any{
			"usage_1",
			args[1].(string),
			usage,
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			"ws_1",
		}}
	case strings.Contains(query, "-- name: GetScheduledSocialPostByIdempotencyKey"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "FROM social_posts sp") && strings.Contains(query, "sp.status = 'scheduled'"):
		reserved := f.reserved
		if !f.reservedSet {
			reserved = 1
		}
		return scanRow{values: []any{reserved}}
	case strings.Contains(query, "-- name: CreateSocialPost"):
		f.createSocialPostCalls++
		if f.createPost {
			return scheduledIdempotencySocialPostRow(f.createdPost)
		}
		return scanRow{err: errors.New("CreateSocialPost should not be called when free quota is exceeded")}
	default:
		return scanRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}

type recordingQuotaEmailService struct {
	evals []quotaemail.Evaluation
}

func (s *recordingQuotaEmailService) EvaluateAndSend(_ context.Context, eval quotaemail.Evaluation) error {
	s.evals = append(s.evals, eval)
	return nil
}

func scheduledQuotaCreatedPost(t *testing.T, scheduledAt time.Time) db.SocialPost {
	t.Helper()

	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: "acct_linkedin",
		Caption:   "Codex regression coverage for free quota email trigger.",
	}})
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}

	return db.SocialPost{
		ID:             "post_created",
		Caption:        pgtype.Text{String: "Codex regression coverage for free quota email trigger.", Valid: true},
		MediaUrls:      []string{},
		Status:         "scheduled",
		ScheduledAt:    pgtype.Timestamptz{Time: scheduledAt, Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC), Valid: true},
		Metadata:       metadata,
		IdempotencyKey: pgtype.Text{},
		WorkspaceID:    "ws_1",
		Source:         "api",
		ProfileIds:     []string{"prof_1"},
	}
}

type scheduledExecutionQuotaTestDB struct {
	createdResultStatuses []string
	deliveryJobCalls      int
	updatedPostStatus     string
	errorSummary          string
	postFailureCalls      int
}

func (f *scheduledExecutionQuotaTestDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: UpdateSocialPostStatus"):
		f.updatedPostStatus = args[1].(string)
	case strings.Contains(query, "-- name: UpdateSocialPostErrorMetadata"):
		f.errorSummary = args[1].(string)
	case strings.Contains(query, "-- name: UpdateSocialPostResultFailureDetails"):
		return pgconn.CommandTag{}, nil
	default:
		return pgconn.CommandTag{}, errors.New("unexpected Exec: " + query)
	}
	return pgconn.CommandTag{}, nil
}

func (f *scheduledExecutionQuotaTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (f *scheduledExecutionQuotaTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSocialAccountByIDAndWorkspace"):
		id := args[0].(string)
		return scanRow{values: socialAccountValues(db.SocialAccount{
			ID:                id,
			ProfileID:         "prof_1",
			Platform:          "youtube",
			AccessToken:       "token",
			ExternalAccountID: id + "_external",
			AccountName:       pgtype.Text{String: "YouTube Channel", Valid: true},
			ConnectedAt:       pgtype.Timestamptz{Time: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), Valid: true},
			Status:            "connected",
		})}
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		return scanRow{values: []any{
			"sub_1",
			"free",
			pgtype.Text{},
			pgtype.Text{},
			"active",
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			pgtype.Bool{},
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			false,
			"ws_1",
		}}
	case strings.Contains(query, "-- name: GetPlan"):
		return scanRow{values: []any{
			"free",
			"Free",
			int32(0),
			int32(100),
			pgtype.Text{},
			pgtype.Timestamptz{},
			false,
			false,
			false,
			false,
			pgtype.Int4{},
			pgtype.Int4{},
		}}
	case strings.Contains(query, "-- name: GetUsage"):
		return scanRow{values: []any{
			"usage_1",
			args[1].(string),
			int32(100),
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			"ws_1",
		}}
	case strings.Contains(query, "FROM social_posts sp") && strings.Contains(query, "sp.status = 'scheduled'"):
		return scanRow{values: []any{int32(0)}}
	case strings.Contains(query, "-- name: CreateSocialPostResult"):
		f.createdResultStatuses = append(f.createdResultStatuses, args[3].(string))
		id := fmt.Sprintf("result_%d", len(f.createdResultStatuses))
		return socialPostResultScanRow(db.SocialPostResult{
			ID:              id,
			PostID:          args[0].(string),
			SocialAccountID: args[1].(string),
			Caption:         args[2].(string),
			Status:          args[3].(string),
			ErrorMessage:    args[5].(pgtype.Text),
		})
	case strings.Contains(query, "-- name: CreatePostFailure"):
		f.postFailureCalls++
		return scanRow{values: []any{
			fmt.Sprintf("pf_%d", f.postFailureCalls),
			args[0].(string),
			args[1].(pgtype.Text),
			args[2].(string),
			args[3].(pgtype.Text),
			args[4].(string),
			args[5].(string),
			args[6].(string),
			args[7].(pgtype.Text),
			args[8].(string),
			args[9].(pgtype.Text),
			args[10].(bool),
			pgtype.Timestamptz{},
		}}
	case strings.Contains(query, "-- name: CreatePostDeliveryJob"):
		f.deliveryJobCalls++
		return scanRow{err: errors.New("delivery job should not be created when scheduled execution exceeds free quota")}
	default:
		return scanRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}

func scheduledExecutionQuotaPost(t *testing.T) db.SocialPost {
	t.Helper()
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{
		{AccountID: "acct_youtube_1", Caption: "Scheduled video one"},
		{AccountID: "acct_youtube_2", Caption: "Scheduled video two"},
	})
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}
	return db.SocialPost{
		ID:          "post_scheduled",
		Caption:     pgtype.Text{String: "Scheduled video one", Valid: true},
		Status:      "publishing",
		Metadata:    metadata,
		WorkspaceID: "ws_1",
		Source:      "api",
		ProfileIds:  []string{"prof_1"},
	}
}

func socialPostResultScanRow(result db.SocialPostResult) scanRow {
	return scanRow{values: []any{
		result.ID,
		result.PostID,
		result.SocialAccountID,
		result.Status,
		result.ExternalID,
		result.ErrorMessage,
		result.PublishedAt,
		result.Caption,
		result.Url,
		result.DebugCurl,
		result.FbMediaType,
		result.RemotelyDeletedAt,
		result.ErrorCode,
		result.FailureStage,
		result.PlatformErrorCode,
		result.IsRetriable,
		result.NextAction,
		result.ErrorSource,
		result.ErrorTemporality,
		result.ProviderError,
	}}
}

type scheduledQuotaRows struct {
	values [][]any
	index  int
}

func (r *scheduledQuotaRows) Close()                                       {}
func (r *scheduledQuotaRows) Err() error                                   { return nil }
func (r *scheduledQuotaRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *scheduledQuotaRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *scheduledQuotaRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}
func (r *scheduledQuotaRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.values) {
		return errors.New("Scan called without current row")
	}
	return scanRow{values: r.values[r.index-1]}.Scan(dest...)
}
func (r *scheduledQuotaRows) Values() ([]any, error) { return r.values[r.index-1], nil }
func (r *scheduledQuotaRows) RawValues() [][]byte    { return nil }
func (r *scheduledQuotaRows) Conn() *pgx.Conn        { return nil }

func socialAccountValues(a db.SocialAccount) []any {
	return []any{
		a.ID,
		a.ProfileID,
		a.Platform,
		a.AccessToken,
		a.RefreshToken,
		a.TokenExpiresAt,
		a.ExternalAccountID,
		a.AccountName,
		a.AccountAvatarUrl,
		a.ConnectedAt,
		a.DisconnectedAt,
		a.Metadata,
		a.Scope,
		a.Status,
		a.ConnectionType,
		a.ConnectSessionID,
		a.ExternalUserID,
		a.ExternalUserEmail,
		a.LastRefreshedAt,
	}
}
