const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Types

export interface Profile {
  id: string;
  workspace_id: string;
  name: string;
  account_count?: number;
  created_at: string;
  updated_at: string;
  // Hosted Connect profile branding
  branding_logo_url?: string;
  branding_display_name?: string;
  branding_primary_color?: string;
  branding_hide_powered_by?: boolean;
}

export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  environment: "production" | "test";
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
}

export interface ApiKeyCreateResponse {
  id: string;
  name: string;
  key: string;
  prefix: string;
  environment: string;
  created_at: string;
}

export interface CliSetupTokenResponse {
  setup_token: string;
  client: string;
  key_name: string;
  expires_at: string;
  command: string;
  recommended_prompt: string;
}

export interface WebhookSubscription {
  id: string;
  name: string;
  url: string;
  events: string[];
  active: boolean;
  secret_preview: string;
  created_at: string;
}

export interface WebhookCreateResponse extends WebhookSubscription {
  secret: string;
}

export interface ApiResponse<T> {
  data: T;
  meta?: {
    total?: number;
    limit?: number;
    has_more?: boolean;
    next_cursor?: string;
  };
  request_id?: string;
}

export type ErrorSource = "unipost" | "platform" | "worker" | "unknown";
export type ErrorTemporality = "temporary" | "permanent" | "unknown";
export type RetryState = "not_retriable" | "scheduled" | "running" | "exhausted" | "blocked" | "manual_only" | "unknown";

export interface ProviderError {
  provider?: string;
  http_status?: number;
  code?: string;
  subcode?: string;
  type?: string;
  reason?: string;
  domain?: string;
  quota_limit?: string;
  quota_location?: string;
  is_transient?: boolean;
}

export interface RetryPolicy {
  is_retriable: boolean;
  will_retry: boolean;
  retry_state: RetryState;
  next_run_at?: string;
  attempts_made?: number;
  max_attempts?: number;
  attempts_remaining?: number;
  manual_retry_allowed: boolean;
  reason?: string;
}

export interface ApiError {
  error: {
    code: string;
    normalized_code?: string;
    message: string;
    hint?: string;
    next_action?: string;
    is_retriable?: boolean;
    error_source?: ErrorSource;
    error_temporality?: ErrorTemporality;
    provider_error?: ProviderError;
    retry_policy?: RetryPolicy;
    docs_url?: string;
    issues?: SocialPostValidationIssue[];
  };
  request_id?: string;
}

// ApiFetchError is the Error subtype thrown by request() when the API
// returns a non-2xx response. It carries the HTTP status and the
// server-provided error code so callers can branch on
// well-known values like "NEEDS_RECONNECT" or "VALIDATION_ERROR"
// without string-matching the error message.
export interface ApiFetchError extends Error {
  status?: number;
  code?: string;
  rawCode?: string;
  requestId?: string;
  hint?: string;
  nextAction?: string;
  isRetriable?: boolean;
  errorSource?: ErrorSource;
  errorTemporality?: ErrorTemporality;
  providerError?: ProviderError;
  retryPolicy?: RetryPolicy;
  docsUrl?: string;
  issues?: SocialPostValidationIssue[];
}

export interface CreateSocialPostPayload {
  caption?: string;
  account_ids?: string[];
  media_urls?: string[];
  scheduled_at?: string;
  status?: "draft";
  platform_posts?: Array<{
    account_id: string;
    caption: string;
    media_urls?: string[];
    media_ids?: string[];
    platform_options?: Record<string, unknown>;
    in_reply_to?: string;
    thread_position?: number;
    first_comment?: string;
  }>;
}

export interface SocialPostValidationIssue {
  platform_post_index?: number;
  account_id?: string;
  platform?: string;
  field: string;
  code: string;
  message: string;
  hint?: string;
  next_action?: string;
  actual?: unknown;
  limit?: unknown;
  severity: "error" | "warning";
}

export interface SocialPostValidationResult {
  valid: boolean;
  errors: SocialPostValidationIssue[];
  warnings: SocialPostValidationIssue[];
}

export type AIPostAssistMode =
  | "brief"
  | "improve"
  | "adapt"
  | "media"
  | "fix_validation";

export interface AIPostAssistSuggestion {
  request_id: string;
  mode: AIPostAssistMode;
  summary?: string;
  main_caption?: string;
  platform_captions?: Array<{
    account_id: string;
    platform: string;
    caption: string;
    reason?: string;
  }>;
  hashtags?: string[];
  warnings?: string[];
  first_comment_suggestions?: Array<{
    account_id: string;
    text: string;
  }>;
}

export interface AIPostAssistRequest {
  mode: AIPostAssistMode;
  profile_id?: string;
  main_caption?: string;
  selected_account_ids?: string[];
  platform_posts?: Array<{
    account_id: string;
    caption: string;
  }>;
  validation_issues?: SocialPostValidationIssue[];
  media_context?: Array<{
    media_id?: string;
    filename: string;
    content_type: string;
    duration_sec?: number | null;
    width?: number | null;
    height?: number | null;
  }>;
  objective?: "awareness" | "engagement" | "clicks" | "sales";
  tone?: "professional" | "friendly" | "bold" | "playful";
  brief?: string;
  include_cta?: boolean;
  media_ids?: string[];
}

export interface PlatformPublishCapability {
  display_name: string;
  text: {
    max_length: number;
    min_length: number;
    required: boolean;
    supports_threads?: boolean;
  };
  thread: {
    supported: boolean;
    max_items?: number;
  };
  scheduling: {
    supported: boolean;
  };
  first_comment: {
    supported: boolean;
    max_length?: number;
  };
}

export interface PlatformCapabilitiesEnvelope {
  schema_version: string;
  platforms: Record<string, PlatformPublishCapability>;
}

export interface IntegrationLog {
  id: number;
  workspace_id: string;
  ts: string;
  level: "debug" | "info" | "warn" | "error";
  status: "success" | "warning" | "error";
  category: "publishing" | "api_request" | "oauth" | "webhook" | "system";
  action: string;
  source: "api" | "dashboard" | "worker" | "webhook" | "oauth";
  message: string;
  request_id?: string;
  trace_id?: string;
  actor_user_id?: string;
  actor_api_key_id?: string;
  profile_id?: string;
  social_account_id?: string;
  post_id?: string;
  platform_post_id?: string;
  platform?: string;
  endpoint?: string;
  method?: string;
  http_status_code?: number;
  remote_status_code?: number;
  duration_ms?: number;
  error_code?: string;
  metadata?: Record<string, unknown> | null;
  request_payload?: Record<string, unknown> | null;
  response_payload?: Record<string, unknown> | null;
}

export interface IntegrationLogListParams {
  q?: string;
  workspace_id?: string;
  owner_email?: string;
  category?: string;
  action?: string;
  source?: string;
  level?: string;
  status?: string;
  platform?: string;
  profile_id?: string;
  social_account_id?: string;
  post_id?: string;
  request_id?: string;
  error_code?: string;
  from?: string;
  to?: string;
  limit?: number;
}

export interface AdminIntegrationLog extends IntegrationLog {
  workspace_name?: string;
  owner_email?: string;
  plan_id?: string;
}

export interface AdminSupportBundle {
  id: string;
  workspace_id: string;
  workspace_name?: string;
  owner_email?: string;
  plan_id?: string;
  run_id: string;
  schema_version: string;
  cli_version?: string;
  summary: string;
  report_markdown?: string;
  finding_count: number;
  recent_error_count: number;
  created_at: string;
}

export interface AdminSupportBundleListParams {
  workspace_id?: string;
  owner_email?: string;
  q?: string;
  limit?: number;
}

export type AdminSearchHistoryFieldKey =
  | "admin.logs.q"
  | "admin.logs.workspace_id"
  | "admin.logs.owner_email"
  | "admin.errors.search"
  | "admin.api_metrics.workspace_id"
  | "admin.email.search"
  | "admin.posts.search"
  | "admin.users.search";

export interface AdminSearchHistoryItem {
  id: string;
  field_key: AdminSearchHistoryFieldKey;
  value: string;
  usage_count: number;
  last_used_at: string;
}

export type AdminChangelogAction = "publish" | "save" | "discard";

export interface AdminChangelogLink {
  label: string;
  href: string;
}

export interface AdminChangelogSDKVersion {
  ecosystem: "npm" | "pip" | "go" | "maven";
  packageName: string;
  version: string;
  href: string;
  installCommand?: string;
}

export interface AdminChangelogReleaseCandidate {
  id: string;
  date: string;
  displayDate?: string;
  title: string;
  summary: string;
  category: "api" | "sdk" | "dashboard" | "platform" | "dx" | "reliability";
  impact: "new" | "improved" | "changed" | "fixed";
  isBreaking: boolean;
  sdkVersions?: AdminChangelogSDKVersion[];
  links: AdminChangelogLink[];
  sourceLinks: AdminChangelogLink[];
  confidence?: string;
  whyUserVisible: string;
  excludedCommits?: string[];
}

export interface AdminChangelogCandidatePayload {
  hasCandidate: boolean;
  candidate?: AdminChangelogReleaseCandidate;
  reason?: string;
  excludedCommits?: string[];
}

export interface AdminChangelogCandidate {
  id: string;
  source_hash: string;
  status: "pending" | "saved" | "discarded" | "publishing" | "published" | "failed";
  payload: AdminChangelogCandidatePayload;
  window_start: string;
  window_end: string;
  discord_message_id?: string;
  action_request_id?: string;
  workflow_run_url?: string;
  acted_by_admin_id?: string;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface AdminChangelogCandidatePreview {
  candidate: AdminChangelogCandidate;
  action: AdminChangelogAction;
}

export interface AdminChangelogActionResult {
  candidate_id: string;
  status: AdminChangelogCandidate["status"];
  action: AdminChangelogAction;
  message: string;
  workflow_run_url?: string;
}

// Client

function formatIssueValue(value: unknown): string {
  if (value === null) return "null";
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

export function formatApiValidationIssue(issue: SocialPostValidationIssue): string {
  const target = [issue.platform, issue.field].filter(Boolean).join(" ");
  const base = issue.message || issue.code || issue.field || "Validation issue";
  const facts: string[] = [];
  if (issue.actual !== undefined) facts.push(`current: ${formatIssueValue(issue.actual)}`);
  if (issue.limit !== undefined) facts.push(`limit: ${formatIssueValue(issue.limit)}`);
  if (issue.hint) facts.push(`next: ${issue.hint}`);
  const suffix = facts.length > 0 ? ` (${facts.join("; ")})` : "";
  return target ? `${target}: ${base}${suffix}` : `${base}${suffix}`;
}

export function createApiFetchError(status: number, body: unknown): ApiFetchError {
  const envelope = body as Partial<ApiError>;
  const apiError = envelope?.error;
  const issues = Array.isArray(apiError?.issues) ? apiError.issues : undefined;
  let message = apiError?.message || `Request failed: ${status}`;
  if (issues && issues.length > 0) {
    const details = issues.map(formatApiValidationIssue).filter(Boolean).join("; ");
    if (details) message += `: ${details}`;
  }

  const thrown = new Error(message) as ApiFetchError;
  thrown.status = status;
  thrown.rawCode = apiError?.code;
  if (apiError?.normalized_code || apiError?.code) {
    thrown.code = apiError.normalized_code || apiError.code;
  }
  thrown.requestId = envelope?.request_id;
  thrown.hint = apiError?.hint;
  thrown.nextAction = apiError?.next_action;
  thrown.isRetriable = apiError?.is_retriable;
  thrown.errorSource = apiError?.error_source;
  thrown.errorTemporality = apiError?.error_temporality;
  thrown.providerError = apiError?.provider_error;
  thrown.retryPolicy = apiError?.retry_policy;
  thrown.docsUrl = apiError?.docs_url;
  thrown.issues = issues;
  return thrown;
}

async function request<T>(
  path: string,
  token: string,
  options?: RequestInit
): Promise<T> {
  const isFormDataBody = options?.body instanceof FormData;
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      ...(isFormDataBody ? {} : { "Content-Type": "application/json" }),
      Authorization: `Bearer ${token}`,
      ...options?.headers,
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw createApiFetchError(res.status, body);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json();
}

async function requestPublic<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, options);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw createApiFetchError(res.status, body);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json();
}

// Workspaces

export interface Workspace {
  id: string;
  name: string;
  per_account_monthly_limit: number | null;
  usage_modes: string[];
  custom_platform_slot: string | null;
  created_at: string;
  updated_at: string;
}

export async function getWorkspace(
  token: string
): Promise<ApiResponse<Workspace>> {
  return request(`/v1/workspace`, token);
}

