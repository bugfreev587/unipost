import type { SocialPostResult } from "./api";

export type PostResultFailureInput = Pick<
  SocialPostResult,
  | "status"
  | "platform"
  | "error_message"
  | "error_code"
  | "failure_stage"
  | "platform_error_code"
  | "is_retriable"
  | "next_action"
  | "error_source"
  | "error_temporality"
  | "provider_error"
  | "retry_policy"
>;

export type PostResultFailureDescription = {
  title: string;
  message: string;
  nextActionLabel?: string;
  retryStatusLabel?: string;
  actionHref?: string;
  canRetry: boolean;
};

const NEXT_ACTION_LABELS: Record<string, string> = {
  fix_request: "Fix request",
  edit_caption: "Edit caption",
  fix_media: "Fix media",
  review_platform_options: "Review platform settings",
  reconnect_account: "Reconnect account",
  reconnect_or_update_permissions: "Reconnect account",
  select_valid_target: "Select a valid target",
  wait_and_retry: "Wait and retry",
  retry_later: "Retry later",
  review_quota: "Review quota",
  contact_support: "Contact support",
};

function platformName(platform?: string): string {
  if (!platform) return "Platform";
  switch (platform.toLowerCase()) {
    case "twitter":
      return "X";
    case "tiktok":
      return "TikTok";
    case "youtube":
      return "YouTube";
    case "linkedin":
      return "LinkedIn";
    default:
      return platform.charAt(0).toUpperCase() + platform.slice(1);
  }
}

function titleForErrorCode(errorCode: string | undefined, platform?: string): string {
  const name = platformName(platform);
  switch (errorCode) {
    case "validation_error":
      return "Request needs changes";
    case "platform_request_invalid":
      return `${name} rejected the publish request`;
    case "media_error":
      return "Media needs changes";
    case "temporary_platform_error":
      return `${name} is temporarily unavailable`;
    case "rate_limit":
      return "Rate limit reached";
    case "quota_exceeded":
      return "Quota exceeded";
    case "account_reconnect_required":
    case "auth_token_invalid":
      return "Reconnect account";
    case "missing_permission":
      return "Update account permissions";
    case "target_not_found":
      return "Destination not found";
    case "unknown_error":
    case "platform_error":
      return "Publish failed";
    default:
      return "Publish failed";
  }
}

function actionHrefFor(nextAction?: string): string | undefined {
  switch (nextAction) {
    case "reconnect_account":
    case "reconnect_or_update_permissions":
      return "/projects/:id/accounts";
    default:
      return undefined;
  }
}

function legacyDescription(result: PostResultFailureInput): PostResultFailureDescription {
  const message = result.error_message || "Publish failed (no error message reported).";
  const e = message.toLowerCase();
  if (e.includes("account is disconnected") || e.includes("account not found")) {
    return {
      title: "Reconnect account",
      message,
      nextActionLabel: "Reconnect account",
      actionHref: "/projects/:id/accounts",
      canRetry: false,
    };
  }
  if (e.includes("token") && (e.includes("expired") || e.includes("invalid") || e.includes("revoked") || e.includes("unauthorized"))) {
    return {
      title: "Reconnect account",
      message,
      nextActionLabel: "Reconnect account",
      actionHref: "/projects/:id/accounts",
      canRetry: false,
    };
  }
  if (e.includes("rate limit") || e.includes("too many requests") || e.includes("429")) {
    return {
      title: "Rate limit reached",
      message,
      nextActionLabel: "Wait and retry",
      canRetry: true,
    };
  }
  if (e.includes("instagram requires at least one")) {
    return {
      title: "Media required",
      message,
      nextActionLabel: "Fix media",
      canRetry: false,
    };
  }
  if (e.includes("duplicate") || e.includes("duplicate_post")) {
    return {
      title: "Duplicate post",
      message,
      nextActionLabel: "Edit content",
      canRetry: false,
    };
  }
  return {
    title: "Publish failed",
    message,
    canRetry: true,
  };
}

