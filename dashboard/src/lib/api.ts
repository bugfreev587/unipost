const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Types

export interface Profile {
  id: string;
  workspace_id: string;
  name: string;
  created_at: string;
  updated_at: string;
  // Sprint 4 PR4: white-label Connect branding
  branding_logo_url?: string;
  branding_display_name?: string;
  branding_primary_color?: string;
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

export interface ApiResponse<T> {
  data: T;
  meta?: {
    total: number;
    page: number;
    per_page: number;
  };
}

export interface ApiError {
  error: {
    code: string;
    message: string;
  };
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
  actual?: unknown;
  limit?: unknown;
  severity: "error" | "warning";
}

export interface SocialPostValidationResult {
  valid: boolean;
  errors: SocialPostValidationIssue[];
  warnings: SocialPostValidationIssue[];
}

// Client

async function request<T>(
  path: string,
  token: string,
  options?: RequestInit
): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...options?.headers,
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const err = body as ApiError & { error?: { issues?: Array<{ message?: string; field?: string; code?: string }> } };
    // Include validation issue details if present
    let message = err.error?.message || `Request failed: ${res.status}`;
    if (err.error?.issues && err.error.issues.length > 0) {
      const details = err.error.issues.map((i) => i.message || i.code).filter(Boolean).join("; ");
      if (details) message += `: ${details}`;
    }
    throw new Error(message);
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
  created_at: string;
  updated_at: string;
}

export async function listWorkspaces(
  token: string
): Promise<ApiResponse<Workspace[]>> {
  return request("/v1/workspaces", token);
}

export async function getWorkspace(
  token: string,
  workspaceId: string
): Promise<ApiResponse<Workspace>> {
  return request(`/v1/workspaces/${workspaceId}`, token);
}