export async function updateWorkspace(
  token: string,
  data: { name?: string; per_account_monthly_limit?: number | null; custom_platform_slot?: string | null }
): Promise<ApiResponse<Workspace>> {
  return request(`/v1/workspace`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

// Profiles (dashboard / Clerk auth).

export async function listProfiles(
  token: string
): Promise<ApiResponse<Profile[]>> {
  return request("/v1/profiles", token);
}

export async function getPlatformCapabilities(): Promise<ApiResponse<PlatformCapabilitiesEnvelope>> {
  return requestPublic("/v1/platforms/capabilities");
}

export async function getProfile(
  token: string,
  id: string
): Promise<ApiResponse<Profile>> {
  return request(`/v1/profiles/${id}`, token);
}

export async function createProfile(
  token: string,
  data: {
    name: string;
    branding_logo_url?: string;
    branding_display_name?: string;
    branding_primary_color?: string;
    branding_hide_powered_by?: boolean;
  }
): Promise<ApiResponse<Profile>> {
  return request("/v1/profiles", token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateProfile(
  token: string,
  id: string,
  data: {
    name?: string;
    branding_logo_url?: string;
    branding_display_name?: string;
    branding_primary_color?: string;
    branding_hide_powered_by?: boolean;
  }
): Promise<ApiResponse<Profile>> {
  return request(`/v1/profiles/${id}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function uploadProfileLogo(
  token: string,
  id: string,
  file: File
): Promise<ApiResponse<Profile>> {
  const formData = new FormData();
  formData.append("file", file);
  return request(`/v1/profiles/${id}/branding/logo`, token, {
    method: "POST",
    body: formData,
  });
}

export async function deleteProfileLogo(
  token: string,
  id: string
): Promise<ApiResponse<Profile>> {
  return request(`/v1/profiles/${id}/branding/logo`, token, { method: "DELETE" });
}

export async function deleteProfile(
  token: string,
  id: string
): Promise<void> {
  return request(`/v1/profiles/${id}`, token, { method: "DELETE" });
}

// Platform credentials (workspace-scoped OAuth app credentials)

export interface PlatformCredential {
  platform: string;
  client_id: string;
  created_at: string;
}

export async function listPlatformCredentials(
  token: string,
): Promise<ApiResponse<PlatformCredential[]>> {
  return request(`/v1/platform-credentials`, token);
}

export async function createPlatformCredential(
  token: string,
  data: { platform: string; client_id: string; client_secret: string }
): Promise<ApiResponse<PlatformCredential>> {
  return request(`/v1/platform-credentials`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function deletePlatformCredential(
  token: string,
  platform: string
): Promise<void> {
  return request(
    `/v1/platform-credentials/${platform}`,
    token,
    { method: "DELETE" }
  );
}

// API Keys (workspace-scoped)

export async function listApiKeys(
  token: string,
): Promise<ApiResponse<ApiKey[]>> {
  return request(`/v1/api-keys`, token);
}

export async function createApiKey(
  token: string,
  data: { name: string; environment?: string; expires_at?: string }
): Promise<ApiResponse<ApiKeyCreateResponse>> {
  return request(`/v1/api-keys`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function createCliSetupToken(
  token: string,
  data: { client: "terminal" | "codex" | "claude-code" | "cursor" | "windsurf" }
): Promise<ApiResponse<CliSetupTokenResponse>> {
  return request(`/v1/cli/setup-tokens`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function revokeApiKey(
  token: string,
  keyId: string
): Promise<void> {
  return request(`/v1/api-keys/${keyId}`, token, {
    method: "DELETE",
  });
}

// Developer webhooks (workspace-scoped)

export async function listWebhooks(
  token: string,
): Promise<ApiResponse<WebhookSubscription[]>> {
  return request(`/v1/webhooks`, token);
}

export async function createWebhook(
  token: string,
  data: { name: string; url: string; events: string[]; active?: boolean; secret?: string }
): Promise<ApiResponse<WebhookCreateResponse>> {
  return request(`/v1/webhooks`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateWebhook(
  token: string,
  webhookId: string,
  data: { name?: string; url?: string; events?: string[]; active?: boolean }
): Promise<ApiResponse<WebhookSubscription>> {
  return request(`/v1/webhooks/${webhookId}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function rotateWebhookSecret(
  token: string,
  webhookId: string
): Promise<ApiResponse<WebhookCreateResponse>> {
  return request(`/v1/webhooks/${webhookId}/rotate`, token, {
    method: "POST",
  });
}

export async function deleteWebhook(
  token: string,
  webhookId: string
): Promise<void> {
  return request(`/v1/webhooks/${webhookId}`, token, {
    method: "DELETE",
  });
}

// Social Accounts (profile-scoped)

export interface SocialAccount {
  id: string;
  profile_id: string;
  profile_name?: string;
  platform: string;
  account_name: string | null;
  external_account_id?: string;
  account_avatar_url?: string | null;
  connected_at: string;
  status: "active" | "reconnect_required" | "disconnected";
  connection_type: "byo" | "managed";
  external_user_id?: string;
  external_user_email?: string;
  scope?: string[];
}

export async function listSocialAccounts(
  token: string,
  profileId: string,
  filters?: { external_user_id?: string; platform?: string; include_disconnected?: boolean }
): Promise<ApiResponse<SocialAccount[]>> {
  const qs = new URLSearchParams();
  if (filters?.external_user_id) qs.set("external_user_id", filters.external_user_id);
  if (filters?.platform) qs.set("platform", filters.platform);
  if (filters?.include_disconnected) qs.set("include_disconnected", "1");
  const suffix = qs.toString() ? `?${qs}` : "";
  return request(`/v1/profiles/${profileId}/accounts${suffix}`, token);
}

export async function connectSocialAccount(
  token: string,
  profileId: string,
  data: { platform: string; credentials: Record<string, string> }
): Promise<ApiResponse<SocialAccount>> {
  return request(`/v1/profiles/${profileId}/accounts/connect`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// Facebook Page Picker types + helpers. Lives in lib/api alongside the
// other account-connection helpers so callers find it next to
// listSocialAccounts / connectSocialAccount. The backend endpoint is
// gated by ENABLE_FACEBOOK_PAGES — expect 403 FACEBOOK_DISABLED until
// the flag is flipped on.
export interface PendingFacebookPage {
  id: string;
  name: string;
  category: string;
  picture_url: string;
  tasks: string[];
  business_id?: string;
  business_name?: string;
  business_relationship?: "owned" | "client" | "page_business" | string;
  // can_publish reflects whether the current user's admin role on
  // this Page includes content-publishing permissions. When false,
  // the picker should still show the row but disable selection so
  // the user understands why they can't connect it.
  can_publish: boolean;
}

export interface PendingFacebookBusiness {
  id: string;
  name: string;
}

export interface PendingConnection {
  id: string;
  platform: string;
  profile_id: string;
  meta_user: { meta_user_id: string };
  pages: PendingFacebookPage[];
  businesses: PendingFacebookBusiness[];
  expires_at: string;
}

export async function getPendingConnection(
  token: string,
  pendingId: string
): Promise<ApiResponse<PendingConnection>> {
  return request(
    `/v1/pending-connections/${pendingId}`,
    token
  );
}

export async function finalizePendingConnection(
  token: string,
  pendingId: string,
  pageIds: string[]
): Promise<ApiResponse<{ connected_account_ids: string[]; connected_count: number }>> {
  return request(
    `/v1/pending-connections/${pendingId}/finalize`,
    token,
    { method: "POST", body: JSON.stringify({ page_ids: pageIds }) }
  );
}

export async function disconnectSocialAccount(
  token: string,
  profileId: string,
  accountId: string
): Promise<void> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}`,
    token,
    { method: "DELETE" }
  );
}

export async function dismissSocialAccount(
  token: string,
  profileId: string,
  accountId: string
): Promise<ApiResponse<{ dismissed: boolean }>> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/dismiss`,
    token,
    { method: "POST" }
  );
}

export interface PinterestBoard {
  id: string;
  name: string;
}

export async function createPinterestBoard(
  token: string,
  profileId: string,
  accountId: string,
  name: string
): Promise<ApiResponse<{ board: PinterestBoard }>> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/pinterest/boards`,
    token,
    { method: "POST", body: JSON.stringify({ name }) }
  );
}

export async function listPinterestBoards(
  token: string,
  profileId: string,
  accountId: string
): Promise<ApiResponse<{ boards: PinterestBoard[]; sandbox_mode?: boolean }>> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/pinterest/boards`,
    token
  );
}

// TikTok creator_info — required by TikTok's Content Posting API audit.
// Populates the compose UI's creator nickname, privacy options, interaction
// toggle availability, and max video length. See
// internal/handler/tiktok_creator_info.go on the API side.
export interface TikTokCreatorInfo {
  creator_avatar_url: string;
  creator_username: string;
  creator_nickname: string;
  privacy_level_options: string[];
  comment_disabled: boolean;
  duet_disabled: boolean;
  stitch_disabled: boolean;
  max_video_post_duration_sec: number;
}

export async function getTikTokCreatorInfo(
  token: string,
  profileId: string,
  accountId: string
): Promise<ApiResponse<TikTokCreatorInfo>> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/tiktok/creator-info`,
    token
  );
}

export interface AccountMetrics {
  social_account_id: string;
  platform: string;
  follower_count: number;
  following_count: number;
  post_count: number;
  platform_specific?: Record<string, unknown>;
  fetched_at: string;
}

export async function getAccountMetrics(
  token: string,
  profileId: string,
  accountId: string
): Promise<ApiResponse<AccountMetrics>> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/metrics`,
    token
  );
}

export interface TikTokProfile {
  social_account_id: string;
  platform: string;
  open_id: string;
  display_name: string;
  avatar_url: string;
  username: string;
  profile_web_link: string;
  profile_deep_link: string;
  bio_description: string;
  is_verified: boolean;
  fetched_at: string;
}

export interface TikTokVideo {
  id: string;
  title?: string;
  video_description?: string;
  cover_image_url?: string;
  share_url?: string;
  create_time?: number;
  view_count: number;
  like_count: number;
  comment_count: number;
  share_count: number;
  duration?: number;
  embed_link?: string;
}

export interface TikTokVideosResponse {
  videos: TikTokVideo[];
  cursor: number;
  has_more: boolean;
  fetched_at: string;
}

export async function getTikTokProfile(
  token: string,
  profileId: string,
  accountId: string
): Promise<ApiResponse<TikTokProfile>> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/tiktok/profile`,
    token
  );
}

export async function getTikTokVideos(
  token: string,
  profileId: string,
  accountId: string,
  opts?: { cursor?: number; limit?: number }
): Promise<ApiResponse<TikTokVideosResponse>> {
  const qs = new URLSearchParams();
  if (opts?.cursor) qs.set("cursor", String(opts.cursor));
  if (opts?.limit) qs.set("limit", String(opts.limit));
  const suffix = qs.toString() ? `?${qs}` : "";
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/tiktok/videos${suffix}`,
    token
  );
}

export interface InstagramProfile {
  social_account_id: string;
  platform: "instagram";
  id: string;
  username: string;
  profile_picture_url: string;
  followers_count: number;
  follows_count: number;
  media_count: number;
  fetched_at: string;
}

export interface InstagramMedia {
  id: string;
  caption?: string;
  media_type?: string;
  media_url?: string;
  thumbnail_url?: string;
  permalink?: string;
  timestamp?: string;
  like_count: number;
  comments_count: number;
  reach: number;
  shares: number;
  saves: number;
  metrics_unavailable_reason?: string;
}

export interface InstagramMediaResponse {
  media: InstagramMedia[];
  fetched_at: string;
  limit: number;
}

export async function getInstagramProfile(
  token: string,
  profileId: string,
  accountId: string
): Promise<ApiResponse<InstagramProfile>> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/instagram/profile`,
    token
  );
}

export async function getInstagramMedia(
  token: string,
  profileId: string,
  accountId: string,
  opts?: { limit?: number }
): Promise<ApiResponse<InstagramMediaResponse>> {
  const qs = new URLSearchParams();
  if (opts?.limit) qs.set("limit", String(opts.limit));
  const suffix = qs.toString() ? `?${qs}` : "";
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/instagram/media${suffix}`,
    token
  );
}

export interface ThreadsProfile {
  social_account_id: string;
  platform: "threads";
  id: string;
  username: string;
  threads_profile_picture_url: string;
  fetched_at: string;
}

export interface ThreadsPost {
  id: string;
  text?: string;
  media_type?: string;
  media_url?: string;
  permalink?: string;
  timestamp?: string;
  views: number;
  likes: number;
  replies: number;
  reposts: number;
  quotes: number;
  shares: number;
  metrics_unavailable_reason?: string;
}

export interface ThreadsPostsResponse {
  posts: ThreadsPost[];
  fetched_at: string;
  limit: number;
}

