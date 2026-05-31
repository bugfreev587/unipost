"use client";

import { useEffect, useCallback, useState, useRef, useMemo, memo } from "react";
import Link from "next/link";
import { AlertTriangle, Loader2, PanelRightClose, PanelRightOpen, Plus, Sparkles, Wand2 } from "lucide-react";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { ConnectedAccountsGrid, PostToGrid } from "./account-card-grid";
import { PlatformEditorBlock } from "./platform-editor-block";
import { EmptyPlatformState } from "./empty-platform-state";
import { PublishModePanel } from "./publish-mode-panel";
import { getAccountDisplayName } from "./account-labels";
import {
  buildAIAssistAccountLabels,
  buildAIAssistCurrentFirstComments,
  buildAIAssistCurrentPlatformCaptions,
  buildAIPostAssistRequest,
  canGenerateAIAssist,
  getFirstCommentMaxLength,
  getPlatformCaptionLimit,
  supportsFirstComment,
  supportsScheduling,
  supportsThreads,
  type AIAssistObjective,
  type AIAssistTone,
} from "./ai-assist";
import {
  useCreatePostForm,
  PRIMARY_BUTTON_LABELS,
  measureVideoMetadata,
  type ExistingMediaItem,
  type MediaItem,
} from "./use-create-post-form";
import { ChevronDown } from "lucide-react";
import type { SocialAccount, Profile } from "@/lib/api";
import {
  postAssistAIDraft,
  createSocialPost,
  friendlyRateLimitMessage,
  createMedia,
  type AIPostAssistSuggestion,
  getPlatformCapabilities,
  getMe,
  getMedia,
  listProfiles,
  listSocialAccounts,
  type PlatformCapabilitiesEnvelope,
  validateSocialPost,
  type CreateSocialPostPayload,
  type SocialPostValidationIssue,
  type SocialPostValidationResult,
} from "@/lib/api";
import { isFeatureInDevEnabledForMe } from "@/lib/features-in-dev";
import { cn } from "@/lib/utils";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";

const MIN_DRAWER_WIDTH = 880;
const MAX_DRAWER_WIDTH = 1680;
const AI_DRAWER_WIDTH = 1480;
const COMPOSE_MIN_LEFT_PANE_WIDTH = 520;
const COMPOSE_MIN_RIGHT_PANE_WIDTH = 360;
const COMPOSE_MIN_AI_PANE_WIDTH = 320;
const COMPOSE_DEFAULT_RIGHT_PANE_WIDTH = 560;
const COMPOSE_DEFAULT_AI_PANE_WIDTH = 400;
const COMPOSE_RESIZER_WIDTH = 12;

function clampDrawerWidth(width: number) {
  if (typeof window === "undefined") return width;
  const viewportMax = Math.max(MIN_DRAWER_WIDTH, window.innerWidth - 160);
  return Math.min(Math.max(width, MIN_DRAWER_WIDTH), Math.min(MAX_DRAWER_WIDTH, viewportMax));
}

function clampNumber(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function clampComposePaneWidths({
  totalWidth,
  rightPaneWidth,
  aiPaneWidth,
  aiOpen,
}: {
  totalWidth: number;
  rightPaneWidth: number;
  aiPaneWidth: number;
  aiOpen: boolean;
}) {
  const handleCount = aiOpen ? 2 : 1;
  const minRequiredWidth =
    COMPOSE_MIN_LEFT_PANE_WIDTH +
    COMPOSE_MIN_RIGHT_PANE_WIDTH +
    (aiOpen ? COMPOSE_MIN_AI_PANE_WIDTH : 0) +
    handleCount * COMPOSE_RESIZER_WIDTH;
  const usableWidth = Math.max(totalWidth, minRequiredWidth);

  if (!aiOpen) {
    return {
      rightPaneWidth: clampNumber(
        rightPaneWidth,
        COMPOSE_MIN_RIGHT_PANE_WIDTH,
        usableWidth - COMPOSE_MIN_LEFT_PANE_WIDTH - COMPOSE_RESIZER_WIDTH
      ),
      aiPaneWidth,
    };
  }

  const maxAIPaneWidth = usableWidth - COMPOSE_MIN_LEFT_PANE_WIDTH - COMPOSE_MIN_RIGHT_PANE_WIDTH - handleCount * COMPOSE_RESIZER_WIDTH;
  const clampedAIPaneWidth = clampNumber(aiPaneWidth, COMPOSE_MIN_AI_PANE_WIDTH, maxAIPaneWidth);
  const maxRightPaneWidth = usableWidth - COMPOSE_MIN_LEFT_PANE_WIDTH - clampedAIPaneWidth - handleCount * COMPOSE_RESIZER_WIDTH;
  const clampedRightPaneWidth = clampNumber(rightPaneWidth, COMPOSE_MIN_RIGHT_PANE_WIDTH, maxRightPaneWidth);
  const finalMaxAIPaneWidth = usableWidth - COMPOSE_MIN_LEFT_PANE_WIDTH - clampedRightPaneWidth - handleCount * COMPOSE_RESIZER_WIDTH;

  return {
    rightPaneWidth: clampedRightPaneWidth,
    aiPaneWidth: clampNumber(clampedAIPaneWidth, COMPOSE_MIN_AI_PANE_WIDTH, finalMaxAIPaneWidth),
  };
}

function ColumnResizeHandle({
  label,
  onMouseDown,
}: {
  label: string;
  onMouseDown: (event: React.MouseEvent<HTMLDivElement>) => void;
}) {
  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      onMouseDown={onMouseDown}
      className="group relative h-full shrink-0 cursor-col-resize"
      style={{ width: COMPOSE_RESIZER_WIDTH }}
    >
      <div
        className="pointer-events-none absolute inset-y-0 left-1/2 w-px -translate-x-1/2 transition-colors"
        style={{ background: "color-mix(in srgb, var(--dborder2) 78%, var(--dborder))" }}
      />
      <div
        className="pointer-events-none absolute inset-y-0 left-1/2 w-[3px] -translate-x-1/2 rounded-full opacity-0 transition-opacity group-hover:opacity-100"
        style={{ background: "color-mix(in srgb, var(--primary) 40%, transparent)" }}
      />
    </div>
  );
}

// ── Stable-URL media thumbnail (prevents flicker on re-render) ──────

const MediaThumb = memo(function MediaThumb({ item, onRemove, onRetry, onPreview }: {
  item: MediaItem;
  onRemove: () => void;
  onRetry?: () => void;
  onPreview?: (file: File) => void;
}) {
  const url = useMemo(() => URL.createObjectURL(item.file), [item.file]);
  useEffect(() => () => URL.revokeObjectURL(url), [url]);

  const uploading = item.mediaId === null && !item.error;
  const failed = !!item.error;
  const ready = item.mediaId !== null;
  const previewable = ready && !failed;

  return (
    <div
      className={cn(
        "relative h-[88px] w-[88px] flex-shrink-0 overflow-hidden rounded-lg border group/thumb"
      )}
      style={{
        background: "var(--surface1)",
        borderColor: failed ? "var(--danger)" : ready ? "var(--dborder2)" : "color-mix(in srgb, var(--primary) 60%, transparent)",
        cursor: previewable ? "zoom-in" : "default",
      }}
      title={
        failed
          ? `Upload failed: ${item.error}`
          : uploading
          ? `Uploading… ${item.progress}%`
          : item.file.name
      }
      role={previewable ? "button" : undefined}
      tabIndex={previewable ? 0 : undefined}
      onClick={() => {
        if (previewable) onPreview?.(item.file);
      }}
      onKeyDown={(e) => {
        if (!previewable) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onPreview?.(item.file);
        }
      }}
    >
      {item.file.type.startsWith("video/") ? (
        <video
          src={url}
          className="w-full h-full object-cover"
          muted
          preload="metadata"
          onLoadedData={(e) => { (e.target as HTMLVideoElement).currentTime = 0.5; }}
        />
      ) : (
        <img src={url} alt={item.file.name} className="w-full h-full object-cover" />
      )}

      {/* Upload-in-progress overlay */}
      {uploading && (
        <div className="absolute inset-0 flex flex-col items-center justify-center bg-black/45 font-mono text-[10px]" style={{ color: "var(--primary)" }}>
          <div className="mb-1 h-8 w-8 animate-spin rounded-full border-2" style={{ borderColor: "color-mix(in srgb, var(--primary) 28%, transparent)", borderTopColor: "var(--primary)" }} />
          {item.progress}%
        </div>
      )}

      {/* Failure overlay */}
      {failed && (
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); onRetry?.(); }}
          className="absolute inset-0 flex flex-col items-center justify-center bg-black/55 text-[10px] transition-colors"
          style={{ color: "var(--danger)" }}
        >
          <div className="mb-1 flex h-6 w-6 items-center justify-center rounded-full border-2 text-sm font-bold" style={{ borderColor: "var(--danger)" }}>!</div>
          Retry
        </button>
      )}

      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); onRemove(); }}
        className="absolute top-1 right-1 w-5 h-5 rounded-full bg-black/70 text-white flex items-center justify-center opacity-0 group-hover/thumb:opacity-100 transition-opacity text-xs z-10"
      >
        &times;
      </button>
      {previewable && (
        <div className="pointer-events-none absolute inset-0 bg-black/0 transition-colors group-hover/thumb:bg-black/12" />
      )}
      <div className="absolute bottom-0 left-0 right-0 truncate bg-black/60 px-1.5 py-0.5 font-mono text-[9px]" style={{ color: "rgb(229 231 235)" }}>
        {item.file.name}
      </div>
    </div>
  );
});

function ExistingMediaThumb({ item, onRemove }: { item: ExistingMediaItem; onRemove: () => void }) {
  return (
    <div
      className="relative flex h-[88px] w-[88px] flex-shrink-0 items-center justify-center overflow-hidden rounded-lg border group/thumb"
      style={{ background: "var(--surface1)", borderColor: "var(--dborder2)" }}
      title={item.label}
    >
      {item.url ? (
        <img src={item.url} alt={item.label} className="h-full w-full object-cover" />
      ) : (
        <div className="px-2 text-center font-mono text-[10px] leading-tight" style={{ color: "var(--dmuted)" }}>
          Existing media
        </div>
      )}
      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); onRemove(); }}
        className="absolute right-1 top-1 z-10 flex h-5 w-5 items-center justify-center rounded-full bg-black/70 text-xs text-white opacity-0 transition-opacity group-hover/thumb:opacity-100"
      >
        &times;
      </button>
      <div className="absolute bottom-0 left-0 right-0 truncate bg-black/60 px-1.5 py-0.5 font-mono text-[9px]" style={{ color: "rgb(229 231 235)" }}>
        {item.label}
      </div>
    </div>
  );
}

function MediaThumbnails({
  existingItems,
  items,
  onRemoveExisting,
  onRemove,
  onAdd,
  onRetry,
  onPreview,
  strictestTiktokMaxSec,
}: {
  existingItems: ExistingMediaItem[];
  items: MediaItem[];
  onRemoveExisting: (index: number) => void;
  onRemove: (index: number) => void;
  onAdd: (files: File[]) => void;
  onRetry: (index: number) => void;
  onPreview: (file: File) => void;
  strictestTiktokMaxSec: number | null;
}) {
  // The per-thumb "Retry" icon is too small to explain *why* the
  // upload failed (server errors like "size_bytes exceeds the global
  // hard cap of 4294967296" only showed up as a tooltip before).
  // Roll all failed items into a single red banner beneath the grid
  // so users see the actual error without having to hover.
  // TikTok-duration rejections are shown in a dedicated banner below
  // *while a cap is in force*; if the user unselects TikTok we move
  // them back here so Retry is available.
  const failedItems = items
    .map((item, index) => ({ item, index }))
    .filter(({ item }) => {
      if (!item.error) return false;
      if (item.error === "TIKTOK_VIDEO_TOO_LONG" && strictestTiktokMaxSec) return false;
      return true;
    });

  // Over-cap videos — whether blocked pre-upload or already in R2 but
  // retroactively too long because a TikTok account was just added.
  const oversizeVideos = strictestTiktokMaxSec
    ? items
        .map((item, index) => ({ item, index }))
        .filter(
          ({ item }) =>
            typeof item.durationSec === "number" &&
            item.durationSec > strictestTiktokMaxSec
        )
    : [];

  return (
    <section className="mt-6">
      <label className="mb-2.5 block text-xs font-medium uppercase tracking-wider" style={{ color: "var(--dmuted2)" }}>
        Media
      </label>
      <div className="flex gap-2.5 flex-wrap">
        {existingItems.map((item, i) => (
          <ExistingMediaThumb
            key={item.id ? `id:${item.id}` : `url:${item.url}`}
            item={item}
            onRemove={() => onRemoveExisting(i)}
          />
        ))}
        {items.map((item, i) => (
          <MediaThumb
            key={item.fingerprint}
            item={item}
            onRemove={() => onRemove(i)}
            onRetry={() => onRetry(i)}
            onPreview={onPreview}
          />
        ))}
        <label className="group flex h-[88px] w-[88px] flex-shrink-0 cursor-pointer flex-col items-center justify-center rounded-lg border border-dashed transition-colors" style={{ borderColor: "var(--dborder2)", background: "color-mix(in srgb, var(--surface1) 60%, transparent)" }}>
          <Plus className="h-4 w-4 transition-colors" style={{ color: "var(--dmuted)" }} />
          <div className="mt-1 text-center leading-tight text-[9px] transition-colors" style={{ color: "var(--dmuted2)" }}>
            Add media
          </div>
          <input
            type="file"
            multiple
            className="hidden"
            accept="image/jpeg,image/png,image/webp,image/gif,image/heic,video/mp4,video/quicktime,video/webm,video/x-m4v"
            onChange={(e) => { if (e.target.files) { onAdd(Array.from(e.target.files)); e.target.value = ""; } }}
          />
        </label>
      </div>
      {failedItems.length > 0 && (
        <div
          className="mt-2.5 rounded-md border px-3 py-2 text-[12px] leading-relaxed"
          style={{
            background: "color-mix(in srgb, var(--danger) 12%, var(--surface-raised))",
            borderColor: "color-mix(in srgb, var(--danger) 45%, transparent)",
            color: "color-mix(in srgb, var(--danger) 26%, white)",
          }}
        >
          {failedItems.map(({ item, index }) => (
            <div key={item.fingerprint} className="flex items-start gap-2">
              <span className="font-mono text-[11px] opacity-80">{item.file.name}</span>
              <span>— {humanizeMediaError(item.error!, item.file)}</span>
              <button
                type="button"
                onClick={() => onRetry(index)}
                className="ml-auto underline"
                style={{ color: "inherit" }}
              >
                Retry
              </button>
            </div>
          ))}
        </div>
      )}
      {oversizeVideos.length > 0 && strictestTiktokMaxSec && (
        <div
          className="mt-2.5 rounded-md border px-3 py-2 text-[12px] leading-relaxed"
          style={{
            background: "color-mix(in srgb, var(--danger) 12%, var(--surface-raised))",
            borderColor: "color-mix(in srgb, var(--danger) 45%, transparent)",
            color: "color-mix(in srgb, var(--danger) 26%, white)",
          }}
        >
          <div className="mb-1 font-semibold">Video is too long for TikTok</div>
          {oversizeVideos.map(({ item, index }) => (
            <div key={item.fingerprint} className="flex items-start gap-2">
              <span className="font-mono text-[11px] opacity-80">{item.file.name}</span>
              <span>
                — {formatDurationShort(Math.round(item.durationSec as number))} long;
                this TikTok account allows at most {formatDurationShort(strictestTiktokMaxSec)}.
              </span>
              <button
                type="button"
                onClick={() => onRemove(index)}
                className="ml-auto underline"
                style={{ color: "inherit" }}
              >
                Remove
              </button>
            </div>
          ))}
          <div className="mt-1 opacity-80">
            Replace with a shorter video, or unselect the TikTok account to keep this one.
          </div>
        </div>
      )}
    </section>
  );
}

