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
	"net/http/httptest"
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

func TestTwitterInboxFetchMentionsUsesOfficialUserMentionsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/2/users/user-1/mentions" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer user-token" {
			t.Fatalf("Authorization = %q", got)
		}
		want := url.Values{
			"tweet.fields":     {"created_at,author_id,conversation_id,referenced_tweets"},
			"expansions":       {"author_id"},
			"user.fields":      {"id,name,username,profile_image_url"},
			"max_results":      {"25"},
			"pagination_token": {"next-page"},
			"start_time":       {"2026-07-15T12:00:00Z"},
		}
		if !reflect.DeepEqual(r.URL.Query(), want) {
			t.Fatalf("query = %#v, want %#v", r.URL.Query(), want)
		}
		_, _ = io.WriteString(w, `{
			"data":[{
				"id":"tweet-1",
				"text":"@unipost hello",
				"author_id":"author-1",
				"created_at":"2026-07-16T10:00:00Z",
				"conversation_id":"conversation-1",
				"referenced_tweets":[{"type":"replied_to","id":"parent-1"}]
			}],
			"includes":{"users":[{
				"id":"author-1",
				"name":"Robin",
				"username":"robin",
				"profile_image_url":"https://example.test/robin.png"
			}]},
			"meta":{"next_token":"page-2"}
		}`)
	}))
	defer server.Close()

	adapter := &TwitterAdapter{client: server.Client(), apiBaseURL: server.URL}
	page, err := adapter.FetchInboxMentions(
		context.Background(),
		"user-token",
		"user-1",
		time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC),
		"next-page",
		25,
	)
	if err != nil {
		t.Fatalf("FetchInboxMentions: %v", err)
	}
	if page.NextToken != "page-2" || len(page.Entries) != 1 {
		t.Fatalf("page = %+v", page)
	}
	got := page.Entries[0]
	if got.ExternalID != "tweet-1" || got.ParentExternalID != "parent-1" ||
		got.ThreadKey != "conversation-1" || got.AuthorID != "author-1" ||
		got.AuthorName != "Robin" || got.AuthorAvatarURL == "" || !got.ReplyEligible {
		t.Fatalf("entry = %+v", got)
	}
}

func TestTwitterInboxFetchDMEventsUsesOfficialThirtyDayLookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/2/dm_events" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer user-token" {
			t.Fatalf("Authorization = %q", got)
		}
		want := url.Values{
			"dm_event.fields":  {"id,text,event_type,created_at,sender_id,participant_ids,dm_conversation_id"},
			"expansions":       {"sender_id"},
			"user.fields":      {"id,name,username,profile_image_url"},
			"event_types":      {"MessageCreate"},
			"max_results":      {"100"},
			"pagination_token": {"next-page"},
		}
		if !reflect.DeepEqual(r.URL.Query(), want) {
			t.Fatalf("query = %#v, want %#v", r.URL.Query(), want)
		}
		_, _ = io.WriteString(w, `{
			"data":[
				{
					"id":"dm-1",
					"event_type":"MessageCreate",
					"text":"private",
					"sender_id":"author-1",
					"participant_ids":["user-1","author-1"],
					"dm_conversation_id":"conversation-1",
					"created_at":"2026-07-16T10:00:00Z"
				},
				{
					"id":"dm-old",
					"event_type":"MessageCreate",
					"text":"old private",
					"sender_id":"author-1",
					"participant_ids":["user-1","author-1"],
					"dm_conversation_id":"conversation-1",
					"created_at":"2026-06-15T10:00:00Z"
				}
			],
			"includes":{"users":[{"id":"author-1","name":"Robin","username":"robin"}]},
			"meta":{"next_token":"page-2"}
		}`)
	}))
	defer server.Close()

	adapter := &TwitterAdapter{client: server.Client(), apiBaseURL: server.URL}
	page, err := adapter.FetchInboxDMEvents(
		context.Background(),
		"user-token",
		time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC),
		"next-page",
		100,
	)
	if err != nil {
		t.Fatalf("FetchInboxDMEvents: %v", err)
	}
	if page.NextToken != "page-2" || len(page.Entries) != 1 || !page.HorizonReached {
		t.Fatalf("page = %+v", page)
	}
	got := page.Entries[0]
	if got.ExternalID != "dm-1" || got.ParentExternalID != "conversation-1" ||
		got.ThreadKey != "conversation-1" || got.AuthorName != "Robin" {
		t.Fatalf("entry = %+v", got)
	}
}

