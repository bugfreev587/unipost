package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/aiproviders"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type AIPostAssistHandler struct {
	queries           *db.Queries
	superAdminChecker *auth.SuperAdminChecker
	client            *http.Client
	aiProviders       *aiproviders.Service
}

func NewAIPostAssistHandler(queries *db.Queries, superAdminChecker *auth.SuperAdminChecker) *AIPostAssistHandler {
	return &AIPostAssistHandler{
		queries:           queries,
		superAdminChecker: superAdminChecker,
		client:            http.DefaultClient,
	}
}

func (h *AIPostAssistHandler) WithAIProviders(service *aiproviders.Service) *AIPostAssistHandler {
	h.aiProviders = service
	return h
}

type aiPostAssistRequest struct {
	Mode               string   `json:"mode"`
	ProfileID          string   `json:"profile_id"`
	MainCaption        string   `json:"main_caption"`
	SelectedAccountIDs []string `json:"selected_account_ids"`
	PlatformPosts      []struct {
		AccountID string `json:"account_id"`
		Caption   string `json:"caption"`
	} `json:"platform_posts"`
	ValidationIssues []struct {
		AccountID string `json:"account_id"`
		Platform  string `json:"platform"`
		Field     string `json:"field"`
		Code      string `json:"code"`
		Message   string `json:"message"`
		Severity  string `json:"severity"`
	} `json:"validation_issues"`
	MediaContext []struct {
		MediaID     string   `json:"media_id"`
		Filename    string   `json:"filename"`
		ContentType string   `json:"content_type"`
		DurationSec *float64 `json:"duration_sec"`
		Width       *int     `json:"width"`
		Height      *int     `json:"height"`
	} `json:"media_context"`
	Objective  string   `json:"objective"`
	Tone       string   `json:"tone"`
	Brief      string   `json:"brief"`
	IncludeCTA bool     `json:"include_cta"`
	MediaIDs   []string `json:"media_ids"`
}

type aiPostAssistResponse struct {
	RequestID           string                     `json:"request_id"`
	Mode                string                     `json:"mode"`
	Summary             string                     `json:"summary,omitempty"`
	MainCaption         string                     `json:"main_caption,omitempty"`
	PlatformCaptions    []aiPlatformCaption        `json:"platform_captions,omitempty"`
	Hashtags            []string                   `json:"hashtags,omitempty"`
	Warnings            []string                   `json:"warnings,omitempty"`
	FirstCommentSuggest []aiFirstCommentSuggestion `json:"first_comment_suggestions,omitempty"`
}

type aiPlatformCaption struct {
	AccountID string `json:"account_id"`
	Platform  string `json:"platform"`
	Caption   string `json:"caption"`
	Reason    string `json:"reason,omitempty"`
}

type aiFirstCommentSuggestion struct {
	AccountID string `json:"account_id"`
	Text      string `json:"text"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	ResponseFormat any             `json:"response_format,omitempty"`
	Temperature    float64         `json:"temperature,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// PostAssist is the first AI-assist stub endpoint for the compose drawer.
// It is intentionally super-admin-only while the feature is in development.
//
// Current behavior:
// - accepts the future-facing request shape
// - supports "improve" end to end
// - returns deterministic suggestion payloads that the dashboard can apply
//
// Once a real model is wired in, the handler contract can stay stable while
// only the suggestion generation implementation changes.
func (h *AIPostAssistHandler) PostAssist(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" || h.superAdminChecker == nil || !h.superAdminChecker.IsSuperAdmin(r.Context(), userID) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "AI assist is not enabled for your account")
		return
	}

	var body aiPostAssistRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	mode := strings.TrimSpace(body.Mode)
	switch mode {
	case "improve":
		if strings.TrimSpace(body.MainCaption) == "" {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "main_caption is required for improve mode")
			return
		}
		resp, err := h.generateImproveSuggestion(r, body)
		if err != nil {
			writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "AI assist failed: "+err.Error())
			return
		}
		writeSuccess(w, resp)
		return
	case "adapt":
		if strings.TrimSpace(body.MainCaption) == "" {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "main_caption is required for adapt mode")
			return
		}
		if len(body.SelectedAccountIDs) == 0 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "selected_account_ids is required for adapt mode")
			return
		}
		resp, err := h.generateAdaptSuggestion(r, body)
		if err != nil {
			writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "AI assist failed: "+err.Error())
			return
		}
		writeSuccess(w, resp)
		return
	case "fix_validation":
		if len(body.ValidationIssues) == 0 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation_issues is required for fix_validation mode")
			return
		}
		resp, err := h.generateFixValidationSuggestion(r, body)
		if err != nil {
			writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "AI assist failed: "+err.Error())
			return
		}
		writeSuccess(w, resp)
		return
	case "media":
		if len(body.MediaContext) == 0 && len(body.MediaIDs) == 0 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "media_context or media_ids is required for media mode")
			return
		}
		resp, err := h.generateMediaSuggestion(r, body)
		if err != nil {
			writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "AI assist failed: "+err.Error())
			return
		}
		writeSuccess(w, resp)
		return
	case "brief":
		if strings.TrimSpace(body.Brief) == "" {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "brief is required for brief mode")
			return
		}
		resp, err := h.generateBriefSuggestion(r, body)
		if err != nil {
			writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "AI assist failed: "+err.Error())
			return
		}
		writeSuccess(w, resp)
		return
	default:
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "mode must be one of: improve, brief, adapt, media, fix_validation")
		return
	}
}