function formatDurationShort(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const rem = seconds % 60;
  return rem === 0 ? `${minutes}m` : `${minutes}m ${rem}s`;
}

// humanizeMediaError rewrites the rawest server messages into something
// a user can act on. The hard-cap branch handles the global upload
// ceiling (currently 4 GB) — "size_bytes exceeds the global hard cap
// of 4294967296" means nothing to a non-developer. Everything else
// falls through unchanged so we don't accidentally hide useful detail.
function humanizeMediaError(raw: string, file: File): string {
  const hardCapMatch = /exceeds the global hard cap of (\d+)/.exec(raw);
  if (hardCapMatch) {
    const capBytes = parseInt(hardCapMatch[1], 10);
    const capDisplay = formatBytesHuman(capBytes);
    const fileDisplay = formatBytesHuman(file.size);
    const isVideo = file.type.startsWith("video/");
    const isImage = file.type.startsWith("image/");
    const kind = isVideo ? "Video" : isImage ? "Image" : "File";
    const fix = isVideo
      ? "Trim the clip, re-export at a lower bitrate, or downscale to 1080p before retrying."
      : isImage
      ? "Downscale the resolution or increase JPEG compression before retrying."
      : "Compress it before retrying.";
    return `${kind} is ${fileDisplay} — UniPost caps managed uploads at ${capDisplay}. ${fix}`;
  }
  if (raw === "TIKTOK_VIDEO_TOO_LONG") {
    return "Video was rejected because it was too long for the TikTok account that was selected. Retry now that TikTok is unselected, or pick a shorter video.";
  }
  return raw;
}

function formatBytesHuman(bytes: number): string {
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(gb >= 10 ? 0 : 1)} GB`;
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(mb >= 10 ? 0 : 1)} MB`;
}

