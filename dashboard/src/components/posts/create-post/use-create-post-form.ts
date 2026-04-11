"use client";

import { useState, useCallback, useMemo, useRef } from "react";
import type { SocialAccount } from "@/lib/api";
import { PLATFORM_LIMITS, countCharacters, getCountStatus } from "@/components/tools/platform-limits";

// --- Types ---

export type PublishMode = "now" | "schedule" | "queue" | "draft";

export interface PlatformOverride {
  caption: string;
  // YouTube-specific
  youtube?: {
    title: string;
    category: string;
    visibility: "public" | "unlisted" | "private";
  };
  // TikTok-specific
  tiktok?: {
    privacy: "public" | "friends" | "private";
    interactions: "allow_all" | "comments_only" | "disable_all";
  };
  // Instagram-specific
  instagram?: {
    mediaType: "feed" | "reels" | "story";
  };
  // LinkedIn-specific
  linkedin?: {
    visibility: "anyone" | "connections";
  };
}

export interface CreatePostFormState {
  mainContent: string;
  mediaFiles: File[];
  selectedAccountIds: Set<string>;
  overrides: Record<string, PlatformOverride>;
  publishMode: PublishMode;
  scheduledAt: string;
  queueId: string;
}

export interface CharCountInfo {
  count: number;
  limit: number;
  status: "ok" | "warning" | "over";
}

// Canonical platform order for left-side editor blocks
export const PLATFORM_ORDER = [
  "twitter",
  "linkedin",
  "bluesky",
  "threads",
  "instagram",
  "tiktok",
  "youtube",
] as const;

export const PLATFORM_LABELS: Record<string, string> = {
  twitter: "X",
  linkedin: "LinkedIn",
  bluesky: "Bluesky",
  threads: "Threads",
  instagram: "Instagram",
  tiktok: "TikTok",
  youtube: "YouTube",
};

export const PLATFORM_CHAR_LIMITS: Record<string, number> = {
  twitter: 280,
  linkedin: 3000,
  bluesky: 300,
  threads: 500,
  instagram: 2200,
  tiktok: 2200,
  youtube: 5000,
};

export const PLATFORM_BRAND_COLORS: Record<string, string> = {
  twitter: "#e7e7ea",
  linkedin: "#0a66c2",
  bluesky: "#1185fe",
  threads: "#e7e7ea",
  instagram: "#e1306c",
  tiktok: "#ff0050",
  youtube: "#ff0000",
};

export const PRIMARY_BUTTON_LABELS: Record<PublishMode, string> = {
  now: "Publish now",
  schedule: "Schedule post",
  queue: "Add to queue",
  draft: "Save draft",
};

// --- Hook ---

export interface MediaItem {
  file: File;
  fingerprint: string; // unique key: name + size + lastModified
  mediaId: string | null; // null = not uploaded yet
  progress: number; // 0-100
  error: string | null;
}

/** Fingerprint a File for dedup — same file re-selected → same key */
function fileFingerprint(file: File): string {
  return `${file.name}::${file.size}::${file.lastModified}`;
}

