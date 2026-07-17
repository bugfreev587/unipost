package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/mediaprocessing"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

const (
	gifConversionKind          = "gif_to_mp4"
	gifConversionOutputProfile = "universal_mp4_v1"
	gifConversionDefaultColor  = "#FFFFFF"
	gifConversionMaxBytes      = int64(50 * 1024 * 1024)
	gifConversionMaxBodyBytes  = int64(16 * 1024)
	gifConversionDocsURL       = "https://unipost.dev/docs/api/media/gif-conversions"
)

var gifConversionColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type mediaGIFConversionQueries interface {
	GetMediaByIDAndWorkspace(context.Context, db.GetMediaByIDAndWorkspaceParams) (db.Media, error)
	MarkMediaUploaded(context.Context, db.MarkMediaUploadedParams) (db.Media, error)
	GetMediaProcessingJobByIDAndWorkspace(context.Context, db.GetMediaProcessingJobByIDAndWorkspaceParams) (db.MediaProcessingJob, error)
	GetMediaProcessingJobByIdempotencyKey(context.Context, db.GetMediaProcessingJobByIdempotencyKeyParams) (db.MediaProcessingJob, error)
}

type mediaGIFConversionObjectStore interface {
	Head(context.Context, string) (storage.HeadResult, error)
}

type MediaGIFConversionHandler struct {
	queries     mediaGIFConversionQueries
	objectStore mediaGIFConversionObjectStore
	admitter    mediaprocessing.GIFAdmitter
}

func NewMediaGIFConversionHandler(queries mediaGIFConversionQueries, objectStore mediaGIFConversionObjectStore, admitter mediaprocessing.GIFAdmitter) *MediaGIFConversionHandler {
	return &MediaGIFConversionHandler{queries: queries, objectStore: objectStore, admitter: admitter}
}

type mediaGIFConversionCreateRequest struct {
	GIFMediaID      string `json:"gif_media_id"`
	BackgroundColor string `json:"background_color"`
}

type normalizedGIFConversionRequest struct {
	GIFMediaID      string `json:"gif_media_id"`
	BackgroundColor string `json:"background_color"`
	OutputProfile   string `json:"output_profile"`
}

type mediaGIFConversionResponse struct {
	ID              string                   `json:"id"`
	Kind            string                   `json:"kind"`
	Status          string                   `json:"status"`
	GIFMediaID      string                   `json:"gif_media_id"`
	BackgroundColor string                   `json:"background_color"`
	OutputProfile   string                   `json:"output_profile"`
	OutputMediaID   *string                  `json:"output_media_id"`
	CreatedAt       time.Time                `json:"created_at"`
	StartedAt       *time.Time               `json:"started_at"`
	CompletedAt     *time.Time               `json:"completed_at"`
	Error           *mediaGIFConversionError `json:"error"`
}

