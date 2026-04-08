package storage

import (
	"context"
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
	if _, err := c.Head(ctx, "k"); err != ErrNotConfigured {
		t.Errorf("Head on nil: want ErrNotConfigured, got %v", err)
	}
	if err := c.Delete(ctx, "k"); err != ErrNotConfigured {
		t.Errorf("Delete on nil: want ErrNotConfigured, got %v", err)
	}
	if _, err := c.UploadFromURL(ctx, "https://x.com/y.jpg"); err != ErrNotConfigured {
		t.Errorf("UploadFromURL on nil: want ErrNotConfigured, got %v", err)
	}
}
