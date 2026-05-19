package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestScheduledIdempotencyPayloadHashIgnoresPostOrder(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{
		{AccountID: "sa_threads", Caption: "Launching today", MediaIDs: []string{"media_1"}},
		{AccountID: "sa_linkedin", Caption: "Launching today", MediaURLs: []string{"https://cdn.example.com/a.jpg"}},
	}
	reordered := []platform.PlatformPostInput{posts[1], posts[0]}

	first, err := scheduledIdempotencyPayloadHash(posts, scheduledAt)
	if err != nil {
		t.Fatalf("hash posts: %v", err)
	}
	second, err := scheduledIdempotencyPayloadHash(reordered, scheduledAt)
	if err != nil {
		t.Fatalf("hash reordered posts: %v", err)
	}
	if first != second {
		t.Fatalf("expected same hash for reordered posts, got %q and %q", first, second)
	}
}

func TestScheduledIdempotencyPayloadHashIncludesScheduledAt(t *testing.T) {
	posts := []platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching today"}}

	first, err := scheduledIdempotencyPayloadHash(posts, time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("hash first time: %v", err)
	}
	second, err := scheduledIdempotencyPayloadHash(posts, time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("hash second time: %v", err)
	}
	if first == second {
		t.Fatal("expected different hash when scheduled_at changes")
	}
}

func TestScheduledIdempotencyPayloadHashIncludesCaption(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)

	first, err := scheduledIdempotencyPayloadHash([]platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching today"}}, scheduledAt)
	if err != nil {
		t.Fatalf("hash first caption: %v", err)
	}
	second, err := scheduledIdempotencyPayloadHash([]platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching tomorrow"}}, scheduledAt)
	if err != nil {
		t.Fatalf("hash second caption: %v", err)
	}
	if first == second {
		t.Fatal("expected different hash when payload changes")
	}
}

func TestScheduledIdempotencyPayloadHashMatchesMetadataRoundTrip(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{
		{
			AccountID:       "sa_threads",
			Caption:         "Launching today",
			MediaIDs:        []string{"media_1"},
			PlatformOptions: map[string]any{"privacy": "public", "allow_comments": true},
			ThreadPosition:  1,
			FirstComment:    "First comment",
		},
		{
			AccountID:       "sa_linkedin",
			Caption:         "Launching today on LinkedIn",
			MediaURLs:       []string{"https://cdn.example.com/a.jpg"},
			PlatformOptions: map[string]any{"visibility": "PUBLIC"},
		},
	}

	metadata, err := platform.EncodePostMetadata(posts)
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}
	decoded, err := platform.DecodePostMetadata(metadata, "")
	if err != nil {
		t.Fatalf("decode metadata: %v", err)
	}

	originalHash, err := scheduledIdempotencyPayloadHash(posts, scheduledAt)
	if err != nil {
		t.Fatalf("hash original posts: %v", err)
	}
	decodedHash, err := scheduledIdempotencyPayloadHash(decoded, scheduledAt)
	if err != nil {
		t.Fatalf("hash decoded posts: %v", err)
	}
	if originalHash != decodedHash {
		t.Fatalf("expected metadata round trip to preserve hash, got %q and %q", originalHash, decodedHash)
	}
}

func TestMaybeReplayScheduledIdempotencyWritesCreatedReplay(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching today"}}
	existing := scheduledIdempotencyExistingPost(t, posts, scheduledAt)
	handler := &SocialPostHandler{queries: db.New(&scheduledIdempotencyTestDB{existing: existing})}

	rr := httptest.NewRecorder()
	handled := handler.maybeReplayScheduledIdempotency(rr, httptest.NewRequest(http.MethodPost, "/v1/social-posts", nil), "ws_1", scheduledIdempotencyParsed(posts, scheduledAt))

	if !handled {
		t.Fatal("expected scheduled idempotency replay to handle the response")
	}
	assertScheduledReplayResponse(t, rr, existing.ID)
}

