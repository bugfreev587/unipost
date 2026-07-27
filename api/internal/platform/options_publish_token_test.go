package platform

import (
	"errors"
	"testing"
)

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
	opts := map[string]any{OptOnPublishToken: func(s string) error { got = s; return nil }}

	if err := persistPublishToken(opts, "tok_new"); err != nil {
		t.Fatal(err)
	}
	if got != "tok_new" {
		t.Fatalf("persist hook received %q, want tok_new", got)
	}

	// Empty token, nil opts, and missing hook must all be safe no-ops.
	got = ""
	if err := persistPublishToken(opts, ""); err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatal("empty token must not invoke the hook")
	}
	if err := persistPublishToken(nil, "x"); err != nil {
		t.Fatal(err)
	}
	if err := persistPublishToken(map[string]any{}, "x"); err != nil {
		t.Fatal(err)
	}
}

func TestPersistPublishTokenReturnsHookError(t *testing.T) {
	want := errors.New("publish token was not durably persisted")
	opts := map[string]any{OptOnPublishToken: func(string) error { return want }}

	if err := persistPublishToken(opts, "tok_new"); !errors.Is(err, want) {
		t.Fatalf("persistPublishToken error = %v, want %v", err, want)
	}
}
