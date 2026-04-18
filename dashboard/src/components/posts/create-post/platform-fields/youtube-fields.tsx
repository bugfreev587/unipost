"use client";

import { useEffect, useState, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";
import type { SocialPostValidationIssue } from "@/lib/api";
import type { PlatformOverride } from "../use-create-post-form";

interface YouTubeFieldsProps {
  fields: NonNullable<PlatformOverride["youtube"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["youtube"]>>) => void;
  issues?: SocialPostValidationIssue[];
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
const LICENSE_OPTIONS = [
  { id: "youtube", label: "Standard YouTube License" },
  { id: "creativeCommon", label: "Creative Commons" },
] as const;

function firstIssue(issues: SocialPostValidationIssue[], ...fields: string[]) {
  return issues.find((issue) => fields.includes(issue.field) && issue.severity === "error");
}

function FieldError({ issue }: { issue?: SocialPostValidationIssue }) {
  if (!issue) return null;
  return <p className="mt-1.5 text-[11px] leading-relaxed text-[#fca5a5]">{issue.message}</p>;
}

function ToggleButton({
  active,
  children,
  onClick,
}: {
  active: boolean;
  children: ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded py-1.5 text-xs font-medium transition-all duration-[160ms] ${
        active ? "bg-[#f4f4f5] text-[#0a0a0b]" : "text-[#8a8a93] hover:text-[#f4f4f5]"
      }`}
    >
      {children}
    </button>
  );
}

export function YouTubeFields({ fields, onChange, issues = [] }: YouTubeFieldsProps) {
  const titleIssue = firstIssue(issues, "platform_options.title", "title");
  const madeForKidsIssue = firstIssue(issues, "platform_options.made_for_kids", "made_for_kids");
  const visibilityIssue = firstIssue(issues, "platform_options.privacy_status", "privacy_status");
  const licenseIssue = firstIssue(issues, "platform_options.license", "license");
  const publishAtIssue = firstIssue(issues, "platform_options.publish_at", "publish_at");
  const defaultLanguageIssue = firstIssue(issues, "platform_options.default_language", "default_language");
  const recordingDateIssue = firstIssue(issues, "platform_options.recording_date", "recording_date");
  const optionalFieldNames = new Set([
    "platform_options.category_id",
    "category_id",
    "platform_options.privacy_status",
    "privacy_status",
    "platform_options.license",
    "license",
    "platform_options.publish_at",
    "publish_at",
    "platform_options.default_language",
    "default_language",
    "platform_options.recording_date",
    "recording_date",
    "platform_options.playlist_id",
    "playlist_id",
    "platform_options.tags",
    "tags",
    "platform_options.notify_subscribers",
    "notify_subscribers",
    "platform_options.embeddable",
    "embeddable",
    "platform_options.public_stats_viewable",
    "public_stats_viewable",
    "platform_options.contains_synthetic_media",
    "contains_synthetic_media",
    "platform_options.shorts",
    "shorts",
  ]);
  const hasOptionalErrors = issues.some(
    (issue) => issue.severity === "error" && optionalFieldNames.has(issue.field)
  );
  const [showOptionalFields, setShowOptionalFields] = useState(hasOptionalErrors);

  useEffect(() => {
    if (hasOptionalErrors) {
      setShowOptionalFields(true);
    }
  }, [hasOptionalErrors]);

  return (
    <div className="space-y-4">
      <div>
        <label
          className={cn(
            "text-[11px] uppercase tracking-wider font-medium block mb-1.5",
            titleIssue ? "text-[#fca5a5]" : "text-[#55555c]"
          )}
        >
          Video title <span className="text-[#f59e0b]">*</span>
        </label>
        <input
          type="text"
          placeholder="Required for YouTube uploads…"
          value={fields.title}
          onChange={(e) => onChange({ title: e.target.value })}
          className={cn(
            "w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)] placeholder:text-[#55555c]",
            titleIssue ? "border-[#ef4444]" : "border-[#22222a]"
          )}
        />
        <FieldError issue={titleIssue} />
      </div>

      <div>
        <label
          className={cn(
            "text-[11px] uppercase tracking-wider font-medium block mb-1.5",
            madeForKidsIssue ? "text-[#fca5a5]" : "text-[#55555c]"
          )}
        >
          Audience <span className="text-[#f59e0b]">*</span>
        </label>
        <div
          className={cn(
            "grid grid-cols-2 gap-1 p-1 bg-[#0a0a0b] rounded-md border",
            madeForKidsIssue ? "border-[#ef4444]" : "border-[#22222a]"
          )}
        >
          <ToggleButton active={fields.madeForKids === "yes"} onClick={() => onChange({ madeForKids: "yes" })}>
            Yes, it&apos;s for kids
          </ToggleButton>
          <ToggleButton active={fields.madeForKids === "no"} onClick={() => onChange({ madeForKids: "no" })}>
            No, it&apos;s not for kids
          </ToggleButton>
        </div>
        <FieldError issue={madeForKidsIssue} />
      </div>

      <div className="rounded-xl border border-[#22222a] bg-[#111114]/70 overflow-hidden">
        <button
          type="button"
          onClick={() => setShowOptionalFields((current) => !current)}
          className="flex w-full items-center justify-between px-3 py-2.5 text-left transition-colors hover:bg-[#17171a]"
        >
          <div>
            <div className="text-[11px] font-mono uppercase tracking-[0.12em] text-[#8a8a93]">
              Optional fields
            </div>
            <div className="mt-1 text-[12px] text-[#55555c]">
              Category, visibility, scheduling, playlists, tags, and advanced YouTube settings.
            </div>
          </div>
          <ChevronDown
            className={cn(
              "h-4 w-4 text-[#8a8a93] transition-transform duration-200",
              showOptionalFields ? "rotate-180" : "rotate-0"
            )}
          />
        </button>

        {showOptionalFields && (
          <div className="border-t border-[#22222a] px-3 py-3 space-y-4">
            <div>
              <label
                className={cn(
                  "text-[11px] uppercase tracking-wider font-medium block mb-1.5",
                  visibilityIssue ? "text-[#fca5a5]" : "text-[#55555c]"
                )}
              >
                Visibility
              </label>
              <div
                className={cn(
                  "grid grid-cols-3 gap-1 p-1 bg-[#0a0a0b] rounded-md border",
                  visibilityIssue ? "border-[#ef4444]" : "border-[#22222a]"
                )}
              >
                {VISIBILITY_OPTIONS.map((v) => (
                  <ToggleButton key={v} active={fields.visibility === v} onClick={() => onChange({ visibility: v })}>
                    {v.charAt(0).toUpperCase() + v.slice(1)}
                  </ToggleButton>
                ))}
              </div>
              <FieldError issue={visibilityIssue} />
            </div>

            <div className="grid grid-cols-2 gap-3">
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
              <div>
                <label
                  className={cn(
                    "text-[11px] uppercase tracking-wider font-medium block mb-1.5",
                    licenseIssue ? "text-[#fca5a5]" : "text-[#55555c]"
                  )}
                >
                  License
                </label>
                <select
                  value={fields.license}
                  onChange={(e) => onChange({ license: e.target.value as NonNullable<PlatformOverride["youtube"]>["license"] })}
                  className={cn(
                    "w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]",
                    licenseIssue ? "border-[#ef4444]" : "border-[#22222a]"
                  )}
                >
                  {LICENSE_OPTIONS.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </select>
                <FieldError issue={licenseIssue} />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label
                  className={cn(
                    "text-[11px] uppercase tracking-wider font-medium block mb-1.5",
                    defaultLanguageIssue ? "text-[#fca5a5]" : "text-[#55555c]"
                  )}
                >
                  Default language
                </label>
                <input
                  type="text"
                  placeholder="en, en-US, zh-CN…"
                  value={fields.defaultLanguage}
                  onChange={(e) => onChange({ defaultLanguage: e.target.value })}
                  className={cn(
                    "w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)] placeholder:text-[#55555c]",
                    defaultLanguageIssue ? "border-[#ef4444]" : "border-[#22222a]"
                  )}
                />
                <FieldError issue={defaultLanguageIssue} />
              </div>
              <div>
                <label
                  className={cn(
                    "text-[11px] uppercase tracking-wider font-medium block mb-1.5",
                    publishAtIssue ? "text-[#fca5a5]" : "text-[#55555c]"
                  )}
                >
                  Publish at
                </label>
                <input
                  type="datetime-local"
                  value={fields.publishAt}
                  onChange={(e) => onChange({ publishAt: e.target.value })}
                  className={cn(
                    "w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]",
                    publishAtIssue ? "border-[#ef4444]" : "border-[#22222a]"
                  )}
                />
                <FieldError issue={publishAtIssue} />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label
                  className={cn(
                    "text-[11px] uppercase tracking-wider font-medium block mb-1.5",
                    recordingDateIssue ? "text-[#fca5a5]" : "text-[#55555c]"
                  )}
                >
                  Recording date
                </label>
                <input
                  type="date"
                  value={fields.recordingDate}
                  onChange={(e) => onChange({ recordingDate: e.target.value })}
                  className={cn(
                    "w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]",
                    recordingDateIssue ? "border-[#ef4444]" : "border-[#22222a]"
                  )}
                />
                <FieldError issue={recordingDateIssue} />
              </div>
              <div>
                <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
                  Playlist ID
                </label>
                <input
                  type="text"
                  placeholder="PLxxxxxxxx"
                  value={fields.playlistId}
                  onChange={(e) => onChange({ playlistId: e.target.value })}
                  className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)] placeholder:text-[#55555c]"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
                  Tags
                </label>
                <input
                  type="text"
                  placeholder="product, quarterly, update"
                  value={fields.tags}
                  onChange={(e) => onChange({ tags: e.target.value })}
                  className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)] placeholder:text-[#55555c]"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <label className="flex items-center gap-2 rounded-md border border-[#22222a] bg-[#0a0a0b] px-3 py-2 text-sm text-[#d4d4d8]">
                <input
                  type="checkbox"
                  checked={fields.notifySubscribers}
                  onChange={(e) => onChange({ notifySubscribers: e.target.checked })}
                  className="h-4 w-4 rounded border-[#3f3f46] bg-[#09090b] text-[#10b981] focus:ring-[#10b981]"
                />
                Notify subscribers
              </label>
              <label className="flex items-center gap-2 rounded-md border border-[#22222a] bg-[#0a0a0b] px-3 py-2 text-sm text-[#d4d4d8]">
                <input
                  type="checkbox"
                  checked={fields.embeddable}
                  onChange={(e) => onChange({ embeddable: e.target.checked })}
                  className="h-4 w-4 rounded border-[#3f3f46] bg-[#09090b] text-[#10b981] focus:ring-[#10b981]"
                />
                Allow embedding
              </label>
              <label className="flex items-center gap-2 rounded-md border border-[#22222a] bg-[#0a0a0b] px-3 py-2 text-sm text-[#d4d4d8]">
                <input
                  type="checkbox"
                  checked={fields.publicStatsViewable}
                  onChange={(e) => onChange({ publicStatsViewable: e.target.checked })}
                  className="h-4 w-4 rounded border-[#3f3f46] bg-[#09090b] text-[#10b981] focus:ring-[#10b981]"
                />
                Show public stats
              </label>
              <label className="flex items-center gap-2 rounded-md border border-[#22222a] bg-[#0a0a0b] px-3 py-2 text-sm text-[#d4d4d8]">
                <input
                  type="checkbox"
                  checked={fields.containsSyntheticMedia}
                  onChange={(e) => onChange({ containsSyntheticMedia: e.target.checked })}
                  className="h-4 w-4 rounded border-[#3f3f46] bg-[#09090b] text-[#10b981] focus:ring-[#10b981]"
                />
                Contains synthetic media
              </label>
              <label className="flex items-center gap-2 rounded-md border border-[#22222a] bg-[#0a0a0b] px-3 py-2 text-sm text-[#d4d4d8]">
                <input
                  type="checkbox"
                  checked={fields.shorts}
                  onChange={(e) => onChange({ shorts: e.target.checked })}
                  className="h-4 w-4 rounded border-[#3f3f46] bg-[#09090b] text-[#10b981] focus:ring-[#10b981]"
                />
                Add Shorts hint
              </label>
            </div>
          </div>
        )}
      </div>

      <p className="text-[11px] text-[#55555c]">
        UniPost sends caption text as YouTube `snippet.description`. `title` and `audience` are required. All other YouTube settings are optional.
      </p>
    </div>
  );
}
