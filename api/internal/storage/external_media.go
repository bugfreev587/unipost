package storage

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/safefetch"
)

var (
	ErrExternalMediaFetch  = errors.New("storage: verified external media fetch failed")
	ErrExternalMediaUpload = errors.New("storage: verified external media upload failed")
)

type ExternalMediaResult struct {
	PublicURL string
	MediaType string
	SizeBytes int64
	SHA256Hex string
}

type verifiedFileUploader func(context.Context, string, string, string, string) error
type publicURLBuilder func(string) string

func (c *Client) StageExternalMedia(ctx context.Context, rawURL string, policy safefetch.Policy) (ExternalMediaResult, error) {
	if c == nil {
		return ExternalMediaResult{}, ErrNotConfigured
	}
	if c.externalFetcher == nil {
		return ExternalMediaResult{}, ErrExternalMediaFetch
	}
	return stageExternalMedia(ctx, rawURL, policy, c.externalFetcher, c.PutFile, c.PublicURL)
}

func stageExternalMedia(
	ctx context.Context,
	rawURL string,
	policy safefetch.Policy,
	fetcher safefetch.Fetcher,
	putFile verifiedFileUploader,
	publicURL publicURLBuilder,
) (ExternalMediaResult, error) {
	if fetcher == nil || putFile == nil || publicURL == nil {
		return ExternalMediaResult{}, ErrExternalMediaFetch
	}
	fetched, err := fetcher.Fetch(ctx, rawURL, policy)
	if err != nil {
		return ExternalMediaResult{}, err
	}
	if fetched == nil {
		return ExternalMediaResult{}, ErrExternalMediaFetch
	}
	defer fetched.Close()

	hash, err := normalizeVerifiedSHA256(fetched.SHA256Hex)
	if err != nil {
		return ExternalMediaResult{}, ErrExternalMediaFetch
	}
	extension, ok := verifiedMediaExtension(fetched.MediaType)
	if !ok || fetched.Path == "" || fetched.SizeBytes <= 0 {
		return ExternalMediaResult{}, ErrExternalMediaFetch
	}
	key := path.Join("pull", hash+extension)
	if err := putFile(ctx, key, fetched.Path, fetched.MediaType, "public, max-age=86400, immutable"); err != nil {
		return ExternalMediaResult{}, fmt.Errorf("%w: object storage unavailable", ErrExternalMediaUpload)
	}
	return ExternalMediaResult{
		PublicURL: publicURL(key),
		MediaType: fetched.MediaType,
		SizeBytes: fetched.SizeBytes,
		SHA256Hex: hash,
	}, nil
}

func normalizeVerifiedSHA256(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return "", ErrExternalMediaFetch
	}
	return raw, nil
}

func verifiedMediaExtension(mediaType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/gif":
		return ".gif", true
	case "image/webp":
		return ".webp", true
	case "video/mp4":
		return ".mp4", true
	case "video/quicktime":
		return ".mov", true
	case "video/webm":
		return ".webm", true
	default:
		return "", false
	}
}
