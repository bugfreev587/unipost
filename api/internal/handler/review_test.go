package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestReviewCreateDomainReturnsDNSRecords(t *testing.T) {
	store := &reviewStoreFake{}
	h := NewReviewHandler(store).WithTokenGenerator(fixedReviewTokenGenerator)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/domains", strings.NewReader(`{"domain":"review.example.com","provider":"manual"}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.CreateDomain(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data reviewDomainResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Domain != "review.example.com" || env.Data.CnameTarget != "review.unipost.dev" {
		t.Fatalf("unexpected domain response: %+v", env.Data)
	}
	if len(env.Data.DNSRecords) != 2 {
		t.Fatalf("expected 2 dns records, got %+v", env.Data.DNSRecords)
	}
	if store.createdDomain.VerificationToken == "" || !strings.HasPrefix(store.createdDomain.VerificationToken, "unipost-review=") {
		t.Fatalf("verification token not generated: %+v", store.createdDomain)
	}
}

func TestReviewCreateKitRequiresReadyDomainCredentialsScopesAndRedirectAttestation(t *testing.T) {
	store := &reviewStoreFake{
		domain:             db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready"},
		platformCredential: db.PlatformCredential{WorkspaceID: "ws_1", Platform: "tiktok"},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/kits", strings.NewReader(`{
		"platform":"tiktok",
		"use_case":"content_posting",
		"review_domain_id":"rvdom_1",
		"redirect_uri_attested":true,
		"brand_snapshot":{"display_name":"Acme"},
		"required_scopes":["video.upload","user.info.basic","video.publish"]
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.CreateKit(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createdKit.Platform != "tiktok" || store.createdKit.Status != "ready" {
		t.Fatalf("unexpected created kit: %+v", store.createdKit)
	}
}

func TestReviewCreateKitRejectsMissingRedirectAttestation(t *testing.T) {
	store := &reviewStoreFake{
		domain:             db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready"},
		platformCredential: db.PlatformCredential{WorkspaceID: "ws_1", Platform: "tiktok"},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/kits", strings.NewReader(`{
		"platform":"tiktok",
		"use_case":"content_posting",
		"review_domain_id":"rvdom_1",
		"required_scopes":["user.info.basic","video.publish","video.upload"]
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.CreateKit(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "redirect URI") {
		t.Fatalf("expected redirect URI remediation, got %s", rec.Body.String())
	}
}

func TestReviewCreateJobIssuesTokensAndPinnedCommand(t *testing.T) {
	store := &reviewStoreFake{
		kit:    db.ReviewKit{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", Status: "ready", ReviewDomainID: "rvdom_1"},
		domain: db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready"},
		now:    time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
	}
	h := NewReviewHandler(store).WithTokenGenerator(fixedReviewTokenGenerator).WithNow(func() time.Time { return store.now })
	req := httptest.NewRequest(http.MethodPost, "/v1/review/jobs", strings.NewReader(`{"review_kit_id":"rvkit_1"}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.CreateJob(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data reviewJobResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.AgentVersion != reviewAgentVersion {
		t.Fatalf("agent version = %q", env.Data.AgentVersion)
	}
	if !strings.Contains(env.Data.AgentCommand, "npx --yes @unipost/review-agent@0.1.0 run --token revtok_fixed") {
		t.Fatalf("command not version pinned: %q", env.Data.AgentCommand)
	}
	if store.createdAgentToken.TokenHash != "hash:revtok_fixed" || store.createdSession.TokenHash != "hash:revsess_fixed" {
		t.Fatalf("tokens not hashed into store: agent=%+v session=%+v", store.createdAgentToken, store.createdSession)
	}
}

func TestReviewJobScriptUsesClosedActions(t *testing.T) {
	store := &reviewStoreFake{
		job:     db.ReviewJob{ID: "rvjob_1", WorkspaceID: "ws_1", ReviewKitID: "rvkit_1", Platform: "tiktok", AgentVersion: pgtype.Text{String: reviewAgentVersion, Valid: true}},
		kit:     db.ReviewKit{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", Status: "ready", ReviewDomainID: "rvdom_1"},
		domain:  db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready"},
		session: db.ReviewSession{ID: "rvsess_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", ReviewDomain: "review.example.com", ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 5, 26, 21, 0, 0, 0, time.UTC), Valid: true}},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/review/jobs/rvjob_1/script", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "rvjob_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.GetJobScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	steps, ok := env.Data["steps"].([]any)
	if !ok || len(steps) == 0 {
		t.Fatalf("missing steps: %+v", env.Data)
	}
	for _, item := range steps {
		step := item.(map[string]any)
		action := step["action"].(string)
		if action == "eval" || action == "js" {
			t.Fatalf("script leaked unsafe action: %+v", step)
		}
	}
}

func TestReviewAgentScriptUsesBearerToken(t *testing.T) {
	store := &reviewStoreFake{
		agentToken: db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", TokenHash: hashReviewToken("revtok_live")},
		job:        db.ReviewJob{ID: "rvjob_1", WorkspaceID: "ws_1", ReviewKitID: "rvkit_1", Platform: "tiktok", AgentVersion: pgtype.Text{String: reviewAgentVersion, Valid: true}},
		kit:        db.ReviewKit{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", Status: "ready", ReviewDomainID: "rvdom_1"},
		domain:     db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready", TlsStatus: "issued"},
		session:    db.ReviewSession{ID: "rvsess_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", ReviewDomain: "review.example.com", ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 5, 26, 21, 0, 0, 0, time.UTC), Valid: true}},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/review/agent/script", nil)
	req.Header.Set("Authorization", "Bearer revtok_live")
	rec := httptest.NewRecorder()

	h.GetAgentJobScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.agentTokenHashLookup != hashReviewToken("revtok_live") {
		t.Fatalf("token hash lookup = %q", store.agentTokenHashLookup)
	}
}

func TestReviewAgentScriptRejectsMissingBearerToken(t *testing.T) {
	h := NewReviewHandler(&reviewStoreFake{})
	req := httptest.NewRequest(http.MethodGet, "/v1/review/agent/script", nil)
	rec := httptest.NewRecorder()

	h.GetAgentJobScript(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestReviewAgentEventRecordsEventAndMarksJobRunning(t *testing.T) {
	store := &reviewStoreFake{
		agentToken: db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", TokenHash: hashReviewToken("revtok_live")},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/agent/events", strings.NewReader(`{
		"event_type":"recording_started",
		"message":"Recorder started",
		"metadata":{"step_id":"open_review_app"},
		"elapsed_ms":42
	}`))
	req.Header.Set("Authorization", "Bearer revtok_live")
	rec := httptest.NewRecorder()

	h.RecordAgentEvent(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createdEvent.ReviewJobID != "rvjob_1" || store.createdEvent.EventType != "recording_started" {
		t.Fatalf("event not persisted correctly: %+v", store.createdEvent)
	}
	if store.markedRunningJobID != "rvjob_1" {
		t.Fatalf("job was not marked running: %q", store.markedRunningJobID)
	}
	if !strings.Contains(string(store.createdEvent.Metadata), `"step_id":"open_review_app"`) {
		t.Fatalf("metadata not encoded: %s", string(store.createdEvent.Metadata))
	}
}

func TestReviewAgentCompleteAndFailUseBearerToken(t *testing.T) {
	store := &reviewStoreFake{
		agentToken: db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", TokenHash: hashReviewToken("revtok_live")},
	}
	h := NewReviewHandler(store)
	completeReq := httptest.NewRequest(http.MethodPost, "/v1/review/agent/complete", strings.NewReader(`{
		"video_file_id":"file_123",
		"artifacts":{"markers":[{"elapsed_ms":42,"label":"Open customer review domain"}]}
	}`))
	completeReq.Header.Set("Authorization", "Bearer revtok_live")
	completeRec := httptest.NewRecorder()

	h.CompleteAgentJob(completeRec, completeReq)

	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete status = %d, body = %s", completeRec.Code, completeRec.Body.String())
	}
	if store.completedJob.ID != "rvjob_1" || store.completedJob.VideoFileID.String != "file_123" {
		t.Fatalf("job was not completed: %+v", store.completedJob)
	}

	failReq := httptest.NewRequest(http.MethodPost, "/v1/review/agent/fail", strings.NewReader(`{
		"failure_reason":"redirect URI mismatch",
		"artifacts":{"last_step":"wait_for_oauth"}
	}`))
	failReq.Header.Set("Authorization", "Bearer revtok_live")
	failRec := httptest.NewRecorder()

	h.FailAgentJob(failRec, failReq)

	if failRec.Code != http.StatusOK {
		t.Fatalf("fail status = %d, body = %s", failRec.Code, failRec.Body.String())
	}
	if store.failedJob.ID != "rvjob_1" || store.failedJob.FailureReason.String != "redirect URI mismatch" {
		t.Fatalf("job was not failed: %+v", store.failedJob)
	}
}

type reviewStoreFake struct {
	domain               db.ReviewDomain
	kit                  db.ReviewKit
	job                  db.ReviewJob
	session              db.ReviewSession
	platformCredential   db.PlatformCredential
	agentToken           db.ReviewAgentToken
	agentTokenHashLookup string
	credentialErr        error
	now                  time.Time

	createdDomain      db.CreateReviewDomainParams
	createdKit         db.CreateReviewKitParams
	createdJob         db.CreateReviewJobParams
	createdSession     db.CreateReviewSessionParams
	createdAgentToken  db.CreateReviewAgentTokenParams
	createdEvent       db.CreateReviewJobEventParams
	markedRunningJobID string
	markedWaitingJobID string
	completedJob       db.CompleteReviewJobParams
	failedJob          db.FailReviewJobParams
}

func (f *reviewStoreFake) CreateReviewDomain(_ context.Context, arg db.CreateReviewDomainParams) (db.ReviewDomain, error) {
	f.createdDomain = arg
	return db.ReviewDomain{ID: "rvdom_1", WorkspaceID: arg.WorkspaceID, Domain: arg.Domain, Provider: arg.Provider, Status: arg.Status, VerificationToken: arg.VerificationToken, CnameTarget: arg.CnameTarget, TlsStatus: arg.TlsStatus}, nil
}
func (f *reviewStoreFake) GetReviewDomain(_ context.Context, arg db.GetReviewDomainParams) (db.ReviewDomain, error) {
	if f.domain.ID == "" {
		return db.ReviewDomain{}, pgx.ErrNoRows
	}
	return f.domain, nil
}
func (f *reviewStoreFake) GetPlatformCredential(_ context.Context, arg db.GetPlatformCredentialParams) (db.PlatformCredential, error) {
	if f.credentialErr != nil {
		return db.PlatformCredential{}, f.credentialErr
	}
	return f.platformCredential, nil
}
func (f *reviewStoreFake) CreateReviewKit(_ context.Context, arg db.CreateReviewKitParams) (db.ReviewKit, error) {
	f.createdKit = arg
	return db.ReviewKit{ID: "rvkit_1", WorkspaceID: arg.WorkspaceID, Platform: arg.Platform, UseCase: arg.UseCase, ReviewDomainID: arg.ReviewDomainID, BrandSnapshot: arg.BrandSnapshot, RequiredScopes: arg.RequiredScopes, Status: arg.Status}, nil
}
func (f *reviewStoreFake) GetReviewKit(_ context.Context, arg db.GetReviewKitParams) (db.ReviewKit, error) {
	if f.kit.ID == "" {
		return db.ReviewKit{}, pgx.ErrNoRows
	}
	return f.kit, nil
}
func (f *reviewStoreFake) CreateReviewJob(_ context.Context, arg db.CreateReviewJobParams) (db.ReviewJob, error) {
	f.createdJob = arg
	return db.ReviewJob{ID: "rvjob_1", ReviewKitID: arg.ReviewKitID, WorkspaceID: arg.WorkspaceID, Platform: arg.Platform, Status: arg.Status, AgentVersion: arg.AgentVersion}, nil
}
func (f *reviewStoreFake) GetReviewJob(_ context.Context, arg db.GetReviewJobParams) (db.ReviewJob, error) {
	if f.job.ID == "" {
		return db.ReviewJob{}, pgx.ErrNoRows
	}
	return f.job, nil
}
func (f *reviewStoreFake) CreateReviewSession(_ context.Context, arg db.CreateReviewSessionParams) (db.ReviewSession, error) {
	f.createdSession = arg
	return db.ReviewSession{ID: "rvsess_1", ReviewJobID: arg.ReviewJobID, ReviewKitID: arg.ReviewKitID, WorkspaceID: arg.WorkspaceID, Platform: arg.Platform, ReviewDomain: arg.ReviewDomain, TokenHash: arg.TokenHash, ExpiresAt: arg.ExpiresAt}, nil
}
func (f *reviewStoreFake) GetActiveReviewSessionForJob(_ context.Context, arg db.GetActiveReviewSessionForJobParams) (db.ReviewSession, error) {
	if f.session.ID == "" {
		return db.ReviewSession{}, pgx.ErrNoRows
	}
	return f.session, nil
}
func (f *reviewStoreFake) AttachReviewSessionToJob(_ context.Context, arg db.AttachReviewSessionToJobParams) (db.ReviewJob, error) {
	return db.ReviewJob{ID: arg.ID, WorkspaceID: arg.WorkspaceID, ReviewSessionTokenID: arg.ReviewSessionTokenID}, nil
}
func (f *reviewStoreFake) CreateReviewAgentToken(_ context.Context, arg db.CreateReviewAgentTokenParams) (db.ReviewAgentToken, error) {
	f.createdAgentToken = arg
	return db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: arg.ReviewJobID, WorkspaceID: arg.WorkspaceID, Platform: arg.Platform, TokenHash: arg.TokenHash, ExpiresAt: arg.ExpiresAt}, nil
}
func (f *reviewStoreFake) GetReviewAgentTokenByHash(_ context.Context, hash string) (db.ReviewAgentToken, error) {
	f.agentTokenHashLookup = hash
	if f.agentToken.ID == "" {
		return db.ReviewAgentToken{}, pgx.ErrNoRows
	}
	return f.agentToken, nil
}
func (f *reviewStoreFake) CreateReviewJobEvent(_ context.Context, arg db.CreateReviewJobEventParams) (db.ReviewJobEvent, error) {
	f.createdEvent = arg
	return db.ReviewJobEvent{ID: 1, ReviewJobID: arg.ReviewJobID, EventType: arg.EventType, Message: arg.Message, Metadata: arg.Metadata, ElapsedMs: arg.ElapsedMs}, nil
}
func (f *reviewStoreFake) MarkReviewJobRunning(_ context.Context, arg db.MarkReviewJobRunningParams) (db.ReviewJob, error) {
	f.markedRunningJobID = arg.ID
	return db.ReviewJob{ID: arg.ID, WorkspaceID: arg.WorkspaceID, Status: "running"}, nil
}
func (f *reviewStoreFake) MarkReviewJobWaitingForUser(_ context.Context, arg db.MarkReviewJobWaitingForUserParams) (db.ReviewJob, error) {
	f.markedWaitingJobID = arg.ID
	return db.ReviewJob{ID: arg.ID, WorkspaceID: arg.WorkspaceID, Status: "waiting_for_user"}, nil
}
func (f *reviewStoreFake) CompleteReviewJob(_ context.Context, arg db.CompleteReviewJobParams) (db.ReviewJob, error) {
	f.completedJob = arg
	return db.ReviewJob{ID: arg.ID, WorkspaceID: arg.WorkspaceID, Status: "completed"}, nil
}
func (f *reviewStoreFake) FailReviewJob(_ context.Context, arg db.FailReviewJobParams) (db.ReviewJob, error) {
	f.failedJob = arg
	return db.ReviewJob{ID: arg.ID, WorkspaceID: arg.WorkspaceID, Status: "failed"}, nil
}

func fixedReviewTokenGenerator(prefix string) (string, string, error) {
	return prefix + "fixed", "hash:" + prefix + "fixed", nil
}
