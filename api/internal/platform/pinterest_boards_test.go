package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	pinterestTestBoardID = "1131529543818288706"
	pinterestTestOwner   = "mustafa57620090"
)

func pinterestTestDispatchContext(accountID string) context.Context {
	return WithDispatchMetadata(context.Background(), DispatchMetadata{
		SocialAccountID: accountID,
		Environment:     PinterestEnvironment(),
	})
}

func pinterestTestPost(adapter *PinterestAdapter, accountID, token string) error {
	_, err := adapter.Post(
		pinterestTestDispatchContext(accountID),
		token,
		"caption",
		[]MediaItem{{URL: "https://cdn.example.com/image.jpg", Kind: MediaKindImage}},
		map[string]any{"board_id": pinterestTestBoardID},
	)
	return err
}

func writePinterestJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func TestPinterestBoardPreflightDirectOwnedBoardProceedsToCreatePin(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/v5/user_account":
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards/" + pinterestTestBoardID:
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"id": pinterestTestBoardID, "name": "Launches", "owner": map[string]any{"username": pinterestTestOwner},
			})
		case "/v5/pins":
			writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	err := pinterestTestPost(&PinterestAdapter{client: srv.Client()}, "sa_1", "token_1")
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	want := []string{"GET /v5/user_account", "GET /v5/boards/" + pinterestTestBoardID, "POST /v5/pins"}
	if fmt.Sprint(paths) != fmt.Sprint(want) {
		t.Fatalf("request sequence = %v, want %v", paths, want)
	}
}

func TestPinterestBoardPreflightPermanentLookupFailuresPreventCreatePin(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		code       int
		wantReason string
	}{
		{name: "not found", status: http.StatusNotFound, code: 40, wantReason: "board_not_found"},
		{name: "not accessible", status: http.StatusForbidden, code: 29, wantReason: "board_not_accessible"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PINTEREST_USE_SANDBOX", "")
			pinCalls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v5/user_account":
					writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
				case "/v5/boards/" + pinterestTestBoardID:
					writePinterestJSON(w, tt.status, map[string]any{"code": tt.code, "message": "provider detail"})
				case "/v5/pins":
					pinCalls++
					writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
				default:
					http.Error(w, "unexpected path", http.StatusBadRequest)
				}
			}))
			defer srv.Close()
			t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

			err := pinterestTestPost(&PinterestAdapter{client: srv.Client()}, "sa_1", "token_1")
			if err == nil {
				t.Fatal("expected preflight failure")
			}
			if pinCalls != 0 {
				t.Fatalf("create Pin calls = %d, want 0", pinCalls)
			}
			failure := err.(interface{ FailureContractFields() map[string]any }).FailureContractFields()
			if failure["error_code"] != "target_not_found" || failure["failure_stage"] != "destination_preflight" || failure["is_retriable"] != false {
				t.Fatalf("failure fields = %#v", failure)
			}
			provider := err.(interface{ ProviderErrorFields() map[string]any }).ProviderErrorFields()
			if provider["reason"] != tt.wantReason || provider["code"] != fmt.Sprint(tt.code) {
				t.Fatalf("provider fields = %#v", provider)
			}
		})
	}
}

func TestPinterestBoardPreflightFallsBackToBoundedOwnedBoardList(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	var bookmarks []string
	pinCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards/" + pinterestTestBoardID:
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"id": pinterestTestBoardID, "owner": map[string]any{"username": "shared_board_owner"},
			})
		case "/v5/boards":
			if r.URL.Query().Get("page_size") != "250" {
				t.Fatalf("page_size = %q, want 250", r.URL.Query().Get("page_size"))
			}
			bookmark := r.URL.Query().Get("bookmark")
			bookmarks = append(bookmarks, bookmark)
			if bookmark == "" {
				writePinterestJSON(w, http.StatusOK, map[string]any{
					"items":    []any{map[string]any{"id": "111", "owner": map[string]any{"username": pinterestTestOwner}}},
					"bookmark": "page_2",
				})
				return
			}
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"items":    []any{map[string]any{"id": pinterestTestBoardID, "owner": map[string]any{"username": pinterestTestOwner}}},
				"bookmark": "",
			})
		case "/v5/pins":
			pinCalls++
			writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	err := pinterestTestPost(&PinterestAdapter{client: srv.Client()}, "sa_1", "token_1")
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if fmt.Sprint(bookmarks) != fmt.Sprint([]string{"", "page_2"}) {
		t.Fatalf("bookmarks = %v", bookmarks)
	}
	if pinCalls != 1 {
		t.Fatalf("create Pin calls = %d, want 1", pinCalls)
	}
}

