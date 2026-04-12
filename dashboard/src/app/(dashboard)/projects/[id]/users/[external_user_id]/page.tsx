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
import { ConfirmModal } from "@/components/confirm-modal";

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
  const [disconnectTarget, setDisconnectTarget] = useState<string | null>(null);

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

  async function handleDisconnect() {
    if (!disconnectTarget) return;
    try {
      const token = await getToken();
      if (!token) return;
      await disconnectSocialAccount(token, profileId, disconnectTarget);
      setDisconnectTarget(null);
      load();
    } catch (err) {
      console.error("Disconnect failed:", err);
    }
  }

  if (loading) return <div className="p-8 text-[var(--dmuted)]">Loading…</div>;
  if (error || !user) {
    return (
      <div className="p-8 text-[var(--danger)]">
        {error || "User not found"}
        <div className="mt-4">
          <Link href={`/projects/${profileId}/users`} className="text-sm text-[var(--success)] hover:opacity-80">
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
        className="mb-6 inline-flex items-center gap-2 text-sm text-[var(--dmuted)] hover:text-[var(--dtext)]"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to users
      </Link>

      <div className="mb-6 rounded-lg border border-[var(--dborder)] bg-[var(--surface)] p-6">
        <h1 className="mb-2 break-all font-mono text-xl text-[var(--dtext)]">
          {user.external_user_id}
        </h1>
        <div className="flex flex-wrap gap-4 text-sm text-[var(--dmuted)]">
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

      <h2 className="mb-3 text-sm uppercase text-[var(--dmuted)]">Connected accounts</h2>
      <div className="space-y-3">
        {user.accounts.map((acc) => (
          <div
            key={acc.id}
            className="flex items-center gap-4 rounded-lg border border-[var(--dborder)] bg-[var(--surface)] p-4"
          >
            <PlatformIcon platform={acc.platform} size={24} />
            <div className="flex-1 min-w-0">
              <div className="truncate font-medium text-[var(--dtext)]">
                {acc.account_name || acc.id}
              </div>
              <div className="mt-0.5 text-xs text-[var(--dmuted)]">
                {acc.platform} · {acc.connection_type} ·{" "}
                {new Date(acc.connected_at).toLocaleDateString()}
              </div>
            </div>
            <div>
              {acc.status === "active" ? (
                <span className="rounded px-2 py-1 text-xs text-[var(--success)]" style={{ background: "var(--success-soft)" }}>
                  Active
                </span>
              ) : (
                <span className="rounded px-2 py-1 text-xs text-[var(--warning)]" style={{ background: "var(--warning-soft)" }}>
                  {acc.status}
                </span>
              )}
            </div>
            <button
              onClick={() => setDisconnectTarget(acc.id)}
              className="p-2 text-[var(--dmuted)] hover:text-[var(--danger)]"
              title="Disconnect"
            >
              <Unplug className="w-4 h-4" />
            </button>
          </div>
        ))}
      </div>

      <ConfirmModal
        open={disconnectTarget !== null}
        title="Disconnect Account"
        message="Disconnect this account? The end user will need to re-Connect to publish again."
        confirmLabel="Disconnect"
        variant="danger"
        onConfirm={handleDisconnect}
        onCancel={() => setDisconnectTarget(null)}
      />
    </div>
  );
}
