// media.go is the Sprint 2 media library handler. Three endpoints:
//
//   POST   /v1/media         create a row + return a presigned PUT URL
//   GET    /v1/media/{id}    metadata + a fresh signed download URL
//   DELETE /v1/media/{id}    soft-delete (sweeper hard-deletes later)
//
// The two-step upload flow keeps the API server out of the binary
// path. Clients hit POST first to register the upload, then PUT the
// bytes directly to R2 using the returned presigned URL. The next
// time the media_id appears in a publish call, the API HEADs the R2
// object to confirm it exists and copies the actual size /
// content-type from the response (the "poll-on-attach" hydration
// pattern that replaces R2-side webhooks).

package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

// MediaHandler owns the media library endpoints. Storage is optional —
// when nil (R2 not configured), all endpoints return 503 with a clear
// error so a half-deployed environment fails loudly instead of
// quietly losing uploads.
type MediaHandler struct {
	queries *db.Queries
	storage *storage.Client
}

func NewMediaHandler(queries *db.Queries, store *storage.Client) *MediaHandler {
	return &MediaHandler{queries: queries, storage: store}
}

// MediaSizeHardCap is the global ceiling for any single upload, on
// top of the per-platform soft caps from the capabilities table.
// Set to 25 MB based on Sprint 2 founder hand-off Q4 — covers
// everything except long-form video, keeps R2 storage / egress costs
// predictable.
const MediaSizeHardCap = 25 * 1024 * 1024

// allowedMimeTypes is the union of every adapter's accepted formats.
// We could derive this from the capabilities map but a static set
// is faster and lets us reject obviously wrong types (executables,
// HTML, archives) at the very front door.
var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/jpg":       true,
	"image/png":       true,
	"image/webp":      true,
	"image/gif":       true,
	"image/heic":      true,
	"video/mp4":       true,
	"video/quicktime": true,
	"video/webm":      true,
	"video/x-m4v":     true,
}

