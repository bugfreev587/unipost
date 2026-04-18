"use client";

import { useEffect, useCallback, useState, useRef, useMemo, memo } from "react";
import { AlertTriangle, Loader2, Plus } from "lucide-react";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { ConnectedAccountsGrid, PostToGrid } from "./account-card-grid";
import { PlatformEditorBlock } from "./platform-editor-block";
import { EmptyPlatformState } from "./empty-platform-state";
import { PublishModePanel } from "./publish-mode-panel";
import {
  useCreatePostForm,
  PRIMARY_BUTTON_LABELS,
  type PublishMode,
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
  type SocialPostValidationIssue,
  type SocialPostValidationResult,
} from "@/lib/api";
import { cn } from "@/lib/utils";

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
        "relative w-[88px] h-[88px] rounded-lg overflow-hidden bg-[#0a0a0b] flex-shrink-0 group/thumb",
        "border",
        failed ? "border-[#ef4444]" : ready ? "border-[#2e2e38]" : "border-[#10b981]/60"
      )}
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
        <div className="absolute inset-0 bg-black/55 flex flex-col items-center justify-center text-[10px] font-mono text-[#10b981]">
          <div className="w-8 h-8 rounded-full border-2 border-[#10b981]/30 border-t-[#10b981] animate-spin mb-1" />
          {item.progress}%
        </div>
      )}

      {/* Failure overlay */}
      {failed && (
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); onRetry?.(); }}
          className="absolute inset-0 bg-black/65 flex flex-col items-center justify-center text-[10px] text-[#ef4444] hover:text-[#fca5a5] transition-colors"
        >
          <div className="w-6 h-6 rounded-full border-2 border-[#ef4444] flex items-center justify-center mb-1 text-sm font-bold">!</div>
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
      <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-[9px] text-[#ccc] px-1.5 py-0.5 truncate font-mono">
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
      <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium block mb-2.5">
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
        <label className="group flex flex-col items-center justify-center w-[88px] h-[88px] rounded-lg border border-dashed border-[#2e2e38] hover:border-[#8a8a93] bg-[#0a0a0b]/40 cursor-pointer transition-colors flex-shrink-0">
          <Plus className="w-4 h-4 text-[#8a8a93] group-hover:text-[#f4f4f5] transition-colors" />
          <div className="text-[9px] text-[#55555c] group-hover:text-[#8a8a93] mt-1 text-center leading-tight transition-colors">
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
  youtube_title_required: "Add a YouTube video title or main caption before publishing.",
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
        <div className="rounded-xl border border-[#7f1d1d] bg-[#261013] px-4 py-3.5">
          <div className="flex items-center gap-2 text-[#fecaca] mb-2">
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
                className="w-full text-left rounded-lg border border-[#7f1d1d]/60 bg-[#331418] px-3 py-2.5 hover:border-[#b91c1c] transition-colors"
              >
                <div className="text-[11px] font-mono uppercase tracking-[0.12em] text-[#fca5a5] mb-1">
                  {issueTargetLabel(issue, accounts)}
                </div>
                <div className="text-[13px] text-[#fee2e2] leading-relaxed">{issueSummary(issue)}</div>
              </button>
            ))}
          </div>
        </div>
      )}

      {warnings.length > 0 && (
        <div className="rounded-xl border border-[#92400e] bg-[#2d1d0f] px-4 py-3.5">
          <div className="flex items-center gap-2 text-[#fde68a] mb-2">
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
                className="w-full text-left rounded-lg border border-[#92400e]/55 bg-[#382411] px-3 py-2.5 hover:border-[#d97706] transition-colors"
              >
                <div className="text-[11px] font-mono uppercase tracking-[0.12em] text-[#fcd34d] mb-1">
                  {issueTargetLabel(issue, accounts)}
                </div>
                <div className="text-[13px] text-[#fef3c7] leading-relaxed">{issueSummary(issue)}</div>
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
  const [queues, setQueues] = useState<Array<{ id: string; name: string }>>([]);
  const [queuesLoaded, setQueuesLoaded] = useState(false);
  const [isValidating, setIsValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<SocialPostValidationResult | null>(null);
  const [validationChecked, setValidationChecked] = useState(false);
  const [warningsAcknowledged, setWarningsAcknowledged] = useState(false);
  const pendingCloseRef = useRef(false);
  const mainContentRef = useRef<HTMLTextAreaElement | null>(null);
  const mediaSectionRef = useRef<HTMLDivElement | null>(null);
  const publishPanelRef = useRef<HTMLDivElement | null>(null);
  const platformBlockRefs = useRef<Record<string, HTMLDivElement | null>>({});

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
      pendingCloseRef.current = false;
    }
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!open) return;
    setValidationResult(null);
    setValidationChecked(false);
    setWarningsAcknowledged(false);
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

  async function runValidation(payload: ReturnType<typeof form.buildPayload>) {
    const token = await getToken();
    if (!token) return { ok: false as const, tokenMissing: true as const };

    setIsValidating(true);
    try {
      const res = await validateSocialPost(token, payload as any);
      const result = res.data;
      setValidationResult(result);
      setValidationChecked(true);
      return { ok: result.errors.length === 0, result, token };
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
      await createSocialPost(token, workspaceId, payload as any);
      onCreated();
      onOpenChange(false);
    } catch (err) {
      console.error("Create post failed:", err);
      console.error("[CreatePost] payload was:", JSON.stringify(form.buildPayload(), null, 2));
    } finally {
      form.setSubmitting(false);
    }
  }

  async function handleSaveDraft() {
    form.setSubmitting(true);
    try {
      const token = await getToken();
      if (!token) return;
      const payload = form.buildPayload();
      (payload as any).publish_mode = "draft";
      await createSocialPost(token, workspaceId, payload as any);
      onCreated();
      onOpenChange(false);
    } catch (err) {
      console.error("Save draft failed:", err);
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
  ]);

  return (
    <Sheet open={open} onOpenChange={handleOpenChange} modal>
      <SheetContent
        showCloseButton={false}
        className="w-[75vw] bg-[#111113] border-l border-[#22222a]"
      >
        {/* Header */}
        <header className="flex items-start justify-between px-8 pt-7 pb-5 border-b border-[#22222a] flex-shrink-0">
          <div>
            <h2 className="font-serif text-3xl tracking-tight leading-none mb-1.5 text-[#f4f4f5]">
              Create post
            </h2>
            <p className="text-[#8a8a93] text-sm">
              Compose once, publish to any platform you&apos;ve connected.
            </p>
          </div>
          <button
            type="button"
            onClick={attemptClose}
            className="w-9 h-9 rounded-lg flex items-center justify-center text-[#8a8a93] hover:text-[#f4f4f5] hover:bg-[#17171a] transition-colors"
          >
            <svg width="16" height="16" viewBox="0 0 15 15" fill="none">
              <path d="M11.25 3.75l-7.5 7.5M3.75 3.75l7.5 7.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </header>

        {/* Body: two columns */}
        <div className="flex-1 flex min-h-0">
          {/* LEFT: Content + per-platform editors (3:2 ratio) */}
          <div className="flex-[3] overflow-y-auto px-8 py-7 border-r border-[#22222a] [&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-[#2e2e38] [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb:hover]:bg-[#3a3a46]">
            {/* Main content */}
            <section>
              <div className="flex items-center justify-between mb-2.5">
                <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium">
                  Content
                </label>
                <span className="text-[11px] text-[#55555c] font-mono">optional</span>
              </div>
              <textarea
                ref={mainContentRef}
                rows={5}
                placeholder="What's on your mind?"
                value={form.mainContent}
                onChange={(e) => form.setMainContent(e.target.value)}
                autoFocus
                className="w-full rounded-lg px-4 py-3 text-sm resize-none leading-relaxed bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)] placeholder:text-[#55555c]"
              />
              <div className="flex items-center justify-between mt-2">
                <p className="text-[11px] text-[#55555c]">
                  Used as the default for every selected platform unless overridden below.
                </p>
                <span className="text-[11px] font-mono text-[#55555c]">
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
                <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium">
                  Per-platform customization
                </label>
                <span className="text-[11px] text-[#55555c] font-mono">
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
          <aside className="flex-[2] overflow-y-auto px-6 py-7 bg-[#0a0a0b]/40 [&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-[#2e2e38] [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb:hover]:bg-[#3a3a46]">

            {/* 1. Profile selector */}
            {profiles.length > 0 && (
              <div className="mb-5">
                <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium block mb-2">
                  Profile
                </label>
                <div className="relative">
                  <select
                    value={selectedProfileId}
                    onChange={(e) => setSelectedProfileId(e.target.value)}
                    className="w-full rounded-lg px-3 py-2.5 pr-8 text-sm bg-[#17171a] border border-[#22222a] text-[#f4f4f5] outline-none appearance-none cursor-pointer transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]"
                  >
                    {profiles.map((p) => (
                      <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                  </select>
                  <ChevronDown className="absolute right-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-[#55555c] pointer-events-none" />
                </div>
              </div>
            )}

            {/* 2. Connected Accounts */}
            <div className="mb-5">
              <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium block mb-2">
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
                <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium">
                  Post to
                </label>
                <span className="text-[11px] text-[#55555c] font-mono">
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
            <div className="my-5 border-t border-[#22222a]" />

            <ValidationPanel
              errors={validationResult?.errors || []}
              warnings={validationResult?.warnings || []}
              accounts={form.selectedAccounts}
              onSelectIssue={focusIssue}
            />

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
        <footer className="flex items-center justify-between px-8 py-4 border-t border-[#22222a] bg-[#111113] flex-shrink-0">
          <div className="text-[11px] text-[#55555c] font-mono flex items-center gap-2">
            <kbd className="px-1.5 py-0.5 rounded border border-[#22222a] bg-[#17171a]">Esc</kbd>
            <span>to close</span>
            <span className="mx-1">&middot;</span>
            <kbd className="px-1.5 py-0.5 rounded border border-[#22222a] bg-[#17171a]">&#8984;</kbd>
            <kbd className="px-1.5 py-0.5 rounded border border-[#22222a] bg-[#17171a]">&#8629;</kbd>
            <span>to publish</span>
          </div>
          <div className="flex items-center gap-2.5">
            {disabledReason && (
              <span className="text-[11px] text-[#8a8a93] max-w-[260px] text-right leading-snug">
                {disabledReason}
              </span>
            )}
            <button
              type="button"
              onClick={attemptClose}
              className="px-4 py-2 text-sm text-[#8a8a93] hover:text-[#f4f4f5] rounded-lg transition-colors"
            >
              Cancel
            </button>
            {form.publishMode !== "draft" && (
              <button
                type="button"
                onClick={handleSaveDraft}
                disabled={form.submitting}
                className="px-4 py-2 text-sm text-[#f4f4f5] bg-[#17171a] hover:bg-[#1c1c20] border border-[#22222a] rounded-lg transition-colors disabled:opacity-50"
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
                "bg-[#10b981] hover:bg-emerald-400 text-black",
                "shadow-[0_0_0_1px_rgba(16,185,129,0.4),0_8px_24px_-8px_rgba(16,185,129,0.4)]",
                "disabled:opacity-50 disabled:cursor-not-allowed"
              )}
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
            <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-[61] bg-[#17171a] border border-[#22222a] rounded-xl p-6 w-[400px] shadow-2xl">
              <h3 className="text-base font-medium text-[#f4f4f5] mb-2">
                Discard unsaved changes?
              </h3>
              <p className="text-sm text-[#8a8a93] mb-6">
                You have unsaved content that will be lost if you close this drawer.
              </p>
              <div className="flex justify-end gap-2.5">
                <button
                  type="button"
                  onClick={() => setShowDiscardConfirm(false)}
                  className="px-4 py-2 text-sm text-[#8a8a93] hover:text-[#f4f4f5] rounded-lg transition-colors"
                >
                  Keep editing
                </button>
                <button
                  type="button"
                  onClick={confirmDiscard}
                  className="px-4 py-2 text-sm font-medium text-white bg-[#ef4444] hover:bg-red-400 rounded-lg transition-colors"
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
