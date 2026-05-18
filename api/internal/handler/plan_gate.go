package handler

import (
	"net/http"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
)

// plan_gate.go houses chi-style middleware that enforce the
// product-tier feature gates added by migration 059. Each gate is
// a one-line opt-in for a route group:
//
//	r.Route("/v1/inbox", func(r chi.Router) {
//	    r.Use(handler.RequirePlanInbox(quotaChecker))
//	    r.Get("/", inboxHandler.List)
//	    ...
//	})
//
// Gates run AFTER the auth middleware that stamps workspace_id into
// the request context, so a missing workspace context is treated as an
// internal routing/auth-context error rather than an auth or plan failure.
//
// Fail-open in the Checker: if the plans table is briefly unreadable
// the gate lets the request through. A paying customer briefly seeing
// "their" feature is much less bad than a global outage triggered by
// a transient DB hiccup.

// RequirePlanInbox blocks /v1/inbox/* on plans where allow_inbox is
// FALSE (Free, API). Returns 402 PLAN_FEATURE_NOT_AVAILABLE so
// clients can render an upgrade CTA. The error message names the
// minimum tier so support burden stays low.
func RequirePlanInbox(q *quota.Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workspaceID := auth.GetWorkspaceID(r.Context())
			if workspaceID == "" {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Missing workspace context")
				return
			}
			if q != nil && !q.PlanAllowsInbox(r.Context(), workspaceID) {
				writeError(w, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE",
					"Inbox requires the Basic plan or higher — upgrade at unipost.dev/pricing")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireFeatureFlag blocks a route group when its remote feature flag is
// disabled. This is separate from plan gates: flags control rollout and
// emergency shutdown, while plan gates control packaging.
func RequireFeatureFlag(flag featureflags.Flag) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workspaceID := auth.GetWorkspaceID(r.Context())
			if workspaceID == "" {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Missing workspace context")
				return
			}

			if !featureflags.Enabled(r.Context(), flag, featureflags.Target{
				UserID:      auth.GetUserID(r.Context()),
				WorkspaceID: workspaceID,
				Env:         runtimeenv.Current(),
			}) {
				writeError(w, http.StatusForbidden, "FEATURE_DISABLED", "This feature is currently disabled.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePlanAnalytics blocks /v1/analytics/* (and per-post analytics)
// on plans where allow_analytics is FALSE (Free). API and up are
// allowed at the server layer; the "API tier is read-only" framing on
// the pricing page is enforced by the dashboard hiding the Analytics
// page rather than by a separate server-side bit.
func RequirePlanAnalytics(q *quota.Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workspaceID := auth.GetWorkspaceID(r.Context())
			if workspaceID == "" {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Missing workspace context")
				return
			}
			if q != nil && !q.PlanAllowsAnalytics(r.Context(), workspaceID) {
				writeError(w, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE",
					"Analytics requires any paid plan ($10/mo and up) — upgrade at unipost.dev/pricing")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePlanWhiteLabel blocks the white-label / native-mode platform
// credentials endpoints (POST/DELETE /v1/platform-credentials) on
// plans where plans.white_label is FALSE (Free / API / Basic).
// Growth and up are allowed. Read access (GET /v1/platform-credentials)
// stays open so the dashboard can render "you have no creds yet"
// instead of a 402 toast.
func RequirePlanWhiteLabel(q *quota.Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workspaceID := auth.GetWorkspaceID(r.Context())
			if workspaceID == "" {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Missing workspace context")
				return
			}
			if q != nil && !q.PlanAllowsWhiteLabel(r.Context(), workspaceID) {
				writeError(w, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE",
					"White-label / native mode requires the Growth plan or higher — upgrade at unipost.dev/pricing")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
