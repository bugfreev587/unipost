// Shared account identity label rule (TikTok identity accuracy PRD,
// 2026-07-31 §9.6).
//
// Every surface that presents a publishing destination renders:
//   primary label : display_name → username → legacy account_name → id
//   handle        : @username, only when a verified username exists
//
// Legacy TikTok rows are detected exclusively through the server-computed
// identity_refresh_required flag (backed by metadata's identity schema
// version). Never infer legacy state from username/display-name equality:
// valid accounts can intentionally use the same value for both fields.

import type { SocialAccount } from "./api";

export type AccountIdentitySource = Pick<
  SocialAccount,
  "id" | "account_name" | "username" | "display_name" | "identity_refresh_required"
>;

export interface AccountIdentityLabels {
  /** Friendly label: display name first, then handle, then legacy name. */
  primary: string;
  /** Secondary handle line (`@username`) when a verified username exists. */
  handle: string | null;
  /** True only for legacy TikTok rows that need one post-fix reconnect. */
  identityRefreshRequired: boolean;
}

/** Inline guidance for legacy TikTok rows; reuses the existing reconnect flow. */
export const TIKTOK_IDENTITY_RECONNECT_GUIDANCE =
  "Reconnect TikTok to refresh the account username";

export function accountIdentityLabels(account: AccountIdentitySource): AccountIdentityLabels {
  const displayName = account.display_name?.trim() || "";
  const username = account.username?.trim() || "";
  const legacyName = account.account_name?.trim() || "";
  return {
    primary: displayName || username || legacyName || account.id,
    handle: username ? `@${username}` : null,
    identityRefreshRequired: account.identity_refresh_required === true,
  };
}
