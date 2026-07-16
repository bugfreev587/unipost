package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type twitterRoundTripFunc func(*http.Request) (*http.Response, error)

func (f twitterRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTwitterOAuthPKCEAuthorizationUsesConfiguredScopesAndS256(t *testing.T) {
	adapter := NewTwitterAdapter()
	config := OAuthConfig{
		ClientID:     "client-id",
		AuthURL:      "https://twitter.example/authorize",
		RedirectURL:  "https://api.example/v1/oauth/callback/twitter",
		Scopes:       []string{"custom.read", "custom.write"},
		PKCEVerifier: "stored-random-verifier",
	}

	authURL := adapter.GetAuthURL(config, "csrf-state")
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	sum := sha256.Sum256([]byte(config.PKCEVerifier))
	wantChallenge := base64.RawURLEncoding.EncodeToString(sum[:])
	query := parsed.Query()
	if got := query.Get("scope"); got != "custom.read custom.write" {
		t.Fatalf("scope = %q, want configured scopes", got)
	}
	if got := query.Get("code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method = %q, want S256", got)
	}
	if got := query.Get("code_challenge"); got != wantChallenge {
		t.Fatalf("code_challenge = %q, want %q", got, wantChallenge)
	}
	if query.Get("code_challenge") == "csrf-state" {
		t.Fatal("code_challenge must not be derived from OAuth state")
	}
}

func TestTwitterOAuthPKCEDefaultScopesIncludeInboxDMs(t *testing.T) {
	t.Setenv("TWITTER_CLIENT_ID", "client-id")
	t.Setenv("TWITTER_CLIENT_SECRET", "client-secret")

	got := NewTwitterAdapter().DefaultOAuthConfig("https://api.example").Scopes
	want := []string{"tweet.read", "tweet.write", "users.read", "offline.access", "media.write", "dm.read", "dm.write"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scopes = %#v, want %#v", got, want)
	}
}

func TestTwitterOAuthPKCEExchangeUsesStoredVerifierAndGrantedScopes(t *testing.T) {
	var tokenForm url.Values
	adapter := &TwitterAdapter{client: &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "token.example":
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			tokenForm = req.Form
			return jsonResponse(http.StatusOK, `{
				"access_token":"access-token",
				"refresh_token":"refresh-token",
				"expires_in":7200,
				"scope":"tweet.read dm.read"
			}`), nil
		case "api.x.com":
			return jsonResponse(http.StatusOK, `{
				"data":{"id":"x-user","username":"robyn","profile_image_url":"https://example/avatar.png"}
			}`), nil
		default:
			t.Fatalf("unexpected request URL: %s", req.URL)
			return nil, nil
		}
	})}}
	config := OAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		TokenURL:     "https://token.example/oauth2/token",
		RedirectURL:  "https://api.example/v1/oauth/callback/twitter",
		Scopes:       []string{"tweet.read", "tweet.write", "dm.read", "dm.write"},
		PKCEVerifier: "stored-random-verifier",
	}

	result, err := adapter.ExchangeCode(context.Background(), config, "authorization-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if got := tokenForm.Get("code_verifier"); got != config.PKCEVerifier {
		t.Fatalf("code_verifier = %q, want stored verifier", got)
	}
	if !reflect.DeepEqual(result.Scopes, []string{"tweet.read", "dm.read"}) {
		t.Fatalf("scopes = %#v, want granted token scopes", result.Scopes)
	}
}