func TestTwitterInboxMentionsAllowsOneResultWithoutPaidOverRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("max_results"); got != "1" {
			t.Fatalf("max_results = %q, want 1", got)
		}
		_, _ = io.WriteString(w, `{"data":[],"meta":{}}`)
	}))
	defer server.Close()

	adapter := &TwitterAdapter{client: server.Client(), apiBaseURL: server.URL}
	if _, err := adapter.FetchInboxMentions(
		context.Background(),
		"user-token",
		"user-1",
		time.Time{},
		"",
		1,
	); err != nil {
		t.Fatalf("FetchInboxMentions: %v", err)
	}
}

func TestTwitterInboxReplyAndDMSendUseOfficialOAuth2UserEndpoints(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer user-token" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		switch r.URL.Path {
		case "/2/tweets":
			reply, _ := body["reply"].(map[string]any)
			if body["text"] != "public reply" || reply["in_reply_to_tweet_id"] != "tweet-1" {
				t.Fatalf("tweet payload = %#v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"data":{"id":"tweet-2","text":"public reply"}}`)
		case "/2/dm_conversations/conversation-1/messages":
			if body["text"] != "conversation dm" {
				t.Fatalf("conversation DM payload = %#v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"data":{"dm_event_id":"dm-2","dm_conversation_id":"conversation-1"}}`)
		case "/2/dm_conversations/with/participant-1/messages":
			if body["text"] != "participant dm" {
				t.Fatalf("participant DM payload = %#v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"data":{"dm_event_id":"dm-3","dm_conversation_id":"conversation-2"}}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := &TwitterAdapter{client: server.Client(), apiBaseURL: server.URL}
	reply, err := adapter.SendInboxReply(context.Background(), "user-token", "tweet-1", "public reply")
	if err != nil || reply.ExternalID != "tweet-2" || reply.URL != "https://x.com/i/status/tweet-2" {
		t.Fatalf("SendInboxReply = %+v, %v", reply, err)
	}
	conversationDM, err := adapter.SendInboxDMToConversation(context.Background(), "user-token", "conversation-1", "conversation dm")
	if err != nil || conversationDM.ExternalID != "dm-2" || conversationDM.ConversationID != "conversation-1" {
		t.Fatalf("SendInboxDMToConversation = %+v, %v", conversationDM, err)
	}
	participantDM, err := adapter.SendInboxDMToParticipant(context.Background(), "user-token", "participant-1", "participant dm")
	if err != nil || participantDM.ExternalID != "dm-3" || participantDM.ConversationID != "conversation-2" {
		t.Fatalf("SendInboxDMToParticipant = %+v, %v", participantDM, err)
	}
	if want := []string{
		"/2/tweets",
		"/2/dm_conversations/conversation-1/messages",
		"/2/dm_conversations/with/participant-1/messages",
	}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestTwitterInboxWriteTransportErrorsPreserveOutcomeStage(t *testing.T) {
	adapter := &TwitterAdapter{
		apiBaseURL: "https://api.x.test",
		client: &http.Client{Transport: twitterRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})},
	}
	_, err := adapter.SendInboxReply(context.Background(), "user-token", "tweet-1", "reply")
	if err == nil || !strings.HasPrefix(err.Error(), "create_tweet_reply timeout") {
		t.Fatalf("reply err = %v", err)
	}
	_, err = adapter.SendInboxDMToConversation(context.Background(), "user-token", "conversation-1", "dm")
	if err == nil || !strings.HasPrefix(err.Error(), "create_dm timeout") {
		t.Fatalf("dm err = %v", err)
	}
}

func TestTwitterInboxMalformedCreatedResponseIsOutcomeUnknown(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		response string
		call     func(*TwitterAdapter) error
	}{
		{
			name:     "reply malformed JSON",
			path:     "/2/tweets",
			response: `{"data":`,
			call: func(adapter *TwitterAdapter) error {
				_, err := adapter.SendInboxReply(context.Background(), "user-token", "tweet-1", "reply")
				return err
			},
		},
		{
			name:     "reply missing post id",
			path:     "/2/tweets",
			response: `{"data":{"text":"reply"}}`,
			call: func(adapter *TwitterAdapter) error {
				_, err := adapter.SendInboxReply(context.Background(), "user-token", "tweet-1", "reply")
				return err
			},
		},
		{
			name:     "reply trailing malformed JSON",
			path:     "/2/tweets",
			response: `{"data":{"id":"tweet-2"}} trailing`,
			call: func(adapter *TwitterAdapter) error {
				_, err := adapter.SendInboxReply(context.Background(), "user-token", "tweet-1", "reply")
				return err
			},
		},
		{
			name:     "DM malformed JSON",
			path:     "/2/dm_conversations/conversation-1/messages",
			response: `{"data":`,
			call: func(adapter *TwitterAdapter) error {
				_, err := adapter.SendInboxDMToConversation(context.Background(), "user-token", "conversation-1", "dm")
				return err
			},
		},
		{
			name:     "DM missing event id",
			path:     "/2/dm_conversations/conversation-1/messages",
			response: `{"data":{"dm_conversation_id":"conversation-1"}}`,
			call: func(adapter *TwitterAdapter) error {
				_, err := adapter.SendInboxDMToConversation(context.Background(), "user-token", "conversation-1", "dm")
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					t.Fatalf("path = %q, want %q", r.URL.Path, tt.path)
				}
				w.WriteHeader(http.StatusCreated)
				_, _ = io.WriteString(w, tt.response)
			}))
			defer server.Close()
			adapter := &TwitterAdapter{client: server.Client(), apiBaseURL: server.URL}
			err := tt.call(adapter)
			if err == nil || !xWriteOutcomeUnknownForTest(err) {
				t.Fatalf("err = %v, want outcome-unknown stage", err)
			}
		})
	}
}

