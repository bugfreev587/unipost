package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestManagedUsersProfileScopeErrors(t *testing.T) {
	tests := []struct {
		name           string
		workspaceID    string
		profileExists  bool
		profileWS      string
		wantStatus     int
		wantCode       string
		wantNormalized string
	}{
		{
			name:           "missing authentication context stays unauthorized",
			wantStatus:     http.StatusUnauthorized,
			wantCode:       "UNAUTHORIZED",
			wantNormalized: "unauthorized",
		},
		{
			name:           "missing profile is concealed as inaccessible",
			workspaceID:    "ws_1",
			wantStatus:     http.StatusNotFound,
			wantCode:       "PROFILE_INACCESSIBLE",
			wantNormalized: "profile_inaccessible",
		},
		{
			name:           "profile from another workspace is concealed as inaccessible",
			workspaceID:    "ws_1",
			profileExists:  true,
			profileWS:      "ws_other",
			wantStatus:     http.StatusNotFound,
			wantCode:       "PROFILE_INACCESSIBLE",
			wantNormalized: "profile_inaccessible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &managedUsersTestDB{
				profileExists:      tt.profileExists,
				profileWorkspaceID: tt.profileWS,
			}
			h := NewManagedUsersHandler(db.New(store))
			req := managedUsersRequest(http.MethodGet, "/v1/profiles/prof_hidden/users", map[string]string{
				"profileID": "prof_hidden",
			})
			if tt.workspaceID != "" {
				req = req.WithContext(auth.SetWorkspaceID(req.Context(), tt.workspaceID))
			}
			rec := httptest.NewRecorder()

			h.List(rec, req)

			assertManagedUsersError(t, rec, tt.wantStatus, tt.wantCode, tt.wantNormalized)
			if store.managedUsersQueryCalls != 0 {
				t.Fatalf("managed users query calls = %d, want 0", store.managedUsersQueryCalls)
			}
		})
	}
}

func TestManagedUsersGetMissingUserReturnsStableCode(t *testing.T) {
	store := &managedUsersTestDB{
		profileExists:      true,
		profileWorkspaceID: "ws_1",
	}
	h := NewManagedUsersHandler(db.New(store))
	req := managedUsersRequest(http.MethodGet, "/v1/profiles/prof_1/users/missing", map[string]string{
		"profileID":        "prof_1",
		"external_user_id": "missing",
	})
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Get(rec, req)

	assertManagedUsersError(
		t,
		rec,
		http.StatusNotFound,
		"MANAGED_USER_NOT_FOUND",
		"managed_user_not_found",
	)
	if store.managedUsersQueryCalls != 1 {
		t.Fatalf("managed users query calls = %d, want 1", store.managedUsersQueryCalls)
	}
}

func TestManagedUsersDismissUsesScopedErrorContract(t *testing.T) {
	t.Run("inaccessible profile", func(t *testing.T) {
		store := &managedUsersTestDB{}
		h := NewManagedUsersHandler(db.New(store))
		req := managedUsersRequest(http.MethodPost, "/v1/profiles/prof_hidden/users/user_1/dismiss", map[string]string{
			"profileID":        "prof_hidden",
			"external_user_id": "user_1",
		})
		req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
		rec := httptest.NewRecorder()

		h.DismissDisconnected(rec, req)

		assertManagedUsersError(
			t,
			rec,
			http.StatusNotFound,
			"PROFILE_INACCESSIBLE",
			"profile_inaccessible",
		)
		if store.dismissCalls != 0 {
			t.Fatalf("dismiss calls = %d, want 0", store.dismissCalls)
		}
	})

	t.Run("missing managed user", func(t *testing.T) {
		store := &managedUsersTestDB{
			profileExists:      true,
			profileWorkspaceID: "ws_1",
		}
		h := NewManagedUsersHandler(db.New(store))
		req := managedUsersRequest(http.MethodPost, "/v1/profiles/prof_1/users/missing/dismiss", map[string]string{
			"profileID":        "prof_1",
			"external_user_id": "missing",
		})
		req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
		rec := httptest.NewRecorder()

		h.DismissDisconnected(rec, req)

		assertManagedUsersError(
			t,
			rec,
			http.StatusNotFound,
			"MANAGED_USER_NOT_FOUND",
			"managed_user_not_found",
		)
		if store.dismissCalls != 1 {
			t.Fatalf("dismiss calls = %d, want 1", store.dismissCalls)
		}
	})
}

func TestManagedUsersUnexpectedProfileLookupErrorsReturnInternalError(t *testing.T) {
	lookupErr := errors.New("profile database unavailable")
	tests := []struct {
		name   string
		method string
		target string
		params map[string]string
		invoke func(*ManagedUsersHandler, http.ResponseWriter, *http.Request)
	}{
		{
			name:   "list",
			method: http.MethodGet,
			target: "/v1/profiles/prof_1/users",
			params: map[string]string{"profileID": "prof_1"},
			invoke: (*ManagedUsersHandler).List,
		},
		{
			name:   "get",
			method: http.MethodGet,
			target: "/v1/profiles/prof_1/users/user_1",
			params: map[string]string{"profileID": "prof_1", "external_user_id": "user_1"},
			invoke: (*ManagedUsersHandler).Get,
		},
		{
			name:   "dismiss",
			method: http.MethodPost,
			target: "/v1/profiles/prof_1/users/user_1/dismiss",
			params: map[string]string{"profileID": "prof_1", "external_user_id": "user_1"},
			invoke: (*ManagedUsersHandler).DismissDisconnected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &managedUsersTestDB{profileErr: lookupErr}
			h := NewManagedUsersHandler(db.New(store))
			req := managedUsersRequest(tt.method, tt.target, tt.params)
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
			rec := httptest.NewRecorder()

			tt.invoke(h, rec, req)

			assertManagedUsersError(
				t,
				rec,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"internal_error",
			)
			if store.managedUsersQueryCalls != 0 || store.dismissCalls != 0 {
				t.Fatalf(
					"downstream calls = (queries %d, dismiss %d), want zero",
					store.managedUsersQueryCalls,
					store.dismissCalls,
				)
			}
		})
	}
}

