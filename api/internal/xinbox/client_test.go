package xinbox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestXClientEnsureFilteredStreamRuleUsesStableContract(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
			t.Fatalf("Authorization = %q", got)
		}
		switch calls {
		case 1:
			if r.Method != http.MethodGet || r.URL.Path != "/2/tweets/search/stream/rules" {
				t.Fatalf("list request = %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":[],"meta":{"result_count":0}}`))
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/2/tweets/search/stream/rules" {
				t.Fatalf("create request = %s %s", r.Method, r.URL.Path)
			}
			var body struct {
				Add []StreamRule `json:"add"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			want := []StreamRule{{
				Value: "(@unipostdev OR to:unipostdev) -is:retweet",
				Tag:   "unipost:x:account:account-123",
			}}
			if !reflect.DeepEqual(body.Add, want) {
				t.Fatalf("add = %#v, want %#v", body.Add, want)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"rule-1","value":"(@unipostdev OR to:unipostdev) -is:retweet","tag":"unipost:x:account:account-123"}]}`))
		default:
			t.Fatalf("unexpected request %d", calls)
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	rule, err := client.EnsureFilteredStreamRule(context.Background(), "app-token", "account-123", "@UniPostDev")
	if err != nil {
		t.Fatal(err)
	}
	if rule.ID != "rule-1" || calls != 2 {
		t.Fatalf("rule=%+v calls=%d", rule, calls)
	}
}

func TestXClientEnsureFilteredStreamRuleReusesExistingStableTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET only", r.Method)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"rule-existing","value":"(@unipostdev OR to:unipostdev) -is:retweet","tag":"unipost:x:account:account-123"}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	rule, err := client.EnsureFilteredStreamRule(context.Background(), "app-token", "account-123", "unipostdev")
	if err != nil {
		t.Fatal(err)
	}
	if rule.ID != "rule-existing" {
		t.Fatalf("rule = %+v", rule)
	}
}

func TestXClientEnsureFilteredStreamRuleReplacesStaleHandleRule(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[{"id":"rule-old","value":"(@oldhandle OR to:oldhandle) -is:retweet","tag":"unipost:x:account:account-123"}]}`))
		case 2:
			var body struct {
				Delete struct {
					IDs []string `json:"ids"`
				} `json:"delete"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if want := []string{"rule-old"}; !reflect.DeepEqual(body.Delete.IDs, want) {
				t.Fatalf("delete ids = %v, want %v", body.Delete.IDs, want)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"rule-old"}],"meta":{"summary":{"deleted":1}}}`))
		case 3:
			_, _ = w.Write([]byte(`{"data":[{"id":"rule-new","value":"(@newhandle OR to:newhandle) -is:retweet","tag":"unipost:x:account:account-123"}]}`))
		default:
			t.Fatalf("unexpected call %d", calls)
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	rule, err := client.EnsureFilteredStreamRule(context.Background(), "app-token", "account-123", "newhandle")
	if err != nil {
		t.Fatal(err)
	}
	if rule.ID != "rule-new" || calls != 3 {
		t.Fatalf("rule=%+v calls=%d", rule, calls)
	}
}

func TestXClientDeleteFilteredStreamRuleIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/2/tweets/search/stream/rules" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"title":"Not Found"}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	if err := client.DeleteFilteredStreamRule(context.Background(), "app-token", "rule-missing"); err != nil {
		t.Fatal(err)
	}
}

func TestXClientDeleteFilteredStreamRuleRequiresConfirmedRuleID(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "confirmed exact id",
			body: `{"data":[{"id":"rule-1"}],"meta":{"summary":{"deleted":1}}}`,
		},
		{
			name:    "partial 200 error",
			body:    `{"data":[],"errors":[{"title":"Invalid Request","type":"https://api.x.com/2/problems/invalid-request","detail":"not deleted","status":400}]}`,
			wantErr: true,
		},
		{
			name:    "unconfirmed empty 200",
			body:    `{"data":[],"meta":{"summary":{"deleted":0}}}`,
			wantErr: true,
		},
		{
			name: "explicit already missing",
			body: `{"errors":[{"resource_id":"rule-1","title":"Not Found Error","type":"https://api.x.com/2/problems/resource-not-found","detail":"rule missing","status":404}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			err := client.DeleteFilteredStreamRule(context.Background(), "app-token", "rule-1")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestXClientOpenFilteredStreamRequestsRequiredFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/2/tweets/search/stream" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
			t.Fatalf("Authorization = %q", got)
		}
		want := url.Values{
			"tweet.fields": {"id,text,author_id,created_at,conversation_id,referenced_tweets"},
			"expansions":   {"author_id,referenced_tweets.id,referenced_tweets.id.author_id"},
			"user.fields":  {"id,name,username,profile_image_url"},
		}
		if !reflect.DeepEqual(r.URL.Query(), want) {
			t.Fatalf("query = %v, want %v", r.URL.Query(), want)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("\r\n"))
		_, _ = w.Write([]byte(`{"data":{"id":"tweet-1","text":"hello"}}` + "\n"))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	var ids []string
	err := client.ConsumeFilteredStream(context.Background(), "app-token", func(event StreamEvent) error {
		ids = append(ids, event.Data.ID)
		return nil
	})
	if !errors.Is(err, ErrStreamDisconnected) {
		t.Fatalf("err = %v, want ErrStreamDisconnected after server closes", err)
	}
	if want := []string{"tweet-1"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestXClientReconnectsWhenFilteredStreamMissesKeepaliveDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response does not support flush")
		}
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		<-time.After(100 * time.Millisecond)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:           server.URL,
		HTTPClient:        server.Client(),
		StreamIdleTimeout: 20 * time.Millisecond,
	})
	started := time.Now()
	err := client.ConsumeFilteredStream(context.Background(), "app-token", func(StreamEvent) error { return nil })
	if !errors.Is(err, ErrStreamDisconnected) {
		t.Fatalf("err = %v, want ErrStreamDisconnected", err)
	}
	if elapsed := time.Since(started); elapsed >= 80*time.Millisecond {
		t.Fatalf("idle reconnect took %s, want before server close", elapsed)
	}
}
