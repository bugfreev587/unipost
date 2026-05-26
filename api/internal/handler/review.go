package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/reviewscript"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
)

const (
	reviewAgentVersion        = "0.1.0"
	reviewCnameTarget         = "review.unipost.dev"
	reviewSessionCookieName   = "__unipost_review_session"
	reviewTokenTTL            = 30 * time.Minute
	reviewDefaultUseCase      = "content_posting"
	reviewDefaultPlatform     = "tiktok"
	reviewDomainStatusReady   = "ready"
	reviewJobStatusQueued     = "queued"
	reviewKitStatusReady      = "ready"
	reviewDomainStatusPending = "dns_pending"
	reviewTLSStatusIssued     = "issued"
)

var requiredTikTokReviewScopes = []string{"user.info.basic", "video.publish", "video.upload"}

type reviewTokenGenerator func(prefix string) (raw string, hash string, err error)

type ReviewHandler struct {
	store          reviewStore
	now            func() time.Time
	tokenGenerator reviewTokenGenerator
}

type reviewStore interface {
	CreateReviewDomain(context.Context, db.CreateReviewDomainParams) (db.ReviewDomain, error)
	GetReviewDomain(context.Context, db.GetReviewDomainParams) (db.ReviewDomain, error)
	GetPlatformCredential(context.Context, db.GetPlatformCredentialParams) (db.PlatformCredential, error)
	CreateReviewKit(context.Context, db.CreateReviewKitParams) (db.ReviewKit, error)
	GetReviewKit(context.Context, db.GetReviewKitParams) (db.ReviewKit, error)
	CreateReviewJob(context.Context, db.CreateReviewJobParams) (db.ReviewJob, error)
	GetReviewJob(context.Context, db.GetReviewJobParams) (db.ReviewJob, error)
	CreateReviewSession(context.Context, db.CreateReviewSessionParams) (db.ReviewSession, error)
	GetActiveReviewSessionForJob(context.Context, db.GetActiveReviewSessionForJobParams) (db.ReviewSession, error)
	AttachReviewSessionToJob(context.Context, db.AttachReviewSessionToJobParams) (db.ReviewJob, error)
	CreateReviewAgentToken(context.Context, db.CreateReviewAgentTokenParams) (db.ReviewAgentToken, error)
	GetReviewAgentTokenByHash(context.Context, string) (db.ReviewAgentToken, error)
	CreateReviewJobEvent(context.Context, db.CreateReviewJobEventParams) (db.ReviewJobEvent, error)
	MarkReviewJobRunning(context.Context, db.MarkReviewJobRunningParams) (db.ReviewJob, error)
	MarkReviewJobWaitingForUser(context.Context, db.MarkReviewJobWaitingForUserParams) (db.ReviewJob, error)
	CompleteReviewJob(context.Context, db.CompleteReviewJobParams) (db.ReviewJob, error)
	FailReviewJob(context.Context, db.FailReviewJobParams) (db.ReviewJob, error)
}

type reviewDomainResponse struct {
	ID          string            `json:"id"`
	Domain      string            `json:"domain"`
	Status      string            `json:"status"`
	CnameTarget string            `json:"cname_target"`
	TLSStatus   string            `json:"tls_status"`
	DNSRecords  []reviewDNSRecord `json:"dns_records"`
}

type reviewDNSRecord struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type reviewKitResponse struct {
	ID             string   `json:"id"`
	Platform       string   `json:"platform"`
	UseCase        string   `json:"use_case"`
	ReviewDomainID string   `json:"review_domain_id"`
	RequiredScopes []string `json:"required_scopes"`
	Status         string   `json:"status"`
}

type reviewJobResponse struct {
	ID             string `json:"id"`
	ReviewKitID    string `json:"review_kit_id"`
	Platform       string `json:"platform"`
	Status         string `json:"status"`
	AgentVersion   string `json:"agent_version"`
	AgentCommand   string `json:"agent_command"`
	TokenExpiresAt string `json:"token_expires_at"`
}

type reviewAgentEventResponse struct {
	ReviewJobID string `json:"review_job_id"`
	EventType   string `json:"event_type"`
	Status      string `json:"status"`
}

type reviewAgentJobStateResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func NewReviewHandler(store reviewStore) *ReviewHandler {
	return &ReviewHandler{
		store:          store,
		now:            time.Now,
		tokenGenerator: defaultReviewTokenGenerator,
	}
}