function MediaPreviewDialog({
  file,
  open,
  onOpenChange,
}: {
  file: File | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const url = useMemo(() => (file ? URL.createObjectURL(file) : null), [file]);
  useEffect(() => () => {
    if (url) URL.revokeObjectURL(url);
  }, [url]);

  const isVideo = !!file && file.type.startsWith("video/");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton
        className={cn(
          "border-none bg-transparent p-0 shadow-none ring-0",
          isVideo ? "max-w-4xl" : "max-w-6xl"
        )}
      >
        <DialogTitle className="sr-only">
          {isVideo ? "Video preview" : "Image preview"}
        </DialogTitle>
        <DialogDescription className="sr-only">
          Preview uploaded media before publishing.
        </DialogDescription>
        {file && url && (
          <div
            className="overflow-hidden rounded-2xl"
            style={{ background: "color-mix(in srgb, #000 82%, var(--surface1))" }}
          >
            {isVideo ? (
              <div className="p-3 sm:p-4">
                <video
                  src={url}
                  controls
                  autoPlay
                  preload="metadata"
                  className="max-h-[78vh] w-full rounded-xl bg-black"
                />
              </div>
            ) : (
              <div className="flex items-center justify-center p-2 sm:p-4">
                <img
                  src={url}
                  alt={file.name}
                  className="max-h-[84vh] max-w-full rounded-xl object-contain"
                />
              </div>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

type AIAssistMode = "brief" | "improve" | "adapt" | "media" | "fix_validation";

function AIAssistPanel({
  mode,
  onModeChange,
  onGenerate,
  brief,
  onBriefChange,
  objective,
  onObjectiveChange,
  tone,
  onToneChange,
  includeCTA,
  onIncludeCTAChange,
  onApplyMainCaption,
  onApplyPlatformCaption,
  onApplyAllPlatformCaptions,
  onApplyFirstCommentSuggestion,
  onApplyAllFirstCommentSuggestions,
  selectedPlatformsCount,
  mediaCount,
  hasMainContent,
  loading,
  error,
  suggestion,
  accountLabels,
  accountPlatforms,
  platformCapabilities,
  currentMainCaption,
  currentPlatformCaptions,
  currentFirstComments,
}: {
  mode: AIAssistMode | null;
  onModeChange: (mode: AIAssistMode) => void;
  onGenerate: () => void;
  brief: string;
  onBriefChange: (value: string) => void;
  objective: "awareness" | "engagement" | "clicks" | "sales";
  onObjectiveChange: (value: "awareness" | "engagement" | "clicks" | "sales") => void;
  tone: "professional" | "friendly" | "bold" | "playful";
  onToneChange: (value: "professional" | "friendly" | "bold" | "playful") => void;
  includeCTA: boolean;
  onIncludeCTAChange: (value: boolean) => void;
  onApplyMainCaption: () => void;
  onApplyPlatformCaption: (accountId: string, caption: string) => void;
  onApplyAllPlatformCaptions: () => void;
  onApplyFirstCommentSuggestion: (accountId: string, text: string) => void;
  onApplyAllFirstCommentSuggestions: () => void;
  selectedPlatformsCount: number;
  mediaCount: number;
  hasMainContent: boolean;
  loading: boolean;
  error: string | null;
  suggestion: AIPostAssistSuggestion | null;
  accountLabels: Record<string, string>;
  accountPlatforms: Record<string, string>;
  platformCapabilities: PlatformCapabilitiesEnvelope["platforms"] | null;
  currentMainCaption: string;
  currentPlatformCaptions: Record<string, string>;
  currentFirstComments: Record<string, string>;
}) {
  const actions: Array<{
    id: AIAssistMode;
    title: string;
    description: string;
    disabled?: boolean;
  }> = [
    {
      id: "brief",
      title: "Generate from brief",
      description: "Start from a product brief, campaign angle, or promotion goal.",
    },
    {
      id: "improve",
      title: "Improve current draft",
      description: "Tighten wording, strengthen CTA, and make the copy clearer.",
      disabled: !hasMainContent,
    },
    {
      id: "adapt",
      title: "Adapt per platform",
      description: "Turn one draft into platform-specific variants for selected accounts.",
      disabled: !hasMainContent || selectedPlatformsCount === 0,
    },
    {
      id: "media",
      title: "Write from media",
      description: "Use uploaded images or video as context for hooks, captions, and CTA ideas.",
      disabled: mediaCount === 0,
    },
    {
      id: "fix_validation",
      title: "Fix validation issues",
      description: "Repair caption-style issues before you run publish again.",
    },
  ];

  return (
    <aside
      className="flex h-full min-w-0 flex-col"
      style={{ background: "color-mix(in srgb, var(--surface2) 52%, var(--surface-raised))" }}
    >
      <div className="border-b px-6 py-5" style={{ borderBottomColor: "color-mix(in srgb, var(--dborder2) 78%, var(--dborder))" }}>
        <div className="mb-2 flex items-center gap-2">
          <div
            className="flex h-8 w-8 items-center justify-center rounded-lg"
            style={{ background: "color-mix(in srgb, var(--primary) 16%, transparent)", color: "var(--primary)" }}
          >
            <Sparkles className="h-4 w-4" />
          </div>
          <div>
            <div className="text-[15px] font-semibold" style={{ color: "var(--dtext)" }}>AI Assist</div>
            <div className="compose-meta-text font-mono text-[10.5px] uppercase tracking-[0.12em]">
              In development
            </div>
          </div>
        </div>
        <p className="compose-support-text text-[13px] leading-relaxed">
          Generate, refine, and compare post suggestions without leaving the compose flow.
        </p>
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-5">
        <section className="mb-5">
          <div className="compose-panel-label mb-2 text-[11px] font-semibold uppercase tracking-[0.11em]">
            Quick actions
          </div>
          <div className="space-y-2.5">
            {actions.map((action) => {
              const active = mode === action.id;
              return (
                <button
                  key={action.id}
                  type="button"
                  disabled={action.disabled}
                  onClick={() => onModeChange(action.id)}
                  className="compose-action-card w-full rounded-xl border px-4 py-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                  style={{
                    borderColor: active ? "color-mix(in srgb, var(--primary) 55%, transparent)" : "var(--dborder)",
                    background: active ? "color-mix(in srgb, var(--primary) 10%, var(--surface-raised))" : "var(--surface-raised)",
                  }}
                >
                  <div className="mb-1 flex items-center gap-2">
                    <Wand2 className="h-3.5 w-3.5" style={{ color: active ? "var(--primary)" : "var(--dmuted2)" }} />
                    <span className="text-[13.5px] font-medium" style={{ color: "var(--dtext)" }}>{action.title}</span>
                  </div>
                  <div className="compose-support-text text-[12.5px] leading-relaxed">
                    {action.description}
                  </div>
                </button>
              );
            })}
          </div>
        </section>

        <section className="compose-surface-panel mb-5 rounded-xl border px-4 py-4">
          <div className="compose-panel-label mb-2 text-[11px] font-semibold uppercase tracking-[0.11em]">
            Current context
          </div>
          <div className="compose-support-text space-y-2 text-[12.5px]">
            <div className="flex items-center justify-between gap-3">
              <span>Selected platforms</span>
              <span className="font-mono text-[11px]" style={{ color: "var(--dtext)" }}>{selectedPlatformsCount}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span>Uploaded media</span>
              <span className="font-mono text-[11px]" style={{ color: "var(--dtext)" }}>{mediaCount}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span>Main caption</span>
              <span className="font-mono text-[11px]" style={{ color: "var(--dtext)" }}>
                {hasMainContent ? "ready" : "empty"}
              </span>
            </div>
          </div>
        </section>

        <section className="compose-surface-panel rounded-xl border px-4 py-4">
          <div className="compose-panel-label mb-2 text-[11px] font-semibold uppercase tracking-[0.11em]">
            Preview
          </div>
          <div className="compose-support-text mb-3 text-[13px] leading-relaxed">
            {mode === "brief" && "AI will turn a campaign brief into a first-pass caption and platform-specific variants."}
            {mode === "improve" && "AI will refine your current draft for clarity, energy, and conversion intent."}
            {mode === "adapt" && "AI will create separate versions for each selected destination account."}
            {mode === "media" && "AI will use your uploaded media as context for hooks, captions, and CTA ideas."}
            {mode === "fix_validation" && "AI will target text-related problems while leaving compliance-sensitive settings alone."}
            {!mode && "Choose a mode to see generated suggestions, apply controls, and side-by-side comparisons here."}
          </div>
          {mode === "brief" ? (
            <div className="space-y-3">
              <div className="compose-surface-subtle space-y-3 rounded-lg border px-3.5 py-3.5">
                <div>
                  <label className="compose-panel-label mb-1 block font-mono text-[10.5px] uppercase tracking-[0.12em]">
                    Campaign brief
                  </label>
                  <textarea
                    value={brief}
                    onChange={(event) => onBriefChange(event.target.value)}
                    rows={5}
                    placeholder="Describe the launch, offer, audience, and the angle you want to emphasize."
                    className="compose-field w-full rounded-lg border px-3 py-2.5 text-[13px] leading-relaxed outline-none transition-colors"
                    style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="compose-panel-label mb-1 block font-mono text-[10.5px] uppercase tracking-[0.12em]">
                      Objective
                    </label>
                    <select
                      value={objective}
                      onChange={(event) => onObjectiveChange(event.target.value as "awareness" | "engagement" | "clicks" | "sales")}
                      className="compose-field w-full rounded-lg border px-3 py-2.5 text-[13px] outline-none transition-colors"
                      style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}
                    >
                      <option value="awareness">Awareness</option>
                      <option value="engagement">Engagement</option>
                      <option value="clicks">Clicks</option>
                      <option value="sales">Sales</option>
                    </select>
                  </div>
                  <div>
                    <label className="compose-panel-label mb-1 block font-mono text-[10.5px] uppercase tracking-[0.12em]">
                      Tone
                    </label>
                    <select
                      value={tone}
                      onChange={(event) => onToneChange(event.target.value as "professional" | "friendly" | "bold" | "playful")}
                      className="compose-field w-full rounded-lg border px-3 py-2.5 text-[13px] outline-none transition-colors"
                      style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}
                    >
                      <option value="professional">Professional</option>
                      <option value="friendly">Friendly</option>
                      <option value="bold">Bold</option>
                      <option value="playful">Playful</option>
                    </select>
                  </div>
                </div>
                <label className="compose-support-text flex items-center gap-2 text-[12.5px]">
                  <input
                    type="checkbox"
                    checked={includeCTA}
                    onChange={(event) => onIncludeCTAChange(event.target.checked)}
                  />
                  Include a stronger CTA in the first draft
                </label>
              </div>
              <button
                type="button"
                onClick={onGenerate}
                disabled={!brief.trim() || loading}
                className="inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
              >
                {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                {loading ? "Generating..." : "Generate from brief"}
              </button>
              {error ? (
                <div className="rounded-lg border px-3 py-3 text-[12.5px] leading-relaxed" style={{ borderColor: "color-mix(in srgb, var(--danger) 55%, transparent)", color: "var(--danger)" }}>
                  {error}
                </div>
              ) : null}
              {suggestion?.main_caption || (suggestion?.platform_captions && suggestion.platform_captions.length > 0) ? (
                <div className="space-y-3">
                  {suggestion.summary ? (
                    <div className="compose-support-text text-[12px] leading-relaxed">
                      {suggestion.summary}
                    </div>
                  ) : null}
                  {suggestion.main_caption ? (
                    <div className="rounded-lg border px-4 py-4" style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}>
                      <div className="mb-2 text-[13px] font-medium" style={{ color: "var(--dtext)" }}>
                        Main caption from brief
                      </div>
                      <div className="mb-3 rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
                          <div className="compose-panel-label mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]">
                            Current
                          </div>
                          <div className="compose-support-text text-[12.5px] leading-relaxed">
                            {currentMainCaption || "(empty)"}
                          </div>
                      </div>
                      <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
                        {suggestion.main_caption}
                      </div>
                      <div className="mt-3">
                        <button
                          type="button"
                          onClick={onApplyMainCaption}
                          className="rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors"
                          style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
                        >
                          Apply to main caption
                        </button>
                      </div>
                    </div>
                  ) : null}
                  {suggestion.platform_captions && suggestion.platform_captions.length > 1 ? (
                    <div className="flex items-center justify-end">
                      <button
                        type="button"
                        onClick={onApplyAllPlatformCaptions}
                        className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
                        style={{ background: "var(--surface1)", color: "var(--dtext)", border: "1px solid var(--dborder)" }}
                      >
                        Apply all platform variants
                      </button>
                    </div>
                  ) : null}
                  {suggestion.platform_captions?.map((item) => (
                    <div
                      key={`${item.account_id}-${item.platform}-brief`}
                      className="rounded-lg border px-4 py-4"
                      style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}
                    >
                      <div className="mb-2 flex items-center justify-between gap-3">
                        <div>
                          <div className="text-[13px] font-medium" style={{ color: "var(--dtext)" }}>
                            {accountLabels[item.account_id] || item.platform}
                          </div>
                          <div className="compose-panel-label font-mono text-[10.5px] uppercase tracking-[0.12em]">
                            {item.platform}
                          </div>
                        </div>
                        <button
                          type="button"
                          onClick={() => onApplyPlatformCaption(item.account_id, item.caption)}
                          className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
                          style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
                        >
                          Apply
                        </button>
                      </div>
                      <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
                        <div className="mb-2 rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
                          <div className="compose-panel-label mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]">
                            Current
                          </div>
                          <div className="compose-support-text text-[12.5px] leading-relaxed">
                            {currentPlatformCaptions[item.account_id] || currentMainCaption || "(empty)"}
                          </div>
                        </div>
                        <div className="compose-panel-label mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]">
                          Suggested
                        </div>
                        {item.caption}
                      </div>
                      {item.reason ? (
                        <div className="compose-support-text mt-2 text-[12px] leading-relaxed">
                          {item.reason}
                        </div>
                      ) : null}
                    </div>
                  ))}
                  {suggestion.warnings && suggestion.warnings.length > 0 ? (
                    <div className="compose-meta-text text-[12px] leading-relaxed">
                      {suggestion.warnings[0]}
                    </div>
                  ) : null}
                </div>
              ) : (
                <div
                  className="compose-support-text rounded-lg border border-dashed px-4 py-5 text-[12.5px] leading-relaxed"
                  style={{ borderColor: "color-mix(in srgb, var(--dborder) 80%, transparent)" }}
                >
                  Add a short campaign brief to generate a first-pass caption and optional per-platform variants for the selected accounts.
                </div>
              )}
            </div>
          ) : mode === "improve" ? (
            <div className="space-y-3">
              <button
                type="button"
                onClick={onGenerate}
                disabled={!hasMainContent || loading}
                className="inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
              >
                {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                {loading ? "Generating..." : "Generate suggestion"}
              </button>
              {error ? (
                <div className="rounded-lg border px-3 py-3 text-[12.5px] leading-relaxed" style={{ borderColor: "color-mix(in srgb, var(--danger) 55%, transparent)", color: "var(--danger)" }}>
                  {error}
                </div>
              ) : null}
              {suggestion?.main_caption ? (
                <div className="rounded-lg border px-4 py-4" style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}>
                  {suggestion.summary ? (
                    <div className="mb-2 text-[12px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                      {suggestion.summary}
                    </div>
                  ) : null}
                  <div className="mb-3 grid gap-2">
                    <div className="rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
                      <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                        Current
                      </div>
                      <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                        {currentMainCaption || "(empty)"}
                      </div>
                    </div>
                  </div>
                  <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
                    {suggestion.main_caption}
                  </div>
                  <div className="mt-3 flex items-center gap-2">
                    <button
                      type="button"
                      onClick={onApplyMainCaption}
                      className="rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors"
                      style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
                    >
                      Apply to main caption
                    </button>
                  </div>
                  <div className="mt-3">
                    <AIFirstCommentSuggestions
                      suggestion={suggestion}
                      accountLabels={accountLabels}
                      accountPlatforms={accountPlatforms}
                      platformCapabilities={platformCapabilities}
                      currentFirstComments={currentFirstComments}
                      onApplyFirstCommentSuggestion={onApplyFirstCommentSuggestion}
                      onApplyAllFirstCommentSuggestions={onApplyAllFirstCommentSuggestions}
                    />
                  </div>
                  {suggestion.warnings && suggestion.warnings.length > 0 ? (
                    <div className="compose-meta-text mt-3 text-[12px] leading-relaxed">
                      {suggestion.warnings[0]}
                    </div>
                  ) : null}
                </div>
              ) : (
                <div
                  className="compose-support-text rounded-lg border border-dashed px-4 py-5 text-[12.5px] leading-relaxed"
                  style={{ borderColor: "color-mix(in srgb, var(--dborder) 80%, transparent)" }}
                >
                  Generate a suggestion to preview an improved version of the main caption and apply it back into the compose flow.
                </div>
              )}
            </div>
          ) : mode === "adapt" ? (
            <div className="space-y-3">
              <button
                type="button"
                onClick={onGenerate}
                disabled={!hasMainContent || selectedPlatformsCount === 0 || loading}
                className="inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
              >
                {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                {loading ? "Generating..." : "Generate platform variants"}
              </button>
              {error ? (
                <div className="rounded-lg border px-3 py-3 text-[12.5px] leading-relaxed" style={{ borderColor: "color-mix(in srgb, var(--danger) 55%, transparent)", color: "var(--danger)" }}>
                  {error}
                </div>
              ) : null}
              {suggestion?.platform_captions && suggestion.platform_captions.length > 0 ? (
                <div className="space-y-3">
                  {suggestion.summary ? (
                    <div className="text-[12px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                      {suggestion.summary}
                    </div>
                  ) : null}
                  <div className="flex items-center justify-end">
                    <button
                      type="button"
                      onClick={onApplyAllPlatformCaptions}
                      className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
                      style={{ background: "var(--surface1)", color: "var(--dtext)", border: "1px solid var(--dborder)" }}
                    >
                      Apply all
                    </button>
                  </div>
                  {suggestion.platform_captions.map((item) => (
                    <div
                      key={`${item.account_id}-${item.platform}`}
                      className="rounded-lg border px-4 py-4"
                      style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}
                    >
                      <div className="mb-2 flex items-center justify-between gap-3">
                        <div>
                          <div className="text-[13px] font-medium" style={{ color: "var(--dtext)" }}>
                            {accountLabels[item.account_id] || item.platform}
                          </div>
                          <div className="font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                            {item.platform}
                          </div>
                        </div>
                        <button
                          type="button"
                          onClick={() => onApplyPlatformCaption(item.account_id, item.caption)}
                          className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
                          style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
                        >
                          Apply
                        </button>
                      </div>
                      <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
                        <div className="mb-2 rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
                          <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                            Current
                          </div>
                          <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                            {currentPlatformCaptions[item.account_id] || currentMainCaption || "(empty)"}
                          </div>
                        </div>
                        <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                          Suggested
                        </div>
                        {item.caption}
                      </div>
                      {item.reason ? (
                        <div className="mt-2 text-[12px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                          {item.reason}
                        </div>
                      ) : null}
                    </div>
                  ))}
                  {suggestion.warnings && suggestion.warnings.length > 0 ? (
                    <div className="text-[12px] leading-relaxed" style={{ color: "var(--dmuted2)" }}>
                      {suggestion.warnings[0]}
                    </div>
                  ) : null}
                  <AIFirstCommentSuggestions
                    suggestion={suggestion}
                    accountLabels={accountLabels}
                    accountPlatforms={accountPlatforms}
                    platformCapabilities={platformCapabilities}
                    currentFirstComments={currentFirstComments}
                    onApplyFirstCommentSuggestion={onApplyFirstCommentSuggestion}
                    onApplyAllFirstCommentSuggestions={onApplyAllFirstCommentSuggestions}
                  />
                </div>
              ) : (
                <div
                  className="compose-support-text rounded-lg border border-dashed px-4 py-5 text-[12.5px] leading-relaxed"
                  style={{ borderColor: "color-mix(in srgb, var(--dborder) 80%, transparent)" }}
                >
                  Generate per-platform variants to preview account-specific captions and apply them into the platform override editors.
                </div>
              )}
            </div>
          ) : mode === "media" ? (
            <div className="space-y-3">
              <button
                type="button"
                onClick={onGenerate}
                disabled={mediaCount === 0 || loading}
                className="inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
              >
                {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                {loading ? "Generating..." : "Generate from media"}
              </button>
              {error ? (
                <div className="rounded-lg border px-3 py-3 text-[12.5px] leading-relaxed" style={{ borderColor: "color-mix(in srgb, var(--danger) 55%, transparent)", color: "var(--danger)" }}>
                  {error}
                </div>
              ) : null}
              {suggestion?.main_caption || (suggestion?.platform_captions && suggestion.platform_captions.length > 0) ? (
                <div className="space-y-3">
                  {suggestion.summary ? (
                    <div className="text-[12px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                      {suggestion.summary}
                    </div>
                  ) : null}
                  {suggestion.main_caption ? (
                    <div className="rounded-lg border px-4 py-4" style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}>
                      <div className="mb-2 text-[13px] font-medium" style={{ color: "var(--dtext)" }}>
                        Main caption from media
                      </div>
                      <div className="mb-3 rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
                        <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                          Current
                        </div>
                        <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                          {currentMainCaption || "(empty)"}
                        </div>
                      </div>
                      <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
                        {suggestion.main_caption}
                      </div>
                      <div className="mt-3">
                        <button
                          type="button"
                          onClick={onApplyMainCaption}
                          className="rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors"
                          style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
                        >
                          Apply to main caption
                        </button>
                      </div>
                    </div>
                  ) : null}
                  {suggestion.platform_captions && suggestion.platform_captions.length > 1 ? (
                    <div className="flex items-center justify-end">
                      <button
                        type="button"
                        onClick={onApplyAllPlatformCaptions}
                        className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
                        style={{ background: "var(--surface1)", color: "var(--dtext)", border: "1px solid var(--dborder)" }}
                      >
                        Apply all platform variants
                      </button>
                    </div>
                  ) : null}
                  {suggestion.platform_captions?.map((item) => (
                    <div
                      key={`${item.account_id}-${item.platform}-media`}
                      className="rounded-lg border px-4 py-4"
                      style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}
                    >
                      <div className="mb-2 flex items-center justify-between gap-3">
                        <div>
                          <div className="text-[13px] font-medium" style={{ color: "var(--dtext)" }}>
                            {accountLabels[item.account_id] || item.platform}
                          </div>
                          <div className="font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                            {item.platform}
                          </div>
                        </div>
                        <button
                          type="button"
                          onClick={() => onApplyPlatformCaption(item.account_id, item.caption)}
                          className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
                          style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
                        >
                          Apply
                        </button>
                      </div>
                      <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
                        <div className="mb-2 rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
                          <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                            Current
                          </div>
                          <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                            {currentPlatformCaptions[item.account_id] || currentMainCaption || "(empty)"}
                          </div>
                        </div>
                        <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                          Suggested
                        </div>
                        {item.caption}
                      </div>
                      {item.reason ? (
                        <div className="mt-2 text-[12px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                          {item.reason}
                        </div>
                      ) : null}
                    </div>
                  ))}
                  {suggestion.warnings && suggestion.warnings.length > 0 ? (
                    <div className="text-[12px] leading-relaxed" style={{ color: "var(--dmuted2)" }}>
                      {suggestion.warnings[0]}
                    </div>
                  ) : null}
                  <AIFirstCommentSuggestions
                    suggestion={suggestion}
                    accountLabels={accountLabels}
                    accountPlatforms={accountPlatforms}
                    platformCapabilities={platformCapabilities}
                    currentFirstComments={currentFirstComments}
                    onApplyFirstCommentSuggestion={onApplyFirstCommentSuggestion}
                    onApplyAllFirstCommentSuggestions={onApplyAllFirstCommentSuggestions}
                  />
                </div>
              ) : (
                <div
                  className="compose-support-text rounded-lg border border-dashed px-4 py-5 text-[12.5px] leading-relaxed"
                  style={{ borderColor: "color-mix(in srgb, var(--dborder) 80%, transparent)" }}
                >
                  Generate a caption from your uploaded images or video, then apply the main draft or per-platform variants back into the compose flow.
                </div>
              )}
            </div>
          ) : mode === "fix_validation" ? (
            <div className="space-y-3">
              <button
                type="button"
                onClick={onGenerate}
                disabled={loading}
                className="inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
              >
                {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                {loading ? "Generating..." : "Suggest fixes"}
              </button>
              {error ? (
                <div className="rounded-lg border px-3 py-3 text-[12.5px] leading-relaxed" style={{ borderColor: "color-mix(in srgb, var(--danger) 55%, transparent)", color: "var(--danger)" }}>
                  {error}
                </div>
              ) : null}
              {suggestion?.main_caption || (suggestion?.platform_captions && suggestion.platform_captions.length > 0) ? (
                <div className="space-y-3">
                  {suggestion.summary ? (
                    <div className="text-[12px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                      {suggestion.summary}
                    </div>
                  ) : null}
                  {suggestion.main_caption ? (
                    <div className="rounded-lg border px-4 py-4" style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}>
                      <div className="mb-2 text-[13px] font-medium" style={{ color: "var(--dtext)" }}>
                        Main caption fix
                      </div>
                      <div className="mb-3 rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
                        <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                          Current
                        </div>
                        <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                          {currentMainCaption || "(empty)"}
                        </div>
                      </div>
                      <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
                        {suggestion.main_caption}
                      </div>
                      <div className="mt-3">
                        <button
                          type="button"
                          onClick={onApplyMainCaption}
                          className="rounded-lg px-3.5 py-2 text-[12.5px] font-medium transition-colors"
                          style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
                        >
                          Apply to main caption
                        </button>
                      </div>
                    </div>
                  ) : null}
                  {suggestion.platform_captions && suggestion.platform_captions.length > 1 ? (
                    <div className="flex items-center justify-end">
                      <button
                        type="button"
                        onClick={onApplyAllPlatformCaptions}
                        className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
                        style={{ background: "var(--surface1)", color: "var(--dtext)", border: "1px solid var(--dborder)" }}
                      >
                        Apply all platform fixes
                      </button>
                    </div>
                  ) : null}
                  {suggestion.platform_captions?.map((item) => (
                    <div
                      key={`${item.account_id}-${item.platform}-fix`}
                      className="rounded-lg border px-4 py-4"
                      style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}
                    >
                      <div className="mb-2 flex items-center justify-between gap-3">
                        <div>
                          <div className="text-[13px] font-medium" style={{ color: "var(--dtext)" }}>
                            {accountLabels[item.account_id] || item.platform}
                          </div>
                          <div className="font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                            {item.platform}
                          </div>
                        </div>
                        <button
                          type="button"
                          onClick={() => onApplyPlatformCaption(item.account_id, item.caption)}
                          className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
                          style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
                        >
                          Apply
                        </button>
                      </div>
                      <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
                        <div className="mb-2 rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
                          <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                            Current
                          </div>
                          <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                            {currentPlatformCaptions[item.account_id] || currentMainCaption || "(empty)"}
                          </div>
                        </div>
                        <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                          Suggested
                        </div>
                        {item.caption}
                      </div>
                      {item.reason ? (
                        <div className="mt-2 text-[12px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                          {item.reason}
                        </div>
                      ) : null}
                    </div>
                  ))}
                  {suggestion.warnings && suggestion.warnings.length > 0 ? (
                    <div className="text-[12px] leading-relaxed" style={{ color: "var(--dmuted2)" }}>
                      {suggestion.warnings[0]}
                    </div>
                  ) : null}
                  <AIFirstCommentSuggestions
                    suggestion={suggestion}
                    accountLabels={accountLabels}
                    accountPlatforms={accountPlatforms}
                    platformCapabilities={platformCapabilities}
                    currentFirstComments={currentFirstComments}
                    onApplyFirstCommentSuggestion={onApplyFirstCommentSuggestion}
                    onApplyAllFirstCommentSuggestions={onApplyAllFirstCommentSuggestions}
                  />
                </div>
              ) : (
                <div
                  className="compose-support-text rounded-lg border border-dashed px-4 py-5 text-[12.5px] leading-relaxed"
                  style={{ borderColor: "color-mix(in srgb, var(--dborder) 80%, transparent)" }}
                >
                  Suggest targeted text fixes for the current validation issues and apply them back into the compose flow.
                </div>
              )}
            </div>
          ) : (
            <div
              className="compose-support-text rounded-lg border border-dashed px-4 py-5 text-[12.5px] leading-relaxed"
              style={{ borderColor: "color-mix(in srgb, var(--dborder) 80%, transparent)" }}
            >
              Suggestion results, per-platform apply actions, and compare views will appear here as each AI mode is implemented.
            </div>
          )}
        </section>
      </div>
    </aside>
  );
}

