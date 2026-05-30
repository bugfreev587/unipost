# AI-guided App Review Autopilot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first AI-guided TikTok App Review Autopilot path so UniPost can produce reviewer-readable demo videos with server-side Anthropic orchestration, constrained browser actions, and evidence gates.

**Architecture:** The UniPost API owns the review plan, Anthropic key, action schema, evidence gates, and job event log. The local review agent observes the controlled browser window, asks the API for the next validated action, executes only allowlisted actions, records video with stable pacing, and uploads artifacts. The dashboard remains the setup and live-status surface.

**Tech Stack:** Go API (`api/internal/handler`, `api/internal/featureflags`, new `api/internal/reviewai`), PostgreSQL/sqlc migrations if needed, Node review agent (`review-agent/src`), Playwright, macOS `screencapture`, Anthropic Messages API, Next.js dashboard.

---

## File Structure

- Modify `api/.env.example`: document `ANTHROPIC_API_KEY`, `ANTHROPIC_MODEL`, and `FEATURE_APP_REVIEW_AI_AGENT_V1`.
- Modify `docs/feature-flags-unleash.md`: document `app_review.ai_agent_v1`.
- Modify `api/internal/featureflags/flags.go`: add `AppReviewAIAgentV1`.
- Modify `api/internal/featureflags/flags_test.go`: verify env var and development default.
- Create `api/internal/reviewai/action.go`: action schema, validation, allowlist.
- Create `api/internal/reviewai/anthropic.go`: backend-only Anthropic client.
- Create `api/internal/reviewai/observation.go`: observation request shape and redaction helpers.
- Create `api/internal/reviewai/evidence.go`: TikTok evidence gate definitions and checks.
- Add tests under `api/internal/reviewai/*_test.go`.
- Modify `api/internal/handler/review.go`: inject AI service, add agent endpoints, and gate them behind `app_review.ai_agent_v1`.
- Modify `api/cmd/api/main.go`: wire Anthropic client into `ReviewHandler`.
- Modify `api/internal/handler/review_test.go`: endpoint and failure behavior tests.
- Modify `review-agent/src/client.js`: add `nextAction`, `submitObservation`, and `submitEvidence` API calls.
- Create `review-agent/src/ai-runner.js`: AI-guided state machine runner.
- Create `review-agent/src/observation.js`: visible text, DOM hints, screenshot capture, redaction boundary.
- Modify `review-agent/src/index.js`: add `--ai-guided` and pass through to the AI runner.
- Modify `review-agent/src/native-capture.js`: verify actual browser bounds after setting window size.
- Modify `review-agent/src/runner.js`: expose reusable action helpers and configurable step holds.
- Add tests in `review-agent/tests/ai-runner.test.js`, `review-agent/tests/observation.test.js`, and extend existing runner/native capture tests.
- Modify `dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx`: show AI-guided mode, live events, and manual action states after backend supports them.

## Task 1: Configuration, Feature Flag, and Documentation

**Files:**
- Modify: `api/.env.example`
- Modify: `docs/feature-flags-unleash.md`
- Modify: `api/internal/featureflags/flags.go`
- Modify: `api/internal/featureflags/flags_test.go`

- [ ] **Step 1: Add a failing feature flag test**

Add this test to `api/internal/featureflags/flags_test.go`:

```go
func TestAppReviewAIAgentFlagDefinition(t *testing.T) {
	unsetenv(t, "FEATURE_APP_REVIEW_AI_AGENT_V1")

	def, ok := definitions[AppReviewAIAgentV1]
	if !ok {
		t.Fatal("AppReviewAIAgentV1 definition missing")
	}
	if def.EnvVar != "FEATURE_APP_REVIEW_AI_AGENT_V1" {
		t.Fatalf("unexpected env var: %s", def.EnvVar)
	}
	if def.DefaultEnabled(Target{Env: "production"}) {
		t.Fatal("AI review agent must default off in production")
	}
	if !def.DefaultEnabled(Target{Env: "development"}) {
		t.Fatal("AI review agent may default on outside production")
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/featureflags -run TestAppReviewAIAgentFlagDefinition
```

Expected: fail because `AppReviewAIAgentV1` is undefined.

- [ ] **Step 3: Add the feature flag**

In `api/internal/featureflags/flags.go`, add the constant:

```go
AppReviewAIAgentV1 Flag = "app_review.ai_agent_v1"
```

Add the definition:

```go
AppReviewAIAgentV1: {
	Flag:        AppReviewAIAgentV1,
	EnvVar:      "FEATURE_APP_REVIEW_AI_AGENT_V1",
	Description: "Controls the AI-guided App Review Autopilot executor, server-side Anthropic orchestration, and evidence-gated browser actions.",
	DefaultEnabled: func(target Target) bool {
		return !isProduction(target.Env)
	},
},
```

- [ ] **Step 4: Document backend-only Anthropic env vars**

In `api/.env.example`, add:

