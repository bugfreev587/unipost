package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

const (
	audioOverlayKind                 = "audio_overlay"
	audioOverlayStatusQueued         = "queued"
	audioOverlayModeMix              = "mix"
	audioOverlayModeReplace          = "replace"
	audioOverlayFitTrimToVideo       = "trim_to_video"
	audioOverlayFitLoopToVideo       = "loop_to_video"
	audioOverlayMaxVideoSizeBytes    = 500 * 1024 * 1024
	audioOverlayMaxAudioSizeBytes    = 100 * 1024 * 1024
	audioOverlayMaxVideoDurationMS   = 10 * 60 * 1000
	audioOverlayInputHoldWindow      = 2 * time.Hour
	audioOverlayDefaultVideoVolume   = 100
	audioOverlayDefaultAudioVolume   = 100
	audioOverlayDefaultAudioStartMS  = 0
	audioOverlayDocsURL              = "https://unipost.dev/docs/api/media/audio-overlays"
	audioOverlayIdempotencyErrorCode = "IDEMPOTENCY_CONFLICT"
)

type mediaAudioOverlayQueries interface {
	GetMediaByIDAndWorkspace(context.Context, db.GetMediaByIDAndWorkspaceParams) (db.Media, error)
	MarkMediaUploaded(context.Context, db.MarkMediaUploadedParams) (db.Media, error)
	GetMediaProcessingJobByIdempotencyKey(context.Context, db.GetMediaProcessingJobByIdempotencyKeyParams) (db.MediaProcessingJob, error)
	CreateMediaProcessingJob(context.Context, db.CreateMediaProcessingJobParams) (db.MediaProcessingJob, error)
	GetMediaProcessingJobByIDAndWorkspace(context.Context, db.GetMediaProcessingJobByIDAndWorkspaceParams) (db.MediaProcessingJob, error)
	ScheduleMediaCleanup(context.Context, db.ScheduleMediaCleanupParams) error
}

type mediaAudioOverlayObjectStore interface {
	Head(context.Context, string) (storage.HeadResult, error)
	ProbeVideo(context.Context, string) (storage.VideoMetadata, error)
}

type MediaAudioOverlayHandler struct {
	queries     mediaAudioOverlayQueries
	objectStore mediaAudioOverlayObjectStore
	holdWindow  time.Duration
}

func NewMediaAudioOverlayHandler(queries mediaAudioOverlayQueries, objectStore mediaAudioOverlayObjectStore) *MediaAudioOverlayHandler {
	return &MediaAudioOverlayHandler{
		queries:     queries,
		objectStore: objectStore,
		holdWindow:  audioOverlayInputHoldWindow,
	}
}

type audioOverlayCreateRequest struct {
	VideoMediaID string `json:"video_media_id"`
	AudioMediaID string `json:"audio_media_id"`
	Mode         string `json:"mode"`
	VideoVolume  *int32 `json:"video_volume"`
	AudioVolume  *int32 `json:"audio_volume"`
	AudioStartMs *int32 `json:"audio_start_ms"`
	Fit          string `json:"fit"`
}

type normalizedAudioOverlayRequest struct {
	VideoMediaID string `json:"video_media_id"`
	AudioMediaID string `json:"audio_media_id"`
	Mode         string `json:"mode"`
	VideoVolume  int32  `json:"video_volume"`
	AudioVolume  int32  `json:"audio_volume"`
	AudioStartMs int32  `json:"audio_start_ms"`
	Fit          string `json:"fit"`
}

type mediaAudioOverlayResponse struct {
	ID            string                  `json:"id"`
	Status        string                  `json:"status"`
	VideoMediaID  string                  `json:"video_media_id"`
	AudioMediaID  string                  `json:"audio_media_id"`
	OutputMediaID *string                 `json:"output_media_id"`
	Mode          string                  `json:"mode"`
	Fit           string                  `json:"fit"`
	CreatedAt     time.Time               `json:"created_at"`
	StartedAt     *time.Time              `json:"started_at"`
	CompletedAt   *time.Time              `json:"completed_at"`
	Error         *mediaAudioOverlayError `json:"error"`
}

