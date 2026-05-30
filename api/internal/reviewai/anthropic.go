package reviewai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAnthropicURL   = "https://api.anthropic.com/v1/messages"
	defaultAnthropicModel = "claude-sonnet-4-20250514"
)

type AnthropicClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Error   *anthropicError    `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Message string `json:"message"`
}

func NewAnthropicClient(apiKey, model, baseURL string, client *http.Client) *AnthropicClient {
	if strings.TrimSpace(model) == "" {
		model = defaultAnthropicModel
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultAnthropicURL
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &AnthropicClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  client,
	}
}

func (c *AnthropicClient) NextAction(ctx context.Context, obs Observation, goal string) (Action, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return Action{}, fmt.Errorf("ANTHROPIC_API_KEY not configured")
	}

	redacted := RedactObservation(obs)
	payload := anthropicRequest{
		Model:     c.model,
		MaxTokens: 700,
		System:    reviewPlannerSystemPrompt(),
		Messages: []anthropicMessage{{
			Role:    "user",
			Content: buildReviewPlannerPrompt(redacted, goal),
		}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Action{}, fmt.Errorf("marshal anthropic request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return Action{}, fmt.Errorf("create anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return Action{}, fmt.Errorf("call anthropic messages api: %w", err)
	}
	defer resp.Body.Close()

	limitedBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Action{}, fmt.Errorf("read anthropic response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Action{}, fmt.Errorf("anthropic messages api returned status %d", resp.StatusCode)
	}

	var decoded anthropicResponse
	if err := json.Unmarshal(limitedBody, &decoded); err != nil {
		return Action{}, fmt.Errorf("decode anthropic response: %w", err)
	}
	if decoded.Error != nil {
		return Action{}, fmt.Errorf("anthropic messages api error: %s", decoded.Error.Message)
	}

	text := firstAnthropicText(decoded.Content)
	if text == "" {
		return Action{}, fmt.Errorf("anthropic response did not include text content")
	}
	actionJSON, err := extractJSONObject(text)
	if err != nil {
		return Action{}, err
	}

	var action Action
	if err := json.Unmarshal([]byte(actionJSON), &action); err != nil {
		return Action{}, fmt.Errorf("decode review ai action: %w", err)
	}
	if err := ValidateAction(action); err != nil {
		return Action{}, err
	}
	return action, nil
}

func reviewPlannerSystemPrompt() string {
	return strings.Join([]string{
		"You are UniPost's App Review browser planner.",
		"Return strict JSON for one allowed action only.",
		"Never request secrets, arbitrary JavaScript, shell commands, cookies, tokens, passwords, or verification codes.",
	}, " ")
}

func buildReviewPlannerPrompt(obs Observation, goal string) string {
	payload := map[string]any{
		"goal":            goal,
		"observation":     obs,
		"allowed_actions": allowedActionNames(),
		"response_schema": map[string]any{
			"action":               "navigate | click | type | upload_file | scroll | wait | assert | pause_for_user | open_link | return_to_review_page",
			"target.selector":      "required for click, type, upload_file, assert, and open_link",
			"value":                "text, URL, or approved upload file path when required",
			"reason":               "short explanation for the human event log",
			"hold_ms_after_action": "0-30000; prefer 1500-3000 so reviewers can follow the recording",
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"goal":%q}`, goal)
	}
	return string(encoded)
}

func allowedActionNames() []string {
	names := make([]string, 0, len(allowedActions))
	for name := range allowedActions {
		names = append(names, name)
	}
	return names
}

func firstAnthropicText(content []anthropicContent) string {
	for _, item := range content {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			return item.Text
		}
	}
	return ""
}

func extractJSONObject(value string) (string, error) {
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("anthropic response did not include a JSON object")
	}
	return value[start : end+1], nil
}
