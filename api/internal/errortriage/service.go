package errortriage

import (
	"context"
	"fmt"
	"time"
)

type RunType string

const (
	RunTypeScheduled RunType = "scheduled"
	RunTypeManual    RunType = "manual"
)

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

const PromptVersion = "error-triage-v1"

type RunOptions struct {
	RunType         RunType
	WindowStart     time.Time
	WindowEnd       time.Time
	AdminUserID     string
	SupersedesRunID string
}

type RunRecord struct {
	ID               string    `json:"id"`
	RunType          RunType   `json:"run_type"`
	Status           RunStatus `json:"status"`
	WindowStart      time.Time `json:"window_start"`
	WindowEnd        time.Time `json:"window_end"`
	FailuresAnalyzed int       `json:"failures_analyzed"`
}

type CreateRunParams struct {
	RunType         RunType
	WindowStart     time.Time
	WindowEnd       time.Time
	AdminUserID     string
	SupersedesRunID string
}

type CompleteRunParams struct {
	Model              string
	PromptVersion      string
	FailuresAnalyzed   int
	AffectedUsers      int
	AffectedWorkspaces int
	Summary            string
}

type Store interface {
	CreateRun(ctx context.Context, params CreateRunParams) (RunRecord, bool, error)
	CompleteRun(ctx context.Context, runID string, params CompleteRunParams) (RunRecord, error)
	FailRun(ctx context.Context, runID string, message string) error
	LoadFailures(ctx context.Context, start, end time.Time) ([]Failure, error)
	FindPreviousItem(ctx context.Context, dedupeKey, runID string) (string, error)
	InsertItem(ctx context.Context, runID string, draft ItemDraft, duplicateID string) (string, error)
	InsertItemFailure(ctx context.Context, itemID string, failure Failure) error
	InsertRecipient(ctx context.Context, itemID string, recipient RecipientCandidate) error
}

type Analyzer interface {
	Analyze(bucket Bucket) ItemDraft
}

type Service struct {
	store    Store
	analyzer Analyzer
	model    string
}

func NewService(store Store, analyzer Analyzer) *Service {
	if analyzer == nil {
		analyzer = DeterministicAnalyzer{}
	}
	return &Service{store: store, analyzer: analyzer, model: "deterministic"}
}

func (s *Service) WithModelName(model string) *Service {
	if model != "" {
		s.model = model
	}
	return s
}

func (s *Service) Run(ctx context.Context, opts RunOptions) (RunRecord, error) {
	if s == nil || s.store == nil {
		return RunRecord{}, fmt.Errorf("error triage service is not configured")
	}
	run, created, err := s.store.CreateRun(ctx, CreateRunParams{
		RunType:         opts.RunType,
		WindowStart:     opts.WindowStart,
		WindowEnd:       opts.WindowEnd,
		AdminUserID:     opts.AdminUserID,
		SupersedesRunID: opts.SupersedesRunID,
	})
	if err != nil {
		return RunRecord{}, err
	}
	if !created {
		return run, nil
	}

	failures, err := s.store.LoadFailures(ctx, opts.WindowStart, opts.WindowEnd)
	if err != nil {
		_ = s.store.FailRun(ctx, run.ID, err.Error())
		return RunRecord{}, err
	}

	buckets := BuildBuckets(failures)
	for _, bucket := range buckets {
		draft := s.analyzer.Analyze(bucket)
		duplicateID, err := s.store.FindPreviousItem(ctx, draft.DedupeKey, run.ID)
		if err != nil {
			_ = s.store.FailRun(ctx, run.ID, err.Error())
			return RunRecord{}, err
		}
		itemID, err := s.store.InsertItem(ctx, run.ID, draft, duplicateID)
		if err != nil {
			_ = s.store.FailRun(ctx, run.ID, err.Error())
			return RunRecord{}, err
		}
		for _, failure := range bucket.Failures {
			if err := s.store.InsertItemFailure(ctx, itemID, failure); err != nil {
				_ = s.store.FailRun(ctx, run.ID, err.Error())
				return RunRecord{}, err
			}
		}
		if draft.ActionKind == ActionKindEmail {
			for _, recipient := range bucket.Recipients {
				if err := s.store.InsertRecipient(ctx, itemID, recipient); err != nil {
					_ = s.store.FailRun(ctx, run.ID, err.Error())
					return RunRecord{}, err
				}
			}
		}
	}

	completed, err := s.store.CompleteRun(ctx, run.ID, CompleteRunParams{
		Model:              s.model,
		PromptVersion:      PromptVersion,
		FailuresAnalyzed:   len(failures),
		AffectedUsers:      countUsers(failures),
		AffectedWorkspaces: countWorkspaces(failures),
		Summary:            buildRunSummary(len(failures), buckets),
	})
	if err != nil {
		_ = s.store.FailRun(ctx, run.ID, err.Error())
		return RunRecord{}, err
	}
	return completed, nil
}

func countUsers(failures []Failure) int {
	seen := map[string]bool{}
	for _, failure := range failures {
		if failure.UserID != "" {
			seen[failure.UserID] = true
		}
	}
	return len(seen)
}

func countWorkspaces(failures []Failure) int {
	seen := map[string]bool{}
	for _, failure := range failures {
		if failure.WorkspaceID != "" {
			seen[failure.WorkspaceID] = true
		}
	}
	return len(seen)
}

func buildRunSummary(failureCount int, buckets []Bucket) string {
	if failureCount == 0 {
		return "No publishing failures were found in this window."
	}
	if len(buckets) == 0 {
		return "Publishing failures were found, but no triage buckets were created."
	}
	return "Analyzed " + strconvItoa(failureCount) + " publishing failure(s) across " + strconvItoa(len(buckets)) + " triage bucket(s)."
}
