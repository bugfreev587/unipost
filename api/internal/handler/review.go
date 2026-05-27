package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/reviewscript"
	"github.com/xiaoboyu/unipost-api/internal/reviewtemplate"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
)

const (
	reviewAgentVersion        = "0.1.0"
	reviewDefaultCnameTarget  = "review.unipost.dev"
	reviewSessionCookieName   = "__unipost_review_session"
	reviewTokenTTL            = 30 * time.Minute
	reviewDefaultPlatform     = "tiktok"
	reviewDomainStatusReady   = "ready"
	reviewJobStatusQueued     = "queued"
	reviewKitStatusReady      = "ready"
	reviewDomainStatusPending = "dns_pending"
	reviewTLSStatusIssued     = "issued"
	reviewDefaultTestVideoURL = "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4"
	reviewDefaultCaption      = "UniPost app review test video"
)

const (
	reviewArtifactUploadTTL   = 15 * time.Minute
	reviewArtifactDownloadTTL = 30 * time.Minute
	reviewArtifactMaxBytes    = 1024 * 1024 * 1024
)

type reviewTokenGenerator func(prefix string) (raw string, hash string, err error)
type reviewDomainChecker func(context.Context, db.ReviewDomain) reviewDomainCheckResult

type reviewArtifactStorage interface {
	PresignPut(context.Context, string, string, time.Duration) (string, error)
	PresignGet(context.Context, string, time.Duration) (string, error)
}

type reviewTikTokAdapter interface {
	Post(context.Context, string, string, []platform.MediaItem, map[string]any) (*platform.PostResult, error)
	RefreshToken(context.Context, string) (string, string, time.Time, error)
	FetchCreatorInfo(context.Context, string) (*platform.TikTokCreatorInfo, error)
}

type reviewDomainCheckResult struct {
	DNSReady  bool
	TLSIssued bool
	Message   string
}

type ReviewHandler struct {
	store           reviewStore
	now             func() time.Time
	tokenGenerator  reviewTokenGenerator
	domainChecker   reviewDomainChecker
	cnameTarget     string
	apiBaseURL      string
	artifactStorage reviewArtifactStorage
	encryptor       *appcrypto.AESEncryptor
	tiktokAdapter   reviewTikTokAdapter
	testVideoURL    string
}

