"use client";

// Developer → App Users list view.
//
// One summary row per end user (external_user_id) onboarded through the
// Connect flow, with complete platform badges and inline expansion of that
// user's connected accounts. BYO accounts (no external_user_id) are excluded —
// this view is for multi-tenant Connect users only.
//
// Expansion is a disclosure state, not a selection: the expanded row keeps the
// normal surface background and communicates state through the chevron, a
// divider, and content visibility.

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import {
  dismissManagedUserDisconnected,
  getManagedUser,
  listManagedUsers,
  type ManagedUserDetail,
  type ManagedUserListEntry,
} from "@/lib/api";
import { Users, AlertTriangle, ArrowRight, ChevronRight } from "lucide-react";
import { AccountDestinationIcon } from "@/components/account-destination-icon";
import { ManagedUsersStats } from "@/components/dashboard/connection-stats";
import { ConfirmModal } from "@/components/confirm-modal";
import {
  ManagedUserAccounts,
  type ManagedUserAccountsStatus,
} from "@/components/managed-users/managed-user-accounts";
import { MANAGED_USER_PLATFORMS, platformDisplayName } from "@/lib/managed-user-platforms";

// Per-App-User detail, cached by external_user_id. A failed load is never
// cached as a success, so Retry always issues a fresh request.
interface DetailState {
  status: ManagedUserAccountsStatus;
  detail?: ManagedUserDetail;
  error?: string;
}

// Grid template for the desktop summary row. Below `md` the cells stack.
const ROW_GRID =
  "md:grid-cols-[minmax(0,1.4fr)_minmax(0,1.2fr)_minmax(0,1fr)_auto_auto_auto]";

