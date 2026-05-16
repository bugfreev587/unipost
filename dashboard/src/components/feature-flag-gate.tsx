"use client";

import { ShieldOff } from "lucide-react";
import type { ReactNode } from "react";
import { useFeatureFlags } from "@/lib/use-feature-flags";

type FeatureFlagGateProps = {
  flag: string;
  children: ReactNode;
  title?: string;
  description?: string;
};

export function FeatureFlagGate({
  flag,
  children,
  title = "Not available",
  description = "This dashboard surface is not enabled in the current environment.",
}: FeatureFlagGateProps) {
  const { flags, loading } = useFeatureFlags();

  if (loading) {
    return <div style={{ color: "var(--dmuted)", padding: 40, textAlign: "center" }}>Loading...</div>;
  }

  if (!flags[flag]) {
    return (
      <div style={{ textAlign: "center", padding: 60, color: "var(--dmuted)" }}>
        <ShieldOff style={{ width: 40, height: 40, margin: "0 auto 12px", opacity: 0.35 }} />
        <div style={{ fontSize: 15, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>{title}</div>
        <div style={{ fontSize: 13 }}>{description}</div>
      </div>
    );
  }

  return <>{children}</>;
}
