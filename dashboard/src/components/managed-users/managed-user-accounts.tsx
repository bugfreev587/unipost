"use client";

// Shared connected-account presentation for one App User (external_user_id).
//
// Both entry points render this component so they cannot drift:
//   - inline expansion on the App Users list
//   - the /projects/{id}/users/{external_user_id} deep-link page
//
// The component owns per-account identity, timestamps, status, and the
// disconnect/dismiss confirmation flow. It deliberately does NOT fetch the
// App Users list or resolve Workspace/profile authorization — those stay with
// the page and the API.

import { useState, type ReactNode } from "react";
import { useAuth } from "@clerk/nextjs";
import { AlertCircle, RefreshCw, Unplug } from "lucide-react";
import {
  disconnectSocialAccount,
  dismissSocialAccount,
  type SocialAccount,
} from "@/lib/api";
import { AccountDestinationIcon } from "@/components/account-destination-icon";
import { YouTubeChannelIdentity } from "@/components/youtube/youtube-channel-identity";
import {
  accountIdentityLabels,
  TIKTOK_IDENTITY_RECONNECT_GUIDANCE,
} from "@/lib/account-identity";
import { connectionTypeLabel, platformDisplayName } from "@/lib/managed-user-platforms";
import { ConfirmModal } from "@/components/confirm-modal";

export type ManagedUserAccountsStatus = "loading" | "error" | "ready";

export interface ManagedUserAccountsProps {
  profileId: string;
  /** Loaded accounts. Ignored unless `status` is `ready`. */
  accounts: SocialAccount[];
  status: ManagedUserAccountsStatus;
  /** User-readable load failure, shown with the Retry action. */
  errorMessage?: string | null;
  /** Re-runs the detail request. Omit to hide Retry. */
  onRetry?: () => void;
  /**
   * Called after a successful disconnect/dismiss so the caller can refresh
   * both this App User's detail data and the list aggregates.
   */
  onMutated: () => void | Promise<void>;
  /** Extra content below the accounts, e.g. the `Open full detail` link. */
  footer?: ReactNode;
}

export function ManagedUserAccounts({
  profileId,
  accounts,
  status,
  errorMessage,
  onRetry,
  onMutated,
  footer,
}: ManagedUserAccountsProps) {
  const { getToken } = useAuth();
  const [disconnectTarget, setDisconnectTarget] = useState<string | null>(null);
  const [dismissTarget, setDismissTarget] = useState<string | null>(null);
  // Action failures used to be console-only. Inline expansion makes that
  // invisible, so failures surface next to the accounts they affect.
  const [actionError, setActionError] = useState<string | null>(null);

  async function runAction(
    action: (token: string) => Promise<unknown>,
    failureMessage: string
  ) {
    setActionError(null);
    try {
      const token = await getToken();
      if (!token) {
        setActionError("Your session expired. Reload the page and try again.");
        return;
      }
      await action(token);
      await onMutated();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : failureMessage);
    }
  }

  async function handleDisconnect() {
    const accountId = disconnectTarget;
    if (!accountId) return;
    setDisconnectTarget(null);
    await runAction(
      (token) => disconnectSocialAccount(token, profileId, accountId),
      "Failed to disconnect the account."
    );
  }

  async function handleDismiss() {
    const accountId = dismissTarget;
    if (!accountId) return;
    setDismissTarget(null);
    await runAction(
      (token) => dismissSocialAccount(token, profileId, accountId),
      "Failed to dismiss the account."
    );
  }

  return (
    <div data-managed-user-accounts data-status={status}>
      {status === "loading" ? (
        <AccountsSkeleton />
      ) : status === "error" ? (
        <ScopedMessage
          tone="danger"
          message={errorMessage || "Failed to load connected accounts."}
          onRetry={onRetry}
        />
      ) : accounts.length === 0 ? (
        <ScopedMessage
          tone="muted"
          message="No managed accounts found for this App User."
          onRetry={onRetry}
        />
      ) : (
        <>
          {actionError ? (
            <div
              data-managed-user-action-error
              role="alert"
              className="mb-3 flex items-start gap-2 rounded-lg px-3 py-2 text-sm text-[var(--danger)]"
              style={{ background: "var(--danger-soft)" }}
            >
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{actionError}</span>
            </div>
          ) : null}

          <div className="space-y-3">
            {accounts.map((acc) => (
              <AccountCard
                key={acc.id}
                account={acc}
                onDisconnect={() => setDisconnectTarget(acc.id)}
                onDismiss={() => setDismissTarget(acc.id)}
              />
            ))}
          </div>
        </>
      )}

      {footer ? <div className="mt-3">{footer}</div> : null}

      <ConfirmModal
        open={disconnectTarget !== null}
        title="Disconnect Account"
        message="Disconnect this account? The end user will need to re-Connect to publish again."
        confirmLabel="Disconnect"
        variant="danger"
        onConfirm={handleDisconnect}
        onCancel={() => setDisconnectTarget(null)}
      />

      <ConfirmModal
        open={dismissTarget !== null}
        title="Dismiss Disconnected Account"
        message="Hide this disconnected account from Developer App Users permanently? Historical data will be kept, but this account will no longer appear in these dashboard views."
        confirmLabel="Dismiss"
        onConfirm={handleDismiss}
        onCancel={() => setDismissTarget(null)}
      />
    </div>
  );
}