type mediaAudioOverlayError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (h *MediaAudioOverlayHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h.queries == nil {
		writeError(w, http.StatusServiceUnavailable, "STORAGE_NOT_CONFIGURED", "Media processing is not configured on this server")
		return
	}

	var body audioOverlayCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	normalized, issues := normalizeAudioOverlayRequest(body)
	if len(issues) > 0 {
		writeAudioOverlayValidationError(w, issues)
		return
	}
	requestHash, requestJSON, err := audioOverlayRequestHash(normalized)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to normalize audio overlay request")
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey != "" {
		existing, err := h.queries.GetMediaProcessingJobByIdempotencyKey(r.Context(), db.GetMediaProcessingJobByIdempotencyKeyParams{
			WorkspaceID:    workspaceID,
			IdempotencyKey: pgtype.Text{String: idempotencyKey, Valid: true},
		})
		if err == nil {
			if existing.RequestHash.Valid && existing.RequestHash.String == requestHash {
				writeAccepted(w, audioOverlayJobResponse(existing))
				return
			}
			writeError(w, http.StatusConflict, audioOverlayIdempotencyErrorCode, "Idempotency-Key was already used with a different audio overlay request")
			return
		}
		if err != pgx.ErrNoRows {
			slog.Error("media audio overlay: idempotency lookup failed", "err", err, "workspace_id", workspaceID)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check idempotency key")
			return
		}
	}

	video, audio, ok := h.loadAndValidateInputs(w, r, workspaceID, normalized)
	if !ok {
		return
	}
	if ok := h.verifyInputObjects(w, r, video, audio); !ok {
		return
	}
	if ok := h.holdInputMedia(w, r, video.ID, audio.ID); !ok {
		return
	}

	params := db.CreateMediaProcessingJobParams{
		WorkspaceID:       workspaceID,
		Kind:              audioOverlayKind,
		Status:            audioOverlayStatusQueued,
		InputVideoMediaID: normalized.VideoMediaID,
		InputAudioMediaID: normalized.AudioMediaID,
		OutputMediaID:     pgtype.Text{},
		Mode:              normalized.Mode,
		Fit:               normalized.Fit,
		VideoVolume:       normalized.VideoVolume,
		AudioVolume:       normalized.AudioVolume,
		AudioStartMs:      normalized.AudioStartMs,
		RequestJson:       requestJSON,
	}
	if idempotencyKey != "" {
		params.IdempotencyKey = pgtype.Text{String: idempotencyKey, Valid: true}
		params.RequestHash = pgtype.Text{String: requestHash, Valid: true}
	}

	job, err := h.queries.CreateMediaProcessingJob(r.Context(), params)
	if err != nil {
		slog.Error("media audio overlay: create job failed", "err", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create audio overlay job")
		return
	}

	writeAccepted(w, audioOverlayJobResponse(job))
}

func (h *MediaAudioOverlayHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h.queries == nil {
		writeError(w, http.StatusServiceUnavailable, "STORAGE_NOT_CONFIGURED", "Media processing is not configured on this server")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/v1/media/audio-overlays/")
	}
	job, err := h.queries.GetMediaProcessingJobByIDAndWorkspace(r.Context(), db.GetMediaProcessingJobByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Audio overlay job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load audio overlay job")
		return
	}

	writeSuccess(w, audioOverlayJobResponse(job))
}

func normalizeAudioOverlayRequest(body audioOverlayCreateRequest) (normalizedAudioOverlayRequest, []platform.Issue) {
	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	if mode == "" {
		mode = audioOverlayModeMix
	}
	fit := strings.ToLower(strings.TrimSpace(body.Fit))
	if fit == "" {
		fit = audioOverlayFitTrimToVideo
	}

	videoVolume := int32(audioOverlayDefaultVideoVolume)
	if body.VideoVolume != nil {
		videoVolume = *body.VideoVolume
	}
	audioVolume := int32(audioOverlayDefaultAudioVolume)
	if body.AudioVolume != nil {
		audioVolume = *body.AudioVolume
	}
	audioStartMs := int32(audioOverlayDefaultAudioStartMS)
	if body.AudioStartMs != nil {
		audioStartMs = *body.AudioStartMs
	}

	var issues []platform.Issue
	if strings.TrimSpace(body.VideoMediaID) == "" {
		issues = append(issues, audioOverlayIssue("video_media_id", "video_media_id_required", "video_media_id is required"))
	}
	if strings.TrimSpace(body.AudioMediaID) == "" {
		issues = append(issues, audioOverlayIssue("audio_media_id", "audio_media_id_required", "audio_media_id is required"))
	}
	if mode != audioOverlayModeMix && mode != audioOverlayModeReplace {
		issues = append(issues, audioOverlayIssue("mode", "invalid_audio_overlay_mode", "mode must be mix or replace"))
	}
	if fit != audioOverlayFitTrimToVideo && fit != audioOverlayFitLoopToVideo {
		issues = append(issues, audioOverlayIssue("fit", "invalid_audio_overlay_fit", "fit must be trim_to_video or loop_to_video"))
	}
	if videoVolume < 0 || videoVolume > 100 {
		issues = append(issues, audioOverlayIssue("video_volume", "invalid_audio_overlay_volume", "video_volume must be between 0 and 100"))
	}
	if audioVolume < 0 || audioVolume > 100 {
		issues = append(issues, audioOverlayIssue("audio_volume", "invalid_audio_overlay_volume", "audio_volume must be between 0 and 100"))
	}
	if audioStartMs < 0 {
		issues = append(issues, audioOverlayIssue("audio_start_ms", "invalid_audio_overlay_offset", "audio_start_ms must be greater than or equal to 0"))
	}

	return normalizedAudioOverlayRequest{
		VideoMediaID: strings.TrimSpace(body.VideoMediaID),
		AudioMediaID: strings.TrimSpace(body.AudioMediaID),
		Mode:         mode,
		VideoVolume:  videoVolume,
		AudioVolume:  audioVolume,
		AudioStartMs: audioStartMs,
		Fit:          fit,
	}, issues
}

func audioOverlayRequestHash(req normalizedAudioOverlayRequest) (string, []byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), body, nil
}

