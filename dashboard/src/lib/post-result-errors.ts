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
>;

export type PostResultFailureDescription = {
  title: string;
  message: string;
  nextActionLabel?: string;
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

export function describePostResultFailure(result: PostResultFailureInput): PostResultFailureDescription {
  if (!result.error_code && !result.next_action && typeof result.is_retriable !== "boolean") {
    return legacyDescription(result);
  }

  const messageParts = [result.error_message || "Publish failed."];
  if (result.platform_error_code && result.error_message !== result.platform_error_code) {
    messageParts.push(`Provider error: ${result.platform_error_code}.`);
  }

  return {
    title: titleForErrorCode(result.error_code, result.platform),
    message: messageParts.join(" "),
    nextActionLabel: result.next_action ? NEXT_ACTION_LABELS[result.next_action] || result.next_action : undefined,
    actionHref: actionHrefFor(result.next_action),
    canRetry: result.is_retriable === true,
  };
}
