package requestevents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
)

type recordingEnqueuer struct {
	events  []Event
	details []*Detail
}

func (r *recordingEnqueuer) Enqueue(event Event, detail *Detail) {
	r.events = append(r.events, event)
	r.details = append(r.details, detail)
}

func TestMiddlewareRecordsSuccessfulAPIKeyRequestAsMetadataOnly(t *testing.T) {
	recorder := &recordingEnqueuer{}
	router := chi.NewRouter()
	router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
	router.Use(Middleware(recorder))
	router.Post("/v1/posts/{postID}/publish", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"title":"unchanged"}` {
			t.Fatalf("handler body = %q", body)
		}
		w.Header().Set("X-Business-Header", "unchanged")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/42/publish?token=secret", strings.NewReader(`{"title":"unchanged"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted || rr.Body.String() != `{"ok":true}` || rr.Header().Get("X-Business-Header") != "unchanged" {
		t.Fatalf("response changed: status=%d headers=%v body=%q", rr.Code, rr.Header(), rr.Body.String())
	}
	if len(recorder.events) != 1 || len(recorder.details) != 1 || recorder.details[0] != nil {
		t.Fatalf("writes = events %#v details %#v, want one metadata-only event", recorder.events, recorder.details)
	}
	event := recorder.events[0]
	if event.WorkspaceID != "workspace-1" || event.APIKeyID != "api-key-1" || event.StatusCode != http.StatusAccepted {
		t.Fatalf("event identity/status = %#v", event)
	}
	if event.RoutePattern != "/v1/posts/:id/publish" || event.Outcome != OutcomeSuccess || event.HasErrorDetail {
		t.Fatalf("event route/outcome = %#v", event)
	}
}

func TestMiddlewareRecordsFailureWithBoundedRedactedDetail(t *testing.T) {
	recorder := &recordingEnqueuer{}
	router := chi.NewRouter()
	router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
	router.Use(Middleware(recorder))
	router.Post("/v1/test", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"VALIDATION_FAILED","access_token":"response-secret"}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/test?signature=query-secret", strings.NewReader(`{"password":"request-secret","safe":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer header-secret")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "response-secret") {
		t.Fatalf("customer response changed: %d %q", rr.Code, rr.Body.String())
	}
	if len(recorder.events) != 1 || recorder.details[0] == nil {
		t.Fatalf("failure write missing: %#v %#v", recorder.events, recorder.details)
	}
	event, detail := recorder.events[0], recorder.details[0]
	if event.ErrorCode != "validation_failed" || event.Outcome != OutcomeValidationError || !event.HasErrorDetail {
		t.Fatalf("failure event = %#v", event)
	}
	encoded := detail.RequestPayload + detail.ResponsePayload
	for _, secret := range []string{"request-secret", "response-secret", "header-secret", "query-secret"} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("detail leaked %q: %#v", secret, detail)
		}
	}
	if len(detail.QueryKeys) != 0 || detail.ResponseHeaders["Retry-After"] != "10" {
		t.Fatalf("safe detail metadata missing: %#v", detail)
	}
}