type aiAccountContext struct {
	AccountID   string
	Platform    string
	AccountName string
}

func (h *AIPostAssistHandler) generateImproveSuggestion(r *http.Request, body aiPostAssistRequest) (aiPostAssistResponse, error) {
	if aiResp, err := h.generateImproveSuggestionWithOpenAI(r, body); err == nil && aiResp.MainCaption != "" {
		return aiResp, nil
	}
	return buildImproveStubResponse(body), nil
}

func (h *AIPostAssistHandler) generateAdaptSuggestion(r *http.Request, body aiPostAssistRequest) (aiPostAssistResponse, error) {
	accounts, err := h.loadAccountContexts(r, body.SelectedAccountIDs)
	if err != nil {
		return aiPostAssistResponse{}, err
	}
	if aiResp, err := h.generateAdaptSuggestionWithOpenAI(r, body, accounts); err == nil && len(aiResp.PlatformCaptions) > 0 {
		return aiResp, nil
	}
	return buildAdaptStubResponse(body, accounts), nil
}

func (h *AIPostAssistHandler) generateFixValidationSuggestion(r *http.Request, body aiPostAssistRequest) (aiPostAssistResponse, error) {
	accounts, err := h.loadAccountContexts(r, body.SelectedAccountIDs)
	if err != nil {
		return aiPostAssistResponse{}, err
	}
	if aiResp, err := h.generateFixValidationSuggestionWithOpenAI(r, body, accounts); err == nil && (aiResp.MainCaption != "" || len(aiResp.PlatformCaptions) > 0) {
		return aiResp, nil
	}
	return buildFixValidationStubResponse(body, accounts), nil
}

func (h *AIPostAssistHandler) generateMediaSuggestion(r *http.Request, body aiPostAssistRequest) (aiPostAssistResponse, error) {
	accounts, err := h.loadAccountContexts(r, body.SelectedAccountIDs)
	if err != nil {
		return aiPostAssistResponse{}, err
	}
	if aiResp, err := h.generateMediaSuggestionWithOpenAI(r, body, accounts); err == nil && (aiResp.MainCaption != "" || len(aiResp.PlatformCaptions) > 0) {
		return aiResp, nil
	}
	return buildMediaStubResponse(body, accounts), nil
}

func (h *AIPostAssistHandler) generateBriefSuggestion(r *http.Request, body aiPostAssistRequest) (aiPostAssistResponse, error) {
	accounts, err := h.loadAccountContexts(r, body.SelectedAccountIDs)
	if err != nil {
		return aiPostAssistResponse{}, err
	}
	if aiResp, err := h.generateBriefSuggestionWithOpenAI(r, body, accounts); err == nil && (aiResp.MainCaption != "" || len(aiResp.PlatformCaptions) > 0) {
		return aiResp, nil
	}
	return buildBriefStubResponse(body, accounts), nil
}

func getOpenAIModel() (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not configured")
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return model, nil
}

func (h *AIPostAssistHandler) callOpenAIJSON(r *http.Request, systemPrompt, userPrompt string, temperature float64, out any) error {
	if h.aiProviders != nil {
		_, err := h.aiProviders.ChatCompletionsJSON(
			r.Context(),
			aiproviders.SurfacePostAssist,
			[]aiproviders.ChatMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
			map[string]string{"type": "json_object"},
			temperature,
			out,
		)
		return err
	}

	model, err := getOpenAIModel()
	if err != nil {
		return err
	}

	reqBody := openAIRequest{
		Model: model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
		Temperature:    temperature,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(os.Getenv("OPENAI_API_KEY")))

	httpClient := h.client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var apiErr openAIResponse
		if json.Unmarshal(payload, &apiErr) == nil && apiErr.Error != nil && apiErr.Error.Message != "" {
			return errors.New(apiErr.Error.Message)
		}
		return fmt.Errorf("openai returned HTTP %d", res.StatusCode)
	}

	var parsed openAIResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return err
	}
	if len(parsed.Choices) == 0 {
		return fmt.Errorf("openai returned no choices")
	}
	return json.Unmarshal([]byte(parsed.Choices[0].Message.Content), out)
}

