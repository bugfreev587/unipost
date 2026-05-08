package integrationlogs

func RetentionDaysForPlan(planID string) int {
	switch planID {
	case "free":
		return 1
	case "api":
		return 7
	case "basic":
		return 14
	case "growth":
		return 30
	case "team":
		return 90
	case "enterprise":
		return 180
	default:
		return 7
	}
}
