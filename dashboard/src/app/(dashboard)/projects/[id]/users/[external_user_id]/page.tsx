"use client";

// Developer → App User detail view (deep link).
//
// Kept as a bookmarkable/shareable route alongside inline expansion on the
// App Users list. It fetches its own ManagedUserDetail so it works
// independently of the list page's in-memory cache, but renders the shared
// ManagedUserAccounts component so the two entry points cannot drift.

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { getManagedUser, type ManagedUserDetail } from "@/lib/api";
import { ArrowLeft, Mail, Calendar } from "lucide-react";
import {
  ManagedUserAccounts,
  type ManagedUserAccountsStatus,
} from "@/components/managed-users/managed-user-accounts";

export default function ManagedUserDetailPage() {
  const { id: profileId, external_user_id: rawExternalUserID } = useParams<{
    id: string;
    external_user_id: string;
  }>();
  const externalUserID = decodeURIComponent(rawExternalUserID);
  const { getToken } = useAuth();
  const [user, setUser] = useState<ManagedUserDetail | null>(null);
  const [status, setStatus] = useState<ManagedUserAccountsStatus>("loading");
  const [error, setError] = useState<string | null>(null);

  // No state is written before the first await: the mount effect below calls
  // this directly, and a synchronous setState there cascades renders.
  const load = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) {
        setError("Your session expired. Reload the page and try again.");
        setStatus("error");
        return;
      }
      const res = await getManagedUser(token, profileId, externalUserID);
      setUser(res.data);
      setError(null);
      setStatus("ready");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load user");
      setStatus("error");
    }
  }, [getToken, profileId, externalUserID]);

  useEffect(() => {
    // `load` writes no state before its first await, so this is an ordinary
    // mount fetch rather than the synchronous render cascade the rule targets.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  // Retry re-runs the request from a visible loading state. A failed load is
  // never left cached as a success.
  function retry() {
    setStatus("loading");
    setError(null);
    load();
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
          {user?.external_user_id || externalUserID}
        </h1>
        <div className="flex flex-wrap gap-4 text-sm text-[var(--dmuted)]">
          {user?.external_user_email && (
            <div className="flex items-center gap-1">
              <Mail className="w-3 h-3" />
              {user.external_user_email}
            </div>
          )}
          {user ? (
            <div className="flex items-center gap-1">
              <Calendar className="w-3 h-3" />
              {user.account_count} {user.account_count === 1 ? "account" : "accounts"}
            </div>
          ) : null}
        </div>
      </div>

      <h2 className="mb-3 text-sm uppercase text-[var(--dmuted)]">Connected accounts</h2>
      <ManagedUserAccounts
        profileId={profileId}
        status={status}
        accounts={user?.accounts ?? []}
        errorMessage={error}
        onRetry={retry}
        onMutated={load}
      />
    </div>
  );
}
