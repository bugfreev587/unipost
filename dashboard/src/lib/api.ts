const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Types

export interface Project {
  id: string;
  owner_id: string;
  name: string;
  mode: "quickstart" | "whitelabel";
  created_at: string;
  updated_at: string;
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
    const err: ApiError = await res.json();
    throw new Error(err.error?.message || `Request failed: ${res.status}`);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json();
}

// Projects

export async function listProjects(
  token: string
): Promise<ApiResponse<Project[]>> {
  return request("/v1/projects", token);
}

export async function getProject(
  token: string,
  id: string
): Promise<ApiResponse<Project>> {
  return request(`/v1/projects/${id}`, token);
}

export async function createProject(
  token: string,
  data: { name: string; mode?: string }
): Promise<ApiResponse<Project>> {
  return request("/v1/projects", token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateProject(
  token: string,
  id: string,
  data: { name: string }
): Promise<ApiResponse<Project>> {
  return request(`/v1/projects/${id}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function deleteProject(
  token: string,
  id: string
): Promise<void> {
  return request(`/v1/projects/${id}`, token, { method: "DELETE" });
}

// API Keys

export async function listApiKeys(
  token: string,
  projectId: string
): Promise<ApiResponse<ApiKey[]>> {
  return request(`/v1/projects/${projectId}/api-keys`, token);
}

export async function createApiKey(
  token: string,
  projectId: string,
  data: { name: string; environment?: string; expires_at?: string }
): Promise<ApiResponse<ApiKeyCreateResponse>> {
  return request(`/v1/projects/${projectId}/api-keys`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function revokeApiKey(
  token: string,
  projectId: string,
  keyId: string
): Promise<void> {
  return request(`/v1/projects/${projectId}/api-keys/${keyId}`, token, {
    method: "DELETE",
  });
}

// Social Accounts

export interface SocialAccount {
  id: string;
  platform: string;
  account_name: string | null;
  connected_at: string;
  status: "active" | "reconnect_required";
}

export async function listSocialAccounts(
  token: string,
  projectId: string
): Promise<ApiResponse<SocialAccount[]>> {
  return request(`/v1/projects/${projectId}/social-accounts`, token);
}

export async function connectSocialAccount(
  token: string,
  projectId: string,
  data: { platform: string; credentials: Record<string, string> }
): Promise<ApiResponse<SocialAccount>> {
  return request(`/v1/projects/${projectId}/social-accounts/connect`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function disconnectSocialAccount(
  token: string,
  projectId: string,
  accountId: string
): Promise<void> {
  return request(
    `/v1/projects/${projectId}/social-accounts/${accountId}`,
    token,
    { method: "DELETE" }
  );
}

export async function getOAuthConnectURL(
  token: string,
  projectId: string,
  platform: string,
  redirectUrl: string
): Promise<ApiResponse<{ auth_url: string }>> {
  const params = new URLSearchParams({ redirect_url: redirectUrl });
  return request(
    `/v1/projects/${projectId}/oauth/connect/${platform}?${params}`,
    token
  );
}

// Social Posts

export interface SocialPostResult {
  social_account_id: string;
  platform?: string;
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
  projectId: string
): Promise<ApiResponse<SocialPost[]>> {
  return request(`/v1/projects/${projectId}/social-posts`, token);
}

// Billing

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
  projectId: string
): Promise<ApiResponse<BillingInfo>> {
  return request(`/v1/projects/${projectId}/billing`, token);
}

export async function createCheckout(
  token: string,
  projectId: string,
  planId: string
): Promise<ApiResponse<{ checkout_url: string }>> {
  return request(`/v1/projects/${projectId}/billing/checkout`, token, {
    method: "POST",
    body: JSON.stringify({ plan_id: planId }),
  });
}

export async function createPortal(
  token: string,
  projectId: string
): Promise<ApiResponse<{ portal_url: string }>> {
  return request(`/v1/projects/${projectId}/billing/portal`, token, {
    method: "POST",
  });
}

export async function listPlans(): Promise<ApiResponse<Plan[]>> {
  const res = await fetch(`${API_URL}/v1/plans`);
  return res.json();
}

export async function createSocialPost(
  token: string,
  projectId: string,
  data: { caption: string; account_ids: string[]; media_urls?: string[]; scheduled_at?: string }
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/projects/${projectId}/social-posts`, token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// Analytics

export interface PostAnalytics {
  post_id: string;
  social_account_id: string;
  platform: string;
  external_id: string;
  views: number;
  likes: number;
  comments: number;
  shares: number;
  reach: number;
  impressions: number;
  engagement_rate: number;
  fetched_at: string;
}

export async function getPostAnalytics(
  token: string,
  projectId: string,
  postId: string
): Promise<ApiResponse<PostAnalytics[]>> {
  return request(`/v1/projects/${projectId}/social-posts/${postId}/analytics`, token);
}
