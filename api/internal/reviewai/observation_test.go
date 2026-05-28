package reviewai

import (
	"strings"
	"testing"
)

func TestRedactVisibleTextRemovesSecrets(t *testing.T) {
	input := "email y@example.com password hunter2 token abc123 sk-ant-secret"
	got := RedactVisibleText(input)
	for _, forbidden := range []string{"hunter2", "abc123", "sk-ant-secret"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("redacted text leaked %q: %s", forbidden, got)
		}
	}
}

func TestRedactDOMHintsDropsPasswordControls(t *testing.T) {
	hints := []DOMHint{
		{Role: "textbox", Text: "Password", SelectorHint: "input[type=password]"},
		{Role: "button", Text: "Connect TikTok", SelectorHint: "[data-review-step='connect-tiktok']"},
	}
	got := RedactDOMHints(hints)
	if len(got) != 1 || got[0].Text != "Connect TikTok" {
		t.Fatalf("unexpected hints after redaction: %+v", got)
	}
}
