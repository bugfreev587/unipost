package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/changelog"
)

type fakeChangelogStore struct {
	record  changelog.CandidateRecord
	created bool
	err     error
}

func (s *fakeChangelogStore) CreateCandidate(_ context.Context, input changelog.CreateCandidateInput) (changelog.CandidateRecord, bool, error) {
	if s.err != nil {
		return changelog.CandidateRecord{}, false, s.err
	}
	s.record = changelog.CandidateRecord{
		ID:          input.Payload.Candidate.ID,
		SourceHash:  input.SourceHash,
		Status:      changelog.StatusPending,
		Payload:     input.Payload,
		WindowStart: input.WindowStart,
		WindowEnd:   input.WindowEnd,
	}
	return s.record, s.created, nil
}

func (s *fakeChangelogStore) GetCandidate(_ context.Context, id string) (changelog.CandidateRecord, error) {
	if s.record.ID != id {
		return changelog.CandidateRecord{}, changelog.ErrCandidateNotFound
	}
	return s.record, nil
}

type fakeChangelogActions struct {
	record changelog.CandidateRecord
	result changelog.ActionResult
	err    error
}

func (s *fakeChangelogActions) BuildActionLinks(changelog.CandidateRecord) changelog.ActionLinks {
	return changelog.ActionLinks{
		Publish: "https://app.unipost.dev/admin/changelog-actions?action=publish",
		Save:    "https://app.unipost.dev/admin/changelog-actions?action=save",
		Discard: "https://app.unipost.dev/admin/changelog-actions?action=discard",
	}
}

func (s *fakeChangelogActions) VerifyActionLink(_ context.Context, id string, action changelog.Action, _ int64, _ string) (changelog.CandidateRecord, error) {
	if id != s.record.ID || !changelog.ValidAction(action) {
		return changelog.CandidateRecord{}, changelog.ErrInvalidSignature
	}
	return s.record, nil
}

func (s *fakeChangelogActions) HandleAction(_ context.Context, req changelog.ActionRequest) (changelog.ActionResult, error) {
	if s.err != nil {
		return changelog.ActionResult{}, s.err
	}
	s.result.CandidateID = req.CandidateID
	s.result.Action = req.Action
	return s.result, nil
}

func TestCreateInternalCandidateRequiresAutomationToken(t *testing.T) {
	h := NewChangelogAutomationHandler(&fakeChangelogStore{}, &fakeChangelogActions{}, "secret")
	req := httptest.NewRequest(http.MethodPost, "/internal/changelog-candidates", nil)
	rr := httptest.NewRecorder()

	h.CreateInternalCandidate(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCreateInternalCandidateReturnsActionLinks(t *testing.T) {
	payload := changelog.CandidatePayload{
		HasCandidate: true,
		Candidate: changelog.Candidate{
			ID:             "developer-logs-api",
			Date:           "2026-06-18",
			Title:          "Developer Logs API",
			Summary:        "Workspace-scoped logs.",
			Category:       changelog.CategoryReliability,
			Impact:         changelog.ImpactNew,
			WhyUserVisible: "Developers can inspect logs.",
			Links:          []changelog.Link{{Label: "Docs", Href: "/docs/api/logs"}},
			SourceLinks:    []changelog.Link{{Label: "PR", Href: "https://github.com/bugfreev587/unipost/pull/67"}},
		},
	}
	body, _ := json.Marshal(changelog.CreateCandidateInput{
		Payload:     payload,
		SourceHash:  "source-hash",
		WindowStart: time.Date(2026, 6, 17, 7, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 6, 18, 7, 0, 0, 0, time.UTC),
	})
	store := &fakeChangelogStore{created: true}
	h := NewChangelogAutomationHandler(store, &fakeChangelogActions{}, "secret")
	req := httptest.NewRequest(http.MethodPost, "/internal/changelog-candidates", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	h.CreateInternalCandidate(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("changelog-actions?action=publish")) {
		t.Fatalf("response missing publish link: %s", rr.Body.String())
	}
}

func TestGetAdminCandidateRequiresSignedQuery(t *testing.T) {
	actions := &fakeChangelogActions{record: changelog.CandidateRecord{ID: "candidate-1", Status: changelog.StatusPending}}
	h := NewChangelogAutomationHandler(&fakeChangelogStore{}, actions, "secret")
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/changelog-candidates/candidate-1?action=publish&expires=1200&signature=sig", nil)
	req = withRouteParam(req, "id", "candidate-1")
	rr := httptest.NewRecorder()

	h.GetAdminCandidate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestConfirmAdminActionUsesActorAndReturnsAlreadyHandled(t *testing.T) {
	actions := &fakeChangelogActions{err: changelog.ErrCandidateAlreadyHandled}
	h := NewChangelogAutomationHandler(&fakeChangelogStore{}, actions, "secret")
	body := bytes.NewBufferString(`{"action":"publish","expires":1200,"signature":"sig"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/changelog-candidates/candidate-1/actions", body)
	req = withRouteParam(req, "id", "candidate-1")
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "admin_1"))
	rr := httptest.NewRecorder()

	h.ConfirmAdminAction(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func withRouteParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
