package handler

import "net/http"

type SocialAccountHandler struct{}

func NewSocialAccountHandler() *SocialAccountHandler {
	return &SocialAccountHandler{}
}

func (h *SocialAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	writeSuccessWithMeta(w, []any{}, 0)
}
