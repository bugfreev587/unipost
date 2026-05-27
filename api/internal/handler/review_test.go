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
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/reviewscript"
	"github.com/xiaoboyu/unipost-api/internal/reviewtemplate"
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

func TestReviewCreateDomainUsesConfiguredCnameTarget(t *testing.T) {
	store := &reviewStoreFake{}
	h := NewReviewHandler(store).
		WithTokenGenerator(fixedReviewTokenGenerator).
		WithReviewCnameTarget("Dev.UniPost.Dev.")
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
	if env.Data.CnameTarget != "dev.unipost.dev" {
		t.Fatalf("cname target = %q", env.Data.CnameTarget)
	}
	if store.createdDomain.CnameTarget != "dev.unipost.dev" {
		t.Fatalf("stored cname target = %q", store.createdDomain.CnameTarget)
	}
}

func TestReviewVerifyDomainMarksReadyWhenDNSMatches(t *testing.T) {
	store := &reviewStoreFake{
		domain: db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "dns_pending", VerificationToken: "unipost-review=hash", CnameTarget: "review.unipost.dev", TlsStatus: "pending"},
	}
	h := NewReviewHandler(store).WithDomainChecker(func(context.Context, db.ReviewDomain) reviewDomainCheckResult {
		return reviewDomainCheckResult{DNSReady: true, TLSIssued: true}
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/review/domains/rvdom_1/verify", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "rvdom_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.VerifyDomain(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.updatedDomain.Status != "ready" || store.updatedDomain.TlsStatus != "issued" {
		t.Fatalf("domain not marked ready: %+v", store.updatedDomain)
	}
}

func TestReviewVerifyDomainExplainsPendingDNS(t *testing.T) {
	store := &reviewStoreFake{
		domain: db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "dns_pending", VerificationToken: "unipost-review=hash", CnameTarget: "review.unipost.dev", TlsStatus: "pending"},
	}
	h := NewReviewHandler(store).WithDomainChecker(func(context.Context, db.ReviewDomain) reviewDomainCheckResult {
		return reviewDomainCheckResult{DNSReady: false, TLSIssued: false, Message: "CNAME or TXT record has not propagated yet"}
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/review/domains/rvdom_1/verify", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "rvdom_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.VerifyDomain(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CNAME or TXT") {
		t.Fatalf("missing remediation: %s", rec.Body.String())
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

func TestReviewTikTokScopeTemplatesReturnsSupportedScopes(t *testing.T) {
	h := NewReviewHandler(&reviewStoreFake{})
	req := httptest.NewRequest(http.MethodGet, "/v1/review/tiktok/scope-templates", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.GetTikTokScopeTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []reviewtemplate.TikTokScopeTemplate `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data) != 6 {
		t.Fatalf("expected 6 templates, got %+v", env.Data)
	}
}

func TestReviewTikTokDemoPlanReturnsOAuthPreludeAndSegments(t *testing.T) {
	h := NewReviewHandler(&reviewStoreFake{})
	req := httptest.NewRequest(http.MethodPost, "/v1/review/tiktok/demo-plan", strings.NewReader(`{
		"scopes":["user.info.basic","video.upload","video.publish"]
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.CreateTikTokDemoPlan(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data reviewtemplate.TikTokDemoPlan `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Platform != "tiktok" || !env.Data.OAuthPrelude.Required || len(env.Data.Segments) != 3 {
		t.Fatalf("unexpected plan: %+v", env.Data)
	}
}

func TestReviewCreateKitAcceptsAnalyticsScopesAndStoresGeneratedPlan(t *testing.T) {
	store := &reviewStoreFake{
		domain:             db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready"},
		platformCredential: db.PlatformCredential{WorkspaceID: "ws_1", Platform: "tiktok"},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/kits", strings.NewReader(`{
		"platform":"tiktok",
		"use_case":"analytics",
		"review_domain_id":"rvdom_1",
		"redirect_uri_attested":true,
		"brand_snapshot":{"display_name":"Acme"},
		"required_scopes":["user.info.profile","user.info.stats"]
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.CreateKit(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createdKit.UseCase != "analytics" {
		t.Fatalf("use case = %q", store.createdKit.UseCase)
	}
	if strings.Join(store.createdKit.RequiredScopes, ",") != "user.info.profile,user.info.stats" {
		t.Fatalf("stored scopes = %+v", store.createdKit.RequiredScopes)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(store.createdKit.BrandSnapshot, &snapshot); err != nil {
		t.Fatalf("brand snapshot json: %v", err)
	}
	if snapshot["scope_template_version"] != reviewtemplate.TikTokTemplateVersion {
		t.Fatalf("missing template version: %+v", snapshot)
	}
	if snapshot["review_plan"] == nil || snapshot["oauth_reset_required"] != true {
		t.Fatalf("missing review plan metadata: %+v", snapshot)
	}
}

func TestReviewGetStateRestoresReadyKitDomainAndLatestJob(t *testing.T) {
	store := &reviewStoreFake{
		reviewDomains: []db.ReviewDomain{
			{ID: "rvdom_pending", WorkspaceID: "ws_1", Domain: "pending.example.com", Status: "dns_pending"},
			{ID: "rvdom_ready", WorkspaceID: "ws_1", Domain: "tiktok-review.tailtales.ai", Status: "ready", TlsStatus: "issued", CnameTarget: "dev.unipost.dev"},
		},
		reviewKits: []db.ReviewKit{
			{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", UseCase: "content_posting", Status: "ready", ReviewDomainID: "rvdom_ready", RequiredScopes: []string{"user.info.basic", "video.publish", "video.upload"}},
		},
		reviewJobsByKit: map[string][]db.ReviewJob{
			"rvkit_1": {
				{ID: "rvjob_latest", WorkspaceID: "ws_1", ReviewKitID: "rvkit_1", Platform: "tiktok", Status: "failed", AgentVersion: pgtype.Text{String: reviewAgentVersion, Valid: true}},
			},
		},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/review/state", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.GetState(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data reviewStateResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Domain == nil || env.Data.Domain.Domain != "tiktok-review.tailtales.ai" || env.Data.Domain.Status != "ready" {
		t.Fatalf("domain was not restored from ready kit: %+v", env.Data.Domain)
	}
	if env.Data.Kit == nil || env.Data.Kit.ID != "rvkit_1" {
		t.Fatalf("kit was not restored: %+v", env.Data.Kit)
	}
	if env.Data.Job == nil || env.Data.Job.ID != "rvjob_latest" {
		t.Fatalf("latest job was not restored: %+v", env.Data.Job)
	}
}

func TestReviewCreateJobIssuesTokensAndPinnedCommand(t *testing.T) {
	store := &reviewStoreFake{
		kit:    db.ReviewKit{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", Status: "ready", ReviewDomainID: "rvdom_1"},
		domain: db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready"},
		now:    time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
	}
	h := NewReviewHandler(store).
		WithTokenGenerator(fixedReviewTokenGenerator).
		WithNow(func() time.Time { return store.now }).
		WithAPIBaseURL("https://dev-api.example.com")
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
	if !strings.Contains(env.Data.AgentCommand, "npx --yes @unipost/review-agent@0.1.0 run --token revtok_fixed --session-token revsess_fixed") {
		t.Fatalf("command not version pinned with session token: %q", env.Data.AgentCommand)
	}
	if !strings.Contains(env.Data.AgentCommand, "--api-url https://dev-api.example.com") {
		t.Fatalf("command missing API base URL: %q", env.Data.AgentCommand)
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

func TestReviewJobScriptUsesStoredAnalyticsPlan(t *testing.T) {
	plan, err := reviewtemplate.BuildTikTokDemoPlan(reviewtemplate.TikTokDemoPlanInput{Scopes: []string{"user.info.profile", "user.info.stats"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	brandSnapshot, err := json.Marshal(map[string]any{"review_plan": plan})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store := &reviewStoreFake{
		job:     db.ReviewJob{ID: "rvjob_analytics", WorkspaceID: "ws_1", ReviewKitID: "rvkit_analytics", Platform: "tiktok", AgentVersion: pgtype.Text{String: reviewAgentVersion, Valid: true}},
		kit:     db.ReviewKit{ID: "rvkit_analytics", WorkspaceID: "ws_1", Platform: "tiktok", UseCase: "analytics", Status: "ready", ReviewDomainID: "rvdom_1", BrandSnapshot: brandSnapshot},
		domain:  db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready"},
		session: db.ReviewSession{ID: "rvsess_1", ReviewJobID: "rvjob_analytics", WorkspaceID: "ws_1", Platform: "tiktok", ReviewDomain: "review.example.com", ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 5, 26, 21, 0, 0, 0, time.UTC), Valid: true}},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/review/jobs/rvjob_analytics/script", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "rvjob_analytics")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.GetJobScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data reviewscript.Script `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.StartURL != "https://review.example.com/tiktok/analytics" {
		t.Fatalf("unexpected start URL: %s", env.Data.StartURL)
	}
	if len(env.Data.Segments) != 2 || env.Data.Segments[0].Key != "analytics_part_1" {
		t.Fatalf("missing analytics segment metadata: %+v", env.Data.Segments)
	}
	for _, step := range env.Data.Steps {
		if step.ID == "assert_video_list" {
			t.Fatalf("video.list step should not be present unless requested: %+v", env.Data.Steps)
		}
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

func TestReviewPublicSessionReturnsConnectedAccountAndCreatorInfo(t *testing.T) {
	encryptor := testReviewEncryptor(t)
	accessToken, err := encryptor.Encrypt("access_live")
	if err != nil {
		t.Fatalf("encrypt access: %v", err)
	}
	store := &reviewStoreFake{
		reviewSessionByHash: db.ReviewSession{ID: "rvsess_1", ReviewJobID: "rvjob_1", ReviewKitID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", ReviewDomain: "review.example.com", TokenHash: hashReviewToken("revsess_live"), ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 5, 26, 21, 0, 0, 0, time.UTC), Valid: true}},
		kit:                 db.ReviewKit{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", Status: "ready", ReviewDomainID: "rvdom_1", BrandSnapshot: []byte(`{"profile_id":"prof_1"}`)},
		domain:              db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready", TlsStatus: "issued"},
		profile:             db.Profile{ID: "prof_1", WorkspaceID: "ws_1", Name: "Acme"},
		socialAccounts: []db.SocialAccount{{
			ID:                "sa_review_1",
			ProfileID:         "prof_1",
			Platform:          "tiktok",
			AccessToken:       accessToken,
			ExternalAccountID: "open_123",
			ExternalUserID:    pgtype.Text{String: "app-review:rvjob_1", Valid: true},
			AccountName:       pgtype.Text{String: "Review Creator", Valid: true},
			Scope:             []string{"user.info.basic", "video.publish", "video.upload"},
		}},
	}
	adapter := &reviewTikTokAdapterFake{creatorInfo: &platform.TikTokCreatorInfo{CreatorNickname: "Review Creator", CreatorUsername: "reviewer", PrivacyLevelOptions: []string{"SELF_ONLY"}, MaxVideoPostDurationSec: 600}}
	h := NewReviewHandler(store).WithEncryptor(encryptor).WithTikTokAdapter(adapter)
	req := httptest.NewRequest(http.MethodGet, "/v1/review/session", nil)
	req.AddCookie(&http.Cookie{Name: reviewSessionCookieName, Value: "revsess_live"})
	rec := httptest.NewRecorder()

	h.GetPublicReviewSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data reviewPublicSessionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Data.Connected || env.Data.ConnectAuthorizeURL != "" {
		t.Fatalf("expected connected session without authorize url: %+v", env.Data)
	}
	if env.Data.Account == nil || env.Data.Account.ID != "sa_review_1" {
		t.Fatalf("missing review account: %+v", env.Data.Account)
	}
	if env.Data.CreatorInfo == nil || env.Data.CreatorInfo.CreatorNickname != "Review Creator" {
		t.Fatalf("missing creator_info: %+v", env.Data.CreatorInfo)
	}
	if env.Data.TestVideoURL == "" || env.Data.DefaultCaption == "" {
		t.Fatalf("missing review publish defaults: %+v", env.Data)
	}
	if adapter.creatorAccessToken != "access_live" {
		t.Fatalf("creator_info used access token %q", adapter.creatorAccessToken)
	}
	if store.createdConnectSession.ProfileID != "" {
		t.Fatalf("connected session should not create a new connect session: %+v", store.createdConnectSession)
	}
}

func TestReviewPublishTikTokPostUsesAdapterAndRecordsEvent(t *testing.T) {
	encryptor := testReviewEncryptor(t)
	accessToken, err := encryptor.Encrypt("access_live")
	if err != nil {
		t.Fatalf("encrypt access: %v", err)
	}
	store := &reviewStoreFake{
		reviewSessionByHash: db.ReviewSession{ID: "rvsess_1", ReviewJobID: "rvjob_1", ReviewKitID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", ReviewDomain: "review.example.com", TokenHash: hashReviewToken("revsess_live"), ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 5, 26, 21, 0, 0, 0, time.UTC), Valid: true}},
		kit:                 db.ReviewKit{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", Status: "ready", ReviewDomainID: "rvdom_1", BrandSnapshot: []byte(`{"profile_id":"prof_1"}`)},
		profile:             db.Profile{ID: "prof_1", WorkspaceID: "ws_1", Name: "Acme"},
		socialAccounts: []db.SocialAccount{{
			ID:                "sa_review_1",
			ProfileID:         "prof_1",
			Platform:          "tiktok",
			AccessToken:       accessToken,
			ExternalAccountID: "open_123",
			ExternalUserID:    pgtype.Text{String: "app-review:rvjob_1", Valid: true},
			AccountName:       pgtype.Text{String: "Review Creator", Valid: true},
		}},
	}
	adapter := &reviewTikTokAdapterFake{postResult: &platform.PostResult{ExternalID: "publish_123", Status: "processing"}}
	h := NewReviewHandler(store).WithEncryptor(encryptor).WithTikTokAdapter(adapter).WithTikTokTestVideoURL("https://review.example.com/test-video.mp4")
	req := httptest.NewRequest(http.MethodPost, "/v1/review/session/tiktok/publish", strings.NewReader(`{"privacy_level":"SELF_ONLY"}`))
	req.AddCookie(&http.Cookie{Name: reviewSessionCookieName, Value: "revsess_live"})
	rec := httptest.NewRecorder()

	h.PublishReviewTikTokPost(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if adapter.postAccessToken != "access_live" || adapter.postText != reviewDefaultCaption {
		t.Fatalf("unexpected adapter call token=%q text=%q", adapter.postAccessToken, adapter.postText)
	}
	if len(adapter.postMedia) != 1 || adapter.postMedia[0].URL != "https://review.example.com/test-video.mp4" || adapter.postMedia[0].Kind != platform.MediaKindVideo {
		t.Fatalf("unexpected media: %+v", adapter.postMedia)
	}
	if adapter.postOpts["privacy_level"] != "SELF_ONLY" || adapter.postOpts["disable_comment"] != true {
		t.Fatalf("unexpected opts: %+v", adapter.postOpts)
	}
	if store.createdEvent.EventType != "review_publish_completed" || !strings.Contains(string(store.createdEvent.Metadata), "publish_123") {
		t.Fatalf("publish event not recorded: %+v metadata=%s", store.createdEvent, string(store.createdEvent.Metadata))
	}
	var env struct {
		Data reviewTikTokPublishResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.ExternalID != "publish_123" || env.Data.Status != "processing" {
		t.Fatalf("unexpected publish response: %+v", env.Data)
	}
}

func TestReviewPublicSessionUsesCookieAndCreatesTikTokConnectSession(t *testing.T) {
	store := &reviewStoreFake{
		reviewSessionByHash: db.ReviewSession{ID: "rvsess_1", ReviewJobID: "rvjob_1", ReviewKitID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", ReviewDomain: "review.example.com", TokenHash: hashReviewToken("revsess_live"), ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 5, 26, 21, 0, 0, 0, time.UTC), Valid: true}},
		kit:                 db.ReviewKit{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", Status: "ready", ReviewDomainID: "rvdom_1", BrandSnapshot: []byte(`{"profile_id":"prof_1"}`)},
		domain:              db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready", TlsStatus: "issued"},
		profile:             db.Profile{ID: "prof_1", WorkspaceID: "ws_1", Name: "Acme"},
	}
	h := NewReviewHandler(store).WithTokenGenerator(fixedReviewTokenGenerator).WithAPIBaseURL("https://api.example.com")
	req := httptest.NewRequest(http.MethodGet, "/v1/review/session", nil)
	req.AddCookie(&http.Cookie{Name: reviewSessionCookieName, Value: "revsess_live"})
	rec := httptest.NewRecorder()

	h.GetPublicReviewSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createdConnectSession.ProfileID != "prof_1" || store.createdConnectSession.Platform != "tiktok" {
		t.Fatalf("connect session not created: %+v", store.createdConnectSession)
	}
	var env struct {
		Data reviewPublicSessionResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(env.Data.ConnectAuthorizeURL, "https://api.example.com/v1/public/connect/sessions/csess_1/authorize?state=rvstate_fixed") {
		t.Fatalf("missing authorize url: %+v", env.Data)
	}
}

func TestReviewPublicSessionUsesAnalyticsReturnURLForAnalyticsKit(t *testing.T) {
	store := &reviewStoreFake{
		reviewSessionByHash: db.ReviewSession{ID: "rvsess_1", ReviewJobID: "rvjob_1", ReviewKitID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", ReviewDomain: "review.example.com", TokenHash: hashReviewToken("revsess_live"), ExpiresAt: pgtype.Timestamptz{Time: time.Date(2026, 5, 26, 21, 0, 0, 0, time.UTC), Valid: true}},
		kit:                 db.ReviewKit{ID: "rvkit_1", WorkspaceID: "ws_1", Platform: "tiktok", UseCase: "analytics", Status: "ready", ReviewDomainID: "rvdom_1", BrandSnapshot: []byte(`{"profile_id":"prof_1"}`)},
		domain:              db.ReviewDomain{ID: "rvdom_1", WorkspaceID: "ws_1", Domain: "review.example.com", Status: "ready", TlsStatus: "issued"},
		profile:             db.Profile{ID: "prof_1", WorkspaceID: "ws_1", Name: "Acme"},
	}
	h := NewReviewHandler(store).WithTokenGenerator(fixedReviewTokenGenerator).WithAPIBaseURL("https://api.example.com")
	req := httptest.NewRequest(http.MethodGet, "/v1/review/session", nil)
	req.AddCookie(&http.Cookie{Name: reviewSessionCookieName, Value: "revsess_live"})
	rec := httptest.NewRecorder()

	h.GetPublicReviewSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createdConnectSession.ReturnUrl.String != "https://review.example.com/tiktok/analytics?connect_status=success" {
		t.Fatalf("return URL = %q", store.createdConnectSession.ReturnUrl.String)
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

func TestReviewAgentManualPauseCompletedMarksJobRunning(t *testing.T) {
	store := &reviewStoreFake{
		agentToken: db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", TokenHash: hashReviewToken("revtok_live")},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/agent/events", strings.NewReader(`{
		"event_type":"manual_pause_completed",
		"message":"TikTok OAuth returned",
		"metadata":{"step_id":"wait_for_oauth"}
	}`))
	req.Header.Set("Authorization", "Bearer revtok_live")
	rec := httptest.NewRecorder()

	h.RecordAgentEvent(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.markedRunningJobID != "rvjob_1" {
		t.Fatalf("job was not marked running after pause completion: %q", store.markedRunningJobID)
	}
}

func TestReviewAgentCompleteAndFailUseBearerToken(t *testing.T) {
	store := &reviewStoreFake{
		agentToken: db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", TokenHash: hashReviewToken("revtok_live")},
	}
	h := NewReviewHandler(store)
	completeReq := httptest.NewRequest(http.MethodPost, "/v1/review/agent/complete", strings.NewReader(`{
		"video_file_id":"review-artifacts/ws_1/rvjob_1/demo-video.webm",
		"artifacts":{"markers":[{"elapsed_ms":42,"label":"Open customer review domain"}]}
	}`))
	completeReq.Header.Set("Authorization", "Bearer revtok_live")
	completeRec := httptest.NewRecorder()

	h.CompleteAgentJob(completeRec, completeReq)

	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete status = %d, body = %s", completeRec.Code, completeRec.Body.String())
	}
	if store.completedJob.ID != "rvjob_1" || store.completedJob.VideoFileID.String != "review-artifacts/ws_1/rvjob_1/demo-video.webm" {
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

func TestReviewAgentCompleteRejectsForeignArtifactID(t *testing.T) {
	store := &reviewStoreFake{
		agentToken: db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", TokenHash: hashReviewToken("revtok_live")},
	}
	h := NewReviewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/agent/complete", strings.NewReader(`{
		"video_file_id":"review-artifacts/ws_other/rvjob_other/demo-video.webm",
		"artifacts":{}
	}`))
	req.Header.Set("Authorization", "Bearer revtok_live")
	rec := httptest.NewRecorder()

	h.CompleteAgentJob(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.completedJob.ID != "" {
		t.Fatalf("foreign artifact should not complete job: %+v", store.completedJob)
	}
}

func TestReviewAgentCreatesArtifactUploadURL(t *testing.T) {
	store := &reviewStoreFake{
		agentToken: db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", TokenHash: hashReviewToken("revtok_live")},
	}
	artifacts := &reviewArtifactStorageFake{putURL: "https://uploads.example.com/review-video"}
	h := NewReviewHandler(store).WithArtifactStorage(artifacts)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/agent/artifacts", strings.NewReader(`{
		"artifact_type":"demo_video",
		"content_type":"video/webm",
		"size_bytes":1234
	}`))
	req.Header.Set("Authorization", "Bearer revtok_live")
	rec := httptest.NewRecorder()

	h.CreateAgentArtifactUpload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if artifacts.putKey != "review-artifacts/ws_1/rvjob_1/demo-video.webm" || artifacts.putContentType != "video/webm" {
		t.Fatalf("unexpected presign args: key=%q content_type=%q", artifacts.putKey, artifacts.putContentType)
	}
	var env struct {
		Data reviewAgentArtifactUploadResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.FileID != "review-artifacts/ws_1/rvjob_1/demo-video.webm" || env.Data.UploadURL != artifacts.putURL {
		t.Fatalf("unexpected upload response: %+v", env.Data)
	}
}

func TestReviewAgentCreatesSegmentArtifactUploadURL(t *testing.T) {
	store := &reviewStoreFake{
		agentToken: db.ReviewAgentToken{ID: "rvatok_1", ReviewJobID: "rvjob_1", WorkspaceID: "ws_1", Platform: "tiktok", TokenHash: hashReviewToken("revtok_live")},
	}
	artifacts := &reviewArtifactStorageFake{putURL: "https://uploads.example.com/review-video-part-1"}
	h := NewReviewHandler(store).WithArtifactStorage(artifacts)
	req := httptest.NewRequest(http.MethodPost, "/v1/review/agent/artifacts", strings.NewReader(`{
		"artifact_type":"demo_video",
		"segment_key":"posting_part_1",
		"content_type":"video/mp4",
		"size_bytes":1234
	}`))
	req.Header.Set("Authorization", "Bearer revtok_live")
	rec := httptest.NewRecorder()

	h.CreateAgentArtifactUpload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if artifacts.putKey != "review-artifacts/ws_1/rvjob_1/demo-video-posting_part_1.mp4" {
		t.Fatalf("unexpected segment upload key: %q", artifacts.putKey)
	}
	var env struct {
		Data reviewAgentArtifactUploadResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.FileID != artifacts.putKey {
		t.Fatalf("file id = %q, want %q", env.Data.FileID, artifacts.putKey)
	}
}

func TestReviewGetJobReturnsVideoDownloadAndEvents(t *testing.T) {
	store := &reviewStoreFake{
		job: db.ReviewJob{
			ID: "rvjob_1", WorkspaceID: "ws_1", ReviewKitID: "rvkit_1", Platform: "tiktok", Status: "completed",
			VideoFileID:   pgtype.Text{String: "review-artifacts/ws_1/rvjob_1/demo-video.webm", Valid: true},
			ArtifactsJson: []byte(`{"markers":[{"label":"Publish test video","elapsed_ms":42000}]}`),
		},
		events: []db.ReviewJobEvent{{ReviewJobID: "rvjob_1", EventType: "recording_started", Message: "Recorder started", ElapsedMs: pgtype.Int8{Int64: 0, Valid: true}}},
	}
	artifacts := &reviewArtifactStorageFake{getURL: "https://downloads.example.com/review-video"}
	h := NewReviewHandler(store).WithArtifactStorage(artifacts)
	req := httptest.NewRequest(http.MethodGet, "/v1/review/jobs/rvjob_1", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "rvjob_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.GetJob(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if artifacts.getKey != "review-artifacts/ws_1/rvjob_1/demo-video.webm" {
		t.Fatalf("unexpected download key: %q", artifacts.getKey)
	}
	var env struct {
		Data reviewJobDetailResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.VideoDownloadURL != artifacts.getURL || len(env.Data.Events) != 1 {
		t.Fatalf("missing artifact detail: %+v", env.Data)
	}
	if env.Data.Artifacts["markers"] == nil {
		t.Fatalf("missing artifacts json: %+v", env.Data.Artifacts)
	}
}

func TestReviewGetJobReturnsSegmentVideoDownloads(t *testing.T) {
	store := &reviewStoreFake{
		job: db.ReviewJob{
			ID: "rvjob_1", WorkspaceID: "ws_1", ReviewKitID: "rvkit_1", Platform: "tiktok", Status: "completed",
			ArtifactsJson: []byte(`{
				"video_segments":[
					{"segment_key":"posting_part_1","filename":"tiktok-content-posting-part-1.mp4","file_id":"review-artifacts/ws_1/rvjob_1/demo-video-posting_part_1.mp4","format":"mp4","duration_sec":42,"size_bytes":42000000,"scopes":["user.info.basic","video.upload"]},
					{"segment_key":"posting_part_2","filename":"tiktok-content-posting-part-2.mp4","file_id":"review-artifacts/ws_1/rvjob_1/demo-video-posting_part_2.mp4","format":"mp4","duration_sec":39,"size_bytes":39000000,"scopes":["video.publish"]}
				]
			}`),
		},
	}
	artifacts := &reviewArtifactStorageFake{getURLs: map[string]string{
		"review-artifacts/ws_1/rvjob_1/demo-video-posting_part_1.mp4": "https://downloads.example.com/part-1",
		"review-artifacts/ws_1/rvjob_1/demo-video-posting_part_2.mp4": "https://downloads.example.com/part-2",
	}}
	h := NewReviewHandler(store).WithArtifactStorage(artifacts)
	req := httptest.NewRequest(http.MethodGet, "/v1/review/jobs/rvjob_1", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "rvjob_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.GetJob(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data reviewJobDetailResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.VideoArtifacts) != 2 {
		t.Fatalf("expected segment downloads, got %+v", env.Data.VideoArtifacts)
	}
	if env.Data.VideoArtifacts[0].SegmentKey != "posting_part_1" || env.Data.VideoArtifacts[0].DownloadURL != "https://downloads.example.com/part-1" {
		t.Fatalf("unexpected first segment: %+v", env.Data.VideoArtifacts[0])
	}
	if env.Data.VideoArtifacts[1].Filename != "tiktok-content-posting-part-2.mp4" || env.Data.VideoArtifacts[1].SizeBytes != 39000000 {
		t.Fatalf("unexpected second segment: %+v", env.Data.VideoArtifacts[1])
	}
}

type reviewStoreFake struct {
	domain               db.ReviewDomain
	kit                  db.ReviewKit
	job                  db.ReviewJob
	session              db.ReviewSession
	platformCredential   db.PlatformCredential
	agentToken           db.ReviewAgentToken
	reviewSessionByHash  db.ReviewSession
	profile              db.Profile
	events               []db.ReviewJobEvent
	reviewDomains        []db.ReviewDomain
	reviewKits           []db.ReviewKit
	reviewJobsByKit      map[string][]db.ReviewJob
	socialAccounts       []db.SocialAccount
	listAccountsParams   db.ListSocialAccountsByProfileFilteredParams
	agentTokenHashLookup string
	credentialErr        error
	now                  time.Time

	createdDomain         db.CreateReviewDomainParams
	updatedDomain         db.UpdateReviewDomainVerificationParams
	createdKit            db.CreateReviewKitParams
	createdJob            db.CreateReviewJobParams
	createdSession        db.CreateReviewSessionParams
	createdAgentToken     db.CreateReviewAgentTokenParams
	createdConnectSession db.CreateConnectSessionParams
	createdEvent          db.CreateReviewJobEventParams
	updatedTokens         db.UpdateSocialAccountTokensParams
	markedRunningJobID    string
	markedWaitingJobID    string
	completedJob          db.CompleteReviewJobParams
	failedJob             db.FailReviewJobParams
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
func (f *reviewStoreFake) ListReviewDomainsByWorkspace(_ context.Context, workspaceID string) ([]db.ReviewDomain, error) {
	out := make([]db.ReviewDomain, 0, len(f.reviewDomains))
	for _, domain := range f.reviewDomains {
		if domain.WorkspaceID == workspaceID {
			out = append(out, domain)
		}
	}
	return out, nil
}
func (f *reviewStoreFake) UpdateReviewDomainVerification(_ context.Context, arg db.UpdateReviewDomainVerificationParams) (db.ReviewDomain, error) {
	f.updatedDomain = arg
	return db.ReviewDomain{ID: arg.ID, WorkspaceID: arg.WorkspaceID, Domain: f.domain.Domain, Status: arg.Status, VerificationToken: f.domain.VerificationToken, CnameTarget: f.domain.CnameTarget, TlsStatus: arg.TlsStatus}, nil
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
func (f *reviewStoreFake) ListReviewKitsByWorkspace(_ context.Context, workspaceID string) ([]db.ReviewKit, error) {
	out := make([]db.ReviewKit, 0, len(f.reviewKits))
	for _, kit := range f.reviewKits {
		if kit.WorkspaceID == workspaceID {
			out = append(out, kit)
		}
	}
	return out, nil
}
func (f *reviewStoreFake) GetProfile(_ context.Context, id string) (db.Profile, error) {
	if f.profile.ID == "" {
		return db.Profile{}, pgx.ErrNoRows
	}
	return f.profile, nil
}
func (f *reviewStoreFake) CreateConnectSession(_ context.Context, arg db.CreateConnectSessionParams) (db.ConnectSession, error) {
	f.createdConnectSession = arg
	return db.ConnectSession{ID: "csess_1", ProfileID: arg.ProfileID, Platform: arg.Platform, ExternalUserID: arg.ExternalUserID, ReturnUrl: arg.ReturnUrl, OauthState: arg.OauthState, ExpiresAt: arg.ExpiresAt}, nil
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
func (f *reviewStoreFake) ListReviewJobsByKit(_ context.Context, arg db.ListReviewJobsByKitParams) ([]db.ReviewJob, error) {
	jobs := f.reviewJobsByKit[arg.ReviewKitID]
	out := make([]db.ReviewJob, 0, len(jobs))
	for _, job := range jobs {
		if job.WorkspaceID == arg.WorkspaceID {
			out = append(out, job)
		}
	}
	return out, nil
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
func (f *reviewStoreFake) GetReviewSessionByTokenHash(_ context.Context, hash string) (db.ReviewSession, error) {
	if f.reviewSessionByHash.ID == "" {
		return db.ReviewSession{}, pgx.ErrNoRows
	}
	return f.reviewSessionByHash, nil
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
func (f *reviewStoreFake) ListReviewJobEvents(_ context.Context, _ db.ListReviewJobEventsParams) ([]db.ReviewJobEvent, error) {
	return f.events, nil
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
func (f *reviewStoreFake) ListSocialAccountsByProfileFiltered(_ context.Context, arg db.ListSocialAccountsByProfileFilteredParams) ([]db.SocialAccount, error) {
	f.listAccountsParams = arg
	out := make([]db.SocialAccount, 0, len(f.socialAccounts))
	for _, account := range f.socialAccounts {
		if account.ProfileID != arg.ProfileID {
			continue
		}
		if arg.ExternalUserID.Valid && (!account.ExternalUserID.Valid || account.ExternalUserID.String != arg.ExternalUserID.String) {
			continue
		}
		if arg.Platform.Valid && account.Platform != arg.Platform.String {
			continue
		}
		out = append(out, account)
	}
	return out, nil
}

func (f *reviewStoreFake) UpdateSocialAccountTokens(_ context.Context, arg db.UpdateSocialAccountTokensParams) error {
	f.updatedTokens = arg
	return nil
}

func fixedReviewTokenGenerator(prefix string) (string, string, error) {
	return prefix + "fixed", "hash:" + prefix + "fixed", nil
}

type reviewArtifactStorageFake struct {
	putKey         string
	putContentType string
	putURL         string
	getKey         string
	getKeys        []string
	getURL         string
	getURLs        map[string]string
}

func (f *reviewArtifactStorageFake) PresignPut(_ context.Context, key string, contentType string, _ time.Duration) (string, error) {
	f.putKey = key
	f.putContentType = contentType
	return f.putURL, nil
}

func (f *reviewArtifactStorageFake) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	f.getKey = key
	f.getKeys = append(f.getKeys, key)
	if f.getURLs != nil {
		if url := f.getURLs[key]; url != "" {
			return url, nil
		}
	}
	return f.getURL, nil
}

func testReviewEncryptor(t *testing.T) *appcrypto.AESEncryptor {
	t.Helper()
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}
	return encryptor
}

type reviewTikTokAdapterFake struct {
	creatorInfo        *platform.TikTokCreatorInfo
	creatorErr         error
	creatorAccessToken string
	postResult         *platform.PostResult
	postErr            error
	postAccessToken    string
	postText           string
	postMedia          []platform.MediaItem
	postOpts           map[string]any
	refreshAccess      string
	refreshRefresh     string
	refreshExpiresAt   time.Time
	refreshErr         error
}

func (f *reviewTikTokAdapterFake) Post(_ context.Context, accessToken string, text string, media []platform.MediaItem, opts map[string]any) (*platform.PostResult, error) {
	f.postAccessToken = accessToken
	f.postText = text
	f.postMedia = append([]platform.MediaItem(nil), media...)
	f.postOpts = opts
	if f.postErr != nil {
		return nil, f.postErr
	}
	if f.postResult != nil {
		return f.postResult, nil
	}
	return &platform.PostResult{ExternalID: "publish_default"}, nil
}

func (f *reviewTikTokAdapterFake) RefreshToken(_ context.Context, _ string) (string, string, time.Time, error) {
	if f.refreshErr != nil {
		return "", "", time.Time{}, f.refreshErr
	}
	return f.refreshAccess, f.refreshRefresh, f.refreshExpiresAt, nil
}

func (f *reviewTikTokAdapterFake) FetchCreatorInfo(_ context.Context, accessToken string) (*platform.TikTokCreatorInfo, error) {
	f.creatorAccessToken = accessToken
	if f.creatorErr != nil {
		return nil, f.creatorErr
	}
	if f.creatorInfo != nil {
		return f.creatorInfo, nil
	}
	return &platform.TikTokCreatorInfo{CreatorNickname: "Review Creator", PrivacyLevelOptions: []string{"SELF_ONLY"}}, nil
}
