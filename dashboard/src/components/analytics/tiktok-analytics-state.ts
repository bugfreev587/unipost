type TikTokApiError = Error & {
  rawCode?: string;
  code?: string;
  details?: { reason?: string };
};

export type TikTokAnalyticsIssue = {
  code: string;
  reason: string;
  message: string;
};

const reasonMessages: Record<string, string> = {
  account_disconnected: "This TikTok account is disconnected. Reconnect it to continue.",
  account_token_invalid: "Your TikTok connection has expired. Reconnect the account.",
  analytics_scope_required: "Reconnect TikTok to grant the permissions required for analytics.",
  provider_rate_limited: "TikTok is temporarily rate limiting analytics requests. Try again later.",
  provider_temporary_error: "TikTok analytics are temporarily unavailable. Try again later.",
  video_not_found: "TikTok analytics are not available for this video yet.",
  video_not_ready: "TikTok analytics are not available for this video yet.",
};

export function tiktokAnalyticsIssue(error: unknown): TikTokAnalyticsIssue {
  const apiError = error instanceof Error ? error as TikTokApiError : null;
  const code = apiError?.rawCode || apiError?.code || "TIKTOK_ERROR";
  let reason = apiError?.details?.reason || "";
  if (!reason && code === "ACCOUNT_DISCONNECTED") reason = "account_disconnected";
  if (!reason && code === "NEEDS_RECONNECT") reason = "account_token_invalid";
  if (!reason && code === "UPSTREAM_RATE_LIMITED") reason = "provider_rate_limited";
  if (!reason && code === "TIKTOK_TEMPORARY_ERROR") reason = "provider_temporary_error";

  return {
    code,
    reason,
    message: reasonMessages[reason] || "TikTok analytics could not be loaded. Try again later.",
  };
}

export type ScopeReadinessState = {
  title: string;
  description: string;
  badge: "Ready" | "Verify" | "Reconnect";
  tone: "ready" | "verify" | "reconnect";
};

export function scopeReadinessState(
  missingScopes: readonly string[],
  runtimeReason?: string,
): ScopeReadinessState {
  if (runtimeReason === "analytics_scope_required") {
    return {
      title: "Reconnect required for analytics",
      description: "TikTok did not authorize all permissions required for analytics.",
      badge: "Reconnect",
      tone: "reconnect",
    };
  }
  if (missingScopes.length > 0) {
    return {
      title: "Analytics permissions need verification",
      description: `Stored scope information is missing: ${missingScopes.join(", ")}. Live TikTok responses determine access.`,
      badge: "Verify",
      tone: "verify",
    };
  }
  return {
    title: "Analytics scopes ready",
    description: "TikTok analytics permissions are recorded and will be verified on each request.",
    badge: "Ready",
    tone: "ready",
  };
}
