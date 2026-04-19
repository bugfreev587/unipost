"use client";

import type { SocialPostValidationIssue } from "@/lib/api";
import type { PlatformOverride } from "../use-create-post-form";

interface InstagramFieldsProps {
  fields: NonNullable<PlatformOverride["instagram"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["instagram"]>>) => void;
  issues?: SocialPostValidationIssue[];
}

const MEDIA_TYPES = ["feed", "reels", "story"] as const;

function firstIssue(issues: SocialPostValidationIssue[], ...fields: string[]) {
  return issues.find((issue) => fields.includes(issue.field) && issue.severity === "error");
}

function FieldError({ issue }: { issue?: SocialPostValidationIssue }) {
  if (!issue) return null;
  return <p className="mt-1.5 text-[11px] leading-relaxed text-[#fca5a5]">{issue.message}</p>;
}

export function InstagramFields({ fields, onChange, issues = [] }: InstagramFieldsProps) {
  const mediaTypeIssue = firstIssue(issues, "platform_options.mediaType", "platform_options.media_type", "mediaType", "media_type");
  return (
    <div>
      <label
        className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]"
        style={{ color: mediaTypeIssue ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted2)" }}
      >
        Media type
      </label>
      <div
        className="grid grid-cols-3 gap-1 rounded-md border p-1"
        style={{ background: "var(--surface1)", borderColor: mediaTypeIssue ? "var(--danger)" : "var(--dborder)" }}
      >
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
      <FieldError issue={mediaTypeIssue} />
    </div>
  );
}
