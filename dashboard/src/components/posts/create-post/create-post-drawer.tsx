"use client";

import { useEffect, useCallback, useState, useRef, useMemo, memo } from "react";
import Link from "next/link";
import { AlertTriangle, Loader2, Plus } from "lucide-react";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { ConnectedAccountsGrid, PostToGrid } from "./account-card-grid";
import { PlatformEditorBlock } from "./platform-editor-block";
import { EmptyPlatformState } from "./empty-platform-state";
import { PublishModePanel } from "./publish-mode-panel";
import {
  useCreatePostForm,
  PRIMARY_BUTTON_LABELS,
  type MediaItem,
} from "./use-create-post-form";
import { ChevronDown } from "lucide-react";
import type { SocialAccount, Profile } from "@/lib/api";
import {
  createSocialPost,
  createMedia,
  getMedia,
  listProfiles,
  listSocialAccounts,
  validateSocialPost,
  type CreateSocialPostPayload,
  type SocialPostValidationIssue,
  type SocialPostValidationResult,
} from "@/lib/api";
import { cn } from "@/lib/utils";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";

const MIN_DRAWER_WIDTH = 880;
const MAX_DRAWER_WIDTH = 1440;

function clampDrawerWidth(width: number) {
  if (typeof window === "undefined") return width;
  const viewportMax = Math.max(MIN_DRAWER_WIDTH, window.innerWidth - 160);
  return Math.min(Math.max(width, MIN_DRAWER_WIDTH), Math.min(MAX_DRAWER_WIDTH, viewportMax));
}

// ── Stable-URL media thumbnail (prevents flicker on re-render) ──────

const MediaThumb = memo(function MediaThumb({ item, onRemove, onRetry }: {
  item: MediaItem;
  onRemove: () => void;
  onRetry?: () => void;
}) {
  const url = useMemo(() => URL.createObjectURL(item.file), [item.file]);
  useEffect(() => () => URL.revokeObjectURL(url), [url]);

  const uploading = item.mediaId === null && !item.error;
  const failed = !!item.error;
  const ready = item.mediaId !== null;

  return (
    <div
      className={cn(
        "relative h-[88px] w-[88px] flex-shrink-0 overflow-hidden rounded-lg border group/thumb"
      )}
      style={{
        background: "var(--surface1)",
        borderColor: failed ? "var(--danger)" : ready ? "var(--dborder2)" : "color-mix(in srgb, var(--primary) 60%, transparent)",
      }}
      title={
        failed
          ? `Upload failed: ${item.error}`
          : uploading
          ? `Uploading… ${item.progress}%`
          : item.file.name
      }
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
        onClick={onRemove}
        className="absolute top-1 right-1 w-5 h-5 rounded-full bg-black/70 text-white flex items-center justify-center opacity-0 group-hover/thumb:opacity-100 transition-opacity text-xs z-10"
      >
        &times;
      </button>
      <div className="absolute bottom-0 left-0 right-0 truncate bg-black/60 px-1.5 py-0.5 font-mono text-[9px]" style={{ color: "rgb(229 231 235)" }}>
        {item.file.name}
      </div>
    </div>
  );
});

