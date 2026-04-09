"use client";

import type { PlatformOverride } from "../use-create-post-form";

interface LinkedInFieldsProps {
  fields: NonNullable<PlatformOverride["linkedin"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["linkedin"]>>) => void;
}

export function LinkedInFields({ fields, onChange }: LinkedInFieldsProps) {
  return (
    <div>
      <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
        Visibility
      </label>
      <select
        value={fields.visibility}
        onChange={(e) => onChange({ visibility: e.target.value as "anyone" | "connections" })}
        className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]"
      >
        <option value="anyone">Anyone</option>
        <option value="connections">Connections only</option>
      </select>
    </div>
  );
}
