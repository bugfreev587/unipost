package quota

import "testing"

func TestFreePlanActiveScheduledPostCap(t *testing.T) {
	if FreePlanActiveScheduledPostLimit != 50 {
		t.Fatalf("FreePlanActiveScheduledPostLimit = %d, want 50", FreePlanActiveScheduledPostLimit)
	}

	tests := []struct {
		name       string
		planID     string
		current    int
		additional int
		want       bool
	}{
		{"free below limit", "free", 49, 1, false},
		{"free over limit", "free", 50, 1, true},
		{"free bulk over limit", "free", 49, 2, true},
		{"free ignores nonpositive additional", "free", 50, 0, false},
		{"api has no cap", "api", 500, 1, false},
		{"unknown is treated as free", "unknown", 50, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldBlockActiveScheduledPosts(tt.planID, tt.current, tt.additional)
			if got != tt.want {
				t.Fatalf("ShouldBlockActiveScheduledPosts(%q, %d, %d) = %v, want %v", tt.planID, tt.current, tt.additional, got, tt.want)
			}
		})
	}
}