export async function getThreadsProfile(
  token: string,
  profileId: string,
  accountId: string
): Promise<ApiResponse<ThreadsProfile>> {
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/threads/profile`,
    token
  );
}

export async function getThreadsPosts(
  token: string,
  profileId: string,
  accountId: string,
  opts?: { limit?: number }
): Promise<ApiResponse<ThreadsPostsResponse>> {
  const qs = new URLSearchParams();
  if (opts?.limit) qs.set("limit", String(opts.limit));
  const suffix = qs.toString() ? `?${qs}` : "";
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/threads/posts${suffix}`,
    token
  );
}

export interface FacebookPageProfile {
  id: string;
  name: string;
  category: string;
  username: string;
  picture_url: string;
  link: string;
  about: string;
  verification_status: string;
  fan_count: number;
  followers_count: number;
}

export interface FacebookPageInsights {
  follows: number;
  impressions: number;
  views?: number;
  post_engagements: number;
  below_100_likes_notice: boolean;
  since: string;
  until: string;
}

export interface FacebookPageAnalyticsPost {
  id: string;
  message: string;
  created_time: string;
  permalink_url: string;
  full_picture: string;
  media_url: string;
  media_type: string;
  likes: number;
  comments: number;
  shares: number;
  clicks: number;
  video_views: number;
  engagement_total: number;
  metrics_unavailable_reason?: string;
}

export interface FacebookPageAnalytics {
  social_account_id: string;
  platform: "facebook";
  page: FacebookPageProfile | null;
  insights?: FacebookPageInsights;
  insights_error?: string;
  posts: FacebookPageAnalyticsPost[];
  fetched_at: string;
  post_limit: number;
  granted_scopes?: string[];
  required_scopes: string[];
  recommended_scopes: string[];
}

export async function getFacebookPageAnalytics(
  token: string,
  profileId: string,
  accountId: string,
  opts?: { days?: number; limit?: number }
): Promise<ApiResponse<FacebookPageAnalytics>> {
  const qs = new URLSearchParams();
  if (opts?.days) qs.set("days", String(opts.days));
  if (opts?.limit) qs.set("limit", String(opts.limit));
  const suffix = qs.toString() ? `?${qs}` : "";
  return request(
    `/v1/profiles/${profileId}/accounts/${accountId}/facebook/page-analytics${suffix}`,
    token
  );
}

export async function getOAuthConnectURL(
  token: string,
  profileId: string,
  platform: string,
  redirectUrl: string
): Promise<ApiResponse<{ auth_url: string }>> {
  const params = new URLSearchParams({ redirect_url: redirectUrl });
  return request(
    `/v1/profiles/${profileId}/oauth/connect/${platform}?${params}`,
    token
  );
}

// Social Posts (workspace-scoped)

export interface SocialPostResult {
  // id is the social_post_results row ID — used by the retry endpoint
  // to target a specific failed row. Always present on API responses
  // (falls back to the empty string only on stale cached objects).
  id: string;
  social_account_id: string;
  platform?: string;
  account_name?: string;
  status: string;
  external_id?: string;
  // URL is the platform's canonical post URL (e.g. Threads permalink
  // fetched from the Graph API). When present, the dashboard uses it
  // directly for "View post" links instead of constructing one from
  // external_id. Required for platforms like Threads where the public
  // URL uses shortcodes that aren't derivable from the numeric post ID.
  url?: string;
  error_message?: string;
  error_code?: string;
  failure_stage?: string;
  platform_error_code?: string;
  is_retriable?: boolean;
  next_action?: string;
  error_source?: ErrorSource;
  error_temporality?: ErrorTemporality;
  provider_error?: ProviderError;
  retry_policy?: RetryPolicy;
  published_at?: string;
  // Serialized curl dump of every failing HTTP request the adapter
  // made during this dispatch. Populated only when status === "failed".
  // Auth headers and token query params are redacted server-side
  // (see internal/debugrt on the API). Safe to display to the post owner.
  debug_curl?: string;
  // Snapshot of what the user actually submitted for this account —
  // per-account caption override, media, and platform-specific options.
  // Used to render a "Submitted settings" panel on the expanded post
  // view so users can review their own choices after the fact.
  submitted?: SubmittedSettings;
  // Real-time publish state refreshed server-side when the post is
  // viewed — populated for platforms with async publish lifecycles
  // (TikTok, Facebook video). Shape is platform-specific; the
  // Facebook processing panel reads video_status / *_phase_status.
  publish_status?: Record<string, unknown>;
}

export interface SubmittedSettings {
  caption?: string;
  media_urls?: string[];
  media_ids?: string[];
  platform_options?: Record<string, unknown>;
  first_comment?: string;
  in_reply_to?: string;
  thread_position?: number;
}

export interface EditablePlatformPost {
  account_id: string;
  caption: string;
  media_urls?: string[];
  media_ids?: string[];
  platform_options?: Record<string, unknown>;
  first_comment?: string;
  in_reply_to?: string;
  thread_position?: number;
}

export interface SocialPost {
  id: string;
  caption: string | null;
  media_urls?: string[];
  status: string;
  execution_mode?: string;
  queued_results_count?: number;
  active_job_count?: number;
  retrying_count?: number;
  dead_count?: number;
  scheduled_at?: string;
  created_at: string;
  published_at?: string;
  archived_at?: string;
  // "ui" = dashboard publish, "api" = external API key publish.
  // Stamped at row creation and immutable thereafter.
  source: "ui" | "api";
  // Distinct profile_ids the post landed under, derived from its
  // target social_accounts. A single post can target accounts across
  // multiple profiles. Empty when the post was created before
  // migration 043 and hasn't published yet.
  profile_ids: string[];
  // Derived from stored post metadata so the UI can still show target
  // platforms even when no result rows have been persisted yet.
  target_platforms?: string[];
  // Stored request entries used by draft/scheduled edit surfaces to
  // restore the same per-account form the user created.
  platform_posts?: EditablePlatformPost[];
  results?: SocialPostResult[];
}

export interface SocialPostSummaryResult {
  external_id?: string;
}

export interface SocialPostSummary {
  id: string;
  caption: string | null;
  media_urls?: string[];
  status: string;
  created_at: string;
  published_at?: string;
  results?: SocialPostSummaryResult[];
}

export interface PostDeliveryJob {
  id: string;
  post_id: string;
  social_post_result_id: string;
  social_account_id: string;
  platform: string;
  kind: "dispatch" | "retry";
  state: "pending" | "running" | "retrying" | "succeeded" | "failed" | "dead" | "cancelled";
  attempts: number;
  max_attempts: number;
  failure_stage?: string;
  error_code?: string;
  platform_error_code?: string;
  last_error?: string;
  next_run_at?: string;
  last_attempt_at?: string;
  created_at: string;
  updated_at: string;
}

export interface PostDeliveryJobsSummary {
  pending_count: number;
  running_count: number;
  retrying_count: number;
  dead_count: number;
  recovered_today_count: number;
}

export interface SocialPostQueueResponse {
  post: SocialPost;
  jobs: PostDeliveryJob[];
}

export async function listSocialPosts(
  token: string,
): Promise<ApiResponse<SocialPost[]>> {
  return request(`/v1/posts`, token);
}

export async function listSocialPostSummaries(
  token: string,
): Promise<ApiResponse<SocialPostSummary[]>> {
  return request(`/v1/posts/summaries`, token);
}

export async function archiveSocialPost(
  token: string,
  postId: string
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/posts/${postId}`, token, {
    method: "PATCH",
    body: JSON.stringify({ archived: true }),
  });
}

export async function restoreSocialPost(
  token: string,
  postId: string
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/posts/${postId}`, token, {
    method: "PATCH",
    body: JSON.stringify({ archived: false }),
  });
}

// retrySocialPostResult kicks off a per-platform retry on a single
// failed social_post_result row. The server overwrites the same row
// (no new rows per retry) and returns the updated result. Callers
// usually refetch the parent post afterward because the parent's
// status can flip to "published" or "partial" as a side effect.
export async function retrySocialPostResult(
  token: string,
  postId: string,
  resultId: string
): Promise<ApiResponse<SocialPostResult>> {
  return request(
    `/v1/posts/${postId}/results/${resultId}/retry`,
    token,
    { method: "POST" }
  );
}

export async function deleteSocialPost(
  token: string,
  postId: string
): Promise<ApiResponse<{ deleted: boolean }>> {
  return request(`/v1/posts/${postId}`, token, {
    method: "DELETE",
  });
}

export async function listPostDeliveryJobs(
  token: string,
): Promise<ApiResponse<PostDeliveryJob[]>> {
  return request(`/v1/post-delivery-jobs`, token);
}

export async function getPostDeliveryJobsSummary(
  token: string,
): Promise<ApiResponse<PostDeliveryJobsSummary>> {
  return request(`/v1/post-delivery-jobs/summary`, token);
}

export async function getSocialPostQueue(
  token: string,
  postId: string
): Promise<ApiResponse<SocialPostQueueResponse>> {
  return request(`/v1/posts/${postId}/queue`, token);
}

export async function retryPostDeliveryJobNow(
  token: string,
  jobId: string
): Promise<ApiResponse<PostDeliveryJob>> {
  return request(`/v1/post-delivery-jobs/${jobId}/retry`, token, {
    method: "POST",
  });
}

export async function cancelPostDeliveryJob(
  token: string,
  jobId: string
): Promise<ApiResponse<PostDeliveryJob>> {
  return request(`/v1/post-delivery-jobs/${jobId}/cancel`, token, {
    method: "POST",
  });
}

export async function dismissPostDeliveryJob(
  token: string,
  jobId: string
): Promise<ApiResponse<PostDeliveryJob>> {
  return request(`/v1/post-delivery-jobs/${jobId}/dismiss`, token, {
    method: "POST",
  });
}

// Billing (workspace-scoped)

export interface BillingInfo {
  plan: string;
  plan_name: string;
  status: string;
  usage: number;
  limit: number;
  percentage: number;
  period: string;
  warning?: string;
  cancel_at_period_end: boolean;
  trial_eligible: boolean;
}

export interface Plan {
  id: string;
  name: string;
  price_cents: number;
  post_limit: number;
}

export async function getBilling(
  token: string,
): Promise<ApiResponse<BillingInfo>> {
  return request(`/v1/billing`, token);
}

export async function listIntegrationLogs(
  token: string,
  params?: IntegrationLogListParams
): Promise<ApiResponse<IntegrationLog[]>> {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params || {})) {
    if (value === undefined || value === null || value === "" || value === "all") continue;
    qs.set(key, String(value));
  }
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  return request(`/v1/logs${suffix}`, token);
}

export async function getIntegrationLog(
  token: string,
  id: number | string
): Promise<ApiResponse<IntegrationLog>> {
  return request(`/v1/logs/${id}`, token);
}

export async function listAdminIntegrationLogs(
  token: string,
  params?: IntegrationLogListParams
): Promise<ApiResponse<AdminIntegrationLog[]>> {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params || {})) {
    if (value === undefined || value === null || value === "" || value === "all") continue;
    qs.set(key, String(value));
  }
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  return request(`/v1/admin/logs${suffix}`, token);
}

export async function getAdminIntegrationLog(
  token: string,
  id: number | string
): Promise<ApiResponse<AdminIntegrationLog>> {
  return request(`/v1/admin/logs/${id}`, token);
}

export async function listAdminSearchHistory(
  token: string,
  fieldKey: AdminSearchHistoryFieldKey,
  limit = 8,
): Promise<ApiResponse<AdminSearchHistoryItem[]>> {
  const qs = new URLSearchParams();
  qs.set("field_key", fieldKey);
  qs.set("limit", String(limit));
  return request(`/v1/admin/search-history?${qs.toString()}`, token);
}

export async function saveAdminSearchHistory(
  token: string,
  fieldKey: AdminSearchHistoryFieldKey,
  value: string,
): Promise<ApiResponse<AdminSearchHistoryItem>> {
  return request("/v1/admin/search-history", token, {
    method: "POST",
    body: JSON.stringify({ field_key: fieldKey, value }),
  });
}

export async function deleteAdminSearchHistory(
  token: string,
  id: string,
): Promise<ApiResponse<{ deleted: boolean }>> {
  return request(`/v1/admin/search-history/${encodeURIComponent(id)}`, token, {
    method: "DELETE",
  });
}

export async function listAdminSupportBundles(
  token: string,
  params?: AdminSupportBundleListParams
): Promise<ApiResponse<AdminSupportBundle[]>> {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params || {})) {
    if (value === undefined || value === null || value === "") continue;
    qs.set(key, String(value));
  }
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  return request(`/v1/admin/support-bundles${suffix}`, token);
}

export async function getAdminSupportBundle(
  token: string,
  id: string
): Promise<ApiResponse<AdminSupportBundle>> {
  return request(`/v1/admin/support-bundles/${encodeURIComponent(id)}`, token);
}

