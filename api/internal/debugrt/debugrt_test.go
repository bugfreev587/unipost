package debugrt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
