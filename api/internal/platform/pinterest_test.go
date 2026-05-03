package platform

import "testing"

func TestPinterestEndpointsDefaultToProduction(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	t.Setenv("PINTEREST_API_BASE_URL", "")
	t.Setenv("PINTEREST_TOKEN_URL", "")
	t.Setenv("PINTEREST_AUTH_URL", "")

	if got := pinterestAPIBaseURL(); got != pinterestAPIBase {
		t.Fatalf("api base = %q, want %q", got, pinterestAPIBase)
	}
	if got := pinterestTokenURL(); got != pinterestTokenEndpoint {
		t.Fatalf("token url = %q, want %q", got, pinterestTokenEndpoint)
	}
	if got := pinterestAuthURL(); got != pinterestOAuthEndpoint {
		t.Fatalf("auth url = %q, want %q", got, pinterestOAuthEndpoint)
	}
}

func TestPinterestEndpointsUseSandboxShortcut(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "true")
	t.Setenv("PINTEREST_API_BASE_URL", "")
	t.Setenv("PINTEREST_TOKEN_URL", "")
	t.Setenv("PINTEREST_AUTH_URL", "")

	if got := pinterestAPIBaseURL(); got != pinterestSandboxAPIBase {
		t.Fatalf("api base = %q, want %q", got, pinterestSandboxAPIBase)
	}
	if got := pinterestTokenURL(); got != pinterestSandboxAPIBase+"/oauth/token" {
		t.Fatalf("token url = %q, want %q", got, pinterestSandboxAPIBase+"/oauth/token")
	}
	if got := pinterestAuthURL(); got != pinterestOAuthEndpoint {
		t.Fatalf("auth url = %q, want %q", got, pinterestOAuthEndpoint)
	}
}

func TestPinterestEndpointsHonorExplicitOverrides(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "true")
	t.Setenv("PINTEREST_API_BASE_URL", "https://example.test/v5/")
	t.Setenv("PINTEREST_TOKEN_URL", "https://example.test/oauth/token")
	t.Setenv("PINTEREST_AUTH_URL", "https://example.test/oauth/")

	if got := pinterestAPIBaseURL(); got != "https://example.test/v5" {
		t.Fatalf("api base = %q, want trimmed override", got)
	}
	if got := pinterestTokenURL(); got != "https://example.test/oauth/token" {
		t.Fatalf("token url = %q, want explicit override", got)
	}
	if got := pinterestAuthURL(); got != "https://example.test/oauth/" {
		t.Fatalf("auth url = %q, want explicit override", got)
	}
}