type mediaGIFConversionError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (h *MediaGIFConversionHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h.queries == nil || h.objectStore == nil || h.admitter == nil {
		writeErrorWithDetails(w, http.StatusServiceUnavailable, "media_processing_unavailable", "Media processing is not configured on this server", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}

	var body mediaGIFConversionCreateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, gifConversionMaxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeErrorWithDetails(w, http.StatusUnprocessableEntity, "validation_error", "Invalid request body", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeErrorWithDetails(w, http.StatusUnprocessableEntity, "validation_error", "Invalid request body", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	normalized, code, message := normalizeGIFConversionRequest(body)
	if code != "" {
		writeErrorWithDetails(w, http.StatusUnprocessableEntity, code, message, ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	requestJSON, requestHash, err := gifConversionRequestHash(normalized)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to normalize GIF conversion request")
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey != "" {
		existing, lookupErr := h.queries.GetMediaProcessingJobByIdempotencyKey(r.Context(), db.GetMediaProcessingJobByIdempotencyKeyParams{
			WorkspaceID:    workspaceID,
			IdempotencyKey: pgtype.Text{String: idempotencyKey, Valid: true},
		})
		if lookupErr == nil {
			if existing.Kind == gifConversionKind && existing.RequestHash.Valid && existing.RequestHash.String == requestHash {
				writeAccepted(w, gifConversionJobResponse(existing))
				return
			}
			writeErrorWithDetails(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was already used with a different GIF conversion request", ErrorDetails{DocsURL: gifConversionDocsURL})
			return
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			slog.Error("GIF conversion: idempotency lookup failed", "workspace_id", workspaceID, "error", lookupErr)
			writeErrorWithDetails(w, http.StatusServiceUnavailable, "media_processing_unavailable", "GIF conversion idempotency could not be verified", ErrorDetails{DocsURL: gifConversionDocsURL})
			return
		}
	}

	media, err := h.queries.GetMediaByIDAndWorkspace(r.Context(), db.GetMediaByIDAndWorkspaceParams{ID: normalized.GIFMediaID, WorkspaceID: workspaceID})
	if err != nil || media.Status == "deleted" {
		if err == nil || errors.Is(err, pgx.ErrNoRows) {
			writeErrorWithDetails(w, http.StatusNotFound, "media_not_found", "GIF media was not found in this workspace", ErrorDetails{DocsURL: gifConversionDocsURL})
			return
		}
		slog.Error("GIF conversion: load input media", "workspace_id", workspaceID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load GIF media")
		return
	}
	head, err := h.objectStore.Head(r.Context(), media.StorageKey)
	if err != nil {
		slog.Error("GIF conversion: HEAD input media", "workspace_id", workspaceID, "media_id", media.ID, "error", err)
		writeErrorWithDetails(w, http.StatusServiceUnavailable, "media_processing_unavailable", "Media storage could not verify the GIF input", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	if !head.Exists {
		writeErrorWithDetails(w, http.StatusConflict, "input_media_unavailable", "GIF upload is incomplete or no longer available", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	actualContentType := normalizedMediaContentType(head.ContentType)
	if actualContentType != "image/gif" {
		writeErrorWithDetails(w, http.StatusUnprocessableEntity, "gif_media_required", "gif_media_id must reference an uploaded image/gif asset", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	if head.SizeBytes <= 0 || head.SizeBytes > gifConversionMaxBytes {
		writeErrorWithDetails(w, http.StatusUnprocessableEntity, "gif_size_exceeded", "GIF input must be no larger than 50 MB", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	if media.Status == "pending" {
		media, err = h.queries.MarkMediaUploaded(r.Context(), db.MarkMediaUploadedParams{
			ID: media.ID, SizeBytes: head.SizeBytes, ContentType: actualContentType,
			Width: pgtype.Int4{}, Height: pgtype.Int4{}, DurationMs: pgtype.Int4{},
		})
		if err != nil {
			slog.Error("GIF conversion: hydrate input media", "workspace_id", workspaceID, "media_id", media.ID, "error", err)
			writeErrorWithDetails(w, http.StatusServiceUnavailable, "media_processing_unavailable", "GIF input could not be hydrated", ErrorDetails{DocsURL: gifConversionDocsURL})
			return
		}
	}
	if media.Status != "uploaded" && media.Status != "attached" {
		writeErrorWithDetails(w, http.StatusConflict, "input_media_unavailable", "GIF media is not available for processing", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}

	result, err := h.admitter.AdmitGIF(r.Context(), mediaprocessing.GIFAdmissionRequest{
		WorkspaceID: workspaceID, InputMediaID: normalized.GIFMediaID,
		RequestJSON: requestJSON, RequestHash: requestHash,
		IdempotencyKey: idempotencyKey,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		slog.Error("GIF conversion: admission failed", "workspace_id", workspaceID, "error", err)
		writeErrorWithDetails(w, http.StatusServiceUnavailable, "media_processing_unavailable", "GIF conversion could not be queued", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	switch result.Decision.Code {
	case mediaprocessing.AdmissionAccepted, mediaprocessing.AdmissionIdempotentReplay:
		writeAccepted(w, gifConversionJobResponse(result.Job))
	case mediaprocessing.AdmissionIdempotentConflict:
		writeErrorWithDetails(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was already used with a different GIF conversion request", ErrorDetails{DocsURL: gifConversionDocsURL})
	case mediaprocessing.AdmissionCapacityExceeded:
		writeGIFAdmissionLimit(w, result.Decision, "media_processing_capacity_exceeded", "Workspace active media processing capacity is reached")
	case mediaprocessing.AdmissionGIFRateExceeded:
		writeGIFAdmissionLimit(w, result.Decision, "gif_conversion_rate_limit_exceeded", "Workspace GIF conversion limit for the rolling 24-hour window is reached")
	default:
		writeErrorWithDetails(w, http.StatusServiceUnavailable, "media_processing_unavailable", "GIF conversion could not be queued", ErrorDetails{DocsURL: gifConversionDocsURL})
	}
}

func (h *MediaGIFConversionHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h.queries == nil {
		writeErrorWithDetails(w, http.StatusServiceUnavailable, "media_processing_unavailable", "Media processing is not configured on this server", ErrorDetails{DocsURL: gifConversionDocsURL})
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/v1/media/gif-conversions/")
	}
	job, err := h.queries.GetMediaProcessingJobByIDAndWorkspace(r.Context(), db.GetMediaProcessingJobByIDAndWorkspaceParams{ID: id, WorkspaceID: workspaceID})
	if err != nil || job.Kind != gifConversionKind {
		if err == nil || errors.Is(err, pgx.ErrNoRows) {
			writeErrorWithDetails(w, http.StatusNotFound, "not_found", "GIF conversion not found", ErrorDetails{DocsURL: gifConversionDocsURL})
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load GIF conversion")
		return
	}
	writeSuccess(w, gifConversionJobResponse(job))
}

func normalizeGIFConversionRequest(body mediaGIFConversionCreateRequest) (normalizedGIFConversionRequest, string, string) {
	mediaID := strings.TrimSpace(body.GIFMediaID)
	if mediaID == "" {
		return normalizedGIFConversionRequest{}, "gif_media_required", "gif_media_id is required"
	}
	color := strings.TrimSpace(body.BackgroundColor)
	if color == "" {
		color = gifConversionDefaultColor
	}
	if !gifConversionColorPattern.MatchString(color) {
		return normalizedGIFConversionRequest{}, "invalid_background_color", "background_color must be a six-digit #RRGGBB value"
	}
	return normalizedGIFConversionRequest{GIFMediaID: mediaID, BackgroundColor: strings.ToUpper(color), OutputProfile: gifConversionOutputProfile}, "", ""
}

func gifConversionRequestHash(request normalizedGIFConversionRequest) ([]byte, string, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(requestJSON)
	return requestJSON, hex.EncodeToString(hash[:]), nil
}

func normalizedMediaContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
}

func gifConversionJobResponse(job db.MediaProcessingJob) mediaGIFConversionResponse {
	request := normalizedGIFConversionRequest{OutputProfile: gifConversionOutputProfile, BackgroundColor: gifConversionDefaultColor}
	_ = json.Unmarshal(job.Request, &request)
	if request.GIFMediaID == "" && job.InputMediaID.Valid {
		request.GIFMediaID = job.InputMediaID.String
	}
	status := job.Status
	if status == "retry_wait" {
		status = "queued"
	}
	response := mediaGIFConversionResponse{
		ID: job.ID, Kind: gifConversionKind, Status: status,
		GIFMediaID: request.GIFMediaID, BackgroundColor: request.BackgroundColor, OutputProfile: request.OutputProfile,
	}
	if job.OutputMediaID.Valid && job.Status == "succeeded" {
		outputID := job.OutputMediaID.String
		response.OutputMediaID = &outputID
	}
	if job.CreatedAt.Valid {
		response.CreatedAt = job.CreatedAt.Time
	}
	if job.StartedAt.Valid {
		started := job.StartedAt.Time
		response.StartedAt = &started
	}
	if job.CompletedAt.Valid {
		completed := job.CompletedAt.Time
		response.CompletedAt = &completed
	}
	if job.Status == "failed" && job.ErrorCode.Valid {
		response.Error = &mediaGIFConversionError{Code: job.ErrorCode.String, Message: job.ErrorMessage.String, Retryable: job.Retryable}
	}
	return response
}

func writeGIFAdmissionLimit(w http.ResponseWriter, decision mediaprocessing.AdmissionDecision, code, message string) {
	seconds := int(math.Ceil(decision.RetryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	details := map[string]any{}
	if !decision.ResetAt.IsZero() {
		details["reset_at"] = decision.ResetAt.UTC().Format(time.RFC3339)
	}
	writeErrorWithDetails(w, http.StatusTooManyRequests, code, message, ErrorDetails{DocsURL: gifConversionDocsURL, Details: details})
}