func TestTwitterOAuthPKCEExchangeFallsBackToConfiguredScopesWhenOmitted(t *testing.T) {
	adapter := &TwitterAdapter{client: &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "token.example":
			return jsonResponse(http.StatusOK, `{
				"access_token":"access-token",
				"refresh_token":"refresh-token",
				"expires_in":7200,
				"scope":"   "
			}`), nil
		case "api.x.com":
			return jsonResponse(http.StatusOK, `{
				"data":{"id":"x-user","username":"robyn","profile_image_url":"https://example/avatar.png"}
			}`), nil
		default:
			t.Fatalf("unexpected request URL: %s", req.URL)
			return nil, nil
		}
	})}}
	config := OAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		TokenURL:     "https://token.example/oauth2/token",
		RedirectURL:  "https://api.example/v1/oauth/callback/twitter",
		Scopes:       []string{"tweet.read", "tweet.write", "dm.read", "dm.write"},
		PKCEVerifier: "stored-random-verifier",
	}

	result, err := adapter.ExchangeCode(context.Background(), config, "authorization-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if !reflect.DeepEqual(result.Scopes, config.Scopes) {
		t.Fatalf("scopes = %#v, want configured fallback %#v", result.Scopes, config.Scopes)
	}
}

func TestTwitterUploadMediaChunkedNormalizesParameterizedVideoContentType(t *testing.T) {
	var sawInit bool
	var sawFinalize bool

	adapter := &TwitterAdapter{client: &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Host == "media.example.com":
			return textResponse(http.StatusOK, "fake-mp4-bytes", map[string]string{
				"Content-Type": "video/mp4;codecs=avc1",
			}), nil
		case req.URL.Host == "api.x.com" && req.URL.Path == "/2/media/upload/initialize":
			if req.URL.Query().Get("media_type") != "" || req.URL.Query().Get("media_category") != "" {
				t.Fatalf("INIT metadata must not be sent as query params: %s", req.URL.RawQuery)
			}
			if got := req.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("INIT content type = %q, want application/json", got)
			}
			var initBody struct {
				MediaType     string `json:"media_type"`
				MediaCategory string `json:"media_category"`
				TotalBytes    int    `json:"total_bytes"`
			}
			if err := json.NewDecoder(req.Body).Decode(&initBody); err != nil {
				t.Fatalf("decode INIT JSON: %v", err)
			}
			sawInit = true
			if initBody.MediaType != "video/mp4" {
				t.Fatalf("media_type = %q, want video/mp4", initBody.MediaType)
			}
			if initBody.MediaCategory != "tweet_video" {
				t.Fatalf("media_category = %q, want tweet_video", initBody.MediaCategory)
			}
			if initBody.TotalBytes != 14 {
				t.Fatalf("total_bytes = %d, want 14", initBody.TotalBytes)
			}
			return jsonResponse(http.StatusOK, `{"data":{"id":"media123"}}`), nil
		case req.URL.Host == "api.x.com" && req.URL.Path == "/2/media/upload/media123/append":
			form, err := readMultipartFields(req)
			if err != nil {
				t.Fatalf("read multipart fields: %v", err)
			}
			if form["command"] != "" || form["media_id"] != "" {
				t.Fatalf("APPEND must not send legacy command/media_id fields: %+v", form)
			}
			if got := form["segment_index"]; got != "0" {
				t.Fatalf("segment_index = %q, want 0", got)
			}
			if req.MultipartForm == nil || len(req.MultipartForm.File["media"]) != 1 {
				t.Fatalf("APPEND media file count = %d, want 1", len(req.MultipartForm.File["media"]))
			}
			return jsonResponse(http.StatusOK, `{}`), nil
		case req.URL.Host == "api.x.com" && req.URL.Path == "/2/media/upload/media123/finalize":
			sawFinalize = true
			if req.Body != nil {
				body, _ := io.ReadAll(req.Body)
				if len(strings.TrimSpace(string(body))) > 0 {
					t.Fatalf("FINALIZE body = %q, want empty", string(body))
				}
			}
			return jsonResponse(http.StatusOK, `{"data":{"id":"media123"}}`), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})}}

	mediaID, err := adapter.uploadMedia(context.Background(), "token", MediaItem{
		URL:  "https://media.example.com/video.mp4",
		Kind: MediaKindVideo,
	})
	if err != nil {
		t.Fatalf("uploadMedia: %v", err)
	}
	if mediaID != "media123" {
		t.Fatalf("mediaID = %q, want media123", mediaID)
	}
	if !sawInit {
		t.Fatal("did not see INIT request")
	}
	if !sawFinalize {
		t.Fatal("did not see FINALIZE request")
	}
}