func TestMiddlewareKeeps402MetadataOnlyAnd429ResponseOnly(t *testing.T) {
	tests := []struct {
		name              string
		status            int
		wantDetail        bool
		wantRequestStored bool
		wantResponse      bool
	}{
		{"plan gate", http.StatusPaymentRequired, false, false, false},
		{"rate limit", http.StatusTooManyRequests, true, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recordingEnqueuer{}
			router := chi.NewRouter()
			router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
			router.Use(Middleware(recorder))
			router.Post("/v1/test", func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"code":"RATE_LIMITED","message":"bounded"}`))
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/test", strings.NewReader(`{"safe":"request"}`))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			detail := recorder.details[0]
			if (detail != nil) != tc.wantDetail {
				t.Fatalf("detail = %#v, want present=%v", detail, tc.wantDetail)
			}
			if detail != nil {
				if (detail.RequestPayload != "") != tc.wantRequestStored || (detail.ResponsePayload != "") != tc.wantResponse {
					t.Fatalf("detail payload policy = %#v", detail)
				}
			}
		})
	}
}

func TestMiddlewareNeverStoresBinaryRequestBody(t *testing.T) {
	recorder := &recordingEnqueuer{}
	router := chi.NewRouter()
	router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
	router.Use(Middleware(recorder))
	router.Post("/v1/media", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	})

	payload := bytes.Repeat([]byte{0x00, 0x01, 0xff}, 100)
	req := httptest.NewRequest(http.MethodPost, "/v1/media", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "video/mp4")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	detail := recorder.details[0]
	if detail == nil || detail.RequestPayload != "" || detail.RequestStoredBytes != 0 || detail.RequestSHA256 == "" || detail.RequestOmissionReason != "binary_or_multipart" {
		t.Fatalf("binary detail = %#v", detail)
	}
}

func TestTraceIDAcceptsOnlyBoundedCorrelationFormats(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	request.Header.Set("X-Trace-Id", "opaque-customer-secret")
	if got := traceIDFromRequest(request); got != "" {
		t.Fatalf("untrusted trace id was stored: %q", got)
	}
	request.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	if got := traceIDFromRequest(request); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("traceparent id = %q", got)
	}
}

func TestMiddlewareSkipsClerkAndInternalTraffic(t *testing.T) {
	for _, tc := range []struct {
		path     string
		apiKeyID string
	}{
		{"/v1/posts", ""},
		{"/v1/api-metrics/summary", "api-key-1"},
		{"/v1/admin/logs", "api-key-1"},
	} {
		recorder := &recordingEnqueuer{}
		router := chi.NewRouter()
		router.Use(requestEventTestAuth("workspace-1", tc.apiKeyID))
		router.Use(Middleware(recorder))
		router.Get("/*", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if len(recorder.events) != 0 {
			t.Fatalf("path=%s apiKey=%s recorded %#v", tc.path, tc.apiKeyID, recorder.events)
		}
	}
}

func TestMiddlewarePreservesImplicitStatusAndFlush(t *testing.T) {
	recorder := &recordingEnqueuer{}
	router := chi.NewRouter()
	router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
	router.Use(Middleware(recorder))
	router.Get("/v1/stream", func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("middleware removed http.Flusher")
		}
		_, _ = w.Write([]byte("first"))
		flusher.Flush()
		_, _ = w.Write([]byte("second"))
	})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/stream", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "firstsecond" || recorder.events[0].StatusCode != http.StatusOK || recorder.details[0] != nil {
		t.Fatalf("stream response/event changed: status=%d body=%q event=%#v detail=%#v", rr.Code, rr.Body.String(), recorder.events, recorder.details)
	}
}

type informationalResponseWriter struct {
	header   http.Header
	statuses []int
	body     bytes.Buffer
}

type readerFromResponseWriter struct {
	*informationalResponseWriter
	readFromCalls int
}

func (w *readerFromResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	w.readFromCalls++
	return io.Copy(&w.body, reader)
}

func (w *informationalResponseWriter) Header() http.Header { return w.header }

func (w *informationalResponseWriter) WriteHeader(status int) {
	w.statuses = append(w.statuses, status)
}

func (w *informationalResponseWriter) Write(payload []byte) (int, error) {
	return w.body.Write(payload)
}

func TestMiddlewarePreservesInformationalResponseBeforeFinalFailure(t *testing.T) {
	recorder := &recordingEnqueuer{}
	router := chi.NewRouter()
	router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
	router.Use(Middleware(recorder))
	router.Get("/v1/early-hints", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"PROVIDER_UNAVAILABLE"}`))
	})

	underlying := &informationalResponseWriter{header: make(http.Header)}
	router.ServeHTTP(underlying, httptest.NewRequest(http.MethodGet, "/v1/early-hints", nil))

	if got := underlying.statuses; len(got) != 2 || got[0] != http.StatusEarlyHints || got[1] != http.StatusInternalServerError {
		t.Fatalf("forwarded statuses = %#v", got)
	}
	if underlying.body.String() != `{"code":"PROVIDER_UNAVAILABLE"}` {
		t.Fatalf("response body = %q", underlying.body.String())
	}
	if len(recorder.events) != 1 || recorder.events[0].StatusCode != http.StatusInternalServerError || recorder.details[0] == nil {
		t.Fatalf("recorded event/detail = %#v %#v", recorder.events, recorder.details)
	}
}

