export function humanizeConnectError(raw?: string | null): string {
  const msg = (raw || "").trim();
  if (!msg) return "Failed to connect. Please try again.";

  if (
    msg.includes("Free plan workspaces cannot share the same connected social account")
    || msg.includes("ACCOUNT_NOT_AVAILABLE_ON_FREE_PLAN")
  ) {
    return "This social account is already connected to another workspace. Free plan workspaces cannot share the same connected social account.";
  }

  if (msg.includes("ACCOUNT_ALREADY_CONNECTED")) {
    return "This social account is already connected in this workspace.";
  }

  return msg;
}
