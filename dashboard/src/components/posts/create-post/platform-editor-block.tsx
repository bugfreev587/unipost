"use client";

import { PlatformIcon } from "@/components/platform-icons";
import {
  PLATFORM_LABELS,
  PLATFORM_BRAND_COLORS,
  PLATFORM_CHAR_LIMITS,
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

  const youtubeFields = override.youtube || { title: "", category: "People & Blogs", visibility: "public" as const };
  const tiktokFields = override.tiktok || { privacy: "public" as const, interactions: "allow_all" as const };
  const instagramFields = override.instagram || { mediaType: "feed" as const };
  const linkedinFields = override.linkedin || { visibility: "anyone" as const };

  return (
    <div
      className={cn(
        "rounded-xl border bg-[#17171a]/50 overflow-hidden animate-[slideIn_260ms_cubic-bezier(0.16,1,0.3,1)_backwards]",
        hasErrors
          ? "border-[#ef4444]/70 shadow-[0_0_0_1px_rgba(239,68,68,0.28)]"
          : hasWarnings
            ? "border-[#f59e0b]/60 shadow-[0_0_0_1px_rgba(245,158,11,0.24)]"
            : "border-[#22222a]"
      )}
      style={{ animationDelay: `${index * 40}ms` }}
    >
      {/* Header */}
      <div
        className={cn(
          "flex items-center justify-between px-4 py-3 border-b bg-[#17171a]/60",
          hasErrors
            ? "border-b-[#ef4444]/40"
            : hasWarnings
              ? "border-b-[#f59e0b]/35"
              : "border-b-[#22222a]"
        )}
      >
        <div className="flex items-center gap-2.5">
          <div
            className="w-6 h-6 rounded flex items-center justify-center"
            style={{ background: `${brandColor}20`, color: brandColor }}
          >
            <PlatformIcon platform={account.platform} size={11} />
          </div>
          <div>
            <div className="text-[13px] font-medium text-[#f4f4f5]">{label}</div>
            <div className="flex items-center gap-2 flex-wrap">
              <div className="text-[11px] text-[#55555c] font-mono">
                {account.account_name || account.external_user_email || account.platform}
              </div>
              {hasErrors && (
                <span className="text-[10px] font-mono uppercase tracking-[0.12em] text-[#fca5a5]">
                  {errorIssues.length} issue{errorIssues.length === 1 ? "" : "s"}
                </span>
              )}
              {hasWarnings && (
                <span className="text-[10px] font-mono uppercase tracking-[0.12em] text-[#fcd34d]">
                  {warningIssues.length} warning{warningIssues.length === 1 ? "" : "s"}
                </span>
              )}
            </div>
          </div>
        </div>
        <button
          type="button"
          className="text-[11px] text-[#55555c] hover:text-[#f4f4f5] font-mono transition-colors"
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
              className={cn(
                "rounded-lg border px-3 py-2.5",
                hasErrors
                  ? "border-[#ef4444]/35 bg-[#2a1114]"
                  : "border-[#f59e0b]/35 bg-[#2d2110]"
              )}
            >
              <div
                className={cn(
                  "text-[11px] font-mono uppercase tracking-[0.12em] mb-1.5",
                  hasErrors ? "text-[#fca5a5]" : "text-[#fcd34d]"
                )}
              >
                {hasErrors ? "Needs attention" : "Review before publish"}
              </div>
              <div className="space-y-1.5">
                {issues.slice(0, 3).map((issue, issueIndex) => (
                  <div
                    key={`${issue.code}-${issue.field}-${issueIndex}`}
                    className={cn(
                      "text-[12px] leading-relaxed",
                      issue.severity === "error" ? "text-[#fecaca]" : "text-[#fde68a]"
                    )}
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
              <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium">
                Custom caption
              </label>
              <span
                className={cn(
                  "text-[11px] font-mono",
                  charCount.status === "over"
                    ? "text-[#ef4444]"
                    : charCount.status === "warning"
                      ? "text-[#f59e0b]"
                      : "text-[#55555c]"
                )}
              >
                {charCount.count} / {limit}
              </span>
            </div>
            <textarea
              rows={3}
              placeholder="Leave blank to use main content"
              value={override.caption || ""}
              onChange={(e) => onCaptionChange(e.target.value)}
              className={cn(
                "w-full rounded-md px-3 py-2 text-sm resize-none leading-relaxed",
                "bg-[#0a0a0b] border text-[#f4f4f5] outline-none",
                "transition-[border-color] duration-[140ms]",
                "focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]",
                "placeholder:text-[#55555c]",
                hasErrors
                  ? "border-[#ef4444]"
                  : hasWarnings
                    ? "border-[#f59e0b]"
                    : charCount.status === "over"
                  ? "border-[#ef4444]"
                  : "border-[#22222a]"
              )}
            />
          </div>

          {/* Platform-specific fields */}
          {account.platform === "youtube" && (
            <YouTubeFields
              fields={youtubeFields}
              onChange={(f) => onPlatformFieldChange("youtube", f)}
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