```text
# AI-guided App Review Autopilot. Server-side only; never expose to the dashboard.
ANTHROPIC_API_KEY=
ANTHROPIC_MODEL=claude-sonnet-4-20250514
FEATURE_APP_REVIEW_AI_AGENT_V1=false
```

If the model name changes during implementation after checking current Anthropic docs, update this line and the PRD together.

- [ ] **Step 5: Document the feature flag**

In `docs/feature-flags-unleash.md`, add an entry:

```markdown
### `app_review.ai_agent_v1`

- **Owner area:** White-label / App Review / Review Agent
- **Env fallback:** `FEATURE_APP_REVIEW_AI_AGENT_V1`
- **Production default:** off
- **Development default:** on only after `ANTHROPIC_API_KEY` is configured and backend fallback is safe
- **Rollback:** turn the flag off; existing scripted review kit/job generation remains available
- **Third-party dependency:** Anthropic Messages API and TikTok OAuth/review portal availability
```

- [ ] **Step 6: Run feature flag tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/featureflags
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add api/.env.example docs/feature-flags-unleash.md api/internal/featureflags/flags.go api/internal/featureflags/flags_test.go
git commit -m "feat: add app review ai agent flag"
```

## Task 2: Review AI Action Schema

**Files:**
- Create: `api/internal/reviewai/action.go`
- Create: `api/internal/reviewai/action_test.go`

- [ ] **Step 1: Write failing action schema tests**

Create `api/internal/reviewai/action_test.go`:

```go
package reviewai

import "testing"

func TestValidateActionAllowsSafeClick(t *testing.T) {
	action := Action{
		Action: "click",
		Target: ActionTarget{Selector: "[data-review-step='connect-tiktok']", Description: "Connect TikTok"},
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
		Value: "/etc/passwd",
	}
	if err := ValidateAction(action); err == nil {
		t.Fatal("expected unsafe upload path to be rejected")
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewai
```

Expected: fail because package/files do not exist.

- [ ] **Step 3: Implement action schema**

Create `api/internal/reviewai/action.go`:

```go
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
	return strings.HasPrefix(path, "/Users/xiaoboyu/Movies/UniPost/") ||
		strings.HasPrefix(path, "/private/tmp/unipost-review-") ||
		strings.HasPrefix(path, "/tmp/unipost-review-")
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewai
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add api/internal/reviewai/action.go api/internal/reviewai/action_test.go
git commit -m "feat: add review ai action schema"
```

## Task 3: Observation Redaction and Evidence Gate

**Files:**
- Create: `api/internal/reviewai/observation.go`
- Create: `api/internal/reviewai/observation_test.go`
- Create: `api/internal/reviewai/evidence.go`
- Create: `api/internal/reviewai/evidence_test.go`

- [ ] **Step 1: Write failing redaction tests**

Create `api/internal/reviewai/observation_test.go`:

```go
package reviewai

import "testing"

func TestRedactVisibleTextRemovesSecrets(t *testing.T) {
	input := "email y@example.com password hunter2 token abc123 sk-ant-secret"
	got := RedactVisibleText(input)
	for _, forbidden := range []string{"hunter2", "abc123", "sk-ant-secret"} {
		if contains(got, forbidden) {
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
```

- [ ] **Step 2: Write failing evidence gate tests**

Create `api/internal/reviewai/evidence_test.go`:

```go
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
```

- [ ] **Step 3: Run failing tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewai
```

Expected: fail because observation/evidence types do not exist.

- [ ] **Step 4: Implement observation redaction**

Create `api/internal/reviewai/observation.go`:

```go
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
		if strings.Contains(joined, "password") || strings.Contains(joined, "verification code") || strings.Contains(joined, "2fa") {
			continue
		}
		hint.Text = RedactVisibleText(hint.Text)
		out = append(out, hint)
	}
	return out
}

func contains(value, needle string) bool {
	return strings.Contains(value, needle)
}
```

- [ ] **Step 5: Implement evidence gates**

Create `api/internal/reviewai/evidence.go`:

```go
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
```

- [ ] **Step 6: Run tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewai
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add api/internal/reviewai/observation.go api/internal/reviewai/observation_test.go api/internal/reviewai/evidence.go api/internal/reviewai/evidence_test.go
git commit -m "feat: add review ai evidence gates"
```

## Task 4: Anthropic Client

**Files:**
- Create: `api/internal/reviewai/anthropic.go`
- Create: `api/internal/reviewai/anthropic_test.go`

- [ ] **Step 1: Write failing Anthropic client tests**

Create `api/internal/reviewai/anthropic_test.go`:

```go
package reviewai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicClientReturnsValidatedAction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing anthropic api key")
		}
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			Content: []anthropicContent{{Type: "text", Text: `{"action":"wait","reason":"page is loading","hold_ms_after_action":2000}`}},
		})
	}))
	defer server.Close()

	client := NewAnthropicClient("test-key", "claude-test", server.URL, server.Client())
	action, err := client.NextAction(context.Background(), Observation{JobID: "rvjob_1", StepKey: "loading", VisibleText: "Loading"}, "Wait for page")
	if err != nil {
		t.Fatalf("NextAction error: %v", err)
	}
	if action.Action != "wait" || action.HoldMSAfterAction != 2000 {
		t.Fatalf("unexpected action: %+v", action)
	}
}

