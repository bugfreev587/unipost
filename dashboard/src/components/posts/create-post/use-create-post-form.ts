"use client";

import { useState, useCallback, useMemo } from "react";
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

export function useCreatePostForm(accounts: SocialAccount[]) {
  const [mainContent, setMainContent] = useState("");
  const [mediaFiles, setMediaFiles] = useState<File[]>([]);
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

  const hasUnsavedContent = useMemo(() => {
    if (mainContent.trim()) return true;
    if (mediaFiles.length > 0) return true;
    return Object.values(overrides).some((o) => o.caption?.trim());
  }, [mainContent, mediaFiles, overrides]);

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
    // Must have content or media
    const hasContent = mainContent.trim() || Object.values(overrides).some((o) => o.caption?.trim());
    if (!hasContent && mediaFiles.length === 0) return false;
    if (publishMode === "schedule" && !scheduledAt) return false;
    if (publishMode === "schedule" && scheduledAt && new Date(scheduledAt) <= new Date()) return false;
    if (publishMode === "queue" && !queueId) return false;
    return true;
  }, [submitting, selectedAccountIds, hasOverLimit, mainContent, overrides, mediaFiles, publishMode, scheduledAt, queueId]);

  const buildPayload = useCallback(() => {
    const accountIds = [...selectedAccountIds];
    const hasOverrides = accountIds.some((id) => overrides[id]?.caption?.trim());

    const payload: Record<string, unknown> = {};

    if (hasOverrides) {
      // Use platform_posts[] format when any account has a custom caption
      payload.platform_posts = accountIds.map((id) => {
        const o = overrides[id];
        const entry: Record<string, unknown> = {
          account_id: id,
          caption: o?.caption?.trim() || mainContent.trim(),
        };
        if (o?.youtube) entry.platform_options = { ...o.youtube };
        if (o?.tiktok) entry.platform_options = { ...o.tiktok };
        if (o?.instagram) entry.platform_options = { ...o.instagram };
        if (o?.linkedin) entry.platform_options = { ...o.linkedin };
        return entry;
      });
    } else {
      // Legacy format: same caption for all accounts
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
  }, [selectedAccountIds, overrides, mainContent, publishMode, scheduledAt]);

  const reset = useCallback(() => {
    setMainContent("");
    setMediaFiles([]);
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

    // Actions
    toggleAccount,
    toggleAll,
    toggleBlockCollapse,
    updateOverrideCaption,
    updateOverridePlatformField,
    getCharCount,
    buildPayload,
    reset,

    // Derived
    hasUnsavedContent,
    hasOverLimit,
    canSubmit,
  };
}
