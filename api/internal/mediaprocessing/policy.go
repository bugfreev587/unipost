package mediaprocessing

import "strings"

// Limits is the centralized customer-facing Media Processing fair-use policy.
// ActiveJobs is shared by every processing kind; GIFConversions24H is charged
// only when a new gif_to_mp4 job is created.
type Limits struct {
	ActiveJobs        int
	GIFConversions24H int
}

var limitsByPlan = map[string]Limits{
	"free":       {ActiveJobs: 1, GIFConversions24H: 10},
	"api":        {ActiveJobs: 2, GIFConversions24H: 50},
	"basic":      {ActiveJobs: 2, GIFConversions24H: 100},
	"growth":     {ActiveJobs: 4, GIFConversions24H: 300},
	"team":       {ActiveJobs: 6, GIFConversions24H: 1000},
	"enterprise": {ActiveJobs: 6, GIFConversions24H: 1000},
}

// LimitsForPlan fails closed to Free. An Enterprise override is accepted only
// when both dimensions are positive; the caller is responsible for loading it
// through the product's contract/entitlement authority.
func LimitsForPlan(planID string, enterpriseOverride *Limits) Limits {
	planID = strings.ToLower(strings.TrimSpace(planID))
	limits, ok := limitsByPlan[planID]
	if !ok {
		limits = limitsByPlan["free"]
	}
	if planID == "enterprise" && enterpriseOverride != nil && enterpriseOverride.ActiveJobs > 0 && enterpriseOverride.GIFConversions24H > 0 {
		return *enterpriseOverride
	}
	return limits
}
