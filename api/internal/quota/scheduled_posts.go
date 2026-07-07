package quota

const FreePlanActiveScheduledPostLimit = 50

// ShouldBlockActiveScheduledPosts gates Free workspaces from holding too
// many future scheduled parent posts. Paid plans are intentionally
// unrestricted.
func ShouldBlockActiveScheduledPosts(planID string, current, additional int) bool {
	return ShouldBlockActiveScheduledPostsWithLimit(planID, current, additional, FreePlanActiveScheduledPostLimit)
}

func ShouldBlockActiveScheduledPostsWithLimit(planID string, current, additional, limit int) bool {
	if additional <= 0 {
		return false
	}
	if planID != "free" && planID != "" {
		switch planID {
		case "api", "basic", "growth", "team", "enterprise":
			return false
		}
	}
	if limit <= 0 {
		limit = FreePlanActiveScheduledPostLimit
	}
	return current+additional > limit
}