function AIFirstCommentSuggestions({
  suggestion,
  accountLabels,
  accountPlatforms,
  platformCapabilities,
  currentFirstComments,
  onApplyFirstCommentSuggestion,
  onApplyAllFirstCommentSuggestions,
}: {
  suggestion: AIPostAssistSuggestion;
  accountLabels: Record<string, string>;
  accountPlatforms: Record<string, string>;
  platformCapabilities: PlatformCapabilitiesEnvelope["platforms"] | null;
  currentFirstComments: Record<string, string>;
  onApplyFirstCommentSuggestion: (accountId: string, text: string) => void;
  onApplyAllFirstCommentSuggestions: () => void;
}) {
  const supportedSuggestions = (suggestion.first_comment_suggestions || []).filter((item) =>
    supportsFirstComment(accountPlatforms[item.account_id] || "", platformCapabilities)
  );
  const unsupportedSuggestions = (suggestion.first_comment_suggestions || []).filter((item) =>
    !supportsFirstComment(accountPlatforms[item.account_id] || "", platformCapabilities)
  );
  if (supportedSuggestions.length === 0 && unsupportedSuggestions.length === 0) return null;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="text-[12px] font-medium" style={{ color: "var(--dtext)" }}>
          First comment suggestions
        </div>
        {supportedSuggestions.length > 1 ? (
          <button
            type="button"
            onClick={onApplyAllFirstCommentSuggestions}
            className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
            style={{ background: "var(--surface1)", color: "var(--dtext)", border: "1px solid var(--dborder)" }}
          >
            Apply all
          </button>
        ) : null}
      </div>
      {supportedSuggestions.map((item) => (
        <div
          key={`${item.account_id}-first-comment`}
          className="rounded-lg border px-4 py-4"
          style={{ borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)", background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))" }}
        >
          <div className="mb-2 flex items-center justify-between gap-3">
            <div className="text-[13px] font-medium" style={{ color: "var(--dtext)" }}>
              {accountLabels[item.account_id] || item.account_id}
            </div>
            <button
              type="button"
              onClick={() => onApplyFirstCommentSuggestion(item.account_id, item.text)}
              className="rounded-lg px-3 py-1.5 text-[12px] font-medium transition-colors"
              style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
            >
              Apply
            </button>
          </div>
          <div className="rounded-md border px-3 py-3 text-[13px] leading-relaxed" style={{ borderColor: "var(--dborder)", background: "var(--surface1)", color: "var(--dtext)" }}>
            <div className="mb-2 rounded-md border px-3 py-2.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 60%, transparent)" }}>
              <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
                Current
              </div>
              <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                {currentFirstComments[item.account_id] || "(empty)"}
              </div>
            </div>
            <div className="mb-1 font-mono text-[10.5px] uppercase tracking-[0.12em]" style={{ color: "var(--dmuted2)" }}>
              Suggested
            </div>
            {item.text}
          </div>
        </div>
      ))}
      {unsupportedSuggestions.length > 0 ? (
        <div
          className="rounded-lg border px-3 py-3 text-[12.5px] leading-relaxed"
          style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 55%, transparent)", color: "var(--dmuted)" }}
        >
          {unsupportedSuggestions.map((item) => (
            <div key={`${item.account_id}-unsupported`}>
              {accountLabels[item.account_id] || item.account_id} does not support first comments, so this suggestion was not made editable.
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

const ISSUE_COPY: Record<string, string> = {
  exceeds_max_length: "Shorten the caption for this destination.",
  below_min_length: "Add more content before publishing.",
  missing_required: "Fill in the required field before publishing.",
  youtube_title_required: "Add a YouTube video title before publishing.",
  youtube_made_for_kids_required: "Choose whether the video is made for kids before publishing.",
  invalid_privacy_status: "Pick a valid YouTube visibility.",
  invalid_license: "Pick a valid YouTube license.",
  invalid_publish_at: "Use a valid YouTube publish time.",
  invalid_recording_date: "Use a valid recording date.",
  invalid_default_language: "Use a valid default language such as en or en-US.",
  youtube_publish_at_requires_private: "Set visibility to Private when using YouTube publish time.",
  invalid_instagram_media_type: "Choose a valid Instagram media type.",
  instagram_reels_require_video: "Instagram Reels require exactly one video.",
  instagram_story_single_media_only: "Instagram Stories require exactly one image or video.",
  mixed_media_unsupported: "Use either images or video, not both, for this platform.",
  scheduled_too_soon: "Choose a time at least 30 seconds in the future.",
  scheduled_too_far: "Choose a scheduled time within the supported window.",
  media_not_uploaded: "Wait for uploads to finish or remove the pending media.",
  media_id_not_found: "Re-upload the media asset before publishing.",
  media_id_not_in_workspace: "This media belongs to a different workspace.",
  account_disconnected: "Reconnect this account before publishing.",
  account_not_found: "Select a valid connected account.",
  account_not_in_workspace: "This account is not available in the current workspace.",
  first_comment_unsupported: "Remove the first comment for this platform.",
  unsupported_in_reply_to: "Remove the reply target for this platform.",
  thread_positions_not_contiguous: "Use consecutive thread positions without gaps.",
  thread_mixed_with_single: "Separate thread posts from standalone posts.",
};

function issueSummary(issue: SocialPostValidationIssue): string {
  if (issue.code === "missing_required") return issue.message;
  return ISSUE_COPY[issue.code] || issue.message;
}

function issueTargetLabel(issue: SocialPostValidationIssue, accounts: SocialAccount[]): string {
  if (issue.account_id) {
    const account = accounts.find((candidate) => candidate.id === issue.account_id);
    const platformLabel = account?.platform || issue.platform || "platform";
    const accountLabel = account ? getAccountDisplayName(account) : platformLabel;
    return `${platformLabel} · ${accountLabel}`;
  }
  if (issue.field === "scheduled_at") return "Publish settings";
  if (issue.field === "media_ids" || issue.field === "media_urls") return "Media";
  return "Post setup";
}

function ValidationPanel({
  errors,
  warnings,
  accounts,
  onSelectIssue,
}: {
  errors: SocialPostValidationIssue[];
  warnings: SocialPostValidationIssue[];
  accounts: SocialAccount[];
  onSelectIssue: (issue: SocialPostValidationIssue) => void;
}) {
  if (errors.length === 0 && warnings.length === 0) return null;

  return (
    <section className="mb-5 space-y-3">
      {errors.length > 0 && (
        <div className="rounded-xl border px-4 py-3.5" style={{ borderColor: "color-mix(in srgb, var(--danger) 55%, transparent)", background: "color-mix(in srgb, var(--danger) 16%, var(--surface-raised))" }}>
          <div className="mb-2 flex items-center gap-2" style={{ color: "var(--danger)" }}>
            <AlertTriangle className="w-4 h-4" />
            <div className="text-[12px] font-mono uppercase tracking-[0.12em]">
              {errors.length} blocking issue{errors.length === 1 ? "" : "s"}
            </div>
          </div>
          <div className="space-y-2.5">
            {errors.map((issue, index) => (
              <button
                key={`${issue.code}-${issue.field}-${issue.account_id || "global"}-${index}`}
                type="button"
                onClick={() => onSelectIssue(issue)}
                className="w-full rounded-lg border px-3 py-2.5 text-left transition-colors"
                style={{ borderColor: "color-mix(in srgb, var(--danger) 45%, transparent)", background: "color-mix(in srgb, var(--danger) 22%, var(--surface-raised))" }}
              >
                <div className="mb-1 font-mono text-[11px] uppercase tracking-[0.12em]" style={{ color: "var(--danger)" }}>
                  {issueTargetLabel(issue, accounts)}
                </div>
                <div className="text-[13px] leading-relaxed" style={{ color: "var(--text)" }}>{issueSummary(issue)}</div>
              </button>
            ))}
          </div>
        </div>
      )}

      {warnings.length > 0 && (
        <div className="rounded-xl border px-4 py-3.5" style={{ borderColor: "color-mix(in srgb, var(--warning) 55%, transparent)", background: "color-mix(in srgb, var(--warning) 16%, var(--surface-raised))" }}>
          <div className="mb-2 flex items-center gap-2" style={{ color: "var(--warning)" }}>
            <AlertTriangle className="w-4 h-4" />
            <div className="text-[12px] font-mono uppercase tracking-[0.12em]">
              {warnings.length} warning{warnings.length === 1 ? "" : "s"}
            </div>
          </div>
          <div className="space-y-2.5">
            {warnings.map((issue, index) => (
              <button
                key={`${issue.code}-${issue.field}-${issue.account_id || "global"}-${index}`}
                type="button"
                onClick={() => onSelectIssue(issue)}
                className="w-full rounded-lg border px-3 py-2.5 text-left transition-colors"
                style={{ borderColor: "color-mix(in srgb, var(--warning) 45%, transparent)", background: "color-mix(in srgb, var(--warning) 22%, var(--surface-raised))" }}
              >
                <div className="mb-1 font-mono text-[11px] uppercase tracking-[0.12em]" style={{ color: "var(--warning)" }}>
                  {issueTargetLabel(issue, accounts)}
                </div>
                <div className="text-[13px] leading-relaxed" style={{ color: "var(--text)" }}>{issueSummary(issue)}</div>
              </button>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}

function CapabilitySummary({
  selectedAccounts,
  platformCapabilities,
}: {
  selectedAccounts: SocialAccount[];
  platformCapabilities: PlatformCapabilitiesEnvelope["platforms"] | null;
}) {
  if (selectedAccounts.length === 0) return null;

  const firstCommentCount = selectedAccounts.filter((account) =>
    supportsFirstComment(account.platform, platformCapabilities)
  ).length;
  const threadCount = selectedAccounts.filter((account) =>
    supportsThreads(account.platform, platformCapabilities)
  ).length;
  const schedulingCount = selectedAccounts.filter((account) =>
    supportsScheduling(account.platform, platformCapabilities)
  ).length;

  return (
    <section className="mb-5 rounded-xl border px-4 py-3.5" style={{ borderColor: "var(--dborder)", background: "color-mix(in srgb, var(--surface2) 45%, transparent)" }}>
      <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
        Capability summary
      </div>
      <div className="space-y-1.5 text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
        <div>{firstCommentCount} of {selectedAccounts.length} selected accounts support first comments.</div>
        <div>{threadCount} of {selectedAccounts.length} selected accounts support native threads.</div>
        <div>{schedulingCount} of {selectedAccounts.length} selected accounts support scheduled publishing.</div>
      </div>
    </section>
  );
}

// ─────────────────────────────────────────────────────────────────────

interface CreatePostDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  accounts: SocialAccount[];
  workspaceId: string;
  profileName?: string;
  getToken: () => Promise<string | null>;
  onCreated: (postId?: string) => void | Promise<void>;
  // Activation guide: prefill caption + preselect all connected accounts
  // so a first-time user just clicks Publish.
  initialCaption?: string;
  preselectAllAccounts?: boolean;
  preselectedAccountIds?: string[];
}

export function CreatePostDrawer({
  open,
  onOpenChange,
  accounts,
  workspaceId,
  profileName,
  getToken,
  onCreated,
  initialCaption,
  preselectAllAccounts,
  preselectedAccountIds,
}: CreatePostDrawerProps) {
  // Profile management
  const [previewFile, setPreviewFile] = useState<File | null>(null);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [selectedProfileId, setSelectedProfileId] = useState("");
  const [profileAccounts, setProfileAccounts] = useState<SocialAccount[]>(accounts);
  const [allLoadedAccounts, setAllLoadedAccounts] = useState<SocialAccount[]>(accounts);

  const form = useCreatePostForm(allLoadedAccounts);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);
  const [queues] = useState<Array<{ id: string; name: string }>>([]);
  const [queuesLoaded, setQueuesLoaded] = useState(false);
  const [isValidating, setIsValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<SocialPostValidationResult | null>(null);
  const [validationChecked, setValidationChecked] = useState(false);
  const [warningsAcknowledged, setWarningsAcknowledged] = useState(false);
  const [submitError, setSubmitError] = useState<{ message: string; mailto: string; contactHref: string } | null>(null);
  const [submitSuccess, setSubmitSuccess] = useState<{ message: string; tiktokURL?: string } | null>(null);
  const [isSuperAdmin, setIsSuperAdmin] = useState(false);
  const [isAIPanelOpen, setIsAIPanelOpen] = useState(false);
  const [aiMode, setAIMode] = useState<AIAssistMode | null>(null);
  const [aiSuggestion, setAISuggestion] = useState<AIPostAssistSuggestion | null>(null);
  const [aiLoading, setAILoading] = useState(false);
  const [aiError, setAIError] = useState<string | null>(null);
  const [aiBrief, setAIBrief] = useState("");
  const [aiObjective, setAIObjective] = useState<AIAssistObjective>("engagement");
  const [aiTone, setAITone] = useState<AIAssistTone>("friendly");
  const [aiIncludeCTA, setAIIncludeCTA] = useState(true);
  const [platformCapabilities, setPlatformCapabilities] = useState<PlatformCapabilitiesEnvelope["platforms"] | null>(null);
  // Per-account runtime blockers reported by platform-specific panels
  // (e.g., TikTok creator_info failed, video too long for the creator).
  // These aren't derivable from form state, so we collect them here and
  // fold them into disabledReason + the primary button's disabled prop.
  const [tiktokBlockers, setTiktokBlockers] = useState<Record<string, string>>({});
  // Per-account creator video-length cap reported by each TikTokFields
  // panel once creator_info resolves. Missing entry = cap unknown.
  const [tiktokMaxByAccount, setTiktokMaxByAccount] = useState<Record<string, number>>({});
  const pendingCloseRef = useRef(false);
  const mainContentRef = useRef<HTMLTextAreaElement | null>(null);
  const mediaSectionRef = useRef<HTMLDivElement | null>(null);
  const publishPanelRef = useRef<HTMLDivElement | null>(null);
  const platformBlockRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const bodyLayoutRef = useRef<HTMLDivElement | null>(null);
  const [drawerWidth, setDrawerWidth] = useState(() =>
    typeof window === "undefined" ? 1080 : clampDrawerWidth(window.innerWidth * 0.75)
  );
  const [rightPaneWidth, setRightPaneWidth] = useState(COMPOSE_DEFAULT_RIGHT_PANE_WIDTH);
  const [aiPaneWidth, setAIPaneWidth] = useState(COMPOSE_DEFAULT_AI_PANE_WIDTH);
  const isDraggingWidthRef = useRef(false);

  const getComposeBodyWidth = useCallback(() => {
    return bodyLayoutRef.current?.getBoundingClientRect().width ?? 0;
  }, []);

  // Load profiles when drawer opens
  useEffect(() => {
    if (!open) return;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await listProfiles(token);
        const loaded = Array.isArray(res.data) ? res.data : [];
        setProfiles(loaded);
        if (loaded.length > 0 && !selectedProfileId) {
          setSelectedProfileId(loaded[0].id);
        }
      } catch (err) {
        console.error("Failed to load profiles:", err);
      }
    })();
  }, [open, getToken]); // eslint-disable-line react-hooks/exhaustive-deps

  // Load accounts when profile changes
  useEffect(() => {
    if (!selectedProfileId || !open) return;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await listSocialAccounts(token, selectedProfileId);
        setProfileAccounts(res.data);
        setAllLoadedAccounts((current) => {
          const next = new Map(current.map((account) => [account.id, account]));
          for (const account of res.data) {
            next.set(account.id, account);
          }
          return Array.from(next.values());
        });
      } catch (err) {
        console.error("Failed to load accounts:", err);
      }
    })();
  }, [selectedProfileId, open, getToken]);

  // Load queues lazily when switching to queue mode
  useEffect(() => {
    if (form.publishMode === "queue" && !queuesLoaded) {
      setQueuesLoaded(true);
    }
  }, [form.publishMode, queuesLoaded]);

  // Apply activation/replay prefill when the drawer opens from the
  // tutorial handoff. Replay prefers the exact account reconnected in
  // step 1; first-time activation falls back to selecting every account
  // in the current profile so the user can publish immediately.
  const appliedPrefillRef = useRef(false);
  useEffect(() => {
    if (!open) {
      appliedPrefillRef.current = false;
      return;
    }
    if (appliedPrefillRef.current) return;
    if (initialCaption) {
      form.setMainContent(initialCaption);
    }
    if (preselectedAccountIds && preselectedAccountIds.length > 0) {
      const available = new Set(profileAccounts.map((account) => account.id));
      const matching = preselectedAccountIds.filter((id) => available.has(id));
      if (matching.length > 0) {
        form.replaceSelectedAccounts(matching);
      }
    } else if (preselectAllAccounts && profileAccounts.length > 0) {
      form.replaceSelectedAccounts(profileAccounts.map((a) => a.id));
    }
    appliedPrefillRef.current = true;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialCaption, preselectAllAccounts, preselectedAccountIds, profileAccounts]);

  // Reset form when drawer closes
  useEffect(() => {
    if (!open) {
      form.reset();
      setPreviewFile(null);
      setSelectedProfileId("");
      setProfileAccounts(accounts);
      setAllLoadedAccounts(accounts);
      setShowDiscardConfirm(false);
      setQueuesLoaded(false);
      setValidationResult(null);
      setValidationChecked(false);
      setIsValidating(false);
      setWarningsAcknowledged(false);
      setSubmitError(null);
      setSubmitSuccess(null);
      setIsAIPanelOpen(false);
      setAIMode(null);
      setAISuggestion(null);
      setAILoading(false);
      setAIError(null);
      setTiktokBlockers({});
      setTiktokMaxByAccount({});
      pendingCloseRef.current = false;
    }
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getMe(token);
        if (!cancelled) setIsSuperAdmin(!!res.data.is_super_admin);
      } catch {
        if (!cancelled) setIsSuperAdmin(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, getToken]);

  const aiAssistEnabled = isFeatureInDevEnabledForMe("ai_assist_create_post_drawer", isSuperAdmin);

  useEffect(() => {
    if (!aiAssistEnabled && isAIPanelOpen) {
      setIsAIPanelOpen(false);
      setAIMode(null);
    }
  }, [aiAssistEnabled, isAIPanelOpen]);

  useEffect(() => {
    if (!open || !aiAssistEnabled || !isAIPanelOpen) return;
    setDrawerWidth((current) => clampDrawerWidth(Math.max(current, AI_DRAWER_WIDTH)));
  }, [open, aiAssistEnabled, isAIPanelOpen]);

  useEffect(() => {
    if (!open) return;
    const frame = window.requestAnimationFrame(() => {
      const totalWidth = getComposeBodyWidth();
      if (!totalWidth) return;
      const next = clampComposePaneWidths({
        totalWidth,
        rightPaneWidth,
        aiPaneWidth,
        aiOpen: aiAssistEnabled && isAIPanelOpen,
      });
      setRightPaneWidth((current) => (current === next.rightPaneWidth ? current : next.rightPaneWidth));
      setAIPaneWidth((current) => (current === next.aiPaneWidth ? current : next.aiPaneWidth));
    });

    return () => window.cancelAnimationFrame(frame);
  }, [open, drawerWidth, rightPaneWidth, aiPaneWidth, aiAssistEnabled, isAIPanelOpen, getComposeBodyWidth]);

  // Stable per-account blocker setter. We merge into a dict so each
  // TikTok field component only reports its own account's state, and
  // we prune cleared blockers instead of storing "" so the any-blocker
  // check is a simple Object.keys length.
  const setTiktokBlocker = useCallback((accountId: string, reason: string | null) => {
    setTiktokBlockers((prev) => {
      if (!reason) {
        if (!(accountId in prev)) return prev;
        const next = { ...prev };
        delete next[accountId];
        return next;
      }
      if (prev[accountId] === reason) return prev;
      return { ...prev, [accountId]: reason };
    });
  }, []);

  const setTiktokMaxDuration = useCallback((accountId: string, sec: number | null) => {
    setTiktokMaxByAccount((prev) => {
      if (sec == null || !Number.isFinite(sec) || sec <= 0) {
        if (!(accountId in prev)) return prev;
        const next = { ...prev };
        delete next[accountId];
        return next;
      }
      if (prev[accountId] === sec) return prev;
      return { ...prev, [accountId]: sec };
    });
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const onResize = () => setDrawerWidth((current) => clampDrawerWidth(current));
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  useEffect(() => {
    if (!open) return;
    setValidationResult(null);
    setValidationChecked(false);
    setWarningsAcknowledged(false);
    setSubmitError(null);
  }, [
    open,
    form.mainContent,
    form.selectedAccountIds,
    form.overrides,
    form.mediaItems,
    form.publishMode,
    form.scheduledAt,
    form.queueId,
  ]);

  useEffect(() => {
    setAISuggestion(null);
    setAIError(null);
  }, [aiMode, form.mainContent, form.selectedAccountIds, form.mediaItems, aiBrief, aiObjective, aiTone, aiIncludeCTA]);

  useEffect(() => {
    let cancelled = false;
    if (!open) return;
    (async () => {
      try {
        const res = await getPlatformCapabilities();
        if (!cancelled) setPlatformCapabilities(res.data.platforms);
      } catch {
        if (!cancelled) setPlatformCapabilities(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open]);

  const attemptClose = useCallback(() => {
    if (form.hasUnsavedContent) {
      setShowDiscardConfirm(true);
    } else {
      onOpenChange(false);
    }
  }, [form.hasUnsavedContent, onOpenChange]);

  const confirmDiscard = useCallback(() => {
    setShowDiscardConfirm(false);
    onOpenChange(false);
  }, [onOpenChange]);

  const handleResizeStart = useCallback((event: React.MouseEvent<HTMLDivElement>) => {
    event.preventDefault();
    isDraggingWidthRef.current = true;

    function onMouseMove(moveEvent: MouseEvent) {
      const nextWidth = clampDrawerWidth(window.innerWidth - moveEvent.clientX);
      setDrawerWidth(nextWidth);
    }

    function onMouseUp() {
      isDraggingWidthRef.current = false;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
    }

    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
  }, []);

  const handleContentSidebarResizeStart = useCallback((event: React.MouseEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startRightPaneWidth = rightPaneWidth;
    const startAIPaneWidth = aiPaneWidth;

    function onMouseMove(moveEvent: MouseEvent) {
      const totalWidth = getComposeBodyWidth();
      if (!totalWidth) return;
      const delta = moveEvent.clientX - startX;
      const next = clampComposePaneWidths({
        totalWidth,
        rightPaneWidth: startRightPaneWidth - delta,
        aiPaneWidth: startAIPaneWidth,
        aiOpen: aiAssistEnabled && isAIPanelOpen,
      });
      setRightPaneWidth(next.rightPaneWidth);
      setAIPaneWidth(next.aiPaneWidth);
    }

    function onMouseUp() {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
    }

    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
  }, [rightPaneWidth, aiPaneWidth, getComposeBodyWidth, aiAssistEnabled, isAIPanelOpen]);

  const handleSidebarAIResizeStart = useCallback((event: React.MouseEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startRightPaneWidth = rightPaneWidth;
    const startAIPaneWidth = aiPaneWidth;

    function onMouseMove(moveEvent: MouseEvent) {
      const totalWidth = getComposeBodyWidth();
      if (!totalWidth) return;
      const delta = moveEvent.clientX - startX;
      const next = clampComposePaneWidths({
        totalWidth,
        rightPaneWidth: startRightPaneWidth + delta,
        aiPaneWidth: startAIPaneWidth - delta,
        aiOpen: true,
      });
      setRightPaneWidth(next.rightPaneWidth);
      setAIPaneWidth(next.aiPaneWidth);
    }

    function onMouseUp() {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
    }

    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
  }, [rightPaneWidth, aiPaneWidth, getComposeBodyWidth]);

  // Keyboard shortcuts
  useEffect(() => {
    if (!open) return;
    function handleKeyDown(e: KeyboardEvent) {
      // ⌘↵ / Ctrl↵ → trigger primary action
      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        if (form.canSubmit) handleSubmit();
      }
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open, form.canSubmit]); // eslint-disable-line react-hooks/exhaustive-deps

  // Strictest TikTok video-length cap across currently-selected TikTok
  // accounts. Null when no TikTok account is selected or creator_info
  // hasn't resolved yet. Used to gate R2 uploads pre-hoc and to flag
  // already-uploaded videos that fall foul of a later-added account.
  const strictestTiktokMaxSec = useMemo(() => {
    const caps: number[] = [];
    for (const id of form.selectedAccountIds) {
      const cap = tiktokMaxByAccount[id];
      if (typeof cap === "number" && cap > 0) caps.push(cap);
    }
    return caps.length ? Math.min(...caps) : null;
  }, [form.selectedAccountIds, tiktokMaxByAccount]);

  // Uploaded or in-flight videos that exceed the current cap. Rendered
  // in a banner below MEDIA so the error doesn't try to fit inside the
  // narrow TikTok panel. This is the single source of truth for the
  // UX — the TikTok panel no longer duplicates it.
  const oversizeVideos = useMemo(() => {
    if (!strictestTiktokMaxSec) return [];
    return form.mediaItems.filter(
      (m) => typeof m.durationSec === "number" && m.durationSec > strictestTiktokMaxSec
    );
  }, [form.mediaItems, strictestTiktokMaxSec]);

  // Compute SHA-256 hash of a file
  async function hashFile(file: File): Promise<string> {
    const buffer = await file.arrayBuffer();
    const hashBuffer = await crypto.subtle.digest("SHA-256", buffer);
    return Array.from(new Uint8Array(hashBuffer))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
  }

  async function handleFileUpload(file: File) {
    const { cached, fingerprint } = form.addMediaItem(file);
    if (cached) return;
    try {
      // Always measure video metadata — the TikTok panel, the publish
      // blocker, the MEDIA-level oversize banner all key off duration,
      // and the Facebook placement guidance keys off width/height. One
      // decode pass yields all three.
      if (file.type.startsWith("video/")) {
        form.updateMediaItem(fingerprint, { progress: 5 });
        const meta = await measureVideoMetadata(file);
        form.updateMediaItem(fingerprint, {
          durationSec: meta.durationSec,
          videoWidth: meta.width,
          videoHeight: meta.height,
        });
        // Pre-R2 TikTok duration gate — keeps oversize videos out of
        // object storage entirely. Measured before the first R2 byte.
        if (
          strictestTiktokMaxSec &&
          typeof meta.durationSec === "number" &&
          meta.durationSec > strictestTiktokMaxSec
        ) {
          form.updateMediaItem(fingerprint, {
            error: "TIKTOK_VIDEO_TOO_LONG",
            progress: 0,
          });
          return;
        }
      } else {
        form.updateMediaItem(fingerprint, {
          durationSec: null,
          videoWidth: null,
          videoHeight: null,
        });
      }

      const token = await getToken();
      if (!token) return;
      form.updateMediaItem(fingerprint, { progress: 5 });
      const contentHash = await hashFile(file);
      form.updateMediaItem(fingerprint, { progress: 10 });
      const res = await createMedia(token, {
        filename: file.name,
        content_type: file.type || "application/octet-stream",
        size_bytes: file.size,
        content_hash: contentHash,
      });
      // Dedup hit — file already in R2
      if (res.data.status === "uploaded") {
        form.updateMediaItem(fingerprint, { progress: 100, mediaId: res.data.id });
        return;
      }
      // New file — PUT to R2
      form.updateMediaItem(fingerprint, { progress: 30 });
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open("PUT", res.data.upload_url);
        xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const pct = Math.round(30 + (e.loaded / e.total) * 65);
            form.updateMediaItem(fingerprint, { progress: pct });
          }
        };
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) resolve();
          else reject(new Error(`Upload failed: ${xhr.status}`));
        };
        xhr.onerror = () => reject(new Error("Upload failed"));
        xhr.send(file);
      });
      // Trigger server-side hydration: HEAD the R2 object, flip status
      // from 'pending' to 'uploaded' so the publish validator accepts it.
      await getMedia(token, res.data.id);
      form.updateMediaItem(fingerprint, { progress: 100, mediaId: res.data.id });
    } catch (err) {
      console.error("Media upload failed:", err);
      form.updateMediaItem(fingerprint, { error: (err as Error).message, progress: 0 });
    }
  }

  async function runValidation(payload: CreateSocialPostPayload) {
    const token = await getToken();
    if (!token) return { ok: false as const, tokenMissing: true as const };

    setIsValidating(true);
    try {
      const res = await validateSocialPost(token, payload);
      const result = res.data;
      setValidationResult(result);
      setValidationChecked(true);
      setSubmitError(null);
      return { ok: result.errors.length === 0, result, token };
    } catch (err) {
      const message = err instanceof Error ? err.message : "Validation failed";
      setSubmitError({
        message,
        mailto: buildSupportMailto({
          subject: "Validation failed in dashboard",
          intro: "A validation request failed in the dashboard create post drawer.",
          details: [
            `Workspace ID: ${workspaceId}`,
            profileName ? `Profile: ${profileName}` : undefined,
            `Publish mode: ${form.publishMode}`,
            `Selected accounts: ${form.selectedAccountIds.size}`,
            `Page: ${typeof window !== "undefined" ? window.location.pathname : "/posts"}`,
            `Error: ${message}`,
          ],
        }),
        contactHref: buildContactPageHref({
          topic: "validation-failure",
          source: "create-post-drawer",
          workspace: workspaceId,
          profile: profileName,
          error: message,
        }),
      });
      return { ok: false as const, tokenMissing: false as const };
    } finally {
      setIsValidating(false);
    }
  }

  function focusIssue(issue: SocialPostValidationIssue) {
    if (issue.account_id) {
      form.expandBlock(issue.account_id);
      window.requestAnimationFrame(() => {
        const node = platformBlockRefs.current[issue.account_id!];
        node?.scrollIntoView({ behavior: "smooth", block: "center" });
      });
      return;
    }
    if (issue.field === "media_ids" || issue.field === "media_urls") {
      mediaSectionRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
      return;
    }
    if (issue.field === "scheduled_at") {
      publishPanelRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
      return;
    }
    mainContentRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
    mainContentRef.current?.focus();
  }

  async function handleSubmit() {
    if (!form.canSubmit) return;
    try {
      setSubmitError(null);
      const payload = form.buildPayload();
      const validation = await runValidation(payload);
      if (!validation.ok) {
        if (!validation.tokenMissing && validation.result && validation.result.errors.length > 0) {
          focusIssue(validation.result.errors[0]);
        }
        return;
      }
      if (
        validation.result &&
        validation.result.warnings.length > 0 &&
        !warningsAcknowledged
      ) {
        setWarningsAcknowledged(true);
        focusIssue(validation.result.warnings[0]);
        return;
      }

      form.setSubmitting(true);
      const token = validation.token;
      if (!token) return;
      const response = await createSocialPost(token, payload);
      await onCreated(response.data.id);
      // TikTok processes video/photo uploads asynchronously — the
      // Content Posting API audit requires us to tell the user the post
      // is in-flight, not silently assume "published". Hold the drawer
      // open briefly with a success banner when any selected account is
      // on TikTok; the posts list (which `onCreated` just refreshed)
      // shows the per-platform status after the drawer closes.
      const postingToTikTok = form.selectedAccounts.some((a) => a.platform === "tiktok");
      if (postingToTikTok && form.publishMode === "now") {
        // Pull the TikTok permalink from the create response so the
        // success banner can link the user straight to their video
        // (per PRD Fix 4 §5.3). The adapter returns "https://www.tiktok.com"
        // when it only has a publish_id — skip the link in that case
        // rather than send the user to the TikTok homepage.
        const tiktokResult = response.data.results?.find((r) => r.platform === "tiktok");
        const tiktokURL =
          tiktokResult?.url && tiktokResult.url !== "https://www.tiktok.com"
            ? tiktokResult.url
            : undefined;
        setSubmitSuccess({
          message: tiktokURL
            ? "Posted! Your video should appear on your TikTok profile within a few minutes."
            : "Posted! TikTok is still processing your video — it should appear on your profile within a few minutes.",
          tiktokURL,
        });
        setTimeout(() => {
          setSubmitSuccess(null);
          onOpenChange(false);
        }, 3500);
      } else {
        onOpenChange(false);
      }
    } catch (err) {
      // Rate-limit errors get a friendlier, action-oriented message.
      // Non-429 errors fall through to the raw API message + the
      // existing support links so customers can escalate real bugs.
      const friendly = friendlyRateLimitMessage(err);
      const message = friendly ?? (err instanceof Error ? err.message : "Failed to create post");
      console.error("Create post failed:", err);
      console.error("[CreatePost] payload was:", JSON.stringify(form.buildPayload(), null, 2));
      setSubmitError({
        message,
        mailto: buildSupportMailto({
          subject: "Publish failed in dashboard",
          intro: "A publish action failed in the dashboard create post drawer.",
          details: [
            `Workspace ID: ${workspaceId}`,
            profileName ? `Profile: ${profileName}` : undefined,
            `Publish mode: ${form.publishMode}`,
            `Selected accounts: ${form.selectedAccountIds.size}`,
            `Page: ${typeof window !== "undefined" ? window.location.pathname : "/posts"}`,
            `Error: ${message}`,
          ],
        }),
        contactHref: buildContactPageHref({
          topic: "publish-failure",
          source: "create-post-drawer",
          workspace: workspaceId,
          profile: profileName,
          error: message,
        }),
      });
    } finally {
      form.setSubmitting(false);
    }
  }

  async function handleSaveDraft() {
    form.setSubmitting(true);
    try {
      setSubmitError(null);
      const token = await getToken();
      if (!token) return;
      const payload = form.buildPayload();
      payload.status = "draft";
      const response = await createSocialPost(token, payload);
      await onCreated(response.data.id);
      onOpenChange(false);
    } catch (err) {
      const friendly = friendlyRateLimitMessage(err);
      const message = friendly ?? (err instanceof Error ? err.message : "Failed to save draft");
      console.error("Save draft failed:", err);
      setSubmitError({
        message,
        mailto: buildSupportMailto({
          subject: "Save draft failed in dashboard",
          intro: "A draft save action failed in the dashboard create post drawer.",
          details: [
            `Workspace ID: ${workspaceId}`,
            profileName ? `Profile: ${profileName}` : undefined,
            `Publish mode: ${form.publishMode}`,
            `Selected accounts: ${form.selectedAccountIds.size}`,
            `Page: ${typeof window !== "undefined" ? window.location.pathname : "/posts"}`,
            `Error: ${message}`,
          ],
        }),
        contactHref: buildContactPageHref({
          topic: "save-draft-failure",
          source: "create-post-drawer",
          workspace: workspaceId,
          profile: profileName,
          error: message,
        }),
      });
    } finally {
      form.setSubmitting(false);
    }
  }

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) {
        attemptClose();
      } else {
        onOpenChange(true);
      }
    },
    [attemptClose, onOpenChange]
  );

  const primaryLabel = PRIMARY_BUTTON_LABELS[form.publishMode];

  // Classify the current media selection once per change so the TikTok
  // fields component can hide Duet/Stitch toggles for photo carousels
  // (per the Content Posting API audit requirements) without each child
  // re-deriving the same thing.
  const mediaKind: "video" | "photo" | "none" = useMemo(() => {
    if (form.mediaItems.length === 0) return "none";
    const hasVideo = form.mediaItems.some((m) => m.file.type.startsWith("video/"));
    return hasVideo ? "video" : "photo";
  }, [form.mediaItems]);

  // Hand the TikTok fields the first uploaded video file so they can
  // measure duration client-side. We only care about the first — TikTok
  // videos are single-file anyway, so there's no ambiguity here.
  const primaryVideoFile = useMemo<File | null>(() => {
    const v = form.mediaItems.find((m) => m.file.type.startsWith("video/"));
    return v ? v.file : null;
  }, [form.mediaItems]);

  // The Facebook placement guidance keys off the same primary video,
  // but reads measured dimensions / duration from the MediaItem rather
  // than the raw File. Pulled from the same item so the UI never shows
  // "Reel video, 1080×1920" alongside "Feed video, 720×1280" — there
  // is one video, and one set of measurements.
  const primaryVideoMeta = useMemo(() => {
    const v = form.mediaItems.find((m) => m.file.type.startsWith("video/"));
    if (!v) return null;
    return {
      width: v.videoWidth ?? null,
      height: v.videoHeight ?? null,
      durationSec: v.durationSec ?? null,
    };
  }, [form.mediaItems]);

  // When a TikTok account is selected, the in-flight publish label +
  // the processing notice below both key off this flag.
  const publishingToTikTok = useMemo(
    () => form.selectedAccounts.some((a) => a.platform === "tiktok"),
    [form.selectedAccounts]
  );

  // Why is the primary button disabled? Surface the first blocking reason
  // as a tooltip + inline hint — otherwise the grayed-out button looks
  // like a bug (especially when uploads are silently in flight).
  const disabledReason = useMemo(() => {
    if (form.submitting) return null;
    if (form.selectedAccountIds.size === 0) return "Select at least one account to post to.";
    const uploading = form.mediaItems.filter((m) => m.mediaId === null && !m.error).length;
    if (uploading > 0) return `Waiting for ${uploading} media upload${uploading === 1 ? "" : "s"} to finish…`;
    // Oversize-for-TikTok items are flagged separately so the tooltip
    // matches the MEDIA-section banner instead of the generic "upload
    // failed" copy (which wrongly implies Retry would help).
    if (oversizeVideos.length > 0) {
      return "Video is too long for the selected TikTok account — remove or replace it, or unselect TikTok.";
    }
    const failed = form.mediaItems.filter((m) => m.error && m.error !== "TIKTOK_VIDEO_TOO_LONG").length;
    if (failed > 0) return `${failed} media upload${failed === 1 ? "" : "s"} failed — retry or remove.`;
    if (form.hasOverLimit) return "One of your captions is over its platform limit.";
    // "hasContent" here has to agree with the hook's canSubmit logic,
    // which treats platform-only inputs (YouTube title, Facebook link)
    // as valid post bodies even without a main caption or media.
    const hasContent =
      form.mainContent.trim() ||
      Object.values(form.overrides).some(
        (o) => o.caption?.trim() || o.youtube?.title?.trim() || o.facebook?.link?.trim()
      ) ||
      form.totalMediaCount > 0;
    if (!hasContent) return "Add caption text or media to publish.";
    if (form.publishMode === "schedule" && !form.scheduledAt) return "Pick a time to schedule this post.";
    if (form.publishMode === "schedule" && form.scheduledAt && new Date(form.scheduledAt) <= new Date())
      return "Scheduled time must be in the future.";
    if (form.publishMode === "queue" && !form.queueId) return "Pick a queue to add this post to.";
    // TikTok audit guardrails — block publish until the creator has made
    // explicit choices that satisfy TikTok's Content Posting API UX rules.
    if (form.tiktokBlocker === "tiktok_privacy")
      return "Select a TikTok visibility (TikTok requires an explicit choice).";
    if (form.tiktokBlocker === "tiktok_disclosure")
      return "You need to indicate if your content promotes yourself, a third party, or both.";
    if (form.tiktokBlocker === "tiktok_branded_private")
      return "TikTok doesn't allow Branded Content to be posted as Only me — change the visibility or turn off Branded Content.";
    // Runtime blockers reported by per-account TikTok panels (creator
    // error from creator_info, uploaded video too long, etc.).
    const runtimeBlocker = Object.values(tiktokBlockers).find((v) => v);
    if (runtimeBlocker) return runtimeBlocker;
    return null;
  }, [
    form.submitting,
    form.selectedAccountIds,
    form.mediaItems,
    form.hasOverLimit,
    form.mainContent,
    form.overrides,
    form.publishMode,
    form.scheduledAt,
    form.queueId,
    form.tiktokBlocker,
    form.totalMediaCount,
    tiktokBlockers,
    oversizeVideos.length,
  ]);

  const handleGenerateAISuggestion = useCallback(async () => {
    if (!aiAssistEnabled) return;
    if (!canGenerateAIAssist({
      mode: aiMode,
      mainContent: form.mainContent,
      brief: aiBrief,
      mediaCount: form.totalMediaCount,
    })) return;
    if (!aiMode) return;
    setAILoading(true);
    setAIError(null);
    try {
      const token = await getToken();
      if (!token) {
        setAIError("You need to be signed in to use AI assist.");
        return;
      }
      const res = await postAssistAIDraft(token, buildAIPostAssistRequest({
        mode: aiMode,
        selectedProfileId,
        mainContent: form.mainContent,
        selectedAccounts: form.uniqueSelectedAccounts,
        overrides: form.overrides,
        mediaItems: form.mediaItems,
        validationResult,
        brief: aiBrief,
        objective: aiObjective,
        tone: aiTone,
        includeCTA: aiIncludeCTA,
      }));
      setAISuggestion(res.data);
    } catch (err) {
      setAIError(err instanceof Error ? err.message : "Failed to generate AI suggestion");
    } finally {
      setAILoading(false);
    }
  }, [aiAssistEnabled, aiMode, aiBrief, aiObjective, aiTone, aiIncludeCTA, form.mainContent, form.mediaItems, form.totalMediaCount, form.selectedAccountIds, form.uniqueSelectedAccounts, form.overrides, getToken, selectedProfileId, validationResult]);

  const handleApplyAISuggestion = useCallback(() => {
    if (!aiSuggestion?.main_caption) return;
    form.setMainContent(aiSuggestion.main_caption);
  }, [aiSuggestion, form]);

  const handleApplyPlatformAISuggestion = useCallback((accountId: string, caption: string) => {
    form.updateOverrideCaption(accountId, caption);
    form.expandBlock(accountId);
  }, [form]);

  const handleApplyAIFirstCommentSuggestion = useCallback((accountId: string, text: string) => {
    form.updateOverrideFirstComment(accountId, text);
    form.expandBlock(accountId);
  }, [form]);

  const handleApplyAllPlatformAISuggestions = useCallback(() => {
    if (!aiSuggestion?.platform_captions?.length) return;
    for (const item of aiSuggestion.platform_captions) {
      form.updateOverrideCaption(item.account_id, item.caption);
      form.expandBlock(item.account_id);
    }
  }, [aiSuggestion, form]);

  const handleApplyAllAIFirstCommentSuggestions = useCallback(() => {
    if (!aiSuggestion?.first_comment_suggestions?.length) return;
    for (const item of aiSuggestion.first_comment_suggestions) {
      const account = form.uniqueSelectedAccounts.find((candidate) => candidate.id === item.account_id);
      if (!account || !supportsFirstComment(account.platform, platformCapabilities)) continue;
      form.updateOverrideFirstComment(item.account_id, item.text);
      form.expandBlock(item.account_id);
    }
  }, [aiSuggestion, form, platformCapabilities]);

  const aiAccountLabels = useMemo(() => {
    return buildAIAssistAccountLabels(form.uniqueSelectedAccounts);
  }, [form.uniqueSelectedAccounts]);

  const aiAccountPlatforms = useMemo(() => {
    const platforms: Record<string, string> = {};
    for (const account of form.uniqueSelectedAccounts) {
      platforms[account.id] = account.platform;
    }
    return platforms;
  }, [form.uniqueSelectedAccounts]);

  const aiFirstCommentSupport = useMemo(() => {
    const support: Record<string, boolean> = {};
    for (const account of form.uniqueSelectedAccounts) {
      support[account.id] = supportsFirstComment(account.platform, platformCapabilities);
    }
    return support;
  }, [form.uniqueSelectedAccounts, platformCapabilities]);

  const aiCurrentPlatformCaptions = useMemo(() => {
    return buildAIAssistCurrentPlatformCaptions({
      accounts: form.uniqueSelectedAccounts,
      overrides: form.overrides,
      mainContent: form.mainContent,
    });
  }, [form.uniqueSelectedAccounts, form.overrides, form.mainContent]);

  const aiCurrentFirstComments = useMemo(() => {
    return buildAIAssistCurrentFirstComments({
      accounts: form.uniqueSelectedAccounts,
      overrides: form.overrides,
    });
  }, [form.uniqueSelectedAccounts, form.overrides]);

  return (
    <Sheet open={open} onOpenChange={handleOpenChange} modal>
      <SheetContent
        showCloseButton={false}
        className="border-l"
        style={{ width: drawerWidth, maxWidth: "calc(100vw - 32px)", background: "color-mix(in srgb, var(--surface-raised) 94%, black)", borderLeftColor: "color-mix(in srgb, var(--dborder2) 80%, var(--dborder))" }}
      >
        <div
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize create post drawer"
          onMouseDown={handleResizeStart}
          className="absolute left-0 top-0 z-10 h-full w-1.5 -translate-x-1/2 cursor-col-resize"
          style={{ background: isDraggingWidthRef.current ? "var(--primary)" : "transparent" }}
          onMouseEnter={(e) => { if (!isDraggingWidthRef.current) e.currentTarget.style.background = "color-mix(in srgb, var(--primary) 45%, transparent)"; }}
          onMouseLeave={(e) => { if (!isDraggingWidthRef.current) e.currentTarget.style.background = "transparent"; }}
        />
        {/* Header */}
        <header className="flex flex-shrink-0 items-start justify-between border-b px-8 pb-5 pt-7" style={{ borderBottomColor: "color-mix(in srgb, var(--dborder2) 78%, var(--dborder))" }}>
          <div>
            <h2 className="mb-2 font-serif text-[2.15rem] leading-[1.02] tracking-[-0.035em]" style={{ color: "var(--dtext)", fontWeight: 650 }}>
              Create post
            </h2>
            <p className="compose-support-text text-[14.5px] leading-[1.65]">
              Compose once, publish to any platform you&apos;ve connected.
            </p>
          </div>
          <div className="flex items-center gap-2.5">
            {aiAssistEnabled ? (
              <button
                type="button"
                onClick={() => setIsAIPanelOpen((openNow) => !openNow)}
                className="inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-[12.5px] font-medium transition-colors"
                style={{
                  color: isAIPanelOpen ? "var(--primary)" : "var(--dtext)",
                  borderColor: isAIPanelOpen ? "color-mix(in srgb, var(--primary) 50%, transparent)" : "color-mix(in srgb, var(--dborder2) 78%, var(--dborder))",
                  background: isAIPanelOpen ? "color-mix(in srgb, var(--primary) 10%, var(--surface-raised))" : "color-mix(in srgb, var(--surface1) 96%, var(--surface2))",
                }}
              >
                {isAIPanelOpen ? <PanelRightClose className="h-3.5 w-3.5" /> : <PanelRightOpen className="h-3.5 w-3.5" />}
                <Sparkles className="h-3.5 w-3.5" />
                AI Assist
              </button>
            ) : null}
            <button
              type="button"
              onClick={attemptClose}
              className="flex h-9 w-9 items-center justify-center rounded-lg transition-colors"
              style={{ color: "var(--dmuted)" }}
            >
              <svg width="16" height="16" viewBox="0 0 15 15" fill="none">
                <path d="M11.25 3.75l-7.5 7.5M3.75 3.75l7.5 7.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>
          </div>
        </header>

        {/* Body: default two columns, expands to three when AI assist is active */}
        <div ref={bodyLayoutRef} className="flex min-h-0 flex-1">
          {/* LEFT: Content + per-platform editors */}
          <div
            className="min-w-0 flex-1 overflow-y-auto px-8 py-7"
            style={{ minWidth: COMPOSE_MIN_LEFT_PANE_WIDTH }}
          >
            {/* Main content */}
            <section>
              <div className="flex items-center justify-between mb-2.5">
                <label className="compose-panel-label text-[11px] font-semibold uppercase tracking-[0.11em]">
                  Content
                </label>
                <span className="compose-meta-text font-mono text-[10.5px] tracking-[0.02em]">optional</span>
              </div>
              <textarea
                ref={mainContentRef}
                rows={5}
                placeholder="What's on your mind?"
                value={form.mainContent}
                onChange={(e) => form.setMainContent(e.target.value)}
                autoFocus
                className="compose-field w-full resize-none rounded-lg border px-4 py-3 text-sm leading-relaxed outline-none transition-[border-color,box-shadow] duration-[140ms]"
                style={{ color: "var(--dtext)" }}
              />
              <div className="flex items-center justify-between mt-2">
                <p className="compose-support-text text-[12.5px] leading-[1.55]">
                  Used as the default for every selected platform unless overridden below.
                </p>
                <span className="compose-meta-text font-mono text-[10.5px] tracking-[0.02em]">
                  {form.mainContent.length} chars
                </span>
              </div>
            </section>

            {/* Media upload */}
            <div ref={mediaSectionRef}>
              <MediaThumbnails
                existingItems={form.existingMediaItems}
                items={form.mediaItems}
                onRemoveExisting={(i) => form.removeExistingMediaItem(i)}
                onRemove={(i) => form.removeMediaItem(i)}
                onAdd={(newFiles) => newFiles.forEach((f) => handleFileUpload(f))}
                onPreview={(file) => setPreviewFile(file)}
                onRetry={(i) => {
                  const failed = form.mediaItems[i];
                  if (!failed) return;
                  form.removeMediaItem(i);
                  handleFileUpload(failed.file);
                }}
                strictestTiktokMaxSec={strictestTiktokMaxSec}
              />
            </div>

            {/* Per-platform overrides */}
            <section className="mt-8">
              <div className="flex items-center justify-between mb-3">
                <label className="compose-panel-label text-[11px] font-semibold uppercase tracking-[0.11em]">
                  Per-platform customization
                </label>
                <span className="compose-meta-text font-mono text-[10.5px] tracking-[0.02em]">
                  {form.selectedAccountIds.size} selected
                </span>
              </div>

              {form.uniqueSelectedAccounts.length === 0 ? (
                <EmptyPlatformState />
              ) : (
                <div className="space-y-3">
                  {form.uniqueSelectedAccounts.map((account, i) => {
                    const override = form.overrides[account.id] || { caption: "" };
                    const text = override.caption || form.mainContent;
                    const charCount = form.getCharCount(text, account.platform);
                    const accountIssues = [
                      ...(validationResult?.errors || []),
                      ...(validationResult?.warnings || []),
                    ].filter((issue) => issue.account_id === account.id);
                    return (
                      <div
                        key={account.id}
                        ref={(node) => {
                          platformBlockRefs.current[account.id] = node;
                        }}
                      >
                        <PlatformEditorBlock
                          account={account}
                          index={i}
                          override={override}
                          collapsed={form.collapsedBlocks.has(account.id)}
                          charCount={charCount}
                          captionLimit={getPlatformCaptionLimit(account.platform, charCount.limit, platformCapabilities)}
                          issues={accountIssues}
                          mediaKind={mediaKind}
                          mediaFile={primaryVideoFile}
                          videoMetadata={primaryVideoMeta}
                          getToken={getToken}
                          profileId={account.profile_id || selectedProfileId}
                          onTiktokBlockerChange={(reason) => setTiktokBlocker(account.id, reason)}
                          onTiktokMaxDurationChange={(sec) => setTiktokMaxDuration(account.id, sec)}
                          onCaptionChange={(caption) =>
                            form.updateOverrideCaption(account.id, caption)
                          }
                          onFirstCommentChange={(firstComment) =>
                            form.updateOverrideFirstComment(account.id, firstComment)
                          }
                          firstCommentSupported={aiFirstCommentSupport[account.id] ?? supportsFirstComment(account.platform, platformCapabilities)}
                          firstCommentMaxLength={getFirstCommentMaxLength(account.platform, platformCapabilities)}
                          threadSupported={supportsThreads(account.platform, platformCapabilities)}
                          onThreadFieldsChange={(fields) =>
                            form.updateOverrideThreadFields(account.id, fields)
                          }
                          onAddThreadReply={() => form.addOverrideThreadReply(account.id)}
                          onUpdateThreadReply={(replyIndex, value) =>
                            form.updateOverrideThreadReply(account.id, replyIndex, value)
                          }
                          onRemoveThreadReply={(replyIndex) =>
                            form.removeOverrideThreadReply(account.id, replyIndex)
                          }
                          onPlatformFieldChange={(platform, fields) =>
                            form.updateOverridePlatformField(account.id, platform, fields)
                          }
                          onToggleCollapse={() => form.toggleBlockCollapse(account.id)}
                        />
                      </div>
                    );
                  })}
                </div>
              )}
            </section>
          </div>

          <ColumnResizeHandle
            label="Resize content and composer sidebar panels"
            onMouseDown={handleContentSidebarResizeStart}
          />

          {/* RIGHT: Profile + Connected Accounts + Post To + Publish */}
          <aside
            className="shrink-0 overflow-y-auto px-6 py-7"
            style={{
              width: rightPaneWidth,
              minWidth: COMPOSE_MIN_RIGHT_PANE_WIDTH,
              background: "color-mix(in srgb, var(--surface2) 58%, transparent)",
            }}
          >

            {/* 1. Profile selector */}
            {profiles.length > 0 && (
              <div className="mb-5">
                <label className="compose-panel-label mb-2 block text-[11px] font-semibold uppercase tracking-[0.11em]">
                  Profile
                </label>
                <div className="relative">
                  <select
                    value={selectedProfileId}
                    onChange={(e) => setSelectedProfileId(e.target.value)}
                    className="compose-field w-full appearance-none rounded-lg border px-3 py-2.5 pr-8 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
                    style={{ background: "var(--surface2)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
                  >
                    {profiles.map((p) => (
                      <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                  </select>
                  <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2" style={{ color: "color-mix(in srgb, var(--dtext) 58%, transparent)" }} />
                </div>
              </div>
            )}

            {/* 2. Connected Accounts */}
            <div className="mb-5">
                <label className="mb-2 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                  Connected accounts
                </label>
              <ConnectedAccountsGrid
                accounts={profileAccounts.filter((account) => account.status === "active")}
                selectedIds={form.selectedAccountIds}
                onToggle={form.toggleAccount}
              />
            </div>

            {/* 3. Post To */}
            <div className="mb-5">
              <div className="flex items-center justify-between mb-2">
                <label className="text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                  Post to
                </label>
                <span className="font-mono text-[10.5px] tracking-[0.02em]" style={{ color: "var(--dmuted2)" }}>
                  {form.selectedAccountIds.size} selected
                </span>
              </div>
              <PostToGrid
                accounts={form.activeAccounts}
                selectedIds={form.selectedAccountIds}
                duplicateIds={form.duplicateAccountIds}
                onRemove={form.toggleAccount}
              />
            </div>

            {/* Divider */}
            <div className="my-5 border-t" style={{ borderTopColor: "var(--dborder)" }} />

            <ValidationPanel
              errors={validationResult?.errors || []}
              warnings={validationResult?.warnings || []}
              accounts={form.uniqueSelectedAccounts}
              onSelectIssue={focusIssue}
            />
            {form.submitting && publishingToTikTok && (
              <section
                className="mb-5 rounded-xl border px-4 py-3.5"
                style={{
                  background: "color-mix(in srgb, var(--primary) 8%, var(--surface-raised))",
                  borderColor: "color-mix(in srgb, var(--primary) 35%, transparent)",
                }}
              >
                <div className="flex items-center gap-2">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" style={{ color: "var(--primary)" }} />
                  <div
                    className="font-mono text-[11px] uppercase tracking-[0.12em]"
                    style={{ color: "color-mix(in srgb, var(--primary) 30%, white)" }}
                  >
                    Publishing to TikTok
                  </div>
                </div>
                <p className="mt-1 text-[13px] leading-relaxed" style={{ color: "var(--dtext)" }}>
                  Your content is being processed. It may take a few minutes to appear on your TikTok profile.
                </p>
              </section>
            )}
            {submitSuccess && (
              <section
                className="mb-5 rounded-xl border px-4 py-3.5"
                style={{
                  background: "color-mix(in srgb, var(--primary) 10%, var(--surface-raised))",
                  borderColor: "color-mix(in srgb, var(--primary) 45%, transparent)",
                }}
              >
                <div
                  className="mb-1 font-mono text-[11px] uppercase tracking-[0.12em]"
                  style={{ color: "color-mix(in srgb, var(--primary) 30%, white)" }}
                >
                  Post submitted
                </div>
                <p className="text-[13px] leading-relaxed" style={{ color: "var(--dtext)" }}>
                  {submitSuccess.message}
                </p>
                {submitSuccess.tiktokURL && (
                  <a
                    href={submitSuccess.tiktokURL}
                    target="_blank"
                    rel="noreferrer"
                    className="mt-2 inline-block text-[12px] underline"
                    style={{ color: "var(--primary)" }}
                  >
                    View on TikTok
                  </a>
                )}
              </section>
            )}
            {submitError && (
              <section className="mb-5 rounded-xl border border-[#7f1d1d] bg-[#261013] px-4 py-3.5">
                <div className="flex items-center gap-2 text-[#fecaca] mb-2">
                  <AlertTriangle className="w-4 h-4" />
                  <div className="text-[12px] font-mono uppercase tracking-[0.12em]">
                    Action failed
                  </div>
                </div>
                <p className="text-[13px] text-[#fee2e2] leading-relaxed mb-3">
                  {submitError.message}
                </p>
                <div className="flex items-center gap-2">
                  <a
                    href={submitError.mailto}
                    className="inline-flex items-center rounded-lg border border-[#b91c1c] px-3 py-2 text-[12px] font-medium text-[#fee2e2] hover:bg-[#3b161b] transition-colors"
                  >
                    Contact support
                  </a>
                  <Link
                    href={submitError.contactHref}
                    className="inline-flex items-center rounded-lg border border-[#7f1d1d]/60 px-3 py-2 text-[12px] font-medium text-[#fca5a5] hover:border-[#b91c1c] transition-colors"
                  >
                    Open help center
                  </Link>
                </div>
              </section>
            )}

            {/* 4. Publish */}
            <CapabilitySummary
              selectedAccounts={form.uniqueSelectedAccounts}
              platformCapabilities={platformCapabilities}
            />
            <div ref={publishPanelRef}>
              <PublishModePanel
                mode={form.publishMode}
                onModeChange={form.setPublishMode}
                scheduledAt={form.scheduledAt}
                onScheduledAtChange={form.setScheduledAt}
                queueId={form.queueId}
                onQueueIdChange={form.setQueueId}
                queues={queues}
              />
            </div>
          </aside>
          {aiAssistEnabled && isAIPanelOpen ? (
            <>
              <ColumnResizeHandle
                label="Resize composer sidebar and AI assist panels"
                onMouseDown={handleSidebarAIResizeStart}
              />
              <div
                className="min-w-0 shrink-0"
                style={{ width: aiPaneWidth, minWidth: COMPOSE_MIN_AI_PANE_WIDTH }}
              >
                <AIAssistPanel
                  mode={aiMode}
                  onModeChange={setAIMode}
                  onGenerate={handleGenerateAISuggestion}
                  brief={aiBrief}
                  onBriefChange={setAIBrief}
                  objective={aiObjective}
                  onObjectiveChange={setAIObjective}
                  tone={aiTone}
                  onToneChange={setAITone}
                  includeCTA={aiIncludeCTA}
                  onIncludeCTAChange={setAIIncludeCTA}
                  onApplyMainCaption={handleApplyAISuggestion}
                  onApplyPlatformCaption={handleApplyPlatformAISuggestion}
                  onApplyAllPlatformCaptions={handleApplyAllPlatformAISuggestions}
                  onApplyFirstCommentSuggestion={handleApplyAIFirstCommentSuggestion}
                  onApplyAllFirstCommentSuggestions={handleApplyAllAIFirstCommentSuggestions}
                  selectedPlatformsCount={form.selectedAccountIds.size}
                  mediaCount={form.totalMediaCount}
                  hasMainContent={!!form.mainContent.trim()}
                  loading={aiLoading}
                  error={aiError}
                  suggestion={aiSuggestion}
                  accountLabels={aiAccountLabels}
                  accountPlatforms={aiAccountPlatforms}
                  platformCapabilities={platformCapabilities}
                  currentMainCaption={form.mainContent}
                  currentPlatformCaptions={aiCurrentPlatformCaptions}
                  currentFirstComments={aiCurrentFirstComments}
                />
              </div>
            </>
          ) : null}
        </div>

        {/* Footer */}
        <footer className="flex flex-shrink-0 items-center justify-between border-t px-8 py-4" style={{ borderTopColor: "var(--dborder)", background: "var(--surface-raised)" }}>
          <div className="flex items-center gap-2 font-mono text-[10.5px] tracking-[0.02em]" style={{ color: "var(--dmuted2)" }}>
            <kbd className="rounded border px-1.5 py-0.5" style={{ borderColor: "var(--dborder)", background: "var(--surface2)" }}>Esc</kbd>
            <span>to close</span>
            <span className="mx-1">&middot;</span>
            <kbd className="rounded border px-1.5 py-0.5" style={{ borderColor: "var(--dborder)", background: "var(--surface2)" }}>&#8984;</kbd>
            <kbd className="rounded border px-1.5 py-0.5" style={{ borderColor: "var(--dborder)", background: "var(--surface2)" }}>&#8629;</kbd>
            <span>to publish</span>
          </div>
          <div className="flex items-center gap-2.5">
            {disabledReason && (
              <span className="max-w-[260px] text-right text-[12px] leading-[1.45]" style={{ color: "var(--dmuted)" }}>
                {disabledReason}
              </span>
            )}
            <button
              type="button"
              onClick={attemptClose}
              className="rounded-lg px-4 py-2 text-sm transition-colors"
              style={{ color: "var(--dmuted)" }}
            >
              Cancel
            </button>
            {form.publishMode !== "draft" && (
              <button
                type="button"
                onClick={handleSaveDraft}
                disabled={form.submitting}
                className="rounded-lg border px-4 py-2 text-sm transition-colors disabled:opacity-50"
                style={{ color: "var(--dtext)", background: "var(--surface2)", borderColor: "var(--dborder)" }}
              >
                Save draft
              </button>
            )}
            <button
              type="button"
              onClick={handleSubmit}
              disabled={!form.canSubmit || isValidating || Object.keys(tiktokBlockers).length > 0 || oversizeVideos.length > 0}
              title={disabledReason ?? undefined}
              className={cn(
                "px-5 py-2 text-sm font-medium rounded-lg transition-colors",
                "shadow-[0_0_0_1px_rgba(16,185,129,0.4),0_8px_24px_-8px_rgba(16,185,129,0.4)]",
                "disabled:opacity-50 disabled:cursor-not-allowed"
              )}
              style={{ background: "var(--primary)", color: "var(--primary-foreground)" }}
            >
              {isValidating ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Checking...
                </span>
              ) : form.submitting ? (
                <span className="inline-flex items-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  {publishingToTikTok ? "Publishing to TikTok..." : "Sending..."}
                </span>
              ) : validationChecked && (validationResult?.warnings?.length || 0) > 0 && (validationResult?.errors?.length || 0) === 0 ? "Publish anyway" : primaryLabel}
            </button>
          </div>
        </footer>

        {/* Discard confirmation overlay */}
        {showDiscardConfirm && (
          <>
            <div
              className="fixed inset-0 z-[60] bg-black/50"
              onClick={() => setShowDiscardConfirm(false)}
            />
            <div className="fixed left-1/2 top-1/2 z-[61] w-[400px] -translate-x-1/2 -translate-y-1/2 rounded-xl border p-6 shadow-2xl" style={{ background: "var(--surface-raised)", borderColor: "var(--dborder)" }}>
              <h3 className="mb-2 text-base font-medium" style={{ color: "var(--dtext)" }}>
                Discard unsaved changes?
              </h3>
              <p className="mb-6 text-sm" style={{ color: "var(--dmuted)" }}>
                You have unsaved content that will be lost if you close this drawer.
              </p>
              <div className="flex justify-end gap-2.5">
                <button
                  type="button"
                  onClick={() => setShowDiscardConfirm(false)}
                  className="rounded-lg px-4 py-2 text-sm transition-colors"
                  style={{ color: "var(--dmuted)" }}
                >
                  Keep editing
                </button>
                <button
                  type="button"
                  onClick={confirmDiscard}
                  className="rounded-lg bg-red-500 px-4 py-2 text-sm font-medium text-white transition-colors"
                  style={{ background: "var(--danger)" }}
                >
                  Discard
                </button>
              </div>
            </div>
          </>
        )}
      </SheetContent>
      <MediaPreviewDialog
        file={previewFile}
        open={previewFile !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) setPreviewFile(null);
        }}
      />
    </Sheet>
  );
}