type reviewStore interface {
	CreateReviewDomain(context.Context, db.CreateReviewDomainParams) (db.ReviewDomain, error)
	GetReviewDomain(context.Context, db.GetReviewDomainParams) (db.ReviewDomain, error)
	ListReviewDomainsByWorkspace(context.Context, string) ([]db.ReviewDomain, error)
	UpdateReviewDomainVerification(context.Context, db.UpdateReviewDomainVerificationParams) (db.ReviewDomain, error)
	GetPlatformCredential(context.Context, db.GetPlatformCredentialParams) (db.PlatformCredential, error)
	CreateReviewKit(context.Context, db.CreateReviewKitParams) (db.ReviewKit, error)
	GetReviewKit(context.Context, db.GetReviewKitParams) (db.ReviewKit, error)
	ListReviewKitsByWorkspace(context.Context, string) ([]db.ReviewKit, error)
	GetProfile(context.Context, string) (db.Profile, error)
	CreateConnectSession(context.Context, db.CreateConnectSessionParams) (db.ConnectSession, error)
	CreateReviewJob(context.Context, db.CreateReviewJobParams) (db.ReviewJob, error)
	GetReviewJob(context.Context, db.GetReviewJobParams) (db.ReviewJob, error)
	ListReviewJobsByKit(context.Context, db.ListReviewJobsByKitParams) ([]db.ReviewJob, error)
	CreateReviewSession(context.Context, db.CreateReviewSessionParams) (db.ReviewSession, error)
	GetActiveReviewSessionForJob(context.Context, db.GetActiveReviewSessionForJobParams) (db.ReviewSession, error)
	GetReviewSessionByTokenHash(context.Context, string) (db.ReviewSession, error)
	AttachReviewSessionToJob(context.Context, db.AttachReviewSessionToJobParams) (db.ReviewJob, error)
	CreateReviewAgentToken(context.Context, db.CreateReviewAgentTokenParams) (db.ReviewAgentToken, error)
	GetReviewAgentTokenByHash(context.Context, string) (db.ReviewAgentToken, error)
	CreateReviewJobEvent(context.Context, db.CreateReviewJobEventParams) (db.ReviewJobEvent, error)
	ListReviewJobEvents(context.Context, db.ListReviewJobEventsParams) ([]db.ReviewJobEvent, error)
	MarkReviewJobRunning(context.Context, db.MarkReviewJobRunningParams) (db.ReviewJob, error)
	MarkReviewJobWaitingForUser(context.Context, db.MarkReviewJobWaitingForUserParams) (db.ReviewJob, error)
	CompleteReviewJob(context.Context, db.CompleteReviewJobParams) (db.ReviewJob, error)
	FailReviewJob(context.Context, db.FailReviewJobParams) (db.ReviewJob, error)
	ListSocialAccountsByProfileFiltered(context.Context, db.ListSocialAccountsByProfileFilteredParams) ([]db.SocialAccount, error)
	UpdateSocialAccountTokens(context.Context, db.UpdateSocialAccountTokensParams) error
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

type reviewStateResponse struct {
	Domain *reviewDomainResponse `json:"domain,omitempty"`
	Kit    *reviewKitResponse    `json:"kit,omitempty"`
	Job    *reviewJobResponse    `json:"job,omitempty"`
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

type reviewPublicSessionResponse struct {
	JobID               string                     `json:"job_id"`
	Platform            string                     `json:"platform"`
	ReviewDomain        string                     `json:"review_domain"`
	Status              string                     `json:"status"`
	ExpiresAt           string                     `json:"expires_at"`
	TestVideoURL        string                     `json:"test_video_url"`
	DefaultCaption      string                     `json:"default_caption"`
	Connected           bool                       `json:"connected"`
	Account             *reviewSessionAccount      `json:"account,omitempty"`
	CreatorInfo         *tiktokCreatorInfoResponse `json:"creator_info,omitempty"`
	CreatorInfoError    string                     `json:"creator_info_error,omitempty"`
	ConnectAuthorizeURL string                     `json:"connect_authorize_url,omitempty"`
}

type reviewSessionAccount struct {
	ID                string   `json:"id"`
	AccountName       string   `json:"account_name"`
	ExternalAccountID string   `json:"external_account_id"`
	Scope             []string `json:"scope,omitempty"`
}

type reviewTikTokPublishResponse struct {
	Status       string `json:"status"`
	ExternalID   string `json:"external_id,omitempty"`
	URL          string `json:"url,omitempty"`
	PrivacyLevel string `json:"privacy_level"`
	VideoURL     string `json:"video_url"`
}

type reviewAgentArtifactUploadResponse struct {
	FileID      string            `json:"file_id"`
	UploadURL   string            `json:"upload_url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	ExpiresAt   string            `json:"expires_at"`
	ContentType string            `json:"content_type"`
}

type reviewJobDetailResponse struct {
	ID               string                     `json:"id"`
	ReviewKitID      string                     `json:"review_kit_id"`
	Platform         string                     `json:"platform"`
	Status           string                     `json:"status"`
	AgentVersion     string                     `json:"agent_version"`
	VideoFileID      string                     `json:"video_file_id,omitempty"`
	VideoDownloadURL string                     `json:"video_download_url,omitempty"`
	VideoArtifacts   []reviewVideoArtifactItem  `json:"video_artifacts,omitempty"`
	FailureReason    string                     `json:"failure_reason,omitempty"`
	Artifacts        map[string]any             `json:"artifacts"`
	Events           []reviewJobEventDetailItem `json:"events"`
}

type reviewVideoArtifactItem struct {
	SegmentKey  string   `json:"segment_key,omitempty"`
	Filename    string   `json:"filename,omitempty"`
	FileID      string   `json:"file_id"`
	DownloadURL string   `json:"download_url,omitempty"`
	Format      string   `json:"format,omitempty"`
	DurationSec float64  `json:"duration_sec,omitempty"`
	SizeBytes   int64    `json:"size_bytes,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

type reviewJobEventDetailItem struct {
	EventType string         `json:"event_type"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
	ElapsedMs int64          `json:"elapsed_ms"`
	CreatedAt string         `json:"created_at,omitempty"`
}

func NewReviewHandler(store reviewStore) *ReviewHandler {
	return &ReviewHandler{
		store:          store,
		now:            time.Now,
		tokenGenerator: defaultReviewTokenGenerator,
		domainChecker:  defaultReviewDomainChecker,
		cnameTarget:    reviewDefaultCnameTarget,
		apiBaseURL:     "https://api.unipost.dev",
		testVideoURL:   reviewDefaultTestVideoURL,
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

func (h *ReviewHandler) WithDomainChecker(fn reviewDomainChecker) *ReviewHandler {
	if fn != nil {
		h.domainChecker = fn
	}
	return h
}

func (h *ReviewHandler) WithAPIBaseURL(value string) *ReviewHandler {
	if strings.TrimSpace(value) != "" {
		h.apiBaseURL = strings.TrimRight(strings.TrimSpace(value), "/")
	}
	return h
}

func (h *ReviewHandler) WithReviewCnameTarget(value string) *ReviewHandler {
	if strings.TrimSpace(value) != "" {
		h.cnameTarget = strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
	}
	return h
}

func (h *ReviewHandler) WithArtifactStorage(store reviewArtifactStorage) *ReviewHandler {
	h.artifactStorage = store
	return h
}

func (h *ReviewHandler) WithEncryptor(encryptor *appcrypto.AESEncryptor) *ReviewHandler {
	h.encryptor = encryptor
	return h
}

func (h *ReviewHandler) WithTikTokAdapter(adapter reviewTikTokAdapter) *ReviewHandler {
	h.tiktokAdapter = adapter
	return h
}

func (h *ReviewHandler) WithTikTokTestVideoURL(value string) *ReviewHandler {
	if strings.TrimSpace(value) != "" {
		h.testVideoURL = strings.TrimSpace(value)
	}
	return h
}

func (h *ReviewHandler) GetState(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := reviewWorkspaceID(w, r)
	if !ok {
		return
	}
	domains, err := h.store.ListReviewDomainsByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load review domains")
		return
	}
	kits, err := h.store.ListReviewKitsByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load review kits")
		return
	}

	domainsByID := make(map[string]db.ReviewDomain, len(domains))
	for _, domain := range domains {
		domainsByID[domain.ID] = domain
	}

	var resp reviewStateResponse
	if kit := selectReviewStateKit(kits); kit != nil {
		kitResp := reviewKitFromDB(*kit)
		resp.Kit = &kitResp
		if domain, ok := domainsByID[kit.ReviewDomainID]; ok {
			domainResp := reviewDomainFromDB(domain)
			resp.Domain = &domainResp
		}
		jobs, err := h.store.ListReviewJobsByKit(r.Context(), db.ListReviewJobsByKitParams{ReviewKitID: kit.ID, WorkspaceID: workspaceID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load review jobs")
			return
		}
		if len(jobs) > 0 {
			jobResp := reviewJobSummaryFromDB(jobs[0])
			resp.Job = &jobResp
		}
	}
	if resp.Domain == nil {
		if domain := selectReviewStateDomain(domains); domain != nil {
			domainResp := reviewDomainFromDB(*domain)
			resp.Domain = &domainResp
		}
	}

	writeSuccess(w, resp)
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
		CnameTarget:       h.cnameTarget,
		TlsStatus:         "pending",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create review domain")
		return
	}

	writeCreated(w, reviewDomainFromDB(created))
}

func (h *ReviewHandler) VerifyDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := reviewWorkspaceID(w, r)
	if !ok {
		return
	}
	domain, err := h.store.GetReviewDomain(r.Context(), db.GetReviewDomainParams{ID: strings.TrimSpace(chi.URLParam(r, "id")), WorkspaceID: workspaceID})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review domain not found", "Failed to load review domain")
		return
	}
	check := h.domainChecker(r.Context(), domain)
	if !check.DNSReady {
		message := strings.TrimSpace(check.Message)
		if message == "" {
			message = "Review domain DNS records have not propagated yet. Confirm the CNAME and TXT records, then try again."
		}
		writeError(w, http.StatusConflict, "CONFLICT", message)
		return
	}
	status := reviewDomainStatusReady
	tlsStatus := reviewTLSStatusIssued
	tlsIssuedAt := pgtype.Timestamptz{Time: h.now().UTC(), Valid: true}
	if !check.TLSIssued {
		status = reviewDomainStatusPending
		tlsStatus = "pending"
		tlsIssuedAt = pgtype.Timestamptz{}
	}
	updated, err := h.store.UpdateReviewDomainVerification(r.Context(), db.UpdateReviewDomainVerificationParams{
		ID:            domain.ID,
		WorkspaceID:   workspaceID,
		Status:        status,
		DnsVerifiedAt: pgtype.Timestamptz{Time: h.now().UTC(), Valid: true},
		TlsStatus:     tlsStatus,
		TlsIssuedAt:   tlsIssuedAt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update review domain verification")
		return
	}
	if !check.TLSIssued {
		writeSuccess(w, reviewDomainFromDB(updated))
		return
	}
	writeSuccess(w, reviewDomainFromDB(updated))
}

func (h *ReviewHandler) GetTikTokScopeTemplates(w http.ResponseWriter, r *http.Request) {
	if _, ok := reviewWorkspaceID(w, r); !ok {
		return
	}
	writeSuccess(w, reviewtemplate.ListTikTokScopeTemplates())
}

func (h *ReviewHandler) CreateTikTokDemoPlan(w http.ResponseWriter, r *http.Request) {
	if _, ok := reviewWorkspaceID(w, r); !ok {
		return
	}
	var req struct {
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	plan, err := reviewtemplate.BuildTikTokDemoPlan(reviewtemplate.TikTokDemoPlanInput{Scopes: req.Scopes})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}
	writeCreated(w, plan)
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
		ProfileID           string         `json:"profile_id"`
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
	if platform != reviewDefaultPlatform {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Only TikTok review kits are supported")
		return
	}
	plan, err := reviewtemplate.BuildTikTokDemoPlan(reviewtemplate.TikTokDemoPlanInput{Scopes: req.RequiredScopes})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}
	if useCase == "" {
		useCase = plan.UseCase
	}
	if useCase != plan.UseCase {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "use_case must match the selected TikTok scopes")
		return
	}
	if !req.RedirectURIAttested {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Confirm that the OAuth redirect URI has been added in the TikTok developer portal before recording")
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
	brandSnapshot["scope_template_version"] = plan.TemplateVersion
	brandSnapshot["oauth_reset_required"] = true
	brandSnapshot["review_plan"] = plan
	if profileID := strings.TrimSpace(req.ProfileID); profileID != "" {
		profile, err := h.store.GetProfile(r.Context(), profileID)
		if err != nil || profile.WorkspaceID != workspaceID {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "profile_id must belong to this workspace")
			return
		}
		brandSnapshot["profile_id"] = profile.ID
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
		RequiredScopes: append([]string(nil), plan.RequestedScopes...),
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
		AgentCommand:   "npx --yes @unipost/review-agent@" + reviewAgentVersion + " run --token " + agentRaw + " --session-token " + sessionRaw,
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

func (h *ReviewHandler) authenticateReviewSession(w http.ResponseWriter, r *http.Request) (db.ReviewSession, bool) {
	rawToken := bearerToken(r)
	if rawToken == "" {
		if cookie, err := r.Cookie(reviewSessionCookieName); err == nil {
			rawToken = strings.TrimSpace(cookie.Value)
		}
	}
	if rawToken == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing review session token")
		return db.ReviewSession{}, false
	}
	session, err := h.store.GetReviewSessionByTokenHash(r.Context(), hashReviewToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired review session token")
			return db.ReviewSession{}, false
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load review session")
		return db.ReviewSession{}, false
	}
	if !featureflags.Enabled(r.Context(), featureflags.AppReviewAutopilotV1, featureflags.Target{WorkspaceID: session.WorkspaceID, Env: runtimeenv.Current()}) {
		writeError(w, http.StatusForbidden, "FEATURE_DISABLED", "This feature is currently disabled.")
		return db.ReviewSession{}, false
	}
	return session, true
}

func (h *ReviewHandler) GetPublicReviewSession(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticateReviewSession(w, r)
	if !ok {
		return
	}
	kit, err := h.store.GetReviewKit(r.Context(), db.GetReviewKitParams{ID: session.ReviewKitID, WorkspaceID: session.WorkspaceID})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review kit not found", "Failed to load review kit")
		return
	}
	domain, err := h.store.GetReviewDomain(r.Context(), db.GetReviewDomainParams{ID: kit.ReviewDomainID, WorkspaceID: session.WorkspaceID})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review domain not found", "Failed to load review domain")
		return
	}
	expiresAt := ""
	if session.ExpiresAt.Valid {
		expiresAt = session.ExpiresAt.Time.UTC().Format(time.RFC3339)
	}
	resp := reviewPublicSessionResponse{
		JobID:          session.ReviewJobID,
		Platform:       session.Platform,
		ReviewDomain:   domain.Domain,
		Status:         "ready",
		ExpiresAt:      expiresAt,
		TestVideoURL:   strings.TrimSpace(h.testVideoURL),
		DefaultCaption: reviewDefaultCaption,
	}
	profileID := reviewKitProfileID(kit)
	if profileID != "" {
		profile, err := h.store.GetProfile(r.Context(), profileID)
		if err != nil || profile.WorkspaceID != session.WorkspaceID {
			writeError(w, http.StatusConflict, "CONFLICT", "Review kit profile is unavailable")
			return
		}
		account, connected, err := h.connectedReviewTikTokAccount(r.Context(), profile.ID, session)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load TikTok review account")
			return
		}
		if connected {
			resp.Connected = true
			resp.Account = reviewSessionAccountFromDB(account)
			adapter, err := h.getReviewTikTokAdapter()
			if err != nil {
				resp.Status = "creator_info_error"
				resp.CreatorInfoError = err.Error()
			} else if info, err := h.fetchReviewCreatorInfo(r.Context(), account, adapter); err != nil {
				slog.Warn("review session: creator_info failed", "job_id", session.ReviewJobID, "account_id", account.ID, "error", err)
				resp.Status = "creator_info_error"
				resp.CreatorInfoError = err.Error()
			} else {
				resp.CreatorInfo = info
			}
			writeSuccess(w, resp)
			return
		}
		oauthState, _, err := h.tokenGenerator("rvstate_")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate review connect state")
			return
		}
		connectSession, err := h.store.CreateConnectSession(r.Context(), db.CreateConnectSessionParams{
			ProfileID:            profile.ID,
			Platform:             session.Platform,
			ExternalUserID:       "app-review:" + session.ReviewJobID,
			ReturnUrl:            pgtype.Text{String: "https://" + domain.Domain + reviewReturnPath(kit) + "?connect_status=success", Valid: true},
			OauthState:           oauthState,
			ExpiresAt:            session.ExpiresAt,
			AllowQuickstartCreds: false,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create TikTok review connect session")
			return
		}
		resp.ConnectAuthorizeURL = h.apiBaseURL + "/v1/public/connect/sessions/" + connectSession.ID + "/authorize?state=" + connectSession.OauthState
	}
	writeSuccess(w, resp)
}

func reviewReturnPath(kit db.ReviewKit) string {
	if kit.UseCase == "analytics" {
		return "/tiktok/analytics"
	}
	return "/tiktok/posting"
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
	case "manual_pause", "manual_pause_started":
		if _, err := h.store.MarkReviewJobWaitingForUser(r.Context(), db.MarkReviewJobWaitingForUserParams{ID: agentToken.ReviewJobID, WorkspaceID: agentToken.WorkspaceID}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark review job waiting for user")
			return
		}
		status = "waiting_for_user"
	case "manual_pause_completed":
		if _, err := h.store.MarkReviewJobRunning(r.Context(), db.MarkReviewJobRunningParams{ID: agentToken.ReviewJobID, WorkspaceID: agentToken.WorkspaceID}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to mark review job running")
			return
		}
		status = "running"
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
	videoFileID := strings.TrimSpace(req.VideoFileID)
	if videoFileID != "" && !strings.HasPrefix(videoFileID, reviewArtifactDirectory(agentToken.WorkspaceID, agentToken.ReviewJobID)+"/") {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "video_file_id must belong to the current review job")
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
		VideoFileID:   pgtype.Text{String: videoFileID, Valid: videoFileID != ""},
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

func (h *ReviewHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := reviewWorkspaceID(w, r)
	if !ok {
		return
	}
	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Review job id is required")
		return
	}
	job, err := h.store.GetReviewJob(r.Context(), db.GetReviewJobParams{ID: jobID, WorkspaceID: workspaceID})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review job not found", "Failed to load review job")
		return
	}
	events, err := h.store.ListReviewJobEvents(r.Context(), db.ListReviewJobEventsParams{ReviewJobID: jobID, WorkspaceID: workspaceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load review job events")
		return
	}
	resp, err := h.toReviewJobDetail(r.Context(), job, events)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to build review job response")
		return
	}
	writeSuccess(w, resp)
}

func (h *ReviewHandler) CreateAgentArtifactUpload(w http.ResponseWriter, r *http.Request) {
	agentToken, ok := h.authenticateReviewAgent(w, r)
	if !ok {
		return
	}
	if h.artifactStorage == nil {
		writeError(w, http.StatusServiceUnavailable, "STORAGE_NOT_CONFIGURED", "Review artifact storage is not configured")
		return
	}
	var req struct {
		ArtifactType string `json:"artifact_type"`
		SegmentKey   string `json:"segment_key"`
		ContentType  string `json:"content_type"`
		SizeBytes    int64  `json:"size_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(req.ContentType))
	fileID, err := reviewArtifactFileID(agentToken.WorkspaceID, agentToken.ReviewJobID, req.ArtifactType, req.SegmentKey, contentType)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.SizeBytes <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "size_bytes must be greater than 0")
		return
	}
	if req.SizeBytes > reviewArtifactMaxBytes {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "review artifact exceeds the maximum upload size")
		return
	}
	uploadURL, err := h.artifactStorage.PresignPut(r.Context(), fileID, contentType, reviewArtifactUploadTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create review artifact upload URL")
		return
	}
	expiresAt := h.now().Add(reviewArtifactUploadTTL)
	writeCreated(w, reviewAgentArtifactUploadResponse{
		FileID:      fileID,
		UploadURL:   uploadURL,
		Method:      http.MethodPut,
		Headers:     map[string]string{"Content-Type": contentType},
		ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
		ContentType: contentType,
	})
}

