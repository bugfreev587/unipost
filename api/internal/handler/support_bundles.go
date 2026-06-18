package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

const maxSupportBundleReportBytes = 256 * 1024

type supportBundleStore interface {
	CreateSupportBundle(context.Context, db.CreateSupportBundleParams) (db.SupportBundle, error)
	ListAdminSupportBundles(context.Context, db.ListAdminSupportBundlesParams) ([]db.ListAdminSupportBundlesRow, error)
	GetAdminSupportBundle(context.Context, string) (db.GetAdminSupportBundleRow, error)
}

type SupportBundleHandler struct {
	queries supportBundleStore
}

func NewSupportBundleHandler(queries supportBundleStore) *SupportBundleHandler {
	return &SupportBundleHandler{queries: queries}
}

type supportBundleCreateRequest struct {
	SchemaVersion    string          `json:"schema_version"`
	RunID            string          `json:"run_id"`
	CliVersion       string          `json:"cli_version"`
	Summary          string          `json:"summary"`
	ReportMarkdown   string          `json:"report_markdown"`
	Payload          json.RawMessage `json:"payload"`
	FindingCount     int32           `json:"finding_count"`
	RecentErrorCount int32           `json:"recent_error_count"`
}

type supportBundleResponse struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspace_id"`
	WorkspaceName    string    `json:"workspace_name,omitempty"`
	OwnerEmail       string    `json:"owner_email,omitempty"`
	PlanID           string    `json:"plan_id,omitempty"`
	RunID            string    `json:"run_id"`
	SchemaVersion    string    `json:"schema_version"`
	CliVersion       string    `json:"cli_version,omitempty"`
	Summary          string    `json:"summary"`
	ReportMarkdown   string    `json:"report_markdown,omitempty"`
	FindingCount     int32     `json:"finding_count"`
	RecentErrorCount int32     `json:"recent_error_count"`
	CreatedAt        time.Time `json:"created_at"`
}

func (h *SupportBundleHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	var body supportBundleCreateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSupportBundleReportBytes+32*1024))
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid support bundle payload")
		return
	}

	body.SchemaVersion = strings.TrimSpace(body.SchemaVersion)
	body.RunID = strings.TrimSpace(body.RunID)
	body.CliVersion = strings.TrimSpace(body.CliVersion)
	body.Summary = strings.TrimSpace(body.Summary)
	if body.SchemaVersion != "doctor.v1" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "schema_version must be doctor.v1")
		return
	}
	if body.RunID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "run_id is required")
		return
	}
	if strings.TrimSpace(body.ReportMarkdown) == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "report_markdown is required")
		return
	}
	if len([]byte(body.ReportMarkdown)) > maxSupportBundleReportBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "support bundle report_markdown must be 256KB or smaller")
		return
	}
	if body.Summary == "" {
		body.Summary = "UniPost support bundle"
	}

	report := redactSupportBundleMarkdown(body.ReportMarkdown)
	payload := body.Payload
	if len(payload) == 0 || string(payload) == "null" {
		payload = []byte(`{}`)
	} else {
		var decoded any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "payload must be valid JSON")
			return
		}
		payload = redactSupportBundleJSON(decoded)
	}

	row, err := h.queries.CreateSupportBundle(r.Context(), db.CreateSupportBundleParams{
		ID:               "sb_" + uuid.NewString(),
		WorkspaceID:      workspaceID,
		ActorUserID:      textOrNull(auth.GetUserID(r.Context())),
		ActorApiKeyID:    textOrNull(auth.GetAPIKeyID(r.Context())),
		RunID:            body.RunID,
		SchemaVersion:    body.SchemaVersion,
		CliVersion:       body.CliVersion,
		Summary:          body.Summary,
		ReportMarkdown:   report,
		Payload:          payload,
		FindingCount:     body.FindingCount,
		RecentErrorCount: body.RecentErrorCount,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to store support bundle")
		return
	}

	writeCreated(w, supportBundleFromRow(row, false))
}

