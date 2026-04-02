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
