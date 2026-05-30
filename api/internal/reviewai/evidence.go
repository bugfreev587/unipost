package reviewai

import (
	"fmt"
	"net/url"
	"strings"
)

type Evidence struct {
	CurrentURL  string `json:"current_url"`
	VisibleText string `json:"visible_text"`
}

func CheckEvidence(stepKey string, evidence Evidence) error {
	switch stepKey {
	case "oauth_consent":
		return checkOAuthEvidence(evidence)
	case "creator_info":
		return requireText(evidence.VisibleText, "TailTales", "SELF_ONLY")
	case "video_upload":
		return requireText(evidence.VisibleText, "video", "Uploaded")
	case "compliance":
		return requireText(evidence.VisibleText, "Music Usage Confirmation", "Branded Content Policy")
	case "publish_result":
		return requireText(evidence.VisibleText, "published")
	default:
		if strings.TrimSpace(evidence.VisibleText) == "" {
			return fmt.Errorf("evidence for %s has no visible text", stepKey)
		}
		return nil
	}
}

func checkOAuthEvidence(evidence Evidence) error {
	parsed, err := url.Parse(evidence.CurrentURL)
	if err != nil || !strings.HasSuffix(parsed.Hostname(), "tiktok.com") {
		return fmt.Errorf("oauth consent evidence must be on tiktok.com")
	}
	text := strings.ToLower(evidence.VisibleText)
	for _, term := range []string{"authorize", "user.info.basic", "video.upload", "video.publish"} {
		if !strings.Contains(text, term) {
			return fmt.Errorf("oauth consent evidence missing %q", term)
		}
	}
	return nil
}

func requireText(value string, terms ...string) error {
	lower := strings.ToLower(value)
	for _, term := range terms {
		if !strings.Contains(lower, strings.ToLower(term)) {
			return fmt.Errorf("evidence missing %q", term)
		}
	}
	return nil
}