func (h *ReviewHandler) toReviewJobDetail(ctx context.Context, job db.ReviewJob, events []db.ReviewJobEvent) (reviewJobDetailResponse, error) {
	artifacts := map[string]any{}
	if len(job.ArtifactsJson) > 0 {
		if err := json.Unmarshal(job.ArtifactsJson, &artifacts); err != nil {
			return reviewJobDetailResponse{}, err
		}
	}
	agentVersion := reviewAgentVersion
	if job.AgentVersion.Valid && strings.TrimSpace(job.AgentVersion.String) != "" {
		agentVersion = job.AgentVersion.String
	}
	resp := reviewJobDetailResponse{
		ID:           job.ID,
		ReviewKitID:  job.ReviewKitID,
		Platform:     job.Platform,
		Status:       job.Status,
		AgentVersion: agentVersion,
		Artifacts:    artifacts,
		Events:       make([]reviewJobEventDetailItem, 0, len(events)),
	}
	if job.FailureReason.Valid {
		resp.FailureReason = job.FailureReason.String
	}
	if job.VideoFileID.Valid && strings.TrimSpace(job.VideoFileID.String) != "" {
		resp.VideoFileID = strings.TrimSpace(job.VideoFileID.String)
		if h.artifactStorage != nil {
			downloadURL, err := h.artifactStorage.PresignGet(ctx, resp.VideoFileID, reviewArtifactDownloadTTL)
			if err != nil {
				return reviewJobDetailResponse{}, err
			}
			resp.VideoDownloadURL = downloadURL
		}
	}
	if h.artifactStorage != nil {
		videoArtifacts, err := h.reviewVideoArtifactDownloads(ctx, job, artifacts)
		if err != nil {
			return reviewJobDetailResponse{}, err
		}
		resp.VideoArtifacts = videoArtifacts
	}
	for _, event := range events {
		metadata := map[string]any{}
		if len(event.Metadata) > 0 {
			_ = json.Unmarshal(event.Metadata, &metadata)
		}
		item := reviewJobEventDetailItem{
			EventType: event.EventType,
			Message:   event.Message,
			Metadata:  metadata,
		}
		if event.ElapsedMs.Valid {
			item.ElapsedMs = event.ElapsedMs.Int64
		}
		if event.CreatedAt.Valid {
			item.CreatedAt = event.CreatedAt.Time.UTC().Format(time.RFC3339)
		}
		resp.Events = append(resp.Events, item)
	}
	return resp, nil
}