export async function updateWorkspace(
  token: string,
  workspaceId: string,
  data: { name: string }
): Promise<ApiResponse<Workspace>> {
  return request(`/v1/workspaces/${workspaceId}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

// Profiles (dashboard / Clerk auth).
// Path is /v1/dashboard/profiles to avoid colliding with the
// API-key-auth /v1/profiles routes the public SDK uses — both are
// registered on the same root mux on the backend.

export async function listProfiles(
  token: string
): Promise<ApiResponse<Profile[]>> {
  return request("/v1/dashboard/profiles", token);
}

export async function getProfile(
  token: string,
  id: string
): Promise<ApiResponse<Profile>> {
  return request(`/v1/dashboard/profiles/${id}`, token);
}

export async function createProfile(
  token: string,
  data: { name: string }
): Promise<ApiResponse<Profile>> {
  return request("/v1/dashboard/profiles", token, {
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
  }
): Promise<ApiResponse<Profile>> {
  return request(`/v1/dashboard/profiles/${id}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function deleteProfile(
  token: string,
  id: string
): Promise<void> {
  return request(`/v1/dashboard/profiles/${id}`, token, { method: "DELETE" });
}

// Platform credentials (White Label, workspace-scoped)

export interface PlatformCredential {
  platform: string;
  client_id: string;
  created_at: string;
}

export async function listPlatformCredentials(
  token: string,
  workspaceId: string
): Promise<ApiResponse<PlatformCredential[]>> {
  return request(`/v1/workspaces/${workspaceId}/platform-credentials`, token);
}

export async function createPlatformCredential(
  token: string,
  workspaceId: string,
  data: { platform: string; client_id: string; client_secret: string }
): Promise<ApiResponse<PlatformCredential>> {
  return request(`/v1/workspaces/${workspaceId}/platform-credentials`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function deletePlatformCredential(
  token: string,
  workspaceId: string,
  platform: string
): Promise<void> {
  return request(
    `/v1/workspaces/${workspaceId}/platform-credentials/${platform}`,
    token,
    { method: "DELETE" }
  );
}

// API Keys (workspace-scoped)

export async function listApiKeys(
  token: string,
  workspaceId: string
): Promise<ApiResponse<ApiKey[]>> {
  return request(`/v1/workspaces/${workspaceId}/api-keys`, token);
}

export async function createApiKey(
  token: string,
  workspaceId: string,
  data: { name: string; environment?: string; expires_at?: string }
): Promise<ApiResponse<ApiKeyCreateResponse>> {
  return request(`/v1/workspaces/${workspaceId}/api-keys`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function revokeApiKey(
  token: string,
  workspaceId: string,
  keyId: string
): Promise<void> {
  return request(`/v1/workspaces/${workspaceId}/api-keys/${keyId}`, token, {
    method: "DELETE",
  });
}

// Social Accounts (profile-scoped)

export interface SocialAccount {
  id: string;
  profile_id: string;
  platform: string;
  account_name: string | null;
  connected_at: string;
  status: "active" | "reconnect_required" | "disconnected";
  connection_type: "byo" | "managed";
  external_user_id?: string;
  external_user_email?: string;
}

export async function listSocialAccounts(
  token: string,
  profileId: string,
  filters?: { external_user_id?: string; platform?: string }
): Promise<ApiResponse<SocialAccount[]>> {
  const qs = new URLSearchParams();
  if (filters?.external_user_id) qs.set("external_user_id", filters.external_user_id);
  if (filters?.platform) qs.set("platform", filters.platform);
  const suffix = qs.toString() ? `?${qs}` : "";
  return request(`/v1/profiles/${profileId}/social-accounts${suffix}`, token);
}

export async function connectSocialAccount(
  token: string,
  profileId: string,
  data: { platform: string; credentials: Record<string, string> }
): Promise<ApiResponse<SocialAccount>> {
  return request(`/v1/profiles/${profileId}/social-accounts/connect`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function disconnectSocialAccount(
  token: string,
  profileId: string,
  accountId: string
): Promise<void> {
  return request(
    `/v1/profiles/${profileId}/social-accounts/${accountId}`,
    token,
    { method: "DELETE" }
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
  published_at?: string;
}

export interface SocialPost {
  id: string;
  caption: string | null;
  media_urls?: string[];
  status: string;
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
  results?: SocialPostResult[];
}

export async function listSocialPosts(
  token: string,
  workspaceId: string
): Promise<ApiResponse<SocialPost[]>> {
  return request(`/v1/workspaces/${workspaceId}/social-posts`, token);
}

export async function archiveSocialPost(
  token: string,
  workspaceId: string,
  postId: string
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/workspaces/${workspaceId}/social-posts/${postId}/archive`, token, {
    method: "POST",
  });
}

export async function restoreSocialPost(
  token: string,
  workspaceId: string,
  postId: string
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/workspaces/${workspaceId}/social-posts/${postId}/restore`, token, {
    method: "POST",
  });
}

export async function deleteSocialPost(
  token: string,
  workspaceId: string,
  postId: string
): Promise<ApiResponse<{ deleted: boolean }>> {
  return request(`/v1/workspaces/${workspaceId}/social-posts/${postId}`, token, {
    method: "DELETE",
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
  workspaceId: string
): Promise<ApiResponse<BillingInfo>> {
  return request(`/v1/workspaces/${workspaceId}/billing`, token);
}

export async function createCheckout(
  token: string,
  workspaceId: string,
  planId: string
): Promise<ApiResponse<{ checkout_url: string }>> {
  return request(`/v1/workspaces/${workspaceId}/billing/checkout`, token, {
    method: "POST",
    body: JSON.stringify({ plan_id: planId }),
  });
}

export async function createPortal(
  token: string,
  workspaceId: string
): Promise<ApiResponse<{ portal_url: string }>> {
  return request(`/v1/workspaces/${workspaceId}/billing/portal`, token, {
    method: "POST",
  });
}

export async function listPlans(): Promise<ApiResponse<Plan[]>> {
  const res = await fetch(`${API_URL}/v1/plans`);
  return res.json();
}

export async function createSocialPost(
  token: string,
  workspaceId: string,
  data: CreateSocialPostPayload
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/workspaces/${workspaceId}/social-posts`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function validateSocialPost(
  token: string,
  workspaceId: string,
  data: CreateSocialPostPayload
): Promise<ApiResponse<SocialPostValidationResult>> {
  return request(`/v1/workspaces/${workspaceId}/social-posts/validate`, token, {
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
  workspaceId: string,
  mediaId: string
): Promise<ApiResponse<MediaUpload>> {
  return request(`/v1/workspaces/${workspaceId}/media/${mediaId}`, token);
}

export async function createMedia(
  token: string,
  workspaceId: string,
  data: { filename: string; content_type: string; size_bytes: number; content_hash?: string }
): Promise<ApiResponse<MediaUpload>> {
  return request(`/v1/workspaces/${workspaceId}/media`, token, {
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
  return request(`/v1/social-posts/${postId}`, token, {
    method: "PATCH",
    body: JSON.stringify({ scheduled_at: scheduledAt }),
  });
}

// Sprint 3 PR8: cancel a draft or scheduled post.
export async function cancelSocialPost(
  token: string,
  postId: string
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/social-posts/${postId}/cancel`, token, {
    method: "POST",
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
  return request(`/v1/social-posts/bulk`, token, {
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
  };
  reconnect_count: number;
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

// Connect sessions (Sprint 3 PR2 — multi-tenant Connect)

export interface ConnectSession {
  id: string;
  platform: "twitter" | "linkedin" | "bluesky";
  external_user_id: string;
  external_user_email?: string;
  return_url?: string;
  status: "pending" | "completed" | "expired" | "cancelled";
  url?: string;
  expires_at: string;
  created_at: string;
  completed_at?: string;
  completed_social_account_id?: string;
}

export async function createConnectSession(
  token: string,
  data: {
    platform: "twitter" | "linkedin" | "bluesky";
    external_user_id: string;
    external_user_email?: string;
    return_url?: string;
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
  workspaceId: string,
  postId: string,
  opts?: { refresh?: boolean }
): Promise<ApiResponse<PostAnalytics[]>> {
  const qs = opts?.refresh ? "?refresh=1" : "";
  return request(`/v1/workspaces/${workspaceId}/social-posts/${postId}/analytics${qs}`, token);
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

export interface AnalyticsRangeParams {
  start_date?: string; // YYYY-MM-DD
  end_date?: string;   // YYYY-MM-DD
  platform?: string;   // platform key, omit or "all" to disable
  status?: string;     // post status, omit or "all" to disable
}

function rangeQuery(params?: AnalyticsRangeParams & { metric?: string }): string {
  if (!params) return "";
  const qs = new URLSearchParams();
  if (params.start_date) qs.set("start_date", params.start_date);
  if (params.end_date) qs.set("end_date", params.end_date);
  if (params.platform && params.platform !== "all") qs.set("platform", params.platform);
  if (params.status && params.status !== "all") qs.set("status", params.status);
  if (params.metric) qs.set("metric", params.metric);
  const s = qs.toString();
  return s ? `?${s}` : "";
}

export async function getAnalyticsSummary(
  token: string,
  workspaceId: string,
  params?: AnalyticsRangeParams
): Promise<ApiResponse<AnalyticsSummary>> {
  return request(`/v1/workspaces/${workspaceId}/analytics/summary${rangeQuery(params)}`, token);
}

export async function getAnalyticsTrend(
  token: string,
  workspaceId: string,
  params?: AnalyticsRangeParams & { metric?: string }
): Promise<ApiResponse<AnalyticsTrend>> {
  return request(`/v1/workspaces/${workspaceId}/analytics/trend${rangeQuery(params)}`, token);
}

export async function getAnalyticsByPlatform(
  token: string,
  workspaceId: string,
  params?: AnalyticsRangeParams
): Promise<ApiResponse<PlatformAnalytics[]>> {
  return request(`/v1/workspaces/${workspaceId}/analytics/by-platform${rangeQuery(params)}`, token);
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
  last_visit_at: string | null;
}

export interface AdminLandingSourcesResponse {
  range_days: number;
  total_visits: number;
  unique_visitors: number;
  rows: AdminLandingSourceRow[];
}

export interface AdminUserRow {
  id: string;
  email: string;
  created_at: string;
  workspace_count: number;
  api_key_count: number;
  platform_count: number;
  platforms: string[];
  posts_used: number;
  post_limit: number;
  mrr_cents: number;
  is_paid: boolean;
  last_post_at: string | null;
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

export interface AdminUserPostFailure {
  post_id: string;
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
  platform?: string;
  source?: string;
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
  platform?: string;
  source?: string;
  days?: number;
  limit?: number;
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

// Whoami — returns the authenticated user's identity plus an
// is_admin flag derived from the backend ADMIN_USERS allowlist.
// Used by the dashboard shell to decide whether to show the Admin
// link, and by the admin page to gate client-side rendering.
export interface MeResponse {
  user_id: string;
  email: string;
  name?: string;
  is_admin: boolean;
  // Intent-collection redesign: the dashboard uses these to decide
  // whether to pop the Welcome modal on first load.
  onboarding_intent?: OnboardingIntent;
  onboarding_shown_at?: string;
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

export async function getAdminUser(
  token: string,
  id: string
): Promise<ApiResponse<AdminUserDetail>> {
  return request(`/v1/admin/users/${id}`, token);
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
  if (params?.platform) qs.set("platform", params.platform);
  if (params?.source) qs.set("source", params.source);
  if (params?.days != null) qs.set("days", String(params.days));
  if (params?.limit != null) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return request(`/v1/admin/post-failures${s ? `?${s}` : ""}`, token);
}

export async function listAdminPosts(
  token: string,
  params?: AdminPostListParams
): Promise<ApiResponse<AdminPostRow[]>> {
  const qs = new URLSearchParams();
  if (params?.search) qs.set("search", params.search);
  if (params?.status) qs.set("status", params.status);
  if (params?.platform) qs.set("platform", params.platform);
  if (params?.source) qs.set("source", params.source);
  if (params?.days != null) qs.set("days", String(params.days));
  if (params?.limit != null) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return request(`/v1/admin/posts${s ? `?${s}` : ""}`, token);
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

export async function recordLandingVisit(data: {
  path: string;
  source?: string;
  session_id: string;
  referrer?: string;
}): Promise<void> {
  await fetch(`${API_URL}/v1/public/landing-visit`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
    keepalive: true,
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
}

export interface APIMetricsOverall {
  total_calls: number;
  success_count: number;
  client_error_count: number;
  server_error_count: number;
  reliability_pct: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
}

export async function getAPIMetricsSummary(
  token: string,
  workspaceId: string,
  from: string,
  to: string
): Promise<ApiResponse<APIMetricsSummaryRow[]>> {
  return request(`/v1/workspaces/${workspaceId}/api-metrics/summary?from=${from}&to=${to}`, token);
}

export async function getAPIMetricsTrend(
  token: string,
  workspaceId: string,
  from: string,
  to: string
): Promise<ApiResponse<APIMetricsTrendRow[]>> {
  return request(`/v1/workspaces/${workspaceId}/api-metrics/trend?from=${from}&to=${to}`, token);
}

export async function getAPIMetricsOverall(
  token: string,
  workspaceId: string,
  from: string,
  to: string
): Promise<ApiResponse<APIMetricsOverall>> {
  return request(`/v1/workspaces/${workspaceId}/api-metrics/overall?from=${from}&to=${to}`, token);
}

// Inbox

export interface InboxItem {
  id: string;
  social_account_id: string;
  workspace_id: string;
  source: "ig_comment" | "ig_dm" | "threads_reply";
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
  workspaceId: string,
  filters?: { source?: string; is_read?: string }
): Promise<ApiResponse<InboxItem[]>> {
  const qs = new URLSearchParams();
  if (filters?.source) qs.set("source", filters.source);
  if (filters?.is_read) qs.set("is_read", filters.is_read);
  const q = qs.toString();
  return request(`/v1/workspaces/${workspaceId}/inbox${q ? `?${q}` : ""}`, token);
}

export async function getInboxUnreadCount(
  token: string,
  workspaceId: string
): Promise<ApiResponse<{ count: number }>> {
  return request(`/v1/workspaces/${workspaceId}/inbox/unread-count`, token);
}

export async function markInboxItemRead(
  token: string,
  workspaceId: string,
  id: string
): Promise<void> {
  return request(`/v1/workspaces/${workspaceId}/inbox/${id}/read`, token, {
    method: "POST",
  });
}

export async function markAllInboxRead(
  token: string,
  workspaceId: string
): Promise<ApiResponse<{ marked: number }>> {
  return request(`/v1/workspaces/${workspaceId}/inbox/mark-all-read`, token, {
    method: "POST",
  });
}

export async function replyToInboxItem(
  token: string,
  workspaceId: string,
  id: string,
  text: string
): Promise<ApiResponse<InboxItem>> {
  return request(`/v1/workspaces/${workspaceId}/inbox/${id}/reply`, token, {
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
  workspaceId: string,
  inboxItemId: string
): Promise<ApiResponse<IGMediaContext>> {
  return request(`/v1/workspaces/${workspaceId}/inbox/${inboxItemId}/media-context`, token);
}

export async function updateInboxThreadState(
  token: string,
  workspaceId: string,
  id: string,
  data: { thread_status: "open" | "assigned" | "resolved"; assigned_to?: string }
): Promise<ApiResponse<InboxItem>> {
  return request(`/v1/workspaces/${workspaceId}/inbox/${id}/thread-state`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function syncInbox(
  token: string,
  workspaceId: string
): Promise<ApiResponse<{ new_items: number }>> {
  return request(`/v1/workspaces/${workspaceId}/inbox/sync`, token, {
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
