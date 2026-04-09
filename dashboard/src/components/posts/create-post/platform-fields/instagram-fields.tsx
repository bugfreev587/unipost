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
      <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
        Media type
      </label>
      <div className="grid grid-cols-3 gap-1 p-1 bg-[#0a0a0b] rounded-md border border-[#22222a]">
        {MEDIA_TYPES.map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => onChange({ mediaType: t })}
            className={`rounded py-1.5 text-xs font-medium transition-all duration-[160ms] ${
              fields.mediaType === t
                ? "bg-[#f4f4f5] text-[#0a0a0b]"
                : "text-[#8a8a93] hover:text-[#f4f4f5]"
            }`}
          >
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>
    </div>
  );
}
