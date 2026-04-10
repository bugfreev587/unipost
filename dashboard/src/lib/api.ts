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

// Profiles

export async function listProfiles(
  token: string
): Promise<ApiResponse<Profile[]>> {
  return request("/v1/profiles", token);
}

export async function getProfile(
  token: string,
  id: string
): Promise<ApiResponse<Profile>> {
  return request(`/v1/profiles/${id}`, token);
}

export async function createProfile(
  token: string,
  data: { name: string }
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
  }
): Promise<ApiResponse<Profile>> {
  return request(`/v1/profiles/${id}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function deleteProfile(
  token: string,
  id: string
): Promise<void> {
  return request(`/v1/profiles/${id}`, token, { method: "DELETE" });
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
  error_message?: string;
  published_at?: string;
}

export interface SocialPost {
  id: string;
  caption: string | null;
  status: string;
  scheduled_at?: string;
  created_at: string;
  published_at?: string;
  results?: SocialPostResult[];
}

export async function listSocialPosts(
  token: string,
  workspaceId: string
): Promise<ApiResponse<SocialPost[]>> {
  return request(`/v1/workspaces/${workspaceId}/social-posts`, token);
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
  data: { caption: string; account_ids: string[]; media_urls?: string[]; scheduled_at?: string }
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/workspaces/${workspaceId}/social-posts`, token, {
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

export interface AdminUserListParams {
  search?: string;
  plan?: "all" | "free" | "paid";
  sort?: "newest" | "mrr" | "usage" | "last_active";
  limit?: number;
  offset?: number;
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
}

export async function getMe(token: string): Promise<ApiResponse<MeResponse>> {
  return request("/v1/me", token);
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
  data: { first_name: string; org_name?: string; usage_modes: string[] }
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
