"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { getPlanGates } from "@/lib/api";

export function usePlanGates() {
  const { getToken } = useAuth();
  const [planGates, setPlanGates] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        setLoading(true);
        const token = await getToken();
        if (!token) {
          if (!cancelled) setPlanGates({});
          return;
        }
        const res = await getPlanGates(token);
        if (!cancelled) setPlanGates(res.data.plan_gates || {});
      } catch {
        if (!cancelled) setPlanGates({});
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [getToken]);

  return { planGates, loading };
}
