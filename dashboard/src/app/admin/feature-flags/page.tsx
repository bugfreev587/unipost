"use client";

import { useAuth } from "@clerk/nextjs";
import { Check, LoaderCircle, ShieldCheck, ToggleLeft, ToggleRight } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import {
  listAdminFeatureFlags,
  updateAdminFeatureFlag,
  type AdminFeatureFlag,
} from "@/lib/api";

import { AdminShell } from "../_components/admin-ui";

const FLAG_ORDER = ["x_dms_v1", "x_credits_billing_v1"] as const;

export default function AdminFeatureFlagsPage() {
  const { getToken } = useAuth();
  const [flags, setFlags] = useState<AdminFeatureFlag[]>([]);
  const [loading, setLoading] = useState(true);
  const [savingKey, setSavingKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await listAdminFeatureFlags(token);
      setFlags(
        [...response.data].sort(
          (a, b) => FLAG_ORDER.indexOf(a.key) - FLAG_ORDER.indexOf(b.key),
        ),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load feature flags");
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => {
    void load();
  }, [load]);

  async function setEnabled(flag: AdminFeatureFlag, enabled: boolean) {
    const audience = enabled ? "available to regular users" : "unavailable to regular users";
    if (!window.confirm(`Turn ${flag.label} ${enabled ? "ON" : "OFF"}?\n\nIt will be ${audience}.`)) {
      return;
    }
    setSavingKey(flag.key);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await updateAdminFeatureFlag(token, flag.key, enabled);
      setFlags((current) => current.map((item) => (item.key === flag.key ? response.data : item)));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update feature flag");
    } finally {
      setSavingKey(null);
    }
  }

  return (
    <AdminShell title="Feature Flags" loading={loading} onRefresh={load} requireSuperAdmin>
      <style>{featureFlagsCss}</style>

      <div className="ad-section-header aff-header">
        <div>
          <div className="ad-section-title">Customer feature availability</div>
          <div className="ad-section-meta">
            Changes apply globally across development, staging, and production.
          </div>
        </div>
        <span className="ad-badge ad-b-gray">Super Admin only</span>
      </div>

      <div className="aff-policy">
        <ShieldCheck aria-hidden="true" />
        <div>
          <strong>OFF keeps the rollout private.</strong>
          <span>
            The feature is unavailable to regular users, while Super Admin-owned workspaces retain access for acceptance testing.
          </span>
        </div>
      </div>

      {error ? <div className="aff-error" role="alert">{error}</div> : null}

      <div className="aff-grid" aria-busy={loading}>
        {loading && flags.length === 0 ? (
          <div className="aff-loading"><LoaderCircle aria-hidden="true" /> Loading feature flags…</div>
        ) : flags.map((flag) => {
          const saving = savingKey === flag.key;
          return (
            <article className="aff-card" key={flag.key}>
              <div className="aff-card-top">
                <div>
                  <div className="aff-title-row">
                    <h2>{flag.label}</h2>
                    <span className={`ad-badge ${flag.enabled ? "ad-b-green" : "ad-b-gray"}`}>
                      {flag.enabled ? "ON" : "OFF"}
                    </span>
                  </div>
                  <code>{flag.key}</code>
                </div>
                <button
                  type="button"
                  className={`aff-toggle ${flag.enabled ? "is-on" : ""}`}
                  onClick={() => void setEnabled(flag, !flag.enabled)}
                  disabled={saving}
                  aria-label={`${flag.enabled ? "Disable" : "Enable"} ${flag.label}`}
                  aria-pressed={flag.enabled}
                >
                  {saving
                    ? <LoaderCircle className="aff-spin" aria-hidden="true" />
                    : flag.enabled
                      ? <ToggleRight aria-hidden="true" />
                      : <ToggleLeft aria-hidden="true" />}
                  {saving ? "Saving…" : flag.enabled ? "Turn OFF" : "Turn ON"}
                </button>
              </div>

              <p>{flag.description}</p>

              <div className="aff-state">
                <Check aria-hidden="true" />
                <span>
                  {flag.enabled
                    ? "Feature is available to regular users."
                    : "Feature is not available to regular users."}
                </span>
              </div>

              <dl>
                <div><dt>Owner area</dt><dd>{flag.owner_area}</dd></div>
                <div><dt>Last changed</dt><dd>{new Date(flag.updated_at).toLocaleString()}</dd></div>
              </dl>
            </article>
          );
        })}
      </div>
    </AdminShell>
  );
}

const featureFlagsCss = `
.aff-header { align-items: flex-start; }
.aff-policy {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 18px;
  padding: 14px 16px;
  border: 1px solid var(--dborder);
  border-radius: 10px;
  background: var(--surface2);
}
.aff-policy svg { width: 18px; height: 18px; margin-top: 1px; color: var(--daccent); flex: 0 0 auto; }
.aff-policy div { display: grid; gap: 3px; }
.aff-policy strong { font-size: 13px; }
.aff-policy span { color: var(--dmuted); font-size: 12px; line-height: 1.55; }
.aff-error {
  margin-bottom: 16px;
  padding: 12px 14px;
  border: 1px solid color-mix(in srgb, var(--danger) 24%, transparent);
  border-radius: 8px;
  background: var(--danger-soft);
  color: var(--danger);
}
.aff-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.aff-card {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--dborder);
  border-radius: 10px;
  background: var(--surface);
}
.aff-card-top { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; }
.aff-title-row { display: flex; align-items: center; gap: 9px; }
.aff-title-row h2 { margin: 0; font-size: 17px; letter-spacing: -0.02em; }
.aff-card code { display: block; margin-top: 5px; color: var(--dmuted); font-size: 11px; }
.aff-card p { min-height: 42px; margin: 18px 0 14px; color: var(--dmuted); line-height: 1.6; }
.aff-toggle {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 36px;
  padding: 7px 11px;
  border: 1px solid var(--dborder2);
  border-radius: 8px;
  background: var(--surface2);
  color: var(--dmuted);
  font: inherit;
  font-weight: 650;
  cursor: pointer;
  white-space: nowrap;
}
.aff-toggle svg { width: 20px; height: 20px; }
.aff-toggle.is-on { border-color: color-mix(in srgb, var(--success) 35%, var(--dborder)); color: var(--success); }
.aff-toggle:hover:not(:disabled) { border-color: var(--daccent); color: var(--dtext); }
.aff-toggle:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 2px; }
.aff-toggle:disabled { opacity: .55; cursor: wait; }
.aff-state {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 11px;
  border: 1px solid var(--dborder);
  border-radius: 8px;
  background: var(--surface2);
  font-size: 12px;
}
.aff-state svg { width: 14px; height: 14px; color: var(--daccent); }
.aff-card dl { display: grid; gap: 8px; margin: 16px 0 0; padding-top: 14px; border-top: 1px solid var(--dborder); }
.aff-card dl div { display: flex; justify-content: space-between; gap: 12px; }
.aff-card dt { color: var(--dmuted); }
.aff-card dd { margin: 0; text-align: right; }
.aff-loading {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  min-height: 180px;
  border: 1px solid var(--dborder);
  border-radius: 10px;
  color: var(--dmuted);
}
.aff-loading svg, .aff-spin { animation: aff-spin .8s linear infinite; }
@keyframes aff-spin { to { transform: rotate(360deg); } }
@media (max-width: 820px) {
  .aff-grid { grid-template-columns: 1fr; }
}
@media (max-width: 560px) {
  .aff-card-top { display: grid; }
  .aff-toggle { width: 100%; justify-content: center; }
  .aff-card p { min-height: 0; }
}
`;
