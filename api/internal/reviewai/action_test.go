package reviewai

import "testing"

func TestValidateActionAllowsSafeClick(t *testing.T) {
	action := Action{
		Action:            "click",
		Target:            ActionTarget{Selector: "[data-review-step='connect-tiktok']", Description: "Connect TikTok"},
		HoldMSAfterAction: 2000,
	}
	if err := ValidateAction(action); err != nil {
		t.Fatalf("ValidateAction returned error: %v", err)
	}
}

func TestValidateActionRejectsUnsupportedAction(t *testing.T) {
	action := Action{Action: "eval", Target: ActionTarget{Selector: "body"}}
	if err := ValidateAction(action); err == nil {
		t.Fatal("expected unsupported action to be rejected")
	}
}

func TestValidateActionRejectsClickWithoutSelector(t *testing.T) {
	action := Action{Action: "click", Target: ActionTarget{Description: "missing selector"}}
	if err := ValidateAction(action); err == nil {
		t.Fatal("expected click without selector to be rejected")
	}
}

func TestValidateActionRejectsUnsafeFileUploadPath(t *testing.T) {
	action := Action{
		Action: "upload_file",
		Target: ActionTarget{Selector: "input[type=file]"},
		Value:  "/etc/passwd",
	}
	if err := ValidateAction(action); err == nil {
		t.Fatal("expected unsafe upload path to be rejected")
	}
}