func (h *MediaAudioOverlayHandler) loadAndValidateInputs(w http.ResponseWriter, r *http.Request, workspaceID string, req normalizedAudioOverlayRequest) (db.Media, db.Media, bool) {
	video, ok := h.loadOverlayMedia(w, r, workspaceID, req.VideoMediaID, "video_media_id")
	if !ok {
		return db.Media{}, db.Media{}, false
	}
	audio, ok := h.loadOverlayMedia(w, r, workspaceID, req.AudioMediaID, "audio_media_id")
	if !ok {
		return db.Media{}, db.Media{}, false
	}

	video = h.hydratePendingOverlayMedia(r, video)
	audio = h.hydratePendingOverlayMedia(r, audio)

	if !isUsableOverlayMediaStatus(video.Status) {
		writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayMediaNotUploadedIssue("video_media_id", video)})
		return db.Media{}, db.Media{}, false
	}
	if !isUsableOverlayMediaStatus(audio.Status) {
		writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayMediaNotUploadedIssue("audio_media_id", audio)})
		return db.Media{}, db.Media{}, false
	}
	if MediaKind := platform.MediaFromContentType(video.ContentType).Kind; MediaKind != platform.MediaKindVideo {
		writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayIssue("video_media_id", "video_stream_required", "video_media_id must reference a video media asset")})
		return db.Media{}, db.Media{}, false
	}
	audioKind := platform.MediaFromContentType(audio.ContentType).Kind
	if audioKind != platform.MediaKindAudio && audioKind != platform.MediaKindVideo {
		writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayIssue("audio_media_id", "audio_stream_required", "audio_media_id must reference an audio asset or a video with decodable audio")})
		return db.Media{}, db.Media{}, false
	}
	if video.SizeBytes > audioOverlayMaxVideoSizeBytes {
		writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayIssue("video_media_id", "media_size_exceeded", fmt.Sprintf("video_media_id exceeds the %s input video limit", audioOverlayFormatBytes(audioOverlayMaxVideoSizeBytes)))})
		return db.Media{}, db.Media{}, false
	}
	if audio.SizeBytes > audioOverlayMaxAudioSizeBytes {
		writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayIssue("audio_media_id", "media_size_exceeded", fmt.Sprintf("audio_media_id exceeds the %s input audio limit", audioOverlayFormatBytes(audioOverlayMaxAudioSizeBytes)))})
		return db.Media{}, db.Media{}, false
	}
	if video.DurationMs.Valid && video.DurationMs.Int32 > audioOverlayMaxVideoDurationMS {
		writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayIssue("video_media_id", "media_processing_limit_exceeded", "video_media_id exceeds the 10 minute audio overlay duration limit")})
		return db.Media{}, db.Media{}, false
	}
	if audio.DurationMs.Valid && req.AudioStartMs >= audio.DurationMs.Int32 {
		writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayIssue("audio_start_ms", "invalid_audio_overlay_offset", "audio_start_ms must be less than the audio duration")})
		return db.Media{}, db.Media{}, false
	}
	return video, audio, true
}

func (h *MediaAudioOverlayHandler) hydratePendingOverlayMedia(r *http.Request, row db.Media) db.Media {
	if row.Status != "pending" {
		return row
	}
	hydrated, ok := hydrateMediaRow(r.Context(), h.queries, h.objectStore, row)
	if !ok {
		return row
	}
	return hydrated
}

