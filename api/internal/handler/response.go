package handler

import (
	"encoding/json"
	"net/http"
)

type SuccessResponse struct {
	Data any            `json:"data"`
	Meta *MetaResponse  `json:"meta,omitempty"`
}

type MetaResponse struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, SuccessResponse{Data: data})
}

func writeSuccessWithMeta(w http.ResponseWriter, data any, total int) {
	writeJSON(w, http.StatusOK, SuccessResponse{
		Data: data,
		Meta: &MetaResponse{Total: total, Page: 1, PerPage: 20},
	})
}

func writeCreated(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, SuccessResponse{Data: data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message},
	})
}