func (h *ReviewHandler) reviewVideoArtifactDownloads(ctx context.Context, job db.ReviewJob, artifacts map[string]any) ([]reviewVideoArtifactItem, error) {
	rawSegments, ok := artifacts["video_segments"].([]any)
	if !ok || len(rawSegments) == 0 {
		return nil, nil
	}
	prefix := reviewArtifactDirectory(job.WorkspaceID, job.ID) + "/"
	out := make([]reviewVideoArtifactItem, 0, len(rawSegments))
	for _, raw := range rawSegments {
		segment, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		fileID := strings.TrimSpace(reviewStringField(segment, "file_id"))
		if fileID == "" || !strings.HasPrefix(fileID, prefix) {
			continue
		}
		downloadURL, err := h.artifactStorage.PresignGet(ctx, fileID, reviewArtifactDownloadTTL)
		if err != nil {
			return nil, err
		}
		out = append(out, reviewVideoArtifactItem{
			SegmentKey:  reviewStringField(segment, "segment_key"),
			Filename:    reviewStringField(segment, "filename"),
			FileID:      fileID,
			DownloadURL: downloadURL,
			Format:      reviewStringField(segment, "format"),
			DurationSec: reviewFloatField(segment, "duration_sec"),
			SizeBytes:   reviewIntField(segment, "size_bytes"),
			Scopes:      reviewStringSliceField(segment, "scopes"),
		})
	}
	return out, nil
}