func TestPinterestBoardPreflightVisibleSharedBoardIsNotWritable(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	pinCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards/" + pinterestTestBoardID:
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"id": pinterestTestBoardID, "owner": map[string]any{"username": "shared_board_owner"},
			})
		case "/v5/boards":
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"items":    []any{map[string]any{"id": pinterestTestBoardID, "owner": map[string]any{"username": "shared_board_owner"}}},
				"bookmark": "",
			})
		case "/v5/pins":
			pinCalls++
			writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	err := pinterestTestPost(&PinterestAdapter{client: srv.Client()}, "sa_1", "token_1")
	if err == nil {
		t.Fatal("expected shared-board rejection")
	}
	if pinCalls != 0 {
		t.Fatalf("create Pin calls = %d, want 0", pinCalls)
	}
	failure := err.(interface{ FailureContractFields() map[string]any }).FailureContractFields()
	if failure["error_code"] != "target_not_found" || failure["is_retriable"] != false {
		t.Fatalf("failure fields = %#v", failure)
	}
}

func TestPinterestBoardPreflightCacheUsesAccountEnvironmentTokenAndTTL(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	userCalls, listCalls, directCalls, pinCalls := 0, 0, 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			userCalls++
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards/" + pinterestTestBoardID:
			directCalls++
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": pinterestTestBoardID})
		case "/v5/boards":
			listCalls++
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"items":    []any{map[string]any{"id": pinterestTestBoardID, "owner": map[string]any{"username": pinterestTestOwner}}},
				"bookmark": "",
			})
		case "/v5/pins":
			pinCalls++
			writePinterestJSON(w, http.StatusCreated, map[string]any{"id": fmt.Sprintf("pin_%d", pinCalls)})
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")
	adapter := &PinterestAdapter{client: srv.Client(), now: func() time.Time { return now }}

	if err := pinterestTestPost(adapter, "sa_1", "token_1"); err != nil {
		t.Fatal(err)
	}
	if err := pinterestTestPost(adapter, "sa_1", "token_1"); err != nil {
		t.Fatal(err)
	}
	if userCalls != 1 || listCalls != 1 || directCalls != 2 || pinCalls != 2 {
		t.Fatalf("within TTL calls user/list/direct/pin = %d/%d/%d/%d", userCalls, listCalls, directCalls, pinCalls)
	}

	if err := pinterestTestPost(adapter, "sa_1", "token_2"); err != nil {
		t.Fatal(err)
	}
	if userCalls != 2 || listCalls != 2 {
		t.Fatalf("token change did not miss cache: user/list = %d/%d", userCalls, listCalls)
	}

	now = now.Add(61 * time.Second)
	if err := pinterestTestPost(adapter, "sa_1", "token_2"); err != nil {
		t.Fatal(err)
	}
	if userCalls != 3 || listCalls != 3 {
		t.Fatalf("TTL expiry did not miss cache: user/list = %d/%d", userCalls, listCalls)
	}

	t.Setenv("PINTEREST_USE_SANDBOX", "true")
	if err := pinterestTestPost(adapter, "sa_1", "token_2"); err != nil {
		t.Fatal(err)
	}
	if userCalls != 4 || listCalls != 4 {
		t.Fatalf("environment change did not miss cache: user/list = %d/%d", userCalls, listCalls)
	}
}

