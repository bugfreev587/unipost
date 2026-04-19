"use client";

import { PlatformIcon } from "@/components/platform-icons";
import {
  PLATFORM_LABELS,
  PLATFORM_BRAND_COLORS,
  PLATFORM_CHAR_LIMITS,
  DEFAULT_YOUTUBE_FIELDS,
  type PlatformOverride,
  type CharCountInfo,
} from "./use-create-post-form";
import { YouTubeFields } from "./platform-fields/youtube-fields";
import { TikTokFields } from "./platform-fields/tiktok-fields";
import { InstagramFields } from "./platform-fields/instagram-fields";
import { LinkedInFields } from "./platform-fields/linkedin-fields";
import type { SocialAccount } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { SocialPostValidationIssue } from "@/lib/api";

interface PlatformEditorBlockProps {
  account: SocialAccount;
  index: number;
  override: PlatformOverride;
  collapsed: boolean;
  charCount: CharCountInfo;
  issues?: SocialPostValidationIssue[];
  onCaptionChange: (caption: string) => void;
  onPlatformFieldChange: <K extends "youtube" | "tiktok" | "instagram" | "linkedin">(
    platform: K,
    fields: Partial<NonNullable<PlatformOverride[K]>>
  ) => void;
  onToggleCollapse: () => void;
}

