"use client";

import type { PlatformOverride } from "../use-create-post-form";

interface YouTubeFieldsProps {
  fields: NonNullable<PlatformOverride["youtube"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["youtube"]>>) => void;
}

const CATEGORIES = ["People & Blogs", "Science & Technology", "Education", "Entertainment", "Gaming", "Music", "News & Politics", "Sports"];
const VISIBILITY_OPTIONS = ["public", "unlisted", "private"] as const;

export function YouTubeFields({ fields, onChange }: YouTubeFieldsProps) {
  return (
    <>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
            Video title
          </label>
          <input
            type="text"
            placeholder="Custom title…"
            value={fields.title}
            onChange={(e) => onChange({ title: e.target.value })}
            className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)] placeholder:text-[#55555c]"
          />
        </div>
        <div>
          <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
            Category
          </label>
          <select
            value={fields.category}
            onChange={(e) => onChange({ category: e.target.value })}
            className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]"
          >
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>{c}</option>
            ))}
          </select>
        </div>
      </div>

      <div>
        <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
          Visibility
        </label>
        <div className="grid grid-cols-3 gap-1 p-1 bg-[#0a0a0b] rounded-md border border-[#22222a]">
          {VISIBILITY_OPTIONS.map((v) => (
            <button
              key={v}
              type="button"
              onClick={() => onChange({ visibility: v })}
              className={`rounded py-1.5 text-xs font-medium transition-all duration-[160ms] ${
                fields.visibility === v
                  ? "bg-[#f4f4f5] text-[#0a0a0b]"
                  : "text-[#8a8a93] hover:text-[#f4f4f5]"
              }`}
            >
              {v.charAt(0).toUpperCase() + v.slice(1)}
            </button>
          ))}
        </div>
        <p className="text-[11px] text-[#55555c] mt-2">
          Videos under 3 minutes are automatically published as Shorts.
        </p>
      </div>
    </>
  );
}