func TestPinterestBoardCacheTTLDoesNotSlideOnDirectProof(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	userCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			userCalls++
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards/" + pinterestTestBoardID:
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"id": pinterestTestBoardID, "owner": map[string]any{"username": pinterestTestOwner},
			})
		case "/v5/pins":
			writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")
	adapter := &PinterestAdapter{client: srv.Client(), now: func() time.Time { return now }}

	if err := pinterestTestPost(adapter, "sa_1", "token_1"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	if err := pinterestTestPost(adapter, "sa_1", "token_1"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(31 * time.Second)
	if err := pinterestTestPost(adapter, "sa_1", "token_1"); err != nil {
		t.Fatal(err)
	}
	if userCalls != 2 {
		t.Fatalf("user account calls = %d, want 2 after original TTL expires", userCalls)
	}
}

func TestPinterestBoardPreflightAccountFailureUsesTypedContract(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	boardCalls, pinCalls := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			writePinterestJSON(w, http.StatusUnauthorized, map[string]any{"code": 2, "message": "invalid token secret"})
		case "/v5/boards/" + pinterestTestBoardID:
			boardCalls++
		case "/v5/pins":
			pinCalls++
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	err := pinterestTestPost(&PinterestAdapter{client: srv.Client()}, "sa_1", "token_1")
	if err == nil {
		t.Fatal("expected account failure")
	}
	if boardCalls != 0 || pinCalls != 0 {
		t.Fatalf("board/pin calls = %d/%d, want 0/0", boardCalls, pinCalls)
	}
	failureCarrier, ok := err.(interface{ FailureContractFields() map[string]any })
	if !ok {
		t.Fatalf("account error does not carry typed failure fields: %T", err)
	}
	failure := failureCarrier.FailureContractFields()
	if failure["error_code"] != "auth_token_invalid" || failure["failure_stage"] != "destination_preflight" || failure["is_retriable"] != false {
		t.Fatalf("failure fields = %#v", failure)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("public error leaked provider body: %q", err)
	}
}

func TestPinterestBoardPreflightTraversalSafetyFailuresAreTemporary(t *testing.T) {
	tests := []struct {
		name       string
		listResult func(call int) map[string]any
	}{
		{
			name: "repeated bookmark",
			listResult: func(call int) map[string]any {
				return map[string]any{
					"items":    []any{map[string]any{"id": fmt.Sprintf("board_%d", call), "owner": map[string]any{"username": pinterestTestOwner}}},
					"bookmark": "same",
				}
			},
		},
		{
			name: "duplicate page",
			listResult: func(call int) map[string]any {
				return map[string]any{
					"items":    []any{map[string]any{"id": "duplicate", "owner": map[string]any{"username": pinterestTestOwner}}},
					"bookmark": fmt.Sprintf("bookmark_%d", call),
				}
			},
		},
		{
			name: "eight page cap",
			listResult: func(call int) map[string]any {
				items := make([]any, 250)
				for i := range items {
					items[i] = map[string]any{"id": fmt.Sprintf("page_%d_board_%d", call, i), "owner": map[string]any{"username": pinterestTestOwner}}
				}
				return map[string]any{"items": items, "bookmark": fmt.Sprintf("next_%d", call)}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PINTEREST_USE_SANDBOX", "")
			listCalls, pinCalls := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v5/user_account":
					writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
				case "/v5/boards/" + pinterestTestBoardID:
					writePinterestJSON(w, http.StatusOK, map[string]any{"id": pinterestTestBoardID})
				case "/v5/boards":
					listCalls++
					writePinterestJSON(w, http.StatusOK, tt.listResult(listCalls))
				case "/v5/pins":
					pinCalls++
					writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
				default:
					http.Error(w, "unexpected path", http.StatusBadRequest)
				}
			}))
			defer srv.Close()
			t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

			err := pinterestTestPost(&PinterestAdapter{client: srv.Client()}, "sa_1", "token_1")
			if err == nil {
				t.Fatal("expected traversal safety failure")
			}
			if pinCalls != 0 {
				t.Fatalf("create Pin calls = %d, want 0", pinCalls)
			}
			failure := err.(interface{ FailureContractFields() map[string]any }).FailureContractFields()
			if failure["error_code"] != "temporary_platform_error" || failure["failure_stage"] != "destination_preflight" || failure["is_retriable"] != true {
				t.Fatalf("failure fields = %#v", failure)
			}
			if tt.name == "eight page cap" && listCalls != 8 {
				t.Fatalf("list calls = %d, want 8", listCalls)
			}
		})
	}
}

