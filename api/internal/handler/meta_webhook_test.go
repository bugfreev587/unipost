package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
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
)

func TestMetaWebhookVerify(t *testing.T) {
	h := NewMetaWebhookHandler(nil, nil, nil, "test-secret", "my-verify-token")

	t.Run("valid subscribe", func(t *testing.T) {
		req := httptest.NewRequest("GET",
			"/webhooks/meta?hub.mode=subscribe&hub.verify_token=my-verify-token&hub.challenge=challenge123",
			nil)
		rr := httptest.NewRecorder()
		h.Verify(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if body := rr.Body.String(); body != "challenge123" {
			t.Fatalf("expected challenge echoed back, got %q", body)
		}
	})

	t.Run("wrong verify token", func(t *testing.T) {
		req := httptest.NewRequest("GET",
			"/webhooks/meta?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=c",
			nil)
		rr := httptest.NewRecorder()
		h.Verify(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("wrong mode", func(t *testing.T) {
		req := httptest.NewRequest("GET",
			"/webhooks/meta?hub.mode=unsubscribe&hub.verify_token=my-verify-token&hub.challenge=c",
			nil)
		rr := httptest.NewRecorder()
		h.Verify(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})
}

func TestMetaWebhookHandle(t *testing.T) {
	appSecret := "test-app-secret"
	h := NewMetaWebhookHandler(nil, nil, nil, appSecret, "tok")

	sign := func(body string) string {
		mac := hmac.New(sha256.New, []byte(appSecret))
		mac.Write([]byte(body))
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	t.Run("valid payload", func(t *testing.T) {
		body := `{"object":"page","entry":[{"id":"123","time":1700000000}]}`
		req := httptest.NewRequest("POST", "/webhooks/meta", strings.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", sign(body))
		rr := httptest.NewRecorder()
		h.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("bad signature proceeds with warning", func(t *testing.T) {
		body := `{"object":"instagram","entry":[]}`
		req := httptest.NewRequest("POST", "/webhooks/meta", strings.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", "sha256=0000000000000000000000000000000000000000000000000000000000000000")
		rr := httptest.NewRecorder()
		h.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 (signature mismatch is non-blocking), got %d", rr.Code)
		}
	})

	t.Run("missing signature proceeds with warning", func(t *testing.T) {
		body := `{"object":"instagram","entry":[]}`
		req := httptest.NewRequest("POST", "/webhooks/meta", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 (signature mismatch is non-blocking), got %d", rr.Code)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		unconfigured := NewMetaWebhookHandler(nil, nil, nil, "", "tok")
		body := `{"object":"instagram","entry":[]}`
		req := httptest.NewRequest("POST", "/webhooks/meta", strings.NewReader(body))
		rr := httptest.NewRecorder()
		unconfigured.Handle(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rr.Code)
		}
	})
}

func TestMetaWebhookInstagramRoutingUsesOnlyExactWebhookIdentityMatches(t *testing.T) {
	database := &metaWebhookRoutingDB{
		instagram: map[string][]metaWebhookRoutingAccount{
			"webhook_user_1": {
				{id: "sa_1", externalAccountID: "app_scoped_1", webhookAccountID: "webhook_user_1", workspaceID: "ws_1"},
				{id: "sa_2", externalAccountID: "app_scoped_2", webhookAccountID: "webhook_user_1", workspaceID: "ws_2"},
			},
		},
	}
	handler, notifications := newMetaWebhookRoutingHandler(database)

	entry := decodeMetaWebhookRoutingEntry(t, `{
		"id":"webhook_user_1",
		"changes":[{"field":"comments","value":{"id":"comment_1","text":"comment body","media":{"id":"media_1"},"from":{"id":"webhook_user_1","username":"owner"},"timestamp":1700000000}}],
		"messaging":[{"sender":{"id":"webhook_user_1"},"recipient":{"id":"recipient_1"},"timestamp":1700000000,"message":{"mid":"dm_1","text":"dm body"}}]
	}`)
	handler.handleInstagramEntry(httptest.NewRequest(http.MethodPost, "/webhooks/meta", nil), entry)

	if len(database.upserts) != 4 {
		t.Fatalf("upserts = %d, want 4 (comment and DM for two exact matches)", len(database.upserts))
	}
	if len(*notifications) != 4 {
		t.Fatalf("notifications = %d, want 4", len(*notifications))
	}
	assertMetaWebhookRoutingAccountCounts(t, database.upserts, map[string]int{"sa_1": 2, "sa_2": 2})
	for _, upsert := range database.upserts {
		if !upsert.IsOwn {
			t.Errorf("%s for %s should compare ownership against mapped webhook identity", upsert.Source, upsert.SocialAccountID)
		}
	}
}

func TestMetaWebhookInstagramRoutingFailsClosedForUnmatchedOrErroredIdentity(t *testing.T) {
	for _, test := range []struct {
		name     string
		queryErr error
	}{
		{name: "unmatched"},
		{name: "query error", queryErr: errors.New("database unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			database := &metaWebhookRoutingDB{queryErr: test.queryErr}
			handler, notifications := newMetaWebhookRoutingHandler(database)
			entry := decodeMetaWebhookRoutingEntry(t, `{"id":"unknown_user","changes":[{"field":"comments","value":{"id":"comment_1","text":"private"}}]}`)

			handler.handleInstagramEntry(httptest.NewRequest(http.MethodPost, "/webhooks/meta", nil), entry)

			if len(database.upserts) != 0 || len(*notifications) != 0 {
				t.Fatalf("unmatched/error route wrote %d items and sent %d notifications", len(database.upserts), len(*notifications))
			}
		})
	}
}

func TestMetaWebhookThreadsAndFacebookRoutingUsesOnlyExactDuplicateMatches(t *testing.T) {
	database := &metaWebhookRoutingDB{
		external: map[string][]metaWebhookRoutingAccount{
			"threads:threads_user_1": {
				{id: "threads_sa_1", externalAccountID: "threads_user_1", workspaceID: "ws_1"},
				{id: "threads_sa_2", externalAccountID: "threads_user_1", workspaceID: "ws_2"},
			},
			"facebook:page_1": {
				{id: "facebook_sa_1", externalAccountID: "page_1", workspaceID: "ws_1"},
				{id: "facebook_sa_2", externalAccountID: "page_1", workspaceID: "ws_2"},
			},
		},
	}
	handler, notifications := newMetaWebhookRoutingHandler(database)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/meta", nil)

	handler.handleThreadsEntry(req, decodeMetaWebhookRoutingEntry(t, `{
		"id":"threads_user_1",
		"changes":[{"field":"replies","value":{"id":"reply_1","text":"reply body","media_id":"post_1","from":{"id":"reader_1","username":"reader"},"timestamp":1700000000}}]
	}`))
	handler.handleFacebookEntry(req, decodeMetaWebhookRoutingEntry(t, `{
		"id":"page_1",
		"changes":[{"field":"feed","value":{"item":"comment","verb":"add","comment_id":"fb_comment_1","post_id":"fb_post_1","from":{"id":"reader_2","name":"Reader"},"message":"facebook body","created_time":1700000000}}]
	}`))

	if len(database.upserts) != 4 {
		t.Fatalf("upserts = %d, want 4", len(database.upserts))
	}
	if len(*notifications) != 4 {
		t.Fatalf("notifications = %d, want 4", len(*notifications))
	}
	assertMetaWebhookRoutingAccountCounts(t, database.upserts, map[string]int{
		"threads_sa_1":  1,
		"threads_sa_2":  1,
		"facebook_sa_1": 1,
		"facebook_sa_2": 1,
	})
}

func TestMetaWebhookThreadsAndFacebookRoutingFailsClosedWhenUnmatched(t *testing.T) {
	database := &metaWebhookRoutingDB{}
	handler, notifications := newMetaWebhookRoutingHandler(database)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/meta", nil)

	handler.handleThreadsEntry(req, decodeMetaWebhookRoutingEntry(t, `{"id":"unknown_threads","changes":[{"field":"replies","value":{"id":"reply_1","text":"private"}}]}`))
	handler.handleFacebookEntry(req, decodeMetaWebhookRoutingEntry(t, `{"id":"unknown_page","changes":[{"field":"feed","value":{"item":"comment","verb":"add","comment_id":"comment_1","message":"private"}}]}`))

	if len(database.upserts) != 0 || len(*notifications) != 0 {
		t.Fatalf("unmatched routes wrote %d items and sent %d notifications", len(database.upserts), len(*notifications))
	}
}

func TestMetaWebhookInstagramRoutingLogsDoNotContainContent(t *testing.T) {
	const privateText = "private-comment-content-7f1e"
	database := &metaWebhookRoutingDB{
		instagram: map[string][]metaWebhookRoutingAccount{
			"webhook_user_1": {{id: "sa_1", externalAccountID: "app_scoped_1", webhookAccountID: "webhook_user_1", workspaceID: "ws_1"}},
		},
	}
	handler, _ := newMetaWebhookRoutingHandler(database)
	entry := decodeMetaWebhookRoutingEntry(t, `{"id":"webhook_user_1","changes":[{"field":"comments","value":{"id":"comment_1","text":"`+privateText+`","from":{"id":"reader_1","username":"private-author"}}}]}`)

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	handler.handleInstagramEntry(httptest.NewRequest(http.MethodPost, "/webhooks/meta", nil), entry)

	if strings.Contains(logs.String(), privateText) || strings.Contains(logs.String(), "private-author") {
		t.Fatalf("structured logs contain inbox content: %s", logs.String())
	}
}

func newMetaWebhookRoutingHandler(database *metaWebhookRoutingDB) (*MetaWebhookHandler, *[]string) {
	handler := NewMetaWebhookHandler(db.New(database), nil, nil, "secret", "verify")
	notifications := &[]string{}
	handler.notify = func(_ context.Context, workspaceID string, _ any) {
		*notifications = append(*notifications, workspaceID)
	}
	return handler, notifications
}

func decodeMetaWebhookRoutingEntry(t *testing.T, payload string) metaWebhookEntry {
	t.Helper()
	var entry metaWebhookEntry
	if err := json.Unmarshal([]byte(payload), &entry); err != nil {
		t.Fatalf("decode entry: %v", err)
	}
	return entry
}

func assertMetaWebhookRoutingAccountCounts(t *testing.T, upserts []db.UpsertInboxItemParams, want map[string]int) {
	t.Helper()
	got := make(map[string]int)
	for _, upsert := range upserts {
		got[upsert.SocialAccountID]++
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routed account counts = %#v, want %#v", got, want)
	}
}

type metaWebhookRoutingAccount struct {
	id                string
	externalAccountID string
	webhookAccountID  string
	workspaceID       string
}

type metaWebhookRoutingDB struct {
	instagram map[string][]metaWebhookRoutingAccount
	external  map[string][]metaWebhookRoutingAccount
	queryErr  error
	upserts   []db.UpsertInboxItemParams
}

func (f *metaWebhookRoutingDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec call")
}

func (f *metaWebhookRoutingDB) Query(_ context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	var accounts []metaWebhookRoutingAccount
	switch {
	case strings.Contains(query, "-- name: FindAllActiveInstagramAccountsByWebhookUserID"):
		accounts = f.instagram[args[0].(string)]
		values := make([][]any, 0, len(accounts))
		for _, account := range accounts {
			values = append(values, []any{account.id, account.externalAccountID, account.webhookAccountID, account.workspaceID})
		}
		return &metaWebhookRoutingRows{values: values}, nil
	case strings.Contains(query, "-- name: FindAllSocialAccountsByPlatformAndExternalID"):
		accounts = f.external[args[0].(string)+":"+args[1].(string)]
		values := make([][]any, 0, len(accounts))
		for _, account := range accounts {
			values = append(values, []any{account.id, account.externalAccountID, account.workspaceID})
		}
		return &metaWebhookRoutingRows{values: values}, nil
	default:
		return nil, errors.New("unexpected Query call")
	}
}

func (f *metaWebhookRoutingDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	if !strings.Contains(query, "-- name: UpsertInboxItem") {
		return metaWebhookRoutingRow{err: pgx.ErrNoRows}
	}
	params := db.UpsertInboxItemParams{
		SocialAccountID:  args[0].(string),
		WorkspaceID:      args[1].(string),
		Source:           args[2].(string),
		ExternalID:       args[3].(string),
		ParentExternalID: args[4].(pgtype.Text),
		AuthorName:       args[5].(pgtype.Text),
		AuthorID:         args[6].(pgtype.Text),
		AuthorAvatarUrl:  args[7].(pgtype.Text),
		Body:             args[8].(pgtype.Text),
		IsOwn:            args[9].(bool),
		ReceivedAt:       args[10].(pgtype.Timestamptz),
		Metadata:         args[11].([]byte),
		ThreadKey:        args[12].(string),
		ThreadStatus:     args[13].(string),
		AssignedTo:       args[14].(pgtype.Text),
		LinkedPostID:     args[15].(pgtype.Text),
	}
	f.upserts = append(f.upserts, params)
	createdAt := pgtype.Timestamptz{Time: time.Unix(1700000000, 0), Valid: true}
	return metaWebhookRoutingRow{values: []any{
		"item_" + params.SocialAccountID + "_" + params.ExternalID,
		params.SocialAccountID,
		params.WorkspaceID,
		params.Source,
		params.ExternalID,
		params.ParentExternalID,
		params.AuthorName,
		params.AuthorID,
		params.AuthorAvatarUrl,
		params.Body,
		false,
		params.IsOwn,
		params.ReceivedAt,
		createdAt,
		params.Metadata,
		params.ThreadKey,
		params.ThreadStatus,
		params.AssignedTo,
		params.LinkedPostID,
	}}
}

type metaWebhookRoutingRow struct {
	values []any
	err    error
}

func (r metaWebhookRoutingRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	return scanMetaWebhookRoutingValues(dest, r.values)
}

type metaWebhookRoutingRows struct {
	values [][]any
	index  int
}

func (r *metaWebhookRoutingRows) Close()                                       {}
func (r *metaWebhookRoutingRows) Err() error                                   { return nil }
func (r *metaWebhookRoutingRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *metaWebhookRoutingRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *metaWebhookRoutingRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}
func (r *metaWebhookRoutingRows) Scan(dest ...interface{}) error {
	if r.index == 0 || r.index > len(r.values) {
		return errors.New("Scan called without current row")
	}
	return scanMetaWebhookRoutingValues(dest, r.values[r.index-1])
}
func (r *metaWebhookRoutingRows) Values() ([]interface{}, error) { return r.values[r.index-1], nil }
func (r *metaWebhookRoutingRows) RawValues() [][]byte            { return nil }
func (r *metaWebhookRoutingRows) Conn() *pgx.Conn                { return nil }

func scanMetaWebhookRoutingValues(dest []interface{}, values []any) error {
	if len(dest) != len(values) {
		return errors.New("scan destination count mismatch")
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return errors.New("scan destination is not a pointer")
		}
		value := reflect.ValueOf(values[i])
		if !value.IsValid() || !value.Type().AssignableTo(target.Elem().Type()) {
			return errors.New("scan value type mismatch")
		}
		target.Elem().Set(value)
	}
	return nil
}
