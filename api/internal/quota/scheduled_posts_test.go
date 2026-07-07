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

func TestFreePlanActiveScheduledPostCapOverride(t *testing.T) {
	tests := []struct {
		name       string
		planID     string
		current    int
		additional int
		limit      int
		want       bool
	}{
		{"free below temporary limit", "free", 249, 1, 250, false},
		{"free over temporary limit", "free", 250, 1, 250, true},
		{"unknown plan follows temporary limit", "unknown", 249, 1, 250, false},
		{"paid plan ignores temporary limit", "api", 250, 1, 250, false},
		{"invalid limit falls back to default", "free", 50, 1, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldBlockActiveScheduledPostsWithLimit(tt.planID, tt.current, tt.additional, tt.limit)
			if got != tt.want {
				t.Fatalf("ShouldBlockActiveScheduledPostsWithLimit(%q, %d, %d, %d) = %v, want %v",
					tt.planID, tt.current, tt.additional, tt.limit, got, tt.want)
			}
		})
	}
}
