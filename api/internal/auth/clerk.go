package auth

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
)

type contextKey string

const UserIDKey contextKey = "userID"

func ClerkSessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"Missing authorization header"}}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"Invalid authorization format"}}`, http.StatusUnauthorized)
			return
		}

		client := jwks.NewClient(&clerk.ClientConfig{
			BackendConfig: clerk.BackendConfig{
				Key: clerk.String(os.Getenv("CLERK_SECRET_KEY")),
			},
		})
		claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
			Token:    token,
			JWKSClient: client,
		})
		if err != nil {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"Invalid session token"}}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserID(ctx context.Context) string {
	userID, _ := ctx.Value(UserIDKey).(string)
	return userID
}
