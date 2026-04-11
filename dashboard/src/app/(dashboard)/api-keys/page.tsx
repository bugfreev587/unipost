"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { getBootstrap } from "@/lib/api";

// Top-level /api-keys resolver. The dashboard's API keys page is
// profile-scoped (/projects/{profileId}/api-keys), so the bare path
// /api-keys has no page of its own — which meant the URL printed by
// older AgentPost installs and any hand-typed "just show me my keys"
// navigation 404'd. This shim hits bootstrap, picks the user's last
// or default profile, and hops them over.
export default function ApiKeysRedirectPage() {
  const router = useRouter();
  const { getToken, isLoaded, isSignedIn } = useAuth();

  useEffect(() => {
    if (!isLoaded || !isSignedIn) return;

    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getBootstrap(token);
        if (cancelled) return;

        if (!res.data.onboarding_completed) {
          router.replace("/welcome");
          return;
        }

        const target = res.data.last_profile_id ?? res.data.default_profile_id;
        if (target) {
          router.replace(`/projects/${target}/api-keys`);
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
    <div style={{ color: "var(--dmuted)", fontSize: 13 }}>Loading API keys…</div>
  );
}
