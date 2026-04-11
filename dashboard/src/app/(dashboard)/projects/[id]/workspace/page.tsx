"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

// Legacy URL — the content moved to /settings/workspace.
// Keep this shim so existing bookmarks and external links still land.
export default function LegacyWorkspaceRedirect() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/settings/workspace");
  }, [router]);
  return <div style={{ color: "var(--dmuted)" }}>Redirecting to settings…</div>;
}