func (h *AIPostAssistHandler) generateImproveSuggestionWithOpenAI(r *http.Request, body aiPostAssistRequest) (aiPostAssistResponse, error) {
	accounts, err := h.loadAccountContexts(r, body.SelectedAccountIDs)
	if err != nil {
		accounts = nil
	}

	systemPrompt := strings.Join([]string{
		"You are helping improve a social media draft inside UniPost.",
		"Return strict JSON only.",
		"Keep the meaning intact, tighten wording, and strengthen the close.",
		"Do not invent unsupported product facts.",
		"Return an object with keys: summary, main_caption, first_comment_suggestions, warnings.",
		"first_comment_suggestions must be an array of objects with account_id and text.",
		"warnings must be an array of strings and may be empty.",
	}, " ")

	var suggestion struct {
		Summary             string                     `json:"summary"`
		MainCaption         string                     `json:"main_caption"`
		FirstCommentSuggest []aiFirstCommentSuggestion `json:"first_comment_suggestions"`
		Warnings            []string                   `json:"warnings"`
	}
	if err := h.callOpenAIJSON(r, systemPrompt, buildOpenAIImprovePrompt(body), 0.6, &suggestion); err != nil {
		return aiPostAssistResponse{}, err
	}
	if strings.TrimSpace(suggestion.MainCaption) == "" {
		return aiPostAssistResponse{}, fmt.Errorf("openai returned empty main_caption")
	}
	suggestion.FirstCommentSuggest = filterFirstCommentSuggestions(suggestion.FirstCommentSuggest, accounts)

	return aiPostAssistResponse{
		RequestID:           "aireq_openai_improve",
		Mode:                "improve",
		Summary:             strings.TrimSpace(suggestion.Summary),
		MainCaption:         strings.TrimSpace(suggestion.MainCaption),
		FirstCommentSuggest: suggestion.FirstCommentSuggest,
		Warnings:            suggestion.Warnings,
	}, nil
}

func buildOpenAIImprovePrompt(body aiPostAssistRequest) string {
	var b strings.Builder
	b.WriteString("Improve this draft for a social media composer.\n")
	b.WriteString("Objective: ")
	if strings.TrimSpace(body.Objective) != "" {
		b.WriteString(body.Objective)
	} else {
		b.WriteString("engagement")
	}
	b.WriteString("\nTone: ")
	if strings.TrimSpace(body.Tone) != "" {
		b.WriteString(body.Tone)
	} else {
		b.WriteString("clear and concise")
	}
	if body.IncludeCTA {
		b.WriteString("\nInclude CTA: yes")
	} else {
		b.WriteString("\nInclude CTA: no")
	}
	if strings.TrimSpace(body.Brief) != "" {
		b.WriteString("\nBrief: ")
		b.WriteString(strings.TrimSpace(body.Brief))
	}
	b.WriteString("\nSelected destinations: ")
	if len(body.SelectedAccountIDs) == 0 {
		b.WriteString("unknown")
	} else {
		b.WriteString(fmt.Sprintf("%d accounts selected", len(body.SelectedAccountIDs)))
	}
	if len(body.MediaIDs) > 0 {
		b.WriteString(fmt.Sprintf("\nMedia attached: %d", len(body.MediaIDs)))
	}
	b.WriteString("\nDraft:\n")
	b.WriteString(strings.TrimSpace(body.MainCaption))
	return b.String()
}

func (h *AIPostAssistHandler) generateAdaptSuggestionWithOpenAI(r *http.Request, body aiPostAssistRequest, accounts []aiAccountContext) (aiPostAssistResponse, error) {
	systemPrompt := strings.Join([]string{
		"You are adapting one social media draft into platform-specific variants inside UniPost.",
		"Return strict JSON only.",
		"Keep the meaning consistent across platforms, but adjust style and density to match each destination.",
		"Do not invent unsupported product facts.",
		"Return an object with keys: summary, platform_captions, first_comment_suggestions, warnings.",
		"platform_captions must be an array of objects with account_id, platform, caption, reason.",
		"first_comment_suggestions must be an array of objects with account_id and text.",
		"warnings must be an array of strings and may be empty.",
	}, " ")

	var suggestion struct {
		Summary             string                     `json:"summary"`
		PlatformCaptions    []aiPlatformCaption        `json:"platform_captions"`
		FirstCommentSuggest []aiFirstCommentSuggestion `json:"first_comment_suggestions"`
		Warnings            []string                   `json:"warnings"`
	}
	if err := h.callOpenAIJSON(r, systemPrompt, buildOpenAIAdaptPrompt(body, accounts), 0.7, &suggestion); err != nil {
		return aiPostAssistResponse{}, err
	}
	if len(suggestion.PlatformCaptions) == 0 {
		return aiPostAssistResponse{}, fmt.Errorf("openai returned empty platform_captions")
	}
	suggestion.FirstCommentSuggest = filterFirstCommentSuggestions(suggestion.FirstCommentSuggest, accounts)

	return aiPostAssistResponse{
		RequestID:           "aireq_openai_adapt",
		Mode:                "adapt",
		Summary:             strings.TrimSpace(suggestion.Summary),
		PlatformCaptions:    suggestion.PlatformCaptions,
		FirstCommentSuggest: suggestion.FirstCommentSuggest,
		Warnings:            suggestion.Warnings,
	}, nil
}

