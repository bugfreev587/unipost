package auth

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

const ProjectIDKey contextKey = "projectID"

// APIKeyMiddleware verifies API keys by hashing the presented key and looking up
// the hash in the database. It checks revocation and expiration, updates last_used_at
// in a background goroutine, and injects the project_id into the request context.
func APIKeyMiddleware(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "Missing API key",
					},
				})
				return
			}

			key := strings.TrimPrefix(authHeader, "Bearer ")
			if key == authHeader {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "Invalid authorization format",
					},
				})
				return
			}

			hash := apikey.Hash(key)

			ak, err := queries.GetAPIKeyByHash(r.Context(), hash)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "Invalid API key",
					},
				})
				return
			}

			// Check revocation
			if ak.RevokedAt.Valid {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "API key has been revoked",
					},
				})
				return
			}

			// Check expiration
			if ak.ExpiresAt.Valid && ak.ExpiresAt.Time.Before(time.Now()) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{
						"code":    "UNAUTHORIZED",
						"message": "API key has expired",
					},
				})
				return
			}

			// Update last_used_at in background
			go func() {
				if err := queries.UpdateAPIKeyLastUsedAt(context.Background(), ak.ID); err != nil {
					log.Printf("failed to update last_used_at for key %s: %v", ak.ID, err)
				}
			}()

			ctx := context.WithValue(r.Context(), ProjectIDKey, ak.ProjectID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetProjectID(ctx context.Context) string {
	projectID, _ := ctx.Value(ProjectIDKey).(string)
	return projectID
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
