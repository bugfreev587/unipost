"use client";

import { useState, useCallback, useMemo, useRef } from "react";
import type { CreateSocialPostPayload, EditablePlatformPost, SocialAccount, SocialPost } from "@/lib/api";
import { PLATFORM_LIMITS, countCharacters, getCountStatus } from "@/components/tools/platform-limits";
import { getAccountIdentityKey } from "./account-labels";

// --- Types ---

export type PublishMode = "now" | "schedule" | "queue" | "draft";

export interface PlatformOverride {
  caption: string;
  firstComment?: string;
  inReplyTo?: string;
  threadPosition?: string;
  threadReplies?: string[];
  // YouTube-specific
  youtube?: {
    title: string;
    category: string;
    visibility: "public" | "unlisted" | "private";
    madeForKids: "" | "yes" | "no";
    notifySubscribers: boolean;
    embeddable: boolean;
    license: "youtube" | "creativeCommon";
    publicStatsViewable: boolean;
    containsSyntheticMedia: boolean;
    defaultLanguage: string;
    recordingDate: string;
    publishAt: string;
    playlistId: string;
    tags: string;
    shorts: boolean;
  };
  // TikTok-specific — mirrors the fields TikTok's Content Posting API
  // audit requires the compose UI to expose. `privacy` is one of
  // TikTok's PRIVACY_LEVEL enum values (empty string = user hasn't
  // selected yet; blocks submit). The three disable_* toggles match
  // TikTok's post_info booleans directly so buildPayload can pass
  // them through without a translation layer.
  tiktok?: {
    privacy: "" | "PUBLIC_TO_EVERYONE" | "MUTUAL_FOLLOW_FRIENDS" | "FOLLOWER_OF_CREATOR" | "SELF_ONLY";
    disableComment: boolean;
    disableDuet: boolean;
    disableStitch: boolean;
    disclosureEnabled: boolean;
    yourBrand: boolean;        // → brand_organic_toggle
    brandedContent: boolean;   // → brand_content_toggle
  };
  // Instagram-specific
  instagram?: {
    mediaType: "feed" | "reels" | "story";
  };
  // LinkedIn-specific
  linkedin?: {
    visibility: "anyone" | "connections";
  };
  // Facebook-specific.
  // `link` — optional link attachment for Feed posts. Disallowed
  //   alongside media (validated server-side and enforced in the
  //   composer via the mediaAttached prop).
  // `mediaType` — which FB publish surface the post targets when
  //   media is attached: "feed" (default, /{page_id}/videos for
  //   videos or /{page_id}/photos for images) or "reel"
  //   (/{page_id}/video_reels; video-only). The toggle is hidden
  //   when no media is attached — Facebook Reels require a video.
  facebook?: {
    link: string;
    mediaType: "feed" | "reel";
  };
  pinterest?: {
    boardId: string;
    title: string;
    link: string;
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
  "facebook",
  "pinterest",
  "tiktok",
  "youtube",
] as const;

export const PLATFORM_LABELS: Record<string, string> = {
  twitter: "X",
  linkedin: "LinkedIn",
  bluesky: "Bluesky",
  threads: "Threads",
  instagram: "Instagram",
  facebook: "Facebook",
  pinterest: "Pinterest",
  tiktok: "TikTok",
  youtube: "YouTube",
};

export const PLATFORM_CHAR_LIMITS: Record<string, number> = {
  twitter: 280,
  linkedin: 3000,
  bluesky: 300,
  threads: 500,
  instagram: 2200,
  facebook: 63206,
  pinterest: 800,
  tiktok: 2200,
  youtube: 5000,
};

export const PLATFORM_BRAND_COLORS: Record<string, string> = {
  twitter: "#e7e7ea",
  linkedin: "#0a66c2",
  bluesky: "#1185fe",
  threads: "#e7e7ea",
  instagram: "#e1306c",
  facebook: "#1877f2",
  pinterest: "#e60023",
  tiktok: "#ff0050",
  youtube: "#ff0000",
};

// Defaults chosen to satisfy TikTok's audit UX rules:
//   - privacy: "" forces the user to pick from creator_info options (no default)
//   - disable_*: true means the interaction is OFF by default (all toggles
//     start unchecked; TikTok's field is inverted from "allow" wording)
//   - disclosureEnabled/yourBrand/brandedContent: commercial disclosure is
//     OFF until the user turns it on
export const DEFAULT_TIKTOK_FIELDS: NonNullable<PlatformOverride["tiktok"]> = {
  privacy: "",
  disableComment: true,
  disableDuet: true,
  disableStitch: true,
  disclosureEnabled: false,
  yourBrand: false,
  brandedContent: false,
};

export const DEFAULT_YOUTUBE_FIELDS: NonNullable<PlatformOverride["youtube"]> = {
  title: "",
  category: "22",
  visibility: "public",
  madeForKids: "",
  notifySubscribers: true,
  embeddable: true,
  license: "youtube",
  publicStatsViewable: true,
  containsSyntheticMedia: false,
  defaultLanguage: "",
  recordingDate: "",
  publishAt: "",
  playlistId: "",
  tags: "",
  shorts: false,
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
  // Measured video duration in seconds, or null once we've confirmed the
  // file isn't a readable video / can't be measured. `undefined` means
  // we haven't tried yet (or measurement is still in flight).
  durationSec: number | null | undefined;
  // Visual dimensions in pixels — captured by the same client-side
  // <video> probe that measures duration. Drives Facebook's placement
  // guidance (vertical 9:16 in feed → suggest switching to Reel) so
  // the user sees the conflict BEFORE the validator fires server-side.
  // Same null/undefined semantics as durationSec: undefined = not
  // measured yet, null = measured but not a video / unreadable.
  videoWidth: number | null | undefined;
  videoHeight: number | null | undefined;
}

export interface ExistingMediaItem {
  id?: string;
  url?: string;
  label: string;
}

/** Fingerprint a File for dedup — same file re-selected → same key */
function fileFingerprint(file: File): string {
  return `${file.name}::${file.size}::${file.lastModified}`;
}

/**
 * Measured visual + temporal metadata for a video file, captured
 * client-side via the same hidden <video> probe one decode pass yields.
 * width/height come from videoWidth/videoHeight after `loadedmetadata`;
 * null on either field signals the browser couldn't decode the moov.
 */
export interface VideoMetadata {
  durationSec: number | null;
  width: number | null;
  height: number | null;
}

function toDateTimeLocalValue(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function stringOption(options: Record<string, unknown>, key: string): string {
  const value = options[key];
  return typeof value === "string" ? value : "";
}

function boolOption(options: Record<string, unknown>, key: string, fallback: boolean): boolean {
  const value = options[key];
  return typeof value === "boolean" ? value : fallback;
}

function normalizeMediaType(value: string, fallback: "feed" | "reels" | "story"): "feed" | "reels" | "story" {
  return value === "reels" || value === "story" || value === "feed" ? value : fallback;
}

function normalizeFacebookMediaType(value: string): "feed" | "reel" {
  return value === "reel" ? "reel" : "feed";
}

function normalizeYouTubeVisibility(value: string): "public" | "unlisted" | "private" {
  return value === "unlisted" || value === "private" || value === "public" ? value : "public";
}

function normalizeLinkedInVisibility(value: string): "anyone" | "connections" {
  return value === "connections" ? "connections" : "anyone";
}

function tagsToInput(value: unknown): string {
  if (Array.isArray(value)) return value.filter((item): item is string => typeof item === "string").join(", ");
  return typeof value === "string" ? value : "";
}

function deriveEditablePlatformPosts(post: SocialPost, accounts: SocialAccount[]): EditablePlatformPost[] {
  if (post.platform_posts && post.platform_posts.length > 0) return post.platform_posts;
  const platforms = new Set(post.target_platforms || []);
  if (platforms.size === 0) return [];
  const caption = post.caption || "";
  return accounts
    .filter((account) => platforms.has(account.platform))
    .map((account) => ({
      account_id: account.id,
      caption,
      media_urls: post.media_urls || [],
    }));
}

function deriveExistingMediaItems(post: SocialPost, platformPosts: EditablePlatformPost[]): ExistingMediaItem[] {
  const byKey = new Map<string, ExistingMediaItem>();
  const add = (item: ExistingMediaItem) => {
    const key = item.id ? `id:${item.id}` : item.url ? `url:${item.url}` : "";
    if (key && !byKey.has(key)) byKey.set(key, item);
  };
  for (const entry of platformPosts) {
    for (const id of entry.media_ids || []) {
      add({ id, label: id });
    }
    for (const url of entry.media_urls || []) {
      add({ url, label: mediaLabelFromUrl(url) });
    }
  }
  if (byKey.size === 0) {
    for (const url of post.media_urls || []) {
      add({ url, label: mediaLabelFromUrl(url) });
    }
  }
  return Array.from(byKey.values());
}

function mediaLabelFromUrl(url: string): string {
  try {
    const parsed = new URL(url);
    const name = parsed.pathname.split("/").filter(Boolean).pop();
    return name || "Existing media";
  } catch {
    return url.split("/").filter(Boolean).pop() || "Existing media";
  }
}

function deriveOverridesFromPost(post: SocialPost, accounts: SocialAccount[]): Record<string, PlatformOverride> {
  const accountById = new Map(accounts.map((account) => [account.id, account]));
  const mainCaption = post.caption || "";
  const overrides: Record<string, PlatformOverride> = {};
  for (const entry of deriveEditablePlatformPosts(post, accounts)) {
    const account = accountById.get(entry.account_id);
    const options = entry.platform_options || {};
    const override: PlatformOverride = {
      caption: entry.caption || mainCaption,
      firstComment: entry.first_comment || "",
      inReplyTo: entry.in_reply_to || "",
      threadPosition: entry.thread_position ? String(entry.thread_position) : "",
    };
    if (account?.platform === "youtube") {
      override.youtube = {
        ...DEFAULT_YOUTUBE_FIELDS,
        title: stringOption(options, "title"),
        category: stringOption(options, "category_id") || DEFAULT_YOUTUBE_FIELDS.category,
        visibility: normalizeYouTubeVisibility(stringOption(options, "privacy_status")),
        madeForKids: typeof options.made_for_kids === "boolean" ? (options.made_for_kids ? "yes" : "no") : "",
        notifySubscribers: boolOption(options, "notify_subscribers", DEFAULT_YOUTUBE_FIELDS.notifySubscribers),
        embeddable: boolOption(options, "embeddable", DEFAULT_YOUTUBE_FIELDS.embeddable),
        license: stringOption(options, "license") === "creativeCommon" ? "creativeCommon" : "youtube",
        publicStatsViewable: boolOption(options, "public_stats_viewable", DEFAULT_YOUTUBE_FIELDS.publicStatsViewable),
        containsSyntheticMedia: boolOption(options, "contains_synthetic_media", DEFAULT_YOUTUBE_FIELDS.containsSyntheticMedia),
        defaultLanguage: stringOption(options, "default_language"),
        recordingDate: stringOption(options, "recording_date"),
        publishAt: stringOption(options, "publish_at") ? toDateTimeLocalValue(stringOption(options, "publish_at")) : "",
        playlistId: stringOption(options, "playlist_id"),
        tags: tagsToInput(options.tags),
        shorts: boolOption(options, "shorts", DEFAULT_YOUTUBE_FIELDS.shorts),
      };
    }
    if (account?.platform === "tiktok") {
      const brandOrganic = boolOption(options, "brand_organic_toggle", false);
      const brandContent = boolOption(options, "brand_content_toggle", false);
      override.tiktok = {
        privacy: stringOption(options, "privacy_level") as NonNullable<PlatformOverride["tiktok"]>["privacy"],
        disableComment: boolOption(options, "disable_comment", DEFAULT_TIKTOK_FIELDS.disableComment),
        disableDuet: boolOption(options, "disable_duet", DEFAULT_TIKTOK_FIELDS.disableDuet),
        disableStitch: boolOption(options, "disable_stitch", DEFAULT_TIKTOK_FIELDS.disableStitch),
        disclosureEnabled: brandOrganic || brandContent,
        yourBrand: brandOrganic,
        brandedContent: brandContent,
      };
    }
    if (account?.platform === "instagram") {
      override.instagram = {
        mediaType: normalizeMediaType(stringOption(options, "mediaType") || stringOption(options, "media_type"), "feed"),
      };
    }
    if (account?.platform === "linkedin") {
      override.linkedin = {
        visibility: normalizeLinkedInVisibility(stringOption(options, "visibility")),
      };
    }
    if (account?.platform === "facebook") {
      override.facebook = {
        link: stringOption(options, "link"),
        mediaType: normalizeFacebookMediaType(stringOption(options, "mediaType") || stringOption(options, "media_type")),
      };
    }
    if (account?.platform === "pinterest") {
      override.pinterest = {
        boardId: stringOption(options, "board_id"),
        title: stringOption(options, "title"),
        link: stringOption(options, "link"),
      };
    }
    overrides[entry.account_id] = override;
  }
  return overrides;
}

/**
 * Measure a video file's metadata (duration + visual dimensions) using
 * a hidden <video> element. Resolves to all-null when the file isn't a
 * video the browser can decode (unusual codecs, truncated files).
 *
 * Same probe drives:
 *   - the TikTok pre-R2 duration gate (rejects oversize uploads before
 *     they hit storage)
 *   - the Facebook placement guidance (vertical 9:16 in feed mode →
 *     suggest switching to Reel before publish)
 *
 * Captures all three fields in one decode pass so we don't pay the
 * createObjectURL / video element cost twice.
 */
export function measureVideoMetadata(file: File): Promise<VideoMetadata> {
  const empty: VideoMetadata = { durationSec: null, width: null, height: null };
  if (!file.type.startsWith("video/")) return Promise.resolve(empty);
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const video = document.createElement("video");
    video.preload = "metadata";
    let settled = false;
    const cleanup = () => {
      URL.revokeObjectURL(url);
      video.src = "";
    };
    video.onloadedmetadata = () => {
      if (settled) return;
      settled = true;
      const d = video.duration;
      // videoWidth/videoHeight are 0 until metadata is loaded, then
      // they reflect the visual dimensions in pixels (with rotation
      // already applied — a portrait phone clip reports as 1080×1920
      // even when stored as 1920×1080 with a rotation tag).
      const w = video.videoWidth;
      const h = video.videoHeight;
      cleanup();
      resolve({
        durationSec: Number.isFinite(d) ? d : null,
        width: w > 0 ? w : null,
        height: h > 0 ? h : null,
      });
    };
    video.onerror = () => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve(empty);
    };
    video.src = url;
  });
}

