// social_posts_metadata.go houses the persisted-metadata shape for
// social_posts.metadata. We bump SchemaVersion when the shape changes
// so the scheduler (which reads stale rows) can detect old vs new and
// take the right path.

package handler

import (
	"encoding/json"
	"fmt"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// PostMetadataSchemaVersion identifies the JSON shape stored in
// social_posts.metadata. Bump on additive changes (existing fields
// keep working); bump the major when changing field meanings.
//
// Schema 1 (legacy, before Sprint 1):
//
//	{ "account_ids": ["...", "..."], "platform_options": { "tiktok": {...} } }
//
// Schema 2 (Sprint 1, AgentPost-aware):
//
//	{
//	  "schema_version": 2,
//	  "platform_posts": [
//	    { "account_id": "...", "caption": "...", "media_urls": [...],
//	      "platform_options": {...}, "in_reply_to": "..." }
//	  ]
//	}
//
// The scheduler reads both. Schema 1 is detected by the absence of a
// schema_version field; the scheduler expands it to schema-2 shape
// using the parent post's caption.
const PostMetadataSchemaVersion = 2

// postMetadataV2 mirrors a parsedRequest's Posts but is JSON-friendly
// (capital letters dropped via tags) and explicitly versioned.
type postMetadataV2 struct {
	SchemaVersion int                          `json:"schema_version"`
	PlatformPosts []postMetadataPlatformPostV2 `json:"platform_posts"`
}

type postMetadataPlatformPostV2 struct {
	AccountID       string         `json:"account_id"`
	Caption         string         `json:"caption"`
	MediaURLs       []string       `json:"media_urls,omitempty"`
	PlatformOptions map[string]any `json:"platform_options,omitempty"`
	InReplyTo       string         `json:"in_reply_to,omitempty"`
}

// encodePostMetadata serializes a parsed request's posts into the v2
// metadata blob. Used for both scheduled posts (so the scheduler can
// reconstruct the request) and immediate posts (so reads / replays
// can recover what was originally sent).
func encodePostMetadata(posts []platform.PlatformPostInput) ([]byte, error) {
	out := postMetadataV2{
		SchemaVersion: PostMetadataSchemaVersion,
		PlatformPosts: make([]postMetadataPlatformPostV2, 0, len(posts)),
	}
	for _, p := range posts {
		out.PlatformPosts = append(out.PlatformPosts, postMetadataPlatformPostV2{
			AccountID:       p.AccountID,
			Caption:         p.Caption,
			MediaURLs:       p.MediaURLs,
			PlatformOptions: p.PlatformOptions,
			InReplyTo:       p.InReplyTo,
		})
	}
	return json.Marshal(out)
}

// decodePostMetadata is the inverse: turn a stored metadata blob back
// into a slice of PlatformPostInput. Reads BOTH the v2 schema and the
// legacy v1 schema (the latter requires a fallback caption from the
// parent post since v1 didn't store per-account captions).
func decodePostMetadata(raw []byte, fallbackCaption string) ([]platform.PlatformPostInput, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Peek at schema_version first.
	var head struct {
		SchemaVersion int `json:"schema_version"`
	}
	_ = json.Unmarshal(raw, &head)

	if head.SchemaVersion >= 2 {
		var v2 postMetadataV2
		if err := json.Unmarshal(raw, &v2); err != nil {
			return nil, fmt.Errorf("decode metadata v%d: %w", head.SchemaVersion, err)
		}
		out := make([]platform.PlatformPostInput, 0, len(v2.PlatformPosts))
		for _, p := range v2.PlatformPosts {
			out = append(out, platform.PlatformPostInput{
				AccountID:       p.AccountID,
				Caption:         p.Caption,
				MediaURLs:       p.MediaURLs,
				PlatformOptions: p.PlatformOptions,
				InReplyTo:       p.InReplyTo,
			})
		}
		return out, nil
	}

	// v1 (legacy): account_ids + per-platform options. Expand into one
	// PlatformPostInput per account, all sharing the parent caption.
	var v1 struct {
		AccountIDs      []string                    `json:"account_ids"`
		PlatformOptions map[string]map[string]any   `json:"platform_options"`
		MediaUrls       []string                    `json:"media_urls"`
	}
	if err := json.Unmarshal(raw, &v1); err != nil {
		return nil, fmt.Errorf("decode metadata v1: %w", err)
	}
	out := make([]platform.PlatformPostInput, 0, len(v1.AccountIDs))
	for _, id := range v1.AccountIDs {
		out = append(out, platform.PlatformPostInput{
			AccountID: id,
			Caption:   fallbackCaption,
			MediaURLs: v1.MediaUrls,
			// platform_options is keyed by platform name, not by
			// account; the scheduler resolves it after reading the
			// account's platform from the DB.
		})
	}
	return out, nil
}
