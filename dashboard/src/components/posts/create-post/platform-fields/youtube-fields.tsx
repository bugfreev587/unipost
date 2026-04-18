"use client";

import type { PlatformOverride } from "../use-create-post-form";

interface YouTubeFieldsProps {
  fields: NonNullable<PlatformOverride["youtube"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["youtube"]>>) => void;
}

export const YOUTUBE_CATEGORY_OPTIONS = [
  { id: "22", label: "People & Blogs" },
  { id: "28", label: "Science & Technology" },
  { id: "27", label: "Education" },
  { id: "24", label: "Entertainment" },
  { id: "20", label: "Gaming" },
  { id: "10", label: "Music" },
  { id: "25", label: "News & Politics" },
  { id: "17", label: "Sports" },
] as const;
const VISIBILITY_OPTIONS = ["public", "unlisted", "private"] as const;

export function YouTubeFields({ fields, onChange }: YouTubeFieldsProps) {
  return (
    <>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
            Video title <span className="text-[#f59e0b]">*</span>
          </label>
          <input
            type="text"
            placeholder="Required for YouTube uploads…"
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
            {YOUTUBE_CATEGORY_OPTIONS.map((c) => (
              <option key={c.id} value={c.id}>{c.label}</option>
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
          UniPost uses this as YouTube `snippet.title`. Videos under 3 minutes are automatically published as Shorts.
        </p>
      </div>
    </>
  );
}
