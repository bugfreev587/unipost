package platform

import "testing"

func TestResumePublishToken(t *testing.T) {
	if got := resumePublishToken(map[string]any{OptResumePublishToken: "tok_prior"}); got != "tok_prior" {
		t.Fatalf("resumePublishToken = %q, want tok_prior", got)
	}
	if got := resumePublishToken(nil); got != "" {
		t.Fatalf("resumePublishToken(nil) = %q, want empty", got)
	}
	if got := resumePublishToken(map[string]any{}); got != "" {
		t.Fatalf("resumePublishToken(no key) = %q, want empty", got)
	}
}

func TestPersistPublishToken(t *testing.T) {
	var got string
	opts := map[string]any{OptOnPublishToken: func(s string) { got = s }}

	persistPublishToken(opts, "tok_new")
	if got != "tok_new" {
		t.Fatalf("persist hook received %q, want tok_new", got)
	}

	// Empty token, nil opts, and missing hook must all be safe no-ops.
	got = ""
	persistPublishToken(opts, "")
	if got != "" {
		t.Fatal("empty token must not invoke the hook")
	}
	persistPublishToken(nil, "x")
	persistPublishToken(map[string]any{}, "x")
}