function MediaThumbnails({ items, onRemove, onAdd, onRetry }: {
  items: MediaItem[];
  onRemove: (index: number) => void;
  onAdd: (files: File[]) => void;
  onRetry: (index: number) => void;
}) {
  return (
    <section className="mt-6">
      <label className="mb-2.5 block text-xs font-medium uppercase tracking-wider" style={{ color: "var(--dmuted2)" }}>
        Media
      </label>
      <div className="flex gap-2.5 flex-wrap">
        {items.map((item, i) => (
          <MediaThumb
            key={item.fingerprint}
            item={item}
            onRemove={() => onRemove(i)}
            onRetry={() => onRetry(i)}
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
            accept="image/png,image/jpeg,video/mp4"
            onChange={(e) => { if (e.target.files) { onAdd(Array.from(e.target.files)); e.target.value = ""; } }}
          />
        </label>
      </div>
    </section>
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
    const accountLabel = account?.account_name || account?.external_user_email || platformLabel;
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
        <div className="rounded-xl border px-4 py-3.5" style={{ borderColor: "color-mix(in srgb, var(--danger) 45%, transparent)", background: "color-mix(in srgb, var(--danger) 12%, var(--surface-raised))" }}>
          <div className="mb-2 flex items-center gap-2" style={{ color: "color-mix(in srgb, var(--danger) 26%, white)" }}>
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
                style={{ borderColor: "color-mix(in srgb, var(--danger) 35%, transparent)", background: "color-mix(in srgb, var(--danger) 16%, var(--surface-raised))" }}
              >
                <div className="mb-1 font-mono text-[11px] uppercase tracking-[0.12em]" style={{ color: "color-mix(in srgb, var(--danger) 45%, white)" }}>
                  {issueTargetLabel(issue, accounts)}
                </div>
                <div className="text-[13px] leading-relaxed" style={{ color: "color-mix(in srgb, var(--danger) 26%, white)" }}>{issueSummary(issue)}</div>
              </button>
            ))}
          </div>
        </div>
      )}

      {warnings.length > 0 && (
        <div className="rounded-xl border px-4 py-3.5" style={{ borderColor: "color-mix(in srgb, var(--warning) 45%, transparent)", background: "color-mix(in srgb, var(--warning) 12%, var(--surface-raised))" }}>
          <div className="mb-2 flex items-center gap-2" style={{ color: "color-mix(in srgb, var(--warning) 32%, white)" }}>
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
                style={{ borderColor: "color-mix(in srgb, var(--warning) 35%, transparent)", background: "color-mix(in srgb, var(--warning) 16%, var(--surface-raised))" }}
              >
                <div className="mb-1 font-mono text-[11px] uppercase tracking-[0.12em]" style={{ color: "color-mix(in srgb, var(--warning) 48%, white)" }}>
                  {issueTargetLabel(issue, accounts)}
                </div>
                <div className="text-[13px] leading-relaxed" style={{ color: "color-mix(in srgb, var(--warning) 28%, white)" }}>{issueSummary(issue)}</div>
              </button>
            ))}
          </div>
        </div>
      )}
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
  onCreated: () => void;
  // Activation guide: prefill caption + preselect all connected accounts
  // so a first-time user just clicks Publish.
  initialCaption?: string;
  preselectAllAccounts?: boolean;
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
}: CreatePostDrawerProps) {
  // Profile management
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [selectedProfileId, setSelectedProfileId] = useState("");
  const [profileAccounts, setProfileAccounts] = useState<SocialAccount[]>(accounts);

  const form = useCreatePostForm(profileAccounts);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);
  const [queues] = useState<Array<{ id: string; name: string }>>([]);
  const [queuesLoaded, setQueuesLoaded] = useState(false);
  const [isValidating, setIsValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<SocialPostValidationResult | null>(null);
  const [validationChecked, setValidationChecked] = useState(false);
  const [warningsAcknowledged, setWarningsAcknowledged] = useState(false);
  const [submitError, setSubmitError] = useState<{ message: string; mailto: string; contactHref: string } | null>(null);
  const [submitSuccess, setSubmitSuccess] = useState<{ message: string } | null>(null);
  const pendingCloseRef = useRef(false);
  const mainContentRef = useRef<HTMLTextAreaElement | null>(null);
  const mediaSectionRef = useRef<HTMLDivElement | null>(null);
  const publishPanelRef = useRef<HTMLDivElement | null>(null);
  const platformBlockRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const [drawerWidth, setDrawerWidth] = useState(() =>
    typeof window === "undefined" ? 1080 : clampDrawerWidth(window.innerWidth * 0.75)
  );
  const isDraggingWidthRef = useRef(false);

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

  // Apply activation-guide prefill: caption + preselect all connected
  // accounts when the drawer opens from ?action=new&template=welcome.
  // Runs only on open transition so subsequent edits aren't overwritten.
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
    if (preselectAllAccounts && profileAccounts.length > 0) {
      profileAccounts.forEach((a) => form.toggleAccount(a.id));
    }
    appliedPrefillRef.current = true;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialCaption, preselectAllAccounts, profileAccounts.length]);

  // Reset form when drawer closes
  useEffect(() => {
    if (!open) {
      form.reset();
      setSelectedProfileId("");
      setProfileAccounts(accounts);
      setShowDiscardConfirm(false);
      setQueuesLoaded(false);
      setValidationResult(null);
      setValidationChecked(false);
      setIsValidating(false);
      setWarningsAcknowledged(false);
      setSubmitError(null);
      setSubmitSuccess(null);
      pendingCloseRef.current = false;
    }
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

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
      const token = await getToken();
      if (!token) return;
      form.updateMediaItem(fingerprint, { progress: 5 });
      const contentHash = await hashFile(file);
      form.updateMediaItem(fingerprint, { progress: 10 });
      const res = await createMedia(token, workspaceId, {
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
      await getMedia(token, workspaceId, res.data.id);
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
      const res = await validateSocialPost(token, workspaceId, payload);
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
      await createSocialPost(token, workspaceId, payload);
      onCreated();
      // TikTok processes video/photo uploads asynchronously — the
      // Content Posting API audit requires us to tell the user the post
      // is in-flight, not silently assume "published". Hold the drawer
      // open briefly with a success banner when any selected account is
      // on TikTok; the posts list (which `onCreated` just refreshed)
      // shows the per-platform status after the drawer closes.
      const postingToTikTok = form.selectedAccounts.some((a) => a.platform === "tiktok");
      if (postingToTikTok && form.publishMode === "now") {
        setSubmitSuccess({
          message: "Posted! TikTok is processing your video — it should appear on your profile within a few minutes.",
        });
        setTimeout(() => {
          setSubmitSuccess(null);
          onOpenChange(false);
        }, 3500);
      } else {
        onOpenChange(false);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to create post";
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
      await createSocialPost(token, workspaceId, payload);
      onCreated();
      onOpenChange(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to save draft";
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

  // Why is the primary button disabled? Surface the first blocking reason
  // as a tooltip + inline hint — otherwise the grayed-out button looks
  // like a bug (especially when uploads are silently in flight).
  const disabledReason = useMemo(() => {
    if (form.submitting) return null;
    if (form.selectedAccountIds.size === 0) return "Select at least one account to post to.";
    const uploading = form.mediaItems.filter((m) => m.mediaId === null && !m.error).length;
    if (uploading > 0) return `Waiting for ${uploading} media upload${uploading === 1 ? "" : "s"} to finish…`;
    const failed = form.mediaItems.filter((m) => m.error).length;
    if (failed > 0) return `${failed} media upload${failed === 1 ? "" : "s"} failed — retry or remove.`;
    if (form.hasOverLimit) return "One of your captions is over its platform limit.";
    const hasContent =
      form.mainContent.trim() ||
      Object.values(form.overrides).some((o) => o.caption?.trim()) ||
      form.mediaItems.length > 0;
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
      return "Pick Your Brand or Branded Content to finish disclosing commercial content on TikTok.";
    if (form.tiktokBlocker === "tiktok_branded_private")
      return "TikTok doesn't allow Branded Content to be posted as Only me — change the visibility or turn off Branded Content.";
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
  ]);

  return (
    <Sheet open={open} onOpenChange={handleOpenChange} modal>
      <SheetContent
        showCloseButton={false}
        className="border-l"
        style={{ width: drawerWidth, maxWidth: "calc(100vw - 32px)", background: "var(--surface-raised)", borderLeftColor: "var(--dborder)" }}
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
        <header className="flex flex-shrink-0 items-start justify-between border-b px-8 pb-5 pt-7" style={{ borderBottomColor: "var(--dborder)" }}>
          <div>
            <h2 className="mb-2 font-serif text-[2.15rem] leading-[1.02] tracking-[-0.035em]" style={{ color: "var(--dtext)", fontWeight: 650 }}>
              Create post
            </h2>
            <p className="text-[14.5px] leading-[1.65]" style={{ color: "var(--dmuted)" }}>
              Compose once, publish to any platform you&apos;ve connected.
            </p>
          </div>
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
        </header>

        {/* Body: two columns */}
        <div className="flex-1 flex min-h-0">
          {/* LEFT: Content + per-platform editors (3:2 ratio) */}
          <div className="flex-[3] overflow-y-auto border-r px-8 py-7" style={{ borderRightColor: "var(--dborder)" }}>
            {/* Main content */}
            <section>
              <div className="flex items-center justify-between mb-2.5">
                <label className="text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                  Content
                </label>
                <span className="font-mono text-[10.5px] tracking-[0.02em]" style={{ color: "var(--dmuted2)" }}>optional</span>
              </div>
              <textarea
                ref={mainContentRef}
                rows={5}
                placeholder="What's on your mind?"
                value={form.mainContent}
                onChange={(e) => form.setMainContent(e.target.value)}
                autoFocus
                className="w-full resize-none rounded-lg border px-4 py-3 text-sm leading-relaxed outline-none transition-[border-color,box-shadow] duration-[140ms]"
                style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
              />
              <div className="flex items-center justify-between mt-2">
                <p className="text-[12.5px] leading-[1.55]" style={{ color: "var(--dmuted2)" }}>
                  Used as the default for every selected platform unless overridden below.
                </p>
                <span className="font-mono text-[10.5px] tracking-[0.02em]" style={{ color: "var(--dmuted2)" }}>
                  {form.mainContent.length} chars
                </span>
              </div>
            </section>

            {/* Media upload */}
            <div ref={mediaSectionRef}>
              <MediaThumbnails
                items={form.mediaItems}
                onRemove={(i) => form.removeMediaItem(i)}
                onAdd={(newFiles) => newFiles.forEach((f) => handleFileUpload(f))}
                onRetry={(i) => {
                  const failed = form.mediaItems[i];
                  if (!failed) return;
                  form.removeMediaItem(i);
                  handleFileUpload(failed.file);
                }}
              />
            </div>

            {/* Per-platform overrides */}
            <section className="mt-8">
              <div className="flex items-center justify-between mb-3">
                <label className="text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                  Per-platform customization
                </label>
                <span className="font-mono text-[10.5px] tracking-[0.02em]" style={{ color: "var(--dmuted2)" }}>
                  {form.selectedAccountIds.size} selected
                </span>
              </div>

              {form.selectedAccounts.length === 0 ? (
                <EmptyPlatformState />
              ) : (
                <div className="space-y-3">
                  {form.selectedAccounts.map((account, i) => {
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
                          issues={accountIssues}
                          mediaKind={mediaKind}
                          getToken={getToken}
                          profileId={account.profile_id || selectedProfileId}
                          onCaptionChange={(caption) =>
                            form.updateOverrideCaption(account.id, caption)
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

          {/* RIGHT: Profile + Connected Accounts + Post To + Publish */}
          <aside className="flex-[2] overflow-y-auto px-6 py-7" style={{ background: "color-mix(in srgb, var(--surface2) 45%, transparent)" }}>

            {/* 1. Profile selector */}
            {profiles.length > 0 && (
              <div className="mb-5">
                <label className="mb-2 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                  Profile
                </label>
                <div className="relative">
                  <select
                    value={selectedProfileId}
                    onChange={(e) => setSelectedProfileId(e.target.value)}
                    className="w-full appearance-none rounded-lg border px-3 py-2.5 pr-8 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
                    style={{ background: "var(--surface2)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
                  >
                    {profiles.map((p) => (
                      <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                  </select>
                  <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2" style={{ color: "var(--dmuted2)" }} />
                </div>
              </div>
            )}

            {/* 2. Connected Accounts */}
            <div className="mb-5">
                <label className="mb-2 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                  Connected accounts
                </label>
              <ConnectedAccountsGrid
                accounts={form.activeAccounts}
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
              accounts={form.selectedAccounts}
              onSelectIssue={focusIssue}
            />
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
              disabled={!form.canSubmit || isValidating}
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
              ) : form.submitting ? "Sending..." : validationChecked && (validationResult?.warnings?.length || 0) > 0 && (validationResult?.errors?.length || 0) === 0 ? "Publish anyway" : primaryLabel}
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
    </Sheet>
  );
}