type mediaResponse struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	UploadURL   string    `json:"upload_url,omitempty"`
	DownloadURL string    `json:"download_url,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func toMediaResponse(m db.Media) mediaResponse {
	return mediaResponse{
		ID:          m.ID,
		Status:      m.Status,
		ContentType: m.ContentType,
		SizeBytes:   m.SizeBytes,
		CreatedAt:   m.CreatedAt.Time,
	}
}

func (h *MediaHandler) writeExistingMediaResponse(w http.ResponseWriter, r *http.Request, row db.Media) {
	resp := toMediaResponse(row)
	resp.Status = row.Status

	if row.Status == "pending" && h.storage != nil {
		uploadURL, err := h.storage.PresignPut(r.Context(), row.StorageKey, row.ContentType, 15*time.Minute)
		if err != nil {
			slog.Error("media.Create: presign existing upload URL failed", "err", err, "media_id", row.ID)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to presign upload URL")
			return
		}
		resp.UploadURL = uploadURL
		resp.ExpiresAt = time.Now().Add(15 * time.Minute)
	}

	writeSuccess(w, resp)
}

// Create handles POST /v1/media. Validates filename / content_type /
// size, mints a row in `pending` status, and returns a presigned PUT
// URL the client can upload to directly.
func (h *MediaHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, "STORAGE_NOT_CONFIGURED",
			"R2 storage is not configured on this server")
		return
	}

	var body struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
		ContentHash string `json:"content_hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	body.ContentType = strings.ToLower(strings.TrimSpace(body.ContentType))
	if !allowedMimeTypes[body.ContentType] {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			fmt.Sprintf("content_type %q is not allowed", body.ContentType))
		return
	}
	if body.SizeBytes <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "size_bytes must be > 0")
		return
	}
	if body.SizeBytes > MediaSizeHardCap {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			fmt.Sprintf("size_bytes exceeds the global hard cap of %d", MediaSizeHardCap))
		return
	}

	// Dedup: if the client sent a content_hash, check for an existing
	// media row with the same hash in this workspace. Return it directly.
	// If it's already uploaded/attached we can reuse it immediately; if
	// it's still pending we return a fresh presigned PUT URL so retries
	// of the same file can resume instead of failing on the unique index.
	if body.ContentHash != "" {
		existing, err := h.queries.GetActiveMediaByHash(r.Context(), db.GetActiveMediaByHashParams{
			WorkspaceID: workspaceID,
			ContentHash: pgtype.Text{String: body.ContentHash, Valid: true},
		})
		if err == nil && existing.ID != "" {
			h.writeExistingMediaResponse(w, r, existing)
			return
		}
	}

	ext := pickExt(body.Filename, body.ContentType)

	// Insert the row with a *unique* placeholder storage_key. We can't
	// know the real key until the row's id is generated, but the column
	// has a global UNIQUE constraint — using a hardcoded literal here
	// (the original implementation) meant that any orphaned row left
	// behind by a crash between INSERT and the UPDATE below would
	// permanently block every future upload across every workspace
	// with a unique-violation error. A per-request UUID guarantees the
	// placeholder collides with nothing, so a leak only ever loses one
	// row instead of bricking the endpoint.
	placeholderKey := "placeholder/" + uuid.New().String()
	row, err := h.queries.CreateMedia(r.Context(), db.CreateMediaParams{
		WorkspaceID: workspaceID,
		StorageKey:  placeholderKey, // overwritten just below
		ContentType: body.ContentType,
		SizeBytes:   body.SizeBytes,
		Status:      "pending",
		ContentHash: pgtype.Text{String: body.ContentHash, Valid: body.ContentHash != ""},
	})
	if err != nil {
		if body.ContentHash != "" {
			existing, lookupErr := h.queries.GetActiveMediaByHash(r.Context(), db.GetActiveMediaByHashParams{
				WorkspaceID: workspaceID,
				ContentHash: pgtype.Text{String: body.ContentHash, Valid: true},
			})
			if lookupErr == nil && existing.ID != "" {
				h.writeExistingMediaResponse(w, r, existing)
				return
			}
		}
		slog.Error("media.Create: insert row failed", "err", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create media row")
		return
	}

	// Now we know the ID, so we can construct the real storage_key.
	finalKey := storage.MediaKey(row.ID, ext)
	updated, err := h.queries.UpdateMediaStorageKey(r.Context(), db.UpdateMediaStorageKeyParams{
		ID:         row.ID,
		StorageKey: finalKey,
	})
	if err != nil {
		slog.Error("media.Create: update storage_key failed", "err", err, "media_id", row.ID)
		// Cleanup: delete the orphan row so we don't leak it.
		_ = h.queries.HardDeleteMedia(r.Context(), row.ID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to set storage key")
		return
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	uploadURL, err := h.storage.PresignPut(r.Context(), finalKey, body.ContentType, 15*time.Minute)
	if err != nil {
		slog.Error("media.Create: presign upload URL failed", "err", err, "media_id", row.ID)
		_ = h.queries.HardDeleteMedia(r.Context(), row.ID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to presign upload URL")
		return
	}

	resp := toMediaResponse(updated)
	resp.UploadURL = uploadURL
	resp.ExpiresAt = expiresAt
	writeCreated(w, resp)
}

// Get handles GET /v1/media/{id}. Returns the row + a fresh signed
// download URL the caller can use to verify the upload landed.
// Triggers a HEAD-and-hydrate when the row is still in 'pending'
// status — the same poll-on-attach pattern the publish path uses.
func (h *MediaHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	id := chi.URLParam(r, "id")

	row, err := h.queries.GetMediaByIDAndWorkspace(r.Context(), db.GetMediaByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Media not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load media")
		return
	}

	// Lazy hydration on first GET after upload.
	if row.Status == "pending" && h.storage != nil {
		if hydrated, ok := h.tryHydrate(r, row); ok {
			row = hydrated
		}
	}

	resp := toMediaResponse(row)
	if h.storage != nil && row.Status != "pending" {
		if dlURL, err := h.storage.PresignGet(r.Context(), row.StorageKey, 15*time.Minute); err == nil {
			resp.DownloadURL = dlURL
			resp.ExpiresAt = time.Now().Add(15 * time.Minute)
		}
	}
	writeSuccess(w, resp)
}

// Delete handles DELETE /v1/media/{id}. Soft-delete only — the
// sweeper hard-deletes the row + R2 object on its next tick.
func (h *MediaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.queries.SoftDeleteMedia(r.Context(), db.SoftDeleteMediaParams{
		ID:          id,
		WorkspaceID: workspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete media")
		return
	}
	writeSuccess(w, map[string]bool{"deleted": true})
}

// tryHydrate is the publish-path equivalent of an R2 upload-complete
// webhook. HEAD the object; on a successful response, copy the
// authoritative size + content type from R2 into the row and flip
// status to 'uploaded'. Returns the updated row + true on success,
// or the original row + false on any failure.
//
// Failures here are SOFT — we don't propagate them, because the
// caller (Get / publish path) needs to handle the not-yet-uploaded
// case anyway.
func (h *MediaHandler) tryHydrate(r *http.Request, row db.Media) (db.Media, bool) {
	if h.storage == nil {
		return row, false
	}
	head, err := h.storage.Head(r.Context(), row.StorageKey)
	if err != nil || !head.Exists {
		return row, false
	}
	updated, err := h.queries.MarkMediaUploaded(r.Context(), db.MarkMediaUploadedParams{
		ID:          row.ID,
		SizeBytes:   head.SizeBytes,
		ContentType: pickContentType(head.ContentType, row.ContentType),
	})
	if err != nil {
		return row, false
	}
	return updated, true
}

// pickExt returns a sensible file extension for the storage key.
// Prefers the filename's extension when present, falls back to one
// derived from the Content-Type header.
func pickExt(filename, contentType string) string {
	if ext := strings.ToLower(path.Ext(filename)); ext != "" {
		return ext
	}
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/heic":
		return ".heic"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "video/webm":
		return ".webm"
	}
	return ".bin"
}

// pickContentType prefers the value R2 reports for the actual object
// over whatever the client originally claimed. R2 echoes back what the
// client uploaded, so this is normally identical, but it guards against
// a client that lied at create-time.
func pickContentType(fromR2, fromClient string) string {
	if fromR2 != "" {
		return fromR2
	}
	return fromClient
}

// _ unused but kept for the lint linter to leave the import alive
// while pgtype only references it from the generated code.
var _ = pgtype.Text{}
