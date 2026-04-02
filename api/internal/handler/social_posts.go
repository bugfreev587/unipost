package handler

import "net/http"

type SocialPostHandler struct{}

func NewSocialPostHandler() *SocialPostHandler {
	return &SocialPostHandler{}
}

func (h *SocialPostHandler) List(w http.ResponseWriter, r *http.Request) {
	writeSuccessWithMeta(w, []any{}, 0)
}

func (h *SocialPostHandler) Create(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusBadRequest, "NO_ACCOUNTS", "No social accounts connected")
}
