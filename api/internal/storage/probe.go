// probe.go reads enough of an uploaded video object out of R2 to
// extract its visual width / height / duration so the validator can
// pre-flight per-platform placement rules (Facebook feed vs reel,
// Instagram reels, etc.) BEFORE the publish call hits the network.
//
// Why this exists: Facebook silently re-routes vertical 9:16 videos
// posted via /{page_id}/videos into the Reels publishing pipeline.
// Without metadata up-front, the user discovers this only when the
// reclassified post then fails for an unrelated reason (missing app
// permission, scheduled publish, etc.) — see internal/platform/
// facebook.go around the "feed video reclassified as Reel" branch.
// With metadata up-front, the validator rejects the bad combination
// at submit time so the request never reaches Meta in a state Meta
// will mutate.
//
// What it can probe: any ISOBMFF container go-mp4 understands —
// mp4 / mov / m4v with the moov atom in the first VideoProbeMaxBytes
// bytes. That's the overwhelming majority of social uploads (any
// encoder run with `+faststart`, which iOS / ffmpeg / Premiere all
// default to for streaming-friendly output).
//
// What it cannot probe (returns zero metadata + nil error so the
// validator falls back to a warning rather than blocking):
//   - non-mp4 containers (webm, mkv) — go-mp4 doesn't parse them.
//   - "moov-at-end" QuickTime files that don't fit in the first
//     VideoProbeMaxBytes bytes. A future PR can add a tail-fetch
//     fallback if this turns out to be common in the wild.
//   - corrupt / truncated files. The hydrate path treats this as
//     "uploaded but not pre-flightable", same as the format case.
//
// A non-nil error from ProbeVideo means R2 itself is unreachable
// (network failure, 5xx, missing object). Callers should treat that
// as a transient problem worth retrying, not a permanent skip.

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	mp4 "github.com/abema/go-mp4"
)

// VideoProbeMaxBytes caps how many bytes ProbeVideo pulls down for
// parsing. 16 MB comfortably fits the moov atom of a 90-minute
// 4K HEVC clip with `+faststart`; bigger would be wasteful for the
// common case and would push us toward a streaming parse for the
// rare case. We accept that videos with moov-at-end > 16 MB into
// the file return zero metadata — see the package doc.
const VideoProbeMaxBytes = 16 * 1024 * 1024

// VideoMetadata is the subset of probe output the validator needs.
// All fields are zero when probing wasn't possible — callers MUST
// treat (0, 0, 0) as "unknown", not "valid zero-size video".
type VideoMetadata struct {
	// Width is the visual width in pixels, taken from the largest
	// tkhd box (i.e. the video track; audio tracks have 0,0). Read
	// from the 16.16 fixed-point field as the integer part — sub-
	// pixel widths are nonsense for our purposes.
	Width int
	// Height is the visual height in pixels (same source as Width).
	Height int
	// DurationMS is movie-level duration converted to milliseconds
	// from mvhd.Duration / mvhd.Timescale. We use ms (not seconds)
	// so the validator can express FB's 90-second Reel cap and
	// TikTok's 60-second Story cap exactly.
	DurationMS int
}

// HasDimensions reports whether width AND height look usable. Used
// by the validator to decide between "validate against placement
// specs" and "fall back to a warning".
func (m VideoMetadata) HasDimensions() bool {
	return m.Width > 0 && m.Height > 0
}

// ProbeVideo fetches the first VideoProbeMaxBytes of the object at
// `key` from R2 and parses it for visual dimensions + duration. See
// the package doc for the failure-mode contract.
func (c *Client) ProbeVideo(ctx context.Context, key string) (VideoMetadata, error) {
	if c == nil {
		return VideoMetadata{}, ErrNotConfigured
	}

	// Request only the bytes we'll actually parse. R2 honors the
	// HTTP Range header, so this avoids streaming an entire 1 GB
	// upload through the API just to read the moov atom.
	rangeHdr := fmt.Sprintf("bytes=0-%d", VideoProbeMaxBytes-1)
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Range:  aws.String(rangeHdr),
	})
	if err != nil {
		return VideoMetadata{}, fmt.Errorf("storage: probe video get %s: %w", key, err)
	}
	defer out.Body.Close()

	// Bound memory to VideoProbeMaxBytes even if R2 returns more
	// than we asked for (it shouldn't, but defense in depth — a
	// surprise OOM here would crash the API server).
	buf, err := io.ReadAll(io.LimitReader(out.Body, VideoProbeMaxBytes))
	if err != nil {
		return VideoMetadata{}, fmt.Errorf("storage: probe video read %s: %w", key, err)
	}

	return parseVideoMetadata(bytes.NewReader(buf)), nil
}

// parseVideoMetadata is the IO-free half of ProbeVideo, split out
// so probe_test.go can exercise it against fixture bytes without
// needing a live R2 client.
//
// Returns VideoMetadata{} on any parse failure — the contract is
// "no metadata, no error" so the hydrate path can persist the row
// as 'uploaded' even when probing didn't yield anything useful.
func parseVideoMetadata(r io.ReadSeeker) VideoMetadata {
	paths := []mp4.BoxPath{
		{mp4.BoxTypeMoov(), mp4.BoxTypeMvhd()},
		{mp4.BoxTypeMoov(), mp4.BoxTypeTrak(), mp4.BoxTypeTkhd()},
	}
	bips, err := mp4.ExtractBoxesWithPayload(r, nil, paths)
	if err != nil {
		return VideoMetadata{}
	}

	var meta VideoMetadata
	for _, bip := range bips {
		switch p := bip.Payload.(type) {
		case *mp4.Mvhd:
			if p.Timescale > 0 {
				// uint64 multiply before the divide — for a 4-hour
				// video at timescale 600 the intermediate is well
				// under uint64 max. Cast to int at the end so the
				// column type matches.
				ms := p.GetDuration() * 1000 / uint64(p.Timescale)
				meta.DurationMS = int(ms)
			}
		case *mp4.Tkhd:
			// tkhd is per-track; a normal mp4 has one video and one
			// audio track. Audio tkhd width/height are always zero,
			// so we keep the largest non-zero pair as "the video".
			// Multi-video container (rare — typically PiP / multicam
			// experiments) keeps the visually largest track, which
			// is the right choice for aspect-ratio routing.
			w := int(p.GetWidthInt())
			h := int(p.GetHeightInt())
			if w > 0 && h > 0 && w*h > meta.Width*meta.Height {
				meta.Width = w
				meta.Height = h
			}
		}
	}
	return meta
}
