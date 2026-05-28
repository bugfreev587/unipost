package reviewai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAnthropicClientReturnsValidatedAction(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing anthropic api key")
		}
		var body strings.Builder
		if err := json.NewEncoder(&body).Encode(anthropicResponse{
			Content: []anthropicContent{{Type: "text", Text: `{"action":"wait","reason":"page is loading","hold_ms_after_action":2000}`}},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body.String())),
		}, nil
	})}

	client := NewAnthropicClient("test-key", "claude-test", "https://anthropic.test/messages", httpClient)
	action, err := client.NextAction(context.Background(), Observation{JobID: "rvjob_1", StepKey: "loading", VisibleText: "Loading"}, "Wait for page")
	if err != nil {
		t.Fatalf("NextAction error: %v", err)
	}
	if action.Action != "wait" || action.HoldMSAfterAction != 2000 {
		t.Fatalf("unexpected action: %+v", action)
	}
}

func TestAnthropicClientRejectsInvalidAction(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body strings.Builder
		if err := json.NewEncoder(&body).Encode(anthropicResponse{
			Content: []anthropicContent{{Type: "text", Text: `{"action":"eval","value":"alert(1)"}`}},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body.String())),
		}, nil
	})}

	client := NewAnthropicClient("test-key", "claude-test", "https://anthropic.test/messages", httpClient)
	if _, err := client.NextAction(context.Background(), Observation{JobID: "rvjob_1"}, "Do not eval"); err == nil {
		t.Fatal("expected invalid action to be rejected")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
