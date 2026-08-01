package storage

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/safefetch"
)

var (
	ErrExternalMediaFetch  = errors.New("storage: verified external media fetch failed")
	ErrExternalMediaUpload = errors.New("storage: verified external media upload failed")
)

type ExternalMediaResult struct {
	PublicURL string
	ObjectKey string
	MediaType string
	SizeBytes int64
	SHA256Hex string
}

type verifiedFileUploader func(context.Context, string, string, string, string) error
type publicURLBuilder func(string) string

const publishingObjectLifecycleWriteTimeout = 5 * time.Second
const publishingObjectUploadTimeout = 10 * time.Minute

// externalMediaFetchTimeout bounds a whole verified fetch. It deliberately
// matches the R2 upload bound rather than the safefetch library default:
// staging accepts media up to the platform capability limit, and a 30s wall
// clock turned every large video into a permanent retry loop even though the
// bytes were arriving. Dead, stalled, and slow-loris sources still fail fast
// through the unchanged connect, response-header, and idle-read budgets.
const externalMediaFetchTimeout = 10 * time.Minute

// ExternalFetchConfig builds the safe-fetch budget used for request-supplied
// media. Both knobs are optional and exist so a deployment can shorten the
// budget or move staging scratch space onto a dedicated volume without a
// release.
func ExternalFetchConfig() safefetch.Config {
	config := safefetch.DefaultConfig()
	config.TotalTimeout = externalMediaFetchTimeout
	if override := parseDurationEnv("EXTERNAL_MEDIA_FETCH_TIMEOUT"); override > 0 {
		config.TotalTimeout = override
	}
	config.TempDir = strings.TrimSpace(os.Getenv("EXTERNAL_MEDIA_TEMP_DIR"))
	return config
}

// externalMediaByteCeiling is a deployment-wide clamp on how many bytes a
// single staging attempt may buffer on local disk. Zero (the default) keeps
// the caller's platform capability limit unchanged.
func externalMediaByteCeiling() int64 {
	raw := strings.TrimSpace(os.Getenv("EXTERNAL_MEDIA_MAX_BYTES"))
	if raw == "" {
		return 0
	}
	ceiling, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ceiling <= 0 {
		return 0
	}
	return ceiling
}

// clampPolicyBytes applies the deployment ceiling to a caller-supplied policy.
// It only ever lowers limits, so an unset ceiling leaves the policy identical.
func clampPolicyBytes(policy safefetch.Policy, ceiling int64) safefetch.Policy {
	if ceiling <= 0 {
		return policy
	}
	if policy.MaxBytes <= 0 || policy.MaxBytes > ceiling {
		policy.MaxBytes = ceiling
	}
	if len(policy.MaxBytesByMediaType) == 0 {
		return policy
	}
	clamped := make(map[string]int64, len(policy.MaxBytesByMediaType))
	for mediaType, limit := range policy.MaxBytesByMediaType {
		if limit <= 0 || limit > ceiling {
			limit = ceiling
		}
		clamped[mediaType] = limit
	}
	policy.MaxBytesByMediaType = clamped
	return policy
}

func parseDurationEnv(name string) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func (c *Client) StageExternalMedia(ctx context.Context, rawURL string, policy safefetch.Policy) (ExternalMediaResult, error) {
	if c == nil {
		return ExternalMediaResult{}, ErrNotConfigured
	}
	if c.externalFetcher == nil {
		return ExternalMediaResult{}, ErrExternalMediaFetch
	}
	policy = clampPolicyBytes(policy, externalMediaByteCeiling())
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
	lifecycle, ok := publishingObjectContextFromContext(ctx)
	if !ok {
		return ExternalMediaResult{}, ErrExternalMediaLifecycle
	}
	reservation := PublishingObjectReservation{
		ObjectKey:   key,
		WorkspaceID: lifecycle.WorkspaceID,
		PostID:      lifecycle.PostID,
		ContentType: fetched.MediaType,
		SizeBytes:   fetched.SizeBytes,
	}
	usageID, err := lifecycle.Lifecycle.ReservePublishingObject(ctx, reservation)
	if err != nil || strings.TrimSpace(usageID) == "" {
		return ExternalMediaResult{}, fmt.Errorf("%w: reservation failed", ErrExternalMediaLifecycle)
	}
	uploadCtx, cancelUpload := context.WithTimeout(ctx, publishingObjectUploadTimeout)
	uploadErr := putFile(uploadCtx, key, fetched.Path, fetched.MediaType, "public, max-age=86400, immutable")
	cancelUpload()
	if uploadErr != nil {
		if abandonErr := abandonPublishingObjectDetached(ctx, lifecycle.Lifecycle, usageID); abandonErr != nil {
			return ExternalMediaResult{}, errors.Join(
				fmt.Errorf("%w: object storage unavailable", ErrExternalMediaUpload),
				fmt.Errorf("%w: failed upload reservation could not be released", ErrExternalMediaLifecycle),
			)
		}
		return ExternalMediaResult{}, fmt.Errorf("%w: object storage unavailable", ErrExternalMediaUpload)
	}
	activateCtx, cancelActivate := detachedPublishingLifecycleContext(ctx)
	activateErr := lifecycle.Lifecycle.ActivatePublishingObject(activateCtx, usageID)
	cancelActivate()
	if activateErr != nil {
		if abandonErr := abandonPublishingObjectDetached(ctx, lifecycle.Lifecycle, usageID); abandonErr != nil {
			return ExternalMediaResult{}, errors.Join(
				fmt.Errorf("%w: uploaded object could not be activated", ErrExternalMediaLifecycle),
				fmt.Errorf("%w: pending upload reservation could not be released", ErrExternalMediaLifecycle),
			)
		}
		return ExternalMediaResult{}, fmt.Errorf("%w: uploaded object could not be activated", ErrExternalMediaLifecycle)
	}
	return ExternalMediaResult{
		PublicURL: publicURL(key),
		ObjectKey: key,
		MediaType: fetched.MediaType,
		SizeBytes: fetched.SizeBytes,
		SHA256Hex: hash,
	}, nil
}

func abandonPublishingObjectDetached(ctx context.Context, lifecycle PublishingObjectLifecycle, usageID string) error {
	abandonCtx, cancel := detachedPublishingLifecycleContext(ctx)
	defer cancel()
	return lifecycle.AbandonPublishingObject(abandonCtx, usageID)
}

func detachedPublishingLifecycleContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), publishingObjectLifecycleWriteTimeout)
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
