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

interface PlatformEditorBlockProps {
  account: SocialAccount;
  index: number;
  override: PlatformOverride;
  collapsed: boolean;
  charCount: CharCountInfo;
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
  onCaptionChange,
  onPlatformFieldChange,
  onToggleCollapse,
}: PlatformEditorBlockProps) {
  const brandColor = PLATFORM_BRAND_COLORS[account.platform] || "#888";
  const label = PLATFORM_LABELS[account.platform] || account.platform;
  const limit = PLATFORM_CHAR_LIMITS[account.platform] || 5000;

  const youtubeFields = override.youtube || { title: "", category: "People & Blogs", visibility: "public" as const };
  const tiktokFields = override.tiktok || { privacy: "public" as const, interactions: "allow_all" as const };
  const instagramFields = override.instagram || { mediaType: "feed" as const };
  const linkedinFields = override.linkedin || { visibility: "anyone" as const };

  return (
    <div
      className="rounded-xl border border-[#22222a] bg-[#17171a]/50 overflow-hidden animate-[slideIn_260ms_cubic-bezier(0.16,1,0.3,1)_backwards]"
      style={{ animationDelay: `${index * 40}ms` }}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#22222a] bg-[#17171a]/60">
        <div className="flex items-center gap-2.5">
          <div
            className="w-6 h-6 rounded flex items-center justify-center"
            style={{ background: `${brandColor}20`, color: brandColor }}
          >
            <PlatformIcon platform={account.platform} size={11} />
          </div>
          <div>
            <div className="text-[13px] font-medium text-[#f4f4f5]">{label}</div>
            <div className="text-[11px] text-[#55555c] font-mono">
              {account.account_name || account.external_user_email || account.platform}
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
                charCount.status === "over"
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