func TestMiddlewarePreservesInformationalResponseBeforeFinalSuccess(t *testing.T) {
	recorder := &recordingEnqueuer{}
	router := chi.NewRouter()
	router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
	router.Use(Middleware(recorder))
	router.Get("/v1/early-hints", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	underlying := &informationalResponseWriter{header: make(http.Header)}
	router.ServeHTTP(underlying, httptest.NewRequest(http.MethodGet, "/v1/early-hints", nil))
	if got := underlying.statuses; len(got) != 2 || got[0] != http.StatusEarlyHints || got[1] != http.StatusOK {
		t.Fatalf("forwarded statuses = %#v", got)
	}
	if underlying.body.String() != "ok" || recorder.events[0].StatusCode != http.StatusOK || recorder.details[0] != nil {
		t.Fatalf("success response/event changed: body=%q event=%#v detail=%#v", underlying.body.String(), recorder.events, recorder.details)
	}
}

func TestMiddlewarePreservesOptimizedReadFromForSuccessfulResponses(t *testing.T) {
	underlying := &readerFromResponseWriter{informationalResponseWriter: &informationalResponseWriter{header: make(http.Header)}}
	wrapper := &observedResponseWriter{ResponseWriter: underlying, status: http.StatusOK}
	written, err := wrapper.ReadFrom(strings.NewReader("successful-stream"))
	if err != nil {
		t.Fatal(err)
	}
	if written != int64(len("successful-stream")) || underlying.readFromCalls != 1 || underlying.body.String() != "successful-stream" {
		t.Fatalf("optimized read = bytes=%d calls=%d body=%q", written, underlying.readFromCalls, underlying.body.String())
	}
	if wrapper.total != 0 || wrapper.body.Len() != 0 {
		t.Fatalf("successful response was captured: total=%d body=%q", wrapper.total, wrapper.body.String())
	}
}

func TestMiddlewareCapsHashingForLargeRequestAndFailureResponse(t *testing.T) {
	recorder := &recordingEnqueuer{}
	router := chi.NewRouter()
	router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
	router.Use(Middleware(recorder))
	router.Post("/v1/large-failure", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("response-safe ", maxPayloadBytes)))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/large-failure", bytes.NewReader(bytes.Repeat([]byte{0x01}, maxPayloadBytes+4096)))
	req.Header.Set("Content-Type", "application/octet-stream")
	router.ServeHTTP(httptest.NewRecorder(), req)

	detail := recorder.details[0]
	if detail == nil {
		t.Fatal("failure detail missing")
	}
	if detail.RequestSHA256 != "" || detail.ResponseSHA256 != "" {
		t.Fatalf("oversized payload hashes must be omitted: request=%q response=%q", detail.RequestSHA256, detail.ResponseSHA256)
	}
	if !strings.Contains(detail.RequestOmissionReason, "hash_omitted_size") || !strings.Contains(detail.ResponseOmissionReason, "hash_omitted_size") {
		t.Fatalf("oversized omission reasons = request %q response %q", detail.RequestOmissionReason, detail.ResponseOmissionReason)
	}
	if detail.ResponseStoredBytes > maxPayloadBytes {
		t.Fatalf("response stored bytes = %d", detail.ResponseStoredBytes)
	}
}

func TestExtractErrorCodeAcceptsOnlyStructuredApplicationCodes(t *testing.T) {
	if got := extractErrorCode([]byte(`{"code":"opaque-secret"}`)); got != "" {
		t.Fatalf("unstructured error code stored: %q", got)
	}
	if got := extractErrorCode([]byte(`{"error":{"code":"PROVIDER_UNAVAILABLE"}}`)); got != "PROVIDER_UNAVAILABLE" {
		t.Fatalf("structured error code = %q", got)
	}
}