func TestPinterestBoardCollectionForbiddenMapsToMissingPermission(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	pinCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards/" + pinterestTestBoardID:
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": pinterestTestBoardID})
		case "/v5/boards":
			writePinterestJSON(w, http.StatusForbidden, map[string]any{"code": 29, "message": "scope rejected"})
		case "/v5/pins":
			pinCalls++
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	err := pinterestTestPost(&PinterestAdapter{client: srv.Client()}, "sa_1", "token_1")
	if err == nil {
		t.Fatal("expected collection permission failure")
	}
	if pinCalls != 0 {
		t.Fatalf("create Pin calls = %d, want 0", pinCalls)
	}
	failure := err.(interface{ FailureContractFields() map[string]any }).FailureContractFields()
	if failure["error_code"] != "missing_permission" || failure["is_retriable"] != false {
		t.Fatalf("failure fields = %#v", failure)
	}
	provider := err.(interface{ ProviderErrorFields() map[string]any }).ProviderErrorFields()
	if provider["reason"] != "missing_permission" {
		t.Fatalf("provider fields = %#v", provider)
	}
}

func TestPinterestBoardPreflightNeverLeaksBoardAPIURL(t *testing.T) {
	err := pinterestBoardProofTemporaryFailure("repeated_bookmark")
	if strings.Contains(err.Error(), "boards") || strings.Contains(err.Error(), "http") {
		t.Fatalf("public error leaked destination detail: %q", err)
	}
}

func TestPinterestFetchBoardsReturnsOnlyBoardsOwnedByOperationUser(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards":
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"items": []any{
					map[string]any{"id": "owned_1", "name": "Owned", "owner": map[string]any{"username": pinterestTestOwner}},
					map[string]any{"id": "shared_1", "name": "Shared", "owner": map[string]any{"username": "another_user"}},
				},
				"bookmark": "",
			})
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	boards, err := (&PinterestAdapter{client: srv.Client()}).FetchBoards(context.Background(), "token_1")
	if err != nil {
		t.Fatalf("FetchBoards: %v", err)
	}
	if len(boards) != 1 || boards[0].ID != "owned_1" || boards[0].Name != "Owned" {
		t.Fatalf("boards = %#v, want only owned_1", boards)
	}
}

func TestPinterestFetchBoardsRejectsDuplicatePages(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	listCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards":
			listCalls++
			bookmark := "next_1"
			if listCalls == 2 {
				bookmark = "next_2"
			}
			if listCalls > 2 {
				bookmark = ""
			}
			writePinterestJSON(w, http.StatusOK, map[string]any{
				"items":    []any{map[string]any{"id": "duplicate", "name": "Duplicate", "owner": map[string]any{"username": pinterestTestOwner}}},
				"bookmark": bookmark,
			})
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	_, err := (&PinterestAdapter{client: srv.Client()}).FetchBoards(context.Background(), "token_1")
	if err == nil {
		t.Fatal("expected duplicate-page safety failure")
	}
	failureCarrier, ok := err.(interface{ FailureContractFields() map[string]any })
	if !ok {
		t.Fatalf("error does not carry failure fields: %T", err)
	}
	failure := failureCarrier.FailureContractFields()
	if failure["error_code"] != "temporary_platform_error" || failure["is_retriable"] != true {
		t.Fatalf("failure fields = %#v", failure)
	}
}