export async function createCheckout(
  token: string,
  planId: string
): Promise<ApiResponse<{ checkout_url: string }>> {
  return request(`/v1/billing/checkout`, token, {
    method: "POST",
    body: JSON.stringify({ plan_id: planId }),
  });
}

export async function createPortal(
  token: string,
): Promise<ApiResponse<{ portal_url: string }>> {
  return request(`/v1/billing/portal`, token, {
    method: "POST",
  });
}

export async function listPlans(): Promise<ApiResponse<Plan[]>> {
  const res = await fetch(`${API_URL}/v1/plans`);
  return res.json();
}

// API Limits / runtime safety caps. Read-only — values come from
// the same internal/ratelimit/plans.go map the API actually
// enforces, so the page never drifts from reality. queue_depth_current
// is a snapshot at request time; the page polls to refresh.
export interface ApiLimits {
  plan_id: string;
  request_rate_per_min: number;
  request_burst: number;
  enqueue_posts_per_min: number;
  enqueue_posts_per_5min: number;
  queue_depth_cap: number;
  managed_user_depth_cap: number;
  queue_depth_current: number;
  per_platform_daily_cap: Record<string, number>;
  plan_allows_twitter: boolean;
  plan_allows_inbox: boolean;
  plan_allows_analytics: boolean;
  plan_allows_white_label: boolean;
  plan_allows_hosted_connect_branding: boolean;
  plan_allows_hide_powered_by: boolean;
  white_label_platform_limit: number;
  custom_platform_slot: string | null;
  max_profiles: number; // -1 = unlimited
  current_profiles: number;
  max_members: number; // -1 = unlimited
  current_members: number;
  max_api_keys: number; // -1 = unlimited
  current_api_keys: number;
  max_webhooks: number; // -1 = unlimited
  current_webhooks: number;
  max_managed_accounts: number; // -1 = unlimited
  current_managed_accounts: number;
  max_managed_users: number; // -1 = unlimited
  current_managed_users: number;
}

export async function getApiLimits(token: string): Promise<ApiResponse<ApiLimits>> {
  return request(`/v1/limits`, token);
}

// friendlyRateLimitMessage upgrades a generic 429 Error to a
// human-readable message branched on the limiter that fired
// (rate / enqueue / depth). Handlers that catch a publish failure
// call this first and fall back to the raw error message when it
// returns null. Keeping the strings here so all dashboard surfaces
// — drawer, queue page, future re-publish UIs — stay consistent.
export function friendlyRateLimitMessage(err: unknown): string | null {
  if (!(err instanceof Error)) return null;
  const e = err as ApiFetchError;
  if (e.status !== 429) return null;
  switch (e.code) {
    case "rate_limited":
      return "You're publishing too quickly. Wait a few seconds and retry.";
    case "enqueue_rate_limited":
      return "This workspace is creating posts too quickly. Slow down and retry shortly.";
    case "queue_depth_exceeded":
      return "Your queue has too many active deliveries. Wait for them to drain or upgrade your plan.";
    default:
      return "Too many requests. Please retry shortly.";
  }
}

export async function createSocialPost(
  token: string,
  data: CreateSocialPostPayload
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/posts`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function validateSocialPost(
  token: string,
  data: CreateSocialPostPayload
): Promise<ApiResponse<SocialPostValidationResult>> {
  return request(`/v1/posts/validate`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function postAssistAIDraft(
  token: string,
  data: AIPostAssistRequest
): Promise<ApiResponse<AIPostAssistSuggestion>> {
  return request(`/v1/ai/post-assist`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// Media upload — two-step: POST returns presigned URL, then PUT to R2

export interface MediaUpload {
  id: string;
  status: string;
  content_type: string;
  size_bytes: number;
  upload_url: string;
  expires_at: string;
}

export async function getMedia(
  token: string,
  mediaId: string
): Promise<ApiResponse<MediaUpload>> {
  return request(`/v1/media/${mediaId}`, token);
}

export async function createMedia(
  token: string,
  data: { filename: string; content_type: string; size_bytes: number; content_hash?: string }
): Promise<ApiResponse<MediaUpload>> {
  return request(`/v1/media`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// Sprint 3 PR8: reschedule a scheduled post (only scheduled_at editable).
export async function rescheduleSocialPost(
  token: string,
  postId: string,
  scheduledAt: string
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/posts/${postId}`, token, {
    method: "PATCH",
    body: JSON.stringify({ scheduled_at: scheduledAt }),
  });
}

export async function updateSocialPost(
  token: string,
  postId: string,
  data: CreateSocialPostPayload
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/posts/${postId}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

// Sprint 3 PR8: cancel a draft or scheduled post.
export async function cancelSocialPost(
  token: string,
  postId: string
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/posts/${postId}`, token, {
    method: "PATCH",
    body: JSON.stringify({ status: "canceled" }),
  });
}

// Sprint 4 PR2: bulk publish — up to 50 posts in one call.
// Each entry in the response is either a `data` (success) or `error`
// (per-post failure) envelope. Drafts and scheduled posts are not
// supported in bulk.
export interface BulkPostResultEntry {
  status: number;
  data?: SocialPost;
  error?: { code: string; message: string };
}

export async function bulkCreateSocialPosts(
  token: string,
  posts: Array<{
    caption?: string;
    account_ids?: string[];
    platform_posts?: Array<{
      account_id: string;
      caption?: string;
      media_urls?: string[];
      media_ids?: string[];
      platform_options?: Record<string, unknown>;
      thread_position?: number;
    }>;
    media_urls?: string[];
    idempotency_key?: string;
  }>
): Promise<ApiResponse<BulkPostResultEntry[]>> {
  return request(`/v1/posts/bulk`, token, {
    method: "POST",
    body: JSON.stringify({ posts }),
  });
}

// Managed Users (profile-scoped — Sprint 4 PR5: list / detail of end
// users onboarded via Connect, grouped by external_user_id)

export interface ManagedUserListEntry {
  external_user_id: string;
  external_user_email?: string;
  account_count: number;
  platform_counts: {
    twitter: number;
    linkedin: number;
    bluesky: number;
    youtube: number;
  };
  reconnect_count: number;
  disconnected_count: number;
  first_connected_at: string;
  last_refreshed_at?: string;
}

export interface ManagedUserDetail {
  external_user_id: string;
  external_user_email?: string;
  account_count: number;
  accounts: SocialAccount[];
}

export async function listManagedUsers(
  token: string,
  profileId: string,
  limit?: number
): Promise<ApiResponse<ManagedUserListEntry[]>> {
  const qs = limit ? `?limit=${limit}` : "";
  return request(`/v1/profiles/${profileId}/users${qs}`, token);
}

export async function getManagedUser(
  token: string,
  profileId: string,
  externalUserId: string
): Promise<ApiResponse<ManagedUserDetail>> {
  return request(
    `/v1/profiles/${profileId}/users/${encodeURIComponent(externalUserId)}`,
    token
  );
}

export async function dismissManagedUserDisconnected(
  token: string,
  profileId: string,
  externalUserId: string
): Promise<ApiResponse<{ dismissed: boolean }>> {
  return request(
    `/v1/profiles/${profileId}/users/${encodeURIComponent(externalUserId)}/dismiss`,
    token,
    { method: "POST" }
  );
}

// Connect sessions (Sprint 3 PR2 — multi-tenant Connect)

export type ConnectSessionPlatform =
  | "twitter"
  | "linkedin"
  | "bluesky"
  | "youtube"
  | "tiktok"
  | "instagram"
  | "threads"
  | "facebook"
  | "pinterest";

export interface ConnectSession {
  id: string;
  platform: ConnectSessionPlatform;
  profile_id?: string;
  external_user_id: string;
  external_user_email?: string;
  return_url?: string;
  allow_quickstart_creds?: boolean;
  status: "pending" | "completed" | "expired" | "cancelled";
  url?: string;
  expires_at: string;
  created_at: string;
  completed_at?: string;
  completed_social_account_id?: string;
  managed_account_id?: string;
}

export async function createConnectSession(
  token: string,
  data: {
    platform: ConnectSessionPlatform;
    profile_id?: string;
    external_user_id: string;
    external_user_email?: string;
    return_url?: string;
    allow_quickstart_creds?: boolean;
  }
): Promise<ApiResponse<ConnectSession>> {
  return request(`/v1/connect/sessions`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function getConnectSession(
  token: string,
  sessionId: string
): Promise<ApiResponse<ConnectSession>> {
  return request(`/v1/connect/sessions/${sessionId}`, token);
}

// Analytics (workspace-scoped)

export interface PostAnalytics {
  post_id: string;
  social_account_id: string;
  platform: string;
  external_id: string;
  impressions: number;
  reach: number;
  likes: number;
  comments: number;
  shares: number;
  saves: number;
  clicks: number;
  video_views: number;
  views: number; // legacy alias for video_views; will be removed when backend drops it
  engagement_rate: number;
  platform_specific?: Record<string, unknown>;
  fetched_at: string;
  consecutive_failures: number;
  last_failure_reason?: string;
}

export async function getPostAnalytics(
  token: string,
  postId: string,
  opts?: { refresh?: boolean }
): Promise<ApiResponse<PostAnalytics[]>> {
  const qs = opts?.refresh ? "?refresh=true" : "";
  return request(`/v1/posts/${postId}/analytics${qs}`, token);
}

// Aggregated analytics (powers the analytics page)

export interface AnalyticsSummary {
  period: { start: string; end: string };
  posts: {
    total: number;
    published: number;
    scheduled: number;
    failed: number;
    failed_rate: number;
  };
  engagement: {
    impressions: number;
    reach: number;
    likes: number;
    comments: number;
    shares: number;
    saves: number;
    clicks: number;
    video_views: number;
    engagement_rate: number;
  };
  vs_previous_period: {
    impressions_change: number;
    likes_change: number;
    engagement_change: number;
  };
}

export interface AnalyticsTrend {
  dates: string[];
  series: Record<string, number[]>;
}

export interface PlatformAnalytics {
  platform: string;
  posts: number;
  accounts: number;
  impressions: number;
  reach: number;
  likes: number;
  comments: number;
  shares: number;
  saves: number;
  clicks: number;
  video_views: number;
  engagement_rate: number;
}

export interface AnalyticsPostListItem {
  post_id: string;
  social_post_result_id: string;
  social_account_id: string;
  profile_id: string;
  platform: string;
  external_id?: string;
  external_user_id?: string;
  result_status: string;
  post_status: string;
  caption?: string;
  url?: string;
  created_at: string;
  published_at?: string;
  impressions: number;
  reach: number;
  likes: number;
  comments: number;
  shares: number;
  saves: number;
  clicks: number;
  video_views: number;
  engagement_rate: number;
  platform_specific?: Record<string, unknown>;
  fetched_at?: string;
  consecutive_failures: number;
  last_failure_reason?: string;
}

export interface AnalyticsRollupGroup {
  platform?: string;
  social_account_id?: string;
  external_user_id?: string;
  status?: string;
  published_count: number;
  failed_count: number;
  partial_count: number;
  impressions: number;
  reach: number;
  likes: number;
  comments: number;
  shares: number;
  saves: number;
  clicks: number;
  video_views: number;
  engagement_rate: number;
}

export interface AnalyticsRollup {
  granularity: "day" | "week" | "month";
  group_by: string[];
  series: Array<{
    bucket: string;
    groups: AnalyticsRollupGroup[];
  }>;
}

export interface AnalyticsPlatformAvailability {
  platform: string;
  supported_metrics: string[];
  refresh_supported: boolean;
  account_count: number;
  active_account_count: number;
  needs_reconnect_count: number;
  analytics_row_count: number;
  last_successful_fetch_at?: string;
  last_failure_reason?: string;
  health: "not_connected" | "needs_reconnect" | "partial_reconnect_required" | "pending" | "degraded" | "ready" | string;
  notes?: string[];
}

export interface AnalyticsPlatformSummary {
  posts: number;
  accounts: number;
  impressions: number;
  reach: number;
  likes: number;
  comments: number;
  shares: number;
  saves: number;
  clicks: number;
  video_views: number;
  engagement_rate: number;
}

export interface AnalyticsPlatformDetail {
  platform: string;
  period: { start: string; end: string };
  availability: AnalyticsPlatformAvailability;
  summary: AnalyticsPlatformSummary;
  trend: Array<{
    date: string;
    posts: number;
    impressions: number;
    reach: number;
    likes: number;
    comments: number;
    shares: number;
    saves: number;
    clicks: number;
    video_views: number;
  }>;
  accounts: Array<{
    social_account_id: string;
    profile_id: string;
    account_name?: string;
    external_user_id?: string;
    status: string;
    post_count: number;
    last_successful_fetch_at?: string;
    last_failure_reason?: string;
  }>;
  top_posts: AnalyticsPostListItem[];
}

export interface AnalyticsRefreshRequest {
  platform?: string;
  profile_id?: string;
  account_id?: string;
  post_id?: string;
  from?: string;
  to?: string;
  limit?: number;
}

export interface AnalyticsRefreshResponse {
  status: "queued";
  matched_count: number;
  requested_count: number;
  limit: number;
  processed_by: "analytics_refresh_worker" | string;
  filters: AnalyticsRefreshRequest;
}

export interface AnalyticsRangeParams {
  from?: string;       // YYYY-MM-DD
  to?: string;         // YYYY-MM-DD
  start_date?: string; // YYYY-MM-DD
  end_date?: string;   // YYYY-MM-DD
  profile_id?: string;
  platform?: string;   // platform key, omit or "all" to disable
  status?: string;     // post status, omit or "all" to disable
}

export interface AnalyticsPostsParams extends AnalyticsRangeParams {
  account_id?: string;
  social_account_id?: string;
  post_id?: string;
  limit?: number;
  cursor?: string;
  sort?: "published_at" | "published_at_asc" | "created_at" | "created_at_asc" | "impressions" | "reach" | "likes" | "comments" | "shares" | "saves" | "clicks" | "video_views" | "engagement_rate" | `-${string}`;
}

export interface AnalyticsRollupParams {
  from: string; // RFC3339
  to: string;   // RFC3339
  granularity?: "day" | "week" | "month";
  group_by?: string | string[];
  profile_id?: string;
}

function rangeQuery(params?: AnalyticsRangeParams & { metric?: string }): string {
  if (!params) return "";
  const qs = new URLSearchParams();
  if (params.from || params.start_date) qs.set("from", params.from || params.start_date || "");
  if (params.to || params.end_date) qs.set("to", params.to || params.end_date || "");
  if (params.profile_id && params.profile_id !== "all") qs.set("profile_id", params.profile_id);
  if (params.platform && params.platform !== "all") qs.set("platform", params.platform);
  if (params.status && params.status !== "all") qs.set("status", params.status);
  if (params.metric) qs.set("metric", params.metric);
  const s = qs.toString();
  return s ? `?${s}` : "";
}

function analyticsPostsQuery(params?: AnalyticsPostsParams): string {
  const qs = new URLSearchParams(rangeQuery(params).replace(/^\?/, ""));
  if (params?.account_id) qs.set("account_id", params.account_id);
  if (params?.social_account_id) qs.set("social_account_id", params.social_account_id);
  if (params?.post_id) qs.set("post_id", params.post_id);
  if (typeof params?.limit === "number") qs.set("limit", String(params.limit));
  if (params?.cursor) qs.set("cursor", params.cursor);
  if (params?.sort) qs.set("sort", params.sort);
  const s = qs.toString();
  return s ? `?${s}` : "";
}

function analyticsRollupQuery(params: AnalyticsRollupParams): string {
  const qs = new URLSearchParams();
  qs.set("from", params.from);
  qs.set("to", params.to);
  if (params.granularity) qs.set("granularity", params.granularity);
  if (params.group_by) qs.set("group_by", Array.isArray(params.group_by) ? params.group_by.join(",") : params.group_by);
  if (params.profile_id && params.profile_id !== "all") qs.set("profile_id", params.profile_id);
  return `?${qs.toString()}`;
}

export async function getAnalyticsSummary(
  token: string,
  params?: AnalyticsRangeParams
): Promise<ApiResponse<AnalyticsSummary>> {
  return request(`/v1/analytics/summary${rangeQuery(params)}`, token);
}

export async function getAnalyticsTrend(
  token: string,
  params?: AnalyticsRangeParams & { metric?: string }
): Promise<ApiResponse<AnalyticsTrend>> {
  return request(`/v1/analytics/trend${rangeQuery(params)}`, token);
}

export async function getAnalyticsByPlatform(
  token: string,
  params?: AnalyticsRangeParams
): Promise<ApiResponse<PlatformAnalytics[]>> {
  return request(`/v1/analytics/by-platform${rangeQuery(params)}`, token);
}

export async function getAnalyticsPosts(
  token: string,
  params?: AnalyticsPostsParams
): Promise<ApiResponse<AnalyticsPostListItem[]>> {
  return request(`/v1/analytics/posts${analyticsPostsQuery(params)}`, token);
}

export async function exportAnalyticsPostsCSV(
  token: string,
  params?: AnalyticsPostsParams
): Promise<string> {
  const res = await fetch(`${API_URL}/v1/analytics/posts/export${analyticsPostsQuery(params)}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const err = body as ApiError;
    throw new Error(err.error?.message || `Request failed: ${res.status}`);
  }
  return res.text();
}

