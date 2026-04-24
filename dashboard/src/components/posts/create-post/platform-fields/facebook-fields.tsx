"use client";

// FacebookFields renders the Facebook-specific inputs in the compose
// drawer.
//   - Optional Link input — passes through as
//     platform_options.facebook.link. Link + media is disallowed, so
//     the input locks whenever media is attached.
//   - mediaType toggle (Feed video / Reel) — shown only when a video
//     is attached, since FB Reels require a video and the two
//     publish paths have distinct endpoint behavior on the server.
//     Choosing Reel passes platform_options.facebook.mediaType="reel"
//     which routes the post through /{page_id}/video_reels.
//
// Mutual-exclusion copy matches the validator error messages so the
// same phrasing appears for users who bypass the UI and hit the API.

import type { PlatformOverride } from "../use-create-post-form";

interface FacebookFieldsProps {
  fields: NonNullable<PlatformOverride["facebook"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["facebook"]>>) => void;
  // mediaAttached is true whenever the composer has any image or
  // video queued — in that case the link field locks.
  mediaAttached: boolean;
  // videoAttached is true when media includes at least one video.
  // Controls whether the Reel toggle is rendered; photo-only posts
  // never take the Reel path.
  videoAttached: boolean;
}

export function FacebookFields({ fields, onChange, mediaAttached, videoAttached }: FacebookFieldsProps) {
  const mediaType = fields.mediaType || "feed";
  const reelSelected = mediaType === "reel";

  return (
    <div className="space-y-4">
      {/* mediaType toggle — only shown when a video is attached. */}
      {videoAttached && (
        <div>
          <label
            className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]"
            style={{ color: "var(--dmuted2)" }}
          >
            Publish surface
          </label>
          <div
            className="inline-flex overflow-hidden rounded-md border"
            style={{ borderColor: "var(--dborder)" }}
            role="tablist"
            aria-label="Facebook publish surface"
          >
            <button
              type="button"
              role="tab"
              aria-selected={!reelSelected}
              onClick={() => onChange({ mediaType: "feed" })}
              className="px-3 py-1.5 text-xs font-medium transition-colors"
              style={{
                background: !reelSelected ? "var(--surface2)" : "transparent",
                color: !reelSelected ? "var(--dtext)" : "var(--dmuted)",
              }}
            >
              Feed video
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={reelSelected}
              onClick={() => onChange({ mediaType: "reel" })}
              className="border-l px-3 py-1.5 text-xs font-medium transition-colors"
              style={{
                borderColor: "var(--dborder)",
                background: reelSelected ? "var(--surface2)" : "transparent",
                color: reelSelected ? "var(--dtext)" : "var(--dmuted)",
              }}
            >
              Reel
            </button>
          </div>
          <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
            {reelSelected
              ? "Posts as a vertical Reel via /{page_id}/video_reels. Reels require a video and do not accept a link attachment."
              : "Publishes as a feed video via /{page_id}/videos. Best for horizontal or square clips."}
          </p>
        </div>
      )}

      {/* Link input — disabled when Reel is picked or any media is attached. */}
      <div>
        <label
          className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]"
          style={{ color: "var(--dmuted2)" }}
        >
          Link (optional)
        </label>
        <input
          type="url"
          inputMode="url"
          placeholder={
            reelSelected
              ? "Reels don't accept a link attachment"
              : mediaAttached
                ? "Remove media to add a link"
                : "https://example.com"
          }
          value={reelSelected || mediaAttached ? "" : fields.link || ""}
          disabled={reelSelected || mediaAttached}
          onChange={(e) => onChange({ link: e.target.value })}
          className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms] disabled:opacity-55 disabled:cursor-not-allowed"
          style={{
            background: "var(--surface1)",
            borderColor: "var(--dborder)",
            color: "var(--dtext)",
          }}
        />
        <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
          {reelSelected
            ? "Reels don't carry a separate link preview — drop it into the caption instead."
            : mediaAttached
              ? "Facebook doesn't allow a link preview alongside photo or video posts."
              : "Facebook will fetch a preview card for the link automatically."}
        </p>
      </div>
    </div>
  );
}