func reviewStringField(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func reviewFloatField(values map[string]any, key string) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func reviewIntField(values map[string]any, key string) int64 {
	switch value := values[key].(type) {
	case float64:
		return int64(value)
	case int:
		return int64(value)
	case int64:
		return value
	default:
		return 0
	}
}

func reviewStringSliceField(values map[string]any, key string) []string {
	raw, ok := values[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func reviewArtifactFileID(workspaceID string, jobID string, artifactType string, segmentKey string, contentType string) (string, error) {
	slug, ok := reviewArtifactSlug(strings.TrimSpace(artifactType))
	if !ok {
		return "", errors.New("artifact_type must be one of: demo_video, execution_evidence")
	}
	ext, ok := reviewArtifactExtension(contentType)
	if !ok {
		return "", errors.New("content_type must be one of: video/mp4, video/webm, video/quicktime, application/json")
	}
	if slug == "execution-evidence" && contentType != "application/json" {
		return "", errors.New("execution_evidence artifacts must use application/json")
	}
	if slug == "demo-video" && !strings.HasPrefix(contentType, "video/") {
		return "", errors.New("demo_video artifacts must use a video content type")
	}
	if slug == "demo-video" {
		if segmentSlug := reviewArtifactSegmentSlug(segmentKey); segmentSlug != "" {
			slug += "-" + segmentSlug
		}
	}
	return reviewArtifactDirectory(workspaceID, jobID) + "/" + slug + ext, nil
}

func reviewArtifactSegmentSlug(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func reviewArtifactDirectory(workspaceID string, jobID string) string {
	return "review-artifacts/" + workspaceID + "/" + jobID
}

func reviewArtifactSlug(value string) (string, bool) {
	switch value {
	case "demo_video":
		return "demo-video", true
	case "execution_evidence":
		return "execution-evidence", true
	default:
		return "", false
	}
}

func reviewArtifactExtension(contentType string) (string, bool) {
	switch contentType {
	case "video/mp4":
		return ".mp4", true
	case "video/webm":
		return ".webm", true
	case "video/quicktime":
		return ".mov", true
	case "application/json":
		return ".json", true
	default:
		return "", false
	}
}

func (h *ReviewHandler) PublishReviewTikTokPost(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authenticateReviewSession(w, r)
	if !ok {
		return
	}
	if session.Platform != reviewDefaultPlatform {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Only TikTok review publishing is supported")
		return
	}
	kit, err := h.store.GetReviewKit(r.Context(), db.GetReviewKitParams{ID: session.ReviewKitID, WorkspaceID: session.WorkspaceID})
	if err != nil {
		writeNotFoundOrInternal(w, err, "Review kit not found", "Failed to load review kit")
		return
	}
	profileID := reviewKitProfileID(kit)
	if profileID == "" {
		writeError(w, http.StatusConflict, "CONFLICT", "Review kit profile is unavailable")
		return
	}
	profile, err := h.store.GetProfile(r.Context(), profileID)
	if err != nil || profile.WorkspaceID != session.WorkspaceID {
		writeError(w, http.StatusConflict, "CONFLICT", "Review kit profile is unavailable")
		return
	}
	account, connected, err := h.connectedReviewTikTokAccount(r.Context(), profile.ID, session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load TikTok review account")
		return
	}
	if !connected {
		writeError(w, http.StatusConflict, "TIKTOK_NOT_CONNECTED", "Connect TikTok before publishing the review test video")
		return
	}
	if events, err := h.store.ListReviewJobEvents(r.Context(), db.ListReviewJobEventsParams{ReviewJobID: session.ReviewJobID, WorkspaceID: session.WorkspaceID}); err == nil {
		if prior, ok := previousReviewPublishResponse(events); ok {
			writeSuccess(w, prior)
			return
		}
	}

	var req struct {
		Caption            string `json:"caption"`
		PrivacyLevel       string `json:"privacy_level"`
		DisableComment     *bool  `json:"disable_comment"`
		DisableDuet        *bool  `json:"disable_duet"`
		DisableStitch      *bool  `json:"disable_stitch"`
		BrandContentToggle *bool  `json:"brand_content_toggle"`
		BrandOrganicToggle *bool  `json:"brand_organic_toggle"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
			return
		}
	}
	caption := strings.TrimSpace(req.Caption)
	if caption == "" {
		caption = reviewDefaultCaption
	}
	privacyLevel := strings.TrimSpace(req.PrivacyLevel)
	if privacyLevel == "" {
		privacyLevel = "SELF_ONLY"
	}
	if !isAllowedTikTokPrivacy(privacyLevel) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid TikTok privacy_level")
		return
	}
	videoURL := strings.TrimSpace(h.testVideoURL)
	if videoURL == "" {
		writeError(w, http.StatusServiceUnavailable, "TEST_VIDEO_UNAVAILABLE", "Review test video is not configured")
		return
	}
	adapter, err := h.getReviewTikTokAdapter()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "TikTok adapter unavailable")
		return
	}
	accessToken, err := h.reviewTikTokAccessToken(r.Context(), account, adapter)
	if err != nil {
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "TikTok credentials are unavailable. Reconnect the review account.")
		return
	}
	opts := map[string]any{
		"privacy_level":        privacyLevel,
		"disable_comment":      reviewBoolDefault(req.DisableComment, true),
		"disable_duet":         reviewBoolDefault(req.DisableDuet, true),
		"disable_stitch":       reviewBoolDefault(req.DisableStitch, true),
		"brand_content_toggle": reviewBoolDefault(req.BrandContentToggle, false),
		"brand_organic_toggle": reviewBoolDefault(req.BrandOrganicToggle, false),
	}
	result, err := adapter.Post(r.Context(), accessToken, caption, []platform.MediaItem{{URL: videoURL, Kind: platform.MediaKindVideo}}, opts)
	if err != nil {
		h.recordReviewPublishEvent(r.Context(), session, "review_publish_failed", "TikTok review publish failed", map[string]any{
			"error":         err.Error(),
			"privacy_level": privacyLevel,
			"video_url":     videoURL,
		})
		writeError(w, http.StatusBadGateway, "TIKTOK_PUBLISH_FAILED", err.Error())
		return
	}
	status := "published"
	if result != nil && strings.TrimSpace(result.Status) != "" {
		status = strings.TrimSpace(result.Status)
	}
	resp := reviewTikTokPublishResponse{Status: status, PrivacyLevel: privacyLevel, VideoURL: videoURL}
	if result != nil {
		resp.ExternalID = result.ExternalID
		resp.URL = result.URL
	}
	h.recordReviewPublishEvent(r.Context(), session, "review_publish_completed", "Published TikTok review test video", map[string]any{
		"status":        resp.Status,
		"external_id":   resp.ExternalID,
		"url":           resp.URL,
		"privacy_level": resp.PrivacyLevel,
		"video_url":     resp.VideoURL,
	})
	writeSuccess(w, resp)
}

func (h *ReviewHandler) getReviewTikTokAdapter() (reviewTikTokAdapter, error) {
	if h.tiktokAdapter != nil {
		return h.tiktokAdapter, nil
	}
	adapter, err := platform.Get(reviewDefaultPlatform)
	if err != nil {
		return nil, err
	}
	tiktok, ok := adapter.(reviewTikTokAdapter)
	if !ok {
		return nil, fmt.Errorf("tiktok adapter does not expose review publishing methods")
	}
	return tiktok, nil
}

func (h *ReviewHandler) connectedReviewTikTokAccount(ctx context.Context, profileID string, session db.ReviewSession) (db.SocialAccount, bool, error) {
	accounts, err := h.store.ListSocialAccountsByProfileFiltered(ctx, db.ListSocialAccountsByProfileFilteredParams{
		ProfileID:      profileID,
		ExternalUserID: pgtype.Text{String: "app-review:" + session.ReviewJobID, Valid: true},
		Platform:       pgtype.Text{String: session.Platform, Valid: session.Platform != ""},
	})
	if err != nil {
		return db.SocialAccount{}, false, err
	}
	if len(accounts) == 0 {
		return db.SocialAccount{}, false, nil
	}
	return accounts[0], true, nil
}

func reviewSessionAccountFromDB(account db.SocialAccount) *reviewSessionAccount {
	name := ""
	if account.AccountName.Valid {
		name = account.AccountName.String
	}
	return &reviewSessionAccount{
		ID:                account.ID,
		AccountName:       name,
		ExternalAccountID: account.ExternalAccountID,
		Scope:             append([]string(nil), account.Scope...),
	}
}

func (h *ReviewHandler) fetchReviewCreatorInfo(ctx context.Context, account db.SocialAccount, adapter reviewTikTokAdapter) (*tiktokCreatorInfoResponse, error) {
	accessToken, err := h.reviewTikTokAccessToken(ctx, account, adapter)
	if err != nil {
		return nil, err
	}
	info, err := adapter.FetchCreatorInfo(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return reviewCreatorInfoResponse(info), nil
}

func (h *ReviewHandler) reviewTikTokAccessToken(ctx context.Context, account db.SocialAccount, adapter reviewTikTokAdapter) (string, error) {
	if h.encryptor == nil {
		return "", fmt.Errorf("review token decryptor is not configured")
	}
	accessToken, err := h.encryptor.Decrypt(account.AccessToken)
	if err != nil {
		return "", err
	}
	if accessToken == "" {
		return "", fmt.Errorf("stored TikTok access token is empty")
	}
	if account.TokenExpiresAt.Valid && account.TokenExpiresAt.Time.Before(time.Now()) && account.RefreshToken.Valid {
		refreshToken, err := h.encryptor.Decrypt(account.RefreshToken.String)
		if err != nil {
			return "", err
		}
		newAccess, newRefresh, expiresAt, err := adapter.RefreshToken(ctx, refreshToken)
		if err != nil || newAccess == "" {
			if err == nil {
				err = fmt.Errorf("TikTok returned an empty refreshed access token")
			}
			return "", err
		}
		encAccess, encErr := h.encryptor.Encrypt(newAccess)
		encRefresh, encRefreshErr := h.encryptor.Encrypt(newRefresh)
		if encErr != nil || encRefreshErr != nil {
			return "", fmt.Errorf("encrypt refreshed TikTok tokens: access=%v refresh=%v", encErr, encRefreshErr)
		}
		accessToken = newAccess
		if err := h.store.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
			ID:             account.ID,
			AccessToken:    encAccess,
			RefreshToken:   pgtype.Text{String: encRefresh, Valid: encRefresh != ""},
			TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: !expiresAt.IsZero()},
		}); err != nil {
			slog.Error("review session: update TikTok tokens failed", "account_id", account.ID, "error", err)
		}
	}
	return accessToken, nil
}

func reviewCreatorInfoResponse(info *platform.TikTokCreatorInfo) *tiktokCreatorInfoResponse {
	if info == nil {
		return nil
	}
	return &tiktokCreatorInfoResponse{
		CreatorAvatarURL:        info.CreatorAvatarURL,
		CreatorUsername:         info.CreatorUsername,
		CreatorNickname:         info.CreatorNickname,
		PrivacyLevelOptions:     append([]string(nil), info.PrivacyLevelOptions...),
		CommentDisabled:         info.CommentDisabled,
		DuetDisabled:            info.DuetDisabled,
		StitchDisabled:          info.StitchDisabled,
		MaxVideoPostDurationSec: info.MaxVideoPostDurationSec,
	}
}

func reviewBoolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func isAllowedTikTokPrivacy(value string) bool {
	for _, allowed := range platform.TikTokPrivacyValues {
		if value == allowed {
			return true
		}
	}
	return false
}

func (h *ReviewHandler) recordReviewPublishEvent(ctx context.Context, session db.ReviewSession, eventType string, message string, metadata map[string]any) {
	encoded, err := marshalReviewJSON(metadata)
	if err != nil {
		slog.Warn("review publish: encode event metadata failed", "job_id", session.ReviewJobID, "error", err)
		encoded = []byte(`{}`)
	}
	if _, err := h.store.CreateReviewJobEvent(ctx, db.CreateReviewJobEventParams{
		ReviewJobID: session.ReviewJobID,
		EventType:   eventType,
		Message:     message,
		Metadata:    encoded,
	}); err != nil {
		slog.Warn("review publish: record event failed", "job_id", session.ReviewJobID, "event_type", eventType, "error", err)
	}
}

func previousReviewPublishResponse(events []db.ReviewJobEvent) (reviewTikTokPublishResponse, bool) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != "review_publish_completed" {
			continue
		}
		var metadata map[string]any
		if err := json.Unmarshal(events[i].Metadata, &metadata); err != nil {
			return reviewTikTokPublishResponse{}, false
		}
		return reviewTikTokPublishResponse{
			Status:       stringFromReviewMetadata(metadata, "status"),
			ExternalID:   stringFromReviewMetadata(metadata, "external_id"),
			URL:          stringFromReviewMetadata(metadata, "url"),
			PrivacyLevel: stringFromReviewMetadata(metadata, "privacy_level"),
			VideoURL:     stringFromReviewMetadata(metadata, "video_url"),
		}, true
	}
	return reviewTikTokPublishResponse{}, false
}

func stringFromReviewMetadata(metadata map[string]any, key string) string {
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
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
		BrowserWindowWidth:  1920,
		BrowserWindowHeight: 1080,
		Plan:                reviewKitDemoPlan(kit),
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

func reviewJobSummaryFromDB(row db.ReviewJob) reviewJobResponse {
	agentVersion := reviewAgentVersion
	if row.AgentVersion.Valid && strings.TrimSpace(row.AgentVersion.String) != "" {
		agentVersion = row.AgentVersion.String
	}
	return reviewJobResponse{
		ID:           row.ID,
		ReviewKitID:  row.ReviewKitID,
		Platform:     row.Platform,
		Status:       row.Status,
		AgentVersion: agentVersion,
	}
}

func selectReviewStateKit(kits []db.ReviewKit) *db.ReviewKit {
	var fallback *db.ReviewKit
	for i := range kits {
		kit := &kits[i]
		if kit.Platform != reviewDefaultPlatform {
			continue
		}
		if fallback == nil {
			fallback = kit
		}
		if kit.Status == reviewKitStatusReady {
			return kit
		}
	}
	return fallback
}

func selectReviewStateDomain(domains []db.ReviewDomain) *db.ReviewDomain {
	var fallback *db.ReviewDomain
	for i := range domains {
		domain := &domains[i]
		if fallback == nil {
			fallback = domain
		}
		if isReviewDomainReady(*domain) {
			return domain
		}
	}
	return fallback
}

func reviewKitProfileID(kit db.ReviewKit) string {
	var snapshot map[string]any
	if len(kit.BrandSnapshot) == 0 || json.Unmarshal(kit.BrandSnapshot, &snapshot) != nil {
		return ""
	}
	profileID, _ := snapshot["profile_id"].(string)
	return strings.TrimSpace(profileID)
}

func reviewKitDemoPlan(kit db.ReviewKit) *reviewtemplate.TikTokDemoPlan {
	var snapshot struct {
		ReviewPlan json.RawMessage `json:"review_plan"`
	}
	if len(kit.BrandSnapshot) == 0 || json.Unmarshal(kit.BrandSnapshot, &snapshot) != nil || len(snapshot.ReviewPlan) == 0 {
		return nil
	}
	var plan reviewtemplate.TikTokDemoPlan
	if err := json.Unmarshal(snapshot.ReviewPlan, &plan); err != nil {
		return nil
	}
	if plan.Platform != reviewDefaultPlatform || len(plan.Segments) == 0 {
		return nil
	}
	return &plan
}

func marshalReviewJSON(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return json.Marshal(value)
}

func defaultReviewDomainChecker(ctx context.Context, domain db.ReviewDomain) reviewDomainCheckResult {
	resolver := net.DefaultResolver
	wantCNAME := strings.Trim(strings.ToLower(domain.CnameTarget), ".")
	gotCNAME, cnameErr := resolver.LookupCNAME(ctx, domain.Domain)
	if cnameErr != nil || strings.Trim(strings.ToLower(gotCNAME), ".") != wantCNAME {
		return reviewDomainCheckResult{Message: "CNAME record has not propagated yet. Point " + domain.Domain + " to " + domain.CnameTarget + "."}
	}
	txtRecords, txtErr := resolver.LookupTXT(ctx, "_unipost-review."+domain.Domain)
	if txtErr != nil {
		return reviewDomainCheckResult{Message: "TXT verification record has not propagated yet."}
	}
	for _, record := range txtRecords {
		if strings.TrimSpace(record) == domain.VerificationToken {
			if message := checkReviewDomainHTTPS(ctx, domain.Domain); message != "" {
				return reviewDomainCheckResult{DNSReady: true, TLSIssued: false, Message: message}
			}
			return reviewDomainCheckResult{DNSReady: true, TLSIssued: true}
		}
	}
	return reviewDomainCheckResult{Message: "TXT verification record does not match the UniPost token."}
}

func checkReviewDomainHTTPS(ctx context.Context, host string) string {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, "https://"+host+"/tiktok/posting", nil)
	if err != nil {
		return "Review app URL could not be prepared for HTTPS verification."
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "DNS is verified, but HTTPS is not ready yet. Wait for certificate issuance and review app routing, then try again."
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return "DNS and TLS responded, but the review app is not reachable yet. Try again after deployment finishes."
	}
	return ""
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

func writeNotFoundOrInternal(w http.ResponseWriter, err error, notFoundMessage string, internalMessage string) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", notFoundMessage)
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", internalMessage)
}
