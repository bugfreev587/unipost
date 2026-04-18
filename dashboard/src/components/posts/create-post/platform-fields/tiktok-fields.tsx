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
        <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider" style={{ color: "var(--dmuted2)" }}>
          Privacy
        </label>
        <select
          value={fields.privacy}
          onChange={(e) => onChange({ privacy: e.target.value as "public" | "friends" | "private" })}
          className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
          style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
        >
          <option value="public">Public</option>
          <option value="friends">Friends</option>
          <option value="private">Private</option>
        </select>
      </div>
      <div>
        <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider" style={{ color: "var(--dmuted2)" }}>
          Interactions
        </label>
        <select
          value={fields.interactions}
          onChange={(e) => onChange({ interactions: e.target.value as "allow_all" | "comments_only" | "disable_all" })}
          className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
          style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
        >
          <option value="allow_all">Allow comments, duet, stitch</option>
          <option value="comments_only">Comments only</option>
          <option value="disable_all">Disable all</option>
        </select>
      </div>
    </div>
  );
}