func buildOpenAIAdaptPrompt(body aiPostAssistRequest, accounts []aiAccountContext) string {
	var b strings.Builder
	b.WriteString("Adapt this draft for each destination account.\n")
	b.WriteString("Objective: ")
	if strings.TrimSpace(body.Objective) != "" {
		b.WriteString(body.Objective)
	} else {
		b.WriteString("engagement")
	}
	b.WriteString("\nTone: ")
	if strings.TrimSpace(body.Tone) != "" {
		b.WriteString(body.Tone)
	} else {
		b.WriteString("clear and concise")
	}
	if body.IncludeCTA {
		b.WriteString("\nInclude CTA: yes")
	} else {
		b.WriteString("\nInclude CTA: no")
	}
	if strings.TrimSpace(body.Brief) != "" {
		b.WriteString("\nBrief: ")
		b.WriteString(strings.TrimSpace(body.Brief))
	}
	b.WriteString("\nAccounts:\n")
	for _, account := range accounts {
		b.WriteString("- account_id=")
		b.WriteString(account.AccountID)
		b.WriteString(", platform=")
		b.WriteString(account.Platform)
		if account.AccountName != "" {
			b.WriteString(", name=")
			b.WriteString(account.AccountName)
		}
		b.WriteString("\n")
	}
	b.WriteString("Draft:\n")
	b.WriteString(strings.TrimSpace(body.MainCaption))
	return b.String()
}

func (h *AIPostAssistHandler) generateFixValidationSuggestionWithOpenAI(r *http.Request, body aiPostAssistRequest, accounts []aiAccountContext) (aiPostAssistResponse, error) {
	systemPrompt := strings.Join([]string{
		"You are fixing validation issues inside UniPost's social post composer.",
		"Return strict JSON only.",
		"Only fix text-related issues such as caption length, phrasing, or platform tone.",
		"Do not invent unsupported product facts.",
		"Do not fill compliance-sensitive fields such as privacy settings or made-for-kids decisions.",
		"Return an object with keys: summary, main_caption, platform_captions, warnings.",
		"platform_captions must be an array of objects with account_id, platform, caption, reason.",
	}, " ")

	var suggestion struct {
		Summary          string              `json:"summary"`
		MainCaption      string              `json:"main_caption"`
		PlatformCaptions []aiPlatformCaption `json:"platform_captions"`
		Warnings         []string            `json:"warnings"`
	}
	if err := h.callOpenAIJSON(r, systemPrompt, buildOpenAIFixValidationPrompt(body, accounts), 0.5, &suggestion); err != nil {
		return aiPostAssistResponse{}, err
	}
	if strings.TrimSpace(suggestion.MainCaption) == "" && len(suggestion.PlatformCaptions) == 0 {
		return aiPostAssistResponse{}, fmt.Errorf("openai returned no usable fixes")
	}
	return aiPostAssistResponse{
		RequestID:        "aireq_openai_fix_validation",
		Mode:             "fix_validation",
		Summary:          strings.TrimSpace(suggestion.Summary),
		MainCaption:      strings.TrimSpace(suggestion.MainCaption),
		PlatformCaptions: suggestion.PlatformCaptions,
		Warnings:         suggestion.Warnings,
	}, nil
}

func buildOpenAIFixValidationPrompt(body aiPostAssistRequest, accounts []aiAccountContext) string {
	var b strings.Builder
	b.WriteString("Fix these UniPost validation issues.\n")
	b.WriteString("Main caption:\n")
	b.WriteString(strings.TrimSpace(body.MainCaption))
	b.WriteString("\nAccounts:\n")
	for _, account := range accounts {
		b.WriteString("- account_id=")
		b.WriteString(account.AccountID)
		b.WriteString(", platform=")
		b.WriteString(account.Platform)
		if caption := captionForAccount(body, account.AccountID); caption != "" {
			b.WriteString(", current_caption=")
			b.WriteString(caption)
		}
		b.WriteString("\n")
	}
	b.WriteString("Validation issues:\n")
	for _, issue := range body.ValidationIssues {
		b.WriteString("- ")
		if issue.AccountID != "" {
			b.WriteString("account_id=" + issue.AccountID + ", ")
		}
		if issue.Platform != "" {
			b.WriteString("platform=" + issue.Platform + ", ")
		}
		b.WriteString("field=" + issue.Field + ", code=" + issue.Code + ", message=" + issue.Message + "\n")
	}
	return b.String()
}

func (h *AIPostAssistHandler) generateMediaSuggestionWithOpenAI(r *http.Request, body aiPostAssistRequest, accounts []aiAccountContext) (aiPostAssistResponse, error) {
	systemPrompt := strings.Join([]string{
		"You are generating social media copy from uploaded media context inside UniPost.",
		"Return strict JSON only.",
		"Use the media metadata as context but do not invent visual details that are not implied by the filenames or metadata.",
		"Return an object with keys: summary, main_caption, platform_captions, first_comment_suggestions, warnings.",
		"platform_captions must be an array of objects with account_id, platform, caption, reason.",
		"first_comment_suggestions must be an array of objects with account_id and text.",
	}, " ")

	var suggestion struct {
		Summary             string                     `json:"summary"`
		MainCaption         string                     `json:"main_caption"`
		PlatformCaptions    []aiPlatformCaption        `json:"platform_captions"`
		FirstCommentSuggest []aiFirstCommentSuggestion `json:"first_comment_suggestions"`
		Warnings            []string                   `json:"warnings"`
	}
	if err := h.callOpenAIJSON(r, systemPrompt, buildOpenAIMediaPrompt(body, accounts), 0.7, &suggestion); err != nil {
		return aiPostAssistResponse{}, err
	}
	if strings.TrimSpace(suggestion.MainCaption) == "" && len(suggestion.PlatformCaptions) == 0 {
		return aiPostAssistResponse{}, fmt.Errorf("openai returned no usable media suggestion")
	}
	suggestion.FirstCommentSuggest = filterFirstCommentSuggestions(suggestion.FirstCommentSuggest, accounts)
	return aiPostAssistResponse{
		RequestID:           "aireq_openai_media",
		Mode:                "media",
		Summary:             strings.TrimSpace(suggestion.Summary),
		MainCaption:         strings.TrimSpace(suggestion.MainCaption),
		PlatformCaptions:    suggestion.PlatformCaptions,
		FirstCommentSuggest: suggestion.FirstCommentSuggest,
		Warnings:            suggestion.Warnings,
	}, nil
}

