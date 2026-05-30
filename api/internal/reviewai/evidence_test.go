package reviewai

import "testing"

func TestTikTokOAuthEvidenceRequiresTikTokHostAndConsentText(t *testing.T) {
	err := CheckEvidence("oauth_consent", Evidence{
		CurrentURL:  "https://www.tiktok.com/v2/auth/authorize?scope=user.info.basic,video.upload,video.publish",
		VisibleText: "Authorize TailTales to access user.info.basic video.upload video.publish",
	})
	if err != nil {
		t.Fatalf("expected oauth evidence to pass: %v", err)
	}
}

func TestTikTokOAuthEvidenceFailsWhenConsentSkipped(t *testing.T) {
	err := CheckEvidence("oauth_consent", Evidence{
		CurrentURL:  "https://tiktok-review.tailtales.ai/tiktok/posting?connect_status=success",
		VisibleText: "TikTok account connected",
	})
	if err == nil {
		t.Fatal("expected skipped oauth consent to fail")
	}
}
