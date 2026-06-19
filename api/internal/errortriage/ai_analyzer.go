package errortriage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultOpenAIChatCompletionsURL = "https://api.openai.com/v1/chat/completions"
	minAIConfidenceForAction        = 0.70
)

type OpenAIAnalyzer struct {
	apiKey   string
	model    string
	baseURL  string
	client   *http.Client
	fallback Analyzer
}

func NewOpenAIAnalyzerFromEnv(fallback Analyzer) *OpenAIAnalyzer {
	model := strings.TrimSpace(os.Getenv("OPENAI_ERROR_TRIAGE_MODEL"))
	if model == "" {
		model = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	}
	if model == "" {
		model = "gpt-4.1-mini"
	}
	baseURL := strings.TrimSpace(os.Getenv("OPENAI_ERROR_TRIAGE_URL"))
	if baseURL == "" {
		baseURL = defaultOpenAIChatCompletionsURL
	}
	return NewOpenAIAnalyzer(strings.TrimSpace(os.Getenv("OPENAI_API_KEY")), model, baseURL, http.DefaultClient, fallback)
}

func NewOpenAIAnalyzer(apiKey, model, baseURL string, client *http.Client, fallback Analyzer) *OpenAIAnalyzer {
	if fallback == nil {
		fallback = DeterministicAnalyzer{}
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-4.1-mini"
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOpenAIChatCompletionsURL
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &OpenAIAnalyzer{
		apiKey:   strings.TrimSpace(apiKey),
		model:    strings.TrimSpace(model),
		baseURL:  strings.TrimSpace(baseURL),
		client:   client,
		fallback: fallback,
	}
}

func (a *OpenAIAnalyzer) Enabled() bool {
	return a != nil && a.apiKey != ""
}

func (a *OpenAIAnalyzer) ModelName() string {
	if !a.Enabled() {
		return "deterministic"
	}
	return "openai:" + a.model
}

func (a *OpenAIAnalyzer) Analyze(bucket Bucket) ItemDraft {
	fallback := DeterministicAnalyzer{}.Analyze(bucket)
	if a != nil && a.fallback != nil {
		fallback = a.fallback.Analyze(bucket)
	}
	if !a.Enabled() {
		return fallback
	}
	suggestion, err := a.call(bucket, fallback)
	if err != nil {
		slog.Warn("error triage: openai analyzer fallback", "error", err)
		return fallback
	}
	merged, ok := mergeAISuggestion(fallback, suggestion)
	if !ok {
		return fallback
	}
	return merged
}

type aiTriageSuggestion struct {
	Classification Classification `json:"classification"`
	Confidence     float64        `json:"confidence"`
	Summary        string         `json:"summary"`
	ReviewAnalysis ReviewAnalysis `json:"review_analysis"`
	EmailDraft     EmailDraft     `json:"email_draft"`
	BugPlan        BugPlan        `json:"bug_plan"`
	CTAURL         string         `json:"cta_url"`
	Safety         struct {
		RequiresHumanReview bool   `json:"requires_human_review"`
		Reason              string `json:"reason"`
	} `json:"safety"`
}

func (a *OpenAIAnalyzer) call(bucket Bucket, fallback ItemDraft) (aiTriageSuggestion, error) {
	reqBody := map[string]any{
		"model": a.model,
		"messages": []map[string]string{
			{"role": "system", "content": strings.Join([]string{
				"You triage UniPost publishing failures for an admin-only operations dashboard.",
				"Return strict JSON only.",
				"Classify each bucket as one of: unipost_bug, user_action_needed, upstream_platform_issue, transient_no_action, needs_human_review.",
				"Do not mark known duplicates; dedupe is handled by deterministic code.",
				"For user_action_needed, include email_draft with subject and body.",
				"For unipost_bug, include bug_plan with title, impact, evidence, suspected_area, proposed_fix, validation_plan, rollback_plan.",
				"For every classification, include review_analysis with what_is_this_error, why_it_happened, how_to_resolve, missing_evidence, and next_inspection_path.",
				"Do not include secrets or raw tokens in any output.",
				"Set safety.requires_human_review=true if the email draft or fix plan could be unsafe, uncertain, or contain sensitive data.",
			}, " ")},
			{"role": "user", "content": buildOpenAITriagePrompt(bucket, fallback)},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.2,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return aiTriageSuggestion{}, err
	}
	req, err := http.NewRequest(http.MethodPost, a.baseURL, bytes.NewReader(raw))
	if err != nil {
		return aiTriageSuggestion{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	res, err := a.client.Do(req)
	if err != nil {
		return aiTriageSuggestion{}, err
	}
	defer res.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return aiTriageSuggestion{}, err
	}
	if res.StatusCode >= 300 {
		return aiTriageSuggestion{}, fmt.Errorf("openai returned HTTP %d", res.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return aiTriageSuggestion{}, err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return aiTriageSuggestion{}, errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return aiTriageSuggestion{}, errors.New("openai returned no choices")
	}
	var suggestion aiTriageSuggestion
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &suggestion); err != nil {
		return aiTriageSuggestion{}, err
	}
	return suggestion, nil
}

func buildOpenAITriagePrompt(bucket Bucket, fallback ItemDraft) string {
	evidence := buildEvidence(bucket)
	rawEvidence, _ := json.Marshal(evidence)
	var b strings.Builder
	b.WriteString("Deterministic preclassification: ")
	b.WriteString(string(fallback.Classification))
	b.WriteString("\nDeterministic action: ")
	b.WriteString(string(fallback.ActionKind))
	b.WriteString("\nAffected users: ")
	b.WriteString(strconvItoa(bucket.AffectedUserCount))
	b.WriteString("\nAffected workspaces: ")
	b.WriteString(strconvItoa(bucket.AffectedWorkspaceCount))
	b.WriteString("\nAffected posts: ")
	b.WriteString(strconvItoa(bucket.AffectedPostCount))
	b.WriteString("\nEvidence JSON:\n")
	b.Write(rawEvidence)
	b.WriteString("\nReturn JSON with keys: classification, confidence, summary, review_analysis, email_draft, bug_plan, cta_url, safety.")
	return b.String()
}

func mergeAISuggestion(fallback ItemDraft, suggestion aiTriageSuggestion) (ItemDraft, bool) {
	if !validClassification(suggestion.Classification) {
		return ItemDraft{}, false
	}
	if aiOutputContainsSecret(suggestion) {
		return aiNeedsHumanReview(fallback, suggestion.Confidence, "model output contained secret-shaped content"), true
	}
	if suggestion.Safety.RequiresHumanReview {
		return aiNeedsHumanReview(fallback, suggestion.Confidence, firstNonEmpty(suggestion.Safety.Reason, "model requested human review")), true
	}
	if suggestion.Confidence > 0 && suggestion.Confidence < minAIConfidenceForAction {
		return aiNeedsHumanReview(fallback, suggestion.Confidence, "model confidence below threshold"), true
	}
	out := fallback
	action, status := workflowForClassification(suggestion.Classification)
	out.Classification = suggestion.Classification
	out.ActionKind = action
	out.WorkflowStatus = status
	if suggestion.Confidence > 0 && suggestion.Confidence <= 1 {
		out.Confidence = suggestion.Confidence
	}
	if strings.TrimSpace(suggestion.Summary) != "" {
		out.Summary = strings.TrimSpace(suggestion.Summary)
	}
	if reviewAnalysis := normalizeReviewAnalysis(suggestion.ReviewAnalysis); len(reviewAnalysis) > 0 {
		out.ReviewAnalysis = reviewAnalysis
		setReviewAnalysisEvidence(&out, reviewAnalysis)
	}
	out.EmailDraft = EmailDraft{}
	out.BugPlan = BugPlan{}
	out.CTAURL = strings.TrimSpace(suggestion.CTAURL)
	switch suggestion.Classification {
	case ClassificationUserActionNeeded:
		if strings.TrimSpace(suggestion.EmailDraft.Subject) != "" || strings.TrimSpace(suggestion.EmailDraft.Body) != "" {
			out.EmailDraft = suggestion.EmailDraft
			if out.CTAURL == "" {
				out.CTAURL = strings.TrimSpace(suggestion.EmailDraft.CTAURL)
			}
		} else {
			out.EmailDraft = fallback.EmailDraft
			out.CTAURL = fallback.CTAURL
		}
	case ClassificationUnipostBug:
		if strings.TrimSpace(suggestion.BugPlan.Title) != "" || strings.TrimSpace(suggestion.BugPlan.ProposedFix) != "" {
			out.BugPlan = suggestion.BugPlan
		} else {
			out.BugPlan = fallback.BugPlan
		}
	}
	return out, true
}

func aiNeedsHumanReview(fallback ItemDraft, confidence float64, reason string) ItemDraft {
	out := fallback
	out.Classification = ClassificationNeedsHumanReview
	out.ActionKind = ActionKindReview
	out.WorkflowStatus = WorkflowStatusPendingReview
	if confidence > 0 && confidence <= 1 {
		out.Confidence = confidence
	} else if out.Confidence > 0.49 {
		out.Confidence = 0.49
	}
	out.Summary = "AI triage requires human review: " + strings.TrimSpace(reason) + "."
	out.EmailDraft = EmailDraft{}
	out.BugPlan = BugPlan{}
	out.CTAURL = ""
	reviewAnalysis := normalizeReviewAnalysis(out.ReviewAnalysis)
	if len(reviewAnalysis) == 0 {
		reviewAnalysis = ReviewAnalysis{
			"what_is_this_error":   "AI triage could not safely classify this failure bucket.",
			"why_it_happened":      "AI requested human review because " + strings.TrimSpace(reason) + ".",
			"how_to_resolve":       "Open the raw failure evidence, inspect the provider response and worker logs, then classify the bucket manually.",
			"missing_evidence":     "A safer classification needs more concrete provider, account, or worker evidence.",
			"next_inspection_path": "Start from the linked raw error sample and compare it with logs for the same post.",
		}
	} else {
		reviewAnalysis["why_it_happened"] = "AI requested human review because " + strings.TrimSpace(reason) + ". " + strings.TrimSpace(reviewAnalysis["why_it_happened"])
		if strings.TrimSpace(reviewAnalysis["how_to_resolve"]) == "" {
			reviewAnalysis["how_to_resolve"] = "Open the raw failure evidence, inspect the provider response and worker logs, then classify the bucket manually."
		}
	}
	out.ReviewAnalysis = reviewAnalysis
	setReviewAnalysisEvidence(&out, reviewAnalysis)
	return out
}

func aiOutputContainsSecret(suggestion aiTriageSuggestion) bool {
	parts := []string{
		suggestion.Summary,
		suggestion.CTAURL,
		suggestion.EmailDraft.Subject,
		suggestion.EmailDraft.Body,
		suggestion.EmailDraft.CTAURL,
		suggestion.BugPlan.Title,
		suggestion.BugPlan.Impact,
		suggestion.BugPlan.SuspectedArea,
		suggestion.BugPlan.ProposedFix,
		suggestion.BugPlan.ValidationPlan,
		suggestion.BugPlan.RollbackPlan,
	}
	parts = append(parts, suggestion.BugPlan.Evidence...)
	for _, value := range suggestion.ReviewAnalysis {
		parts = append(parts, value)
	}
	return ContainsSecretPattern(strings.Join(parts, "\n"))
}

func normalizeReviewAnalysis(analysis ReviewAnalysis) ReviewAnalysis {
	out := ReviewAnalysis{}
	for _, key := range []string{"what_is_this_error", "why_it_happened", "how_to_resolve", "missing_evidence", "next_inspection_path"} {
		if value := strings.TrimSpace(analysis[key]); value != "" {
			out[key] = value
		}
	}
	return out
}

func setReviewAnalysisEvidence(out *ItemDraft, analysis ReviewAnalysis) {
	if out.Evidence == nil {
		out.Evidence = map[string]any{}
	}
	out.Evidence["review_analysis"] = map[string]string(analysis)
}

func validClassification(classification Classification) bool {
	switch classification {
	case ClassificationUnipostBug,
		ClassificationUserActionNeeded,
		ClassificationUpstreamPlatformIssue,
		ClassificationTransientNoAction,
		ClassificationNeedsHumanReview:
		return true
	default:
		return false
	}
}
