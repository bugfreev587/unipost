package reviewai

import (
	"regexp"
	"strings"
)

type Observation struct {
	JobID         string    `json:"job_id"`
	StepKey       string    `json:"step_key"`
	CurrentURL    string    `json:"current_url"`
	PageTitle     string    `json:"page_title"`
	VisibleText   string    `json:"visible_text"`
	DOMHints      []DOMHint `json:"dom_hints"`
	ScreenshotRef string    `json:"screenshot_ref,omitempty"`
}

type DOMHint struct {
	Role         string `json:"role"`
	Text         string `json:"text"`
	SelectorHint string `json:"selector_hint"`
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passcode|verification code|2fa|token|secret|api key)\s*[:=]?\s*\S+`),
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]+`),
	regexp.MustCompile(`Bearer\s+[A-Za-z0-9._-]+`),
}

func RedactObservation(obs Observation) Observation {
	obs.VisibleText = RedactVisibleText(obs.VisibleText)
	obs.DOMHints = RedactDOMHints(obs.DOMHints)
	return obs
}

func RedactVisibleText(value string) string {
	out := value
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllString(out, "[redacted]")
	}
	return out
}

func RedactDOMHints(hints []DOMHint) []DOMHint {
	out := make([]DOMHint, 0, len(hints))
	for _, hint := range hints {
		joined := strings.ToLower(hint.Role + " " + hint.Text + " " + hint.SelectorHint)
		if strings.Contains(joined, "password") ||
			strings.Contains(joined, "verification code") ||
			strings.Contains(joined, "2fa") {
			continue
		}
		hint.Text = RedactVisibleText(hint.Text)
		out = append(out, hint)
	}
	return out
}