export default function ManagedUsersPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [users, setUsers] = useState<ManagedUserListEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [dismissTarget, setDismissTarget] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [details, setDetails] = useState<Record<string, DetailState>>({});

  const load = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listManagedUsers(token, profileId);
      setUsers(res.data);
      setTotal(res.meta?.total ?? res.data.length);
    } catch (err) {
      console.error("Failed to load managed users:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken, profileId]);

  useEffect(() => {
    load();
  }, [load]);

  // Detail is fetched only on expansion, so the list request stays the only
  // request on initial page load and rows never fan out into an N+1 burst.
  const loadDetail = useCallback(
    async (externalUserId: string) => {
      setDetails((prev) => ({ ...prev, [externalUserId]: { status: "loading" } }));
      try {
        const token = await getToken();
        if (!token) {
          setDetails((prev) => ({
            ...prev,
            [externalUserId]: {
              status: "error",
              error: "Your session expired. Reload the page and try again.",
            },
          }));
          return;
        }
        const res = await getManagedUser(token, profileId, externalUserId);
        setDetails((prev) => ({
          ...prev,
          [externalUserId]: { status: "ready", detail: res.data },
        }));
        if (res.data.accounts.length === 0) {
          // The list row exists but the detail view has nothing to show. Record
          // it because the two views disagree; the id is a customer-chosen key,
          // never a token or provider payload.
          console.warn("App User detail returned no accounts", { externalUserId });
        }
      } catch (err) {
        setDetails((prev) => ({
          ...prev,
          [externalUserId]: {
            status: "error",
            error:
              err instanceof Error ? err.message : "Failed to load connected accounts.",
          },
        }));
      }
    },
    [getToken, profileId]
  );

  function toggleRow(externalUserId: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(externalUserId)) {
        next.delete(externalUserId);
      } else {
        next.add(externalUserId);
      }
      return next;
    });
    // Cache hit: reopening a loaded row must not issue a second request.
    const cached = details[externalUserId];
    if (!expanded.has(externalUserId) && cached?.status !== "ready") {
      loadDetail(externalUserId);
    }
  }

  // An account mutation changes both the row's accounts and the list
  // aggregates, so refresh the affected cache entry and the summary together.
  const refreshAfterMutation = useCallback(
    async (externalUserId: string) => {
      await Promise.all([loadDetail(externalUserId), load()]);
    },
    [loadDetail, load]
  );

  async function handleDismiss() {
    if (!dismissTarget) return;
    try {
      const token = await getToken();
      if (!token) return;
      await dismissManagedUserDisconnected(token, profileId, dismissTarget);
      const externalUserId = dismissTarget;
      setDismissTarget(null);
      await load();
      if (expanded.has(externalUserId)) {
        await loadDetail(externalUserId);
      } else {
        setDetails((prev) => {
          const next = { ...prev };
          delete next[externalUserId];
          return next;
        });
      }
    } catch (err) {
      console.error("Failed to dismiss managed user accounts:", err);
    }
  }

  if (loading) {
    return <div className="p-8 text-[var(--dmuted)]">Loading…</div>;
  }

  return (
    <div className="p-8 max-w-6xl">
      <div className="flex items-center gap-3 mb-6">
        <div className="p-2 rounded-lg" style={{ background: "var(--success-soft)" }}>
          <Users className="w-5 h-5 text-[var(--success)]" />
        </div>
        <div>
          <h1 className="text-2xl font-semibold text-[var(--dtext)]">Managed Users</h1>
          <p className="text-sm text-[var(--dmuted)]">
            End users onboarded via Connect — {total} total
          </p>
        </div>
      </div>

      {users.length > 0 && <ManagedUsersStats users={users} />}

      {users.length === 0 ? (
        <EmptyState />
      ) : (
        <div className="rounded-lg overflow-hidden border border-[var(--dborder)] bg-[var(--surface)]">
          <div
            className={`hidden md:grid ${ROW_GRID} gap-4 bg-[var(--surface2)] px-4 py-3 text-xs uppercase text-[var(--dmuted)]`}
          >
            <div>External User</div>
            <div>Email</div>
            <div>Platforms</div>
            <div>Connected</div>
            <div>Status</div>
            <div />
          </div>

          {users.map((u) => {
            const isExpanded = expanded.has(u.external_user_id);
            const panelId = `app-user-panel-${encodeURIComponent(u.external_user_id)}`;
            const detail = details[u.external_user_id];
            return (
              <div
                key={u.external_user_id}
                data-app-user-row={u.external_user_id}
                data-expanded={isExpanded}
                className="border-t border-[var(--dborder)]"
              >
                <div
                  onClick={() => toggleRow(u.external_user_id)}
                  className={`grid grid-cols-1 ${ROW_GRID} cursor-pointer items-center gap-2 px-4 py-4 transition hover:bg-[var(--surface2)] md:gap-4`}
                >
                  <div className="min-w-0 break-all font-mono text-sm text-[var(--dtext)]">
                    {u.external_user_id}
                  </div>
                  <div className="min-w-0 break-all text-sm text-[var(--dmuted)]">
                    {u.external_user_email || "—"}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {MANAGED_USER_PLATFORMS.filter(
                      (platform) => (u.platform_counts?.[platform] ?? 0) > 0
                    ).map((platform) => (
                      <PlatformBadge
                        key={platform}
                        platform={platform}
                        count={u.platform_counts[platform]}
                      />
                    ))}
                  </div>
                  <div className="text-sm text-[var(--dmuted)]">
                    {new Date(u.first_connected_at).toLocaleDateString()}
                  </div>
                  <div>
                    {u.disconnected_count > 0 ? (
                      <span
                        className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[var(--danger)]"
                        style={{ background: "var(--danger-soft)" }}
                      >
                        <AlertTriangle className="w-3 h-3" />
                        {u.disconnected_count} disconnected
                      </span>
                    ) : u.reconnect_count > 0 ? (
                      <span
                        className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[var(--warning)]"
                        style={{ background: "var(--warning-soft)" }}
                      >
                        <AlertTriangle className="w-3 h-3" />
                        {u.reconnect_count} need reconnect
                      </span>
                    ) : (
                      <span
                        className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[var(--success)]"
                        style={{ background: "var(--success-soft)" }}
                      >
                        Active
                      </span>
                    )}
                  </div>
                  <div className="flex items-center justify-end gap-3">
                    {u.disconnected_count > 0 ? (
                      <button
                        type="button"
                        // Nested actions must not toggle the row.
                        onClick={(e) => {
                          e.stopPropagation();
                          setDismissTarget(u.external_user_id);
                        }}
                        className="text-sm text-[var(--dmuted)] hover:text-[var(--dtext)]"
                      >
                        Dismiss
                      </button>
                    ) : null}
                    <button
                      type="button"
                      aria-expanded={isExpanded}
                      aria-controls={panelId}
                      aria-label={`${isExpanded ? "Collapse" : "Expand"} connected accounts for ${u.external_user_id}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        toggleRow(u.external_user_id);
                      }}
                      className="rounded p-1 text-[var(--dmuted)] transition hover:text-[var(--dtext)]"
                    >
                      <ChevronRight
                        className={`h-4 w-4 transition-transform ${isExpanded ? "rotate-90" : ""}`}
                      />
                    </button>
                  </div>
                </div>

                {isExpanded ? (
                  <div
                    id={panelId}
                    // Neutral disclosure surface: no accent or selected-state fill.
                    className="border-t border-dashed border-[var(--dborder)] px-4 py-4"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <ManagedUserAccounts
                      profileId={profileId}
                      status={detail?.status ?? "loading"}
                      accounts={detail?.detail?.accounts ?? []}
                      errorMessage={detail?.error}
                      onRetry={() => loadDetail(u.external_user_id)}
                      onMutated={() => refreshAfterMutation(u.external_user_id)}
                      footer={
                        <Link
                          href={`/projects/${profileId}/users/${encodeURIComponent(u.external_user_id)}`}
                          className="inline-flex items-center gap-1 text-sm text-[var(--success)] hover:opacity-80"
                        >
                          Open full detail <ArrowRight className="w-3 h-3" />
                        </Link>
                      }
                    />
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      )}

      <ConfirmModal
        open={dismissTarget !== null}
        title="Dismiss Disconnected Accounts"
        message="Hide the disconnected accounts for this managed user from Developer App Users? Historical data will be kept, but those disconnected accounts will no longer appear in these dashboard views."
        confirmLabel="Dismiss"
        onConfirm={handleDismiss}
        onCancel={() => setDismissTarget(null)}
      />
    </div>
  );
}

function PlatformBadge({ platform, count }: { platform: string; count: number }) {
  return (
    <div
      data-platform-badge={platform}
      title={platformDisplayName(platform)}
      className="inline-flex items-center gap-1 rounded border border-[var(--dborder)] bg-[var(--surface2)] px-2 py-1 text-xs text-[var(--dmuted)]"
    >
      <AccountDestinationIcon platform={platform} size={12} />
      <span className="sr-only">{platformDisplayName(platform)}</span>
      {count}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-lg border border-dashed border-[var(--dborder)] p-12 text-center">
      <Users className="mx-auto mb-4 h-10 w-10 text-[var(--dmuted2)]" />
      <h3 className="mb-2 text-lg font-medium text-[var(--dtext)]">
        No managed users yet
      </h3>
      <p className="mx-auto mb-4 max-w-md text-sm text-[var(--dmuted)]">
        End users will appear here after they complete a Connect flow.
        Use{" "}
        <code className="rounded bg-[var(--surface2)] px-1.5 py-0.5 text-[var(--dmuted)]">
          POST /v1/connect/sessions
        </code>{" "}
        to generate a hosted link, then email it to your user.
      </p>
      <Link
        href="https://docs.unipost.dev#connect"
        target="_blank"
        className="text-sm text-[var(--success)] hover:opacity-80"
      >
        See Connect docs →
      </Link>
    </div>
  );
}
