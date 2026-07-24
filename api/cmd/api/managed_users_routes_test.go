package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/handler"
)

func TestManagedUsersRoutesRejectUnauthorizedRequestsBeforeHandler(t *testing.T) {
	routes := []struct {
		name   string
		method string
		path   string
	}{
		{name: "list", method: http.MethodGet, path: "/v1/profiles/prof_1/users"},
		{name: "get", method: http.MethodGet, path: "/v1/profiles/prof_1/users/user_1"},
		{name: "dismiss", method: http.MethodPost, path: "/v1/profiles/prof_1/users/user_1/dismiss"},
	}
	authCases := []struct {
		name          string
		authorization string
	}{
		{name: "missing API key"},
		{name: "invalid API key", authorization: "Bearer up_test_invalid"},
	}

	for _, authCase := range authCases {
		for _, route := range routes {
			t.Run(authCase.name+"/"+route.name, func(t *testing.T) {
				store := &managedUsersRouteAuthDB{}
				queries := db.New(store)
				router := chi.NewRouter()
				router.Group(func(r chi.Router) {
					r.Use(auth.DualAuthMiddleware(queries))
					registerManagedUsersRoutes(r, handler.NewManagedUsersHandler(queries))
				})
				req := httptest.NewRequest(route.method, route.path, nil)
				if authCase.authorization != "" {
					req.Header.Set("Authorization", authCase.authorization)
				}
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
				}
				var response struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if response.Error.Code != "UNAUTHORIZED" {
					t.Fatalf("error.code = %q, want UNAUTHORIZED", response.Error.Code)
				}
				if store.handlerQueries != 0 || store.handlerExecs != 0 {
					t.Fatalf(
						"handler database calls = (queries %d, execs %d), want zero",
						store.handlerQueries,
						store.handlerExecs,
					)
				}
			})
		}
	}
}

type managedUsersRouteAuthDB struct {
	handlerQueries int
	handlerExecs   int
}

func (f *managedUsersRouteAuthDB) Exec(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
	f.handlerExecs++
	return pgconn.CommandTag{}, nil
}

func (f *managedUsersRouteAuthDB) Query(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	f.handlerQueries++
	return nil, pgx.ErrNoRows
}

func (f *managedUsersRouteAuthDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	if !strings.Contains(query, "-- name: GetAPIKeyByHash") {
		f.handlerQueries++
	}
	return managedUsersRejectedAuthRow{}
}

type managedUsersRejectedAuthRow struct{}

func (managedUsersRejectedAuthRow) Scan(...any) error { return pgx.ErrNoRows }
