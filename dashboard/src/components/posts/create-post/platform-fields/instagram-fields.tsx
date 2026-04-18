"use client";

import type { PlatformOverride } from "../use-create-post-form";

interface InstagramFieldsProps {
  fields: NonNullable<PlatformOverride["instagram"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["instagram"]>>) => void;
}

const MEDIA_TYPES = ["feed", "reels", "story"] as const;

export function InstagramFields({ fields, onChange }: InstagramFieldsProps) {
  return (
    <div>
      <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider" style={{ color: "var(--dmuted2)" }}>
        Media type
      </label>
      <div className="grid grid-cols-3 gap-1 rounded-md border p-1" style={{ background: "var(--surface1)", borderColor: "var(--dborder)" }}>
        {MEDIA_TYPES.map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => onChange({ mediaType: t })}
            className="rounded py-1.5 text-xs font-medium transition-all duration-[160ms]"
            style={
              fields.mediaType === t
                ? { background: "var(--surface3)", color: "var(--dtext)" }
                : { color: "var(--dmuted)" }
            }
          >
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>
    </div>
  );
}
