package mediaretention

import (
	"testing"
	"time"
)

func TestRetentionForPlanStatus(t *testing.T) {
	tests := []struct {
		name       string
		planID     string
		postStatus string
		want       time.Duration
		wantOK     bool
	}{
		{"free published", "free", "published", 24 * time.Hour, true},
		{"free failed", "free", "failed", 48 * time.Hour, true},
		{"free partial", "free", "partial", 48 * time.Hour, true},
		{"free cancelled", "free", "cancelled", 48 * time.Hour, true},
		{"api published", "api", "published", 2 * 24 * time.Hour, true},
		{"api failed", "api", "failed", 4 * 24 * time.Hour, true},
		{"basic published", "basic", "published", 4 * 24 * time.Hour, true},
		{"basic failed", "basic", "failed", 8 * 24 * time.Hour, true},
		{"growth published", "growth", "published", 15 * 24 * time.Hour, true},
		{"growth failed", "growth", "failed", 30 * 24 * time.Hour, true},
		{"team published", "team", "published", 30 * 24 * time.Hour, true},
		{"team failed", "team", "failed", 60 * 24 * time.Hour, true},
		{"enterprise follows team", "enterprise", "failed", 60 * 24 * time.Hour, true},
		{"unknown plan falls back to free", "mystery", "published", 24 * time.Hour, true},
		{"scheduled is active", "free", "scheduled", 0, false},
		{"draft is active", "basic", "draft", 0, false},
		{"publishing is active", "growth", "publishing", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := RetentionForPlanStatus(tt.planID, tt.postStatus)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("duration = %s, want %s", got, tt.want)
			}
		})
	}
}