func TestMaybeReplayScheduledIdempotencyPayloadConflict(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	existingPosts := []platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching today"}}
	requestPosts := []platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching tomorrow"}}
	existing := scheduledIdempotencyExistingPost(t, existingPosts, scheduledAt)
	handler := &SocialPostHandler{queries: db.New(&scheduledIdempotencyTestDB{existing: existing})}

	rr := httptest.NewRecorder()
	handled := handler.maybeReplayScheduledIdempotency(rr, httptest.NewRequest(http.MethodPost, "/v1/social-posts", nil), "ws_1", scheduledIdempotencyParsed(requestPosts, scheduledAt))

	if !handled {
		t.Fatal("expected scheduled idempotency conflict to handle the response")
	}
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
	var envelope ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if envelope.Error.Code != "IDEMPOTENCY_KEY_CONFLICT" {
		t.Fatalf("error code = %q, want IDEMPOTENCY_KEY_CONFLICT", envelope.Error.Code)
	}
}

func TestCreateScheduledPostRecoversUniqueViolationWithReplay(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{{AccountID: "sa_threads", Caption: "Launching today"}}
	existing := scheduledIdempotencyExistingPost(t, posts, scheduledAt)
	dbtx := &scheduledIdempotencyTestDB{
		existing: existing,
		createErr: &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "social_posts_workspace_scheduled_idempotency_uniq",
		},
	}
	handler := &SocialPostHandler{queries: db.New(dbtx)}

	rr := httptest.NewRecorder()
	handler.createScheduledPost(rr, httptest.NewRequest(http.MethodPost, "/v1/social-posts", nil), "ws_1", scheduledIdempotencyParsed(posts, scheduledAt))

	if dbtx.createCalls != 1 {
		t.Fatalf("CreateSocialPost calls = %d, want 1", dbtx.createCalls)
	}
	if dbtx.getScheduledCalls != 1 {
		t.Fatalf("GetScheduledSocialPostByIdempotencyKey calls = %d, want 1", dbtx.getScheduledCalls)
	}
	assertScheduledReplayResponse(t, rr, existing.ID)
}

func TestApplyIdempotencyKeyHeaderFallback(t *testing.T) {
	t.Run("uses header when body omitted key", func(t *testing.T) {
		parsed := parsedRequest{}

		applyIdempotencyKeyHeaderFallback(&parsed, " sdk-key-001 ")

		if parsed.IdempotencyKey != "sdk-key-001" {
			t.Fatalf("idempotency key = %q, want sdk-key-001", parsed.IdempotencyKey)
		}
	})

	t.Run("body key wins over header", func(t *testing.T) {
		parsed := parsedRequest{IdempotencyKey: "body-key"}

		applyIdempotencyKeyHeaderFallback(&parsed, "header-key")

		if parsed.IdempotencyKey != "body-key" {
			t.Fatalf("idempotency key = %q, want body-key", parsed.IdempotencyKey)
		}
	})

	t.Run("empty header leaves key empty", func(t *testing.T) {
		parsed := parsedRequest{}

		applyIdempotencyKeyHeaderFallback(&parsed, " ")

		if parsed.IdempotencyKey != "" {
			t.Fatalf("idempotency key = %q, want empty", parsed.IdempotencyKey)
		}
	})
}

func scheduledIdempotencyParsed(posts []platform.PlatformPostInput, scheduledAt time.Time) parsedRequest {
	t := scheduledAt
	return parsedRequest{
		Posts:          posts,
		ScheduledAt:    &t,
		IdempotencyKey: "idem_1",
	}
}

func scheduledIdempotencyExistingPost(t *testing.T, posts []platform.PlatformPostInput, scheduledAt time.Time) db.SocialPost {
	t.Helper()

	metadata, err := platform.EncodePostMetadata(posts)
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}

	caption := pgtype.Text{}
	mediaURLs := []string{}
	if len(posts) > 0 {
		if posts[0].Caption != "" {
			caption = pgtype.Text{String: posts[0].Caption, Valid: true}
		}
		mediaURLs = posts[0].MediaURLs
	}
	if mediaURLs == nil {
		mediaURLs = []string{}
	}

	return db.SocialPost{
		ID:             "post_existing",
		Caption:        caption,
		MediaUrls:      mediaURLs,
		Status:         "scheduled",
		ScheduledAt:    pgtype.Timestamptz{Time: scheduledAt, Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: scheduledAt.Add(-time.Minute), Valid: true},
		Metadata:       metadata,
		IdempotencyKey: pgtype.Text{String: "idem_1", Valid: true},
		WorkspaceID:    "ws_1",
		Source:         "api",
		ProfileIds:     []string{"profile_1"},
	}
}