func TestTelemetryQueueSaturationDoesNotChangeBusinessOutcome(t *testing.T) {
	recorder := NewRecorder(&recordingStore{}, Config{NormalQueueSize: 1})
	recorder.Enqueue(validEvent("prefill", httpStatusOK), nil)

	type result struct {
		status        int
		header        string
		body          string
		mutations     int
		providerCalls int
	}
	run := func(enqueuer Enqueuer) result {
		mutations := 0
		providerCalls := 0
		router := chi.NewRouter()
		router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
		router.Use(Middleware(enqueuer))
		router.Post("/v1/business", func(w http.ResponseWriter, _ *http.Request) {
			mutations++
			providerCalls++
			w.Header().Set("X-Business-Result", "committed")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"business-row"}`))
		})
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/business", nil))
		return result{rr.Code, rr.Header().Get("X-Business-Result"), rr.Body.String(), mutations, providerCalls}
	}

	baseline := run(nil)
	candidate := run(recorder)
	if candidate != baseline {
		t.Fatalf("saturated telemetry changed business outcome: baseline=%#v candidate=%#v", baseline, candidate)
	}
	if stats := recorder.Stats(); stats.SuccessDropped != 1 {
		t.Fatalf("saturation was not observed: %#v", stats)
	}
}

func TestTelemetryDatabaseFailureDoesNotChangeFailureResponse(t *testing.T) {
	store := &recordingStore{failureErr: errors.New("telemetry database unavailable"), wrote: make(chan struct{}, 1)}
	recorder := NewRecorder(store, Config{FailureQueueSize: 1, WriteTimeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recorder.Start(ctx)
	waitForRecorderStart(t, recorder)

	router := chi.NewRouter()
	router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
	router.Use(Middleware(recorder))
	mutations := 0
	providerCalls := 0
	router.Post("/v1/business", func(w http.ResponseWriter, _ *http.Request) {
		mutations++
		providerCalls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Business-Result", "retryable")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"PROVIDER_UNAVAILABLE"}`))
	})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/business", nil))

	if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("X-Business-Result") != "retryable" ||
		rr.Body.String() != `{"code":"PROVIDER_UNAVAILABLE"}` || mutations != 1 || providerCalls != 1 {
		t.Fatalf("telemetry database failure changed business behavior: status=%d headers=%v body=%q mutations=%d calls=%d",
			rr.Code, rr.Header(), rr.Body.String(), mutations, providerCalls)
	}
	waitForRecorderWrite(t, store.wrote)
	deadline := time.Now().Add(time.Second)
	for recorder.Stats().FailureWriteFailures != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if stats := recorder.Stats(); stats.FailureWriteFailures != 1 {
		t.Fatalf("contained telemetry failure was not observable: %#v", stats)
	}
}

func TestDetailCombinedPayloadIsBoundedBelowDatabaseLimit(t *testing.T) {
	detail := &Detail{
		RequestHeaders:  map[string]string{"User-Agent": strings.Repeat("a", 8*1024)},
		ResponseHeaders: map[string]string{"Content-Type": strings.Repeat("b", 8*1024)},
		QueryKeys:       []string{strings.Repeat("q", 8*1024)},
		RequestPayload:  strings.Repeat("r", maxPayloadBytes),
		ResponsePayload: strings.Repeat("s", maxPayloadBytes),
	}
	boundDetail(detail)
	metadata, err := json.Marshal(struct {
		Request  map[string]string
		Response map[string]string
		Query    []string
	}{detail.RequestHeaders, detail.ResponseHeaders, detail.QueryKeys})
	if err != nil {
		t.Fatal(err)
	}
	combined := len(metadata) + len(detail.RequestPayload) + len(detail.ResponsePayload)
	if combined > detailPayloadBudget || detail.RequestStoredBytes != len(detail.RequestPayload) || detail.ResponseStoredBytes != len(detail.ResponsePayload) {
		t.Fatalf("bounded detail bytes=%d request=%d response=%d", combined, detail.RequestStoredBytes, detail.ResponseStoredBytes)
	}
}

func requestEventTestAuth(workspaceID, apiKeyID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.SetWorkspaceID(r.Context(), workspaceID)
			if apiKeyID != "" {
				ctx = auth.SetAPIKeyID(ctx, apiKeyID)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type discardEnqueuer struct{}

func (discardEnqueuer) Enqueue(Event, *Detail) {}

func BenchmarkRequestEventMiddlewareOverhead(b *testing.B) {
	for _, benchmark := range []struct {
		name       string
		middleware bool
	}{
		{name: "baseline"},
		{name: "canonical_metadata", middleware: true},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			router := chi.NewRouter()
			router.Use(requestEventTestAuth("workspace-1", "api-key-1"))
			if benchmark.middleware {
				router.Use(Middleware(discardEnqueuer{}))
			}
			router.Get("/v1/posts/{id}", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodGet, "/v1/posts/post-1", nil)
				router.ServeHTTP(recorder, request)
			}
		})
	}
}
