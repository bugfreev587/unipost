"use client";

import type { PlatformOverride } from "../use-create-post-form";

interface TikTokFieldsProps {
  fields: NonNullable<PlatformOverride["tiktok"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["tiktok"]>>) => void;
}

export function TikTokFields({ fields, onChange }: TikTokFieldsProps) {
  return (
    <div className="grid grid-cols-2 gap-3">
      <div>
        <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
          Privacy
        </label>
        <select
          value={fields.privacy}
          onChange={(e) => onChange({ privacy: e.target.value as "public" | "friends" | "private" })}
          className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]"
        >
          <option value="public">Public</option>
          <option value="friends">Friends</option>
          <option value="private">Private</option>
        </select>
      </div>
      <div>
        <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
          Interactions
        </label>
        <select
          value={fields.interactions}
          onChange={(e) => onChange({ interactions: e.target.value as "allow_all" | "comments_only" | "disable_all" })}
          className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]"
        >
          <option value="allow_all">Allow comments, duet, stitch</option>
          <option value="comments_only">Comments only</option>
          <option value="disable_all">Disable all</option>
        </select>
      </div>
    </div>
  );
}
