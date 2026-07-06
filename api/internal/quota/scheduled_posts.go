package quota

const FreePlanActiveScheduledPostLimit = 50

// ShouldBlockActiveScheduledPosts gates Free workspaces from holding too
// many future scheduled parent posts. Paid plans are intentionally
// unrestricted.
func ShouldBlockActiveScheduledPosts(planID string, current, additional int) bool {
	if additional <= 0 {
		return false
	}
	if planID != "free" && planID != "" {
		switch planID {
		case "api", "basic", "growth", "team", "enterprise":
			return false
		}
	}
	return current+additional > FreePlanActiveScheduledPostLimit
}
