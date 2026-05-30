package reviewai

import (
	"fmt"
	"strings"
)

type Action struct {
	Action            string                 `json:"action"`
	Target            ActionTarget           `json:"target,omitempty"`
	Value             string                 `json:"value,omitempty"`
	Reason            string                 `json:"reason,omitempty"`
	ExpectedEvidence  map[string]any         `json:"expected_evidence,omitempty"`
	HoldMSAfterAction int                    `json:"hold_ms_after_action,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type ActionTarget struct {
	Selector    string `json:"selector,omitempty"`
	Description string `json:"description,omitempty"`
}

var allowedActions = map[string]bool{
	"navigate":              true,
	"click":                 true,
	"type":                  true,
	"upload_file":           true,
	"scroll":                true,
	"wait":                  true,
	"assert":                true,
	"pause_for_user":        true,
	"open_link":             true,
	"return_to_review_page": true,
}

func ValidateAction(action Action) error {
	name := strings.TrimSpace(action.Action)
	if !allowedActions[name] {
		return fmt.Errorf("review ai action %q is not allowed", action.Action)
	}
	switch name {
	case "click", "type", "upload_file", "assert", "open_link":
		if strings.TrimSpace(action.Target.Selector) == "" {
			return fmt.Errorf("review ai action %q requires target.selector", name)
		}
	}
	if name == "upload_file" && !isApprovedUploadPath(action.Value) {
		return fmt.Errorf("review ai upload path is not approved")
	}
	if action.HoldMSAfterAction < 0 || action.HoldMSAfterAction > 30000 {
		return fmt.Errorf("review ai hold_ms_after_action is out of range")
	}
	return nil
}

func isApprovedUploadPath(value string) bool {
	path := strings.TrimSpace(value)
	if path == "" {
		return false
	}
	return strings.HasPrefix(path, "/private/tmp/unipost-review-") ||
		strings.HasPrefix(path, "/tmp/unipost-review-") ||
		(strings.HasPrefix(path, "/Users/") && strings.Contains(path, "/Movies/UniPost/"))
}
