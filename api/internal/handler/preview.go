// preview.go houses the Sprint 2 hosted preview endpoints. Two halves:
//
//   POST /v1/social-posts/{id}/preview-link  → API key auth, returns
//     a signed preview URL valid for 24h. Draft only.
//
//   GET  /v1/public/drafts/{id}              → no auth (token in query
//     param). Returns the draft + resolved media URLs in a shape the
//     dashboard preview page can render directly.
//
// The dashboard route /preview/[id] in the dashboard app fetches the
// public endpoint and renders one column per platform. See B3 for
// the decision to host the page on app.unipost.dev rather than a
// dedicated preview.unipost.dev subdomain.

package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

// PreviewHandler owns the two preview endpoints. Holds the storage
// client because the public endpoint resolves media_ids to fresh
// presigned download URLs the dashboard can <img src=...> directly.
type PreviewHandler struct {
	queries     *db.Queries
	storage     *storage.Client
	jwtSecret   []byte
	publicAppURL string
}

// NewPreviewHandler constructs the handler. publicAppURL is the
// dashboard origin (e.g. https://app.unipost.dev) that preview-link
// URLs are built against. jwtSecret is the raw HMAC key — main.go
// passes the ENCRYPTION_KEY value here.
func NewPreviewHandler(queries *db.Queries, store *storage.Client, jwtSecret []byte, publicAppURL string) *PreviewHandler {
	if publicAppURL == "" {
		publicAppURL = "https://app.unipost.dev"
	}
	return &PreviewHandler{
		queries:      queries,
		storage:      store,
		jwtSecret:    jwtSecret,
		publicAppURL: publicAppURL,
	}
}

type previewLinkResponse struct {
	URL       string    `json:"url"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateLink handles POST /v1/social-posts/{id}/preview-link.
// Drafts only — returns 409 for non-drafts so a published post can't
// be re-shared via preview link (use the platform URLs from the
// publish response instead).
func (h *PreviewHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	projectID := auth.GetWorkspaceID(r.Context())
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}
	postID := chi.URLParam(r, "id")

	post, err := h.queries.GetSocialPostByIDAndWorkspace(r.Context(), db.GetSocialPostByIDAndWorkspaceParams{
		ID:        postID,
		WorkspaceID: projectID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load post")
		return
	}
	if post.Status != "draft" {
		writeError(w, http.StatusConflict, "CONFLICT", "Preview links are only available for drafts")
		return
	}

	token, expires, err := signPreviewToken(post.ID, h.jwtSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to sign preview token")
		return
	}

	url := h.publicAppURL + "/preview/" + post.ID + "?token=" + token
	writeSuccess(w, previewLinkResponse{
		URL:       url,
		Token:     token,
		ExpiresAt: expires,
	})
}

// publicDraftPlatformPost is the per-account half of the public
// response. Includes the resolved media URLs the dashboard can render
// inline + a per-platform character count derived from the platform's
// caption max length (the naive rune count is approximate; see PRD
// review S5).
type publicDraftPlatformPost struct {
	AccountID      string   `json:"account_id"`
	Platform       string   `json:"platform"`
	AccountName    string   `json:"account_name,omitempty"`
	Caption        string   `json:"caption"`
	CaptionLength  int      `json:"caption_length"`
	CaptionMax     int      `json:"caption_max"`
	MediaURLs      []string `json:"media_urls"`
	ThreadPosition int      `json:"thread_position,omitempty"`
}

type publicDraftResponse struct {
	PostID         string                    `json:"post_id"`
	Status         string                    `json:"status"`
	CreatedAt      time.Time                 `json:"created_at"`
	ScheduledAt    *time.Time                `json:"scheduled_at,omitempty"`
	PlatformPosts  []publicDraftPlatformPost `json:"platform_posts"`
}

// PublicGet handles GET /v1/public/drafts/{id}?token=... — no auth,
// signature verification gates access. The token contains the post_id
// to prevent a token issued for one draft from reading another (we
// match the URL param against the verified payload).
func (h *PreviewHandler) PublicGet(w http.ResponseWriter, r *http.Request) {
	urlPostID := chi.URLParam(r, "id")
	token := r.URL.Query().Get("token")

	verifiedPostID, err := verifyPreviewToken(token, h.jwtSecret)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
		return
	}
	if verifiedPostID != urlPostID {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "token does not match post id")
		return
	}

	// Load the post WITHOUT a project filter — preview tokens are the
	// authorization, not the project context. The token signature is
	// what proves the caller is allowed to see this draft.
	post, err := h.queries.GetSocialPostByID(r.Context(), urlPostID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Draft not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load draft")
		return
	}

	fallbackCaption := ""
	if post.Caption.Valid {
		fallbackCaption = post.Caption.String
	}
	posts, err := platform.DecodePostMetadata(post.Metadata, fallbackCaption)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decode metadata")
		return
	}

	// Resolve account names + platforms in one DB hit so the response
	// shows real handles in each preview column.
	allAccounts, _ := h.queries.ListAllSocialAccountsByProfile(r.Context(), post.WorkspaceID)
	accountInfo := make(map[string]struct {
		Platform string
		Name     string
	}, len(allAccounts))
	for _, a := range allAccounts {
		name := ""
		if a.AccountName.Valid {
			name = a.AccountName.String
		}
		accountInfo[a.ID] = struct {
			Platform string
			Name     string
		}{Platform: a.Platform, Name: name}
	}

	resp := publicDraftResponse{
		PostID:    post.ID,
		Status:    post.Status,
		CreatedAt: post.CreatedAt.Time,
	}
	if post.ScheduledAt.Valid {
		t := post.ScheduledAt.Time
		resp.ScheduledAt = &t
	}

	for _, pp := range posts {
		info := accountInfo[pp.AccountID]
		platName := info.Platform

		// Resolve media: stitch together direct URLs + media_id-derived
		// presigned downloads. The 15-min TTL on the presigned URLs
		// matches the rest of the publish path.
		mediaURLs := append([]string(nil), pp.MediaURLs...)
		if h.storage != nil {
			for _, mid := range pp.MediaIDs {
				row, mErr := h.queries.GetMedia(r.Context(), mid)
				if mErr != nil || row.Status == "deleted" {
					continue
				}
				dlURL, dlErr := h.storage.PresignGet(r.Context(), row.StorageKey, 15*time.Minute)
				if dlErr == nil {
					mediaURLs = append(mediaURLs, dlURL)
				}
			}
		}

		captionMax := 0
		if cap, ok := platform.CapabilityFor(platName); ok {
			captionMax = cap.Text.MaxLength
		}

		resp.PlatformPosts = append(resp.PlatformPosts, publicDraftPlatformPost{
			AccountID:      pp.AccountID,
			Platform:       platName,
			AccountName:    info.Name,
			Caption:        pp.Caption,
			CaptionLength:  len([]rune(pp.Caption)),
			CaptionMax:     captionMax,
			MediaURLs:      mediaURLs,
			ThreadPosition: pp.ThreadPosition,
		})
	}

	// Cache headers — let CDNs / browsers cache the preview body for
	// a short window so refreshing the page doesn't re-hit the API,
	// but make sure it expires before the JWT does.
	w.Header().Set("Cache-Control", "private, max-age=60")
	writeSuccess(w, resp)
}
