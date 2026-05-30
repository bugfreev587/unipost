// media.go is the Sprint 2 media-library half of the storage package.
// While r2.go's UploadFromURL stages bytes for the TikTok PULL_FROM_URL
// workaround, this file exposes the primitives the new POST /v1/media
// endpoints need:
//
//	PresignPut(key, contentType, ttl)  → URL the client can PUT to
//	Head(key)                          → metadata + size + content type
//	PresignGet(key, ttl)               → short-lived download URL for
//	                                     adapter dispatch
//	Delete(key)                        → soft cleanup, used by sweeper
//
// All four go through the same s3.Client constructed in r2.go's New().
// Keys live under the "media/" prefix to keep them out of the
// "tiktok/" namespace used by UploadFromURL.

package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// MediaPrefix is the R2 key prefix for user-uploaded media. Kept
// distinct from the "tiktok/" prefix used by UploadFromURL so the
// sweeper can target only one or the other without affecting the
// other path.
const MediaPrefix = "media/"

// BrandingPrefix is the R2 key prefix for long-lived hosted Connect
// branding assets. It is intentionally separate from MediaPrefix
// because post media can be swept after publish, while logos are
// workspace configuration and must be retained indefinitely.
const BrandingPrefix = "branding/"

// MediaKey returns a stable R2 key for a media row. We use the row's
// UUID directly so the key never collides and so a HEAD lookup at
// dispatch time only needs the row ID, not extra storage_key state.
func MediaKey(mediaID, ext string) string {
	if ext != "" && ext[0] != '.' {
		ext = "." + ext
	}
	return MediaPrefix + mediaID + ext
}

// BrandingLogoKey returns a unique R2 key for one profile logo upload.
// UUID file names let us serve logos with immutable cache headers while
// replacements use a fresh URL.
func BrandingLogoKey(workspaceID, profileID, ext string) string {
	if ext != "" && ext[0] != '.' {
		ext = "." + ext
	}
	return path.Join("branding", workspaceID, profileID, "logo_"+uuid.NewString()+ext)
}

// PublicURL returns the public R2 URL for a stored key. Nil clients return
// an empty string, matching the package's "nil client is not configured"
// convention without forcing callers to special-case URL assembly.
func (c *Client) PublicURL(key string) string {
	if c == nil {
		return ""
	}
	return c.publicBase + "/" + strings.TrimLeft(key, "/")
}

// PutObject stores caller-provided bytes at a specific key. It is used for
// small API-mediated uploads such as branding logos where a direct browser
// presigned PUT would create extra orphan handling.
func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, contentType string, cacheControl string) error {
	if c == nil {
		return ErrNotConfigured
	}
	if cacheControl == "" {
		cacheControl = "public, max-age=31536000, immutable"
	}
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String(cacheControl),
	})
	if err != nil {
		return fmt.Errorf("storage: put object: %w", err)
	}
	return nil
}

// PresignPut returns a presigned URL the client can PUT bytes to.
// The URL embeds the bucket key, content-type, and an expiry. After
// expiry the client must request a fresh URL via POST /v1/media.
//
// contentType is required by R2's signature — passing it ensures the
// client's PUT must use the same Content-Type header. Any other value
// in the actual PUT will fail signature verification.
func (c *Client) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	psClient := s3.NewPresignClient(c.s3, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	req, err := psClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("storage: presign put: %w", err)
	}
	return req.URL, nil
}

// HeadResult is what Head returns after a successful HEAD against R2.
// Used by the media handler to hydrate a row's size + content type
// the first time a media_id is referenced (the "poll-on-attach"
// pattern that replaces R2-side webhooks).
type HeadResult struct {
	Exists       bool
	ContentType  string
	SizeBytes    int64
	LastModified time.Time
	ETag         string
}

// Head fetches object metadata without downloading the body. Returns
// (HeadResult{Exists: false}, nil) when the object doesn't exist —
// not an error, since "client hasn't uploaded yet" is a normal state
// in the media flow.
func (c *Client) Head(ctx context.Context, key string) (HeadResult, error) {
	if c == nil {
		return HeadResult{}, ErrNotConfigured
	}
	out, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// 404 from R2 surfaces as a smithy NotFound error. We don't
		// distinguish 404 from other errors here — both mean "not
		// usable yet" — but we DO swallow it into a non-error return
		// so callers can do `if !res.Exists { ... }` cleanly.
		return HeadResult{Exists: false}, nil
	}
	res := HeadResult{
		Exists: true,
	}
	if out.ContentType != nil {
		res.ContentType = *out.ContentType
	}
	if out.ContentLength != nil {
		res.SizeBytes = *out.ContentLength
	}
	if out.LastModified != nil {
		res.LastModified = *out.LastModified
	}
	if out.ETag != nil {
		res.ETag = *out.ETag
	}
	return res, nil
}

// PresignGet returns a short-lived download URL for an object. Used
// by the publish path to hand adapters a fetchable URL for media that
// the user uploaded via PresignPut. ttl should be long enough for the
// adapter to download but short enough that the URL isn't shareable
// long-term — 15 minutes is a sensible default.
func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	psClient := s3.NewPresignClient(c.s3, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	req, err := psClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("storage: presign get: %w", err)
	}
	return req.URL, nil
}

// StageObjectForPull copies an existing object in this bucket to a stable
// public URL under a content-addressed "pull/" prefix. This avoids giving
// pull-by-URL platforms a short-lived presigned GET when we already own the
// bytes in R2 and can expose them through the bucket's public domain.
func (c *Client) StageObjectForPull(ctx context.Context, key string) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	sum := sha256.Sum256([]byte(key))
	dstKey := path.Join("pull", hex.EncodeToString(sum[:])+path.Ext(key))
	copySource := url.PathEscape(c.bucket + "/" + key)

	_, err := c.s3.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String(dstKey),
		CopySource:   aws.String(copySource),
		CacheControl: aws.String("public, max-age=86400, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("storage: stage object for pull: %w", err)
	}
	return c.publicBase + "/" + dstKey, nil
}

// Delete removes an object from R2. Used by the media sweeper for
// abandoned uploads (status=pending older than 7 days). Soft errors
// (already-deleted, never-existed) are not returned.
func (c *Client) Delete(ctx context.Context, key string) error {
	if c == nil {
		return ErrNotConfigured
	}
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("storage: delete: %w", err)
	}
	return nil
}
