import { validateScript } from "./script-contract.js";

export const DEFAULT_API_URL = "https://api.unipost.dev";

export async function fetchReviewScript({ token, apiUrl = DEFAULT_API_URL, fetchImpl = globalThis.fetch } = {}) {
  if (!token || typeof token !== "string") {
    throw new Error("review agent token is required");
  }
  if (typeof fetchImpl !== "function") {
    throw new Error("fetch is not available in this Node runtime");
  }
  const base = apiUrl.replace(/\/+$/, "");
  const response = await fetchImpl(`${base}/v1/review/agent/script`, {
    method: "GET",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
  });
  if (!response.ok) {
    let message = `script request failed: ${response.status}`;
    try {
      const body = await response.json();
      message = body?.error?.message || message;
    } catch {
      // Ignore malformed error bodies; status is still actionable.
    }
    throw new Error(message);
  }
  const body = await response.json();
  return validateScript(body.data);
}
