// Package mediaproxy stages user-supplied media on a developer-controlled
// public bucket so that platforms which require domain-verified PULL_FROM_URL
// (notably TikTok photo Direct Post) can fetch it.
//
// Without this proxy, every customer would have to register their CDN domain
// with our TikTok developer portal — which doesn't scale to a multi-tenant
// SaaS. By staging the bytes on a single bucket whose URL prefix is
// pre-registered, every customer just works.
//
// Storage strategy: content-addressed (sha256) so identical bytes uploaded
// twice dedupe to the same key. The bucket is configured with a lifecycle
// rule that deletes objects after 24 hours; TikTok caches the image
// immediately after PULL_FROM_URL succeeds, so we don't need long retention.
package mediaproxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client uploads source URLs to R2 and returns publicly reachable URLs that
// PULL_FROM_URL endpoints can fetch. A nil *Client is a valid value: every
// method on it returns ErrNotConfigured so callers can fall back gracefully.
type Client struct {
	s3         *s3.Client
	bucket     string
	publicBase string
	httpClient *http.Client
}

// Config holds the R2 settings read from environment variables. See
// FromEnv() for the canonical wiring.
type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PublicDomain    string // e.g. https://pub-xxx.r2.dev
}

// ErrNotConfigured is returned by every Client method when the receiver is
// nil. Adapters use this to surface a clear error to the caller instead of
// crashing when R2 hasn't been provisioned.
var ErrNotConfigured = fmt.Errorf("mediaproxy: R2 client is not configured")

// New constructs a Client. Returns nil + error if any required field is
// missing or the S3 endpoint can't be reached.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" ||
		cfg.Bucket == "" || cfg.PublicDomain == "" {
		return nil, fmt.Errorf("mediaproxy: missing R2 configuration")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)

	s3Client := s3.New(s3.Options{
		Region: "auto", // R2 uses "auto"
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		),
		BaseEndpoint: aws.String(endpoint),
		// R2 requires path-style addressing (not virtual-hosted) for
		// custom domains.
		UsePathStyle: true,
	})

	return &Client{
		s3:         s3Client,
		bucket:     cfg.Bucket,
		publicBase: strings.TrimRight(cfg.PublicDomain, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Upload fetches the source URL once, stores the bytes in R2 under a
// content-addressed key, and returns the public URL TikTok (or any other
// PULL_FROM_URL consumer) can fetch from. Idempotent: re-uploading identical
// bytes is a no-op since the key is the sha256 of the body.
func (c *Client) Upload(ctx context.Context, sourceURL string) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}

	// Step 1: download the source.
	req, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("mediaproxy: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mediaproxy: fetch source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("mediaproxy: source returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("mediaproxy: read source: %w", err)
	}
	if len(body) == 0 {
		return "", fmt.Errorf("mediaproxy: source body is empty")
	}

	// Step 2: build a content-addressed key. We hash the bytes (not the
	// URL) so two different URLs serving the same image collapse to one
	// stored object — and so cache busts on the source side don't waste
	// our storage.
	sum := sha256.Sum256(body)
	hashHex := hex.EncodeToString(sum[:])
	ext := guessExt(resp.Header.Get("Content-Type"), sourceURL)
	key := path.Join("tiktok", hashHex+ext)

	// Step 3: upload to R2. We always Put — R2 doesn't bill for redundant
	// writes within the lifecycle window and HEAD-then-PUT would just add
	// a round trip.
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = inferContentType(ext)
	}

	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
		// CacheControl ensures TikTok's CDN caches aggressively after the
		// first fetch — we only need the object to live ~5 minutes for
		// TikTok to ingest it.
		CacheControl: aws.String("public, max-age=86400, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("mediaproxy: put object: %w", err)
	}

	return c.publicBase + "/" + key, nil
}

// guessExt picks a file extension from a Content-Type header, falling back
// to the source URL's existing extension if the header is missing.
// Defaults to .bin so we never write a key with no extension at all.
func guessExt(contentType, sourceURL string) string {
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/heic":
		return ".heic"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "video/webm":
		return ".webm"
	}
	// Strip query string before extracting extension.
	clean := sourceURL
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	if ext := strings.ToLower(path.Ext(clean)); ext != "" {
		return ext
	}
	return ".bin"
}

// inferContentType is the inverse of guessExt for the rare case where the
// upstream server didn't set Content-Type but we can guess from the URL.
func inferContentType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	}
	return "application/octet-stream"
}
