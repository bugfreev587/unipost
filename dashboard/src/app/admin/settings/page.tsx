"use client";

import Link from "next/link";

import { useTheme } from "@/components/theme-provider";

import { AdminShell, PanelRow } from "../_components/admin-ui";

const SETTINGS = {
  appUrl: process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev",
  apiUrl: process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080",
  landingUrl: process.env.NEXT_PUBLIC_LANDING_URL || "https://unipost.dev",
  baseUrl: process.env.NEXT_PUBLIC_BASE_URL || "https://unipost.dev",
  appHost: process.env.NEXT_PUBLIC_APP_HOST || "app.unipost.dev",
  inboxEnabled: process.env.NEXT_PUBLIC_FEATURE_INBOX === "true",
};

function SettingPill({
  active,
  label,
  onClick,
}: {
  active?: boolean;
  label: string;
  onClick?: () => void;
}) {
  return (
    <button
      className="ad-btn"
      onClick={onClick}
      style={{
        background: active ? "var(--accent-dim)" : "var(--surface2)",
        color: active ? "var(--daccent)" : "var(--dtext)",
        border: active
          ? "1px solid color-mix(in srgb, var(--primary) 18%, transparent)"
          : "1px solid var(--dborder2)",
        padding: "5px 12px",
      }}
    >
      {label}
    </button>
  );
}

export default function AdminSettingsPage() {
  const { theme, setTheme } = useTheme();

  return (
    <AdminShell title="Settings">
      <div className="ad-section-header">
        <div className="ad-section-title">Operational settings</div>
        <div className="ad-section-meta">Read-only environment summary plus global admin shortcuts</div>
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "minmax(0, 1.3fr) minmax(320px, 0.7fr)",
          gap: 16,
          alignItems: "start",
        }}
      >
        <div className="ad-tbl-wrap ad-tbl-static" style={{ padding: 18 }}>
          <div className="ad-panel-section">
            <div className="ad-panel-section-title">Public Endpoints</div>
            <PanelRow k="App URL" v={<a className="ad-link" href={SETTINGS.appUrl}>{SETTINGS.appUrl}</a>} />
            <PanelRow k="API URL" v={<a className="ad-link" href={SETTINGS.apiUrl}>{SETTINGS.apiUrl}</a>} />
            <PanelRow k="Landing URL" v={<a className="ad-link" href={SETTINGS.landingUrl}>{SETTINGS.landingUrl}</a>} />
            <PanelRow k="Metadata Base" v={SETTINGS.baseUrl} />
            <PanelRow k="App Host" v={SETTINGS.appHost} />
          </div>

          <div className="ad-panel-section">
            <div className="ad-panel-section-title">Frontend Flags</div>
            <PanelRow
              k="Inbox"
              v={
                <span
                  className="ad-badge"
                  style={SETTINGS.inboxEnabled
                    ? { background: "var(--success-soft)", color: "var(--success)", border: "1px solid color-mix(in srgb, var(--success) 20%, transparent)" }
                    : { background: "var(--surface2)", color: "var(--dmuted)", border: "1px solid var(--dborder2)" }}
                >
                  {SETTINGS.inboxEnabled ? "enabled" : "disabled"}
                </span>
              }
            />
            <PanelRow k="Theme storage key" v={<span className="ad-mono">unipost-theme</span>} />
            <PanelRow k="Admin gate" v={<span className="ad-mono">ADMIN_USERS allowlist</span>} />
          </div>

          <div className="ad-panel-section">
            <div className="ad-panel-section-title">Global Theme</div>
            <div style={{ display: "flex", gap: 8 }}>
              <SettingPill active={theme === "light"} label="Light" onClick={() => setTheme("light")} />
              <SettingPill active={theme === "dark"} label="Dark" onClick={() => setTheme("dark")} />
            </div>
            <div style={{ fontSize: 12, color: "var(--dmuted)", marginTop: 10 }}>
              This writes to the shared `unipost-theme` local storage key and applies across landing, docs, dashboard, and admin.
            </div>
          </div>
        </div>

        <div style={{ display: "grid", gap: 16 }}>
          <div className="ad-tbl-wrap ad-tbl-static" style={{ padding: 18 }}>
            <div className="ad-panel-section-title">Operational Shortcuts</div>
            <div style={{ display: "grid", gap: 10 }}>
              <Link href="/admin/billing" className="ad-link">Open billing operations</Link>
              <Link href="/admin/mrr" className="ad-link">Open revenue breakdown</Link>
              <Link href="/admin/errors" className="ad-link">Open publishing failures</Link>
              <Link href="/settings/notifications" className="ad-link">Open notification settings</Link>
            </div>
          </div>

          <div className="ad-tbl-wrap ad-tbl-static" style={{ padding: 18 }}>
            <div className="ad-panel-section-title">Docs & Surfaces</div>
            <div style={{ display: "grid", gap: 10 }}>
              <Link href="/docs" className="ad-link">Documentation home</Link>
              <Link href="/docs/quickstart" className="ad-link">Quickstart</Link>
              <Link href="/docs/api/billing" className="ad-link">Billing API docs</Link>
              <Link href="/docs/api/notifications" className="ad-link">Notifications API docs</Link>
            </div>
          </div>

          <div className="ad-tbl-wrap ad-tbl-static" style={{ padding: 18 }}>
            <div className="ad-panel-section-title">Notes</div>
            <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.65 }}>
              This page is intentionally read-only for now. The values shown here come from public frontend environment variables and runtime UI state.
              Stripe secrets, webhook secrets, and other server-only settings remain intentionally hidden from the browser surface.
            </div>
          </div>
        </div>
      </div>
    </AdminShell>
  );
}