func xWriteOutcomeUnknownForTest(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.HasPrefix(message, "create_tweet_reply:") ||
		strings.HasPrefix(message, "create_dm:")
}

func TestTwitterOAuthPKCEExchangeRejectsMalformedTokenJSON(t *testing.T) {
	adapter := twitterOAuthResponseAdapter(t, `{`, http.StatusOK, validTwitterUserInfoJSON)
	result, err := adapter.ExchangeCode(context.Background(), validTwitterOAuthConfig(), "authorization-code")
	if err == nil {
		t.Fatal("ExchangeCode error = nil, want malformed token JSON error")
	}
	if result != nil {
		t.Fatalf("ConnectResult = %#v, want nil", result)
	}
}

func TestTwitterOAuthPKCEExchangeRejectsEmptyAccessToken(t *testing.T) {
	adapter := twitterOAuthResponseAdapter(t, `{
		"access_token":"",
		"refresh_token":"refresh-token",
		"expires_in":7200,
		"scope":""
	}`, http.StatusOK, validTwitterUserInfoJSON)
	result, err := adapter.ExchangeCode(context.Background(), validTwitterOAuthConfig(), "authorization-code")
	if err == nil {
		t.Fatal("ExchangeCode error = nil, want empty access token error")
	}
	if result != nil {
		t.Fatalf("ConnectResult = %#v, want nil", result)
	}
}

