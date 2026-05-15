"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "@clerk/nextjs";

import { getFeatureFlags, type FeatureFlagsResponse } from "@/lib/api";
import { AdminShell, StatCard } from "../_components/admin-ui";

type FlagMeta = {
  label: string;
  area: string;
  risk: "Low" | "Medium" | "High";
  productionDefault: boolean;
};

const FLAG_META: Record<string, FlagMeta> = {
  "tiktok.analytics_scopes": {
    label: "TikTok analytics scopes",
    area: "OAuth",
    risk: "High",
    productionDefault: false,
  },
};

function flagLabel(flag: string) {
  return FLAG_META[flag]?.label ?? flag;
}

function riskBadgeClass(risk: FlagMeta["risk"]) {
  if (risk === "High") return "ad-badge ad-b-red";
  if (risk === "Medium") return "ad-badge ad-b-amber";
  return "ad-badge ad-b-gray";
}

export default function AdminFeaturesPage() {
  const { getToken } = useAuth();
  const [features, setFeatures] = useState<FeatureFlagsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadFeatures = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await getFeatureFlags(token);
      setFeatures(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load feature flags");
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => {
    loadFeatures();
  }, [loadFeatures]);

  const rows = useMemo(() => {
    const flags = features?.flags ?? {};
    return Object.entries(flags)
      .map(([key, enabled]) => ({
        key,
        enabled,
        meta: FLAG_META[key] ?? {
          label: key,
          area: "General",
          risk: "Medium" as const,
          productionDefault: false,
        },
      }))
      .sort((a, b) => a.key.localeCompare(b.key));
  }, [features]);

  const enabledCount = rows.filter((row) => row.enabled).length;
  const disabledCount = rows.length - enabledCount;
  const environment = features?.environment || "unknown";
  const provider = features?.provider || "unknown";
  const productionHighRiskOn = rows.some((row) => environment === "production" && row.meta.risk === "High" && row.enabled);

  return (
    <AdminShell title="Features" loading={loading} onRefresh={loadFeatures}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <style>{`
        .ad-b-red { background: var(--danger-soft); color: var(--danger); border: 1px solid color-mix(in srgb, var(--danger) 24%, transparent); }
        .ad-b-amber { background: var(--warning-soft); color: var(--warning); border: 1px solid color-mix(in srgb, var(--warning) 28%, transparent); }
        .ad-feature-state { display: inline-flex; align-items: center; gap: 6px; font-family: var(--font-geist-mono), monospace; font-size: 11px; font-weight: 700; }
        .ad-feature-dot { width: 7px; height: 7px; border-radius: 999px; display: inline-block; }
        .ad-feature-on { color: var(--success); }
        .ad-feature-on .ad-feature-dot { background: var(--success); box-shadow: 0 0 0 3px var(--success-soft); }
        .ad-feature-off { color: var(--dmuted); }
        .ad-feature-off .ad-feature-dot { background: var(--dmuted2); box-shadow: 0 0 0 3px var(--surface2); }
      `}</style>

      <div className="ad-section-header">
        <div className="ad-section-title">Runtime Status</div>
        <div className="ad-section-meta">Backend evaluated values</div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Environment" value={environment} sub={environment === "production" ? "production runtime" : "non-production runtime"} valueColor={environment !== "production" ? "accent" : undefined} />
        <StatCard label="Provider" value={provider} sub={provider === "unleash" ? "remote flag service" : "environment fallback"} valueColor={provider === "unleash" ? "accent" : undefined} />
        <StatCard label="Enabled Flags" value={String(enabledCount)} sub={`${disabledCount} disabled`} />
        <StatCard label="Production Guard" value={productionHighRiskOn ? "Review" : "Clear"} sub={productionHighRiskOn ? "high-risk flag enabled" : "high-risk flags off"} subColor={productionHighRiskOn ? "down" : "up"} />
      </div>

      <div className="ad-section-header">
        <div className="ad-section-title">Flags</div>
        <div className="ad-section-meta">{rows.length} registered</div>
      </div>

      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Feature</th>
              <th>Flag key</th>
              <th>Area</th>
              <th>Risk</th>
              <th>Current</th>
              <th>Prod default</th>
            </tr>
          </thead>
          <tbody>
            {rows.length > 0 ? (
              rows.map((row) => (
                <tr key={row.key}>
                  <td>
                    <div style={{ fontWeight: 600 }}>{flagLabel(row.key)}</div>
                  </td>
                  <td><span className="ad-mono">{row.key}</span></td>
                  <td>{row.meta.area}</td>
                  <td><span className={riskBadgeClass(row.meta.risk)}>{row.meta.risk}</span></td>
                  <td>
                    <span className={`ad-feature-state ${row.enabled ? "ad-feature-on" : "ad-feature-off"}`}>
                      <span className="ad-feature-dot" />
                      {row.enabled ? "ON" : "OFF"}
                    </span>
                  </td>
                  <td>
                    <span className={`ad-feature-state ${row.meta.productionDefault ? "ad-feature-on" : "ad-feature-off"}`}>
                      <span className="ad-feature-dot" />
                      {row.meta.productionDefault ? "ON" : "OFF"}
                    </span>
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td colSpan={6} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>
                  {loading ? "Loading..." : "No feature flags registered"}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </AdminShell>
  );
}
