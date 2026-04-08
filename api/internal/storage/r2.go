// Package storage wraps Cloudflare R2 (S3-compatible) for two related
// jobs:
//
//  1. The TikTok PULL_FROM_URL workaround — stage user-supplied media
//     on a developer-controlled bucket whose URL prefix is verified
//     in our TikTok dev portal. UploadFromURL is the entry point.
//
//  2. (Sprint 2) The media library — accept user uploads via presigned
//     PUT URLs, hold them until they're attached to a post, and serve
//     signed download URLs to the publish path so adapters can fetch
//     them at dispatch time.
//
// Both jobs share one R2 bucket, one set of env vars (R2_ACCOUNT_ID /
// R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME /
// R2_PUBLIC_DOMAIN), and one *Client. They live in this package so we
// have one place to configure timeouts, error handling, and (future)
// retry logic.
//
// A nil *Client is a valid value: every method on it returns
// ErrNotConfigured so callers can fall back gracefully when R2 hasn't
// been provisioned. Tests use this to skip R2-dependent paths without
// stubbing the whole package.
package storage

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
var ErrNotConfigured = fmt.Errorf("storage:R2 client is not configured")

// New constructs a Client. Returns nil + error if any required field is
// missing or the S3 endpoint can't be reached.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" ||
		cfg.Bucket == "" || cfg.PublicDomain == "" {
		return nil, fmt.Errorf("storage:missing R2 configuration")
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

// UploadFromURL fetches the source URL once, stores the bytes in R2 under
// a content-addressed key under the "tiktok/" prefix, and returns the
// public URL TikTok (or any other PULL_FROM_URL consumer) can fetch
// from. Idempotent: re-uploading identical bytes is a no-op since the
// key is the sha256 of the body.
//
// This is the "stage on a verified domain" workaround for TikTok photo
// Direct Post, which only accepts PULL_FROM_URL from
// developer-verified domains. The Sprint 2 media library uses a
// different prefix ("media/") and a different code path; see
// PresignPut / Head / PresignGet in media.go.
func (c *Client) UploadFromURL(ctx context.Context, sourceURL string) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}

	// Step 1: download the source.
	req, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("storage:build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("storage:fetch source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("storage:source returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("storage:read source: %w", err)
	}
	if len(body) == 0 {
		return "", fmt.Errorf("storage:source body is empty")
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
		return "", fmt.Errorf("storage:put object: %w", err)
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