export async function getAnalyticsRollup(
  token: string,
  params: AnalyticsRollupParams
): Promise<ApiResponse<AnalyticsRollup>> {
  return request(`/v1/analytics/rollup${analyticsRollupQuery(params)}`, token);
}

export async function getAnalyticsPlatforms(
  token: string,
  params?: AnalyticsRangeParams
): Promise<ApiResponse<AnalyticsPlatformAvailability[]>> {
  return request(`/v1/analytics/platforms${rangeQuery(params)}`, token);
}

export async function getAnalyticsPlatform(
  token: string,
  platform: string,
  params?: AnalyticsRangeParams
): Promise<ApiResponse<AnalyticsPlatformDetail>> {
  return request(`/v1/analytics/platforms/${encodeURIComponent(platform)}${rangeQuery(params)}`, token);
}

export async function requestAnalyticsRefresh(
  token: string,
  payload: AnalyticsRefreshRequest
): Promise<ApiResponse<AnalyticsRefreshResponse>> {
  return request(`/v1/analytics/refresh`, token, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

// Admin

export interface AdminStats {
  total_users: number;
  new_users_this_month: number;
  paid_users: number;
  mrr_cents: number;
  posts_this_month: number;
  posts_failed_this_month: number;
  active_workspaces: number;
  platform_connections: number;
  new_signups_7d: number;
  prev_signups_7d: number;
  churn_30d: number;
}

export interface AdminLandingSourceRow {
  source_code: string;
  label: string;
  visits: number;
  unique_visitors: number;
  signups: number;
  paid_users: number;
  signup_rate: number;
  paid_conversion_rate: number;
  top_campaign: string | null;
  last_visit_at: string | null;
}

export interface AdminLandingSourcesResponse {
  range_days: number;
  total_visits: number;
  unique_visitors: number;
  rows: AdminLandingSourceRow[];
}

export interface AdminLandingVisitorRow {
  id: number;
  created_at: string;
  path: string;
  source_code: string;
  label: string;
  referrer: string;
  session_id: string;
  country_code: string;
  user_id: string | null;
  user_email: string | null;
  raw_query: string;
  attribution: {
    r?: string;
    utm_source?: string;
    utm_medium?: string;
    utm_campaign?: string;
  };
}

export interface AdminLandingVisitorTrendRow {
  date: string;
  visits: number;
  unique_visitors: number;
  signups: number;
}

export interface AdminCountryBreakdownRow {
  country_code: string;
  count: number;
}

export interface AdminPathBreakdownRow {
  path: string;
  label: string;
  count: number;
}

export interface AdminLandingVisitorsResponse {
  range_days: number;
  total_visits: number;
  unique_visitors: number;
  signups: number;
  rows: AdminLandingVisitorRow[];
  trend: AdminLandingVisitorTrendRow[];
  countries: AdminCountryBreakdownRow[];
  paths: AdminPathBreakdownRow[];
  source_options: string[];
  campaign_options: string[];
}

export interface AdminUserRow {
  id: string;
  email: string;
  created_at: string;
  signup_country_code: string;
  workspace_count: number;
  api_key_count: number;
  platform_count: number;
  platforms: string[];
  posts_used: number;
  scheduled_posts?: number;
  failed_posts_this_month: number;
  post_limit: number;
  mrr_cents: number;
  is_paid: boolean;
  last_post_at: string | null;
}

export interface AdminUserSignupTrend {
  range_days: number;
  // ISO timestamps — bucket on the client in the viewer's local timezone.
  // Server returns events for a slightly wider window (range_days + 2)
  // to cover any IANA timezone offset.
  events: string[];
  countries: AdminCountryBreakdownRow[];
}

export interface AdminUserWorkspace {
  id: string;
  name: string;
  created_at: string;
  plan_id: string;
  plan_name: string;
  price_cents: number;
  posts_used: number;
  post_limit: number;
  status: string;
  platform_count: number;
}

export interface AdminUserDetail {
  id: string;
  email: string;
  name: string;
  created_at: string;
  signup_country_code: string;
  workspace_count: number;
  api_key_count: number;
  platform_count: number;
  platforms: string[];
  posts_used_this_month: number;
  post_limit: number;
  mrr_cents: number;
  total_posts: number;
  failed_posts_30d: number;
  last_post_at: string | null;
  workspaces: AdminUserWorkspace[];
}

export interface AdminUserScheduledPost {
  post_id: string;
  title: string;
  created_at: string;
  scheduled_at: string | null;
  platforms: string[];
}

export interface AdminUserPostFailure {
  post_id: string;
  post_failure_id?: string;
  social_post_result_id?: string;
  user_id: string;
  user_email: string;
  workspace_id: string;
  workspace_name: string;
  created_at: string;
  post_status: string;
  source: string;
  platform?: string;
  account_name?: string;
  caption?: string;
  error_message?: string;
  error_summary?: string;
  error_code?: string;
  failure_stage?: string;
  platform_error_code?: string;
  is_retriable?: boolean;
  next_action?: string;
  // Curl dump of every failing HTTP request the adapter made. Server
  // redacts Authorization header + token query params before sending.
  debug_curl?: string;
}

export type ErrorTriageClassification =
  | "unipost_bug"
  | "user_action_needed"
  | "upstream_platform_issue"
  | "transient_no_action"
  | "needs_human_review";

export type ErrorTriageActionKind = "none" | "email" | "bug_plan" | "monitor" | "review";
export type ErrorTriageWorkflowStatus = "pending_review" | "ready" | "partially_completed" | "completed" | "dismissed" | "failed";
export type ErrorTriageRecipientStatus = "pending" | "sent" | "dismissed" | "send_failed";
export type ErrorTriageRunHealthStatus = "no_actionable_issues" | "actionable_items" | "needs_review";

export interface ErrorTriageRunSummary {
  id: string;
  run_type: "scheduled" | "manual";
  status: "running" | "completed" | "failed";
  window_start: string;
  window_end: string;
  failures_analyzed: number;
  health_status: ErrorTriageRunHealthStatus;
  items_total: number;
  email_drafts: number;
  bug_plans: number;
  needs_review: number;
  summary?: string;
  error_message?: string;
  started_at: string;
  completed_at?: string | null;
  created_at: string;
}

export interface ErrorTriageBugPlan {
  title?: string;
  impact?: string;
  evidence?: string[];
  suspected_area?: string;
  proposed_fix?: string;
  validation_plan?: string;
  rollback_plan?: string;
}

export interface ErrorTriageEmailDraft {
  subject?: string;
  body?: string;
  cta_url?: string;
}

export interface ErrorTriageRecipient {
  id: string;
  item_id: string;
  recipient_scope_key: string;
  workspace_id: string;
  recipient_user_id: string;
  email_snapshot: string;
  current_email?: string;
  status: ErrorTriageRecipientStatus;
  latest_send_attempt_id?: string;
  dismiss_reason?: string;
  created_at: string;
  updated_at: string;
}

export interface ErrorTriageReviewAnalysis {
  what_is_this_error?: string;
  why_it_happened?: string;
  how_to_resolve?: string;
  missing_evidence?: string;
  next_inspection_path?: string;
}

export interface ErrorTriageEvidenceSample {
  post_failure_id?: string;
  post_id?: string;
  social_post_result_id?: string;
  workspace_id?: string;
  platform?: string;
  source?: string;
  error_code?: string;
  platform_error_code?: string;
  failure_stage?: string;
  message?: string;
  debug_curl?: string;
  created_at?: string;
}

export interface ErrorTriageEvidence {
  samples?: ErrorTriageEvidenceSample[];
  truncated?: boolean;
  failure_count?: number;
  affected_user_count?: number;
  affected_workspace_count?: number;
  affected_post_count?: number;
  review_analysis?: ErrorTriageReviewAnalysis;
  [key: string]: unknown;
}

export interface ErrorTriageItem {
  id: string;
  run_id: string;
  dedupe_key: string;
  classification: ErrorTriageClassification;
  action_kind: ErrorTriageActionKind;
  workflow_status: ErrorTriageWorkflowStatus;
  confidence: number;
  platform?: string;
  source?: string;
  error_code?: string;
  platform_error_code?: string;
  failure_stage?: string;
  affected_user_count: number;
  affected_workspace_count: number;
  affected_post_count: number;
  latest_failure_at?: string | null;
  evidence_json?: ErrorTriageEvidence;
  ai_summary?: string;
  admin_notes?: string;
  bug_plan_json?: ErrorTriageBugPlan | null;
  email_draft_json?: ErrorTriageEmailDraft | null;
  cta_url?: string;
  duplicate_of_item_id?: string;
  created_at: string;
  updated_at: string;
  recipients?: ErrorTriageRecipient[];
}

export interface ErrorTriageRunDetail {
  run: ErrorTriageRunSummary;
  items: ErrorTriageItem[];
}

export interface ErrorTriageSendResult {
  attempt_id: string;
  attempt_number: number;
  idempotency_key: string;
  recipient_email: string;
  recipient_user_id: string;
}

export interface AdminUserListParams {
  search?: string;
  plan?: "all" | "free" | "paid";
  sort?: "newest" | "mrr" | "usage" | "last_active";
  limit?: number;
  offset?: number;
}

export interface AdminPostFailureListParams {
  search?: string;
  user_id?: string;
  platform?: string;
  source?: string;
  period?: "this_month";
  days?: number;
  limit?: number;
}

export interface AdminPostRow {
  post_id: string;
  user_id: string;
  user_email: string;
  workspace_id: string;
  workspace_name: string;
  status: string;
  source: string;
  caption?: string;
  created_at: string;
  scheduled_at?: string;
  published_at?: string;
  platforms: string[];
  result_count: number;
  published_result_count: number;
  failed_result_count: number;
}

export interface AdminPostListParams {
  search?: string;
  status?: string;
  result_status?: string;
  platform?: string;
  source?: string;
  user_id?: string;
  workspace_id?: string;
  days?: number;
  limit?: number;
}

export interface AdminPostsPlatformAggregate {
  platform: string;
  published: number;
  failed: number;
  total: number;
}

export interface AdminPostsEvent {
  created_at: string;
  status: "published" | "failed";
}

export interface AdminPostsAggregates {
  total_posts: number;
  unique_users: number;
  by_status: Record<string, number>;
  by_platform: AdminPostsPlatformAggregate[];
  // Per-post events (published + failed only) — bucket on the client by
  // local day so late-evening posts don't slide to the next UTC date.
  events: AdminPostsEvent[];
}

export type AdminEmailNotificationStatus = "pending" | "sent" | "failed" | "skipped";

export interface AdminEmailNotificationRow {
  id: string;
  event_key: string;
  event_type: string;
  trigger_event: string;
  workspace_id: string;
  workspace_name: string;
  user_id: string;
  owner_email: string;
  email: string;
  period: string;
  threshold_percent: number;
  status: AdminEmailNotificationStatus;
  transactional_id: string;
  idempotency_key: string;
  effective_usage: number;
  completed_usage: number;
  reserved_usage: number;
  post_limit: number;
  failure_reason?: string;
  attempted_at: string;
  sent_at?: string;
  created_at: string;
  updated_at: string;
  provider: string;
  delivery_class: string;
  trigger_source: string;
  trigger_reference_id: string;
  subject_snapshot: string;
}

export interface AdminEmailNotificationListParams {
  search?: string;
  status?: "all" | AdminEmailNotificationStatus;
  provider?: "all" | string;
  event_key?: string;
  threshold?: "all" | 80 | 85 | 90 | 95 | 100;
  period?: string;
  limit?: number;
  offset?: number;
}

export interface AdminBillingRow {
  workspace_id: string;
  workspace_name: string;
  user_id: string;
  user_email: string;
  plan_id: string;
  plan_name: string;
  price_cents: number;
  status: string;
  stripe_customer_id?: string;
  stripe_subscription_id?: string;
  current_period_end?: string;
  cancel_at_period_end: boolean;
  trial_used: boolean;
  posts_used: number;
  post_limit: number;
  updated_at: string;
}

export interface AdminBillingListParams {
  search?: string;
  status?: string;
  plan?: string;
  days?: number;
  limit?: number;
}

export type AdminAIProvider = "tokengate" | "openai" | "anthropic";
export type AdminAISurface = "post_assist" | "error_triage";
export type AdminAIClientKind = "chat_completions" | "messages";
export type AdminAIProviderSource = "admin" | "env" | "none";
export type AdminAIValidationStatus =
  | "ok"
  | "auth_failed"
  | "model_failed"
  | "rate_limited"
  | "provider_failed"
  | "config_failed";

export interface AdminAIProviderStatus {
  provider: AdminAIProvider;
  configured: boolean;
  enabled: boolean;
  source: AdminAIProviderSource;
  key_tail: string;
  base_url: string;
  chat_model: string;
  messages_model: string;
  last_validated_at?: string;
  last_validation_status?: string;
  last_validation_error?: string;
  last_rotated_at?: string;
  updated_at?: string;
}

export interface AdminAIRouteStatus {
  surface: AdminAISurface;
  provider?: AdminAIProvider;
  source: AdminAIProviderSource;
  client_kind?: AdminAIClientKind;
  model?: string;
  model_override?: string;
}

export interface AdminAIProvidersResponse {
  providers: AdminAIProviderStatus[];
  effective: Partial<Record<AdminAISurface, AdminAIRouteStatus>>;
  routes: Partial<Record<AdminAISurface, AdminAIRouteStatus>>;
}

export interface AdminAIProviderUpdatePayload {
  api_key?: string;
  base_url: string;
  chat_model?: string;
  messages_model?: string;
  enabled: boolean;
}

export interface AdminAIProviderTestPayload {
  api_key?: string;
  base_url?: string;
  chat_model?: string;
  messages_model?: string;
}

export interface AdminAIProviderValidationResult {
  status: AdminAIValidationStatus;
  message: string;
}

export interface AdminAIRoutePayload {
  provider: AdminAIProvider;
  client_kind: AdminAIClientKind;
  model_override?: string;
}

export interface AdminAIProviderEvent {
  id: number;
  provider?: AdminAIProvider;
  surface?: AdminAISurface;
  action: string;
  category: string;
  actor_admin_id?: string;
  metadata?: Record<string, unknown>;
  created_at?: string;
}

export interface AdminAIProviderEventsResponse {
  events: AdminAIProviderEvent[];
  next_cursor: string;
}

// Whoami — returns the authenticated user's identity plus an
// is_admin flag derived from the backend ADMIN_USERS allowlist.
// Used by the dashboard shell to decide whether to show the Admin
// link, and by the admin page to gate client-side rendering.
export interface MeResponse {
  user_id: string;
  email: string;
  name?: string;
  is_admin: boolean;
  // is_super_admin flags users on the SUPER_ADMINS env var. Dashboard
  // uses it to gate in-development features (currently the Facebook
  // Pages entry in Connections) without a second env var.
  is_super_admin?: boolean;
  // Workspace shortcut for Clerk-auth surfaces. The standalone
  // /v1/workspaces list endpoint was retired in the Apr 2026
  // workspace_id-removal refactor; the hook in
  // src/lib/use-current-workspace.ts reads these instead.
  workspace_id?: string;
  workspace_name?: string;
  // Role in the current workspace ("owner" | "admin" | "editor"),
  // empty when the user has no membership. RBAC migration 060.
  role?: "owner" | "admin" | "editor" | "";
  // Intent-collection redesign: the dashboard uses these to decide
  // whether to pop the Welcome modal on first load.
  onboarding_intent?: OnboardingIntent;
  onboarding_shown_at?: string;
}

export interface PlanGatesResponse {
  plan_gates: Record<string, boolean>;
}

export async function getPlanGates(token: string): Promise<ApiResponse<PlanGatesResponse>> {
  return request("/v1/me/plan-gates", token);
}

// ── Members & invites (RBAC) ──

export interface Member {
  user_id: string;
  email?: string;
  role: "owner" | "admin" | "editor";
  status: "active" | "suspended" | "pending";
  invited_by?: string;
  accepted_at?: string;
  created_at: string;
}

export interface PendingInvite {
  id: string;
  email: string;
  role: "admin" | "editor";
  invited_by: string;
  expires_at: string;
  created_at: string;
  url?: string; // only present on the create-invite response
}

export interface MembersListResponse {
  members: Member[];
  pending_invites: PendingInvite[];
}

export interface PublicInvite {
  workspace_id: string;
  workspace_name: string;
  email: string;
  role: "admin" | "editor";
  expires_at: string;
}

export async function listMembers(token: string): Promise<ApiResponse<MembersListResponse>> {
  return request("/v1/members", token);
}

export async function inviteMember(
  token: string,
  email: string,
  role: "admin" | "editor",
): Promise<ApiResponse<PendingInvite>> {
  return request("/v1/members/invite", token, {
    method: "POST",
    body: JSON.stringify({ email, role }),
  });
}

export async function revokeInvite(token: string, inviteId: string): Promise<void> {
  await request<void>(`/v1/members/invites/${inviteId}`, token, { method: "DELETE" });
}

export async function changeMemberRole(
  token: string,
  userId: string,
  role: "admin" | "editor",
): Promise<ApiResponse<Member>> {
  return request(`/v1/members/${userId}/role`, token, {
    method: "PATCH",
    body: JSON.stringify({ role }),
  });
}

export async function removeMember(token: string, userId: string): Promise<void> {
  await request<void>(`/v1/members/${userId}`, token, { method: "DELETE" });
}

export async function transferOwnership(token: string, userId: string): Promise<void> {
  await request<void>(`/v1/members/${userId}/transfer-ownership`, token, { method: "POST" });
}

// ── Audit log (RBAC Phase 6) ──

export interface AuditLogEntry {
  id: number;
  actor_user_id?: string;
  actor_api_key_id?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  category: string;
  ip_address?: string;
  before?: unknown;
  after?: unknown;
  metadata?: unknown;
  created_at: string;
}

export async function listAuditLog(
  token: string,
  params?: { action?: string; category?: string; days?: number; limit?: number },
): Promise<ApiResponse<AuditLogEntry[]>> {
  const qs = new URLSearchParams();
  if (params?.action) qs.set("action", params.action);
  if (params?.category) qs.set("category", params.category);
  if (params?.days) qs.set("days", String(params.days));
  if (params?.limit) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return request(`/v1/audit-log${s ? `?${s}` : ""}`, token);
}

// Public preview — no token required (Clerk session NOT needed; the
// invite token in the URL IS the authentication).
export async function getPublicInvite(inviteToken: string): Promise<ApiResponse<PublicInvite>> {
  const url = `${API_URL}/v1/public/invites/${inviteToken}`;
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body?.error?.message || `HTTP ${res.status}`);
  }
  return res.json();
}