func (h *ReviewHandler) WithTokenGenerator(fn reviewTokenGenerator) *ReviewHandler {
	if fn != nil {
		h.tokenGenerator = fn
	}
	return h
}

func (h *ReviewHandler) WithNow(fn func() time.Time) *ReviewHandler {
	if fn != nil {
		h.now = fn
	}
	return h
}

func (h *ReviewHandler) CreateDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := reviewWorkspaceID(w, r)
	if !ok {
		return
	}

	var req struct {
		Domain   string `json:"domain"`
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	domain := normalizeReviewDomain(req.Domain)
	if !isValidReviewDomain(domain) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Domain must be a hostname without scheme or path")
		return
	}
	_, tokenHash, err := h.tokenGenerator("rvdns_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate verification token")
		return
	}
	verificationToken := "unipost-review=" + tokenHash

	created, err := h.store.CreateReviewDomain(r.Context(), db.CreateReviewDomainParams{
		WorkspaceID:       workspaceID,
		Domain:            domain,
		Provider:          pgtype.Text{String: strings.TrimSpace(req.Provider), Valid: strings.TrimSpace(req.Provider) != ""},
		Status:            reviewDomainStatusPending,
		VerificationToken: verificationToken,
		CnameTarget:       reviewCnameTarget,
		TlsStatus:         "pending",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create review domain")
		return
	}

	writeCreated(w, reviewDomainFromDB(created))
}