func TestTwitterOAuthPKCEExchangeRejectsNonPositiveExpiry(t *testing.T) {
	adapter := twitterOAuthResponseAdapter(t, `{
		"access_token":"access-token",
		"refresh_token":"refresh-token",
		"expires_in":0,
		"scope":""
	}`, http.StatusOK, validTwitterUserInfoJSON)
	result, err := adapter.ExchangeCode(context.Background(), validTwitterOAuthConfig(), "authorization-code")
	if err == nil {
		t.Fatal("ExchangeCode error = nil, want invalid expires_in error")
	}
	if result != nil {
		t.Fatalf("ConnectResult = %#v, want nil", result)
	}
}

func TestTwitterOAuthPKCEExchangeRejectsUsersMeNon2xxWithoutBodyLeak(t *testing.T) {
	adapter := twitterOAuthResponseAdapter(t, validTwitterTokenJSON, http.StatusUnauthorized, `{"secret":"upstream-sensitive"}`)
	result, err := adapter.ExchangeCode(context.Background(), validTwitterOAuthConfig(), "authorization-code")
	if err == nil {
		t.Fatal("ExchangeCode error = nil, want users/me status error")
	}
	if result != nil {
		t.Fatalf("ConnectResult = %#v, want nil", result)
	}
	if strings.Contains(err.Error(), "upstream-sensitive") || strings.Contains(err.Error(), "access-token") {
		t.Fatalf("error leaked upstream secret/token: %q", err)
	}
}

func TestTwitterOAuthPKCEExchangeRejectsMalformedUsersMeJSON(t *testing.T) {
	adapter := twitterOAuthResponseAdapter(t, validTwitterTokenJSON, http.StatusOK, `{`)
	result, err := adapter.ExchangeCode(context.Background(), validTwitterOAuthConfig(), "authorization-code")
	if err == nil {
		t.Fatal("ExchangeCode error = nil, want malformed users/me JSON error")
	}
	if result != nil {
		t.Fatalf("ConnectResult = %#v, want nil", result)
	}
}

func TestTwitterOAuthPKCEExchangeRejectsEmptyUsersMeID(t *testing.T) {
	adapter := twitterOAuthResponseAdapter(t, validTwitterTokenJSON, http.StatusOK, `{"data":{"username":"robyn"}}`)
	result, err := adapter.ExchangeCode(context.Background(), validTwitterOAuthConfig(), "authorization-code")
	if err == nil {
		t.Fatal("ExchangeCode error = nil, want empty users/me ID error")
	}
	if result != nil {
		t.Fatalf("ConnectResult = %#v, want nil", result)
	}
}

const (
	validTwitterTokenJSON = `{
		"access_token":"access-token",
		"refresh_token":"refresh-token",
		"expires_in":7200,
		"scope":"tweet.read dm.read"
	}`
	validTwitterUserInfoJSON = `{
		"data":{"id":"x-user","username":"robyn","profile_image_url":"https://example/avatar.png"}
	}`
)

func validTwitterOAuthConfig() OAuthConfig {
	return OAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		TokenURL:     "https://token.example/oauth2/token",
		RedirectURL:  "https://api.example/v1/oauth/callback/twitter",
		Scopes:       []string{"tweet.read", "tweet.write", "dm.read", "dm.write"},
		PKCEVerifier: "stored-random-verifier",
	}
}

func twitterOAuthResponseAdapter(t *testing.T, tokenBody string, userStatus int, userBody string) *TwitterAdapter {
	t.Helper()
	return &TwitterAdapter{client: &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "token.example":
			return jsonResponse(http.StatusOK, tokenBody), nil
		case "api.x.com":
			return jsonResponse(userStatus, userBody), nil
		default:
			t.Fatalf("unexpected request URL: %s", req.URL)
			return nil, nil
		}
	})}}
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
