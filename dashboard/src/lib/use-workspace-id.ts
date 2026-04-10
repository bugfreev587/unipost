"use client";

import { useState, useEffect } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { getProfile } from "@/lib/api";

/**
 * Resolves the workspace ID from the profile ID in the current URL.
 *
 * The dashboard routes use /projects/[id] where [id] is a profile ID.
 * Workspace-scoped API calls need the workspace_id from the profile.
 * This hook fetches the profile once and returns its workspace_id.
 */
export function useWorkspaceId(): string {
  const { id } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [workspaceId, setWorkspaceId] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getProfile(token, id);
        if (!cancelled) setWorkspaceId(res.data.workspace_id);
      } catch {
        // Profile fetch failed — workspace_id stays empty, API calls will 401/404
      }
    })();
    return () => { cancelled = true; };
  }, [id, getToken]);

  return workspaceId;
}
