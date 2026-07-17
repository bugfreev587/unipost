package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMediaKey locks down the storage key shape so a future refactor
// doesn't accidentally start writing into a different prefix and
// stranding the existing rows.
func TestMediaKey(t *testing.T) {
	cases := []struct {
		id   string
		ext  string
		want string
	}{
		{"abc123", ".jpg", "media/abc123.jpg"},
		{"abc123", "jpg", "media/abc123.jpg"}, // ext gets a leading dot
		{"abc123", "", "media/abc123"},
		{"abc123", ".png", "media/abc123.png"},
	}
	for _, c := range cases {
		got := MediaKey(c.id, c.ext)
		if got != c.want {
			t.Errorf("MediaKey(%q, %q) = %q, want %q", c.id, c.ext, got, c.want)
		}
	}
}

func TestPullObjectKeyForSource(t *testing.T) {
	got := PullObjectKeyForSource("media/media_123.mp4")
	if !strings.HasPrefix(got, "pull/") {
		t.Fatalf("PullObjectKeyForSource prefix = %q, want pull/", got)
	}
	if !strings.HasSuffix(got, ".mp4") {
		t.Fatalf("PullObjectKeyForSource suffix = %q, want .mp4", got)
	}
	if got != PullObjectKeyForSource("media/media_123.mp4") {
		t.Fatal("PullObjectKeyForSource must be stable for cleanup")
	}
}

// TestNilClient covers the documented "nil *Client is a valid value"
// contract. Every public method on a nil receiver should return
// ErrNotConfigured rather than panicking, so callers can pass a nil
// client through code paths that don't actually need R2.
func TestNilClient(t *testing.T) {
	var c *Client

	ctx := context.TODO()
	if _, err := c.PresignPut(ctx, "k", "image/png", 0); err != ErrNotConfigured {
		t.Errorf("PresignPut on nil: want ErrNotConfigured, got %v", err)
	}
	if _, err := c.PresignGet(ctx, "k", 0); err != ErrNotConfigured {
		t.Errorf("PresignGet on nil: want ErrNotConfigured, got %v", err)
	}
	if _, err := c.StageObjectForPull(ctx, "k"); err != ErrNotConfigured {
		t.Errorf("StageObjectForPull on nil: want ErrNotConfigured, got %v", err)
	}
	if _, err := c.Head(ctx, "k"); err != ErrNotConfigured {
		t.Errorf("Head on nil: want ErrNotConfigured, got %v", err)
	}
	if err := c.Delete(ctx, "k"); err != ErrNotConfigured {
		t.Errorf("Delete on nil: want ErrNotConfigured, got %v", err)
	}
	if _, err := c.UploadFromURL(ctx, "https://x.com/y.jpg"); err != ErrNotConfigured {
		t.Errorf("UploadFromURL on nil: want ErrNotConfigured, got %v", err)
	}
	if err := c.DownloadObject(ctx, "media/in.mp4", "/tmp/in.mp4"); err != ErrNotConfigured {
		t.Errorf("DownloadObject on nil: want ErrNotConfigured, got %v", err)
	}
	if err := c.DownloadObjectLimited(ctx, "media/in.gif", "/tmp/in.gif", 1); err != ErrNotConfigured {
		t.Errorf("DownloadObjectLimited on nil: want ErrNotConfigured, got %v", err)
	}
	if err := c.PutFile(ctx, "media/out.mp4", "/tmp/out.mp4", "video/mp4", "public, max-age=1"); err != ErrNotConfigured {
		t.Errorf("PutFile on nil: want ErrNotConfigured, got %v", err)
	}
}

func TestBrandingLogoKey(t *testing.T) {
	cases := []struct {
		workspaceID string
		profileID   string
		ext         string
		wantPrefix  string
		wantSuffix  string
	}{
		{"ws_123", "pr_456", ".png", "branding/ws_123/pr_456/logo_", ".png"},
		{"ws_123", "pr_456", "jpg", "branding/ws_123/pr_456/logo_", ".jpg"},
	}
	for _, c := range cases {
		got := BrandingLogoKey(c.workspaceID, c.profileID, c.ext)
		if !strings.HasPrefix(got, c.wantPrefix) || !strings.HasSuffix(got, c.wantSuffix) {
			t.Fatalf("BrandingLogoKey(%q,%q,%q) = %q, want prefix %q suffix %q", c.workspaceID, c.profileID, c.ext, got, c.wantPrefix, c.wantSuffix)
		}
	}
}

func TestNilClientBrandingHelpers(t *testing.T) {
	var c *Client
	if err := c.PutObject(context.TODO(), "branding/ws/pr/logo.png", strings.NewReader("x"), "image/png", "public, max-age=1"); err != ErrNotConfigured {
		t.Errorf("PutObject on nil: want ErrNotConfigured, got %v", err)
	}
	if got := c.PublicURL("branding/ws/pr/logo.png"); got != "" {
		t.Errorf("PublicURL on nil = %q, want empty string", got)
	}
}

func TestWriteFileLimitedRejectsBytesBeyondLimitAndRemovesPartialFile(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "input.gif")
	err := writeFileLimited(destination, bytes.NewBufferString("123456"), 5)
	if !errors.Is(err, ErrObjectTooLarge) {
		t.Fatalf("writeFileLimited error = %v, want ErrObjectTooLarge", err)
	}
	if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("partial destination remains: %v", statErr)
	}
}

func TestWriteFileLimitedAllowsExactLimit(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "input.gif")
	if err := writeFileLimited(destination, bytes.NewBufferString("12345"), 5); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "12345" {
		t.Fatalf("body = %q", body)
	}
}
