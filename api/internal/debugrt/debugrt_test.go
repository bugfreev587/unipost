package debugrt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type guardedBody struct {
	reader  io.Reader
	allowed *bool
}

func (b *guardedBody) Read(p []byte) (int, error) {
	if !*b.allowed {
		return 0, fmt.Errorf("request body read before base transport started")
	}
	return b.reader.Read(p)
}

func (b *guardedBody) Close() error { return nil }

func TestNoRecorderNoCapture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := NewClient(5 * time.Second)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	// No recorder on context — roundtripper should be a no-op wrapper.
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestCaptureFailingRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"code":"invalid_params","message":"nope"}}`))
	}))
	defer srv.Close()

	client := NewClient(5 * time.Second)
	rec := NewRecorder()
	ctx := WithRecorder(context.Background(), rec)

	req, _ := http.NewRequestWithContext(ctx, "POST", srv.URL+"/init", strings.NewReader(`{"hello":"world"}`))
	req.Header.Set("Authorization", "Bearer sk-abc123")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	// Verify the caller still sees the full response body.
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "invalid_params") {
		t.Fatalf("caller lost response body, got: %s", body)
	}

	entries := rec.Entries()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Status != 400 {
		t.Errorf("Status = %d, want 400", e.Status)
	}
	if strings.Contains(e.CurlCommand, "Authorization") || strings.Contains(e.CurlCommand, "sk-abc123") {
		t.Errorf("non-allowlisted Authorization header leaked into curl:\n%s", e.CurlCommand)
	}
	if !strings.Contains(e.CurlCommand, `{"hello":"world"}`) {
		t.Errorf("request body missing from curl:\n%s", e.CurlCommand)
	}
	if !strings.Contains(e.ResponseBody, "invalid_params") {
		t.Errorf("response body not recorded: %q", e.ResponseBody)
	}
}

func TestSuccessNotCaptured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewClient(5 * time.Second)
	rec := NewRecorder()
	ctx := WithRecorder(context.Background(), rec)
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := len(rec.Entries()); got != 0 {
		t.Fatalf("successful request should not be recorded, got %d entries", got)
	}
}

func TestOmitAllQueryValuesByDefault(t *testing.T) {
	u := "https://graph.facebook.com/v18.0/me?access_token=secrettoken&fields=name&signed_request=unknown-secret"
	req, _ := http.NewRequest("GET", u, nil)
	got := buildCurl(req, nil)
	for _, value := range []string{"secrettoken", "name", "unknown-secret"} {
		if strings.Contains(got, value) {
			t.Errorf("query value %q leaked:\n%s", value, got)
		}
	}
	for _, key := range []string{"access_token", "fields", "signed_request"} {
		if !strings.Contains(got, key) {
			t.Errorf("query key %q should remain for diagnostic context:\n%s", key, got)
		}
	}
}

func TestRedactURLOmitsUserInfoAndFragment(t *testing.T) {
	req, err := http.NewRequest(
		http.MethodGet,
		"https://provider-user:provider-password@example.test/path?mode=publish#fragment-secret",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := buildCurl(req, nil)
	for _, secret := range []string{"provider-user", "provider-password", "publish", "fragment-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted URL leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "https://example.test/path?mode=") {
		t.Fatalf("redacted URL lost safe route context: %s", got)
	}
}

func TestSerializeMultipleEntries(t *testing.T) {
	rec := NewRecorder()
	rec.append(Entry{CurlCommand: "curl -X POST 'http://a'", Status: 400, ResponseBody: "oops"})
	rec.append(Entry{CurlCommand: "curl -X POST 'http://b'", TransportError: "dial tcp: timeout"})
	out := rec.Serialize()
	if !strings.Contains(out, "# Request 1 — HTTP 400") {
		t.Errorf("missing request 1 header:\n%s", out)
	}
	if !strings.Contains(out, "# Request 2 — transport error") {
		t.Errorf("missing request 2 header:\n%s", out)
	}
	if !strings.Contains(out, "# Response:\n# oops") {
		t.Errorf("response body not formatted as comment:\n%s", out)
	}
}

func TestRecorderCap(t *testing.T) {
	rec := &Recorder{maxEntries: 2}
	for i := 0; i < 5; i++ {
		rec.append(Entry{Status: 400})
	}
	if got := len(rec.Entries()); got != 2 {
		t.Errorf("want 2 kept entries, got %d", got)
	}
	if got := rec.Dropped(); got != 3 {
		t.Errorf("want 3 dropped, got %d", got)
	}
	if !strings.Contains(rec.Serialize(), "3 additional failing request") {
		t.Errorf("dropped count missing from serialize")
	}
}

func TestCaptureDoesNotPreReadRequestBody(t *testing.T) {
	baseStarted := false
	body := &guardedBody{reader: strings.NewReader(`{"hello":"world"}`), allowed: &baseStarted}
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		baseStarted = true
		if _, err := io.ReadAll(req.Body); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("bad request")),
			Request:    req,
		}, nil
	})
	recorder := NewRecorder()
	req, err := http.NewRequestWithContext(WithRecorder(context.Background(), recorder), http.MethodPost, "https://example.test", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Transport: Wrap(base)}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !baseStarted {
		t.Fatal("base transport was never called")
	}
}

func TestBinaryRequestBodyIsForwardedButOmitted(t *testing.T) {
	payload := bytes.Repeat([]byte{0xab}, 2<<20)
	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	recorder := NewRecorder()
	req, err := http.NewRequestWithContext(WithRecorder(context.Background(), recorder), http.MethodPost, srv.URL, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "video/mp4")
	resp, err := NewClient(5 * time.Second).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := <-received; !bytes.Equal(got, payload) {
		t.Fatalf("server received %d bytes, want %d", len(got), len(payload))
	}

	out := recorder.Serialize()
	if strings.Contains(out, "--data '") {
		t.Fatal("binary request body was embedded in curl")
	}
	for _, expected := range []string{
		"body omitted",
		"video/mp4",
		fmt.Sprintf("observed_bytes=%d", len(payload)),
		fmt.Sprintf("sha256=%x", sha256.Sum256(payload)),
	} {
		if !strings.Contains(out, expected) {
			t.Fatalf("binary omission metadata missing %q: %s", expected, out)
		}
	}
}

func TestLargeTextRequestIsForwardedAndSerializedWithinLimit(t *testing.T) {
	prefix := strings.Repeat("a", 96*1024)
	payload := prefix + "tail-marker-that-must-not-be-stored"
	received := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- string(body)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	recorder := NewRecorder()
	req, err := http.NewRequestWithContext(WithRecorder(context.Background(), recorder), http.MethodPost, srv.URL, strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := NewClient(5 * time.Second).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := <-received; got != payload {
		t.Fatalf("server received %d bytes, want %d", len(got), len(payload))
	}

	out := recorder.Serialize()
	if len(out) > 64*1024 {
		t.Fatalf("serialized debug is %d bytes, want <= 65536", len(out))
	}
	if strings.Contains(out, "tail-marker-that-must-not-be-stored") {
		t.Fatal("request tail was stored past the capture limit")
	}
	if !strings.Contains(out, "body omitted") {
		t.Fatalf("unsafe truncated body omission marker missing: %s", out)
	}
}

func TestNewRecorderKeepsAtMostEightEntries(t *testing.T) {
	recorder := NewRecorder()
	for i := 0; i < 9; i++ {
		recorder.append(Entry{Status: http.StatusBadRequest})
	}
	if got := len(recorder.Entries()); got != 8 {
		t.Fatalf("kept entries = %d, want 8", got)
	}
	if got := recorder.Dropped(); got != 1 {
		t.Fatalf("dropped entries = %d, want 1", got)
	}
}

func TestSerializeNeverExceedsHardLimit(t *testing.T) {
	recorder := NewRecorder()
	for i := 0; i < 8; i++ {
		recorder.append(Entry{Status: http.StatusBadRequest, CurlCommand: strings.Repeat("x", 32*1024)})
	}
	out := recorder.Serialize()
	if len(out) > 64*1024 {
		t.Fatalf("serialized debug is %d bytes, want <= 65536", len(out))
	}
	if !strings.Contains(out, "diagnostic truncated") {
		t.Fatal("hard-cap truncation marker missing")
	}
}

func TestCaptureRedactsStructuredRequestAndResponseSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"access_token":"response-secret","nested":{"client_secret":"nested-secret"}},"message":"invalid https://provider.example/error?fields=id&signed_request=response-url-secret"}`))
	}))
	defer server.Close()

	recorder := NewRecorder()
	requestBody := `{"caption":"safe https://callback.example/path?mode=publish&signed_request=request-url-secret","access_token":"request-secret","nested":{"refresh_token":"refresh-secret","password":"password-secret"}}`
	req, err := http.NewRequestWithContext(
		WithRecorder(context.Background(), recorder),
		http.MethodPost,
		server.URL,
		strings.NewReader(requestBody),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer header-secret")
	req.Header.Set("X-Provider-Secret", "unknown-header-secret")
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	resp, err := NewClient(5 * time.Second).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	out := recorder.Serialize()
	for _, secret := range []string{
		"request-secret",
		"refresh-secret",
		"password-secret",
		"response-secret",
		"nested-secret",
		"header-secret",
		"unknown-header-secret",
		"request-url-secret",
		"response-url-secret",
	} {
		if strings.Contains(out, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, out)
		}
	}
	for _, expected := range []string{`"caption":"safe https://callback.example/path?mode=%5BREDACTED%5D`, `"message":"invalid https://provider.example/error?fields=%5BREDACTED%5D`, "X-Restli-Protocol-Version: 2.0.0", "[REDACTED]"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("diagnostic missing %q: %s", expected, out)
		}
	}
	if strings.Contains(out, "X-Provider-Secret") {
		t.Fatalf("non-allowlisted header name was persisted: %s", out)
	}
}

func TestCaptureRedactsFormCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"rejected"}`))
	}))
	defer server.Close()

	recorder := NewRecorder()
	body := "access_token=facebook-secret&message=hello&client_secret=client-secret&api_key=api-secret"
	req, err := http.NewRequestWithContext(
		WithRecorder(context.Background(), recorder),
		http.MethodPost,
		server.URL,
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := NewClient(5 * time.Second).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	out := recorder.Serialize()
	for _, secret := range []string{"facebook-secret", "client-secret", "api-secret"} {
		if strings.Contains(out, secret) {
			t.Fatalf("form diagnostic leaked %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "message=hello") ||
		(!strings.Contains(out, "%5BREDACTED%5D") && !strings.Contains(out, "[REDACTED]")) {
		t.Fatalf("safe form value or redaction marker missing: %s", out)
	}
}

func TestCaptureOmitsUnstructuredAndTruncatedBodies(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "plain text", contentType: "text/plain", body: "token=plain-secret"},
		{name: "truncated json", contentType: "application/json", body: `{"access_token":"` + strings.Repeat("x", maxRequestBodyBytes) + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				_, _ = io.ReadAll(req.Body)
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":"bad"}`)),
					Request:    req,
				}, nil
			})
			recorder := NewRecorder()
			req, err := http.NewRequestWithContext(
				WithRecorder(context.Background(), recorder),
				http.MethodPost,
				"https://example.test",
				strings.NewReader(tc.body),
			)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", tc.contentType)
			resp, err := (&http.Client{Transport: Wrap(base)}).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()

			out := recorder.Serialize()
			if strings.Contains(out, "plain-secret") || strings.Contains(out, strings.Repeat("x", 64)) {
				t.Fatalf("unsafe body content was persisted: %s", out)
			}
			if !strings.Contains(out, "body omitted") {
				t.Fatalf("omission metadata missing: %s", out)
			}
		})
	}
}

func TestSerializeSanitizesTransportErrorsAndInvalidText(t *testing.T) {
	recorder := NewRecorder()
	recorder.append(Entry{
		CurlCommand:    "curl -X POST 'https://example.test'\x00\xff",
		TransportError: `dial failed: Authorization=Bearer transport-secret access_token=query-secret {"access_token":"quoted-secret","client_secret": "quoted-client-secret"}` + "\x00\xff",
		ResponseBody:   "response\x00\xff",
	})
	out := recorder.Serialize()
	if strings.ToValidUTF8(out, "") != out {
		t.Fatal("serialized diagnostic is not valid UTF-8")
	}
	if strings.ContainsRune(out, '\x00') {
		t.Fatal("serialized diagnostic contains a NUL byte")
	}
	for _, secret := range []string{"transport-secret", "query-secret", "quoted-secret", "quoted-client-secret"} {
		if strings.Contains(out, secret) {
			t.Fatalf("transport error leaked %q: %s", secret, out)
		}
	}
}