func TestManagedUsersLegacyRoutesKeepNotFoundCode(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		params map[string]string
		invoke func(*ManagedUsersHandler, http.ResponseWriter, *http.Request)
	}{
		{
			name:   "get",
			method: http.MethodGet,
			target: "/v1/users/missing",
			params: map[string]string{"external_user_id": "missing"},
			invoke: (*ManagedUsersHandler).Get,
		},
		{
			name:   "dismiss",
			method: http.MethodPost,
			target: "/v1/users/missing/dismiss",
			params: map[string]string{"external_user_id": "missing"},
			invoke: (*ManagedUsersHandler).DismissDisconnected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &managedUsersTestDB{}
			h := NewManagedUsersHandler(db.New(store))
			req := managedUsersRequest(tt.method, tt.target, tt.params)
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
			rec := httptest.NewRecorder()

			tt.invoke(h, rec, req)

			assertManagedUsersError(t, rec, http.StatusNotFound, "NOT_FOUND", "not_found")
		})
	}
}

func managedUsersRequest(method, target string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	routeContext := chi.NewRouteContext()
	for key, value := range params {
		routeContext.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
}

func assertManagedUsersError(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
	wantNormalized string,
) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var response ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
	if response.Error.Code != wantCode {
		t.Fatalf("error.code = %q, want %q", response.Error.Code, wantCode)
	}
	if response.Error.NormalizedCode != wantNormalized {
		t.Fatalf("error.normalized_code = %q, want %q", response.Error.NormalizedCode, wantNormalized)
	}
}

type managedUsersTestDB struct {
	profileExists          bool
	profileWorkspaceID     string
	profileErr             error
	managedUsersQueryCalls int
	dismissCalls           int
}

func (f *managedUsersTestDB) Exec(_ context.Context, query string, _ ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: DismissDisconnectedManagedAccountsByExternalUser") {
		f.dismissCalls++
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, nil
}

func (f *managedUsersTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListProfilesByWorkspace"):
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return &managedUsersRows{rows: [][]any{{
			"prof_legacy",
			"Legacy Profile",
			now,
			now,
			pgtype.Text{},
			pgtype.Text{},
			pgtype.Text{},
			"ws_1",
			false,
			pgtype.Text{},
		}}}, nil
	case strings.Contains(query, "-- name: ListManagedAccountsByExternalUser"):
		f.managedUsersQueryCalls++
		return &managedUsersRows{}, nil
	default:
		return nil, fmt.Errorf("unexpected Query: %s", query)
	}
}

func (f *managedUsersTestDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	if !strings.Contains(query, "-- name: GetProfile") {
		return managedUsersScanRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
	if f.profileErr != nil {
		return managedUsersScanRow{err: f.profileErr}
	}
	if !f.profileExists {
		return managedUsersScanRow{err: pgx.ErrNoRows}
	}
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return managedUsersScanRow{values: []any{
		"prof_1",
		"Profile",
		now,
		now,
		pgtype.Text{},
		pgtype.Text{},
		pgtype.Text{},
		f.profileWorkspaceID,
		false,
		pgtype.Text{},
	}}
}

type managedUsersScanRow struct {
	values []any
	err    error
}

func (r managedUsersScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan destination count %d != values count %d", len(dest), len(r.values))
	}
	return assignManagedUsersRow(dest, r.values)
}

type managedUsersRows struct {
	rows  [][]any
	index int
	err   error
}

func (r *managedUsersRows) Close()                                       {}
func (r *managedUsersRows) Err() error                                   { return r.err }
func (r *managedUsersRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *managedUsersRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *managedUsersRows) Next() bool {
	if r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}
func (r *managedUsersRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.rows) {
		return errors.New("Scan called without a current row")
	}
	return assignManagedUsersRow(dest, r.rows[r.index-1])
}
func (r *managedUsersRows) Values() ([]any, error) {
	if r.index == 0 || r.index > len(r.rows) {
		return nil, errors.New("Values called without a current row")
	}
	return r.rows[r.index-1], nil
}
func (*managedUsersRows) RawValues() [][]byte { return nil }
func (*managedUsersRows) Conn() *pgx.Conn     { return nil }

func assignManagedUsersRow(dest []any, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("scan destination count %d != values count %d", len(dest), len(values))
	}
	for i, value := range values {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Ptr || target.IsNil() {
			return fmt.Errorf("scan destination %d is not a non-nil pointer", i)
		}
		source := reflect.ValueOf(value)
		if !source.Type().AssignableTo(target.Elem().Type()) {
			return fmt.Errorf("cannot scan %T into %T", value, dest[i])
		}
		target.Elem().Set(source)
	}
	return nil
}
