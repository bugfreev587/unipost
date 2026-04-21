package handler

import "testing"

func TestTikTokImageWithin1080p(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		expectWithin bool
	}{
		{name: "portrait 1080p", width: 1080, height: 1920, expectWithin: true},
		{name: "landscape 1080p", width: 1920, height: 1080, expectWithin: true},
		{name: "square 1080", width: 1080, height: 1080, expectWithin: true},
		{name: "too tall", width: 1080, height: 2400, expectWithin: false},
		{name: "too wide", width: 2200, height: 1080, expectWithin: false},
		{name: "square too large", width: 1500, height: 1500, expectWithin: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tiktokImageWithin1080p(tc.width, tc.height); got != tc.expectWithin {
				t.Fatalf("tiktokImageWithin1080p(%d, %d) = %v, want %v", tc.width, tc.height, got, tc.expectWithin)
			}
		})
	}
}