func buildOpenAIMediaPrompt(body aiPostAssistRequest, accounts []aiAccountContext) string {
	var b strings.Builder
	b.WriteString("Write social media copy from uploaded media context.\n")
	b.WriteString("Objective: ")
	if strings.TrimSpace(body.Objective) != "" {
		b.WriteString(body.Objective)
	} else {
		b.WriteString("engagement")
	}
	b.WriteString("\nTone: ")
	if strings.TrimSpace(body.Tone) != "" {
		b.WriteString(body.Tone)
	} else {
		b.WriteString("clear and concise")
	}
	if strings.TrimSpace(body.Brief) != "" {
		b.WriteString("\nBrief: ")
		b.WriteString(strings.TrimSpace(body.Brief))
	}
	if strings.TrimSpace(body.MainCaption) != "" {
		b.WriteString("\nExisting caption:\n")
		b.WriteString(strings.TrimSpace(body.MainCaption))
	}
	b.WriteString("\nMedia context:\n")
	for _, media := range body.MediaContext {
		b.WriteString("- filename=" + media.Filename + ", type=" + media.ContentType)
		if media.DurationSec != nil {
			b.WriteString(fmt.Sprintf(", duration_sec=%.1f", *media.DurationSec))
		}
		if media.Width != nil && media.Height != nil {
			b.WriteString(fmt.Sprintf(", size=%dx%d", *media.Width, *media.Height))
		}
		b.WriteString("\n")
	}
	if len(accounts) > 0 {
		b.WriteString("Accounts:\n")
		for _, account := range accounts {
			b.WriteString("- account_id=" + account.AccountID + ", platform=" + account.Platform + "\n")
		}
	}
	return b.String()
}

func (h *AIPostAssistHandler) generateBriefSuggestionWithOpenAI(r *http.Request, body aiPostAssistRequest, accounts []aiAccountContext) (aiPostAssistResponse, error) {
	systemPrompt := strings.Join([]string{
		"You are generating a first-pass social media draft from a campaign brief inside UniPost.",
		"Return strict JSON only.",
		"Do not invent unsupported product facts.",
		"Return an object with keys: summary, main_caption, platform_captions, first_comment_suggestions, warnings.",
		"platform_captions must be an array of objects with account_id, platform, caption, reason.",
		"first_comment_suggestions must be an array of objects with account_id and text.",
	}, " ")

	var suggestion struct {
		Summary             string                     `json:"summary"`
		MainCaption         string                     `json:"main_caption"`
		PlatformCaptions    []aiPlatformCaption        `json:"platform_captions"`
		FirstCommentSuggest []aiFirstCommentSuggestion `json:"first_comment_suggestions"`
		Warnings            []string                   `json:"warnings"`
	}
	if err := h.callOpenAIJSON(r, systemPrompt, buildOpenAIBriefPrompt(body, accounts), 0.8, &suggestion); err != nil {
		return aiPostAssistResponse{}, err
	}
	if strings.TrimSpace(suggestion.MainCaption) == "" && len(suggestion.PlatformCaptions) == 0 {
		return aiPostAssistResponse{}, fmt.Errorf("openai returned no usable brief suggestion")
	}
	suggestion.FirstCommentSuggest = filterFirstCommentSuggestions(suggestion.FirstCommentSuggest, accounts)
	return aiPostAssistResponse{
		RequestID:           "aireq_openai_brief",
		Mode:                "brief",
		Summary:             strings.TrimSpace(suggestion.Summary),
		MainCaption:         strings.TrimSpace(suggestion.MainCaption),
		PlatformCaptions:    suggestion.PlatformCaptions,
		FirstCommentSuggest: suggestion.FirstCommentSuggest,
		Warnings:            suggestion.Warnings,
	}, nil
}

