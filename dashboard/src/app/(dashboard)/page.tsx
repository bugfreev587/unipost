"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { getBootstrap } from "@/lib/api";

// Dashboard root resolver. Routes the user to the project they were
// last working on (or their auto-created Default if it's their first
// login). Falls back to /projects when bootstrap returns nothing —
// the only realistic cause is "Clerk webhook hasn't synced this user
// yet", which is self-healing on the next page load.
//
// We render this as a client-side redirect rather than a Server
// Component because the dashboard talks to the Go API with a Clerk
// session token sourced from `useAuth().getToken()` — replicating
// that on the server would mean threading Clerk's server-side helpers
// for one redirect, which isn't worth it.
export default function DashboardRootPage() {
  const router = useRouter();
  const { getToken, isLoaded, isSignedIn } = useAuth();

  useEffect(() => {
    if (!isLoaded) return;
    if (!isSignedIn) return;

    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getBootstrap(token);
        if (cancelled) return;
        const target = res.data.last_project_id ?? res.data.default_project_id;
        if (target) {
          router.replace(`/projects/${target}`);
        } else {
          router.replace("/projects");
        }
      } catch {
        if (!cancelled) router.replace("/projects");
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isLoaded, isSignedIn, getToken, router]);

  return (
    <div style={{ color: "var(--dmuted)", fontSize: 13 }}>Loading...</div>
  );
}
