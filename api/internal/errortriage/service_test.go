package errortriage

import (
	"context"
	"testing"
	"time"
)

func TestServiceRunCreatesItemsRecipientsAndCompletesRun(t *testing.T) {
	store := &fakeStore{
		failures: []Failure{
			{PostID: "post_1", WorkspaceID: "ws_1", UserID: "user_1", UserEmail: "one@example.com", Platform: "threads", Source: "dashboard", ErrorCode: "missing_permission", FailureStage: "publish", Message: "reconnect required", CreatedAt: time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC)},
			{PostID: "post_2", WorkspaceID: "ws_2", UserID: "user_2", UserEmail: "two@example.com", Platform: "tiktok", Source: "api", ErrorCode: "invalid_params", FailureStage: "upload_init", Message: "chunk size invalid", CreatedAt: time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC)},
		},
	}
	svc := NewService(store, DeterministicAnalyzer{})

	run, err := svc.Run(context.Background(), RunOptions{
		RunType:     RunTypeManual,
		WindowStart: time.Date(2026, 6, 6, 7, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 6, 7, 7, 0, 0, 0, time.UTC),
		AdminUserID: "admin_1",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if run.Status != RunStatusCompleted {
		t.Fatalf("run status = %q, want completed", run.Status)
	}
	if got, want := store.completed.FailuresAnalyzed, 2; got != want {
		t.Fatalf("failures analyzed = %d, want %d", got, want)
	}
	if got, want := len(store.items), 2; got != want {
		t.Fatalf("items inserted = %d, want %d", got, want)
	}
	if got, want := len(store.recipients), 1; got != want {
		t.Fatalf("recipients inserted = %d, want %d", got, want)
	}
	if store.items[0].draft.DedupeKey == "" || store.items[1].draft.DedupeKey == "" {
		t.Fatalf("expected dedupe keys on inserted items: %#v", store.items)
	}
}

func TestServiceRunCapsAnalyzerCallsAndFallsBackDeterministically(t *testing.T) {
	failures := make([]Failure, 0, DefaultMaxAIBucketsPerRun+1)
	for i := 0; i < DefaultMaxAIBucketsPerRun+1; i++ {
		failures = append(failures, Failure{
			PostID:       "post_" + strconvItoa(i),
			WorkspaceID:  "ws_" + strconvItoa(i),
			UserID:       "user_" + strconvItoa(i),
			UserEmail:    "user" + strconvItoa(i) + "@example.com",
			Platform:     "threads",
			ErrorCode:    "missing_permission_" + strconvItoa(i),
			FailureStage: "publish",
			Message:      "reconnect required " + strconvItoa(i),
			CreatedAt:    time.Date(2026, 6, 6, 8, i%60, 0, 0, time.UTC),
		})
	}
	store := &fakeStore{failures: failures}
	analyzer := &countingAnalyzer{}
	svc := NewService(store, analyzer)

	if _, err := svc.Run(context.Background(), RunOptions{
		RunType:     RunTypeManual,
		WindowStart: time.Date(2026, 6, 6, 7, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 6, 7, 7, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if got, want := analyzer.calls, DefaultMaxAIBucketsPerRun; got != want {
		t.Fatalf("analyzer calls = %d, want %d", got, want)
	}
	if got, want := len(store.items), DefaultMaxAIBucketsPerRun+1; got != want {
		t.Fatalf("inserted items = %d, want %d", got, want)
	}
}

type countingAnalyzer struct {
	calls int
}

func (a *countingAnalyzer) Analyze(bucket Bucket) ItemDraft {
	a.calls++
	return DeterministicAnalyzer{}.Analyze(bucket)
}

type fakeStore struct {
	failures   []Failure
	items      []fakeItemInsert
	recipients []RecipientCandidate
	completed  CompleteRunParams
}

type fakeItemInsert struct {
	runID       string
	draft       ItemDraft
	duplicateID string
}

func (s *fakeStore) CreateRun(ctx context.Context, params CreateRunParams) (RunRecord, bool, error) {
	return RunRecord{ID: "run_1", RunType: params.RunType, Status: RunStatusRunning, WindowStart: params.WindowStart, WindowEnd: params.WindowEnd}, true, nil
}

func (s *fakeStore) CompleteRun(ctx context.Context, runID string, params CompleteRunParams) (RunRecord, error) {
	s.completed = params
	return RunRecord{ID: runID, RunType: RunTypeManual, Status: RunStatusCompleted, FailuresAnalyzed: params.FailuresAnalyzed}, nil
}

func (s *fakeStore) FailRun(ctx context.Context, runID string, message string) error {
	return nil
}

func (s *fakeStore) LoadFailures(ctx context.Context, start, end time.Time) ([]Failure, error) {
	return s.failures, nil
}

func (s *fakeStore) FindPreviousItem(ctx context.Context, dedupeKey, runID string) (string, error) {
	return "", nil
}

func (s *fakeStore) InsertItem(ctx context.Context, runID string, draft ItemDraft, duplicateID string) (string, error) {
	s.items = append(s.items, fakeItemInsert{runID: runID, draft: draft, duplicateID: duplicateID})
	return "item_" + strconvItoa(len(s.items)), nil
}

func (s *fakeStore) InsertItemFailure(ctx context.Context, itemID string, failure Failure) error {
	return nil
}

func (s *fakeStore) InsertRecipient(ctx context.Context, itemID string, recipient RecipientCandidate) error {
	s.recipients = append(s.recipients, recipient)
	return nil
}