export function useCreatePostForm(accounts: SocialAccount[]) {
  const [mainContent, setMainContent] = useState("");
  const [mediaFiles, setMediaFiles] = useState<File[]>([]);
  const [mediaItems, setMediaItems] = useState<MediaItem[]>([]);
  const [existingMediaItems, setExistingMediaItems] = useState<ExistingMediaItem[]>([]);
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

  // Detect duplicate platform accounts by stable platform account id when
  // available. Some Threads rows have a generic account_name ("threads"),
  // so using the display label would collapse two real accounts.
  // When the same underlying account is connected via BYO + managed,
  // or selected from two profiles, we should only publish once.
  const { duplicateAccountIds, uniqueSelectedAccounts } = useMemo(() => {
    const seen = new Map<string, string>();
    const dupes = new Set<string>();
    const unique: SocialAccount[] = [];

    for (const acc of selectedAccounts) {
      const key = getAccountIdentityKey(acc);
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

  const replaceSelectedAccounts = useCallback((ids: string[]) => {
    setSelectedAccountIds(new Set(ids));
  }, []);

  const toggleBlockCollapse = useCallback((accountId: string) => {
    setCollapsedBlocks((prev) => {
      const next = new Set(prev);
      if (next.has(accountId)) next.delete(accountId);
      else next.add(accountId);
      return next;
    });
  }, []);

  const expandBlock = useCallback((accountId: string) => {
    setCollapsedBlocks((prev) => {
      if (!prev.has(accountId)) return prev;
      const next = new Set(prev);
      next.delete(accountId);
      return next;
    });
  }, []);

  const updateOverrideCaption = useCallback((accountId: string, caption: string) => {
    setOverrides((prev) => ({
      ...prev,
      [accountId]: { ...prev[accountId], caption },
    }));
  }, []);

  const updateOverrideFirstComment = useCallback((accountId: string, firstComment: string) => {
    setOverrides((prev) => ({
      ...prev,
      [accountId]: { ...prev[accountId], firstComment },
    }));
  }, []);

  const updateOverrideThreadFields = useCallback((accountId: string, fields: Partial<Pick<PlatformOverride, "inReplyTo" | "threadPosition">>) => {
    setOverrides((prev) => ({
      ...prev,
      [accountId]: { ...prev[accountId], ...fields },
    }));
  }, []);

  const addOverrideThreadReply = useCallback((accountId: string) => {
    setOverrides((prev) => {
      const currentReplies = prev[accountId]?.threadReplies || [];
      return {
        ...prev,
        [accountId]: { ...prev[accountId], threadReplies: [...currentReplies, ""] },
      };
    });
  }, []);

  const updateOverrideThreadReply = useCallback((accountId: string, index: number, value: string) => {
    setOverrides((prev) => {
      const currentReplies = [...(prev[accountId]?.threadReplies || [])];
      currentReplies[index] = value;
      return {
        ...prev,
        [accountId]: { ...prev[accountId], threadReplies: currentReplies },
      };
    });
  }, []);

  const removeOverrideThreadReply = useCallback((accountId: string, index: number) => {
    setOverrides((prev) => {
      const currentReplies = (prev[accountId]?.threadReplies || []).filter((_, replyIndex) => replyIndex !== index);
      return {
        ...prev,
        [accountId]: { ...prev[accountId], threadReplies: currentReplies },
      };
    });
  }, []);

  const updateOverridePlatformField = useCallback(
    <K extends "youtube" | "tiktok" | "instagram" | "linkedin" | "facebook" | "pinterest">(
      accountId: string,
      platform: K,
      fields: Partial<NonNullable<PlatformOverride[K]>>
    ) => {
      setOverrides((prev) => {
        const current = prev[accountId]?.[platform] as Record<string, unknown> | undefined;
        // Seed platform defaults on the first touch. Without this,
        // partial updates leave fields undefined in the stored state
        // — the TikTok panel then falls back to React's truthiness
        // rules when computing toggle "checked" states, so a
        // disable_comment that was never explicitly set reads as
        // undefined, !undefined === true, and the Comment box looks
        // ticked the moment the user picks a privacy level. That
        // defaults-on-first-touch problem also silently drops the
        // disable_* booleans from buildPayload's output, which would
        // leave TikTok to apply its own "comments allowed" default —
        // a policy violation since TikTok requires the user to
        // explicitly opt into each interaction type.
        const seedDefaults: Record<string, Record<string, unknown>> = {
          tiktok: DEFAULT_TIKTOK_FIELDS,
          youtube: DEFAULT_YOUTUBE_FIELDS,
        };
        const base = current ?? seedDefaults[platform as string] ?? {};
        return {
          ...prev,
          [accountId]: {
            ...prev[accountId],
            [platform]: { ...base, ...fields },
          },
        };
      });
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
        return [...prev, { file, fingerprint: fp, mediaId: cachedId, progress: 100, error: null, durationSec: undefined, videoWidth: undefined, videoHeight: undefined }];
      }

      // New file — needs upload
      result = { cached: false, fingerprint: fp, mediaId: null };
      return [...prev, { file, fingerprint: fp, mediaId: null, progress: 0, error: null, durationSec: undefined, videoWidth: undefined, videoHeight: undefined }];
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

  const removeExistingMediaItem = useCallback((index: number) => {
    setExistingMediaItems((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const existingMediaIds = useMemo(
    () => existingMediaItems.map((item) => item.id).filter((id): id is string => !!id),
    [existingMediaItems]
  );
  const existingMediaUrls = useMemo(
    () => existingMediaItems.map((item) => item.url).filter((url): url is string => !!url),
    [existingMediaItems]
  );
  const totalMediaCount = existingMediaItems.length + mediaItems.length;

  const allMediaUploaded = useMemo(
    () => mediaItems.length === 0 || mediaItems.every((m) => m.mediaId !== null),
    [mediaItems]
  );

  const hasPlatformOnlyContent = useCallback((accountId: string) => {
    const override = overrides[accountId];
    // Platform-only content means the user has filled in something
    // that counts as a valid post body even without a main caption:
    //   - YouTube: video title (main caption is the description)
    //   - Facebook: link-only post (FB generates the preview card)
    return !!override?.youtube?.title?.trim() || !!override?.facebook?.link?.trim();
  }, [overrides]);

  const hasUnsavedContent = useMemo(() => {
    if (mainContent.trim()) return true;
    if (totalMediaCount > 0) return true;
    return Object.entries(overrides).some(([accountId, o]) => o.caption?.trim() || o.firstComment?.trim() || o.inReplyTo?.trim() || o.threadPosition?.trim() || o.threadReplies?.some((reply) => reply.trim()) || hasPlatformOnlyContent(accountId));
  }, [mainContent, totalMediaCount, overrides, hasPlatformOnlyContent]);

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

  // Compute which selected accounts are TikTok so the canSubmit guard can
  // enforce the Content Posting API audit rules (privacy picked, disclosure
  // complete, no branded-private conflict). All selected TikTok accounts
  // must satisfy the rules for the button to enable.
  const tiktokBlocker = useMemo(() => {
    for (const acc of selectedAccounts) {
      if (acc.platform !== "tiktok") continue;
      const t = overrides[acc.id]?.tiktok;
      // No tiktok override object means the user hasn't touched any
      // of the fields — privacy therefore hasn't been set.
      if (!t || !t.privacy) return "tiktok_privacy";
      if (t.disclosureEnabled && !t.yourBrand && !t.brandedContent) return "tiktok_disclosure";
      if (t.disclosureEnabled && t.brandedContent && t.privacy === "SELF_ONLY") return "tiktok_branded_private";
    }
    return null;
  }, [selectedAccounts, overrides]);

  const pinterestBlocker = useMemo(() => {
    const pinterestAccounts = selectedAccounts.filter((acc) => acc.platform === "pinterest");
    if (pinterestAccounts.length === 0) return null;
    if (totalMediaCount === 0) return "pinterest_media";
    if (totalMediaCount !== 1) return "pinterest_single_media";
    for (const acc of pinterestAccounts) {
      if (!overrides[acc.id]?.pinterest?.boardId?.trim()) return "pinterest_board";
    }
    return null;
  }, [selectedAccounts, totalMediaCount, overrides]);

  const canSubmit = useMemo(() => {
    if (submitting) return false;
    if (selectedAccountIds.size === 0) return false;
    if (hasOverLimit) return false;
    if (!allMediaUploaded) return false; // wait for uploads to finish
    const hasContent = mainContent.trim() || Object.entries(overrides).some(([accountId, o]) => o.caption?.trim() || hasPlatformOnlyContent(accountId));
    if (!hasContent && totalMediaCount === 0) return false;
    if (publishMode === "schedule" && !scheduledAt) return false;
    if (publishMode === "schedule" && scheduledAt && new Date(scheduledAt) <= new Date()) return false;
    if (publishMode === "queue" && !queueId) return false;
    if (tiktokBlocker) return false;
    if (pinterestBlocker) return false;
    return true;
  }, [submitting, selectedAccountIds, hasOverLimit, allMediaUploaded, mainContent, overrides, totalMediaCount, publishMode, scheduledAt, queueId, hasPlatformOnlyContent, tiktokBlocker, pinterestBlocker]);

  // Not wrapped in useCallback — always reads latest state to avoid
  // stale closure issues with mediaItems and preserved edit media.
  function buildPayload(): CreateSocialPostPayload {
    const accountIds = uniqueSelectedAccounts.map((a) => a.id);
    // Any sign of per-account content forces the per-platform
    // payload branch: caption override, a YouTube title-only post,
    // a Facebook link-only post, etc. Without this, platform_options
    // (link / title / etc.) get dropped and the backend sees an
    // empty request — the user's input is silently lost.
    const hasOverrides = accountIds.some(
      (id) => overrides[id]?.caption?.trim() || overrides[id]?.firstComment?.trim() || overrides[id]?.inReplyTo?.trim() || overrides[id]?.threadPosition?.trim() || overrides[id]?.threadReplies?.some((reply) => reply.trim()) || hasPlatformOnlyContent(id)
    );

    const payload: CreateSocialPostPayload = {};

    // Read from ref to always get the absolute latest mediaItems
    const latestMediaItems = mediaItemsRef.current;
    const currentMediaIds = [
      ...existingMediaIds,
      ...latestMediaItems.filter((m) => m.mediaId).map((m) => m.mediaId!),
    ];
    const mediaIds = currentMediaIds.length > 0 ? currentMediaIds : undefined;
    const mediaUrls = existingMediaUrls.length > 0 ? existingMediaUrls : undefined;

    if (hasOverrides || mediaIds || mediaUrls) {
      payload.platform_posts = accountIds.flatMap((id) => {
        const o = overrides[id];
        const account = uniqueSelectedAccounts.find((candidate) => candidate.id === id);
        const entry: NonNullable<CreateSocialPostPayload["platform_posts"]>[number] = {
          account_id: id,
          caption: o?.caption?.trim() || mainContent.trim(),
        };
        if (o?.firstComment?.trim()) entry.first_comment = o.firstComment.trim();
        if (o?.inReplyTo?.trim()) entry.in_reply_to = o.inReplyTo.trim();
        if (o?.threadPosition?.trim()) {
          const parsedThreadPosition = Number.parseInt(o.threadPosition, 10);
          if (Number.isFinite(parsedThreadPosition) && parsedThreadPosition > 0) {
            entry.thread_position = parsedThreadPosition;
          }
        }
        if (mediaIds) entry.media_ids = mediaIds;
        if (mediaUrls) entry.media_urls = mediaUrls;
        if (account?.platform === "youtube") {
          const youtube = { ...DEFAULT_YOUTUBE_FIELDS, ...o?.youtube };
          entry.platform_options = {
            category_id: youtube.category,
            privacy_status: youtube.visibility,
            notify_subscribers: youtube.notifySubscribers,
            embeddable: youtube.embeddable,
            license: youtube.license,
            public_stats_viewable: youtube.publicStatsViewable,
            contains_synthetic_media: youtube.containsSyntheticMedia,
            shorts: youtube.shorts,
          };
          if (youtube.madeForKids) {
            entry.platform_options.made_for_kids = youtube.madeForKids === "yes";
          }
          if (youtube.title.trim()) {
            entry.platform_options.title = youtube.title.trim();
          }
          if (youtube.defaultLanguage.trim()) {
            entry.platform_options.default_language = youtube.defaultLanguage.trim();
          }
          if (youtube.recordingDate.trim()) {
            entry.platform_options.recording_date = youtube.recordingDate.trim();
          }
          if (youtube.publishAt.trim()) {
            entry.platform_options.publish_at = new Date(youtube.publishAt).toISOString();
          }
          if (youtube.playlistId.trim()) {
            entry.platform_options.playlist_id = youtube.playlistId.trim();
          }
          const tags = youtube.tags
            .split(",")
            .map((tag) => tag.trim())
            .filter(Boolean);
          if (tags.length > 0) {
            entry.platform_options.tags = tags;
          }
        }
        if (o?.tiktok) {
          // Translate the UI's friendly field names into the exact keys
          // TikTok's Content Posting API expects, so the Go adapter can
          // forward them straight through post_info without a second
          // mapping layer. Toggles only emitted when the disclosure is
          // ON — otherwise they stay absent (TikTok treats missing as
          // false, same as "no disclosure").
          const t = o.tiktok;
          entry.platform_options = {
            privacy_level: t.privacy,
            disable_comment: t.disableComment,
            disable_duet: t.disableDuet,
            disable_stitch: t.disableStitch,
            brand_organic_toggle: t.disclosureEnabled && t.yourBrand,
            brand_content_toggle: t.disclosureEnabled && t.brandedContent,
          };
        }
        if (o?.instagram) entry.platform_options = { ...o.instagram };
        if (o?.linkedin) entry.platform_options = { ...o.linkedin };
        if (o?.facebook) {
          const fbOptions: Record<string, string> = {};
          const link = (o.facebook.link || "").trim();
          if (link) fbOptions.link = link;
          // Only forward mediaType when the user explicitly picked
          // Reel. Omitting defaults to Feed on the server and keeps
          // old payloads shape-identical.
          if (o.facebook.mediaType === "reel") {
            fbOptions.mediaType = "reel";
          }
          if (Object.keys(fbOptions).length > 0) {
            entry.platform_options = fbOptions;
          }
        }
        if (o?.pinterest) {
          const pinOptions: Record<string, string> = {};
          const boardId = (o.pinterest.boardId || "").trim();
          const title = (o.pinterest.title || "").trim();
          const link = (o.pinterest.link || "").trim();
          if (boardId) pinOptions.board_id = boardId;
          if (title) pinOptions.title = title;
          if (link) pinOptions.link = link;
          if (Object.keys(pinOptions).length > 0) {
            entry.platform_options = pinOptions;
          }
        }
        const threadReplies = (o?.threadReplies || []).map((reply) => reply.trim()).filter(Boolean);
        if (threadReplies.length === 0) {
          return [entry];
        }

        const startPosition = entry.thread_position && entry.thread_position > 0 ? entry.thread_position : 1;
        entry.thread_position = startPosition;
        return [
          entry,
          ...threadReplies.map((reply, replyIndex) => ({
            account_id: id,
            caption: reply,
            thread_position: startPosition + replyIndex + 1,
          })),
        ];
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
    setExistingMediaItems([]);
    setUploadCache(new Map());
    setSelectedAccountIds(new Set());
    setOverrides({});
    setPublishMode("now");
    setScheduledAt("");
    setQueueId("");
    setSubmitting(false);
    setCollapsedBlocks(new Set());
  }, []);

  const hydrateFromPost = useCallback((post: SocialPost, availableAccounts: SocialAccount[]) => {
    const platformPosts = deriveEditablePlatformPosts(post, availableAccounts);
    const firstCaption = platformPosts[0]?.caption || post.caption || "";
    setMainContent(firstCaption);
    setMediaFiles([]);
    setMediaItems([]);
    setExistingMediaItems(deriveExistingMediaItems(post, platformPosts));
    setUploadCache(new Map());
    setSelectedAccountIds(new Set(platformPosts.map((entry) => entry.account_id)));
    setOverrides(deriveOverridesFromPost(post, availableAccounts));
    setPublishMode(post.status === "scheduled" ? "schedule" : "draft");
    setScheduledAt(post.scheduled_at ? toDateTimeLocalValue(post.scheduled_at) : "");
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
    existingMediaItems,
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
    uniqueSelectedAccounts,
    totalMediaCount,

    // Actions
    toggleAccount,
    toggleAll,
    replaceSelectedAccounts,
    toggleBlockCollapse,
    expandBlock,
    updateOverrideCaption,
    updateOverrideFirstComment,
    updateOverrideThreadFields,
    addOverrideThreadReply,
    updateOverrideThreadReply,
    removeOverrideThreadReply,
    updateOverridePlatformField,
    addMediaItem,
    updateMediaItem,
    removeMediaItem,
    removeExistingMediaItem,
    getCharCount,
    buildPayload,
    hydrateFromPost,
    reset,

    // Derived
    hasUnsavedContent,
    hasOverLimit,
    canSubmit,
    tiktokBlocker,
    pinterestBlocker,
  };
}
