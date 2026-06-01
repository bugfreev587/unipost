"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useParams } from "next/navigation";
import { Clipboard, ExternalLink, Lock } from "lucide-react";
import { ConfirmModal } from "@/components/confirm-modal";
import { PlatformCredentialsStats } from "@/components/dashboard/connection-stats";
import {
  createPlatformCredential,
  deletePlatformCredential,
  getApiLimits,
  listPlatformCredentials,
  type PlatformCredential,
} from "@/lib/api";

const API_BASE_URL = (process.env.NEXT_PUBLIC_API_URL || "https://api.unipost.dev").replace(/\/+$/, "");

const CRED_PLATFORMS = [
  { id: "instagram", name: "Instagram", idLabel: "App ID", secretLabel: "App Secret", docs: "/docs/white-label/meta", developerPortal: "https://developers.facebook.com" },
  { id: "threads", name: "Threads", idLabel: "App ID", secretLabel: "App Secret", docs: "/docs/white-label/meta", developerPortal: "https://developers.facebook.com" },
  { id: "facebook", name: "Facebook Page", idLabel: "App ID", secretLabel: "App Secret", docs: "/docs/white-label/meta", developerPortal: "https://developers.facebook.com" },
  { id: "linkedin", name: "LinkedIn", idLabel: "Client ID", secretLabel: "Client Secret", docs: "/docs/white-label/linkedin", developerPortal: "https://developer.linkedin.com" },
  { id: "pinterest", name: "Pinterest", idLabel: "App ID", secretLabel: "App Secret", docs: null, developerPortal: "https://developers.pinterest.com" },
  { id: "tiktok", name: "TikTok", idLabel: "Client Key", secretLabel: "Client Secret", docs: "/docs/white-label/tiktok", developerPortal: "https://developers.tiktok.com" },
  { id: "youtube", name: "YouTube", idLabel: "Client ID", secretLabel: "Client Secret", docs: "/docs/white-label/youtube", developerPortal: "https://console.cloud.google.com" },
  { id: "twitter", name: "X / Twitter", idLabel: "Client ID", secretLabel: "Client Secret", docs: "/docs/white-label/twitter", developerPortal: "https://developer.x.com" },
];

type CredentialForm = {
  clientId: string;
  clientSecret: string;
};

