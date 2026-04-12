"use client";

// Sprint 4 PR5: Managed Users list view.
//
// Shows one row per end user (external_user_id) onboarded via the
// Sprint 3 Connect flow, with aggregate platform counts and a link
// to the per-user detail page. BYO accounts (no external_user_id)
// are excluded — this view is for multi-tenant Connect users only.

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { listManagedUsers, type ManagedUserListEntry } from "@/lib/api";
import { Users, AlertTriangle, ArrowRight } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { ManagedUsersStats } from "@/components/dashboard/connection-stats";

export default function ManagedUsersPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [users, setUsers] = useState<ManagedUserListEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

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

      {users.length > 0 && (
        <ManagedUsersStats users={users} />
      )}

      {users.length === 0 ? (
        <EmptyState />
      ) : (
        <div className="rounded-lg overflow-hidden border border-[var(--dborder)] bg-[var(--surface)]">
          <table className="w-full">
            <thead className="bg-[var(--surface2)] text-xs uppercase text-[var(--dmuted)]">
              <tr>
                <th className="text-left px-4 py-3">External User</th>
                <th className="text-left px-4 py-3">Email</th>
                <th className="text-left px-4 py-3">Platforms</th>
                <th className="text-left px-4 py-3">Connected</th>
                <th className="text-left px-4 py-3">Status</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr
                  key={u.external_user_id}
                  className="border-t border-[var(--dborder)] transition hover:bg-[var(--surface2)]"
                >
                  <td className="px-4 py-4 text-[var(--dtext)] font-mono text-sm">
                    {u.external_user_id}
                  </td>
                  <td className="px-4 py-4 text-[var(--dmuted)] text-sm">
                    {u.external_user_email || "—"}
                  </td>
                  <td className="px-4 py-4">
                    <div className="flex items-center gap-2">
                      {u.platform_counts.twitter > 0 && (
                        <PlatformBadge platform="twitter" count={u.platform_counts.twitter} />
                      )}
                      {u.platform_counts.linkedin > 0 && (
                        <PlatformBadge platform="linkedin" count={u.platform_counts.linkedin} />
                      )}
                      {u.platform_counts.bluesky > 0 && (
                        <PlatformBadge platform="bluesky" count={u.platform_counts.bluesky} />
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-4 text-[var(--dmuted)] text-sm">
                    {new Date(u.first_connected_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-4">
                    {u.reconnect_count > 0 ? (
                      <span className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[var(--warning)]" style={{ background: "var(--warning-soft)" }}>
                        <AlertTriangle className="w-3 h-3" />
                        {u.reconnect_count} need reconnect
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[var(--success)]" style={{ background: "var(--success-soft)" }}>
                        Active
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-4 text-right">
                    <Link
                      href={`/projects/${profileId}/users/${encodeURIComponent(u.external_user_id)}`}
                      className="inline-flex items-center gap-1 text-sm text-[var(--success)] hover:opacity-80"
                    >
                      Detail <ArrowRight className="w-3 h-3" />
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function PlatformBadge({ platform, count }: { platform: string; count: number }) {
  return (
    <div className="inline-flex items-center gap-1 rounded border border-[var(--dborder)] bg-[var(--surface2)] px-2 py-1 text-xs text-[var(--dmuted)]">
      <PlatformIcon platform={platform} size={12} />
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