export function useCreatePostForm(accounts: SocialAccount[]) {
  const [mainContent, setMainContent] = useState("");
  const [mediaFiles, setMediaFiles] = useState<File[]>([]);
  const [mediaItems, setMediaItems] = useState<MediaItem[]>([]);
  // Ref for latest mediaItems — used by buildPayload to avoid stale closures
  const mediaItemsRef = useRef<MediaItem[]>([]);
  mediaItemsRef.current = mediaItems;
  // Cache: fingerprint → mediaId (persists across add/remove in same session)
  const [uploadCache, setUploadCache] = useState<Map<string, string>>(new Map());
  const [selectedAccountIds, setSelectedAccountIds] = useState<Set<string>>(new Set());
  const [overrides, setOverrides] = useState<Record<string, PlatformOverride>>({});
  const [publishMode, setPublishMode] = useState<PublishMode>("now");
  const [scheduledAt, setScheduledAt] = useState("");
  const [queueId, setQueueId] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [collapsedBlocks, setCollapsedBlocks] = useState<Set<string>>(new Set());

  const activeAccounts = useMemo(
    () => accounts.filter((a) => a.status === "active"),
    [accounts]
  );

  // Preserve the same order as activeAccounts (matches the right-side
  // "Post To" card grid) so the per-platform editors on the left align
  // with the account cards on the right.
  const selectedAccounts = useMemo(() => {
    return activeAccounts.filter((a) => selectedAccountIds.has(a.id));
  }, [activeAccounts, selectedAccountIds]);

  // Detect duplicate platform accounts (same platform + account_name).
  // When the same underlying account is connected via BYO + managed,
  // or selected from two profiles, we should only publish once.
  const { duplicateAccountIds, uniqueSelectedAccounts } = useMemo(() => {
    const seen = new Map<string, string>(); // "platform::account_name" → first account id
    const dupes = new Set<string>();
    const unique: SocialAccount[] = [];

    for (const acc of selectedAccounts) {
      const key = `${acc.platform}::${(acc.account_name || "").toLowerCase()}`;
      const existing = seen.get(key);
      if (existing) {
        dupes.add(acc.id);
      } else {
        seen.set(key, acc.id);
        unique.push(acc);
      }
    }
    return { duplicateAccountIds: dupes, uniqueSelectedAccounts: unique };
  }, [selectedAccounts]);

  const toggleAccount = useCallback((id: string) => {
    setSelectedAccountIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleAll = useCallback(() => {
    setSelectedAccountIds((prev) => {
      if (prev.size === activeAccounts.length) return new Set();
      return new Set(activeAccounts.map((a) => a.id));
    });
  }, [activeAccounts]);

  const toggleBlockCollapse = useCallback((accountId: string) => {
    setCollapsedBlocks((prev) => {
      const next = new Set(prev);
      if (next.has(accountId)) next.delete(accountId);
      else next.add(accountId);
      return next;
    });
  }, []);

  const updateOverrideCaption = useCallback((accountId: string, caption: string) => {
    setOverrides((prev) => ({
      ...prev,
      [accountId]: { ...prev[accountId], caption },
    }));
  }, []);

  const updateOverridePlatformField = useCallback(
    <K extends "youtube" | "tiktok" | "instagram" | "linkedin">(
      accountId: string,
      platform: K,
      fields: Partial<NonNullable<PlatformOverride[K]>>
    ) => {
      setOverrides((prev) => ({
        ...prev,
        [accountId]: {
          ...prev[accountId],
          [platform]: {
            ...(prev[accountId]?.[platform] as Record<string, unknown>),
            ...fields,
          },
        },
      }));
    },
    []
  );

  const getCharCount = useCallback(
    (text: string, platform: string): CharCountInfo => {
      const pl = PLATFORM_LIMITS.find((p) => p.platform === platform);
      if (!pl) return { count: text.length, limit: 99999, status: "ok" };
      const count = countCharacters(text, pl.countingMethod);
      const status = getCountStatus(count, pl.maxLength);
      const limit = pl.maxLength;
      // Map the 80% threshold to 90% for the PRD spec (>90% = amber)
      if (status === "warning" && count / limit < 0.9) {
        return { count, limit, status: "ok" };
      }
      return { count, limit, status };
    },
    []
  );

  // Returns true if the file was already uploaded (cache hit → no upload needed).
  // Uses a ref for uploadCache to avoid stale closure issues.
  const uploadCacheRef = useRef(uploadCache);
  uploadCacheRef.current = uploadCache;

  const addMediaItem = useCallback((file: File): { cached: boolean; fingerprint: string; mediaId: string | null } => {
    const fp = fileFingerprint(file);
    let result: { cached: boolean; fingerprint: string; mediaId: string | null } = { cached: false, fingerprint: fp, mediaId: null };

    setMediaItems((prev) => {
      // Check if already displayed (uses current state, not stale closure)
      if (prev.some((m) => m.fingerprint === fp)) {
        result = { cached: true, fingerprint: fp, mediaId: null };
        return prev; // no change
      }

      // Check upload cache — file was uploaded before in this session
      const cachedId = uploadCacheRef.current.get(fp);
      if (cachedId) {
        result = { cached: true, fingerprint: fp, mediaId: cachedId };
        return [...prev, { file, fingerprint: fp, mediaId: cachedId, progress: 100, error: null }];
      }

      // New file — needs upload
      result = { cached: false, fingerprint: fp, mediaId: null };
      return [...prev, { file, fingerprint: fp, mediaId: null, progress: 0, error: null }];
    });

    // Add to mediaFiles unless it was already displayed (duplicate)
    if (!(result.cached && !result.mediaId)) {
      setMediaFiles((prev) => [...prev, file]);
    }

    return result;
  }, []);

  // Indexed by fingerprint, not array index — multiple in-flight uploads
  // queued via Array.forEach all share the same component-render closure,
  // so an index-based lookup races and overwrites the wrong slot.
  const updateMediaItem = useCallback((fingerprint: string, update: Partial<MediaItem>) => {
    setMediaItems((prev) => {
      const updated = prev.map((item) => item.fingerprint === fingerprint ? { ...item, ...update } : item);
      // When upload completes, cache the fingerprint → mediaId
      if (update.mediaId) {
        setUploadCache((cache) => {
          const next = new Map(cache);
          next.set(fingerprint, update.mediaId!);
          return next;
        });
      }
      return updated;
    });
  }, []);

  const removeMediaItem = useCallback((index: number) => {
    // Remove from display but keep the upload cache entry
    setMediaItems((prev) => prev.filter((_, i) => i !== index));
    setMediaFiles((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const uploadedMediaIds = useMemo(
    () => mediaItems.filter((m) => m.mediaId).map((m) => m.mediaId!),
    [mediaItems]
  );

  const allMediaUploaded = useMemo(
    () => mediaItems.length === 0 || mediaItems.every((m) => m.mediaId !== null),
    [mediaItems]
  );

  const hasUnsavedContent = useMemo(() => {
    if (mainContent.trim()) return true;
    if (mediaItems.length > 0) return true;
    return Object.values(overrides).some((o) => o.caption?.trim());
  }, [mainContent, mediaItems, overrides]);

  const hasOverLimit = useMemo(() => {
    for (const acc of selectedAccounts) {
      const override = overrides[acc.id];
      const text = override?.caption?.trim() || mainContent;
      if (!text) continue;
      const info = getCharCount(text, acc.platform);
      if (info.status === "over") return true;
    }
    return false;
  }, [selectedAccounts, overrides, mainContent, getCharCount]);

  const canSubmit = useMemo(() => {
    if (submitting) return false;
    if (selectedAccountIds.size === 0) return false;
    if (hasOverLimit) return false;
    if (!allMediaUploaded) return false; // wait for uploads to finish
    const hasContent = mainContent.trim() || Object.values(overrides).some((o) => o.caption?.trim());
    if (!hasContent && mediaItems.length === 0) return false;
    if (publishMode === "schedule" && !scheduledAt) return false;
    if (publishMode === "schedule" && scheduledAt && new Date(scheduledAt) <= new Date()) return false;
    if (publishMode === "queue" && !queueId) return false;
    return true;
  }, [submitting, selectedAccountIds, hasOverLimit, allMediaUploaded, mainContent, overrides, mediaItems, publishMode, scheduledAt, queueId]);

  // Not wrapped in useCallback — always reads latest state to avoid
  // stale closure issues with mediaItems/uploadedMediaIds.
  function buildPayload() {
    const accountIds = uniqueSelectedAccounts.map((a) => a.id);
    const hasOverrides = accountIds.some((id) => overrides[id]?.caption?.trim());

    const payload: Record<string, unknown> = {};

    // Read from ref to always get the absolute latest mediaItems
    const latestMediaItems = mediaItemsRef.current;
    const currentMediaIds = latestMediaItems.filter((m) => m.mediaId).map((m) => m.mediaId!);
    const mediaIds = currentMediaIds.length > 0 ? currentMediaIds : undefined;
    console.log("[buildPayload] latestMediaItems:", latestMediaItems.length, "currentMediaIds:", currentMediaIds);

    if (hasOverrides || mediaIds) {
      payload.platform_posts = accountIds.map((id) => {
        const o = overrides[id];
        const entry: Record<string, unknown> = {
          account_id: id,
          caption: o?.caption?.trim() || mainContent.trim(),
        };
        if (mediaIds) entry.media_ids = mediaIds;
        if (o?.youtube) entry.platform_options = { ...o.youtube };
        if (o?.tiktok) entry.platform_options = { ...o.tiktok };
        if (o?.instagram) entry.platform_options = { ...o.instagram };
        if (o?.linkedin) entry.platform_options = { ...o.linkedin };
        return entry;
      });
    } else {
      payload.caption = mainContent.trim();
      payload.account_ids = accountIds;
    }

    if (publishMode === "schedule" && scheduledAt) {
      payload.scheduled_at = new Date(scheduledAt).toISOString();
    }
    if (publishMode === "draft") {
      payload.status = "draft";
    }

    return payload;
  }

  const reset = useCallback(() => {
    setMainContent("");
    setMediaFiles([]);
    setMediaItems([]);
    setUploadCache(new Map());
    setSelectedAccountIds(new Set());
    setOverrides({});
    setPublishMode("now");
    setScheduledAt("");
    setQueueId("");
    setSubmitting(false);
    setCollapsedBlocks(new Set());
  }, []);

  return {
    // State
    mainContent,
    setMainContent,
    mediaFiles,
    setMediaFiles,
    mediaItems,
    selectedAccountIds,
    selectedAccounts,
    activeAccounts,
    overrides,
    publishMode,
    setPublishMode,
    scheduledAt,
    setScheduledAt,
    queueId,
    setQueueId,
    submitting,
    setSubmitting,
    collapsedBlocks,
    allMediaUploaded,
    duplicateAccountIds,

    // Actions
    toggleAccount,
    toggleAll,
    toggleBlockCollapse,
    updateOverrideCaption,
    updateOverridePlatformField,
    addMediaItem,
    updateMediaItem,
    removeMediaItem,
    getCharCount,
    buildPayload,
    reset,

    // Derived
    hasUnsavedContent,
    hasOverLimit,
    canSubmit,
  };
}
