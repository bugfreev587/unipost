class UniPostError extends Error {
  constructor(message, status, code) {
    super(message);
    this.name = "UniPostError";
    this.status = status;
    this.code = code;
  }
}

class AuthError extends UniPostError {
  constructor(message = "Authentication failed") {
    super(message, 401, "auth_error");
    this.name = "AuthError";
  }
}

class NotFoundError extends UniPostError {
  constructor(message = "Resource not found") {
    super(message, 404, "not_found");
    this.name = "NotFoundError";
  }
}

class ValidationError extends UniPostError {
  constructor(message = "Validation failed", errors = {}) {
    super(message, 422, "validation_error");
    this.name = "ValidationError";
    this.errors = errors;
  }
}

class RateLimitError extends UniPostError {
  constructor(retryAfter, message = "Rate limit exceeded") {
    super(message, 429, "rate_limit");
    this.name = "RateLimitError";
    this.retryAfter = retryAfter;
  }
}

class PlatformError extends UniPostError {
  constructor(message, platform) {
    super(message, 502, "platform_error");
    this.name = "PlatformError";
    this.platform = platform;
  }
}

class QuotaError extends UniPostError {
  constructor(message = "Monthly quota exceeded") {
    super(message, 403, "quota_exceeded");
    this.name = "QuotaError";
  }
}

function parseApiError(status, body) {
  const msg = body?.error?.message || "Unknown API error";
  const code = body?.error?.code || "unknown";

  switch (status) {
    case 401:
      return new AuthError(msg);
    case 404:
      return new NotFoundError(msg);
    case 422:
      return new ValidationError(msg, body?.error?.errors || {});
    case 429:
      return new RateLimitError(parseInt(body?.error?.retry_after || "1", 10), msg);
    case 403:
      if (code === "quota_exceeded") return new QuotaError(msg);
      return new UniPostError(msg, status, code);
    case 502:
      if (body?.error?.platform) return new PlatformError(msg, body.error.platform);
      return new UniPostError(msg, status, code);
    default:
      return new UniPostError(msg, status, code);
  }
}

const MAX_RETRIES = 2;
const SDK_VERSION = "@unipost/sdk/0.2.0-local";

class HttpClient {
  constructor(options) {
    this.apiKey = options.apiKey;
    this.baseUrl = options.baseUrl;
    this.timeout = options.timeout;
  }

  async request(method, path, options = {}) {
    const url = new URL(path, this.baseUrl);
    if (options.query) {
      for (const [key, value] of Object.entries(options.query)) {
        if (value !== undefined && value !== null) {
          url.searchParams.set(key, String(value));
        }
      }
    }

    const headers = {
      Authorization: `Bearer ${this.apiKey}`,
      "User-Agent": SDK_VERSION,
      ...options.headers,
    };

    if (options.body !== undefined && !headers["Content-Type"]) {
      headers["Content-Type"] = "application/json";
    }

    const init = {
      method,
      headers,
      signal: AbortSignal.timeout(this.timeout),
    };

    if (options.body !== undefined) {
      init.body = JSON.stringify(options.body);
    }

    let lastError = null;
    for (let attempt = 0; attempt <= MAX_RETRIES; attempt += 1) {
      try {
        const response = await fetch(url.toString(), init);

        if (response.ok) {
          if (response.status === 204) return undefined;
          const text = await response.text();
          return text ? JSON.parse(text) : undefined;
        }

        if (response.status === 429) {
          const retryAfter = parseInt(response.headers.get("Retry-After") || "1", 10);
          if (attempt < MAX_RETRIES) {
            await sleep(retryAfter * 1000);
            continue;
          }
          throw new RateLimitError(retryAfter);
        }

        const body = await response.json().catch(() => ({}));
        throw parseApiError(response.status, body);
      } catch (error) {
        if (error instanceof RateLimitError && attempt < MAX_RETRIES) {
          await sleep(error.retryAfter * 1000);
          lastError = error;
          continue;
        }
        throw error;
      }
    }

    throw lastError || new Error("Request failed after retries");
  }

  get(path, query) {
    return this.request("GET", path, { query });
  }

  post(path, body, headers) {
    return this.request("POST", path, { body, headers });
  }

  patch(path, body) {
    return this.request("PATCH", path, { body });
  }

  put(path, body) {
    return this.request("PUT", path, { body });
  }