// Accept the invite. Requires Clerk session (the invitee must be
// signed in). Caller passes the user's Clerk JWT as `clerkToken`.
export async function acceptInvite(clerkToken: string, inviteToken: string): Promise<ApiResponse<Member>> {
  return request(`/v1/invites/${inviteToken}/accept`, clerkToken, { method: "POST" });
}

export type OnboardingIntent = "exploring" | "own_accounts" | "building_api" | "skipped";

export async function getMe(token: string): Promise<ApiResponse<MeResponse>> {
  return request("/v1/me", token);
}

// Intent-collection redesign. Called when the user submits or skips
// the Welcome modal. Never gates any feature — purely for personalization.
export async function setOnboardingIntent(
  token: string,
  intent: OnboardingIntent
): Promise<ApiResponse<{ intent: OnboardingIntent }>> {
  return request("/v1/me/intent", token, {
    method: "PATCH",
    body: JSON.stringify({ intent }),
  });
}

// Marks onboarding_shown_at on first Welcome modal render so we
// never show it again to the same user, even if they skip.
export async function markOnboardingShown(
  token: string
): Promise<ApiResponse<{ ok: boolean }>> {
  return request("/v1/me/onboarding-shown", token, { method: "POST" });
}

// Deletes the authenticated user via the backend (uses Clerk secret key
// server-side, bypassing the "reauthentication required" check that
// Clerk enforces on client-side user.delete() calls). The user.deleted
// webhook cascades DB cleanup. Returns 204 on success; throws on error.
export async function deleteMe(token: string): Promise<void> {
  await request<void>("/v1/me", token, { method: "DELETE" });
}

// Dashboard empty-state activation guide.
export type ActivationStepId = "connect_account" | "send_post" | "create_api_key";

export interface ActivationStep {
  id: ActivationStepId;
  completed: boolean;
  count: number;
}

export interface ActivationResponse {
  completed: boolean;
  dismissed: boolean;
  steps: ActivationStep[];
  progress: { completed: number; total: number };
}