func TestTwitterRefreshTokenRejectsFailedResponse(t *testing.T) {
	adapter := &TwitterAdapter{client: &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.x.com/2/oauth2/token" {
			t.Fatalf("unexpected request URL: %s", req.URL.String())
		}
		return jsonResponse(http.StatusBadRequest, `{"error":"invalid_grant"}`), nil
	})}}

	access, refresh, expiresAt, err := adapter.RefreshToken(context.Background(), "old-refresh")
	if err == nil {
		t.Fatal("RefreshToken error = nil, want non-nil")
	}
	if access != "" || refresh != "" || !expiresAt.IsZero() {
		t.Fatalf("RefreshToken returned partial success: access=%q refresh=%q expiresAt=%s", access, refresh, expiresAt)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("RefreshToken error = %q, want status code context", err.Error())
	}
}

func TestTwitterRefreshTokenRejectsMissingExpiresIn(t *testing.T) {
	adapter := &TwitterAdapter{client: &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh"}`), nil
	})}}

	access, refresh, expiresAt, err := adapter.RefreshToken(context.Background(), "old-refresh")
	if err == nil {
		t.Fatal("RefreshToken error = nil, want non-nil")
	}
	if access != "" || refresh != "" || !expiresAt.IsZero() {
		t.Fatalf("RefreshToken returned partial success: access=%q refresh=%q expiresAt=%s", access, refresh, expiresAt)
	}
	if !strings.Contains(err.Error(), "expires_in") {
		t.Fatalf("RefreshToken error = %q, want expires_in context", err.Error())
	}
}

func TestTwitterRefreshTokenSendsClientCredentials(t *testing.T) {
	t.Setenv("TWITTER_CLIENT_ID", "client-id")
	t.Setenv("TWITTER_CLIENT_SECRET", "client-secret")

	adapter := &TwitterAdapter{client: &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		username, password, ok := req.BasicAuth()
		if !ok {
			t.Fatal("missing basic auth")
		}
		if username != "client-id" || password != "client-secret" {
			t.Fatalf("basic auth = %q/%q, want client-id/client-secret", username, password)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatalf("parse refresh form: %v", err)
		}
		if got := req.Form.Get("client_id"); got != "client-id" {
			t.Fatalf("client_id = %q, want client-id", got)
		}
		if got := req.Form.Get("refresh_token"); got != "old-refresh" {
			t.Fatalf("refresh_token = %q, want old-refresh", got)
		}
		return jsonResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":7200}`), nil
	})}}

	access, refresh, expiresAt, err := adapter.RefreshToken(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if access != "new-access" || refresh != "new-refresh" {
		t.Fatalf("tokens = %q/%q, want new-access/new-refresh", access, refresh)
	}
	if time.Until(expiresAt) < time.Hour {
		t.Fatalf("expiresAt = %s, want roughly 2h in the future", expiresAt)
	}
}

func readMultipartFields(req *http.Request) (map[string]string, error) {
	mediaType := req.Header.Get("Content-Type")
	if !strings.HasPrefix(mediaType, "multipart/form-data") {
		return nil, fmt.Errorf("content type = %q, want multipart/form-data", mediaType)
	}
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	fields := make(map[string]string)
	for key, values := range req.MultipartForm.Value {
		if len(values) > 0 {
			fields[key] = values[0]
		}
	}
	return fields, nil
}

func jsonResponse(status int, body string) *http.Response {
	return textResponse(status, body, map[string]string{"Content-Type": "application/json"})
}

func textResponse(status int, body string, headers map[string]string) *http.Response {
	h := make(http.Header)
	for key, value := range headers {
		h.Set(key, value)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