func buildOpenAIBriefPrompt(body aiPostAssistRequest, accounts []aiAccountContext) string {
	var b strings.Builder
	b.WriteString("Generate a social media draft from this brief.\n")
	b.WriteString("Brief:\n")
	b.WriteString(strings.TrimSpace(body.Brief))
	b.WriteString("\nObjective: ")
	if strings.TrimSpace(body.Objective) != "" {
		b.WriteString(body.Objective)
	} else {
		b.WriteString("engagement")
	}
	b.WriteString("\nTone: ")
	if strings.TrimSpace(body.Tone) != "" {
		b.WriteString(body.Tone)
	} else {
		b.WriteString("friendly")
	}
	b.WriteString(fmt.Sprintf("\nInclude CTA: %t\n", body.IncludeCTA))
	if len(accounts) > 0 {
		b.WriteString("Accounts:\n")
		for _, account := range accounts {
			b.WriteString("- account_id=" + account.AccountID + ", platform=" + account.Platform + "\n")
		}
	}
	return b.String()
}

func (h *AIPostAssistHandler) loadAccountContexts(r *http.Request, ids []string) ([]aiAccountContext, error) {
	if h.queries == nil {
		return nil, fmt.Errorf("account lookup unavailable")
	}
	workspaceID := auth.GetWorkspaceID(r.Context())
	accounts, err := h.queries.ListSocialAccountsByWorkspace(r.Context(), workspaceID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]db.SocialAccount, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}
	out := make([]aiAccountContext, 0, len(ids))
	for _, id := range ids {
		account, ok := byID[id]
		if !ok {
			continue
		}
		name := ""
		if account.AccountName.Valid {
			name = account.AccountName.String
		}
		out = append(out, aiAccountContext{
			AccountID:   account.ID,
			Platform:    account.Platform,
			AccountName: name,
		})
	}
	return out, nil
}

func buildImproveStubResponse(body aiPostAssistRequest) aiPostAssistResponse {
	base := strings.TrimSpace(body.MainCaption)
	normalized := normalizeWhitespace(base)
	improved := normalized

	if !hasSentenceTerminal(improved) {
		improved += "."
	}
	if !containsCTA(improved) && body.IncludeCTA {
		improved += " Shop the launch now."
	}
	if !strings.Contains(strings.ToLower(improved), "learn more") && !body.IncludeCTA {
		improved += " Learn more in the link in bio."
	}

	return aiPostAssistResponse{
		RequestID:           "aireq_stub_improve",
		Mode:                "improve",
		Summary:             "Refined the draft into a cleaner, more direct version with a stronger close.",
		MainCaption:         improved,
		FirstCommentSuggest: buildFirstCommentStubSuggestions(body, nil),
		Warnings: []string{
			"Stub response: this suggestion is generated by deterministic server logic, not a live model yet.",
		},
	}
}

func buildAdaptStubResponse(body aiPostAssistRequest, accounts []aiAccountContext) aiPostAssistResponse {
	platformCaptions := make([]aiPlatformCaption, 0, len(accounts))
	base := normalizeWhitespace(strings.TrimSpace(body.MainCaption))
	for _, account := range accounts {
		caption := base
		reason := "Kept the core message and adjusted the framing for this destination."
		switch account.Platform {
		case "twitter":
			caption = shortenForX(base)
			reason = "Shortened for X and moved the hook earlier."
		case "linkedin":
			caption = expandForLinkedIn(base, body.IncludeCTA)
			reason = "Expanded the context slightly for a more professional, narrative tone."
		case "instagram":
			caption = softenForInstagram(base, body.IncludeCTA)
			reason = "Made the copy a bit more visual and caption-friendly."
		case "threads":
			caption = addThreadsCadence(base)
			reason = "Kept it conversational and slightly more stream-of-thought."
		}
		platformCaptions = append(platformCaptions, aiPlatformCaption{
			AccountID: account.AccountID,
			Platform:  account.Platform,
			Caption:   caption,
			Reason:    reason,
		})
	}
	return aiPostAssistResponse{
		RequestID:           "aireq_stub_adapt",
		Mode:                "adapt",
		Summary:             "Created per-platform variants from the main draft so each destination has a more native tone.",
		PlatformCaptions:    platformCaptions,
		FirstCommentSuggest: buildFirstCommentStubSuggestions(body, accounts),
		Warnings: []string{
			"Stub response: these variants are generated by deterministic server logic unless a live model is configured.",
		},
	}
}