func assertScheduledReplayResponse(t *testing.T, rr *httptest.ResponseRecorder, wantID string) {
	t.Helper()

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if got := rr.Header().Get("X-UniPost-Idempotent-Replay"); got != "true" {
		t.Fatalf("X-UniPost-Idempotent-Replay = %q, want true", got)
	}
	if got := rr.Header().Get("Idempotent-Replay"); got != "true" {
		t.Fatalf("Idempotent-Replay = %q, want true", got)
	}

	var envelope struct {
		Data socialPostResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if envelope.Data.ID != wantID {
		t.Fatalf("post id = %q, want %q", envelope.Data.ID, wantID)
	}
	if !envelope.Data.IdempotencyReplay {
		t.Fatal("expected idempotency_replay=true")
	}
}

type scheduledIdempotencyTestDB struct {
	existing          db.SocialPost
	existingErr       error
	createErr         error
	createCalls       int
	getScheduledCalls int
}

func (f *scheduledIdempotencyTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *scheduledIdempotencyTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return emptyScheduledIdempotencyRows{}, nil
}

func (f *scheduledIdempotencyTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: CreateSocialPost"):
		f.createCalls++
		if f.createErr != nil {
			return scheduledIdempotencyRow{err: f.createErr}
		}
		return socialPostRowFromCreateArgs(args)
	case strings.Contains(query, "-- name: GetScheduledSocialPostByIdempotencyKey"):
		f.getScheduledCalls++
		if f.existingErr != nil {
			return scheduledIdempotencyRow{err: f.existingErr}
		}
		return scheduledIdempotencySocialPostRow(f.existing)
	default:
		return scheduledIdempotencyRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
}

func socialPostRowFromCreateArgs(args []interface{}) pgx.Row {
	post := db.SocialPost{
		ID:             "post_created",
		WorkspaceID:    args[0].(string),
		Caption:        args[1].(pgtype.Text),
		MediaUrls:      args[2].([]string),
		Status:         args[3].(string),
		Metadata:       args[4].([]byte),
		ScheduledAt:    args[5].(pgtype.Timestamptz),
		IdempotencyKey: args[6].(pgtype.Text),
		Source:         args[7].(string),
		ProfileIds:     args[8].([]string),
		CreatedAt:      pgtype.Timestamptz{Time: time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC), Valid: true},
	}
	return scheduledIdempotencySocialPostRow(post)
}

func scheduledIdempotencySocialPostRow(post db.SocialPost) pgx.Row {
	return scheduledIdempotencyRow{values: []any{
		post.ID,
		post.Caption,
		post.MediaUrls,
		post.Status,
		post.ScheduledAt,
		post.PublishedAt,
		post.CreatedAt,
		post.Metadata,
		post.IdempotencyKey,
		post.WorkspaceID,
		post.ArchivedAt,
		post.DeletedAt,
		post.Source,
		post.ProfileIds,
	}}
}

type scheduledIdempotencyRow struct {
	values []any
	err    error
}

func (r scheduledIdempotencyRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan destination count = %d, values = %d", len(dest), len(r.values))
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Ptr || target.IsNil() {
			return fmt.Errorf("scan destination %d is not a non-nil pointer", i)
		}
		value := reflect.ValueOf(r.values[i])
		if !value.Type().AssignableTo(target.Elem().Type()) {
			return fmt.Errorf("scan value %d has type %s, want %s", i, value.Type(), target.Elem().Type())
		}
		target.Elem().Set(value)
	}
	return nil
}

type emptyScheduledIdempotencyRows struct{}

func (emptyScheduledIdempotencyRows) Close()                                       {}
func (emptyScheduledIdempotencyRows) Err() error                                   { return nil }
func (emptyScheduledIdempotencyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (emptyScheduledIdempotencyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (emptyScheduledIdempotencyRows) Next() bool                                   { return false }
func (emptyScheduledIdempotencyRows) Scan(...any) error                            { return nil }
func (emptyScheduledIdempotencyRows) Values() ([]any, error)                       { return nil, nil }
func (emptyScheduledIdempotencyRows) RawValues() [][]byte                          { return nil }
func (emptyScheduledIdempotencyRows) Conn() *pgx.Conn                              { return nil }