func (h *SupportBundleHandler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := h.queries.ListAdminSupportBundles(r.Context(), db.ListAdminSupportBundlesParams{
		WorkspaceID: strings.TrimSpace(q.Get("workspace_id")),
		OwnerEmail:  strings.TrimSpace(q.Get("owner_email")),
		Query:       strings.TrimSpace(q.Get("q")),
		Limit:       int32(limit),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load support bundles")
		return
	}

	out := make([]supportBundleResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, supportBundleFromAdminListRow(row))
	}
	writeSuccessWithListMeta(w, out, len(out), limit)
}

func (h *SupportBundleHandler) GetAdmin(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid support bundle id")
		return
	}

	row, err := h.queries.GetAdminSupportBundle(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Support bundle not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load support bundle")
		return
	}
	writeSuccess(w, supportBundleFromAdminDetailRow(row))
}

func supportBundleFromRow(row db.SupportBundle, includeReport bool) supportBundleResponse {
	out := supportBundleResponse{
		ID:               row.ID,
		WorkspaceID:      row.WorkspaceID,
		RunID:            row.RunID,
		SchemaVersion:    row.SchemaVersion,
		CliVersion:       row.CliVersion,
		Summary:          row.Summary,
		FindingCount:     row.FindingCount,
		RecentErrorCount: row.RecentErrorCount,
		CreatedAt:        timestamptzTime(row.CreatedAt),
	}
	if includeReport {
		out.ReportMarkdown = row.ReportMarkdown
	}
	return out
}

func supportBundleFromAdminListRow(row db.ListAdminSupportBundlesRow) supportBundleResponse {
	return supportBundleResponse{
		ID:               row.ID,
		WorkspaceID:      row.WorkspaceID,
		WorkspaceName:    row.WorkspaceName,
		OwnerEmail:       row.OwnerEmail,
		PlanID:           row.PlanID,
		RunID:            row.RunID,
		SchemaVersion:    row.SchemaVersion,
		CliVersion:       row.CliVersion,
		Summary:          row.Summary,
		FindingCount:     row.FindingCount,
		RecentErrorCount: row.RecentErrorCount,
		CreatedAt:        timestamptzTime(row.CreatedAt),
	}
}

func supportBundleFromAdminDetailRow(row db.GetAdminSupportBundleRow) supportBundleResponse {
	out := supportBundleResponse{
		ID:               row.ID,
		WorkspaceID:      row.WorkspaceID,
		WorkspaceName:    row.WorkspaceName,
		OwnerEmail:       row.OwnerEmail,
		PlanID:           row.PlanID,
		RunID:            row.RunID,
		SchemaVersion:    row.SchemaVersion,
		CliVersion:       row.CliVersion,
		Summary:          row.Summary,
		ReportMarkdown:   redactSupportBundleMarkdown(row.ReportMarkdown),
		FindingCount:     row.FindingCount,
		RecentErrorCount: row.RecentErrorCount,
		CreatedAt:        timestamptzTime(row.CreatedAt),
	}
	return out
}

func textOrNull(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	return pgtype.Text{String: value, Valid: value != ""}
}

func timestamptzTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

var (
	supportBundleAPIKeyPattern = regexp.MustCompile(`up_(live|test)_[A-Za-z0-9]+`)
	supportBundleBearerPattern = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._~+/=-]{12,}`)
)

func redactSupportBundleMarkdown(raw string) string {
	out := supportBundleAPIKeyPattern.ReplaceAllString(raw, "[REDACTED_API_KEY]")
	out = supportBundleBearerPattern.ReplaceAllString(out, "Bearer [REDACTED]")
	return out
}

func redactSupportBundleJSON(v any) []byte {
	raw := integrationlogs.RedactJSON(v)
	if raw == nil {
		return []byte(`{}`)
	}
	return []byte(redactSupportBundleMarkdown(string(raw)))
}
