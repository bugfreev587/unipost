"use client";

import { AccountDestinationIcon } from "@/components/account-destination-icon";
import {
  PLATFORM_LABELS,
  PLATFORM_BRAND_COLORS,
  PLATFORM_CHAR_LIMITS,
  DEFAULT_YOUTUBE_FIELDS,
  DEFAULT_TIKTOK_FIELDS,
  type PlatformOverride,
  type CharCountInfo,
} from "./use-create-post-form";
import { YouTubeFields } from "./platform-fields/youtube-fields";
import { TikTokFields } from "./platform-fields/tiktok-fields";
import { InstagramFields } from "./platform-fields/instagram-fields";
import { LinkedInFields } from "./platform-fields/linkedin-fields";
import { FacebookFields } from "./platform-fields/facebook-fields";
import { PinterestFields } from "./platform-fields/pinterest-fields";
import type { SocialAccount } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { SocialPostValidationIssue } from "@/lib/api";
import { getAccountDisplayName } from "./account-labels";

interface PlatformEditorBlockProps {
  account: SocialAccount;
  index: number;
  override: PlatformOverride;
  collapsed: boolean;
  charCount: CharCountInfo;
  captionLimit?: number;
  issues?: SocialPostValidationIssue[];
  // mediaKind is "video" when any selected media item is a video, "photo"
  // when all items are images, "none" when there are no media items. TikTok
  // needs this to hide Duet/Stitch toggles for photo carousels (per the
  // Content Posting API audit requirements).
  mediaKind: "video" | "photo" | "none";
  // The first video file attached to the post (null when there isn't one).
  // TikTok fields use it to measure duration against the creator's cap.
  mediaFile: File | null;
  // Measured visual + temporal metadata for the primary video, or null
  // when there's no video attached. Each field is independently null
  // when not yet measured / not measurable. Drives Facebook's placement
  // guidance (vertical-9:16 in feed → suggest Reel switch).
  videoMetadata: {
    width: number | null;
    height: number | null;
    durationSec: number | null;
  } | null;
  // getToken is threaded down so the TikTok fields can fetch creator_info
  // with a fresh Clerk session token without owning its own auth context.
  getToken: () => Promise<string | null>;
  // profileId owns the TikTok account — used to build the creator_info URL.
  profileId: string;
  // Called by the platform-specific panel when it has a runtime reason to
  // block publish (e.g., TikTok creator can't post right now, video too
  // long for this account). Null clears the blocker.
  onTiktokBlockerChange: (reason: string | null) => void;
  // Forwarded to TikTokFields so the drawer knows each selected
  // account's creator cap and can gate R2 uploads accordingly.
  onTiktokMaxDurationChange: (sec: number | null) => void;
  onCaptionChange: (caption: string) => void;
  onFirstCommentChange: (firstComment: string) => void;
  firstCommentSupported: boolean;
  firstCommentMaxLength?: number | null;
  threadSupported: boolean;
  onThreadFieldsChange: (fields: Partial<Pick<PlatformOverride, "inReplyTo" | "threadPosition">>) => void;
  onAddThreadReply: () => void;
  onUpdateThreadReply: (index: number, value: string) => void;
  onRemoveThreadReply: (index: number) => void;
  onPlatformFieldChange: <K extends "youtube" | "tiktok" | "instagram" | "linkedin" | "facebook" | "pinterest">(
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
  captionLimit,
  issues = [],
  mediaKind,
  mediaFile,
  videoMetadata,
  getToken,
  profileId,
  onTiktokBlockerChange,
  onTiktokMaxDurationChange,
  onCaptionChange,
  onFirstCommentChange,
  firstCommentSupported,
  firstCommentMaxLength,
  threadSupported,
  onThreadFieldsChange,
  onAddThreadReply,
  onUpdateThreadReply,
  onRemoveThreadReply,
  onPlatformFieldChange,
  onToggleCollapse,
}: PlatformEditorBlockProps) {
  const brandColor = PLATFORM_BRAND_COLORS[account.platform] || "#888";
  const iconColor = account.platform === "youtube" ? "var(--dmuted)" : brandColor;
  const iconBackground = account.platform === "youtube" ? "color-mix(in srgb, var(--dmuted) 12%, transparent)" : `${brandColor}20`;
  const label = PLATFORM_LABELS[account.platform] || account.platform;
  const accountLabel = getAccountDisplayName(account);
  const limit = captionLimit || PLATFORM_CHAR_LIMITS[account.platform] || 5000;
  const errorIssues = issues.filter((issue) => issue.severity === "error");
  const warningIssues = issues.filter((issue) => issue.severity === "warning");
  const hasErrors = errorIssues.length > 0;
  const hasWarnings = !hasErrors && warningIssues.length > 0;
  const captionIssues = issues.filter((issue) => issue.field === "caption");
  const hasCaptionError = captionIssues.some((issue) => issue.severity === "error");
  const captionMessage = captionIssues[0]?.message;
  const firstCommentIssues = issues.filter((issue) => issue.field === "first_comment");
  const hasFirstCommentError = firstCommentIssues.some((issue) => issue.severity === "error");
  const firstCommentMessage = firstCommentIssues[0]?.message;
  const threadIssues = issues.filter((issue) => issue.field === "thread_position" || issue.field === "in_reply_to");
  const threadMessage = threadIssues[0]?.message;

  const youtubeFields = override.youtube || DEFAULT_YOUTUBE_FIELDS;
  const tiktokFields = override.tiktok || DEFAULT_TIKTOK_FIELDS;
  const instagramFields = override.instagram || { mediaType: "feed" as const };
  const linkedinFields = override.linkedin || { visibility: "anyone" as const };
  const facebookFields = override.facebook || { link: "", mediaType: "feed" as const };
  const pinterestFields = override.pinterest || { boardId: "", title: "", link: "" };
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
        background: "color-mix(in srgb, var(--surface-raised) 84%, var(--surface2))",
        borderColor: hasErrors ? "color-mix(in srgb, var(--danger) 70%, transparent)" : hasWarnings ? "color-mix(in srgb, var(--warning) 60%, transparent)" : "color-mix(in srgb, var(--dborder2) 78%, var(--dborder))",
      }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between border-b px-4 py-3"
        style={{
          background: "color-mix(in srgb, var(--surface2) 76%, var(--surface-raised))",
          borderBottomColor: hasErrors ? "color-mix(in srgb, var(--danger) 40%, transparent)" : hasWarnings ? "color-mix(in srgb, var(--warning) 35%, transparent)" : "color-mix(in srgb, var(--dborder2) 76%, var(--dborder))",
        }}
      >
        <div className="flex items-center gap-2.5">
          <div
            className="w-6 h-6 rounded flex items-center justify-center"
            style={{ background: iconBackground, color: iconColor }}
          >
            <AccountDestinationIcon platform={account.platform} size={11} />
          </div>
          <div>
            <div className="text-[13.5px] leading-[1.25]" style={{ color: "var(--dtext)", fontWeight: 600 }}>{label}</div>
            <div className="flex items-center gap-2 flex-wrap">
              <div className="compose-meta-text font-mono text-[10.5px] tracking-[0.02em]">
                {accountLabel}
              </div>
              {hasErrors && (
                <span className="font-mono text-[10px] uppercase tracking-[0.12em]" style={{ color: "color-mix(in srgb, var(--danger) 45%, white)" }}>
                  {errorIssues.length} issue{errorIssues.length === 1 ? "" : "s"}
                </span>
              )}
              {hasWarnings && (
                <span className="font-mono text-[10px] uppercase tracking-[0.12em]" style={{ color: "var(--warning)" }}>
                  {warningIssues.length} warning{warningIssues.length === 1 ? "" : "s"}
                </span>
              )}
            </div>
          </div>
        </div>
        <button
          type="button"
          className="font-mono text-[10.5px] tracking-[0.02em] transition-colors"
          style={{ color: "color-mix(in srgb, var(--dtext) 58%, transparent)" }}
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
                borderColor: hasErrors ? "color-mix(in srgb, var(--danger) 45%, transparent)" : "color-mix(in srgb, var(--warning) 45%, transparent)",
                background: hasErrors ? "color-mix(in srgb, var(--danger) 16%, var(--surface-raised))" : "color-mix(in srgb, var(--warning) 16%, var(--surface-raised))",
              }}
            >
              <div
                className="mb-1.5 font-mono text-[11px] uppercase tracking-[0.12em]"
                style={{ color: hasErrors ? "var(--danger)" : "var(--warning)" }}
              >
                {hasErrors ? "Needs attention" : "Review before publish"}
              </div>
              <div className="space-y-1.5">
                {issues.slice(0, 3).map((issue, issueIndex) => (
                  <div
                    key={`${issue.code}-${issue.field}-${issueIndex}`}
                    className="text-[12px] leading-relaxed"
                    style={{ color: "var(--text)" }}
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
                <label className="compose-panel-label text-[11px] font-semibold uppercase tracking-[0.11em]">
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
              className="compose-field w-full resize-none rounded-md border px-3 py-2 text-sm leading-relaxed outline-none transition-[border-color,box-shadow] duration-[140ms]"
              style={{
                background: undefined,
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

          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="compose-panel-label text-[11px] font-semibold uppercase tracking-[0.11em]">
                First comment
              </label>
              <span className="compose-meta-text font-mono text-[10.5px] tracking-[0.02em]">
                optional
              </span>
            </div>
            <textarea
              rows={2}
              placeholder={firstCommentSupported ? "Optional follow-up comment for supported platforms" : "This platform does not support first comments"}
              value={override.firstComment || ""}
              onChange={(e) => onFirstCommentChange(e.target.value)}
              disabled={!firstCommentSupported}
              className="compose-field w-full resize-none rounded-md border px-3 py-2 text-sm leading-relaxed outline-none transition-[border-color,box-shadow] duration-[140ms]"
              style={{
                background: firstCommentSupported ? undefined : "color-mix(in srgb, var(--surface2) 70%, transparent)",
                color: firstCommentSupported ? "var(--dtext)" : "color-mix(in srgb, var(--dtext) 50%, transparent)",
                borderColor: hasFirstCommentError ? "var(--danger)" : hasWarnings ? "var(--warning)" : "var(--dborder)",
              }}
            />
            <p className="compose-support-text mt-1.5 text-[11px] leading-relaxed">
              {firstCommentSupported
                ? `Used only on supported destinations${firstCommentMaxLength ? `. Max ${firstCommentMaxLength} chars.` : "."} AI-generated first comments will appear here after you apply them.`
                : "Use the main caption or native thread tools instead. This destination rejects first_comment."}
            </p>
            {firstCommentMessage && (
              <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: "color-mix(in srgb, var(--danger) 45%, white)" }}>
                {firstCommentMessage}
              </p>
            )}
          </div>

          {threadSupported && (
            <div className="compose-surface-subtle rounded-lg border px-3 py-3">
              <div className="compose-panel-label mb-2 text-[11px] font-semibold uppercase tracking-[0.11em]">
                Thread options
              </div>
              <div className="space-y-3">
                <div>
                  <label className="compose-panel-label mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]">
                    Thread position
                  </label>
                  <input
                    type="number"
                    min={1}
                    step={1}
                    placeholder="1"
                    value={override.threadPosition || ""}
                    onChange={(e) => onThreadFieldsChange({ threadPosition: e.target.value })}
                    className="compose-field w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
                    style={{ color: "var(--dtext)", borderColor: threadMessage ? "var(--danger)" : "var(--dborder)" }}
                  />
                  <p className="compose-support-text mt-1.5 text-[11px] leading-relaxed">
                    Use `1` for the first post in a native thread, `2` for the next, and so on.
                  </p>
                </div>
                <div>
                  <div className="mb-1.5 flex items-center justify-between gap-3">
                    <label className="compose-panel-label text-[11px] font-semibold uppercase tracking-[0.11em]">
                      Reply chain
                    </label>
                    <button
                      type="button"
                      onClick={onAddThreadReply}
                      className="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-colors"
                      style={{ borderColor: "color-mix(in srgb, var(--dborder2) 78%, var(--dborder))", color: "var(--dtext)", background: "color-mix(in srgb, var(--surface1) 96%, var(--surface2))" }}
                    >
                      Add reply
                    </button>
                  </div>
                  <div className="space-y-2">
                    {(override.threadReplies || []).length === 0 ? (
                      <p className="compose-support-text text-[11px] leading-relaxed">
                        Add follow-up posts for the same account. They will publish as thread positions `2, 3, 4...` after the main caption.
                      </p>
                    ) : (
                      (override.threadReplies || []).map((reply, replyIndex) => (
                        <div key={replyIndex} className="compose-surface-panel rounded-md border px-3 py-3">
                          <div className="mb-2 flex items-center justify-between gap-3">
                            <div className="compose-meta-text font-mono text-[10.5px] uppercase tracking-[0.12em]">
                              Reply {replyIndex + 2}
                            </div>
                            <button
                              type="button"
                              onClick={() => onRemoveThreadReply(replyIndex)}
                              className="text-[11px] underline"
                              style={{ color: "var(--dmuted)" }}
                            >
                              Remove
                            </button>
                          </div>
                          <textarea
                            rows={2}
                            placeholder="Write the next post in this thread"
                            value={reply}
                            onChange={(e) => onUpdateThreadReply(replyIndex, e.target.value)}
                            className="compose-field w-full resize-none rounded-md border px-3 py-2 text-sm leading-relaxed outline-none transition-[border-color,box-shadow] duration-[140ms]"
                            style={{ color: "var(--dtext)", borderColor: "var(--dborder)" }}
                          />
                        </div>
                      ))
                    )}
                  </div>
                </div>
                <div>
                  <label className="compose-panel-label mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]">
                    Reply target
                  </label>
                  <input
                    type="text"
                    placeholder="Optional external post ID"
                    value={override.inReplyTo || ""}
                    onChange={(e) => onThreadFieldsChange({ inReplyTo: e.target.value })}
                    className="compose-field w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
                    style={{ color: "var(--dtext)", borderColor: threadMessage ? "var(--danger)" : "var(--dborder)" }}
                  />
                  <p className="compose-support-text mt-1.5 text-[11px] leading-relaxed">
                    Leave blank for a standalone thread. Use this only when replying to an existing platform post.
                  </p>
                </div>
                {threadMessage ? (
                  <p className="text-[11px] leading-relaxed" style={{ color: "color-mix(in srgb, var(--danger) 45%, white)" }}>
                    {threadMessage}
                  </p>
                ) : null}
              </div>
            </div>
          )}

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
              account={account}
              fields={tiktokFields}
              mediaKind={mediaKind}
              mediaFile={mediaFile}
              getToken={getToken}
              profileId={profileId}
              issues={issues}
              onChange={(f) => onPlatformFieldChange("tiktok", f)}
              onBlockerChange={onTiktokBlockerChange}
              onMaxDurationChange={onTiktokMaxDurationChange}
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
          {account.platform === "facebook" && (
            <FacebookFields
              fields={facebookFields}
              onChange={(f) => onPlatformFieldChange("facebook", f)}
              mediaAttached={mediaKind !== "none"}
              videoAttached={mediaKind === "video"}
              videoMetadata={videoMetadata}
            />
          )}
          {account.platform === "pinterest" && (
            <PinterestFields
              accountId={account.id}
              profileId={profileId}
              getToken={getToken}
              fields={pinterestFields}
              issues={issues}
              onChange={(f) => onPlatformFieldChange("pinterest", f)}
            />
          )}
        </div>
      )}
    </div>
  );
}