func (h *MediaAudioOverlayHandler) loadOverlayMedia(w http.ResponseWriter, r *http.Request, workspaceID, mediaID, field string) (db.Media, bool) {
	media, err := h.queries.GetMediaByIDAndWorkspace(r.Context(), db.GetMediaByIDAndWorkspaceParams{
		ID:          mediaID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayIssue(field, "media_not_found", field+" was not found in this workspace")})
			return db.Media{}, false
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load media")
		return db.Media{}, false
	}
	return media, true
}

func (h *MediaAudioOverlayHandler) verifyInputObjects(w http.ResponseWriter, r *http.Request, video, audio db.Media) bool {
	if h.objectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "STORAGE_NOT_CONFIGURED", "R2 storage is not configured on this server")
		return false
	}
	for _, item := range []struct {
		field string
		row   db.Media
	}{
		{field: "video_media_id", row: video},
		{field: "audio_media_id", row: audio},
	} {
		head, err := h.objectStore.Head(r.Context(), item.row.StorageKey)
		if err != nil || !head.Exists {
			writeAudioOverlayValidationError(w, []platform.Issue{audioOverlayIssue(item.field, "input_media_unavailable", item.field+" object is no longer available in storage")})
			return false
		}
	}
	return true
}

func (h *MediaAudioOverlayHandler) holdInputMedia(w http.ResponseWriter, r *http.Request, mediaIDs ...string) bool {
	holdUntil := pgtype.Timestamptz{Time: time.Now().Add(h.holdWindow), Valid: true}
	for _, mediaID := range mediaIDs {
		if err := h.queries.ScheduleMediaCleanup(r.Context(), db.ScheduleMediaCleanupParams{
			ID:             mediaID,
			CleanupAfterAt: holdUntil,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to hold input media for processing")
			return false
		}
	}
	return true
}

func isUsableOverlayMediaStatus(status string) bool {
	return status == "uploaded" || status == "attached"
}

func audioOverlayJobResponse(job db.MediaProcessingJob) mediaAudioOverlayResponse {
	var output *string
	if job.OutputMediaID.Valid {
		v := job.OutputMediaID.String
		output = &v
	}
	var startedAt *time.Time
	if job.StartedAt.Valid {
		t := job.StartedAt.Time
		startedAt = &t
	}
	var completedAt *time.Time
	if job.CompletedAt.Valid {
		t := job.CompletedAt.Time
		completedAt = &t
	}
	var errPayload *mediaAudioOverlayError
	if job.ErrorCode.Valid || job.ErrorMessage.Valid {
		errPayload = &mediaAudioOverlayError{
			Code:      job.ErrorCode.String,
			Message:   job.ErrorMessage.String,
			Retryable: job.Retryable,
		}
	}
	return mediaAudioOverlayResponse{
		ID:            job.ID,
		Status:        job.Status,
		VideoMediaID:  job.InputVideoMediaID,
		AudioMediaID:  job.InputAudioMediaID,
		OutputMediaID: output,
		Mode:          job.Mode,
		Fit:           job.Fit,
		CreatedAt:     job.CreatedAt.Time,
		StartedAt:     startedAt,
		CompletedAt:   completedAt,
		Error:         errPayload,
	}
}

func writeAudioOverlayValidationError(w http.ResponseWriter, issues []platform.Issue) {
	writeErrorWithDetails(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Audio overlay request is invalid", ErrorDetails{
		DocsURL: audioOverlayDocsURL,
		Issues:  issues,
	})
}

func audioOverlayIssue(field, code, message string) platform.Issue {
	return platform.Issue{
		Field:    field,
		Code:     code,
		Message:  message,
		Severity: platform.SeverityError,
	}
}

func audioOverlayMediaNotUploadedIssue(field string, media db.Media) platform.Issue {
	issue := audioOverlayIssue(field, "media_not_uploaded", field+" must reference uploaded media")
	issue.Actual = map[string]any{
		"media_id":     media.ID,
		"media_status": media.Status,
		"next_step":    "PUT bytes to upload_url, then poll GET /v1/media/{media_id} until status=uploaded",
		"docs_url":     "https://unipost.dev/docs/api/media/reserve",
	}
	return issue
}

func audioOverlayFormatBytes(size int64) string {
	const mb = 1024 * 1024
	if size >= mb {
		return fmt.Sprintf("%d MB", size/mb)
	}
	return fmt.Sprintf("%d bytes", size)
}
