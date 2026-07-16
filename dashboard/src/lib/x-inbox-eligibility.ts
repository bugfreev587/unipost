export type XInboxAppMode =
  | "unipost_managed_app"
  | "workspace_x_app"
  | "legacy_unknown";

export type XInboxCapabilities = {
  comments_enabled: boolean;
  dms_enabled: boolean;
  missing_scopes: string[];
  reconnect_required: boolean;
  delivery_status:
    | "pending"
    | "active"
    | "paused_cap"
    | "paused_allowance"
    | "paused_plan"
    | "error"
    | string;
  app_mode: XInboxAppMode;
  missing_app_credentials: string[];
};

type XAccountEligibilityInput = {
  status: string;
  scope?: string[];
};

export type XInboxEligibility = {
  publishingEnabled: boolean;
  commentsEnabled: boolean;
  dmsEnabled: boolean;
  reconnectRequired: boolean;
  missingScopes: string[];
  missingAppCredentials: string[];
  deliveryStatus: string;
  appMode: XInboxAppMode;
  summary: string;
};

export function evaluateXInboxEligibility(
  account: XAccountEligibilityInput,
  capabilities: XInboxCapabilities,
): XInboxEligibility {
  const scope = new Set((account.scope || []).map((value) => value.trim().toLowerCase()));
  const publishingEnabled =
    account.status === "active" &&
    ["tweet.read", "tweet.write", "users.read"].every((required) => scope.has(required));

  let summary = "X Inbox delivery is being prepared.";
  if (capabilities.delivery_status === "paused_plan") {
    summary = "X Inbox is not included in the current plan.";
  } else if (capabilities.delivery_status === "paused_cap") {
    summary = "X Inbox is paused at the workspace inbound daily cap.";
  } else if (capabilities.delivery_status === "paused_allowance") {
    summary = "Managed X Inbox is paused because the monthly X Credits allowance is exhausted.";
  } else if (capabilities.missing_app_credentials.length > 0) {
    summary = "Complete the workspace X app credentials to enable Inbox delivery.";
  } else if (capabilities.missing_scopes.some((scopeName) => scopeName.startsWith("dm."))) {
    summary = "Reconnect X to grant DM permissions. Existing X publishing remains available.";
  } else if (capabilities.reconnect_required) {
    summary = "Reconnect X to grant the latest Inbox permissions.";
  } else if (capabilities.delivery_status === "active") {
    summary = "X comments and DMs are active.";
  }

  return {
    publishingEnabled,
    commentsEnabled: capabilities.comments_enabled,
    dmsEnabled: capabilities.dms_enabled,
    reconnectRequired: capabilities.reconnect_required,
    missingScopes: capabilities.missing_scopes,
    missingAppCredentials: capabilities.missing_app_credentials,
    deliveryStatus: capabilities.delivery_status,
    appMode: capabilities.app_mode,
    summary,
  };
}