export async function getActivation(
  token: string
): Promise<ApiResponse<ActivationResponse>> {
  return request("/v1/me/activation", token);
}

export async function dismissActivation(
  token: string
): Promise<ApiResponse<{ dismissed_at: string }>> {
  return request("/v1/me/activation/dismiss", token, { method: "POST" });
}

// ── Tutorials framework ──────────────────────────────────────────────
//
// Multi-tutorial system. Replaces the single-tutorial activation guide.
// Each tutorial has independent completion/dismissal. Per-step state is
// still computed from real counts (counts field), same as activation.

export type TutorialId = "quickstart" | "post_with_api";

export interface TutorialState {
  id: TutorialId;
  completed_at?: string;
  dismissed_at?: string;
}

export interface TutorialsCounts {
  connected_accounts: number;
  posts_sent: number;
  api_keys: number;
}

export interface TutorialsResponse {
  tutorials: TutorialState[];
  counts: TutorialsCounts;
}

export async function getTutorials(
  token: string
): Promise<ApiResponse<TutorialsResponse>> {
  return request("/v1/me/tutorials", token);
}

export async function completeTutorial(
  token: string,
  tutorialId: TutorialId
): Promise<ApiResponse<TutorialState>> {
  return request(`/v1/me/tutorials/${tutorialId}/complete`, token, {
    method: "POST",
  });
}

export async function dismissTutorial(
  token: string,
  tutorialId: TutorialId
): Promise<ApiResponse<TutorialState>> {
  return request(`/v1/me/tutorials/${tutorialId}/dismiss`, token, {
    method: "POST",
  });
}

export async function reopenTutorial(
  token: string,
  tutorialId: TutorialId
): Promise<void> {
  await request<void>(`/v1/me/tutorials/${tutorialId}/reopen`, token, {
    method: "POST",
  });
}

// Bootstrap — dashboard root resolver. Returns the user's default and
// last-visited profile ids; lazily creates a "Default" profile for
// fresh signups so the dashboard never has to render an empty state
// after the first login. Both fields can be null when the Clerk
// webhook hasn't synced the user yet, in which case the caller should
// fall back to /profiles.
export interface BootstrapResponse {
  default_profile_id: string | null;
  last_profile_id: string | null;
  onboarding_completed: boolean;
}

