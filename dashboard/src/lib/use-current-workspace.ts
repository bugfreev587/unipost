"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { listWorkspaces, type Workspace } from "@/lib/api";

// Resolves the current workspace by calling listWorkspaces and
// taking the first one. Useful on global/top-level pages where
// there's no profile_id in the URL to thread through.
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
        const res = await listWorkspaces(token);
        if (cancelled) return;
        setWorkspace(res.data[0] ?? null);
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
