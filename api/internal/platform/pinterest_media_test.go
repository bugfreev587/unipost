package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/safefetch"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

type fakePinterestMediaStager struct {
	calls  int
	url    string
	policy safefetch.Policy
	result storage.ExternalMediaResult
	err    error
	onCall func()
}

func (s *fakePinterestMediaStager) StageExternalMedia(_ context.Context, rawURL string, policy safefetch.Policy) (storage.ExternalMediaResult, error) {
	s.calls++
	s.url = rawURL
	s.policy = policy
	if s.onCall != nil {
		s.onCall()
	}
	return s.result, s.err
}

func TestPinterestExternalMediaStagesAfterBoardPreflight(t *testing.T) {
	var order []string
	var createMediaURL string
	srv := newPinterestMediaTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "create_pin")
		var payload struct {
			MediaSource struct {
				URL string `json:"url"`
			} `json:"media_source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode create Pin payload: %v", err)
		}
		createMediaURL = payload.MediaSource.URL
		writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
	}, func(step string) { order = append(order, step) })
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	stager := &fakePinterestMediaStager{
		result: storage.ExternalMediaResult{
			PublicURL: "https://media.unipost.dev/pull/verified.jpg",
			MediaType: "image/jpeg",
			SizeBytes: 512,
		},
		onCall: func() { order = append(order, "stage_media") },
	}
	adapter := &PinterestAdapter{client: srv.Client(), mediaStager: stager}
	sourceURL := "https://customer.example/photo.jpg?signature=secret"

	_, err := adapter.Post(context.Background(), "token", "caption", []MediaItem{{
		URL: sourceURL, Kind: MediaKindImage, Origin: MediaOriginExternal,
	}}, map[string]any{"board_id": pinterestTestBoardID})
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	if stager.calls != 1 || stager.url != sourceURL {
		t.Fatalf("stager calls/url = %d/%q", stager.calls, stager.url)
	}
	if createMediaURL != stager.result.PublicURL {
		t.Fatalf("create Pin media URL = %q, want staged URL %q", createMediaURL, stager.result.PublicURL)
	}
	if want := []string{"user_account", "board", "stage_media", "create_pin"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("operation order = %#v, want %#v", order, want)
	}
	if stager.policy.MaxBytes != Capabilities["pinterest"].Media.Videos.MaxFileSizeBytes {
		t.Fatalf("overall MaxBytes = %d", stager.policy.MaxBytes)
	}
	if want := []string{"image/jpeg", "image/png", "image/webp", "video/mp4", "video/quicktime"}; !reflect.DeepEqual(stager.policy.AllowedMediaTypes, want) {
		t.Fatalf("allowed types = %#v, want %#v", stager.policy.AllowedMediaTypes, want)
	}
	if stager.policy.MaxBytesByMediaType["image/jpeg"] != Capabilities["pinterest"].Media.Images.MaxFileSizeBytes ||
		stager.policy.MaxBytesByMediaType["video/mp4"] != Capabilities["pinterest"].Media.Videos.MaxFileSizeBytes {
		t.Fatalf("per-type limits = %#v", stager.policy.MaxBytesByMediaType)
	}
}

func TestPinterestExternalVideoPolicyFollowsCapabilities(t *testing.T) {
	policy := pinterestExternalMediaPolicy(MediaKindVideo)
	if policy.MaxBytes != Capabilities["pinterest"].Media.Videos.MaxFileSizeBytes {
		t.Fatalf("video MaxBytes = %d", policy.MaxBytes)
	}
	if want := []string{"image/jpeg", "image/png", "image/webp", "video/mp4", "video/quicktime"}; !reflect.DeepEqual(policy.AllowedMediaTypes, want) {
		t.Fatalf("allowed types = %#v, want %#v", policy.AllowedMediaTypes, want)
	}
	if policy.MaxRedirects != 5 {
		t.Fatalf("video MaxRedirects = %d, want 5", policy.MaxRedirects)
	}
	if !reflect.DeepEqual(policy, pinterestExternalMediaPolicy(MediaKindImage)) {
		t.Fatalf("safe-fetch policy must not depend on untrusted URL-derived media kind")
	}
}

func TestPinterestManagedMediaBypassesExternalStaging(t *testing.T) {
	var createMediaURL string
	srv := newPinterestMediaTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			MediaSource struct {
				URL string `json:"url"`
			} `json:"media_source"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		createMediaURL = payload.MediaSource.URL
		writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
	}, nil)
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	stager := &fakePinterestMediaStager{err: errors.New("must not stage managed media")}
	adapter := &PinterestAdapter{client: srv.Client(), mediaStager: stager}
	managedURL := "https://media.unipost.dev/media/owned.jpg"
	_, err := adapter.Post(context.Background(), "token", "caption", []MediaItem{{
		URL: managedURL, Kind: MediaKindImage, Origin: MediaOriginManaged,
	}}, map[string]any{"board_id": pinterestTestBoardID})
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	if stager.calls != 0 {
		t.Fatalf("managed media staging calls = %d, want 0", stager.calls)
	}
	if createMediaURL != managedURL {
		t.Fatalf("create Pin media URL = %q, want %q", createMediaURL, managedURL)
	}
}

