"use client";

// Sprint 4 PR5: Managed User detail view.
//
// All accounts for one external_user_id, with per-account status,
// platform, connection date, and a disconnect button per account.

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import {
  getManagedUser,
  disconnectSocialAccount,
  type ManagedUserDetail,
} from "@/lib/api";
import { ArrowLeft, Unplug, Mail, Calendar } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";

export default function ManagedUserDetailPage() {
  const { id: profileId, external_user_id: rawExternalUserID } = useParams<{
    id: string;
    external_user_id: string;
  }>();
  const externalUserID = decodeURIComponent(rawExternalUserID);
  const { getToken } = useAuth();
  const [user, setUser] = useState<ManagedUserDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await getManagedUser(token, profileId, externalUserID);
      setUser(res.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load user");
    } finally {
      setLoading(false);
    }
  }, [getToken, profileId, externalUserID]);

  useEffect(() => {
    load();
  }, [load]);

  async function handleDisconnect(accountId: string) {
    if (!confirm("Disconnect this account? The end user will need to re-Connect to publish again.")) {
      return;
    }
    try {
      const token = await getToken();
      if (!token) return;
      await disconnectSocialAccount(token, profileId, accountId);
      load();
    } catch (err) {
      console.error("Disconnect failed:", err);
    }
  }

  if (loading) return <div className="p-8 text-[#888]">Loading…</div>;
  if (error || !user) {
    return (
      <div className="p-8 text-[#ef4444]">
        {error || "User not found"}
        <div className="mt-4">
          <Link href={`/projects/${profileId}/users`} className="text-[#10b981] text-sm">
            ← Back to users
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 max-w-4xl">
      <Link
        href={`/projects/${profileId}/users`}
        className="inline-flex items-center gap-2 text-sm text-[#888] hover:text-[#f0f0f0] mb-6"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to users
      </Link>

      <div className="border border-[#242424] rounded-lg p-6 mb-6">
        <h1 className="text-xl font-mono text-[#f0f0f0] mb-2 break-all">
          {user.external_user_id}
        </h1>
        <div className="flex flex-wrap gap-4 text-sm text-[#888]">
          {user.external_user_email && (
            <div className="flex items-center gap-1">
              <Mail className="w-3 h-3" />
              {user.external_user_email}
            </div>
          )}
          <div className="flex items-center gap-1">
            <Calendar className="w-3 h-3" />
            {user.account_count} {user.account_count === 1 ? "account" : "accounts"}
          </div>
        </div>
      </div>

      <h2 className="text-sm uppercase text-[#888] mb-3">Connected accounts</h2>
      <div className="space-y-3">
        {user.accounts.map((acc) => (
          <div
            key={acc.id}
            className="border border-[#242424] rounded-lg p-4 flex items-center gap-4"
          >
            <PlatformIcon platform={acc.platform} size={24} />
            <div className="flex-1 min-w-0">
              <div className="text-[#f0f0f0] font-medium truncate">
                {acc.account_name || acc.id}
              </div>
              <div className="text-xs text-[#888] mt-0.5">
                {acc.platform} · {acc.connection_type} ·{" "}
                {new Date(acc.connected_at).toLocaleDateString()}
              </div>
            </div>
            <div>
              {acc.status === "active" ? (
                <span className="text-xs text-[#10b981] bg-[#10b981]/10 px-2 py-1 rounded">
                  Active
                </span>
              ) : (
                <span className="text-xs text-[#f59e0b] bg-[#f59e0b]/10 px-2 py-1 rounded">
                  {acc.status}
                </span>
              )}
            </div>
            <button
              onClick={() => handleDisconnect(acc.id)}
              className="text-[#888] hover:text-[#ef4444] p-2"
              title="Disconnect"
            >
              <Unplug className="w-4 h-4" />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
