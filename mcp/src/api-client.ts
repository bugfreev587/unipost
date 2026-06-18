export class UniPostApiError extends Error {
  status: number;
  code: string;
  normalizedCode: string;
  requestId: string;
  issues: any[];
  body: any;

  constructor(args: {
    status: number;
    message: string;
    code?: string;
    normalizedCode?: string;
    requestId?: string;
    issues?: any[];
    body?: any;
  }) {
    super(formatApiErrorMessage(args));
    this.name = "UniPostApiError";
    this.status = args.status;
    this.code = args.code || "";
    this.normalizedCode = args.normalizedCode || args.code || "";
    this.requestId = args.requestId || "";
    this.issues = args.issues || [];
    this.body = args.body;
  }
}

function formatApiErrorMessage(args: {
  message: string;
  code?: string;
  normalizedCode?: string;
  requestId?: string;
  issues?: any[];
}): string {
  const parts = [args.message];
  const normalizedCode = args.normalizedCode || args.code;
  if (normalizedCode) parts.push(`normalized_code=${normalizedCode}`);
  if (args.code && args.code !== normalizedCode) parts.push(`code=${args.code}`);
  if (args.requestId) parts.push(`request_id=${args.requestId}`);

  const issueLines = (args.issues || []).slice(0, 3).map((issue) => {
    const field = issue?.field ? `${issue.field}: ` : "";
    const message = issue?.message || issue?.code || "validation issue";
    const code = issue?.code ? ` (code=${issue.code})` : "";
    return `- ${field}${message}${code}`;
  });
  if (issueLines.length > 0) {
    parts.push(`issues:\n${issueLines.join("\n")}`);
  }

  return parts.join("\n");
}

export function canonicalizeApiPath(path: string): string {
  if (path === "/v1/social-posts") {
    return "/v1/posts";
  }
  if (path.startsWith("/v1/social-posts?")) {
    return "/v1/posts" + path.slice("/v1/social-posts".length);
  }
  if (path.startsWith("/v1/social-posts/")) {
    return "/v1/posts/" + path.slice("/v1/social-posts/".length);
  }
  return path;
}

async function readJsonBody(res: Response): Promise<any> {
  const text = await res.text();
  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    return { raw: text };
  }
}

export async function apiRequest(
  apiUrl: string,
  path: string,
  apiKey: string,
  options?: RequestInit
) {
  const res = await fetch(`${apiUrl}${canonicalizeApiPath(path)}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${apiKey}`,
      ...options?.headers,
    },
  });
  const body = await readJsonBody(res);
  if (!res.ok) {
    const backend = body?.error || {};
    throw new UniPostApiError({
      status: res.status,
      message: backend.message || `UniPost API returned HTTP ${res.status}.`,
      code: backend.code,
      normalizedCode: backend.normalized_code || backend.code,
      requestId: body?.request_id || res.headers.get("x-request-id") || "",
      issues: Array.isArray(backend.issues) ? backend.issues : [],
      body,
    });
  }
  return body;
}
