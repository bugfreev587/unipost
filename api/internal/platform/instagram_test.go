package platform

import (
	"net/url"
	"strings"
	"testing"
)

func TestInstagramAuthURLUsesBusinessLoginContract(t *testing.T) {
	adapter := NewInstagramAdapter()
	config := adapter.DefaultOAuthConfig("https://api.unipost.dev")
	config.ClientID = "ig-client"

	got := adapter.GetAuthURL(config, "state-xyz")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}

	if u.Scheme != "https" || u.Host != "www.instagram.com" || u.Path != "/oauth/authorize" {
		t.Fatalf("auth endpoint = %s, want https://www.instagram.com/oauth/authorize", u.String())
	}

	q := u.Query()
	if q.Get("client_id") != "ig-client" || q.Get("response_type") != "code" || q.Get("state") != "state-xyz" {
		t.Fatalf("missing required params: %v", q)
	}
	if q.Get("redirect_uri") != "https://api.unipost.dev/v1/oauth/callback/instagram" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if q.Get("enable_fb_login") != "0" {
		t.Fatalf("enable_fb_login = %q, want 0", q.Get("enable_fb_login"))
	}

	scope := q.Get("scope")
	if strings.Contains(scope, " ") {
		t.Fatalf("scope must be comma-separated for Instagram Business Login, got %q", scope)
	}
	want := strings.Join(config.Scopes, ",")
	if scope != want {
		t.Fatalf("scope = %q, want %q", scope, want)
	}
}