function AccountCard({
  account,
  onDisconnect,
  onDismiss,
}: {
  account: SocialAccount;
  onDisconnect: () => void;
  onDismiss: () => void;
}) {
  const identity = accountIdentityLabels(account);
  const isDisconnected = account.status === "disconnected";

  return (
    <div
      data-managed-user-account
      data-platform={account.platform}
      className="rounded-lg border border-[var(--dborder)] bg-[var(--surface)] p-4"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          {account.platform === "youtube" ? (
            <YouTubeChannelIdentity account={account} compact disclosure="Source: YouTube" />
          ) : (
            <div className="flex items-start gap-3">
              <span className="mt-0.5 shrink-0">
                <AccountDestinationIcon platform={account.platform} size={24} />
              </span>
              <div className="min-w-0">
                <div className="break-words font-medium text-[var(--dtext)]">
                  {identity.primary}
                  {identity.handle ? (
                    <span className="ml-1.5 font-normal text-[var(--dmuted)]">
                      {identity.handle}
                    </span>
                  ) : null}
                </div>
                <div className="mt-0.5 text-xs text-[var(--dmuted)]">
                  {platformDisplayName(account.platform)}
                </div>
                {identity.identityRefreshRequired ? (
                  <div
                    data-identity-refresh-guidance
                    className="mt-0.5 text-xs text-[var(--warning)]"
                  >
                    {TIKTOK_IDENTITY_RECONNECT_GUIDANCE}
                  </div>
                ) : null}
              </div>
            </div>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <span data-unipost-account-status>
            {account.status === "active" ? (
              <span
                className="rounded px-2 py-1 text-xs text-[var(--success)]"
                style={{ background: "var(--success-soft)" }}
              >
                Active
              </span>
            ) : isDisconnected ? (
              <span
                className="rounded px-2 py-1 text-xs text-[var(--danger)]"
                style={{ background: "var(--danger-soft)" }}
              >
                Disconnected
              </span>
            ) : (
              <span
                className="rounded px-2 py-1 text-xs text-[var(--warning)]"
                style={{ background: "var(--warning-soft)" }}
              >
                {account.status}
              </span>
            )}
          </span>
          {isDisconnected ? (
            <button
              type="button"
              onClick={onDismiss}
              className="rounded px-2 py-1 text-xs text-[var(--dmuted)] hover:text-[var(--dtext)]"
              title="Dismiss"
            >
              Dismiss
            </button>
          ) : (
            <button
              type="button"
              onClick={onDisconnect}
              className="rounded p-2 text-[var(--dmuted)] hover:text-[var(--danger)]"
              title="Disconnect"
              aria-label="Disconnect account"
            >
              <Unplug className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      {/* connected_at and last_connected_at describe different events. A
          background token refresh is never a user-authorized connection, so a
          missing last_connected_at reads as "Not recorded" rather than falling
          back to last_refreshed_at. */}
      <dl className="mt-3 grid grid-cols-1 gap-x-6 gap-y-2 border-t border-[var(--dborder)] pt-3 text-xs sm:grid-cols-3">
        <MetaField
          label="First connected"
          value={formatDate(account.connected_at) ?? "Not recorded"}
        />
        <MetaField
          label="Last connected"
          value={formatDate(account.last_connected_at) ?? "Not recorded"}
        />
        <MetaField label="Connection" value={connectionTypeLabel(account.connection_type)} />
      </dl>
    </div>
  );
}

function MetaField({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-[var(--dmuted2)]">{label}</dt>
      <dd className="mt-0.5 break-words text-[var(--dmuted)]">{value}</dd>
    </div>
  );
}

function ScopedMessage({
  tone,
  message,
  onRetry,
}: {
  tone: "danger" | "muted";
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div
      data-managed-user-accounts-message
      className="rounded-lg border border-dashed border-[var(--dborder)] p-4 text-sm"
    >
      <p className={tone === "danger" ? "text-[var(--danger)]" : "text-[var(--dmuted)]"}>
        {message}
      </p>
      {onRetry ? (
        <button
          type="button"
          onClick={onRetry}
          className="mt-2 inline-flex items-center gap-1.5 text-sm text-[var(--success)] hover:opacity-80"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          Retry
        </button>
      ) : null}
    </div>
  );
}

function AccountsSkeleton() {
  return (
    <div className="space-y-3" aria-busy="true" aria-live="polite">
      <span className="sr-only">Loading connected accounts…</span>
      {[0, 1].map((i) => (
        <div
          key={i}
          className="animate-pulse rounded-lg border border-[var(--dborder)] bg-[var(--surface)] p-4"
        >
          <div className="flex items-center gap-3">
            <div className="h-6 w-6 rounded-full bg-[var(--surface2)]" />
            <div className="flex-1 space-y-2">
              <div className="h-3 w-40 max-w-full rounded bg-[var(--surface2)]" />
              <div className="h-2.5 w-24 max-w-full rounded bg-[var(--surface2)]" />
            </div>
          </div>
          <div className="mt-3 grid grid-cols-1 gap-2 border-t border-[var(--dborder)] pt-3 sm:grid-cols-3">
            <div className="h-2.5 w-24 rounded bg-[var(--surface2)]" />
            <div className="h-2.5 w-24 rounded bg-[var(--surface2)]" />
            <div className="h-2.5 w-24 rounded bg-[var(--surface2)]" />
          </div>
        </div>
      ))}
    </div>
  );
}

/** Returns null for missing or unparseable timestamps so callers pick the copy. */
function formatDate(value?: string | null): string | null {
  if (!value) return null;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return null;
  return parsed.toLocaleDateString();
}
