package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type ProjectHandler struct {
	queries *db.Queries
}

func NewProjectHandler(queries *db.Queries) *ProjectHandler {
	return &ProjectHandler{queries: queries}
}

type projectResponse struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Name      string    `json:"name"`
	Mode      string    `json:"mode"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toProjectResponse(p db.Project) projectResponse {
	return projectResponse{
		ID:        p.ID,
		OwnerID:   p.OwnerID,
		Name:      p.Name,
		Mode:      p.Mode,
		CreatedAt: p.CreatedAt.Time,
		UpdatedAt: p.UpdatedAt.Time,
	}
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	projects, err := h.queries.ListProjectsByOwner(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list projects")
		return
	}

	result := make([]projectResponse, len(projects))
	for i, p := range projects {
		result[i] = toProjectResponse(p)
	}

	writeSuccessWithMeta(w, result, len(result))
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	var body struct {
		Name string `json:"name"`
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name is required")
		return
	}

	if body.Mode == "" {
		body.Mode = "quickstart"
	}
	if body.Mode != "quickstart" && body.Mode != "whitelabel" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Mode must be 'quickstart' or 'whitelabel'")
		return
	}

	project, err := h.queries.CreateProject(r.Context(), db.CreateProjectParams{
		OwnerID: userID,
		Name:    body.Name,
		Mode:    body.Mode,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create project")
		return
	}

	writeCreated(w, toProjectResponse(project))
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	projectID := chi.URLParam(r, "id")

	project, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID:      projectID,
		OwnerID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get project")
		return
	}

	writeSuccess(w, toProjectResponse(project))
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	projectID := chi.URLParam(r, "id")

	// Verify ownership
	_, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID:      projectID,
		OwnerID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get project")
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Name is required")
		return
	}

	project, err := h.queries.UpdateProject(r.Context(), db.UpdateProjectParams{
		ID:   projectID,
		Name: body.Name,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update project")
		return
	}

	writeSuccess(w, toProjectResponse(project))
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	projectID := chi.URLParam(r, "id")

	// Verify ownership
	_, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID:      projectID,
		OwnerID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get project")
		return
	}

	if err := h.queries.DeleteProject(r.Context(), projectID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete project")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
