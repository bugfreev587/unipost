package platform

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

func TestThreadsGetUserIDNon2XXIncludesStatusBody(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			name:   "invalid token",
			status: http.StatusUnauthorized,
			body:   `{"error":{"message":"Invalid OAuth access token"}}`,
			want:   `threads get user id failed (401): {"error":{"message":"Invalid OAuth access token"}}`,
		},
		{
			name:   "missing permission",
			status: http.StatusForbidden,
			body:   `{"error":{"message":"Missing required permission threads_basic"}}`,
			want:   `threads get user id failed (403): {"error":{"message":"Missing required permission threads_basic"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := newThreadsAdapterForUserIDTest(tt.status, tt.body)

			_, err := adapter.getUserID(context.Background(), "threads-token")
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); !strings.Contains(got, tt.want) {
				t.Fatalf("error = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestThreadsGetUserIDDecodeError(t *testing.T) {
	adapter := newThreadsAdapterForUserIDTest(http.StatusOK, `{"id":`)

	_, err := adapter.getUserID(context.Background(), "threads-token")
	if err == nil {
		t.Fatal("expected decode error")
	}
	if got := err.Error(); !strings.Contains(got, "threads get user id decode") {
		t.Fatalf("error = %q, want decode context", got)
	}
}

func TestThreadsGetUserIDEmptyID(t *testing.T) {
	adapter := newThreadsAdapterForUserIDTest(http.StatusOK, `{"id":""}`)

	_, err := adapter.getUserID(context.Background(), "threads-token")
	if err == nil {
		t.Fatal("expected empty id error")
	}
	if got := err.Error(); !strings.Contains(got, `threads get user id: empty id`) {
		t.Fatalf("error = %q, want empty-id context", got)
	}
}

func TestThreadsGetUserIDSuccess(t *testing.T) {
	adapter := newThreadsAdapterForUserIDTest(http.StatusOK, `{"id":"th_123"}`)

	got, err := adapter.getUserID(context.Background(), "threads-token")
	if err != nil {
		t.Fatalf("getUserID: %v", err)
	}
	if got != "th_123" {
		t.Fatalf("id = %q, want th_123", got)
	}
}

func TestThreadsExchangeCodeFailsWhenLongLivedTokenSwapFails(t *testing.T) {
	adapter := &ThreadsAdapter{client: &http.Client{Transport: routeResponseTransport{
		routes: map[string]routeResponse{
			"POST graph.threads.net/oauth/access_token": {
				status: http.StatusOK,
				body:   `{"access_token":"short-token","token_type":"bearer","user_id":"th_123"}`,
			},
			"GET graph.threads.net/access_token": {
				status: http.StatusBadRequest,
				body:   `{"error":{"message":"Cannot exchange token","type":"OAuthException"}}`,
			},
		},
	}}}

	_, err := adapter.ExchangeCode(context.Background(), OAuthConfig{
		ClientID:     "threads-client",
		ClientSecret: "threads-secret",
		TokenURL:     "https://graph.threads.net/oauth/access_token",
		RedirectURL:  "https://api.example.com/v1/oauth/callback/threads",
	}, "oauth-code")
	if err == nil {
		t.Fatal("expected long-lived token exchange failure")
	}
	if got := err.Error(); !strings.Contains(got, "threads long-lived token exchange failed") {
		t.Fatalf("error = %q, want long-lived exchange context", got)
	}
}

func TestThreadsExchangeCodeRejectsShortLivedLongLivedToken(t *testing.T) {
	adapter := &ThreadsAdapter{client: &http.Client{Transport: routeResponseTransport{
		routes: map[string]routeResponse{
			"POST graph.threads.net/oauth/access_token": {
				status: http.StatusOK,
				body:   `{"access_token":"short-token","token_type":"bearer","user_id":"th_123"}`,
			},
			"GET graph.threads.net/access_token": {
				status: http.StatusOK,
				body:   `{"access_token":"still-short-token","expires_in":3600}`,
			},
		},
	}}}

	_, err := adapter.ExchangeCode(context.Background(), OAuthConfig{
		ClientID:     "threads-client",
		ClientSecret: "threads-secret",
		TokenURL:     "https://graph.threads.net/oauth/access_token",
		RedirectURL:  "https://api.example.com/v1/oauth/callback/threads",
	}, "oauth-code")
	if err == nil {
		t.Fatal("expected short-lived token rejection")
	}
	if got := err.Error(); !strings.Contains(got, "short-lived Threads token") {
		t.Fatalf("error = %q, want short-lived token context", got)
	}
}

func newThreadsAdapterForUserIDTest(status int, body string) *ThreadsAdapter {
	return &ThreadsAdapter{client: &http.Client{Transport: staticResponseTransport{
		status: status,
		body:   body,
	}}}
}

type staticResponseTransport struct {
	status int
	body   string
}

func (t staticResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.status,
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type routeResponse struct {
	status int
	body   string
}

type routeResponseTransport struct {
	routes map[string]routeResponse
}

func (t routeResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.Method + " " + req.URL.Host + req.URL.Path
	if req.Method == http.MethodPost {
		_ = req.ParseForm()
		req.URL.RawQuery = url.Values(req.Form).Encode()
	}
	resp, ok := t.routes[key]
	if !ok {
		resp = routeResponse{status: http.StatusNotFound, body: `{"error":"missing test route"}`}
	}
	return &http.Response{
		StatusCode: resp.status,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

// --- Container readiness ---------------------------------------------------

// threadsScriptedTransport answers each Threads endpoint from a script so the
// state machine can be driven without real delays or a live Graph API.
type threadsScriptedTransport struct {
	statuses     []string // successive bodies for GET container status
	statusIdx    int
	statusCodes  []int // optional per-call HTTP status, defaults to 200
	publishBody  string
	publishCode  int
	containerIDs []string // successive ids returned by container creation
	containerIdx int

	statusCalls    int
	publishCalls   int
	containerCalls int
	lastPublishURL string
}

func (t *threadsScriptedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	respond := func(code int, body string) (*http.Response, error) {
		return &http.Response{
			StatusCode: code,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}
	path := req.URL.Path
	switch {
	case strings.Contains(path, "threads_publish"):
		t.publishCalls++
		t.lastPublishURL = req.URL.String()
		code := t.publishCode
		if code == 0 {
			code = http.StatusOK
		}
		body := t.publishBody
		if body == "" {
			body = `{"id":"published_1"}`
		}
		return respond(code, body)
	case strings.HasSuffix(path, "/threads"):
		t.containerCalls++
		id := "container_new"
		if t.containerIdx < len(t.containerIDs) {
			id = t.containerIDs[t.containerIdx]
			t.containerIdx++
		}
		return respond(http.StatusOK, `{"id":"`+id+`"}`)
	case strings.Contains(req.URL.RawQuery, "fields=id%2Cstatus%2Cerror_message"),
		strings.Contains(req.URL.RawQuery, "fields=id,status,error_message"):
		t.statusCalls++
		code := http.StatusOK
		if t.statusIdx < len(t.statusCodes) {
			code = t.statusCodes[t.statusIdx]
		}
		body := `{"status":"FINISHED"}`
		if len(t.statuses) > 0 {
			// Repeat the final scripted entry rather than falling back to
			// FINISHED: a test that scripts a permanent IN_PROGRESS must not
			// quietly succeed once the script runs out.
			idx := t.statusIdx
			if idx >= len(t.statuses) {
				idx = len(t.statuses) - 1
			}
			body = t.statuses[idx]
		}
		t.statusIdx++
		return respond(code, body)
	case strings.Contains(req.URL.RawQuery, "permalink"):
		return respond(http.StatusOK, `{"permalink":"https://www.threads.net/@u/post/abc"}`)
	default:
		// getUserID and profile lookups
		return respond(http.StatusOK, `{"id":"user_1","username":"u"}`)
	}
}

func newThreadsAdapterScripted(tr *threadsScriptedTransport) *ThreadsAdapter {
	return &ThreadsAdapter{client: &http.Client{Transport: tr}}
}

// withFastThreadsPolling collapses the poll interval so the state machine runs
// at test speed. The budget stays proportionally meaningful.
func withFastThreadsPolling(t *testing.T, budget time.Duration) {
	t.Helper()
	prevInterval, prevBudget := threadsContainerPollInterval, threadsContainerPollBudget
	threadsContainerPollInterval = time.Millisecond
	threadsContainerPollBudget = budget
	t.Cleanup(func() {
		threadsContainerPollInterval = prevInterval
		threadsContainerPollBudget = prevBudget
	})
}

func TestThreadsPublishesOnlyAfterContainerIsFinished(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{statuses: []string{
		`{"status":"IN_PROGRESS"}`,
		`{"status":"IN_PROGRESS"}`,
		`{"status":"FINISHED"}`,
	}}
	adapter := newThreadsAdapterScripted(tr)

	res, err := adapter.Post(context.Background(), "threads-secret-token", "caption",
		[]MediaItem{{URL: "https://example.com/v.mp4", Kind: MediaKindVideo}}, nil)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if res.ExternalID != "published_1" {
		t.Errorf("external id = %q", res.ExternalID)
	}
	if tr.statusCalls != 3 {
		t.Errorf("status polls = %d, want 3", tr.statusCalls)
	}
	if tr.publishCalls != 1 {
		t.Errorf("publish calls = %d, want 1", tr.publishCalls)
	}
}

func TestThreadsImmediateFinishedPublishesWithoutFixedDelay(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{statuses: []string{`{"status":"FINISHED"}`}}
	adapter := newThreadsAdapterScripted(tr)

	start := time.Now()
	if _, err := adapter.Post(context.Background(), "threads-secret-token", "caption",
		[]MediaItem{{URL: "https://example.com/v.mp4", Kind: MediaKindVideo}}, nil); err != nil {
		t.Fatalf("Post: %v", err)
	}
	// The old adapter slept a flat 30s for any video post.
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("publish took %s, want no fixed video delay", elapsed)
	}
	if tr.statusCalls != 1 || tr.publishCalls != 1 {
		t.Fatalf("status/publish calls = %d/%d, want 1/1", tr.statusCalls, tr.publishCalls)
	}
}

func TestThreadsContainerErrorClassification(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		wantCode    string
		wantRetry   bool
		wantInMsg   string
		wantPublish int
	}{
		{name: "invalid duration is the customer's media", errorMsg: "INVALID_DURATION", wantCode: "media_error", wantRetry: false, wantInMsg: "INVALID_DURATION"},
		{name: "invalid aspect ratio meta spelling", errorMsg: "INVALID_ASPEC_RATIO", wantCode: "media_error", wantRetry: false, wantInMsg: "INVALID_ASPEC_RATIO"},
		{name: "download failure is provider side", errorMsg: "FAILED_DOWNLOADING_VIDEO", wantCode: "temporary_platform_error", wantRetry: true},
		{name: "processing failure is provider side", errorMsg: "FAILED_PROCESSING_VIDEO", wantCode: "temporary_platform_error", wantRetry: true},
		{name: "unknown stays retriable", errorMsg: "UNKNOWN", wantCode: "temporary_platform_error", wantRetry: true},
		{name: "unrecognised value stays retriable", errorMsg: "SOMETHING_NEW", wantCode: "temporary_platform_error", wantRetry: true},
		{name: "missing value stays retriable", errorMsg: "", wantCode: "temporary_platform_error", wantRetry: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFastThreadsPolling(t, time.Second)
			tr := &threadsScriptedTransport{statuses: []string{
				`{"status":"ERROR","error_message":"` + tt.errorMsg + `"}`,
			}}
			adapter := newThreadsAdapterScripted(tr)

			_, err := adapter.Post(context.Background(), "threads-secret-token", "c",
				[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			assertThreadsContract(t, err, tt.wantCode, "container_processing", tt.wantRetry)
			if tt.wantInMsg != "" && !strings.Contains(err.Error(), tt.wantInMsg) {
				t.Errorf("message %q missing %q", err.Error(), tt.wantInMsg)
			}
			if tr.publishCalls != tt.wantPublish {
				t.Errorf("publish calls = %d, want %d", tr.publishCalls, tt.wantPublish)
			}
		})
	}
}

func TestThreadsExpiredContainerIsRetriableAndDoesNotPublish(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{statuses: []string{`{"status":"EXPIRED"}`}}
	adapter := newThreadsAdapterScripted(tr)

	_, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	assertThreadsContract(t, err, "temporary_platform_error", "container_processing", true)
	if tr.publishCalls != 0 {
		t.Errorf("publish calls = %d, want 0", tr.publishCalls)
	}
}

func TestThreadsTimeoutIsRetriableWithDiagnostics(t *testing.T) {
	withFastThreadsPolling(t, 10*time.Millisecond)
	tr := &threadsScriptedTransport{statuses: []string{`{"status":"IN_PROGRESS"}`}}
	adapter := newThreadsAdapterScripted(tr)

	_, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	assertThreadsContract(t, err, "temporary_platform_error", "container_processing", true)
	for _, want := range []string{"container_id=", "poll_count=", "elapsed_ms=", "status=IN_PROGRESS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("diagnostics missing %q: %s", want, err.Error())
		}
	}
	if strings.Contains(err.Error(), "threads-secret-token") {
		t.Fatalf("diagnostics leaked the access token: %s", err.Error())
	}
	if tr.publishCalls != 0 {
		t.Errorf("publish calls = %d, want 0", tr.publishCalls)
	}
}

func TestThreadsUnknownStatusPublishesAfterBoundedGrace(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	// A container type that answers 200 but exposes no documented status must
	// not stall for the whole budget — that would break text and image posts
	// that publish fine today.
	tr := &threadsScriptedTransport{statuses: []string{`{"id":"c1"}`, `{"id":"c1"}`, `{"id":"c1"}`}}
	adapter := newThreadsAdapterScripted(tr)

	if _, err := adapter.Post(context.Background(), "threads-secret-token", "text only", nil, nil); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if tr.statusCalls != threadsUnknownStatusGraceAttempts {
		t.Errorf("status polls = %d, want the grace limit %d", tr.statusCalls, threadsUnknownStatusGraceAttempts)
	}
	if tr.publishCalls != 1 {
		t.Errorf("publish calls = %d, want 1", tr.publishCalls)
	}
}

func TestThreadsPollRetriesTransientHTTPStatuses(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{
		statuses:    []string{`{}`, `{}`, `{"status":"FINISHED"}`},
		statusCodes: []int{http.StatusTooManyRequests, http.StatusBadGateway, http.StatusOK},
	}
	adapter := newThreadsAdapterScripted(tr)

	if _, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, nil); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if tr.statusCalls != 3 || tr.publishCalls != 1 {
		t.Fatalf("status/publish = %d/%d, want 3/1", tr.statusCalls, tr.publishCalls)
	}
}

func TestThreadsPollStopsImmediatelyOnAuthFailure(t *testing.T) {
	withFastThreadsPolling(t, time.Minute)
	tr := &threadsScriptedTransport{
		statuses:    []string{`{"error":{"code":190,"message":"Error validating access token"}}`},
		statusCodes: []int{http.StatusUnauthorized},
	}
	adapter := newThreadsAdapterScripted(tr)

	_, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, nil)
	if err == nil {
		t.Fatal("expected auth error")
	}
	if tr.statusCalls != 1 {
		t.Errorf("status polls = %d, want 1 (no budget burn on a definitive 4xx)", tr.statusCalls)
	}
	// Surfaced verbatim so the existing Meta reconnect taxonomy can classify
	// it; the adapter deliberately does not type this one itself.
	for _, want := range []string{"(401)", "Error validating access token"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
	if strings.Contains(err.Error(), "threads-secret-token") {
		t.Fatalf("auth failure leaked the access token: %s", err.Error())
	}
}

func TestThreadsPollingRespectsContextCancellation(t *testing.T) {
	withFastThreadsPolling(t, time.Hour)
	tr := &threadsScriptedTransport{statuses: []string{`{"status":"IN_PROGRESS"}`}}
	adapter := newThreadsAdapterScripted(tr)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()

	done := make(chan error, 1)
	go func() {
		_, err := adapter.Post(ctx, "threads-secret-token", "c",
			[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("polling did not stop promptly on cancellation")
	}
}

func TestThreadsPublish4279009IsRetriable(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{
		statuses:    []string{`{"status":"FINISHED"}`},
		publishCode: http.StatusBadRequest,
		publishBody: `{"error":{"message":"The requested resource does not exist","type":"OAuthException","code":24,"error_subcode":4279009,"is_transient":false}}`,
	}
	adapter := newThreadsAdapterScripted(tr)

	_, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, nil)
	if err == nil {
		t.Fatal("expected publish error")
	}
	assertThreadsContract(t, err, "temporary_platform_error", "dispatch", true)
}

// --- Publish-token resume --------------------------------------------------

type recordingTokenSink struct{ tokens []string }

func (s *recordingTokenSink) opts() map[string]any {
	return map[string]any{
		OptOnPublishToken: func(tok string) error {
			s.tokens = append(s.tokens, tok)
			return nil
		},
	}
}

func TestThreadsPersistsContainerTokenBeforePolling(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{
		statuses:     []string{`{"status":"FINISHED"}`},
		containerIDs: []string{"container_abc"},
	}
	adapter := newThreadsAdapterScripted(tr)
	sink := &recordingTokenSink{}

	if _, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, sink.opts()); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if len(sink.tokens) != 1 || sink.tokens[0] != "container_abc" {
		t.Fatalf("persisted tokens = %v, want [container_abc]", sink.tokens)
	}
}

func TestThreadsResumeSkipsContainerCreation(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{statuses: []string{`{"status":"FINISHED"}`}}
	adapter := newThreadsAdapterScripted(tr)

	opts := map[string]any{OptResumePublishToken: "container_prior"}
	if _, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, opts); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if tr.containerCalls != 0 {
		t.Errorf("container creations = %d, want 0 on resume", tr.containerCalls)
	}
	if tr.publishCalls != 1 {
		t.Errorf("publish calls = %d, want 1", tr.publishCalls)
	}
}

// The duplicate-publish guard: a prior attempt's threads_publish landed but we
// never observed the response. Publishing again would post twice.
func TestThreadsResumePublishedContainerDoesNotPublishAgain(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{statuses: []string{`{"status":"PUBLISHED"}`}}
	adapter := newThreadsAdapterScripted(tr)

	opts := map[string]any{OptResumePublishToken: "container_already_live"}
	res, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, opts)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if tr.publishCalls != 0 {
		t.Fatalf("publish calls = %d, want 0 — the post was already delivered", tr.publishCalls)
	}
	if tr.containerCalls != 0 {
		t.Errorf("container creations = %d, want 0", tr.containerCalls)
	}
	if res.ExternalID != "container_already_live" {
		t.Errorf("external id = %q, want the already-published container", res.ExternalID)
	}
}

func TestThreadsResumeExpiredContainerCreatesFreshOne(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{
		statuses:     []string{`{"status":"EXPIRED"}`, `{"status":"FINISHED"}`},
		containerIDs: []string{"container_fresh"},
	}
	adapter := newThreadsAdapterScripted(tr)
	sink := &recordingTokenSink{}
	opts := sink.opts()
	opts[OptResumePublishToken] = "container_stale"

	if _, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, opts); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if tr.containerCalls != 1 {
		t.Errorf("container creations = %d, want 1", tr.containerCalls)
	}
	if len(sink.tokens) != 1 || sink.tokens[0] != "container_fresh" {
		t.Fatalf("persisted tokens = %v, want the replacement container", sink.tokens)
	}
	if tr.publishCalls != 1 {
		t.Errorf("publish calls = %d, want 1", tr.publishCalls)
	}
}

func TestThreadsResumeUnknownStatusCreatesFreshContainer(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	// Resumed container answers 200 with nothing recognizable. The grace path
	// would publish it, but we cannot vouch for an unknown object on resume,
	// so a definitive 4xx-style unusable response must rebuild instead.
	tr := &threadsScriptedTransport{
		statuses:     []string{`{"error":{"message":"not found"}}`, `{"status":"FINISHED"}`},
		statusCodes:  []int{http.StatusNotFound, http.StatusOK},
		containerIDs: []string{"container_rebuilt"},
	}
	adapter := newThreadsAdapterScripted(tr)
	opts := map[string]any{OptResumePublishToken: "container_gone"}

	if _, err := adapter.Post(context.Background(), "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, opts); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if tr.containerCalls != 1 {
		t.Errorf("container creations = %d, want 1 rebuild", tr.containerCalls)
	}
	if tr.publishCalls != 1 {
		t.Errorf("publish calls = %d, want 1", tr.publishCalls)
	}
}

func TestThreadsCarouselWaitsOnParentContainer(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{
		statuses:     []string{`{"status":"IN_PROGRESS"}`, `{"status":"FINISHED"}`},
		containerIDs: []string{"child_1", "child_2", "parent_1"},
	}
	adapter := newThreadsAdapterScripted(tr)
	sink := &recordingTokenSink{}

	if _, err := adapter.Post(context.Background(), "threads-secret-token", "c", []MediaItem{
		{URL: "https://e.com/a.jpg", Kind: MediaKindImage},
		{URL: "https://e.com/b.jpg", Kind: MediaKindImage},
	}, sink.opts()); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if tr.containerCalls != 3 {
		t.Errorf("container creations = %d, want 2 children + 1 parent", tr.containerCalls)
	}
	// Only the parent is resumable; children are cheap to recreate.
	if len(sink.tokens) != 1 || sink.tokens[0] != "parent_1" {
		t.Fatalf("persisted tokens = %v, want only the parent", sink.tokens)
	}
	if tr.statusCalls != 2 || tr.publishCalls != 1 {
		t.Errorf("status/publish = %d/%d, want 2/1", tr.statusCalls, tr.publishCalls)
	}
}

func TestThreadsReplyWaitsForContainerAndBindsContext(t *testing.T) {
	withFastThreadsPolling(t, time.Second)
	tr := &threadsScriptedTransport{statuses: []string{`{"status":"IN_PROGRESS"}`, `{"status":"FINISHED"}`}}
	adapter := newThreadsAdapterScripted(tr)

	if _, err := adapter.ReplyToComment(context.Background(), "threads-secret-token", "parent_post", "hi"); err != nil {
		t.Fatalf("ReplyToComment: %v", err)
	}
	if tr.statusCalls != 2 {
		t.Errorf("reply status polls = %d, want 2", tr.statusCalls)
	}
	if tr.publishCalls != 1 {
		t.Errorf("reply publish calls = %d, want 1", tr.publishCalls)
	}
}

func assertThreadsContract(t *testing.T, err error, wantCode, wantStage string, wantRetriable bool) {
	t.Helper()
	var carrier interface{ FailureContractFields() map[string]any }
	if !errors.As(err, &carrier) {
		t.Fatalf("error %v does not carry a typed failure contract", err)
	}
	fields := carrier.FailureContractFields()
	if got, _ := fields["error_code"].(string); got != wantCode {
		t.Errorf("error_code = %q, want %q", got, wantCode)
	}
	if got, _ := fields["failure_stage"].(string); got != wantStage {
		t.Errorf("failure_stage = %q, want %q", got, wantStage)
	}
	if got, _ := fields["is_retriable"].(bool); got != wantRetriable {
		t.Errorf("is_retriable = %v, want %v", got, wantRetriable)
	}
}

// The observed container status has to be queryable through the integration
// log surface, not only process logs — otherwise there is no way to answer
// "what status does each container type actually report?" from production.
func TestThreadsContainerRecordsOneDispatchEventPerWait(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []string
		budget     time.Duration
		wantName   string
		wantStatus string
		wantReason string
	}{
		{
			name:       "ready wait records the observed status",
			statuses:   []string{`{"status":"IN_PROGRESS"}`, `{"status":"FINISHED"}`},
			budget:     time.Second,
			wantName:   integrationlogs.ActionThreadsContainerReady,
			wantStatus: "succeeded",
			wantReason: "FINISHED",
		},
		{
			name:       "container error records the provider reason",
			statuses:   []string{`{"status":"ERROR","error_message":"INVALID_DURATION"}`},
			budget:     time.Second,
			wantName:   integrationlogs.ActionThreadsContainerFailed,
			wantStatus: "failed",
			wantReason: "ERROR:INVALID_DURATION",
		},
		{
			name:       "unknown status records the grace outcome so it is alertable",
			statuses:   []string{`{"id":"c"}`, `{"id":"c"}`, `{"id":"c"}`},
			budget:     time.Second,
			wantName:   integrationlogs.ActionThreadsContainerReady,
			wantStatus: "succeeded",
			wantReason: "UNKNOWN_ASSUMED_READY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFastThreadsPolling(t, tt.budget)
			rec := NewDispatchEventRecorder()
			ctx := WithDispatchMetadata(context.Background(), DispatchMetadata{Events: rec})
			adapter := newThreadsAdapterScripted(&threadsScriptedTransport{statuses: tt.statuses})

			_, _ = adapter.Post(ctx, "threads-secret-token", "c",
				[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo, Origin: MediaOriginManaged}}, nil)

			events := rec.Snapshot()
			if len(events) != 1 {
				t.Fatalf("dispatch events = %d, want exactly 1 summary per wait", len(events))
			}
			got := events[0]
			if got.Name != tt.wantName {
				t.Errorf("event name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("event status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.Reason != tt.wantReason {
				t.Errorf("event reason = %q, want %q", got.Reason, tt.wantReason)
			}
			if got.CustomerInput {
				t.Error("managed media marked as customer input")
			}
		})
	}
}

// A wait that polls to the budget must still leave room for its terminal
// event inside the sixteen-event dispatch buffer.
func TestThreadsLongWaitStaysInsideDispatchEventCap(t *testing.T) {
	withFastThreadsPolling(t, 60*time.Millisecond)
	rec := NewDispatchEventRecorder()
	ctx := WithDispatchMetadata(context.Background(), DispatchMetadata{Events: rec})
	adapter := newThreadsAdapterScripted(&threadsScriptedTransport{statuses: []string{`{"status":"IN_PROGRESS"}`}})

	_, err := adapter.Post(ctx, "threads-secret-token", "c",
		[]MediaItem{{URL: "https://e.com/v.mp4", Kind: MediaKindVideo}}, nil)
	if err == nil {
		t.Fatal("expected a timeout")
	}
	events := rec.Snapshot()
	if len(events) != 1 {
		t.Fatalf("dispatch events = %d after many polls, want 1", len(events))
	}
	if events[0].Name != integrationlogs.ActionThreadsContainerFailed || !events[0].Retriable {
		t.Errorf("terminal event = %s retriable=%v, want the retriable failure", events[0].Name, events[0].Retriable)
	}
}
