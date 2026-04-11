"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

// Legacy URL — the content moved to /settings/billing. Preserve any
// query params (?status=success, ?upgrade=planId) so Stripe return
// URLs still work.
export default function LegacyBillingRedirect() {
  const router = useRouter();
  const searchParams = useSearchParams();
  useEffect(() => {
    const qs = searchParams.toString();
    router.replace(qs ? `/settings/billing?${qs}` : "/settings/billing");
  }, [router, searchParams]);
  return <div style={{ color: "var(--dmuted)" }}>Redirecting to settings…</div>;
}
