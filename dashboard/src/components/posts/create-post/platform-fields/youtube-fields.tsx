"use client";

import { useState, type ReactNode } from "react";
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
  return (
    <p
      className="mt-1.5 text-[11px] leading-relaxed"
      style={{ color: "color-mix(in srgb, var(--danger) 45%, white)" }}
    >
      {issue.message}
    </p>
  );
}

const INPUT_CLASS =
  "w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]";

function inputStyle(hasError: boolean): React.CSSProperties {
  return {
    background: "var(--surface1)",
    borderColor: hasError ? "var(--danger)" : "var(--dborder)",
    color: "var(--dtext)",
  };
}

function labelStyle(hasError: boolean): React.CSSProperties {
  return {
    color: hasError ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted2)",
  };
}

const LABEL_CLASS = "mb-1.5 block text-[11px] font-medium uppercase tracking-wider";

const CHECKBOX_LABEL_CLASS =
  "flex items-center gap-2 rounded-md border px-3 py-2 text-sm";

function checkboxLabelStyle(): React.CSSProperties {
  return {
    background: "var(--surface1)",
    borderColor: "var(--dborder)",
    color: "var(--dtext)",
  };
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
      className="rounded py-1.5 text-xs font-medium transition-all duration-[160ms]"
      style={active ? { background: "var(--surface3)", color: "var(--dtext)" } : { color: "var(--dmuted)" }}
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
  const optionalFieldsExpanded = showOptionalFields || hasOptionalErrors;
  return (
    <div className="space-y-4">
      <div>
        <label
          className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider"
          style={{ color: titleIssue ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted2)" }}
        >
          Video title <span style={{ color: "var(--warning)" }}>*</span>
        </label>
        <input
          type="text"
          placeholder="Required for YouTube uploads…"
          value={fields.title}
          onChange={(e) => onChange({ title: e.target.value })}
          className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
          style={{
            background: "var(--surface1)",
            borderColor: titleIssue ? "var(--danger)" : "var(--dborder)",
            color: "var(--dtext)",
          }}
        />
        <FieldError issue={titleIssue} />
      </div>

      <div>
        <label
          className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider"
          style={{ color: madeForKidsIssue ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted2)" }}
        >
          Audience <span style={{ color: "var(--warning)" }}>*</span>
        </label>
        <div
          className="grid grid-cols-2 gap-1 rounded-md border p-1"
          style={{ background: "var(--surface1)", borderColor: madeForKidsIssue ? "var(--danger)" : "var(--dborder)" }}
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

      <div className="overflow-hidden rounded-xl border" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface-raised) 76%, var(--surface2))" }}>
        <button
          type="button"
          onClick={() => setShowOptionalFields((current) => !current)}
          className="flex w-full items-center justify-between px-3 py-2.5 text-left transition-colors"
          style={{ background: "transparent" }}
        >
          <div>
            <div className="text-[11px] font-mono uppercase tracking-[0.12em]" style={{ color: "var(--dmuted)" }}>
              Optional fields
            </div>
            <div className="mt-1 text-[12px]" style={{ color: "var(--dmuted2)" }}>
              Category, visibility, scheduling, playlists, tags, and advanced YouTube settings.
            </div>
          </div>
          <ChevronDown
            className={cn(
              "h-4 w-4 transition-transform duration-200",
              optionalFieldsExpanded ? "rotate-180" : "rotate-0"
            )}
            style={{ color: "var(--dmuted)" }}
          />
        </button>

        {optionalFieldsExpanded && (
          <div className="space-y-4 border-t px-3 py-3" style={{ borderTopColor: "var(--dborder)" }}>
            <div>
              <label
                className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider"
                style={{ color: visibilityIssue ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted2)" }}
              >
                Visibility
              </label>
              <div
                className="grid grid-cols-3 gap-1 rounded-md border p-1"
                style={{ background: "var(--surface1)", borderColor: visibilityIssue ? "var(--danger)" : "var(--dborder)" }}
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
                <label className={LABEL_CLASS} style={labelStyle(false)}>
                  Category
                </label>
                <select
                  value={fields.category}
                  onChange={(e) => onChange({ category: e.target.value })}
                  className={INPUT_CLASS}
                  style={inputStyle(false)}
                >
                  {YOUTUBE_CATEGORY_OPTIONS.map((c) => (
                    <option key={c.id} value={c.id}>{c.label}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className={LABEL_CLASS} style={labelStyle(!!licenseIssue)}>
                  License
                </label>
                <select
                  value={fields.license}
                  onChange={(e) => onChange({ license: e.target.value as NonNullable<PlatformOverride["youtube"]>["license"] })}
                  className={INPUT_CLASS}
                  style={inputStyle(!!licenseIssue)}
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
                <label className={LABEL_CLASS} style={labelStyle(!!defaultLanguageIssue)}>
                  Default language
                </label>
                <input
                  type="text"
                  placeholder="en, en-US, zh-CN…"
                  value={fields.defaultLanguage}
                  onChange={(e) => onChange({ defaultLanguage: e.target.value })}
                  className={INPUT_CLASS}
                  style={inputStyle(!!defaultLanguageIssue)}
                />
                <FieldError issue={defaultLanguageIssue} />
              </div>
              <div>
                <label className={LABEL_CLASS} style={labelStyle(!!publishAtIssue)}>
                  Publish at
                </label>
                <input
                  type="datetime-local"
                  value={fields.publishAt}
                  onChange={(e) => onChange({ publishAt: e.target.value })}
                  className={INPUT_CLASS}
                  style={inputStyle(!!publishAtIssue)}
                />
                <FieldError issue={publishAtIssue} />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className={LABEL_CLASS} style={labelStyle(!!recordingDateIssue)}>
                  Recording date
                </label>
                <input
                  type="date"
                  value={fields.recordingDate}
                  onChange={(e) => onChange({ recordingDate: e.target.value })}
                  className={INPUT_CLASS}
                  style={inputStyle(!!recordingDateIssue)}
                />
                <FieldError issue={recordingDateIssue} />
              </div>
              <div>
                <label className={LABEL_CLASS} style={labelStyle(false)}>
                  Playlist ID
                </label>
                <input
                  type="text"
                  placeholder="PLxxxxxxxx"
                  value={fields.playlistId}
                  onChange={(e) => onChange({ playlistId: e.target.value })}
                  className={INPUT_CLASS}
                  style={inputStyle(false)}
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className={LABEL_CLASS} style={labelStyle(false)}>
                  Tags
                </label>
                <input
                  type="text"
                  placeholder="product, quarterly, update"
                  value={fields.tags}
                  onChange={(e) => onChange({ tags: e.target.value })}
                  className={INPUT_CLASS}
                  style={inputStyle(false)}
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <label className={CHECKBOX_LABEL_CLASS} style={checkboxLabelStyle()}>
                <input
                  type="checkbox"
                  checked={fields.notifySubscribers}
                  onChange={(e) => onChange({ notifySubscribers: e.target.checked })}
                  className="h-4 w-4 rounded"
                  style={{ accentColor: "var(--success)" }}
                />
                Notify subscribers
              </label>
              <label className={CHECKBOX_LABEL_CLASS} style={checkboxLabelStyle()}>
                <input
                  type="checkbox"
                  checked={fields.embeddable}
                  onChange={(e) => onChange({ embeddable: e.target.checked })}
                  className="h-4 w-4 rounded"
                  style={{ accentColor: "var(--success)" }}
                />
                Allow embedding
              </label>
              <label className={CHECKBOX_LABEL_CLASS} style={checkboxLabelStyle()}>
                <input
                  type="checkbox"
                  checked={fields.publicStatsViewable}
                  onChange={(e) => onChange({ publicStatsViewable: e.target.checked })}
                  className="h-4 w-4 rounded"
                  style={{ accentColor: "var(--success)" }}
                />
                Show public stats
              </label>
              <label className={CHECKBOX_LABEL_CLASS} style={checkboxLabelStyle()}>
                <input
                  type="checkbox"
                  checked={fields.containsSyntheticMedia}
                  onChange={(e) => onChange({ containsSyntheticMedia: e.target.checked })}
                  className="h-4 w-4 rounded"
                  style={{ accentColor: "var(--success)" }}
                />
                Contains synthetic media
              </label>
              <label className={CHECKBOX_LABEL_CLASS} style={checkboxLabelStyle()}>
                <input
                  type="checkbox"
                  checked={fields.shorts}
                  onChange={(e) => onChange({ shorts: e.target.checked })}
                  className="h-4 w-4 rounded"
                  style={{ accentColor: "var(--success)" }}
                />
                Add Shorts hint
              </label>
            </div>
          </div>
        )}
      </div>

      <p className="text-[11px]" style={{ color: "var(--dmuted2)" }}>
        UniPost sends caption text as YouTube `snippet.description`. `title` and `audience` are required. All other YouTube settings are optional.
      </p>
    </div>
  );
}