func TestAnthropicClientRejectsInvalidAction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			Content: []anthropicContent{{Type: "text", Text: `{"action":"eval","value":"alert(1)"}`}},
		})
	}))
	defer server.Close()

	client := NewAnthropicClient("test-key", "claude-test", server.URL, server.Client())
	if _, err := client.NextAction(context.Background(), Observation{JobID: "rvjob_1"}, "Do not eval"); err == nil {
		t.Fatal("expected invalid action to be rejected")
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewai -run Anthropic
```

Expected: fail because client does not exist.

- [ ] **Step 3: Implement Anthropic client**

Create `api/internal/reviewai/anthropic.go` with:

```go
package reviewai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultAnthropicURL = "https://api.anthropic.com/v1/messages"

type AnthropicClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

type anthropicRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    string              `json:"system"`
	Messages  []anthropicMessage  `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func NewAnthropicClient(apiKey, model, baseURL string, client *http.Client) *AnthropicClient {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultAnthropicURL
	}
	return &AnthropicClient{apiKey: strings.TrimSpace(apiKey), model: strings.TrimSpace(model), baseURL: baseURL, client: client}
}

func (c *AnthropicClient) NextAction(ctx context.Context, obs Observation, goal string) (Action, error) {
	if c == nil || c.apiKey == "" {
		return Action{}, fmt.Errorf("ANTHROPIC_API_KEY not configured")
	}
	model := c.model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	obs = RedactObservation(obs)
	userPrompt, err := json.Marshal(map[string]any{
		"goal": goal,
		"observation": obs,
		"allowed_actions": []string{"navigate", "click", "type", "upload_file", "scroll", "wait", "assert", "pause_for_user", "open_link", "return_to_review_page"},
	})
	if err != nil {
		return Action{}, err
	}
	body, err := json.Marshal(anthropicRequest{
		Model: model,
		MaxTokens: 700,
		System: "You are UniPost's App Review browser planner. Return strict JSON for one allowed action only. Never request secrets, arbitrary JavaScript, shell commands, cookies, tokens, passwords, or verification codes.",
		Messages: []anthropicMessage{{Role: "user", Content: string(userPrompt)}},
	})
	if err != nil {
		return Action{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return Action{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	res, err := c.client.Do(req)
	if err != nil {
		return Action{}, err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Action{}, err
	}
	if res.StatusCode >= 300 {
		var apiErr anthropicResponse
		if json.Unmarshal(payload, &apiErr) == nil && apiErr.Error != nil && apiErr.Error.Message != "" {
			return Action{}, fmt.Errorf("anthropic returned HTTP %d: %s", res.StatusCode, apiErr.Error.Message)
		}
		return Action{}, fmt.Errorf("anthropic returned HTTP %d", res.StatusCode)
	}
	var parsed anthropicResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return Action{}, err
	}
	var text string
	for _, part := range parsed.Content {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			text = strings.TrimSpace(part.Text)
			break
		}
	}
	if text == "" {
		return Action{}, fmt.Errorf("anthropic returned no text action")
	}
	var action Action
	if err := json.Unmarshal([]byte(text), &action); err != nil {
		return Action{}, err
	}
	if err := ValidateAction(action); err != nil {
		return Action{}, err
	}
	return action, nil
}
```

- [ ] **Step 4: Run Anthropic tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewai -run Anthropic
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add api/internal/reviewai/anthropic.go api/internal/reviewai/anthropic_test.go
git commit -m "feat: add anthropic review ai client"
```

## Task 5: Review Agent AI API Endpoints

**Files:**
- Modify: `api/internal/handler/review.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/handler/review_test.go`

- [ ] **Step 1: Add failing handler tests**

In `api/internal/handler/review_test.go`, add tests that:

1. Call `POST /v1/review/agent/next-action` with a valid agent token and a fake AI planner returning `wait`.
2. Assert the response contains `{"action":"wait"}`.
3. Disable `app_review.ai_agent_v1` and assert HTTP 403.
4. Return an invalid planner action and assert HTTP 502 or 422 with no job completion.

Use the existing `reviewStore` fake and token helper patterns already present in this file.

- [ ] **Step 2: Run the failing tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Review.*NextAction|Review.*AIAgent'
```

Expected: fail because endpoints and AI planner are not wired.

- [ ] **Step 3: Add planner interface and handler wiring**

In `api/internal/handler/review.go`, add:

```go
type reviewAIPlanner interface {
	NextAction(context.Context, reviewai.Observation, string) (reviewai.Action, error)
}
```

Add field to `ReviewHandler`:

```go
aiPlanner reviewAIPlanner
```

Add method:

```go
func (h *ReviewHandler) WithAIPlanner(planner reviewAIPlanner) *ReviewHandler {
	h.aiPlanner = planner
	return h
}
```

- [ ] **Step 4: Add endpoint handler**

Add request/response structs:

```go
type reviewAgentNextActionRequest struct {
	StepKey       string                `json:"step_key"`
	Goal          string                `json:"goal"`
	Observation   reviewai.Observation  `json:"observation"`
}
```

Implement:

```go
func (h *ReviewHandler) NextAgentAction(w http.ResponseWriter, r *http.Request) {
	agentToken, ok := h.authenticateReviewAgent(w, r)
	if !ok {
		return
	}
	if !featureflags.Enabled(r.Context(), featureflags.AppReviewAIAgentV1, featureflags.Target{WorkspaceID: agentToken.WorkspaceID, Env: runtimeenv.Current()}) {
		writeError(w, http.StatusForbidden, "FEATURE_DISABLED", "AI-guided review agent is disabled.")
		return
	}
	if h.aiPlanner == nil {
		writeError(w, http.StatusServiceUnavailable, "AI_NOT_CONFIGURED", "AI-guided review agent is not configured.")
		return
	}
	var body reviewAgentNextActionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid next action request")
		return
	}
	obs := reviewai.RedactObservation(body.Observation)
	obs.JobID = agentToken.ReviewJobID
	action, err := h.aiPlanner.NextAction(r.Context(), obs, body.Goal)
	if err != nil {
		_, _ = h.store.CreateReviewJobEvent(r.Context(), db.CreateReviewJobEventParams{
			ReviewJobID: agentToken.ReviewJobID,
			WorkspaceID: agentToken.WorkspaceID,
			EventType: "ai_action_rejected",
			Message: pgtype.Text{String: err.Error(), Valid: true},
		})
		writeError(w, http.StatusBadGateway, "AI_ACTION_REJECTED", err.Error())
		return
	}
	_, _ = h.store.CreateReviewJobEvent(r.Context(), db.CreateReviewJobEventParams{
		ReviewJobID: agentToken.ReviewJobID,
		WorkspaceID: agentToken.WorkspaceID,
		EventType: "ai_action_selected",
		Message: pgtype.Text{String: action.Action, Valid: true},
	})
	writeSuccess(w, action)
}
```

Import `github.com/xiaoboyu/unipost-api/internal/reviewai`.

- [ ] **Step 5: Register route**

In `api/cmd/api/main.go`, add:

```go
r.Post("/v1/review/agent/next-action", reviewHandler.NextAgentAction)
```

Wire planner:

```go
reviewHandler := handler.NewReviewHandler(queries).
	WithAPIBaseURL(apiBaseURL).
	WithReviewCnameTarget(os.Getenv("APP_REVIEW_CNAME_TARGET")).
	WithArtifactStorage(storageClient).
	WithEncryptor(encryptor).
	WithTikTokTestVideoURL(os.Getenv("APP_REVIEW_TIKTOK_TEST_VIDEO_URL")).
	WithAIPlanner(reviewai.NewAnthropicClient(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_MODEL"), "", nil))
```

Import `github.com/xiaoboyu/unipost-api/internal/reviewai`.

- [ ] **Step 6: Run handler tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Review.*NextAction|Review.*AIAgent'
```

Expected: pass.

- [ ] **Step 7: Run API tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: pass.

- [ ] **Step 8: Commit**

```bash
git add api/internal/handler/review.go api/cmd/api/main.go api/internal/handler/review_test.go
git commit -m "feat: add review agent ai action endpoint"
```

## Task 6: Local Agent AI-guided Client and Observation

**Files:**
- Modify: `review-agent/src/client.js`
- Create: `review-agent/src/observation.js`
- Create: `review-agent/tests/observation.test.js`
- Modify: `review-agent/tests/client.test.js` if present, or add coverage in a new test file.

- [ ] **Step 1: Write failing observation tests**

Create `review-agent/tests/observation.test.js`:

```js
import test from "node:test";
import assert from "node:assert/strict";
import { collectPageObservation, redactObservation } from "../src/observation.js";

test("redactObservation removes password-like visible text and DOM hints", () => {
  const obs = redactObservation({
    visible_text: "Password hunter2 token abc123 Connect TikTok",
    dom_hints: [
      { role: "textbox", text: "Password", selector_hint: "input[type=password]" },
      { role: "button", text: "Connect TikTok", selector_hint: "[data-review-step='connect-tiktok']" },
    ],
  });
  assert.equal(obs.visible_text.includes("hunter2"), false);
  assert.equal(obs.visible_text.includes("abc123"), false);
  assert.deepEqual(obs.dom_hints, [{ role: "button", text: "Connect TikTok", selector_hint: "[data-review-step='connect-tiktok']" }]);
});

test("collectPageObservation captures url title text and review-step hints", async () => {
  const page = {
    url: () => "https://review.example.com/tiktok/posting",
    title: async () => "TailTales",
    locator: () => ({ allInnerTexts: async () => ["Connect TikTok", "Upload video"] }),
    evaluate: async () => [
      { role: "button", text: "Connect TikTok", selector_hint: "[data-review-step='connect-tiktok']" },
    ],
  };
  const obs = await collectPageObservation(page, { jobId: "rvjob_1", stepKey: "connect_tiktok" });
  assert.equal(obs.job_id, "rvjob_1");
  assert.equal(obs.current_url, "https://review.example.com/tiktok/posting");
  assert.equal(obs.page_title, "TailTales");
  assert.equal(obs.dom_hints[0].selector_hint, "[data-review-step='connect-tiktok']");
});
```

- [ ] **Step 2: Run failing tests**

Run:

```bash
cd review-agent && npm test -- observation
```

Expected: fail because observation module does not exist.

- [ ] **Step 3: Implement observation module**

Create `review-agent/src/observation.js`:

```js
const SECRET_PATTERNS = [
  /(password|passcode|verification code|2fa|token|secret|api key)\s*[:=]?\s*\S+/gi,
  /sk-ant-[A-Za-z0-9_-]+/g,
  /Bearer\s+[A-Za-z0-9._-]+/g,
];

export async function collectPageObservation(page, { jobId = "", stepKey = "" } = {}) {
  const currentUrl = typeof page.url === "function" ? page.url() : "";
  const title = typeof page.title === "function" ? await page.title().catch(() => "") : "";
  const texts = await page.locator("body").allInnerTexts().catch(() => []);
  const domHints = await page.evaluate(() => {
    return Array.from(document.querySelectorAll("[data-review-step],button,a,input,textarea,select"))
      .filter((el) => {
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      })
      .slice(0, 80)
      .map((el) => ({
        role: el.getAttribute("role") || el.tagName.toLowerCase(),
        text: (el.innerText || el.value || el.getAttribute("aria-label") || el.getAttribute("placeholder") || "").trim().slice(0, 160),
        selector_hint: el.getAttribute("data-review-step")
          ? `[data-review-step='${el.getAttribute("data-review-step")}']`
          : "",
      }));
  }).catch(() => []);
  return redactObservation({
    job_id: jobId,
    step_key: stepKey,
    current_url: currentUrl,
    page_title: title,
    visible_text: texts.join("\n").slice(0, 12000),
    dom_hints: domHints,
  });
}

export function redactObservation(observation = {}) {
  return {
    ...observation,
    visible_text: redactText(observation.visible_text || ""),
    dom_hints: (observation.dom_hints || []).filter((hint) => {
      const joined = `${hint.role || ""} ${hint.text || ""} ${hint.selector_hint || ""}`.toLowerCase();
      return !joined.includes("password") && !joined.includes("verification code") && !joined.includes("2fa");
    }).map((hint) => ({ ...hint, text: redactText(hint.text || "") })),
  };
}

function redactText(value) {
  let out = String(value || "");
  for (const pattern of SECRET_PATTERNS) {
    out = out.replace(pattern, "[redacted]");
  }
  return out;
}
```

- [ ] **Step 4: Add client API call**

In `review-agent/src/client.js`, add:

```js
export async function requestNextReviewAction({ token, apiUrl = DEFAULT_API_URL, fetchImpl = globalThis.fetch, request } = {}) {
  const body = await agentRequest("/v1/review/agent/next-action", { token, apiUrl, fetchImpl, method: "POST", body: request });
  return body.data;
}
```

- [ ] **Step 5: Run review-agent tests**

Run:

```bash
cd review-agent && npm test
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add review-agent/src/client.js review-agent/src/observation.js review-agent/tests/observation.test.js
git commit -m "feat: add review agent page observations"
```

## Task 7: AI-guided Local Runner MVP

**Files:**
- Create: `review-agent/src/ai-runner.js`
- Create: `review-agent/tests/ai-runner.test.js`
- Modify: `review-agent/src/index.js`
- Modify: `review-agent/src/runner.js`

- [ ] **Step 1: Write failing AI runner tests**

Create `review-agent/tests/ai-runner.test.js`:

```js
import test from "node:test";
import assert from "node:assert/strict";
import { runAIGuidedScript } from "../src/ai-runner.js";

test("runAIGuidedScript observes page, requests action, executes allowed click, and reports evidence", async () => {
  const calls = [];
  const page = {
    url: () => "https://review.example.com/tiktok/posting",
    title: async () => "TailTales",
    locator: (selector) => ({
      allInnerTexts: async () => ["Connect TikTok"],
      click: async () => calls.push(`click:${selector}`),
    }),
    evaluate: async () => [{ role: "button", text: "Connect TikTok", selector_hint: "[data-review-step='connect-tiktok']" }],
    waitForTimeout: async (ms) => calls.push(`wait:${ms}`),
  };
  await runAIGuidedScript({
    script: {
      job_id: "rvjob_ai",
      steps: [{ id: "connect_tiktok", marker: "Start TikTok OAuth", goal: "Connect TikTok" }],
    },
    page,
    nextActionImpl: async () => ({ action: "click", target: { selector: "[data-review-step='connect-tiktok']" }, hold_ms_after_action: 2000 }),
    reporter: { event: async (type) => calls.push(`event:${type}`) },
  });
  assert.deepEqual(calls, ["event:ai_observation_captured", "click:[data-review-step='connect-tiktok']", "wait:2000", "event:ai_action_completed"]);
});

test("runAIGuidedScript rejects unsupported local actions", async () => {
  const page = {
    url: () => "https://review.example.com",
    title: async () => "",
    locator: () => ({ allInnerTexts: async () => [] }),
    evaluate: async () => [],
  };
  await assert.rejects(
    () => runAIGuidedScript({
      script: { job_id: "rvjob_ai", steps: [{ id: "bad", goal: "bad" }] },
      page,
      nextActionImpl: async () => ({ action: "eval", value: "alert(1)" }),
      reporter: { event: async () => {} },
    }),
    /not supported/
  );
});
```

- [ ] **Step 2: Run failing tests**

Run:

```bash
cd review-agent && npm test -- ai-runner
```

Expected: fail because AI runner does not exist.

- [ ] **Step 3: Implement AI runner**

Create `review-agent/src/ai-runner.js`:

```js
import { collectPageObservation } from "./observation.js";
import { requestNextReviewAction } from "./client.js";

const LOCAL_ALLOWED_ACTIONS = new Set(["navigate", "click", "type", "upload_file", "scroll", "wait", "assert", "pause_for_user", "open_link", "return_to_review_page"]);

export async function runAIGuidedScript({ script, page, token = "", apiUrl = "", reporter = null, nextActionImpl = null, uploadFilePath = "" } = {}) {
  const nextAction = nextActionImpl || ((request) => requestNextReviewAction({ token, apiUrl, request }));
  for (const step of script.steps || []) {
    const observation = await collectPageObservation(page, { jobId: script.job_id, stepKey: step.id });
    await reporter?.event?.("ai_observation_captured", step.marker || step.id, { step_id: step.id });
    const action = await nextAction({
      step_key: step.id,
      goal: step.goal || step.marker || step.id,
      observation,
    });
    await executeAIAction(page, action, { uploadFilePath });
    const hold = Number(action.hold_ms_after_action || 1800);
    if (typeof page.waitForTimeout === "function" && hold > 0) {
      await page.waitForTimeout(Math.min(30000, hold));
    }
    await reporter?.event?.("ai_action_completed", action.action, { step_id: step.id, action: action.action });
  }
}

export async function executeAIAction(page, action = {}, { uploadFilePath = "" } = {}) {
  if (!LOCAL_ALLOWED_ACTIONS.has(action.action)) {
    throw new Error(`AI action ${action.action || ""} is not supported by the local agent`);
  }
  switch (action.action) {
    case "click":
      await page.locator(action.target?.selector).click();
      return;
    case "type":
      await page.locator(action.target?.selector).fill(action.value || "");
      return;
    case "upload_file":
      await page.locator(action.target?.selector).setInputFiles(action.value || uploadFilePath);
      return;
    case "scroll":
      await page.mouse?.wheel?.(0, Number(action.value || 600));
      return;
    case "wait":
      await page.waitForTimeout?.(Number(action.value || action.hold_ms_after_action || 1500));
      return;
    case "navigate":
      await page.goto(action.value, { waitUntil: "domcontentloaded" });
      return;
    case "assert":
      await page.locator(action.target?.selector).first().waitFor({ state: "visible", timeout: 30000 });
      return;
    case "pause_for_user":
      return;
    case "open_link":
      await page.locator(action.target?.selector).click();
      return;
    case "return_to_review_page":
      await page.bringToFront?.();
      return;
    default:
      throw new Error(`AI action ${action.action || ""} has no executor`);
  }
}
```

- [ ] **Step 4: Add CLI flag**

In `review-agent/src/index.js`:

1. Parse `--ai-guided`.
2. If enabled, launch the same browser setup as the existing runner and call `runAIGuidedScript`.
3. Keep scripted mode as the default until the dashboard command includes `--ai-guided`.

Use a small wrapper rather than duplicating all recording logic. If sharing the current `runScript` browser lifecycle is too invasive, add a temporary `runScript(..., { aiGuided: true })` branch that swaps the step executor while preserving capture/post-processing.

- [ ] **Step 5: Run local agent tests**

Run:

```bash
cd review-agent && npm test
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add review-agent/src/ai-runner.js review-agent/tests/ai-runner.test.js review-agent/src/index.js review-agent/src/runner.js
git commit -m "feat: add ai guided review agent mode"
```

## Task 8: Recording Readability and Capture Safety

**Files:**
- Modify: `review-agent/src/native-capture.js`
- Modify: `review-agent/src/runner.js`
- Modify: `review-agent/tests/native-capture.test.js`
- Modify: `review-agent/tests/runner-video.test.js`

- [ ] **Step 1: Add failing actual-bounds test**

In `review-agent/tests/native-capture.test.js`, add:

```js
test("resolveChromiumWindowBounds returns actual visible bounds after the OS clamps the requested window", async () => {
  const sends = [];
  let getCount = 0;
  const session = {
    send: async (method, payload) => {
      sends.push({ method, payload });
      if (method === "Browser.getWindowForTarget") {
        getCount += 1;
        if (getCount === 1) return { windowId: 7, bounds: { left: 12, top: 24, width: 900, height: 700 } };
        return { windowId: 7, bounds: { left: 0, top: 38, width: 1728, height: 1030 } };
      }
      return {};
    },
  };
  const page = { context: () => ({ newCDPSession: async () => session }) };
  const bounds = await resolveChromiumWindowBounds({ page, recording: { window_left: 0, window_top: 0, window_width: 1920, window_height: 1080 } });
  assert.deepEqual(bounds, { left: 0, top: 38, width: 1728, height: 1030 });
  assert.deepEqual(sends.map((call) => call.method), ["Browser.getWindowForTarget", "Browser.setWindowBounds", "Browser.getWindowForTarget"]);
});
```

- [ ] **Step 2: Add failing step-hold test**

In `review-agent/tests/runner-video.test.js`, add:

```js
test("runScript holds after visible actions so review recordings are readable", async () => {
  const waits = [];
  const script = {
    job_id: "rvjob_holds",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    recording: { step_hold_ms: 1750 },
    steps: [{ id: "select_video", action: "click", selector: "[data-review-step='select-video']" }],
  };
  const page = {
    video: () => ({ path: async () => "/tmp/unipost-review-videos/holds.webm" }),
    locator: () => ({ click: async () => {}, first: () => ({ waitFor: async () => {} }) }),
    waitForTimeout: async (ms) => waits.push(ms),
  };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  await runner.runScript(script, {
    reporter: { event: async () => {}, uploadArtifact: async () => "", complete: async () => {}, fail: async () => assert.fail("should not fail") },
    playwrightImpl,
    nativeCaptureImpl: async () => null,
    out: { write() {} },
  });
  assert.equal(waits.includes(1750), true);
});
```

- [ ] **Step 3: Run failing tests**

Run:

```bash
cd review-agent && npm test -- native-capture
cd review-agent && npm test -- runner-video
```

Expected: fail because actual bounds and step holds are not implemented.

- [ ] **Step 4: Implement actual-bounds capture**

In `review-agent/src/native-capture.js`, after `Browser.setWindowBounds`, call `Browser.getWindowForTarget` again and use the returned actual bounds for the capture rectangle. Keep fallback to requested bounds if CDP does not return dimensions.

- [ ] **Step 5: Implement default holds**

In `review-agent/src/runner.js`, add:

```js
function stepHoldDurationMs(recording = {}) {
  const configured = Number(recording.step_hold_ms || "");
  if (Number.isFinite(configured) && configured >= 0) return configured;
  return 1800;
}
```

After each visible action (`click`, `fill`, `open_link`, `assert_visible`, `wait_for_navigation`) call:

```js
await holdForReview(page, script.recording || {});
```

Implement:

```js
async function holdForReview(page, recording = {}) {
  const ms = stepHoldDurationMs(recording);
  if (ms <= 0) return;
  if (typeof page?.waitForTimeout === "function") {
    await page.waitForTimeout(ms);
    return;
  }
  await delay(ms);
}
```

- [ ] **Step 6: Run local agent tests**

Run:

```bash
cd review-agent && npm test
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add review-agent/src/native-capture.js review-agent/src/runner.js review-agent/tests/native-capture.test.js review-agent/tests/runner-video.test.js
git commit -m "fix: make review recordings readable"
```

## Task 9: Generate AI-guided Commands and Dashboard State

**Files:**
- Modify: `api/internal/handler/review.go`
- Modify: `api/internal/handler/review_test.go`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx`

- [ ] **Step 1: Add failing command-generation test**

In `api/internal/handler/review_test.go`, add a test that enables `app_review.ai_agent_v1`, creates a review job, and asserts:

```go
strings.Contains(env.Data.AgentCommand, "--ai-guided")
```

Also assert that disabling the flag omits `--ai-guided`.

- [ ] **Step 2: Run failing command test**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run Review.*AIGuidedCommand
```

Expected: fail because commands never include `--ai-guided`.

- [ ] **Step 3: Add AI-guided command flag**

Change `reviewAgentCommand` to accept an options struct:

```go
type reviewAgentCommandOptions struct {
	AIGuided bool
}
```

Append ` --ai-guided` only when `AIGuided` is true.

When creating a review job, evaluate:

```go
aiGuided := featureflags.Enabled(r.Context(), featureflags.AppReviewAIAgentV1, featureflags.Target{WorkspaceID: workspaceID, Env: runtimeenv.Current()})
```

- [ ] **Step 4: Update dashboard copy**

In the App Review Autopilot page:

- show `AI-guided recording` badge when the job command includes `--ai-guided`
- show this note:

```text
UniPost AI will guide the local browser through the review plan. Passwords, QR scans, and verification codes remain manual and are not sent to AI.
```

- [ ] **Step 5: Run checks**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run Review
cd dashboard && npm run build
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/review.go api/internal/handler/review_test.go dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx
git commit -m "feat: enable ai guided review recording"
```

## Task 10: End-to-end TailTales Dev Validation

**Files:**
- No source files unless validation reveals defects.

- [ ] **Step 1: Push branch to dev**

Run:

```bash
git push origin HEAD:dev
```

Expected: remote `dev` updates.

- [ ] **Step 2: Confirm dev API has Anthropic env**

Use Railway or API startup logs to confirm the API can boot with `ANTHROPIC_API_KEY` configured. Do not print the key.

- [ ] **Step 3: Create a fresh TailTales review job**

In the dashboard, use:

```text
https://dev.unipost.dev
```

Create a fresh TikTok content posting review job for TailTales.

- [ ] **Step 4: Run the generated command**

Run the command shown by the dashboard. It should include:

```text
--ai-guided
```

- [ ] **Step 5: Verify artifacts**

After recording, run:

```bash
ffprobe -v error -select_streams v:0 -show_entries stream=width,height,r_frame_rate,duration -show_entries format=size,duration -of json /Users/xiaoboyu/unipost/.unipost-review-videos/tiktok-content-posting-part-1.mp4
ffprobe -v error -select_streams v:0 -show_entries stream=width,height,r_frame_rate,duration -show_entries format=size,duration -of json /Users/xiaoboyu/unipost/.unipost-review-videos/tiktok-content-posting-part-2.mp4
ffprobe -v error -select_streams v:0 -show_entries stream=width,height,r_frame_rate,duration -show_entries format=size,duration -of json /Users/xiaoboyu/unipost/.unipost-review-videos/tiktok-content-posting-part-3.mp4
```

Expected:

- `width` is `1920`
- `height` is `1080`
- `r_frame_rate` is `30/1`
- each `size` is less than `50000000`

- [ ] **Step 6: Extract review screenshots**

Run:

```bash
ffmpeg -y -ss 5 -i /Users/xiaoboyu/unipost/.unipost-review-videos/tiktok-content-posting-part-1.mp4 -frames:v 1 /private/tmp/review-ai-part1-005.png
ffmpeg -y -ss 30 -i /Users/xiaoboyu/unipost/.unipost-review-videos/tiktok-content-posting-part-1.mp4 -frames:v 1 /private/tmp/review-ai-part1-030.png
ffmpeg -y -ss 5 -i /Users/xiaoboyu/unipost/.unipost-review-videos/tiktok-content-posting-part-2.mp4 -frames:v 1 /private/tmp/review-ai-part2-005.png
ffmpeg -y -ss 5 -i /Users/xiaoboyu/unipost/.unipost-review-videos/tiktok-content-posting-part-3.mp4 -frames:v 1 /private/tmp/review-ai-part3-005.png
```

Use image viewing to confirm:

- no wrong browser window
- no large black bars
- TailTales app flow starts cleanly
- OAuth consent appears or manual pause overlay clearly explains why user action is needed
- video upload and preview are visible
- policy pages are readable
- publish result is visible

- [ ] **Step 7: Compare against approved videos**

Reference:

```text
/Users/xiaoboyu/Movies/UniPost/TikTok-demo/white-label
```

The TailTales output should match or exceed the approved videos for:

- pacing
- section titles
- scope evidence
- upload preview
- compliance links
- final publish status

- [ ] **Step 8: Final validation**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
cd review-agent && npm test
cd dashboard && npm run build
```

Expected: all pass.

- [ ] **Step 9: Push dev**

Run:

```bash
git push origin HEAD:dev
```

Expected: dev branch contains the implementation and validated artifacts can be regenerated.

## Self-review

- Spec coverage: This plan covers backend-only Anthropic config, feature flagging, action allowlist, observation redaction, evidence gates, AI action endpoint, local AI runner, capture safety, dashboard command visibility, and TailTales validation.
- Placeholder scan: No `TBD` or intentionally vague implementation placeholders remain. The broadest task is dashboard live UX, scoped here to the MVP command/badge/manual-safety state; richer live screenshots can follow after endpoint stability.
- Type consistency: `reviewai.Action`, `reviewai.Observation`, `requestNextReviewAction`, `runAIGuidedScript`, and `app_review.ai_agent_v1` are used consistently across tasks.
