// post_metadata.go is the canonical (de)serializer for what we
// persist into social_posts.metadata. Lives in the platform package
// (rather than handler) so both the handler and the scheduler worker
// can import it without forming an import cycle.
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
// DecodePostMetadata reads BOTH so existing scheduled rows survive the
// upgrade — v1 expansion fans out across account_ids using the parent
// post's caption (a single string, since v1 didn't store per-account
// captions).

package platform

import (
	"encoding/json"
	"fmt"
)

// PostMetadataSchemaVersion identifies the JSON shape stored in
// social_posts.metadata. Bump on additive changes (existing fields
// keep working); bump the major when changing field meanings.
const PostMetadataSchemaVersion = 2

// postMetadataV2 mirrors a parsed publish request but is JSON-friendly
// and explicitly versioned.
type postMetadataV2 struct {
	SchemaVersion int                          `json:"schema_version"`
	PlatformPosts []postMetadataPlatformPostV2 `json:"platform_posts"`
}

type postMetadataPlatformPostV2 struct {
	AccountID       string         `json:"account_id"`
	Caption         string         `json:"caption"`
	MediaURLs       []string       `json:"media_urls,omitempty"`
	MediaIDs        []string       `json:"media_ids,omitempty"`
	PlatformOptions map[string]any `json:"platform_options,omitempty"`
	InReplyTo       string         `json:"in_reply_to,omitempty"`
	ThreadPosition  int            `json:"thread_position,omitempty"`
}

// EncodePostMetadata serializes a parsed request's posts into the v2
// metadata blob. Used for both scheduled posts (so the scheduler can
// reconstruct the request) and immediate posts (so reads / replays
// can recover what was originally sent).
func EncodePostMetadata(posts []PlatformPostInput) ([]byte, error) {
	out := postMetadataV2{
		SchemaVersion: PostMetadataSchemaVersion,
		PlatformPosts: make([]postMetadataPlatformPostV2, 0, len(posts)),
	}
	for _, p := range posts {
		out.PlatformPosts = append(out.PlatformPosts, postMetadataPlatformPostV2{
			AccountID:       p.AccountID,
			Caption:         p.Caption,
			MediaURLs:       p.MediaURLs,
			MediaIDs:        p.MediaIDs,
			PlatformOptions: p.PlatformOptions,
			InReplyTo:       p.InReplyTo,
			ThreadPosition:  p.ThreadPosition,
		})
	}
	return json.Marshal(out)
}

// DecodePostMetadata is the inverse: turn a stored metadata blob back
// into a slice of PlatformPostInput. Reads BOTH the v2 schema and the
// legacy v1 schema (the latter requires a fallback caption from the
// parent post since v1 didn't store per-account captions).
//
// fallbackCaption is the parent post's caption — used only by the v1
// expansion path to give every fanned-out account something to post.
// v2 reads ignore it.
func DecodePostMetadata(raw []byte, fallbackCaption string) ([]PlatformPostInput, error) {
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
		out := make([]PlatformPostInput, 0, len(v2.PlatformPosts))
		for _, p := range v2.PlatformPosts {
			out = append(out, PlatformPostInput{
				AccountID:       p.AccountID,
				Caption:         p.Caption,
				MediaURLs:       p.MediaURLs,
				MediaIDs:        p.MediaIDs,
				PlatformOptions: p.PlatformOptions,
				InReplyTo:       p.InReplyTo,
				ThreadPosition:  p.ThreadPosition,
			})
		}
		return out, nil
	}

	// v1 (legacy): account_ids + per-platform options. Expand into one
	// PlatformPostInput per account, all sharing the parent caption.
	// per-platform options can't be pre-resolved here because we don't
	// know each account's platform without a DB lookup — the scheduler
	// resolves it after fetching the account row.
	var v1 struct {
		AccountIDs      []string                  `json:"account_ids"`
		PlatformOptions map[string]map[string]any `json:"platform_options"`
		MediaUrls       []string                  `json:"media_urls"`
	}
	if err := json.Unmarshal(raw, &v1); err != nil {
		return nil, fmt.Errorf("decode metadata v1: %w", err)
	}
	out := make([]PlatformPostInput, 0, len(v1.AccountIDs))
	for _, id := range v1.AccountIDs {
		out = append(out, PlatformPostInput{
			AccountID: id,
			Caption:   fallbackCaption,
			MediaURLs: v1.MediaUrls,
		})
	}
	return out, nil
}

// LegacyV1Metadata exposes the v1 platform_options map keyed by
// platform name, so callers (the scheduler) can resolve per-platform
// options on a v1 row after fetching each account's platform from
// the DB. Returns nil for v2 rows since they encode platform options
// per-post directly.
func LegacyV1Metadata(raw []byte) map[string]map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var head struct {
		SchemaVersion int `json:"schema_version"`
	}
	_ = json.Unmarshal(raw, &head)
	if head.SchemaVersion >= 2 {
		return nil
	}
	var v1 struct {
		PlatformOptions map[string]map[string]any `json:"platform_options"`
	}
	_ = json.Unmarshal(raw, &v1)
	return v1.PlatformOptions
}