func buildFixValidationStubResponse(body aiPostAssistRequest, accounts []aiAccountContext) aiPostAssistResponse {
	platformCaptions := make([]aiPlatformCaption, 0, len(accounts))
	warnings := []string{}
	for _, issue := range body.ValidationIssues {
		if issue.Field != "caption" {
			warnings = append(warnings, "Some validation issues still require manual decisions, especially platform-specific compliance fields.")
			continue
		}
		source := captionForAccount(body, issue.AccountID)
		if source == "" {
			source = body.MainCaption
		}
		if strings.TrimSpace(source) == "" {
			continue
		}
		account := findAccountContext(accounts, issue.AccountID, issue.Platform)
		fixed := source
		reason := "Adjusted the caption to better fit the validation rules for this destination."
		switch issue.Code {
		case "exceeds_max_length":
			switch account.Platform {
			case "twitter":
				fixed = shortenForX(source)
				reason = "Shortened the caption to fit X's tighter length limit."
			default:
				fixed = softTrim(source, 220)
				reason = "Tightened the caption by removing lower-priority phrasing."
			}
		case "below_min_length":
			fixed = strings.TrimSpace(source)
			if !hasSentenceTerminal(fixed) {
				fixed += "."
			}
			fixed += " Learn more."
			reason = "Expanded the caption slightly so it clears the minimum content threshold."
		default:
			fixed = normalizeWhitespace(source)
		}
		platformCaptions = append(platformCaptions, aiPlatformCaption{
			AccountID: issue.AccountID,
			Platform:  account.Platform,
			Caption:   fixed,
			Reason:    reason,
		})
	}
	if len(platformCaptions) == 0 && strings.TrimSpace(body.MainCaption) != "" {
		return aiPostAssistResponse{
			RequestID:   "aireq_stub_fix_validation",
			Mode:        "fix_validation",
			Summary:     "No text-only fix could be generated from the current validation issues, so the draft still needs manual review.",
			MainCaption: buildImproveStubResponse(body).MainCaption,
			Warnings:    append(warnings, "Only caption-like issues can be auto-suggested in the current stub."),
		}
	}
	return aiPostAssistResponse{
		RequestID:        "aireq_stub_fix_validation",
		Mode:             "fix_validation",
		Summary:          "Generated targeted caption fixes for the validation issues that can be safely repaired in text.",
		PlatformCaptions: dedupePlatformCaptions(platformCaptions),
		Warnings:         warnings,
	}
}

func buildMediaStubResponse(body aiPostAssistRequest, accounts []aiAccountContext) aiPostAssistResponse {
	base := strings.TrimSpace(body.MainCaption)
	if base == "" {
		base = buildMediaBaseCaption(body)
	}
	resp := aiPostAssistResponse{
		RequestID:           "aireq_stub_media",
		Mode:                "media",
		Summary:             "Built a draft from uploaded media metadata so you can turn visuals into a stronger starting caption.",
		MainCaption:         base,
		FirstCommentSuggest: buildFirstCommentStubSuggestions(body, accounts),
		Warnings: []string{
			"Stub response: this suggestion uses file metadata and names, not deep visual understanding.",
		},
	}
	if len(accounts) > 0 {
		resp.PlatformCaptions = buildAdaptStubResponse(aiPostAssistRequest{
			MainCaption:        base,
			SelectedAccountIDs: body.SelectedAccountIDs,
			IncludeCTA:         body.IncludeCTA,
		}, accounts).PlatformCaptions
	}
	return resp
}

func buildBriefStubResponse(body aiPostAssistRequest, accounts []aiAccountContext) aiPostAssistResponse {
	base := strings.TrimSpace(body.Brief)
	base = normalizeWhitespace(base)
	if base == "" {
		base = "A new campaign is ready to share."
	}

	mainCaption := base
	switch strings.TrimSpace(body.Objective) {
	case "sales":
		mainCaption += " See what's new and grab it while it's live."
	case "clicks":
		mainCaption += " Tap through to get the full details."
	case "awareness":
		mainCaption += " We wanted to share what makes this worth a look."
	default:
		mainCaption += " We'd love to hear what you think."
	}
	if strings.TrimSpace(body.Tone) == "professional" {
		mainCaption = strings.TrimSpace(mainCaption)
	} else if strings.TrimSpace(body.Tone) == "bold" {
		mainCaption = strings.TrimSpace(mainCaption) + " Big moment. Clear value."
	} else if strings.TrimSpace(body.Tone) == "playful" {
		mainCaption = strings.TrimSpace(mainCaption) + " A little fresh energy goes a long way."
	}
	if body.IncludeCTA && !containsCTA(mainCaption) {
		mainCaption += " Take a look."
	}
	if !hasSentenceTerminal(mainCaption) {
		mainCaption += "."
	}

	resp := aiPostAssistResponse{
		RequestID:           "aireq_stub_brief",
		Mode:                "brief",
		Summary:             "Turned the campaign brief into a first-pass caption you can refine or publish from.",
		MainCaption:         mainCaption,
		FirstCommentSuggest: buildFirstCommentStubSuggestions(body, accounts),
		Warnings: []string{
			"Stub response: this draft is generated from the brief fields using deterministic server logic unless a live model is configured.",
		},
	}
	if len(accounts) > 0 {
		resp.PlatformCaptions = buildAdaptStubResponse(aiPostAssistRequest{
			MainCaption:        mainCaption,
			SelectedAccountIDs: body.SelectedAccountIDs,
			IncludeCTA:         body.IncludeCTA,
		}, accounts).PlatformCaptions
	}
	return resp
}

func buildMediaBaseCaption(body aiPostAssistRequest) string {
	if len(body.MediaContext) == 0 {
		return "A new visual-first post is ready to go. Take a look."
	}
	first := body.MediaContext[0]
	kind := "visual"
	if strings.HasPrefix(first.ContentType, "video/") {
		kind = "video"
	} else if strings.HasPrefix(first.ContentType, "image/") {
		kind = "photo"
	}
	caption := fmt.Sprintf("New %s content is ready to share.", kind)
	if strings.TrimSpace(body.Brief) != "" {
		caption += " " + strings.TrimSpace(body.Brief)
	}
	if body.IncludeCTA && !containsCTA(caption) {
		caption += " Take a look."
	}
	return caption
}

