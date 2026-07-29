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
	if !strings.Contains(e.CurlCommand, "Bearer [REDACTED]") {
		t.Errorf("Authorization header not redacted:\n%s", e.CurlCommand)
	}
	if strings.Contains(e.CurlCommand, "sk-abc123") {
		t.Errorf("bearer token leaked into curl:\n%s", e.CurlCommand)
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

func TestRedactQueryParams(t *testing.T) {
	u := "https://graph.facebook.com/v18.0/me?access_token=secrettoken&fields=name"
	req, _ := http.NewRequest("GET", u, nil)
	got := buildCurl(req, nil)
	if strings.Contains(got, "secrettoken") {
		t.Errorf("access_token query param leaked:\n%s", got)
	}
	if !strings.Contains(got, "access_token=%5BREDACTED%5D") && !strings.Contains(got, "access_token=[REDACTED]") {
		t.Errorf("access_token placeholder missing:\n%s", got)
	}
	if !strings.Contains(got, "fields=name") {
		t.Errorf("non-sensitive query params should be preserved:\n%s", got)
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
	if !strings.Contains(out, "body truncated") {
		t.Fatalf("truncation marker missing: %s", out)
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