export default function CredentialsPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [creds, setCreds] = useState<PlatformCredential[]>([]);
  const [loading, setLoading] = useState(true);
  const [pageError, setPageError] = useState("");
  const [forms, setForms] = useState<Record<string, CredentialForm>>({});
  const [saving, setSaving] = useState<string | null>(null);
  const [formError, setFormError] = useState("");
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);
  const [copiedPlatform, setCopiedPlatform] = useState<string | null>(null);
  const [credentialLimit, setCredentialLimit] = useState<number | null>(null);

  const configuredPlatforms = useMemo(() => new Set(creds.map((c) => c.platform)), [creds]);
  const writesLocked = credentialLimit === 0;

  const loadPage = useCallback(async () => {
    try {
      setPageError("");
      const token = await getToken();
      if (!token) return;
      const [credsRes, limitsRes] = await Promise.all([
        listPlatformCredentials(token),
        getApiLimits(token).catch(() => null),
      ]);
      setCreds(credsRes.data ?? []);
      if (limitsRes) {
        setCredentialLimit(limitsRes.data.white_label_platform_limit);
      }
    } catch (e) {
      setPageError((e as Error).message || "Failed to load platform credentials");
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => { loadPage(); }, [loadPage]);

  function updateForm(platform: string, field: keyof CredentialForm, value: string) {
    setForms((prev) => {
      const current = prev[platform] ?? { clientId: "", clientSecret: "" };
      return { ...prev, [platform]: { ...current, [field]: value } };
    });
  }

  async function handleSave(platform: string) {
    const form = forms[platform];
    if (!form?.clientId || !form?.clientSecret || writesLocked) return;
    setSaving(platform);
    setFormError("");
    try {
      const token = await getToken();
      if (!token) return;
      await createPlatformCredential(token, {
        platform,
        client_id: form.clientId,
        client_secret: form.clientSecret,
      });
      setForms((prev) => ({ ...prev, [platform]: { clientId: "", clientSecret: "" } }));
      await loadPage();
    } catch (e) {
      setFormError((e as Error).message || "Failed to save platform credentials");
    } finally {
      setSaving(null);
    }
  }

  async function handleDelete(platform: string) {
    try {
      const token = await getToken();
      if (!token) return;
      await deletePlatformCredential(token, platform);
      await loadPage();
    } catch (e) {
      setFormError((e as Error).message || "Failed to remove platform credentials");
    } finally {
      setRemoveTarget(null);
    }
  }

  async function copyCallback(platform: string) {
    const callbackUrl = `${API_BASE_URL}/v1/connect/callback/${platform}`;
    await navigator.clipboard.writeText(callbackUrl);
    setCopiedPlatform(platform);
    window.setTimeout(() => setCopiedPlatform((current) => current === platform ? null : current), 1500);
  }

  if (loading) return <div style={{ color: "var(--dmuted)", fontSize: 14, lineHeight: "20px" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div className="dt-page-title">Platform Credentials</div>
          <div className="dt-subtitle" style={{ maxWidth: 720, lineHeight: 1.6 }}>
            Add your official platform developer app credentials when you want UniPost flows to use your app identity and quota.
            Connections and Hosted Connect both use these credentials when a matching platform is configured.
          </div>
        </div>
      </div>

      {pageError && (
        <div style={{ padding: "10px 14px", borderRadius: 8, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", fontSize: 13, color: "var(--danger)", marginBottom: 20 }}>
          {pageError}
        </div>
      )}

      <div
        style={{
          padding: "14px 16px",
          marginBottom: 20,
          borderRadius: 10,
          background: "color-mix(in srgb, var(--surface2) 88%, white)",
          border: "1px solid var(--dborder)",
        }}
      >
        <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)", marginBottom: 4 }}>
          Credentials are shared across connection modes
        </div>
        <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>
          If a platform is configured here, UniPost uses your developer app for dashboard connections and Hosted Connect sessions.
          If it is not configured, eligible flows can still use UniPost&apos;s shared OAuth apps. Hosted Connect branding is managed in{" "}
          <Link href={`/projects/${profileId}/accounts/native`} style={{ color: "var(--daccent)", textDecoration: "none" }}>
            Hosted Connect
          </Link>
          .
        </div>
      </div>

      {credentialLimit !== null && (
        <div
          style={{
            padding: "14px 16px",
            marginBottom: 20,
            borderRadius: 10,
            background: "color-mix(in srgb, var(--surface2) 88%, white)",
            border: "1px solid var(--dborder)",
          }}
        >
          <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)", marginBottom: 4 }}>
            Plan capacity
          </div>
          <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>
            {credentialLimit === 0 && "Your current plan can use UniPost's shared OAuth apps, but cannot save custom platform credentials."}
            {credentialLimit === 1 && `Your current plan includes custom credentials for 1 platform. ${configuredPlatforms.size}/1 platform slot configured.`}
            {credentialLimit === -1 && "Your current plan includes custom credentials across all supported platforms."}
          </div>
        </div>
      )}

      {writesLocked && (
        <div
          style={{
            display: "flex",
            gap: 14,
            alignItems: "flex-start",
            padding: "16px 18px",
            marginBottom: 20,
            background: "color-mix(in srgb, var(--daccent) 9%, transparent)",
            border: "1px solid color-mix(in srgb, var(--daccent) 30%, transparent)",
            borderRadius: 10,
          }}
        >
          <div
            style={{
              width: 34,
              height: 34,
              flexShrink: 0,
              borderRadius: 8,
              background: "color-mix(in srgb, var(--daccent) 18%, transparent)",
              color: "var(--daccent)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Lock size={16} />
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)", marginBottom: 3 }}>
              Custom platform credentials require Basic or higher
            </div>
            <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.55 }}>
              You can still copy redirect URLs and use UniPost&apos;s shared OAuth apps. Upgrade when you want your own developer app identity and quota.
            </div>
          </div>
          <Link
            href="/settings/billing"
            className="dbtn dbtn-primary"
            style={{ fontSize: 13, padding: "8px 16px", whiteSpace: "nowrap", flexShrink: 0 }}
          >
            Upgrade plan
          </Link>
        </div>
      )}

      <PlatformCredentialsStats configuredCount={configuredPlatforms.size} totalPlatforms={CRED_PLATFORMS.length} />

      {formError && (
        <div style={{ padding: "10px 14px", borderRadius: 8, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", fontSize: 13, color: "var(--danger)", marginBottom: 20 }}>
          {formError}
        </div>
      )}

      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {CRED_PLATFORMS.map((p) => {
          const configured = configuredPlatforms.has(p.id);
          const cred = creds.find((c) => c.platform === p.id);
          const form = forms[p.id] || { clientId: "", clientSecret: "" };
          const callbackUrl = `${API_BASE_URL}/v1/connect/callback/${p.id}`;

          return (
            <div key={p.id} className="settings-section" style={{ marginBottom: 0 }}>
              <div className="settings-section-header" style={{ alignItems: "flex-start", gap: 14 }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
                    <span style={{ fontSize: 14, fontWeight: 600, color: "var(--dtext)" }}>{p.name}</span>
                    {configured && (
                      <span className="dbadge dbadge-green" style={{ fontSize: 10 }}>Configured</span>
                    )}
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", marginTop: 8 }}>
                    <span className="dt-mono" style={{ color: "var(--dmuted)", fontSize: 11, overflowWrap: "anywhere" }}>
                      {callbackUrl}
                    </span>
                    <button
                      type="button"
                      className="dbtn dbtn-ghost"
                      onClick={() => void copyCallback(p.id)}
                      style={{ padding: "4px 8px", fontSize: 11 }}
                    >
                      <Clipboard size={12} />
                      {copiedPlatform === p.id ? "Copied" : "Copy URL"}
                    </button>
                  </div>
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", justifyContent: "flex-end" }}>
                  {p.docs && (
                    <Link href={p.docs} style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 12, color: "var(--dmuted)", textDecoration: "none" }}>
                      Docs
                    </Link>
                  )}
                  <a href={p.developerPortal} target="_blank" rel="noopener noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 12, color: "var(--dmuted)", textDecoration: "none" }}>
                    Developer Portal <ExternalLink style={{ width: 11, height: 11 }} />
                  </a>
                  {configured && (
                    <button className="dbtn dbtn-danger" style={{ padding: "4px 10px", fontSize: 11 }} onClick={() => setRemoveTarget(p.id)}>Remove</button>
                  )}
                </div>
              </div>
              <div className="settings-section-body">
                {configured ? (
                  <div style={{ fontSize: 13, color: "var(--dmuted)" }}>
                    Client ID: <span className="mono">{cred?.client_id}</span>
                  </div>
                ) : (
                  <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 10 }}>
                    <input
                      className="dform-input"
                      placeholder={p.idLabel}
                      value={form.clientId}
                      onChange={(e) => updateForm(p.id, "clientId", e.target.value)}
                      disabled={writesLocked}
                    />
                    <input
                      className="dform-input"
                      type="password"
                      placeholder={p.secretLabel}
                      value={form.clientSecret}
                      onChange={(e) => updateForm(p.id, "clientSecret", e.target.value)}
                      disabled={writesLocked}
                    />
                    <button
                      className="dbtn dbtn-primary"
                      onClick={() => handleSave(p.id)}
                      disabled={writesLocked || saving === p.id || !form.clientId || !form.clientSecret}
                      style={{ padding: "8px 16px" }}
                    >
                      {saving === p.id ? "..." : "Save"}
                    </button>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>

      <ConfirmModal
        open={!!removeTarget}
        title="Remove Credentials"
        message="Remove these platform credentials? UniPost will fall back to shared OAuth apps for eligible flows on this platform."
        confirmLabel="Remove"
        variant="danger"
        onConfirm={() => removeTarget && handleDelete(removeTarget)}
        onCancel={() => setRemoveTarget(null)}
      />
    </>
  );
}
