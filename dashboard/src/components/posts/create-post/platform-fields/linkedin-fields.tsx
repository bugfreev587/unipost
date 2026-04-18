"use client";

import type { PlatformOverride } from "../use-create-post-form";

interface LinkedInFieldsProps {
  fields: NonNullable<PlatformOverride["linkedin"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["linkedin"]>>) => void;
}

export function LinkedInFields({ fields, onChange }: LinkedInFieldsProps) {
  return (
    <div>
      <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider" style={{ color: "var(--dmuted2)" }}>
        Visibility
      </label>
      <select
        value={fields.visibility}
        onChange={(e) => onChange({ visibility: e.target.value as "anyone" | "connections" })}
        className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
        style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
      >
        <option value="anyone">Anyone</option>
        <option value="connections">Connections only</option>
      </select>
    </div>
  );
}
