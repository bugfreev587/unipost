import type {
  AIPostAssistMode,
  AIPostAssistRequest,
  PlatformCapabilitiesEnvelope,
  SocialAccount,
  SocialPostValidationIssue,
} from "@/lib/api";
import type { MediaItem, PlatformOverride } from "./use-create-post-form";

export type AIAssistObjective = "awareness" | "engagement" | "clicks" | "sales";
export type AIAssistTone = "professional" | "friendly" | "bold" | "playful";

const FIRST_COMMENT_SUPPORTED_PLATFORMS = new Set(["twitter", "instagram", "linkedin"]);

export function supportsFirstComment(
  platform: string,
  capabilities?: PlatformCapabilitiesEnvelope["platforms"] | null
): boolean {
  const normalized = platform.toLowerCase();
  const capability = capabilities?.[normalized];
  if (capability?.first_comment) {
    return !!capability.first_comment.supported;
  }
  return FIRST_COMMENT_SUPPORTED_PLATFORMS.has(normalized);
}

export function getPlatformCaptionLimit(
  platform: string,
  fallback: number,
  capabilities?: PlatformCapabilitiesEnvelope["platforms"] | null
): number {
  const normalized = platform.toLowerCase();
  return capabilities?.[normalized]?.text?.max_length || fallback;
}

export function getFirstCommentMaxLength(
  platform: string,
  capabilities?: PlatformCapabilitiesEnvelope["platforms"] | null
): number | null {
  const normalized = platform.toLowerCase();
  const capability = capabilities?.[normalized]?.first_comment;
  if (capability?.supported && capability.max_length) {
    return capability.max_length;
  }
  if (!capability && FIRST_COMMENT_SUPPORTED_PLATFORMS.has(normalized)) {
    if (normalized === "twitter") return 280;
    if (normalized === "instagram") return 2200;
    if (normalized === "linkedin") return 1250;
  }
  return null;
}

export function supportsThreads(
  platform: string,
  capabilities?: PlatformCapabilitiesEnvelope["platforms"] | null
): boolean {
  const normalized = platform.toLowerCase();
  const capability = capabilities?.[normalized];
  if (capability?.thread) {
    return !!capability.thread.supported;
  }
  return false;
}

export function supportsScheduling(
  platform: string,
  capabilities?: PlatformCapabilitiesEnvelope["platforms"] | null
): boolean {
  const normalized = platform.toLowerCase();
  const capability = capabilities?.[normalized];
  if (capability?.scheduling) {
    return !!capability.scheduling.supported;
  }
  return true;
}

export function canGenerateAIAssist(params: {
  mode: AIPostAssistMode | null;
  mainContent: string;
  brief: string;
  mediaCount: number;
}): boolean {
  const { mode, mainContent, brief, mediaCount } = params;
  if (!mode) return false;
  if (mode === "brief") return brief.trim().length > 0;
  if (mode === "media") return mediaCount > 0;
  if (mode === "fix_validation") return true;
  return mainContent.trim().length > 0;
}

export function buildAIPostAssistRequest(params: {
  mode: AIPostAssistMode;
  selectedProfileId: string | null;
  mainContent: string;
  selectedAccounts: SocialAccount[];
  overrides: Record<string, PlatformOverride>;
  mediaItems: MediaItem[];
  validationResult: { errors?: SocialPostValidationIssue[]; warnings?: SocialPostValidationIssue[] } | null;
  brief: string;
  objective: AIAssistObjective;
  tone: AIAssistTone;
  includeCTA: boolean;
}): AIPostAssistRequest {
  const {
    mode,
    selectedProfileId,
    mainContent,
    selectedAccounts,
    overrides,
    mediaItems,
    validationResult,
    brief,
    objective,
    tone,
    includeCTA,
  } = params;

  const mediaIds = mediaItems.filter((item) => item.mediaId).map((item) => item.mediaId!);
  const validationIssues = [
    ...(validationResult?.errors || []),
    ...(validationResult?.warnings || []),
  ];

  return {
    mode,
    profile_id: selectedProfileId || undefined,
    main_caption: mainContent,
    selected_account_ids: selectedAccounts.map((account) => account.id),
    platform_posts: selectedAccounts.map((account) => ({
      account_id: account.id,
      caption: overrides[account.id]?.caption || mainContent,
    })),
    validation_issues: mode === "fix_validation" ? validationIssues : undefined,
    media_context: mode === "media" ? buildAIAssistMediaContext(mediaItems) : undefined,
    objective: mode === "brief" ? objective : undefined,
    tone: mode === "brief" ? tone : undefined,
    brief: mode === "brief" ? brief.trim() : undefined,
    include_cta: mode === "brief" ? includeCTA : true,
    media_ids: mediaIds,
  };
}

export function buildAIAssistAccountLabels(accounts: SocialAccount[]): Record<string, string> {
  const labels: Record<string, string> = {};
  for (const account of accounts) {
    labels[account.id] = account.account_name || account.external_user_email || account.platform;
  }
  return labels;
}

export function buildAIAssistCurrentPlatformCaptions(params: {
  accounts: SocialAccount[];
  overrides: Record<string, PlatformOverride>;
  mainContent: string;
}): Record<string, string> {
  const { accounts, overrides, mainContent } = params;
  const captions: Record<string, string> = {};
  for (const account of accounts) {
    captions[account.id] = overrides[account.id]?.caption || mainContent;
  }
  return captions;
}

export function buildAIAssistCurrentFirstComments(params: {
  accounts: SocialAccount[];
  overrides: Record<string, PlatformOverride>;
}): Record<string, string> {
  const { accounts, overrides } = params;
  const comments: Record<string, string> = {};
  for (const account of accounts) {
    comments[account.id] = overrides[account.id]?.firstComment || "";
  }
  return comments;
}

function buildAIAssistMediaContext(mediaItems: MediaItem[]): NonNullable<AIPostAssistRequest["media_context"]> {
  return mediaItems.map((item) => ({
    media_id: item.mediaId || undefined,
    filename: item.file.name,
    content_type: item.file.type,
    duration_sec: typeof item.durationSec === "number" ? item.durationSec : null,
    width: typeof item.videoWidth === "number" ? item.videoWidth : null,
    height: typeof item.videoHeight === "number" ? item.videoHeight : null,
  }));
}
