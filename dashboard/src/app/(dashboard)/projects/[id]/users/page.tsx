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

export default function ManagedUsersPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [users, setUsers] = useState<ManagedUserListEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listManagedUsers(token, projectId);
      setUsers(res.data);
      setTotal(res.meta?.total ?? res.data.length);
    } catch (err) {
      console.error("Failed to load managed users:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken, projectId]);

  useEffect(() => {
    load();
  }, [load]);

  if (loading) {
    return <div className="p-8 text-[#888]">Loading…</div>;
  }

  return (
    <div className="p-8 max-w-6xl">
      <div className="flex items-center gap-3 mb-6">
        <div className="p-2 bg-[#10b981]/10 rounded-lg">
          <Users className="w-5 h-5 text-[#10b981]" />
        </div>
        <div>
          <h1 className="text-2xl font-semibold text-[#f0f0f0]">Managed Users</h1>
          <p className="text-sm text-[#888]">
            End users onboarded via Connect — {total} total
          </p>
        </div>
      </div>

      {users.length === 0 ? (
        <EmptyState />
      ) : (
        <div className="border border-[#242424] rounded-lg overflow-hidden">
          <table className="w-full">
            <thead className="bg-[#141414] text-xs uppercase text-[#888]">
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
                  className="border-t border-[#242424] hover:bg-[#141414] transition"
                >
                  <td className="px-4 py-4 text-[#f0f0f0] font-mono text-sm">
                    {u.external_user_id}
                  </td>
                  <td className="px-4 py-4 text-[#bbb] text-sm">
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
                  <td className="px-4 py-4 text-[#bbb] text-sm">
                    {new Date(u.first_connected_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-4">
                    {u.reconnect_count > 0 ? (
                      <span className="inline-flex items-center gap-1 text-xs text-[#f59e0b] bg-[#f59e0b]/10 px-2 py-1 rounded">
                        <AlertTriangle className="w-3 h-3" />
                        {u.reconnect_count} need reconnect
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-xs text-[#10b981] bg-[#10b981]/10 px-2 py-1 rounded">
                        Active
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-4 text-right">
                    <Link
                      href={`/projects/${projectId}/users/${encodeURIComponent(u.external_user_id)}`}
                      className="inline-flex items-center gap-1 text-sm text-[#10b981] hover:text-[#0d9668]"
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
    <div className="inline-flex items-center gap-1 text-xs text-[#bbb] bg-[#1a1a1a] border border-[#242424] rounded px-2 py-1">
      <PlatformIcon platform={platform} size={12} />
      {count}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="border border-dashed border-[#242424] rounded-lg p-12 text-center">
      <Users className="w-10 h-10 mx-auto text-[#444] mb-4" />
      <h3 className="text-lg font-medium text-[#f0f0f0] mb-2">
        No managed users yet
      </h3>
      <p className="text-sm text-[#888] max-w-md mx-auto mb-4">
        End users will appear here after they complete a Connect flow.
        Use{" "}
        <code className="bg-[#1a1a1a] px-1.5 py-0.5 rounded text-[#bbb]">
          POST /v1/connect/sessions
        </code>{" "}
        to generate a hosted link, then email it to your user.
      </p>
      <Link
        href="https://docs.unipost.dev#connect"
        target="_blank"
        className="text-sm text-[#10b981] hover:text-[#0d9668]"
      >
        See Connect docs →
      </Link>
    </div>
  );
}
