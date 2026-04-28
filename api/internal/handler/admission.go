// admission.go centralizes the rate-limit + queue-admission calls
// the social-post handlers make. The rate-limit PRD (April 2026)
// breaks admission into three controls (request, enqueue, depth);
// this helper keeps the per-route call sites a single line so
// individual handlers stay readable.
//
// The default failure-mode is to LET THE REQUEST THROUGH on any
// unexpected error from the limiter (logged at warn). The limiter's
// own circuit breaker already fails-open into a degraded local
// bucket on Redis errors, so reaching this fallback usually means
// either a misconfiguration or a transient DB hiccup that should
// not break publish.

package handler

import (
	"log/slog"
	"net/http"

	"github.com/xiaoboyu/unipost-api/internal/ratelimit"
)

// admissionOpts toggles which controls run for a given route.
//
//   request : per-workspace token bucket (request rate cap)
//   enqueue : sliding-window cap on accepted post units
//   depth   : workspace queue-depth cap on active delivery jobs
//
// Routes that only do request limiting (drafts, cancel, update,
// bulk in v1) leave enqueue and depth false. Routes that admit new
// queued work (Create immediate, PublishDraft, RetryResult) set all
// three.
type admissionOpts struct {
	request bool
	enqueue bool
	depth   bool

	// units is the post-count weight for the enqueue and depth
	// controls. For a single create / publish / retry this is 1
	// (or len(parsed.Posts) for depth, which counts delivery jobs).
	enqueueUnits int
	depthUnits   int
}

// admit runs the configured controls in order (request → enqueue →
// depth), short-circuits with a 429 on the first denial, and returns
// true if the request may proceed. Callers must check the return
// value and return immediately on false; admit has already written
// the response.
func (h *SocialPostHandler) admit(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID string,
	route string,
	opts admissionOpts,
) bool {
	ctx := r.Context()
	planID := h.quota.PlanIDFor(ctx, workspaceID)

	if opts.request {
		dec, err := h.limiter.AllowRequest(ctx, ratelimit.RequestScope{
			WorkspaceID: workspaceID,
			PlanID:      planID,
			Route:       route,
		})
		if err != nil {
			slog.Warn("ratelimit: request limiter error, allowing", "err", err, "ws", workspaceID, "route", route)
		} else if !dec.Allowed {
			writeRateLimited(w, dec.Reason,
				"Too many requests for this workspace. Please retry shortly.",
				dec.RetryAfter)
			return false
		}
	}

	if opts.enqueue {
		units := opts.enqueueUnits
		if units < 1 {
			units = 1
		}
		dec, err := h.limiter.AllowEnqueue(ctx, ratelimit.EnqueueScope{
			WorkspaceID: workspaceID,
			PlanID:      planID,
		}, units)
		if err != nil {
			slog.Warn("ratelimit: enqueue limiter error, allowing", "err", err, "ws", workspaceID, "route", route)
		} else if !dec.Allowed {
			writeRateLimited(w, dec.Reason,
				"This workspace is creating posts too quickly. Please slow down and retry.",
				dec.RetryAfter)
			return false
		}
	}

	if opts.depth {
		units := opts.depthUnits
		if units < 1 {
			units = 1
		}
		dec, err := h.limiter.CheckQueueDepth(ctx, ratelimit.QueueScope{
			WorkspaceID: workspaceID,
			PlanID:      planID,
		}, units)
		if err != nil {
			slog.Warn("ratelimit: depth limiter error, allowing", "err", err, "ws", workspaceID, "route", route)
		} else if !dec.Allowed {
			writeRateLimited(w, dec.Reason,
				"This workspace already has too many queued deliveries. Wait for the queue to drain before creating more posts.",
				dec.RetryAfter)
			return false
		}
	}

	return true
}