func (h *ReviewHandler) CreateKit(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := reviewWorkspaceID(w, r)
	if !ok {
		return
	}

	var req struct {
		Platform            string         `json:"platform"`
		UseCase             string         `json:"use_case"`
		ReviewDomainID      string         `json:"review_domain_id"`
		RedirectURIAttested bool           `json:"redirect_uri_attested"`
		BrandSnapshot       map[string]any `json:"brand_snapshot"`
		RequiredScopes      []string       `json:"required_scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	useCase := strings.TrimSpace(req.UseCase)
	if platform == "" {
		platform = reviewDefaultPlatform
	}
	if useCase == "" {
		useCase = reviewDefaultUseCase
	}
	if platform != reviewDefaultPlatform || useCase != reviewDefaultUseCase {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Only TikTok content_posting review kits are supported")
		return
	}
	if !req.RedirectURIAttested {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Confirm that the OAuth redirect URI has been added in the TikTok developer portal before recording")
		return
	}
	if missing := missingTikTokScopes(req.RequiredScopes); len(missing) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "TikTok review requires scopes: "+strings.Join(requiredTikTokReviewScopes, ", "))
		return
	}

	domain, err := h.store.GetReviewDomain(r.Context(), db.GetReviewDomainParams{ID: strings.TrimSpace(req.ReviewDomainID), WorkspaceID: workspaceID})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review domain not found", "Failed to load review domain")
		return
	}
	if !isReviewDomainReady(domain) {
		writeError(w, http.StatusConflict, "CONFLICT", "Review domain DNS and TLS must be ready before creating a kit")
		return
	}
	if _, err := h.store.GetPlatformCredential(r.Context(), db.GetPlatformCredentialParams{WorkspaceID: workspaceID, Platform: platform}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "CONFLICT", "TikTok platform credentials are required before creating a review kit")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load TikTok credentials")
		return
	}

	brandSnapshot := req.BrandSnapshot
	if brandSnapshot == nil {
		brandSnapshot = map[string]any{}
	}
	brandJSON, err := json.Marshal(brandSnapshot)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid brand snapshot")
		return
	}
	created, err := h.store.CreateReviewKit(r.Context(), db.CreateReviewKitParams{
		WorkspaceID:    workspaceID,
		Platform:       platform,
		UseCase:        useCase,
		ReviewDomainID: domain.ID,
		BrandSnapshot:  brandJSON,
		RequiredScopes: append([]string(nil), requiredTikTokReviewScopes...),
		Status:         reviewKitStatusReady,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create review kit")
		return
	}

	writeCreated(w, reviewKitFromDB(created))
}

func (h *ReviewHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := reviewWorkspaceID(w, r)
	if !ok {
		return
	}

	var req struct {
		ReviewKitID string `json:"review_kit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	kit, err := h.store.GetReviewKit(r.Context(), db.GetReviewKitParams{ID: strings.TrimSpace(req.ReviewKitID), WorkspaceID: workspaceID})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review kit not found", "Failed to load review kit")
		return
	}
	if kit.Status != reviewKitStatusReady {
		writeError(w, http.StatusConflict, "CONFLICT", "Review kit is not ready")
		return
	}
	domain, err := h.store.GetReviewDomain(r.Context(), db.GetReviewDomainParams{ID: kit.ReviewDomainID, WorkspaceID: workspaceID})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review domain not found", "Failed to load review domain")
		return
	}
	if !isReviewDomainReady(domain) {
		writeError(w, http.StatusConflict, "CONFLICT", "Review domain DNS and TLS must be ready before recording")
		return
	}

	job, err := h.store.CreateReviewJob(r.Context(), db.CreateReviewJobParams{
		ReviewKitID:          kit.ID,
		WorkspaceID:          workspaceID,
		Platform:             kit.Platform,
		Status:               reviewJobStatusQueued,
		AgentVersion:         pgtype.Text{String: reviewAgentVersion, Valid: true},
		ReviewSessionTokenID: pgtype.Text{},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create review job")
		return
	}

	expiresAt := h.now().UTC().Add(reviewTokenTTL)
	sessionRaw, sessionHash, err := h.tokenGenerator("revsess_")
	_ = sessionRaw
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate review session token")
		return
	}
	session, err := h.store.CreateReviewSession(r.Context(), db.CreateReviewSessionParams{
		ReviewJobID:  job.ID,
		ReviewKitID:  kit.ID,
		WorkspaceID:  workspaceID,
		Platform:     kit.Platform,
		ReviewDomain: domain.Domain,
		TokenHash:    sessionHash,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create review session")
		return
	}
	if _, err := h.store.AttachReviewSessionToJob(r.Context(), db.AttachReviewSessionToJobParams{
		ID:                   job.ID,
		WorkspaceID:          workspaceID,
		ReviewSessionTokenID: pgtype.Text{String: session.ID, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to attach review session")
		return
	}

	agentRaw, agentHash, err := h.tokenGenerator("revtok_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate agent token")
		return
	}
	if _, err := h.store.CreateReviewAgentToken(r.Context(), db.CreateReviewAgentTokenParams{
		ReviewJobID: job.ID,
		WorkspaceID: workspaceID,
		Platform:    kit.Platform,
		TokenHash:   agentHash,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create agent token")
		return
	}

	writeCreated(w, reviewJobResponse{
		ID:             job.ID,
		ReviewKitID:    kit.ID,
		Platform:       kit.Platform,
		Status:         job.Status,
		AgentVersion:   reviewAgentVersion,
		AgentCommand:   "npx --yes @unipost/review-agent@" + reviewAgentVersion + " run --token " + agentRaw,
		TokenExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

func (h *ReviewHandler) GetJobScript(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := reviewWorkspaceID(w, r)
	if !ok {
		return
	}
	script, err := h.buildJobScript(r.Context(), chi.URLParam(r, "id"), workspaceID)
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review job script not found", "Failed to load review job script")
		return
	}
	writeSuccess(w, script)
}

func (h *ReviewHandler) GetAgentJobScript(w http.ResponseWriter, r *http.Request) {
	agentToken, ok := h.authenticateReviewAgent(w, r)
	if !ok {
		return
	}
	script, err := h.buildJobScript(r.Context(), agentToken.ReviewJobID, agentToken.WorkspaceID)
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review job script not found", "Failed to load review job script")
		return
	}
	writeSuccess(w, script)
}

func (h *ReviewHandler) RecordAgentEvent(w http.ResponseWriter, r *http.Request) {
	agentToken, ok := h.authenticateReviewAgent(w, r)
	if !ok {
		return
	}
	var req struct {
		EventType string         `json:"event_type"`
		Message   string         `json:"message"`
		Metadata  map[string]any `json:"metadata"`
		ElapsedMS *int64         `json:"elapsed_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "event_type is required")
		return
	}
	metadataJSON, err := marshalReviewJSON(req.Metadata)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "metadata must be valid JSON")
		return
	}
	elapsed := pgtype.Int8{}
	if req.ElapsedMS != nil {
		elapsed = pgtype.Int8{Int64: *req.ElapsedMS, Valid: true}
	}
	if _, err := h.store.CreateReviewJobEvent(r.Context(), db.CreateReviewJobEventParams{
		ReviewJobID: agentToken.ReviewJobID,
		EventType:   eventType,
		Message:     strings.TrimSpace(req.Message),
		Metadata:    metadataJSON,
		ElapsedMs:   elapsed,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to record review agent event")
		return
	}
	status := "event_recorded"
	switch eventType {
	case "recording_started":
		if _, err := h.store.MarkReviewJobRunning(r.Context(), db.MarkReviewJobRunningParams{ID: agentToken.ReviewJobID, WorkspaceID: agentToken.WorkspaceID}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark review job running")
			return
		}
		status = "running"
	case "manual_pause":
		if _, err := h.store.MarkReviewJobWaitingForUser(r.Context(), db.MarkReviewJobWaitingForUserParams{ID: agentToken.ReviewJobID, WorkspaceID: agentToken.WorkspaceID}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark review job waiting for user")
			return
		}
		status = "waiting_for_user"
	}
	writeCreated(w, reviewAgentEventResponse{ReviewJobID: agentToken.ReviewJobID, EventType: eventType, Status: status})
}

func (h *ReviewHandler) CompleteAgentJob(w http.ResponseWriter, r *http.Request) {
	agentToken, ok := h.authenticateReviewAgent(w, r)
	if !ok {
		return
	}
	var req struct {
		VideoFileID string         `json:"video_file_id"`
		Artifacts   map[string]any `json:"artifacts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	artifactsJSON, err := marshalReviewJSON(req.Artifacts)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "artifacts must be valid JSON")
		return
	}
	job, err := h.store.CompleteReviewJob(r.Context(), db.CompleteReviewJobParams{
		ID:            agentToken.ReviewJobID,
		WorkspaceID:   agentToken.WorkspaceID,
		VideoFileID:   pgtype.Text{String: strings.TrimSpace(req.VideoFileID), Valid: strings.TrimSpace(req.VideoFileID) != ""},
		ArtifactsJson: artifactsJSON,
	})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review job not found", "Failed to complete review job")
		return
	}
	writeSuccess(w, reviewAgentJobStateResponse{ID: job.ID, Status: job.Status})
}

func (h *ReviewHandler) FailAgentJob(w http.ResponseWriter, r *http.Request) {
	agentToken, ok := h.authenticateReviewAgent(w, r)
	if !ok {
		return
	}
	var req struct {
		FailureReason string         `json:"failure_reason"`
		Artifacts     map[string]any `json:"artifacts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	failureReason := strings.TrimSpace(req.FailureReason)
	if failureReason == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "failure_reason is required")
		return
	}
	artifactsJSON, err := marshalReviewJSON(req.Artifacts)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "artifacts must be valid JSON")
		return
	}
	job, err := h.store.FailReviewJob(r.Context(), db.FailReviewJobParams{
		ID:            agentToken.ReviewJobID,
		WorkspaceID:   agentToken.WorkspaceID,
		FailureReason: pgtype.Text{String: failureReason, Valid: true},
		ArtifactsJson: artifactsJSON,
	})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review job not found", "Failed to fail review job")
		return
	}
	writeSuccess(w, reviewAgentJobStateResponse{ID: job.ID, Status: job.Status})
}

func (h *ReviewHandler) authenticateReviewAgent(w http.ResponseWriter, r *http.Request) (db.ReviewAgentToken, bool) {
	rawToken := bearerToken(r)
	if rawToken == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing review agent token")
		return db.ReviewAgentToken{}, false
	}
	agentToken, err := h.store.GetReviewAgentTokenByHash(r.Context(), hashReviewToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired review agent token")
			return db.ReviewAgentToken{}, false
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load review agent token")
		return db.ReviewAgentToken{}, false
	}
	if !featureflags.Enabled(r.Context(), featureflags.AppReviewAutopilotV1, featureflags.Target{WorkspaceID: agentToken.WorkspaceID, Env: runtimeenv.Current()}) {
		writeError(w, http.StatusForbidden, "FEATURE_DISABLED", "This feature is currently disabled.")
		return db.ReviewAgentToken{}, false
	}
	return agentToken, true
}

func (h *ReviewHandler) buildJobScript(ctx context.Context, jobID, workspaceID string) (reviewscript.Script, error) {
	job, err := h.store.GetReviewJob(ctx, db.GetReviewJobParams{ID: jobID, WorkspaceID: workspaceID})
	if err != nil {
		return reviewscript.Script{}, err
	}
	kit, err := h.store.GetReviewKit(ctx, db.GetReviewKitParams{ID: job.ReviewKitID, WorkspaceID: workspaceID})
	if err != nil {
		return reviewscript.Script{}, err
	}
	domain, err := h.store.GetReviewDomain(ctx, db.GetReviewDomainParams{ID: kit.ReviewDomainID, WorkspaceID: workspaceID})
	if err != nil {
		return reviewscript.Script{}, err
	}
	session, err := h.store.GetActiveReviewSessionForJob(ctx, db.GetActiveReviewSessionForJobParams{ReviewJobID: job.ID, WorkspaceID: workspaceID})
	if err != nil {
		return reviewscript.Script{}, err
	}

	agentVersion := reviewAgentVersion
	if job.AgentVersion.Valid && strings.TrimSpace(job.AgentVersion.String) != "" {
		agentVersion = job.AgentVersion.String
	}
	expiresAt := ""
	if session.ExpiresAt.Valid {
		expiresAt = session.ExpiresAt.Time.UTC().Format(time.RFC3339)
	}
	script := reviewscript.BuildTikTokScript(reviewscript.BuildTikTokScriptInput{
		JobID:               job.ID,
		AgentVersion:        agentVersion,
		ReviewDomain:        domain.Domain,
		SessionCookieName:   reviewSessionCookieName,
		SessionExpiresAt:    expiresAt,
		RequireAddressBar:   true,
		BrowserWindowWidth:  1440,
		BrowserWindowHeight: 1000,
	})
	if err := script.Validate(); err != nil {
		return reviewscript.Script{}, err
	}
	return script, nil
}

func reviewWorkspaceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return "", false
	}
	return workspaceID, true
}

func reviewDomainFromDB(row db.ReviewDomain) reviewDomainResponse {
	return reviewDomainResponse{
		ID:          row.ID,
		Domain:      row.Domain,
		Status:      row.Status,
		CnameTarget: row.CnameTarget,
		TLSStatus:   row.TlsStatus,
		DNSRecords: []reviewDNSRecord{
			{Type: "CNAME", Name: row.Domain, Value: row.CnameTarget},
			{Type: "TXT", Name: "_unipost-review." + row.Domain, Value: row.VerificationToken},
		},
	}
}

func reviewKitFromDB(row db.ReviewKit) reviewKitResponse {
	return reviewKitResponse{
		ID:             row.ID,
		Platform:       row.Platform,
		UseCase:        row.UseCase,
		ReviewDomainID: row.ReviewDomainID,
		RequiredScopes: row.RequiredScopes,
		Status:         row.Status,
	}
}

func marshalReviewJSON(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return json.Marshal(value)
}

func defaultReviewTokenGenerator(prefix string) (string, string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw := prefix + base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashReviewToken(raw), nil
}

func hashReviewToken(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func normalizeReviewDomain(input string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(input)), ".")
}

func isValidReviewDomain(domain string) bool {
	if domain == "" || strings.Contains(domain, "://") || strings.Contains(domain, "/") || strings.Contains(domain, " ") {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || !strings.Contains(domain, ".") {
		return false
	}
	return true
}

func isReviewDomainReady(domain db.ReviewDomain) bool {
	if domain.Status != reviewDomainStatusReady {
		return false
	}
	return domain.TlsStatus == "" || domain.TlsStatus == reviewTLSStatusIssued
}

func missingTikTokScopes(scopes []string) []string {
	seen := map[string]bool{}
	for _, scope := range scopes {
		seen[strings.TrimSpace(scope)] = true
	}
	missing := []string{}
	for _, scope := range requiredTikTokReviewScopes {
		if !seen[scope] {
			missing = append(missing, scope)
		}
	}
	return missing
}

func writeNotFoundOrInternal(w http.ResponseWriter, err error, notFoundMessage string, internalMessage string) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", notFoundMessage)
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", internalMessage)
}
