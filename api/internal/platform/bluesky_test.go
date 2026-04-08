package platform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeJWT builds a minimally-valid unsigned JWT with the given sub +
// exp claims. The Bluesky adapter calls parseJWTSub / parseJWTExp,
// which only base64-decode the middle segment — no signature check.
func fakeJWT(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := map[string]any{
		"sub": sub,
		"exp": time.Now().Add(2 * time.Hour).Unix(),
	}
	pl, _ := json.Marshal(payload)
	return header + "." + base64.RawURLEncoding.EncodeToString(pl) + ".sig"
}

// TestBlueskyPost_ThreadReplyChain verifies that when thread_root_uri /
// thread_root_cid / thread_parent_uri / thread_parent_cid are passed in
// opts, the resulting AT-proto record carries a reply field with both
// root and parent populated. This is the Sprint 3 PR8 contract that
// enables Bluesky threads.
func TestBlueskyPost_ThreadReplyChain(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "createRecord") {
			http.Error(w, "unexpected path", 400)
			return
		}
		body, _ := io.ReadAll(r.Body)
		capturedBody = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uri":"at://did:plc:fake/app.bsky.feed.post/abc","cid":"bafyrei-new"}`))
	}))
	defer srv.Close()

	adapter := &BlueskyAdapter{
		baseURL: srv.URL,
		client:  srv.Client(),
	}

	opts := map[string]any{
		"thread_root_uri":   "at://did:plc:fake/app.bsky.feed.post/root",
		"thread_root_cid":   "bafyrei-root",
		"thread_parent_uri": "at://did:plc:fake/app.bsky.feed.post/parent",
		"thread_parent_cid": "bafyrei-parent",
	}
	result, err := adapter.Post(context.Background(), fakeJWT("did:plc:fake"), "reply text", nil, opts)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	if result.CID != "bafyrei-new" {
		t.Errorf("expected CID bafyrei-new, got %q", result.CID)
	}

	var sent struct {
		Record map[string]any `json:"record"`
	}
	if err := json.Unmarshal(capturedBody, &sent); err != nil {
		t.Fatalf("decode captured body: %v", err)
	}
	reply, ok := sent.Record["reply"].(map[string]any)
	if !ok {
		t.Fatalf("expected reply field on record, got: %s", string(capturedBody))
	}
	root := reply["root"].(map[string]any)
	parent := reply["parent"].(map[string]any)
	if root["uri"] != "at://did:plc:fake/app.bsky.feed.post/root" || root["cid"] != "bafyrei-root" {
		t.Errorf("root mismatch: %#v", root)
	}
	if parent["uri"] != "at://did:plc:fake/app.bsky.feed.post/parent" || parent["cid"] != "bafyrei-parent" {
		t.Errorf("parent mismatch: %#v", parent)
	}
}

// TestBlueskyPost_NoReplyForSinglePost — single posts (no thread opts)
// must NOT carry a reply field. Bluesky rejects records with empty
// reply objects.
func TestBlueskyPost_NoReplyForSinglePost(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = body
		_, _ = w.Write([]byte(`{"uri":"at://x","cid":"y"}`))
	}))
	defer srv.Close()
	adapter := &BlueskyAdapter{baseURL: srv.URL, client: srv.Client()}

	_, err := adapter.Post(context.Background(), fakeJWT("did:plc:fake"), "hi", nil, nil)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	var sent struct {
		Record map[string]any `json:"record"`
	}
	_ = json.Unmarshal(capturedBody, &sent)
	if _, hasReply := sent.Record["reply"]; hasReply {
		t.Errorf("single post should not have reply field; got: %s", string(capturedBody))
	}
}
