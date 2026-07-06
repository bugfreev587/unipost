package mediaretention

import "time"

type windows struct {
	published time.Duration
	failed    time.Duration
}

var planWindows = map[string]windows{
	"free":       {published: 24 * time.Hour, failed: 48 * time.Hour},
	"api":        {published: 2 * 24 * time.Hour, failed: 4 * 24 * time.Hour},
	"basic":      {published: 4 * 24 * time.Hour, failed: 8 * 24 * time.Hour},
	"growth":     {published: 15 * 24 * time.Hour, failed: 30 * 24 * time.Hour},
	"team":       {published: 30 * 24 * time.Hour, failed: 60 * 24 * time.Hour},
	"enterprise": {published: 30 * 24 * time.Hour, failed: 60 * 24 * time.Hour},
}

// RetentionForPlanStatus returns the media retention duration for a
// terminal parent post status. Non-terminal states return ok=false so
// callers keep the media active and ineligible for cleanup.
func RetentionForPlanStatus(planID, postStatus string) (time.Duration, bool) {
	w, ok := planWindows[planID]
	if !ok {
		w = planWindows["free"]
	}
	switch postStatus {
	case "published":
		return w.published, true
	case "failed", "partial":
		return w.failed, true
	default:
		return 0, false
	}
}