func TestPinterestLegacyMediaOriginIsConservativelyExternal(t *testing.T) {
	srv := newPinterestMediaTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writePinterestJSON(w, http.StatusCreated, map[string]any{"id": "pin_1"})
	}, nil)
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	stager := &fakePinterestMediaStager{result: storage.ExternalMediaResult{PublicURL: "https://media.unipost.dev/pull/verified.jpg", MediaType: "image/jpeg", SizeBytes: 10}}
	adapter := &PinterestAdapter{client: srv.Client(), mediaStager: stager}
	_, err := adapter.Post(context.Background(), "token", "caption", []MediaItem{{
		URL: "https://customer.example/plain.jpg", Kind: MediaKindImage,
	}}, map[string]any{"board_id": pinterestTestBoardID})
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	if stager.calls != 1 {
		t.Fatalf("legacy-origin staging calls = %d, want 1", stager.calls)
	}
}

func TestPinterestMediaPreflightMapsTypedFailuresAndStopsProviderWrite(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantErrorCode string
		wantReason    string
		wantRetriable bool
		wantHTTP      int
	}{
		{name: "source not found", err: &safefetch.FetchError{Kind: safefetch.ErrorSourceNotFound, HTTPStatus: 404}, wantErrorCode: "media_error", wantReason: "customer_media_unreachable", wantHTTP: 404},
		{name: "unsupported media", err: &safefetch.FetchError{Kind: safefetch.ErrorUnsupportedMedia}, wantErrorCode: "media_error", wantReason: "unsupported_media"},
		{name: "too large", err: &safefetch.FetchError{Kind: safefetch.ErrorTooLarge}, wantErrorCode: "media_error", wantReason: "media_too_large"},
		{name: "timeout", err: &safefetch.FetchError{Kind: safefetch.ErrorTimeout, Temporary: true}, wantErrorCode: "temporary_platform_error", wantReason: "media_stage_temporary", wantRetriable: true},
		{name: "source temporary", err: &safefetch.FetchError{Kind: safefetch.ErrorSourceTemporary, HTTPStatus: 503, Temporary: true}, wantErrorCode: "temporary_platform_error", wantReason: "media_stage_temporary", wantRetriable: true, wantHTTP: 503},
		{name: "storage unavailable", err: storage.ErrExternalMediaUpload, wantErrorCode: "temporary_platform_error", wantReason: "media_stage_temporary", wantRetriable: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pinCalls := 0
			srv := newPinterestMediaTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				pinCalls++
				http.Error(w, "must not create Pin", http.StatusInternalServerError)
			}, nil)
			defer srv.Close()
			t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

			recorder := NewDispatchEventRecorder()
			ctx := WithDispatchMetadata(context.Background(), DispatchMetadata{Events: recorder})
			sourceURL := "https://customer.example/media.jpg?token=must-not-leak"
			adapter := &PinterestAdapter{client: srv.Client(), mediaStager: &fakePinterestMediaStager{err: tt.err}}
			_, err := adapter.Post(ctx, "token", "caption", []MediaItem{{URL: sourceURL, Kind: MediaKindImage, Origin: MediaOriginExternal}}, map[string]any{"board_id": pinterestTestBoardID})
			if err == nil {
				t.Fatal("expected media preflight failure")
			}
			if pinCalls != 0 {
				t.Fatalf("create Pin calls = %d, want 0", pinCalls)
			}
			if strings.Contains(err.Error(), sourceURL) || strings.Contains(err.Error(), "must-not-leak") {
				t.Fatalf("public error leaked source URL: %q", err)
			}
			failure := err.(interface{ FailureContractFields() map[string]any }).FailureContractFields()
			if failure["error_code"] != tt.wantErrorCode || failure["failure_stage"] != "media_preflight" || failure["is_retriable"] != tt.wantRetriable {
				t.Fatalf("failure fields = %#v", failure)
			}
			provider := err.(interface{ ProviderErrorFields() map[string]any }).ProviderErrorFields()
			if provider["provider"] != "pinterest" || provider["reason"] != tt.wantReason {
				t.Fatalf("provider fields = %#v", provider)
			}
			if tt.wantHTTP > 0 && provider["http_status"] != tt.wantHTTP {
				t.Fatalf("provider http_status = %#v, want %d", provider["http_status"], tt.wantHTTP)
			}
			events := recorder.Snapshot()
			last := events[len(events)-1]
			if last.Name != "pinterest_media_preflight_failed" || last.FailureStage != "media_preflight" || last.Reason != tt.wantReason || last.Retriable != tt.wantRetriable || last.HTTPStatus != tt.wantHTTP {
				t.Fatalf("media failure event = %#v", last)
			}
		})
	}
}

func TestPinterestBoardFailurePreventsExternalMediaFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards/" + pinterestTestBoardID:
			writePinterestJSON(w, http.StatusNotFound, map[string]any{"code": 40, "message": "missing"})
		default:
			http.Error(w, "unexpected provider write", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	stager := &fakePinterestMediaStager{}
	adapter := &PinterestAdapter{client: srv.Client(), mediaStager: stager}
	_, err := adapter.Post(context.Background(), "token", "caption", []MediaItem{{URL: "https://customer.example/media.jpg", Origin: MediaOriginExternal}}, map[string]any{"board_id": pinterestTestBoardID})
	if err == nil {
		t.Fatal("expected destination preflight failure")
	}
	if stager.calls != 0 {
		t.Fatalf("media stage calls = %d, want 0", stager.calls)
	}
}

func newPinterestMediaTestServer(t *testing.T, createPin http.HandlerFunc, onPreflight func(string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/user_account":
			if onPreflight != nil {
				onPreflight("user_account")
			}
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": "user_1", "username": pinterestTestOwner})
		case "/v5/boards/" + pinterestTestBoardID:
			if onPreflight != nil {
				onPreflight("board")
			}
			writePinterestJSON(w, http.StatusOK, map[string]any{"id": pinterestTestBoardID, "owner": map[string]any{"username": pinterestTestOwner}})
		case "/v5/pins":
			createPin(w, r)
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
}
