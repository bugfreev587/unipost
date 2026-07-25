export type ConnectErrorPresentation = { title: string; body: string };

const FACEBOOK_PAGE_NOT_AVAILABLE_BODY =
  "We couldn’t find a Facebook Page this account can manage or has allowed UniPost to access. Make sure this Facebook account manages a Page and that UniPost is allowed to access it, or ask a Page admin to grant you access. Then open the original connection link and try again.";
const FACEBOOK_PAGE_PERMISSION_REQUIRED_BODY =
  "Your Facebook account can access a Page, but it doesn’t have permission to publish content. Ask a Page admin to grant you Facebook content-management access, then open the original connection link and try again.";
const FACEBOOK_AUTHORIZATION_FAILED_BODY =
  "Facebook authorization couldn’t be completed. Please try again later or contact the developer who sent you the link.";
const FREE_PLAN_ACCOUNT_UNAVAILABLE_BODY =
  "This social account is already connected to another workspace. Free plan workspaces cannot share the same connected social account.";

export function getConnectErrorPresentation(
  raw?: string | null,
  platform?: string,
): ConnectErrorPresentation {
  const reason = (raw || "").trim();

  if (reason === "facebook_page_not_available") {
    return {
      title: "Facebook Page unavailable",
      body: FACEBOOK_PAGE_NOT_AVAILABLE_BODY,
    };
  }

  if (reason === "facebook_page_permission_required") {
    return {
      title: "Facebook Page permission required",
      body: FACEBOOK_PAGE_PERMISSION_REQUIRED_BODY,
    };
  }

  if (reason === "facebook_authorization_failed" || platform === "facebook") {
    return { title: "Connection failed", body: FACEBOOK_AUTHORIZATION_FAILED_BODY };
  }

  if (
    reason.includes("Free plan workspaces cannot share the same connected social account")
    || reason.includes("ACCOUNT_NOT_AVAILABLE_ON_FREE_PLAN")
  ) {
    return { title: "Connection failed", body: FREE_PLAN_ACCOUNT_UNAVAILABLE_BODY };
  }

  if (reason.includes("ACCOUNT_ALREADY_CONNECTED")) {
    return {
      title: "Connection failed",
      body: "This social account is already connected in this workspace.",
    };
  }

  return {
    title: "Connection failed",
    body: reason || "Failed to connect. Please try again.",
  };
}

export function humanizeConnectError(raw?: string | null): string {
  return getConnectErrorPresentation(raw).body;
}