export function PlatformEditorBlock({
  account,
  index,
  override,
  collapsed,
  charCount,
  issues = [],
  onCaptionChange,
  onPlatformFieldChange,
  onToggleCollapse,
}: PlatformEditorBlockProps) {
  const brandColor = PLATFORM_BRAND_COLORS[account.platform] || "#888";
  const label = PLATFORM_LABELS[account.platform] || account.platform;
  const limit = PLATFORM_CHAR_LIMITS[account.platform] || 5000;
  const errorIssues = issues.filter((issue) => issue.severity === "error");
  const warningIssues = issues.filter((issue) => issue.severity === "warning");
  const hasErrors = errorIssues.length > 0;
  const hasWarnings = !hasErrors && warningIssues.length > 0;
  const captionIssues = issues.filter((issue) => issue.field === "caption");
  const hasCaptionError = captionIssues.some((issue) => issue.severity === "error");
  const captionMessage = captionIssues[0]?.message;

  const youtubeFields = override.youtube || DEFAULT_YOUTUBE_FIELDS;
  const tiktokFields = override.tiktok || { privacy: "public" as const, interactions: "allow_all" as const };
  const instagramFields = override.instagram || { mediaType: "feed" as const };
  const linkedinFields = override.linkedin || { visibility: "anyone" as const };

  return (
    <div
      className={cn(
        "overflow-hidden rounded-xl border animate-[slideIn_260ms_cubic-bezier(0.16,1,0.3,1)_backwards]",
        hasErrors
          ? "shadow-[0_0_0_1px_color-mix(in_srgb,var(--danger)_28%,transparent)]"
          : hasWarnings
            ? "shadow-[0_0_0_1px_color-mix(in_srgb,var(--warning)_24%,transparent)]"
            : ""
      )}
      style={{
        animationDelay: `${index * 40}ms`,
        background: "color-mix(in srgb, var(--surface-raised) 78%, var(--surface2))",
        borderColor: hasErrors ? "color-mix(in srgb, var(--danger) 70%, transparent)" : hasWarnings ? "color-mix(in srgb, var(--warning) 60%, transparent)" : "var(--dborder)",
      }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between border-b px-4 py-3"
        style={{
          background: "color-mix(in srgb, var(--surface2) 62%, var(--surface-raised))",
          borderBottomColor: hasErrors ? "color-mix(in srgb, var(--danger) 40%, transparent)" : hasWarnings ? "color-mix(in srgb, var(--warning) 35%, transparent)" : "var(--dborder)",
        }}
      >
        <div className="flex items-center gap-2.5">
          <div
            className="w-6 h-6 rounded flex items-center justify-center"
            style={{ background: `${brandColor}20`, color: brandColor }}
          >
            <PlatformIcon platform={account.platform} size={11} />
          </div>
          <div>
            <div className="text-[13.5px] leading-[1.25]" style={{ color: "var(--dtext)", fontWeight: 600 }}>{label}</div>
            <div className="flex items-center gap-2 flex-wrap">
              <div className="font-mono text-[10.5px] tracking-[0.02em]" style={{ color: "var(--dmuted2)" }}>
                {account.account_name || account.external_user_email || account.platform}
              </div>
              {hasErrors && (
                <span className="font-mono text-[10px] uppercase tracking-[0.12em]" style={{ color: "color-mix(in srgb, var(--danger) 45%, white)" }}>
                  {errorIssues.length} issue{errorIssues.length === 1 ? "" : "s"}
                </span>
              )}
              {hasWarnings && (
                <span className="font-mono text-[10px] uppercase tracking-[0.12em]" style={{ color: "color-mix(in srgb, var(--warning) 70%, white)" }}>
                  {warningIssues.length} warning{warningIssues.length === 1 ? "" : "s"}
                </span>
              )}
            </div>
          </div>
        </div>
        <button
          type="button"
          className="font-mono text-[10.5px] tracking-[0.02em] transition-colors"
          style={{ color: "var(--dmuted2)" }}
          onClick={onToggleCollapse}
        >
          toggle
        </button>
      </div>

      {/* Body */}
      {!collapsed && (
        <div className="p-4 space-y-3">
          {issues.length > 0 && (
            <div
              className="rounded-lg border px-3 py-2.5"
              style={{
                borderColor: hasErrors ? "color-mix(in srgb, var(--danger) 35%, transparent)" : "color-mix(in srgb, var(--warning) 35%, transparent)",
                background: hasErrors ? "color-mix(in srgb, var(--danger) 10%, var(--surface-raised))" : "color-mix(in srgb, var(--warning) 10%, var(--surface-raised))",
              }}
            >
              <div
                className="mb-1.5 font-mono text-[11px] uppercase tracking-[0.12em]"
                style={{ color: hasErrors ? "color-mix(in srgb, var(--danger) 45%, white)" : "color-mix(in srgb, var(--warning) 70%, white)" }}
              >
                {hasErrors ? "Needs attention" : "Review before publish"}
              </div>
              <div className="space-y-1.5">
                {issues.slice(0, 3).map((issue, issueIndex) => (
                  <div
                    key={`${issue.code}-${issue.field}-${issueIndex}`}
                    className="text-[12px] leading-relaxed"
                    style={{ color: issue.severity === "error" ? "color-mix(in srgb, var(--danger) 26%, white)" : "color-mix(in srgb, var(--warning) 28%, white)" }}
                  >
                    {issue.message}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Caption */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
                <label className="text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                  Custom caption
                </label>
              <span
                className="font-mono text-[10.5px] tracking-[0.02em]"
                style={{ color: charCount.status === "over" ? "var(--danger)" : charCount.status === "warning" ? "var(--warning)" : "var(--dmuted2)" }}
              >
                {charCount.count} / {limit}
              </span>
            </div>
            <textarea
              rows={3}
              placeholder="Leave blank to use main content"
              value={override.caption || ""}
              onChange={(e) => onCaptionChange(e.target.value)}
              className="w-full resize-none rounded-md border px-3 py-2 text-sm leading-relaxed outline-none transition-[border-color,box-shadow] duration-[140ms]"
              style={{
                background: "var(--surface1)",
                color: "var(--dtext)",
                borderColor: hasCaptionError || charCount.status === "over" ? "var(--danger)" : hasWarnings ? "var(--warning)" : "var(--dborder)",
              }}
            />
            {captionMessage && (
              <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: "color-mix(in srgb, var(--danger) 45%, white)" }}>
                {captionMessage}
              </p>
            )}
          </div>

          {/* Platform-specific fields */}
          {account.platform === "youtube" && (
            <YouTubeFields
              fields={youtubeFields}
              onChange={(f) => onPlatformFieldChange("youtube", f)}
              issues={issues}
            />
          )}
          {account.platform === "tiktok" && (
            <TikTokFields
              fields={tiktokFields}
              onChange={(f) => onPlatformFieldChange("tiktok", f)}
            />
          )}
          {account.platform === "instagram" && (
            <InstagramFields
              fields={instagramFields}
              onChange={(f) => onPlatformFieldChange("instagram", f)}
              issues={issues}
            />
          )}
          {account.platform === "linkedin" && (
            <LinkedInFields
              fields={linkedinFields}
              onChange={(f) => onPlatformFieldChange("linkedin", f)}
            />
          )}
        </div>
      )}
    </div>
  );
}