  delete(path) {
    return this.request("DELETE", path);
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function toSnakeCase(params = {}) {
  const body = {};
  if (params.caption !== undefined) body.caption = params.caption;
  if (params.accountIds) body.account_ids = params.accountIds;
  if (params.mediaUrls) body.media_urls = params.mediaUrls;
  if (params.mediaIds) body.media_ids = params.mediaIds;
  if (params.scheduledAt) body.scheduled_at = params.scheduledAt;
  if (params.status) body.status = params.status;
  if (params.platformPosts) {
    body.platform_posts = params.platformPosts.map((post) => {
      const entry = {
        account_id: post.accountId,
      };
      if (post.caption !== undefined) entry.caption = post.caption;
      if (post.mediaUrls) entry.media_urls = post.mediaUrls;
      if (post.mediaIds) entry.media_ids = post.mediaIds;
      if (post.threadPosition !== undefined) entry.thread_position = post.threadPosition;
      if (post.firstComment !== undefined) entry.first_comment = post.firstComment;
      if (post.inReplyTo !== undefined) entry.in_reply_to = post.inReplyTo;
      if (post.platformOptions !== undefined) entry.platform_options = post.platformOptions;
      return entry;
    });
  }
  return body;
}

class Accounts {
  constructor(http) {
    this.http = http;
  }

  async list(params) {
    const query = {};
    if (params?.platform) query.platform = params.platform;
    if (params?.externalUserId) query.external_user_id = params.externalUserId;
    if (params?.status) query.status = params.status;
    if (params?.profileId) query.profile_id = params.profileId;
    return this.http.get("/v1/social-accounts", query);
  }

  async get(accountId) {
    const response = await this.http.get(`/v1/social-accounts/${accountId}`);
    return response.data;
  }

  async health(accountId) {
    const response = await this.http.get(`/v1/social-accounts/${accountId}/health`);
    return response.data;
  }
}

class Posts {
  constructor(http) {
    this.http = http;
  }

  async create(params) {
    const headers = {};
    if (params.idempotencyKey) {
      headers["Idempotency-Key"] = params.idempotencyKey;
    }
    const response = await this.http.post("/v1/social-posts", toSnakeCase(params), headers);
    return response.data;
  }

  async list(params) {
    const query = {};
    if (params?.status) query.status = params.status;
    if (params?.platform) query.platform = params.platform;
    if (params?.from) query.from = params.from;
    if (params?.to) query.to = params.to;
    if (params?.limit) query.limit = params.limit;
    if (params?.cursor) query.cursor = params.cursor;
    return this.http.get("/v1/social-posts", query);
  }

  async *listAll(params) {
    let cursor;
    do {
      const page = await this.list({ ...params, cursor });
      for (const post of page.data || []) {
        yield post;
      }
      cursor = page.nextCursor;
    } while (cursor);
  }

  async get(postId) {
    const response = await this.http.get(`/v1/social-posts/${postId}`);
    return response.data;
  }

  async getQueue(postId) {
    const response = await this.http.get(`/v1/social-posts/${postId}/queue`);
    return response.data;
  }

  async analytics(postId) {
    const response = await this.http.get(`/v1/social-posts/${postId}/analytics`);
    return response.data;
  }

  async publish(postId) {
    const response = await this.http.post(`/v1/social-posts/${postId}/publish`);
    return response.data;
  }

  async cancel(postId) {
    const response = await this.http.post(`/v1/social-posts/${postId}/cancel`);
    return response.data;
  }

  async retryResult(postId, resultId) {
    const response = await this.http.post(`/v1/social-posts/${postId}/results/${resultId}/retry`);
    return response.data;
  }

  async bulkCreate(posts) {
    const body = posts.map((post) => toSnakeCase(post));
    const response = await this.http.post("/v1/social-posts/bulk", body);
    return response.data;
  }
}

class Media {
  constructor(http) {
    this.http = http;
  }

  async upload(params) {
    const response = await this.http.post("/v1/media", {
      filename: params.filename,
      content_type: params.contentType,
      size_bytes: params.sizeBytes,
      content_hash: params.contentHash,
    });
    return normalizeMediaUpload(response.data);
  }

  async get(mediaId) {
    const response = await this.http.get(`/v1/media/${mediaId}`);
    return response.data;
  }

  async delete(mediaId) {
    const response = await this.http.delete(`/v1/media/${mediaId}`);
    return response?.data ?? response;
  }

  async uploadFile(filePath) {
    const { readFileSync, statSync } = await import("fs");
    const { basename } = await import("path");
    const stats = statSync(filePath);
    const filename = basename(filePath);
    const ext = filename.split(".").pop()?.toLowerCase() || "";
    const contentType = MIME_TYPES[ext] || "application/octet-stream";
    const { mediaId, uploadUrl } = await this.upload({
      filename,
      contentType,
      sizeBytes: stats.size,
    });

    const fileBuffer = readFileSync(filePath);
    const uploadResponse = await fetch(uploadUrl, {
      method: "PUT",
      body: fileBuffer,
      headers: { "Content-Type": contentType },
    });
    if (!uploadResponse.ok) {
      throw new Error(`Media upload failed with status ${uploadResponse.status}`);
    }

    return mediaId;
  }
}

class Analytics {
  constructor(http) {
    this.http = http;
  }

  async rollup(params) {
    const response = await this.http.get("/v1/analytics/rollup", {
      from: params.from,
      to: params.to,
      granularity: params.granularity,
      group_by: params.groupBy,
    });
    return response.data;
  }
}

class Connect {
  constructor(http) {
    this.http = http;
  }

  async createSession(params) {
    const response = await this.http.post("/v1/connect/sessions", {
      platform: params.platform,
      profile_id: params.profileId,
      external_user_id: params.externalUserId,
      external_user_email: params.externalUserEmail,
      return_url: params.returnUrl,
    });
    return response.data;
  }

  async getSession(sessionId) {
    const response = await this.http.get(`/v1/connect/sessions/${sessionId}`);
    return response.data;
  }
}

class Users {
  constructor(http) {
    this.http = http;
  }

  async list() {
    return this.http.get("/v1/users");
  }

  async get(externalUserId) {
    const response = await this.http.get(`/v1/users/${externalUserId}`);
    return response.data;
  }
}

class Webhooks {
  constructor(http) {
    this.http = http;
  }

  async create(params) {
    const response = await this.http.post("/v1/webhooks", {
      url: params.url,
      events: params.events,
    });
    return response.data;
  }

  async list() {
    return this.http.get("/v1/webhooks");
  }

  async get(webhookId) {
    const response = await this.http.get(`/v1/webhooks/${webhookId}`);
    return response.data;
  }

  async update(webhookId, params) {
    const response = await this.http.patch(`/v1/webhooks/${webhookId}`, {
      url: params.url,
      events: params.events,
      active: params.active,
    });
    return response.data;
  }

  async rotate(webhookId) {
    const response = await this.http.post(`/v1/webhooks/${webhookId}/rotate`);
    return response.data;
  }

  async delete(webhookId) {
    const response = await this.http.delete(`/v1/webhooks/${webhookId}`);
    return response.data;
  }
}

const DEFAULT_BASE_URL = "https://api.unipost.dev";
const DEFAULT_TIMEOUT = 30000;

class UniPost {
  constructor(options = {}) {
    const apiKey = options.apiKey ?? getEnvVar("UNIPOST_API_KEY");
    if (!apiKey) {
      throw new Error("UniPost API key is required. Pass `new UniPost({ apiKey })` or set UNIPOST_API_KEY.");
    }

    const http = new HttpClient({
      apiKey,
      baseUrl: options.baseUrl ?? DEFAULT_BASE_URL,
      timeout: options.timeout ?? DEFAULT_TIMEOUT,
    });

    this.accounts = new Accounts(http);
    this.posts = new Posts(http);
    this.media = new Media(http);
    this.analytics = new Analytics(http);
    this.connect = new Connect(http);
    this.users = new Users(http);
    this.webhooks = new Webhooks(http);
  }
}

function getEnvVar(name) {
  if (typeof process !== "undefined" && process.env) {
    return process.env[name];
  }
  if (typeof globalThis !== "undefined" && "Deno" in globalThis) {
    try {
      return globalThis.Deno.env.get(name);
    } catch {
      return undefined;
    }
  }
  return undefined;
}

function normalizeMediaUpload(data) {
  if (!data) return data;
  return {
    ...data,
    mediaId: data.media_id,
    uploadUrl: data.upload_url,
  };
}

const MIME_TYPES = {
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  png: "image/png",
  gif: "image/gif",
  webp: "image/webp",
  mp4: "video/mp4",
  mov: "video/quicktime",
  webm: "video/webm",
};

async function verifyWebhookSignature(options) {
  const { payload, signature, secret } = options;
  if (!signature || !secret) return false;

  const normalizedSignature = String(signature).trim().replace(/^sha256=/i, "");
  const encoder = new TextEncoder();
  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"]
  );
  const payloadBytes = typeof payload === "string" ? encoder.encode(payload) : new Uint8Array(payload);
  const signed = await crypto.subtle.sign("HMAC", key, payloadBytes);
  const computedSignature = bufferToHex(signed);
  return timingSafeEqual(computedSignature, normalizedSignature);
}

function bufferToHex(buffer) {
  return Array.from(new Uint8Array(buffer))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}

function timingSafeEqual(a, b) {
  if (a.length !== b.length) return false;
  let result = 0;
  for (let index = 0; index < a.length; index += 1) {
    result |= a.charCodeAt(index) ^ b.charCodeAt(index);
  }
  return result === 0;
}

export {
  AuthError,
  NotFoundError,
  PlatformError,
  QuotaError,
  RateLimitError,
  UniPost,
  UniPostError,
  ValidationError,
  verifyWebhookSignature,
};
