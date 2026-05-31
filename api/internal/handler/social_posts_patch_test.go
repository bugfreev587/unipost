package handler

import (
	"bytes"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestParseSocialPostLifecyclePatch(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantOK  bool
		wantErr bool
		assert  func(t *testing.T, patch socialPostLifecyclePatch)
	}{
		{
			name:   "archived true",
			raw:    `{"archived":true}`,
			wantOK: true,
			assert: func(t *testing.T, patch socialPostLifecyclePatch) {
				if patch.Archived == nil || !*patch.Archived {
					t.Fatalf("expected archived=true, got %#v", patch.Archived)
				}
			},
		},
		{
			name:   "cancelled status normalized",
			raw:    `{"status":"cancelled"}`,
			wantOK: true,
			assert: func(t *testing.T, patch socialPostLifecyclePatch) {
				if patch.Status == nil || *patch.Status != "canceled" {
					t.Fatalf("expected status=canceled, got %#v", patch.Status)
				}
			},
		},
		{
			name:    "reject mixed lifecycle and content fields",
			raw:     `{"archived":true,"caption":"nope"}`,
			wantOK:  true,
			wantErr: true,
		},
		{
			name:    "reject unsupported status",
			raw:     `{"status":"published"}`,
			wantOK:  true,
			wantErr: true,
		},
		{
			name:   "regular content patch is not lifecycle",
			raw:    `{"platform_posts":[{"account_id":"sa_1","caption":"hi"}]}`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, ok, err := parseSocialPostLifecyclePatch([]byte(tt.raw))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.assert != nil {
				tt.assert(t, patch)
			}
		})
	}
}

func TestCanEditSocialPostContent(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "draft", want: true},
		{status: "scheduled", want: true},
		{status: "publishing", want: false},
		{status: "published", want: false},
		{status: "failed", want: false},
		{status: "cancelled", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := canEditSocialPostContent(tt.status); got != tt.want {
				t.Fatalf("canEditSocialPostContent(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestBuildContentUpdateParamsIncludesProfilesForScheduledPost(t *testing.T) {
	scheduledAt := time.Date(2026, 6, 1, 18, 30, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{{
		AccountID: "acct_linkedin",
		Caption:   "updated caption",
		MediaURLs: []string{"https://cdn.example/image.jpg"},
	}}
	metadata, err := platform.EncodePostMetadata(posts)
	if err != nil {
		t.Fatal(err)
	}

	params := buildSocialPostContentUpdateParams("post_1", "ws_1", posts, metadata, &scheduledAt, []string{"prof_1"})

	if params.ID != "post_1" || params.WorkspaceID != "ws_1" {
		t.Fatalf("unexpected ids: %#v", params)
	}
	if !params.Caption.Valid || params.Caption.String != "updated caption" {
		t.Fatalf("caption = %#v, want updated caption", params.Caption)
	}
	if len(params.MediaUrls) != 1 || params.MediaUrls[0] != "https://cdn.example/image.jpg" {
		t.Fatalf("media urls = %#v", params.MediaUrls)
	}
	if !bytes.Equal(params.Metadata, metadata) {
		t.Fatalf("metadata mismatch")
	}
	if !params.ScheduledAt.Valid || !params.ScheduledAt.Time.Equal(scheduledAt) {
		t.Fatalf("scheduled_at = %#v, want %s", params.ScheduledAt, scheduledAt)
	}
	if len(params.ProfileIds) != 1 || params.ProfileIds[0] != "prof_1" {
		t.Fatalf("profile_ids = %#v, want prof_1", params.ProfileIds)
	}
}
