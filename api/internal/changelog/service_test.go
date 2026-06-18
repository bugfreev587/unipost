package changelog

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryStore struct {
	record CandidateRecord
}

func (s *memoryStore) GetCandidate(_ context.Context, id string) (CandidateRecord, error) {
	if s.record.ID != id {
		return CandidateRecord{}, ErrCandidateNotFound
	}
	return s.record, nil
}

func (s *memoryStore) ClaimCandidate(_ context.Context, id string, from []CandidateStatus, to CandidateStatus, actor string) (CandidateRecord, error) {
	if s.record.ID != id {
		return CandidateRecord{}, ErrCandidateNotFound
	}
	for _, allowed := range from {
		if s.record.Status == allowed {
			s.record.Status = to
			s.record.ActedByAdminID = actor
			return s.record, nil
		}
	}
	return s.record, ErrCandidateAlreadyHandled
}

func (s *memoryStore) MarkCandidateFailed(_ context.Context, id string, message string) error {
	if s.record.ID != id {
		return ErrCandidateNotFound
	}
	s.record.Status = StatusFailed
	s.record.ErrorMessage = message
	return nil
}

func (s *memoryStore) SetDispatchMetadata(_ context.Context, id string, requestID string, workflowURL string) error {
	if s.record.ID != id {
		return ErrCandidateNotFound
	}
	s.record.ActionRequestID = requestID
	s.record.WorkflowRunURL = workflowURL
	return nil
}

type fakeDispatcher struct {
	err error
	req DispatchRequest
}

func (d *fakeDispatcher) Dispatch(ctx context.Context, req DispatchRequest) (DispatchResult, error) {
	d.req = req
	if d.err != nil {
		return DispatchResult{}, d.err
	}
	return DispatchResult{WorkflowURL: "https://github.com/bugfreev587/unipost/actions/workflows/changelog-publish.yml"}, nil
}

func TestHandleActionSavesAndRejectsDuplicateClicks(t *testing.T) {
	store := &memoryStore{record: CandidateRecord{
		ID:         "candidate-1",
		SourceHash: "source-hash",
		Status:     StatusPending,
		Payload:    validPayload(),
	}}
	svc := NewService(store, NewSigner("secret"), &fakeDispatcher{}, ServiceConfig{})
	expires := time.Now().Add(time.Hour)
	signature := svc.signer.Sign("candidate-1", ActionSave, expires, "source-hash")

	result, err := svc.HandleAction(context.Background(), ActionRequest{
		CandidateID:  "candidate-1",
		Action:       ActionSave,
		ExpiresUnix:  expires.Unix(),
		Signature:    signature,
		ActorAdminID: "admin_1",
	})
	if err != nil {
		t.Fatalf("HandleAction save returned %v", err)
	}
	if result.Status != StatusSaved {
		t.Fatalf("status = %q, want %q", result.Status, StatusSaved)
	}

	_, err = svc.HandleAction(context.Background(), ActionRequest{
		CandidateID:  "candidate-1",
		Action:       ActionSave,
		ExpiresUnix:  expires.Unix(),
		Signature:    signature,
		ActorAdminID: "admin_1",
	})
	if !errors.Is(err, ErrCandidateAlreadyHandled) {
		t.Fatalf("duplicate HandleAction error = %v, want ErrCandidateAlreadyHandled", err)
	}
}

func TestHandleActionPublishDispatchesWorkflowAndStoresMetadata(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	store := &memoryStore{record: CandidateRecord{
		ID:         "candidate-1",
		SourceHash: "source-hash",
		Status:     StatusPending,
		Payload:    validPayload(),
	}}
	svc := NewService(store, NewSigner("secret"), dispatcher, ServiceConfig{
		GitHubRef:      "main",
		GitHubWorkflow: "changelog-publish.yml",
	})
	expires := time.Now().Add(time.Hour)
	signature := svc.signer.Sign("candidate-1", ActionPublish, expires, "source-hash")

	result, err := svc.HandleAction(context.Background(), ActionRequest{
		CandidateID:  "candidate-1",
		Action:       ActionPublish,
		ExpiresUnix:  expires.Unix(),
		Signature:    signature,
		ActorAdminID: "admin_1",
	})
	if err != nil {
		t.Fatalf("HandleAction publish returned %v", err)
	}
	if result.Status != StatusPublishing {
		t.Fatalf("status = %q, want %q", result.Status, StatusPublishing)
	}
	if dispatcher.req.CandidateID != "candidate-1" || dispatcher.req.SourceHash != "source-hash" {
		t.Fatalf("dispatch request = %#v", dispatcher.req)
	}
	if store.record.WorkflowRunURL == "" {
		t.Fatal("workflow URL was not stored")
	}
}

func TestHandleActionPublishMarksFailedWhenDispatchFails(t *testing.T) {
	store := &memoryStore{record: CandidateRecord{
		ID:         "candidate-1",
		SourceHash: "source-hash",
		Status:     StatusPending,
		Payload:    validPayload(),
	}}
	svc := NewService(store, NewSigner("secret"), &fakeDispatcher{err: errors.New("missing token")}, ServiceConfig{})
	expires := time.Now().Add(time.Hour)
	signature := svc.signer.Sign("candidate-1", ActionPublish, expires, "source-hash")

	_, err := svc.HandleAction(context.Background(), ActionRequest{
		CandidateID:  "candidate-1",
		Action:       ActionPublish,
		ExpiresUnix:  expires.Unix(),
		Signature:    signature,
		ActorAdminID: "admin_1",
	})
	if err == nil {
		t.Fatal("HandleAction publish returned nil, want dispatch error")
	}
	if store.record.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", store.record.Status, StatusFailed)
	}
}