func buildFirstCommentStubSuggestions(body aiPostAssistRequest, accounts []aiAccountContext) []aiFirstCommentSuggestion {
	if len(accounts) == 0 {
		return nil
	}
	base := strings.TrimSpace(body.MainCaption)
	if base == "" {
		base = strings.TrimSpace(body.Brief)
	}
	if base == "" {
		base = "More details in the next update."
	}

	out := make([]aiFirstCommentSuggestion, 0, len(accounts))
	for _, account := range accounts {
		if !supportsFirstCommentPlatform(account.Platform) {
			continue
		}
		text := "More details below."
		switch account.Platform {
		case "instagram":
			text = "Save this for later and drop a question below if you want the details."
		case "linkedin":
			text = "Happy to share more context in the comments if that would be useful."
		case "facebook":
			text = "Full details are in the post above. Questions are welcome in the comments."
		case "threads":
			text = "Curious which part stands out most to you."
		case "twitter":
			text = "More context in the thread if you want the full breakdown."
		}
		if body.IncludeCTA && account.Platform != "linkedin" {
			text = appendSentence(text, "Take a look and let us know what you think.")
		}
		out = append(out, aiFirstCommentSuggestion{
			AccountID: account.AccountID,
			Text:      text,
		})
	}
	return out
}

func supportsFirstCommentPlatform(platform string) bool {
	switch platform {
	case "twitter", "instagram", "linkedin":
		return true
	default:
		return false
	}
}

func filterFirstCommentSuggestions(items []aiFirstCommentSuggestion, accounts []aiAccountContext) []aiFirstCommentSuggestion {
	if len(items) == 0 {
		return nil
	}
	platformByAccount := make(map[string]string, len(accounts))
	for _, account := range accounts {
		platformByAccount[account.AccountID] = account.Platform
	}
	out := make([]aiFirstCommentSuggestion, 0, len(items))
	for _, item := range items {
		if !supportsFirstCommentPlatform(platformByAccount[item.AccountID]) {
			continue
		}
		if strings.TrimSpace(item.Text) == "" {
			continue
		}
		out = append(out, aiFirstCommentSuggestion{
			AccountID: item.AccountID,
			Text:      strings.TrimSpace(item.Text),
		})
	}
	return out
}

func captionForAccount(body aiPostAssistRequest, accountID string) string {
	for _, post := range body.PlatformPosts {
		if post.AccountID == accountID {
			return strings.TrimSpace(post.Caption)
		}
	}
	return ""
}

func findAccountContext(accounts []aiAccountContext, accountID, platform string) aiAccountContext {
	for _, account := range accounts {
		if account.AccountID == accountID {
			return account
		}
	}
	return aiAccountContext{AccountID: accountID, Platform: platform}
}

func softTrim(s string, max int) string {
	s = normalizeWhitespace(strings.TrimSpace(s))
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return strings.TrimSpace(s[:max-3]) + "..."
}

func dedupePlatformCaptions(items []aiPlatformCaption) []aiPlatformCaption {
	seen := make(map[string]aiPlatformCaption, len(items))
	order := make([]string, 0, len(items))
	for _, item := range items {
		key := item.AccountID
		if key == "" {
			key = item.Platform
		}
		if _, ok := seen[key]; !ok {
			order = append(order, key)
		}
		seen[key] = item
	}
	out := make([]aiPlatformCaption, 0, len(order))
	for _, key := range order {
		out = append(out, seen[key])
	}
	return out
}

func shortenForX(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 180 {
		if !containsCTA(s) {
			s += " Learn more."
		}
		return s
	}
	return strings.TrimSpace(s[:177]) + "..."
}

func expandForLinkedIn(s string, includeCTA bool) string {
	s = strings.TrimSpace(s)
	if includeCTA && !containsCTA(s) {
		return s + " If this resonates, take a look and see what fits your workflow."
	}
	return s + " Curious how others are approaching this?"
}

func softenForInstagram(s string, includeCTA bool) string {
	s = strings.TrimSpace(s)
	if includeCTA && !containsCTA(s) {
		return s + " Tap through to check it out."
	}
	return s
}

func addThreadsCadence(s string) string {
	s = strings.TrimSpace(s)
	if !hasSentenceTerminal(s) {
		s += "."
	}
	return s + " More soon."
}

func appendSentence(base, suffix string) string {
	base = strings.TrimSpace(base)
	suffix = strings.TrimSpace(suffix)
	if base == "" {
		return suffix
	}
	if suffix == "" {
		return base
	}
	if !hasSentenceTerminal(base) {
		base += "."
	}
	return base + " " + suffix
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func hasSentenceTerminal(s string) bool {
	return strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?")
}

func containsCTA(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "shop now") ||
		strings.Contains(lower, "learn more") ||
		strings.Contains(lower, "get yours") ||
		strings.Contains(lower, "try it") ||
		strings.Contains(lower, "sign up")
}