function providerCodeLabel(result: PostResultFailureInput): string | undefined {
  const provider = result.provider_error;
  const code = provider?.code || provider?.reason || result.platform_error_code;
  if (!code) return undefined;
  const parts = [code];
  if (provider?.subcode && provider.subcode !== code) parts.push(`subcode ${provider.subcode}`);
  if (provider?.http_status) parts.push(`HTTP ${provider.http_status}`);
  return parts.join(", ");
}

function safeErrorDetail(message?: string): string | undefined {
  const trimmed = (message || "").trim();
  if (!trimmed) return undefined;
  if (trimmed.includes('{"error"') || trimmed.includes("{\"error\"")) {
    return "Provider returned a structured error payload.";
  }
  return trimmed.length > 220 ? `${trimmed.slice(0, 217)}...` : trimmed;
}

function retryStatusLabel(result: PostResultFailureInput): string | undefined {
  const policy = result.retry_policy;
  if (!policy) {
    return result.is_retriable ? "Retry may help." : undefined;
  }
  if (policy.will_retry) {
    if (policy.retry_state === "running") return "UniPost is retrying now.";
    if (policy.next_run_at) return `UniPost will retry automatically at ${new Date(policy.next_run_at).toLocaleString()}.`;
    return "UniPost will retry automatically.";
  }
  switch (policy.retry_state) {
    case "exhausted":
      return "Automatic retries are exhausted.";
    case "blocked":
      return "Automatic retry is blocked.";
    case "manual_only":
      return policy.manual_retry_allowed ? "Manual retry is available." : "Manual retry is not available right now.";
    case "not_retriable":
      return policy.manual_retry_allowed ? "Manual retry is available after you address the issue." : "Automatic retry is not scheduled.";
    default:
      return policy.manual_retry_allowed ? "Manual retry is available." : "Retry state is unknown.";
  }
}

function structuredMessage(result: PostResultFailureInput): string | undefined {
  const name = platformName(result.platform);
  const source = result.error_source;
  const temporality = result.error_temporality;
  const policy = result.retry_policy;

  if (temporality === "temporary" && source === "platform") {
    if (policy?.will_retry) return `${name} had a temporary official-platform error. UniPost will retry automatically.`;
    if (policy?.retry_state === "exhausted") return `${name} had a temporary official-platform error, but automatic retries are exhausted.`;
    return `${name} had a temporary official-platform error.`;
  }
  if (temporality === "temporary" && source === "worker") {
    if (policy?.will_retry) return "A UniPost worker hit a temporary issue. UniPost will retry automatically.";
    return "A UniPost worker hit a temporary issue.";
  }
  if (result.next_action === "reconnect_account" || result.next_action === "reconnect_or_update_permissions") {
    return "Reconnect this account before retrying.";
  }
  if (temporality === "permanent") {
    return source === "platform"
      ? `${name} rejected the request. This needs a change before retrying.`
      : "This request needs a change before retrying.";
  }
  if (temporality === "unknown" || source === "unknown") {
    return "UniPost could not classify this failure. Contact support with the request ID.";
  }
  return undefined;
}

export function describePostResultFailure(result: PostResultFailureInput): PostResultFailureDescription {
  if (!result.error_code && !result.next_action && typeof result.is_retriable !== "boolean") {
    return legacyDescription(result);
  }

  const messageParts = [structuredMessage(result) || safeErrorDetail(result.error_message) || "Publish failed."];
  const provider = providerCodeLabel(result);
  if (provider) {
    messageParts.push(`Provider error: ${provider}.`);
  }
  const detail = safeErrorDetail(result.error_message);
  if (detail && detail !== messageParts[0]) {
    messageParts.push(`Details: ${detail}`);
  }

  return {
    title: titleForErrorCode(result.error_code, result.platform),
    message: messageParts.join(" "),
    nextActionLabel: result.next_action ? NEXT_ACTION_LABELS[result.next_action] || result.next_action : undefined,
    retryStatusLabel: retryStatusLabel(result),
    actionHref: actionHrefFor(result.next_action),
    canRetry: result.retry_policy?.manual_retry_allowed ?? result.is_retriable === true,
  };
}