export async function completeOnboarding(
  token: string,
  data: { first_name: string; org_name?: string }
): Promise<ApiResponse<{ completed: boolean }>> {
  return request("/v1/me/onboarding", token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function getBootstrap(
  token: string
): Promise<ApiResponse<BootstrapResponse>> {
  return request("/v1/me/bootstrap", token);
}

export async function getAdminStats(token: string): Promise<ApiResponse<AdminStats>> {
  return request("/v1/admin/stats", token);
}

export async function getAdminLandingSources(
  token: string,
  days = 30
): Promise<ApiResponse<AdminLandingSourcesResponse>> {
  return request(`/v1/admin/landing-sources?days=${days}`, token);
}

export async function getAdminLandingVisitors(
  token: string,
  params?: { days?: number; source?: string; campaign?: string; limit?: number }
): Promise<ApiResponse<AdminLandingVisitorsResponse>> {
  const qs = new URLSearchParams();
  qs.set("days", String(params?.days ?? 30));
  if (params?.source) qs.set("source", params.source);
  if (params?.campaign) qs.set("campaign", params.campaign);
  if (params?.limit != null) qs.set("limit", String(params.limit));
  return request(`/v1/admin/landing-visitors?${qs.toString()}`, token);
}

export async function listAdminUsers(
  token: string,
  params?: AdminUserListParams
): Promise<ApiResponse<AdminUserRow[]>> {
  const qs = new URLSearchParams();
  if (params?.search) qs.set("search", params.search);
  if (params?.plan && params.plan !== "all") qs.set("plan", params.plan);
  if (params?.sort) qs.set("sort", params.sort);
  if (params?.limit != null) qs.set("limit", String(params.limit));
  if (params?.offset != null) qs.set("offset", String(params.offset));
  const s = qs.toString();
  return request(`/v1/admin/users${s ? `?${s}` : ""}`, token);
}

export async function getAdminUserSignups(
  token: string,
  days = 30
): Promise<ApiResponse<AdminUserSignupTrend>> {
  return request(`/v1/admin/users/signups?days=${days}`, token);
}

export async function getAdminUser(
  token: string,
  id: string
): Promise<ApiResponse<AdminUserDetail>> {
  return request(`/v1/admin/users/${id}`, token);
}

export async function getAdminUserScheduledPosts(
  token: string,
  id: string
): Promise<ApiResponse<AdminUserScheduledPost[]>> {
  return request(`/v1/admin/users/${id}/scheduled-posts`, token);
}

export async function getAdminUserPostFailures(
  token: string,
  id: string,
  params?: { days?: number; limit?: number }
): Promise<ApiResponse<AdminUserPostFailure[]>> {
  const qs = new URLSearchParams();
  if (params?.days != null) qs.set("days", String(params.days));
  if (params?.limit != null) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return request(`/v1/admin/users/${id}/post-failures${s ? `?${s}` : ""}`, token);
}

export async function listAdminPostFailures(
  token: string,
  params?: AdminPostFailureListParams
): Promise<ApiResponse<AdminUserPostFailure[]>> {
  const qs = new URLSearchParams();
  if (params?.search) qs.set("search", params.search);
  if (params?.user_id) qs.set("user_id", params.user_id);
  if (params?.platform) qs.set("platform", params.platform);
  if (params?.source) qs.set("source", params.source);
  if (params?.period) qs.set("period", params.period);
  if (params?.days != null) qs.set("days", String(params.days));
  if (params?.limit != null) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return request(`/v1/admin/post-failures${s ? `?${s}` : ""}`, token);
}

export async function listAdminErrorTriageRuns(
  token: string,
  limit = 30,
): Promise<ApiResponse<ErrorTriageRunSummary[]>> {
  return request(`/v1/admin/error-triage/runs?limit=${limit}`, token);
}

export async function getAdminErrorTriageRun(
  token: string,
  id: string,
): Promise<ApiResponse<ErrorTriageRunDetail>> {
  return request(`/v1/admin/error-triage/runs/${encodeURIComponent(id)}`, token);
}

export async function createAdminErrorTriageRun(
  token: string,
  data?: { window_start?: string; window_end?: string; supersedes_run_id?: string },
): Promise<ApiResponse<ErrorTriageRunSummary>> {
  return request("/v1/admin/error-triage/runs", token, {
    method: "POST",
    body: JSON.stringify(data || {}),
  });
}

export async function rerunAdminErrorTriageRun(
  token: string,
  id: string,
): Promise<ApiResponse<ErrorTriageRunSummary>> {
  return request(`/v1/admin/error-triage/runs/${encodeURIComponent(id)}/rerun`, token, {
    method: "POST",
  });
}

export async function updateAdminErrorTriageItem(
  token: string,
  id: string,
  data: { workflow_status?: ErrorTriageWorkflowStatus; admin_notes?: string },
): Promise<ApiResponse<{ ok: boolean }>> {
  return request(`/v1/admin/error-triage/items/${encodeURIComponent(id)}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function approveAdminErrorTriageBugPlan(
  token: string,
  id: string,
  data?: { admin_notes?: string },
): Promise<ApiResponse<{ ok: boolean }>> {
  return request(`/v1/admin/error-triage/items/${encodeURIComponent(id)}/approve-bug-plan`, token, {
    method: "POST",
    body: JSON.stringify(data || {}),
  });
}

export async function sendAdminErrorTriageEmail(
  token: string,
  itemId: string,
  recipientId: string,
): Promise<ApiResponse<ErrorTriageSendResult>> {
  return request(
    `/v1/admin/error-triage/items/${encodeURIComponent(itemId)}/recipients/${encodeURIComponent(recipientId)}/send-email`,
    token,
    { method: "POST" },
  );
}

export async function dismissAdminErrorTriageRecipient(
  token: string,
  itemId: string,
  recipientId: string,
  reason?: string,
): Promise<ApiResponse<{ ok: boolean }>> {
  return request(
    `/v1/admin/error-triage/items/${encodeURIComponent(itemId)}/recipients/${encodeURIComponent(recipientId)}/dismiss`,
    token,
    {
      method: "POST",
      body: JSON.stringify({ reason: reason || "" }),
    },
  );
}

export async function listAdminAIProviders(token: string): Promise<ApiResponse<AdminAIProvidersResponse>> {
  return request("/v1/admin/ai-providers", token);
}

export async function updateAdminAIProvider(
  token: string,
  provider: AdminAIProvider,
  payload: AdminAIProviderUpdatePayload,
): Promise<ApiResponse<AdminAIProviderStatus>> {
  return request(`/v1/admin/ai-providers/${encodeURIComponent(provider)}`, token, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export async function testAdminAIProvider(
  token: string,
  provider: AdminAIProvider,
  payload: AdminAIProviderTestPayload,
): Promise<ApiResponse<AdminAIProviderValidationResult>> {
  return request(`/v1/admin/ai-providers/${encodeURIComponent(provider)}/test`, token, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function routeAdminAIProviderSurface(
  token: string,
  surface: AdminAISurface,
  payload: AdminAIRoutePayload,
): Promise<ApiResponse<AdminAIRouteStatus>> {
  return request(`/v1/admin/ai-provider-routing/${encodeURIComponent(surface)}`, token, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export async function deleteAdminAIProviderSurfaceRoute(
  token: string,
  surface: AdminAISurface,
): Promise<ApiResponse<{ surface: AdminAISurface; source: AdminAIProviderSource }>> {
  return request(`/v1/admin/ai-provider-routing/${encodeURIComponent(surface)}`, token, {
    method: "DELETE",
  });
}

export async function disableAdminAIProvider(
  token: string,
  provider: AdminAIProvider,
): Promise<ApiResponse<AdminAIProviderStatus>> {
  return request(`/v1/admin/ai-providers/${encodeURIComponent(provider)}/disable`, token, {
    method: "POST",
  });
}

export async function listAdminAIProviderEvents(
  token: string,
  params?: { provider?: AdminAIProvider; action?: string; cursor?: string; limit?: number },
): Promise<ApiResponse<AdminAIProviderEventsResponse>> {
  const qs = new URLSearchParams();
  if (params?.provider) qs.set("provider", params.provider);
  if (params?.action) qs.set("action", params.action);
  if (params?.cursor) qs.set("cursor", params.cursor);
  if (params?.limit != null) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return request(`/v1/admin/ai-providers/events${s ? `?${s}` : ""}`, token);
}

export async function listAdminPosts(
  token: string,
  params?: AdminPostListParams
): Promise<ApiResponse<AdminPostRow[]>> {
  const qs = new URLSearchParams();
  if (params?.search) qs.set("search", params.search);
  if (params?.status) qs.set("status", params.status);
  if (params?.result_status) qs.set("result_status", params.result_status);
  if (params?.platform) qs.set("platform", params.platform);
  if (params?.source) qs.set("source", params.source);
  if (params?.user_id) qs.set("user_id", params.user_id);
  if (params?.workspace_id) qs.set("workspace_id", params.workspace_id);
  if (params?.days != null) qs.set("days", String(params.days));
  if (params?.limit != null) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return request(`/v1/admin/posts${s ? `?${s}` : ""}`, token);
}

export async function listAdminPostsAggregates(
  token: string,
  params?: Omit<AdminPostListParams, "limit">
): Promise<ApiResponse<AdminPostsAggregates>> {
  const qs = new URLSearchParams();
  if (params?.search) qs.set("search", params.search);
  if (params?.status) qs.set("status", params.status);
  if (params?.result_status) qs.set("result_status", params.result_status);
  if (params?.platform) qs.set("platform", params.platform);
  if (params?.source) qs.set("source", params.source);
  if (params?.user_id) qs.set("user_id", params.user_id);
  if (params?.workspace_id) qs.set("workspace_id", params.workspace_id);
  if (params?.days != null) qs.set("days", String(params.days));
  const s = qs.toString();
  return request(`/v1/admin/posts/aggregates${s ? `?${s}` : ""}`, token);
}

export async function listAdminEmailNotifications(
  token: string,
  params?: AdminEmailNotificationListParams
): Promise<ApiResponse<AdminEmailNotificationRow[]>> {
  const qs = new URLSearchParams();
  if (params?.search) qs.set("search", params.search);
  if (params?.status && params.status !== "all") qs.set("status", params.status);
  if (params?.provider && params.provider !== "all") qs.set("provider", params.provider);
  if (params?.event_key) qs.set("event_key", params.event_key);
  if (params?.threshold && params.threshold !== "all") qs.set("threshold", String(params.threshold));
  if (params?.period) qs.set("period", params.period);
  if (params?.limit != null) qs.set("limit", String(params.limit));
  if (params?.offset != null) qs.set("offset", String(params.offset));
  const s = qs.toString();
  return request(`/v1/admin/email-notifications${s ? `?${s}` : ""}`, token);
}

export async function listAdminBilling(
  token: string,
  params?: AdminBillingListParams
): Promise<ApiResponse<AdminBillingRow[]>> {
  const qs = new URLSearchParams();
  if (params?.search) qs.set("search", params.search);
  if (params?.status) qs.set("status", params.status);
  if (params?.plan) qs.set("plan", params.plan);
  if (params?.days != null) qs.set("days", String(params.days));
  if (params?.limit != null) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return request(`/v1/admin/billing${s ? `?${s}` : ""}`, token);
}

// Admin-only: flip a workspace's plan_id without going through Stripe.
// Used to test plan-feature gates end-to-end (Inbox / Analytics /
// profile cap). Returns 204 No Content on success.
export async function setAdminWorkspacePlan(
  token: string,
  workspaceId: string,
  planId: string,
): Promise<void> {
  await request<void>(`/v1/admin/workspaces/${workspaceId}/plan`, token, {
    method: "POST",
    body: JSON.stringify({ plan_id: planId }),
  });
}

export async function recordLandingVisit(data: {
  path: string;
  source?: string;
  session_id: string;
  referrer?: string;
  country_code?: string;
  raw_query?: string;
  attribution?: {
    r?: string;
    utm_source?: string;
    utm_medium?: string;
    utm_campaign?: string;
  };
}): Promise<void> {
  await fetch(`${API_URL}/v1/public/landing-visit`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
    keepalive: true,
  });
}

export async function bindLandingAttributionSession(
  token: string,
  sessionId: string
): Promise<ApiResponse<{ bound: boolean }>> {
  return request("/v1/me/landing-attribution", token, {
    method: "POST",
    body: JSON.stringify({ session_id: sessionId }),
  });
}

// API Metrics

export interface APIMetricsSummaryRow {
  path: string;
  method: string;
  total_calls: number;
  success_count: number;
  client_error_count: number;
  server_error_count: number;
  rate_limited_count: number;
  error_rate_pct: number;
  server_failure_rate_pct: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  avg_ms: number;
}

export interface APIMetricsTrendRow {
  bucket: string;
  total_calls: number;
  success_count: number;
  error_count: number;
  client_error_count: number;
  server_error_count: number;
  rate_limited_count: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  avg_ms: number;
}

export interface APIMetricsOverall {
  total_calls: number;
  success_count: number;
  client_error_count: number;
  server_error_count: number;
  rate_limited_count: number;
  error_rate_pct: number;
  server_failure_rate_pct: number;
  reliability_pct: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  avg_ms: number;
}

export interface APIMetricsStatusCodeRow {
  status_code: number;
  method: string;
  path: string;
  total_calls: number;
}

export interface AdminAPIMetricsWorkspaceRow {
  workspace_id: string;
  workspace_name: string;
  total_calls: number;
  rate_limited_count: number;
  error_rate_pct: number;
  server_failure_rate_pct: number;
  p95_ms: number;
  p99_ms: number;
  slowest_endpoint: string;
  slowest_endpoint_p95_ms: number;
}

export interface APIMetricsQueryParams {
  from: string;
  to: string;
  interval?: "hour" | "day";
  method?: string;
  path?: string;
  status_class?: "2xx" | "3xx" | "4xx" | "5xx";
  sort?: "total_calls_desc" | "p95_ms_desc" | "p99_ms_desc" | "server_errors_desc" | "rate_limited_desc";
  limit?: number;
  workspace_id?: string;
  min_calls?: number;
}

function apiMetricsQuery(params: APIMetricsQueryParams): string {
  const q = new URLSearchParams();
  q.set("from", params.from);
  q.set("to", params.to);
  for (const key of ["interval", "method", "path", "status_class", "sort", "workspace_id"] as const) {
    const value = params[key];
    if (value) q.set(key, String(value));
  }
  if (params.limit) q.set("limit", String(params.limit));
  if (params.min_calls) q.set("min_calls", String(params.min_calls));
  return q.toString();
}

export async function getAPIMetricsSummary(
  token: string,
  from: string,
  to: string,
  params?: Partial<APIMetricsQueryParams>
): Promise<ApiResponse<APIMetricsSummaryRow[]>> {
  return request(`/v1/api-metrics/summary?${apiMetricsQuery({ from, to, ...params })}`, token);
}

export async function getAPIMetricsTrend(
  token: string,
  from: string,
  to: string,
  params?: Partial<APIMetricsQueryParams>
): Promise<ApiResponse<APIMetricsTrendRow[]>> {
  return request(`/v1/api-metrics/trend?${apiMetricsQuery({ from, to, ...params })}`, token);
}

export async function getAPIMetricsOverall(
  token: string,
  from: string,
  to: string,
  params?: Partial<APIMetricsQueryParams>
): Promise<ApiResponse<APIMetricsOverall>> {
  return request(`/v1/api-metrics/overall?${apiMetricsQuery({ from, to, ...params })}`, token);
}

export async function getAPIMetricsStatusCodes(
  token: string,
  from: string,
  to: string,
  params?: Partial<APIMetricsQueryParams>
): Promise<ApiResponse<APIMetricsStatusCodeRow[]>> {
  return request(`/v1/api-metrics/status-codes?${apiMetricsQuery({ from, to, ...params })}`, token);
}

export async function getAdminAPIMetricsOverall(
  token: string,
  params: APIMetricsQueryParams
): Promise<ApiResponse<APIMetricsOverall>> {
  return request(`/v1/admin/api-metrics/overall?${apiMetricsQuery(params)}`, token);
}

export async function getAdminAPIMetricsSummary(
  token: string,
  params: APIMetricsQueryParams
): Promise<ApiResponse<APIMetricsSummaryRow[]>> {
  return request(`/v1/admin/api-metrics/summary?${apiMetricsQuery(params)}`, token);
}

export async function getAdminAPIMetricsTrend(
  token: string,
  params: APIMetricsQueryParams
): Promise<ApiResponse<APIMetricsTrendRow[]>> {
  return request(`/v1/admin/api-metrics/trend?${apiMetricsQuery(params)}`, token);
}

export async function getAdminAPIMetricsStatusCodes(
  token: string,
  params: APIMetricsQueryParams
): Promise<ApiResponse<APIMetricsStatusCodeRow[]>> {
  return request(`/v1/admin/api-metrics/status-codes?${apiMetricsQuery(params)}`, token);
}

export async function getAdminAPIMetricsWorkspaces(
  token: string,
  params: APIMetricsQueryParams
): Promise<ApiResponse<AdminAPIMetricsWorkspaceRow[]>> {
  return request(`/v1/admin/api-metrics/workspaces?${apiMetricsQuery(params)}`, token);
}

export async function getAdminChangelogCandidate(
  token: string,
  candidateId: string,
  params: { action: AdminChangelogAction; expires: string; signature: string }
): Promise<ApiResponse<AdminChangelogCandidatePreview>> {
  const qs = new URLSearchParams({
    action: params.action,
    expires: params.expires,
    signature: params.signature,
  });
  return request(`/v1/admin/changelog-candidates/${encodeURIComponent(candidateId)}?${qs.toString()}`, token);
}

export async function confirmAdminChangelogCandidateAction(
  token: string,
  candidateId: string,
  data: { action: AdminChangelogAction; expires: string; signature: string }
): Promise<ApiResponse<AdminChangelogActionResult>> {
  return request(`/v1/admin/changelog-candidates/${encodeURIComponent(candidateId)}/actions`, token, {
    method: "POST",
    body: JSON.stringify({
      action: data.action,
      expires: Number(data.expires),
      signature: data.signature,
    }),
  });
}

// Inbox

export interface InboxItem {
  id: string;
  social_account_id: string;
  workspace_id: string;
  source: "ig_comment" | "ig_dm" | "threads_reply" | "youtube_comment" | "fb_comment" | "fb_dm";
  external_id: string;
  thread_key: string;
  thread_status: "open" | "assigned" | "resolved";
  parent_external_id?: string;
  assigned_to?: string;
  linked_post_id?: string;
  author_name?: string;
  author_id?: string;
  author_avatar_url?: string;
  body?: string;
  is_read: boolean;
  is_own: boolean;
  received_at: string;
  created_at: string;
  account_name?: string;
  account_platform?: string;
  account_avatar_url?: string;
}

export async function listInboxItems(
  token: string,
  filters?: { source?: string; is_read?: string; is_own?: string; limit?: number }
): Promise<ApiResponse<InboxItem[]>> {
  const qs = new URLSearchParams();
  if (filters?.source) qs.set("source", filters.source);
  if (filters?.is_read) qs.set("is_read", filters.is_read);
  if (filters?.is_own) qs.set("is_own", filters.is_own);
  if (typeof filters?.limit === "number") qs.set("limit", String(filters.limit));
  const q = qs.toString();
  return request(`/v1/inbox${q ? `?${q}` : ""}`, token);
}

export async function getInboxUnreadCount(
  token: string,
): Promise<ApiResponse<{ count: number }>> {
  return request(`/v1/inbox/unread-count`, token);
}

export async function markInboxItemRead(
  token: string,
  id: string
): Promise<void> {
  return request(`/v1/inbox/${id}/read`, token, {
    method: "POST",
  });
}

export async function markAllInboxRead(
  token: string,
): Promise<ApiResponse<{ marked: number }>> {
  return request(`/v1/inbox/mark-all-read`, token, {
    method: "POST",
  });
}

export async function replyToInboxItem(
  token: string,
  id: string,
  text: string
): Promise<ApiResponse<InboxItem>> {
  return request(`/v1/inbox/${id}/reply`, token, {
    method: "POST",
    body: JSON.stringify({ text }),
  });
}

export interface IGMediaContext {
  id: string;
  caption: string;
  media_url: string;
  timestamp: string;
  media_type: string;
  permalink: string;
}

export async function getInboxMediaContext(
  token: string,
  inboxItemId: string
): Promise<ApiResponse<IGMediaContext>> {
  return request(`/v1/inbox/${inboxItemId}/media-context`, token);
}

export async function updateInboxThreadState(
  token: string,
  id: string,
  data: { thread_status: "open" | "assigned" | "resolved"; assigned_to?: string }
): Promise<ApiResponse<InboxItem>> {
  return request(`/v1/inbox/${id}/thread-state`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function syncInbox(
  token: string,
): Promise<ApiResponse<{ new_items: number }>> {
  return request(`/v1/inbox/sync`, token, {
    method: "POST",
  });
}

// ── Notifications ────────────────────────────────────────────────────

export interface NotificationEvent {
  event_type: string;
  label: string;
  description: string;
  severity: "critical" | "high" | "medium" | "low";
  default_on: boolean;
}

export interface NotificationChannel {
  id: string;
  kind: "email" | "slack_webhook" | "discord_webhook" | "sms" | "in_app";
  label?: string;
  config: { address?: string; url?: string; e164?: string };
  verified: boolean;
  created_at: string;
}

export interface NotificationSubscription {
  id: string;
  event_type: string;
  channel_id: string;
  enabled: boolean;
  created_at: string;
}

export async function listNotificationEvents(
  token: string
): Promise<ApiResponse<NotificationEvent[]>> {
  return request(`/v1/me/notifications/events`, token);
}

export async function listNotificationChannels(
  token: string
): Promise<ApiResponse<NotificationChannel[]>> {
  return request(`/v1/me/notifications/channels`, token);
}

export async function createNotificationChannel(
  token: string,
  data: { kind: string; address?: string; url?: string; label?: string }
): Promise<ApiResponse<NotificationChannel>> {
  return request(`/v1/me/notifications/channels`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function deleteNotificationChannel(
  token: string,
  id: string
): Promise<void> {
  await request(`/v1/me/notifications/channels/${id}`, token, {
    method: "DELETE",
  });
}

export async function testNotificationChannel(
  token: string,
  id: string
): Promise<ApiResponse<{ id: string; kind: string; message: string }>> {
  return request(`/v1/me/notifications/channels/${id}/test`, token, {
    method: "POST",
  });
}

export async function listNotificationSubscriptions(
  token: string
): Promise<ApiResponse<NotificationSubscription[]>> {
  return request(`/v1/me/notifications/subscriptions`, token);
}

export async function upsertNotificationSubscription(
  token: string,
  data: { event_type: string; channel_id: string; enabled: boolean }
): Promise<ApiResponse<NotificationSubscription>> {
  return request(`/v1/me/notifications/subscriptions`, token, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function deleteNotificationSubscription(
  token: string,
  id: string
): Promise<void> {
  await request(`/v1/me/notifications/subscriptions/${id}`, token, {
    method: "DELETE",
  });
}
