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
//   - Inline placement guidance — when we have measured video
//     dimensions (from the client-side <video> probe in
//     use-create-post-form.ts), we evaluate the same feed/reel specs
//     the backend validator uses and surface a one-click "switch to
//     {Reel|Feed}" recommendation when the current placement won't
//     work. This catches Meta's silent feed→reel reclassification at
//     the composer instead of at submit time.
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
  // Measured metadata for the primary video — null when no video is
  // attached, individual fields null when probing hasn't completed
  // or the file isn't decodable. The placement-guidance block stays
  // hidden until BOTH width and height are known.
  videoMetadata: {
    width: number | null;
    height: number | null;
    durationSec: number | null;
  } | null;
}

// FB_PLACEMENT_SPECS mirrors the canonical specs in
// api/internal/platform/capabilities.go so this UI evaluates the
// same constraints the server-side validator does. Any drift would
// surface as "UI says fine, server rejects" — keeping the values in
// sync is part of the contract; if they diverge, treat the backend
// as authoritative and update this map.
const FB_PLACEMENT_SPECS = {
  feed: {
    displayName: "Facebook Feed",
    minAspectRatio: 1.0, // square or wider; vertical = Reel reclassification
    maxAspectRatio: 0, // unbounded
    minWidth: 0,
    minHeight: 0,
    minDurationSec: 1,
    maxDurationSec: 240 * 60,
  },
  reel: {
    displayName: "Facebook Reel",
    minAspectRatio: 0.5, // ~9:16 — interval absorbs encoder rounding
    maxAspectRatio: 0.62,
    minWidth: 540,
    minHeight: 960,
    minDurationSec: 3,
    maxDurationSec: 90,
  },
} as const;

type Placement = keyof typeof FB_PLACEMENT_SPECS;

interface PlacementCheck {
  ok: boolean;
  // Reasons accumulate ALL failing constraints so the user sees every
  // problem in one shot rather than fixing one and discovering the
  // next on the next render.
  reasons: string[];
}

// checkPlacement returns whether (width, height, duration) clears the
// constraints for `placement`. Any null input means "we don't know" —
// we treat unknowns as "skip that check" and lean toward `ok: true`,
// matching the backend's "warn, don't block" stance for unprobed
// videos. Caller should suppress the UI entirely when width/height
// are both null.
function checkPlacement(
  width: number | null,
  height: number | null,
  durationSec: number | null,
  placement: Placement,
): PlacementCheck {
  const spec = FB_PLACEMENT_SPECS[placement];
  const reasons: string[] = [];

  if (width != null && height != null && height > 0) {
    const aspect = width / height;
    if (spec.minAspectRatio > 0 && aspect < spec.minAspectRatio) {
      reasons.push(
        placement === "feed"
          ? "vertical aspect — Facebook will silently reclassify it as a Reel"
          : `aspect ${aspect.toFixed(2)} is wider than Reels accept (need ~9:16)`,
      );
    }
    if (spec.maxAspectRatio > 0 && aspect > spec.maxAspectRatio) {
      reasons.push(
        placement === "reel"
          ? `aspect ${aspect.toFixed(2)} is wider than Reels accept (need ~9:16)`
          : `aspect ${aspect.toFixed(2)} exceeds the ${spec.displayName} maximum`,
      );
    }
    if (spec.minWidth > 0 && width < spec.minWidth) {
      reasons.push(`${width}px wide — ${spec.displayName} requires ${spec.minWidth}px+`);
    }
    if (spec.minHeight > 0 && height < spec.minHeight) {
      reasons.push(`${height}px tall — ${spec.displayName} requires ${spec.minHeight}px+`);
    }
  }
  if (durationSec != null && durationSec > 0) {
    if (spec.minDurationSec > 0 && durationSec < spec.minDurationSec) {
      reasons.push(`${durationSec.toFixed(1)}s — ${spec.displayName} needs at least ${spec.minDurationSec}s`);
    }
    if (spec.maxDurationSec > 0 && durationSec > spec.maxDurationSec) {
      reasons.push(`${durationSec.toFixed(1)}s — ${spec.displayName} caps at ${spec.maxDurationSec}s`);
    }
  }
  return { ok: reasons.length === 0, reasons };
}

// formatAspect turns a (w, h) pair into a human label — exact match
// against common social aspects ("9:16", "16:9", "1:1") when possible,
// fallback to a decimal ratio. Used in the placement chip so users
// see "9:16" instead of "0.56".
function formatAspect(width: number, height: number): string {
  if (width <= 0 || height <= 0) return "";
  const ratio = width / height;
  // Common social aspects with a small tolerance (encoders are sloppy).
  const candidates: Array<[number, string]> = [
    [9 / 16, "9:16"],
    [16 / 9, "16:9"],
    [4 / 5, "4:5"],
    [1, "1:1"],
    [3 / 4, "3:4"],
    [4 / 3, "4:3"],
  ];
  for (const [target, label] of candidates) {
    if (Math.abs(ratio - target) / target < 0.02) return label;
  }
  return ratio.toFixed(2);
}

