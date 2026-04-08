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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// MediaPrefix is the R2 key prefix for user-uploaded media. Kept
// distinct from the "tiktok/" prefix used by UploadFromURL so the
// sweeper can target only one or the other without affecting the
// other path.
const MediaPrefix = "media/"

// MediaKey returns a stable R2 key for a media row. We use the row's
// UUID directly so the key never collides and so a HEAD lookup at
// dispatch time only needs the row ID, not extra storage_key state.
func MediaKey(mediaID, ext string) string {
	if ext != "" && ext[0] != '.' {
		ext = "." + ext
	}
	return MediaPrefix + mediaID + ext
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
	Exists      bool
	ContentType string
	SizeBytes   int64
	LastModified time.Time
	ETag        string
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
