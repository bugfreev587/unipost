import { validateScript } from "./script-contract.js";

export const DEFAULT_API_URL = "https://api.unipost.dev";

export async function fetchReviewScript({ token, apiUrl = DEFAULT_API_URL, fetchImpl = globalThis.fetch } = {}) {
  const body = await agentRequest("/v1/review/agent/script", { token, apiUrl, fetchImpl });
  return validateScript(body.data);
}

export async function postReviewEvent({ token, apiUrl = DEFAULT_API_URL, fetchImpl = globalThis.fetch, event } = {}) {
  return agentRequest("/v1/review/agent/events", { token, apiUrl, fetchImpl, method: "POST", body: event });
}

export async function completeReviewJob({ token, apiUrl = DEFAULT_API_URL, fetchImpl = globalThis.fetch, data } = {}) {
  return agentRequest("/v1/review/agent/complete", { token, apiUrl, fetchImpl, method: "POST", body: data });
}

export async function failReviewJob({ token, apiUrl = DEFAULT_API_URL, fetchImpl = globalThis.fetch, data } = {}) {
  return agentRequest("/v1/review/agent/fail", { token, apiUrl, fetchImpl, method: "POST", body: data });
}

export async function createReviewArtifactUpload({ token, apiUrl = DEFAULT_API_URL, fetchImpl = globalThis.fetch, artifact } = {}) {
  const body = await agentRequest("/v1/review/agent/artifacts", { token, apiUrl, fetchImpl, method: "POST", body: artifact });
  return body.data;
}

export async function putReviewArtifact({ upload, bytes, fetchImpl = globalThis.fetch } = {}) {
  if (!upload?.upload_url) {
    throw new Error("review artifact upload URL is required");
  }
  const response = await fetchImpl(upload.upload_url, {
    method: upload.method || "PUT",
    headers: upload.headers || {},
    body: bytes,
  });
  if (!response.ok) {
    throw new Error("review artifact upload failed: " + response.status);
  }
}

async function agentRequest(path, { token, apiUrl = DEFAULT_API_URL, fetchImpl = globalThis.fetch, method = "GET", body } = {}) {
  if (!token || typeof token !== "string") {
    throw new Error("review agent token is required");
  }
  if (typeof fetchImpl !== "function") {
    throw new Error("fetch is not available in this Node runtime");
  }
  const base = apiUrl.replace(/\/+$/, "");
  const response = await fetchImpl(base + path, {
    method,
    headers: {
      Accept: "application/json",
      ...(body ? { "Content-Type": "application/json" } : {}),
      Authorization: "Bearer " + token,
    },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
  if (!response.ok) {
    let message = path + " request failed: " + response.status;
    try {
      const errorBody = await response.json();
      message = errorBody?.error?.message || message;
    } catch {
      // Ignore malformed error bodies; status is still actionable.
    }
    throw new Error(message);
  }
  return response.json();
}
