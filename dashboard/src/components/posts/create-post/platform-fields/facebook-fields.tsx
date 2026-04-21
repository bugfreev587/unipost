"use client";

// FacebookFields renders the Facebook-specific inputs in the compose
// drawer. v1 scope per the PRD:
//   - Optional "Link" input — when set, gets passed through as
//     platform_options.facebook.link. Link + media is disallowed,
//     so the input goes disabled whenever the post has any media
//     attached (handled at the drawer level via `mediaAttached`).
//   - No privacy picker, no category selector, no first-comment —
//     the PRD explicitly keeps Facebook compose as the simplest of
//     every platform we support.
//
// The mutual exclusion copy matches the validator's error message so
// a user who ignores the disabled state and POSTs directly via API
// still sees the same phrasing.

import type { PlatformOverride } from "../use-create-post-form";

interface FacebookFieldsProps {
  fields: NonNullable<PlatformOverride["facebook"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["facebook"]>>) => void;
  // mediaAttached is true whenever the composer has at least one
  // image or video queued — in that case the link field locks and
  // the hint explains why.
  mediaAttached: boolean;
}

export function FacebookFields({ fields, onChange, mediaAttached }: FacebookFieldsProps) {
  return (
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
        placeholder={mediaAttached ? "Remove media to add a link" : "https://example.com"}
        value={mediaAttached ? "" : fields.link || ""}
        disabled={mediaAttached}
        onChange={(e) => onChange({ link: e.target.value })}
        className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms] disabled:opacity-55 disabled:cursor-not-allowed"
        style={{
          background: "var(--surface1)",
          borderColor: "var(--dborder)",
          color: "var(--dtext)",
        }}
      />
      <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
        {mediaAttached
          ? "Facebook doesn't allow a link preview alongside photo or video posts."
          : "Facebook will fetch a preview card for the link automatically."}
      </p>
    </div>
  );
}