export function FacebookFields({
  fields,
  onChange,
  mediaAttached,
  videoAttached,
  videoMetadata,
}: FacebookFieldsProps) {
  const mediaType: Placement = fields.mediaType || "feed";
  const reelSelected = mediaType === "reel";

  // Placement guidance is only meaningful when (1) a video is
  // attached and (2) we have measured dimensions. Duration alone
  // isn't enough — the most common bad case (vertical 9:16 to feed)
  // is detected from aspect ratio, which needs both axes.
  const probeReady =
    videoAttached &&
    videoMetadata != null &&
    typeof videoMetadata.width === "number" &&
    typeof videoMetadata.height === "number" &&
    videoMetadata.width > 0 &&
    videoMetadata.height > 0;

  const currentCheck = probeReady
    ? checkPlacement(
        videoMetadata!.width,
        videoMetadata!.height,
        videoMetadata!.durationSec,
        mediaType,
      )
    : null;

  // Identify a recommended switch when the current placement fails.
  // We only suggest a switch if the OTHER placement would actually
  // pass — otherwise the suggestion would just lead to a different
  // error, and silence is better than misleading guidance.
  const otherPlacement: Placement = reelSelected ? "feed" : "reel";
  const otherCheck = probeReady
    ? checkPlacement(
        videoMetadata!.width,
        videoMetadata!.height,
        videoMetadata!.durationSec,
        otherPlacement,
      )
    : null;
  const suggestSwitch =
    currentCheck != null &&
    !currentCheck.ok &&
    otherCheck != null &&
    otherCheck.ok;

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

          {/* Probed-dimensions chip — shows the measured video's
              shape so users can sanity-check that we read the file
              correctly before they trust the placement check. */}
          {probeReady && (
            <div className="mt-2 flex items-center gap-2 text-[11px]" style={{ color: "var(--dmuted)" }}>
              <span
                className="inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 font-mono"
                style={{
                  borderColor: "var(--dborder)",
                  background: "color-mix(in srgb, var(--surface1) 60%, transparent)",
                }}
              >
                <span>{videoMetadata!.width}×{videoMetadata!.height}</span>
                <span style={{ color: "var(--dmuted2)" }}>•</span>
                <span>{formatAspect(videoMetadata!.width!, videoMetadata!.height!)}</span>
                {typeof videoMetadata!.durationSec === "number" && (
                  <>
                    <span style={{ color: "var(--dmuted2)" }}>•</span>
                    <span>{videoMetadata!.durationSec.toFixed(1)}s</span>
                  </>
                )}
              </span>
              {currentCheck?.ok && (
                <span style={{ color: "color-mix(in srgb, var(--success) 70%, var(--dtext))" }}>
                  fits {FB_PLACEMENT_SPECS[mediaType].displayName}
                </span>
              )}
            </div>
          )}

          {/* Inline placement-mismatch banner. Mirrors the server
              validator's error code but renders BEFORE submit so
              the user can fix it without a round-trip. */}
          {currentCheck != null && !currentCheck.ok && (
            <div
              className="mt-2 rounded-md border px-3 py-2 text-[12px] leading-relaxed"
              style={{
                background: "color-mix(in srgb, var(--warning) 10%, var(--surface-raised))",
                borderColor: "color-mix(in srgb, var(--warning) 50%, transparent)",
                color: "color-mix(in srgb, var(--warning) 30%, white)",
              }}
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="font-semibold">
                    {`Won't publish as ${FB_PLACEMENT_SPECS[mediaType].displayName}`}
                  </div>
                  <ul className="mt-1 list-disc pl-4">
                    {currentCheck.reasons.map((r) => (
                      <li key={r}>{r}</li>
                    ))}
                  </ul>
                </div>
                {suggestSwitch && (
                  <button
                    type="button"
                    onClick={() => onChange({ mediaType: otherPlacement })}
                    className="flex-shrink-0 rounded border px-2 py-1 text-[11px] font-medium transition-colors hover:opacity-80"
                    style={{
                      borderColor: "color-mix(in srgb, var(--warning) 60%, transparent)",
                      background: "transparent",
                      color: "inherit",
                    }}
                  >
                    Switch to {FB_PLACEMENT_SPECS[otherPlacement].displayName.replace("Facebook ", "")}
                  </button>
                )}
              </div>
            </div>
          )}
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
