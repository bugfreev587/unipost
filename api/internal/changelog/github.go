package changelog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type DispatchRequest struct {
	CandidateID     string
	SourceHash      string
	ActionRequestID string
	RequestedBy     string
	DryRun          bool
	Ref             string
	Workflow        string
}

type DispatchResult struct {
	WorkflowURL string `json:"workflow_url,omitempty"`
}

type Dispatcher interface {
	Dispatch(context.Context, DispatchRequest) (DispatchResult, error)
}

type GitHubDispatcher struct {
	Token   string
	Repo    string
	Client  *http.Client
	APIBase string
}

func (d *GitHubDispatcher) Dispatch(ctx context.Context, req DispatchRequest) (DispatchResult, error) {
	token := strings.TrimSpace(d.Token)
	repo := strings.Trim(strings.TrimSpace(d.Repo), "/")
	workflow := strings.TrimSpace(req.Workflow)
	if token == "" {
		return DispatchResult{}, errors.New("CHANGELOG_RELEASE_GITHUB_TOKEN is not configured")
	}
	if repo == "" {
		return DispatchResult{}, errors.New("CHANGELOG_GITHUB_REPO is not configured")
	}
	if workflow == "" {
		return DispatchResult{}, errors.New("changelog publish workflow is not configured")
	}
	ref := strings.TrimSpace(req.Ref)
	if ref == "" {
		ref = "main"
	}
	apiBase := strings.TrimRight(strings.TrimSpace(d.APIBase), "/")
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	body := map[string]any{
		"ref": ref,
		"inputs": map[string]string{
			"candidate_id":      req.CandidateID,
			"source_hash":       req.SourceHash,
			"action_request_id": req.ActionRequestID,
			"requested_by":      req.RequestedBy,
			"dry_run":           fmt.Sprintf("%t", req.DryRun),
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return DispatchResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/actions/workflows/%s/dispatches", apiBase, repo, workflow), bytes.NewReader(raw))
	if err != nil {
		return DispatchResult{}, err
	}
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(httpReq)
	if err != nil {
		return DispatchResult{}, err
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return DispatchResult{}, fmt.Errorf("github workflow dispatch returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(payload)))
	}
	return DispatchResult{WorkflowURL: fmt.Sprintf("https://github.com/%s/actions/workflows/%s", repo, workflow)}, nil
}
