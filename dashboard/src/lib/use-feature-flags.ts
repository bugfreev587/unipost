"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { getFeatureFlags } from "@/lib/api";

export function useFeatureFlags() {
  const { getToken } = useAuth();
  const [flags, setFlags] = useState<Record<string, boolean>>({});
  const [planGates, setPlanGates] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        setLoading(true);
        const token = await getToken();
        if (!token) {
          if (!cancelled) {
            setFlags({});
            setPlanGates({});
          }
          return;
        }
        const res = await getFeatureFlags(token);
        if (!cancelled) {
          setFlags(res.data.flags || {});
          setPlanGates(res.data.plan_gates || {});
        }
      } catch {
        if (!cancelled) {
          setFlags({});
          setPlanGates({});
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [getToken]);

  return { flags, planGates, loading };
}
