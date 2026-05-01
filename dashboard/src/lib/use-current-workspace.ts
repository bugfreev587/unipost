"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { getMe, type Workspace } from "@/lib/api";

// Resolves the current workspace from /v1/me. The standalone
// /v1/workspaces list endpoint was retired in the Apr 2026
// workspace_id-removal refactor — workspace context is now derived
// from auth on the server side, and /v1/me carries the id+name
// shortcut for surfaces that still want to display them.
//
// Returns null when the user has no workspace yet (pre-bootstrap)
// or when the call fails. Callers should NOT gate UI on a non-null
// workspace — backend endpoints derive workspace from auth, so the
// dashboard pages work without it.
export function useCurrentWorkspace(): { workspace: Workspace | null; loading: boolean } {
  const { getToken } = useAuth();
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getMe(token);
        if (cancelled) return;
        if (res.data.workspace_id) {
          setWorkspace({
            id: res.data.workspace_id,
            name: res.data.workspace_name ?? "",
            // /v1/me does not surface these fields; consumers that
            // need them should query the relevant scoped endpoint.
            per_account_monthly_limit: null,
            usage_modes: [],
            created_at: "",
            updated_at: "",
          });
        } else {
          setWorkspace(null);
        }
      } catch {
        if (!cancelled) setWorkspace(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [getToken]);

  return { workspace, loading };
}
