"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { ExternalLink, Lock } from "lucide-react";
import { ConfirmModal } from "@/components/confirm-modal";
import { WhiteLabelStats } from "@/components/dashboard/connection-stats";
import {
  getProfile,
  listPlatformCredentials,
  createPlatformCredential,
  deletePlatformCredential,
  getApiLimits,
  type PlatformCredential,
} from "@/lib/api";

const CRED_PLATFORMS = [
  { id: "instagram", name: "Meta (Instagram / Threads)", idLabel: "App ID", secretLabel: "App Secret", whiteLabelDocs: "/docs/white-label/meta", developerPortal: "https://developers.facebook.com" },
  { id: "linkedin", name: "LinkedIn", idLabel: "Client ID", secretLabel: "Client Secret", whiteLabelDocs: "/docs/white-label/linkedin", developerPortal: "https://developer.linkedin.com" },
  { id: "pinterest", name: "Pinterest", idLabel: "App ID", secretLabel: "App Secret", whiteLabelDocs: null, developerPortal: "https://developers.pinterest.com" },
  { id: "tiktok", name: "TikTok", idLabel: "Client Key", secretLabel: "Client Secret", whiteLabelDocs: "/docs/white-label/tiktok", developerPortal: "https://developers.tiktok.com" },
  { id: "youtube", name: "YouTube", idLabel: "Client ID", secretLabel: "Client Secret", whiteLabelDocs: "/docs/white-label/youtube", developerPortal: "https://console.cloud.google.com" },
  { id: "twitter", name: "X / Twitter", idLabel: "Client ID", secretLabel: "Client Secret", whiteLabelDocs: "/docs/white-label/twitter", developerPortal: "https://developer.x.com" },
];

export default function NativeModePage() {
  useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [creds, setCreds] = useState<PlatformCredential[]>([]);
  const [loading, setLoading] = useState(true);
  const [credForms, setCredForms] = useState<Record<string, { clientId: string; clientSecret: string }>>({});
  const [credSaving, setCredSaving] = useState<string | null>(null);
  const [credError, setCredError] = useState("");
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);
  // null = limits not loaded yet (don't render the upgrade banner
  // prematurely); true = plan permits white-label; false = needs upgrade.
  const [planAllowsWhiteLabel, setPlanAllowsWhiteLabel] = useState<boolean | null>(null);

  const loadCreds = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const [credsRes, limitsRes] = await Promise.all([
        listPlatformCredentials(token),
        getApiLimits(token).catch(() => null),
      ]);
      setCreds(credsRes.data ?? []);
      if (limitsRes) setPlanAllowsWhiteLabel(limitsRes.data.plan_allows_white_label);
    } catch { /* silent */ }
    finally { setLoading(false); }
  }, [getToken]);

  useEffect(() => { loadCreds(); }, [loadCreds]);

  function updateCredForm(platform: string, field: "clientId" | "clientSecret", value: string) {
    setCredForms((prev) => ({ ...prev, [platform]: { ...prev[platform], [field]: value } }));
  }

  async function handleCredSave(platform: string) {
    const form = credForms[platform];
    if (!form?.clientId || !form?.clientSecret) return;
    setCredSaving(platform); setCredError("");
    try {
      const token = await getToken();
      if (!token) return;
      await createPlatformCredential(token, {
        platform,
        client_id: form.clientId,
        client_secret: form.clientSecret,
      });
      setCredForms((prev) => ({ ...prev, [platform]: { clientId: "", clientSecret: "" } }));
      loadCreds();
    } catch (e) {
      setCredError((e as Error).message || "Failed to save");
    } finally {
      setCredSaving(null);
    }
  }

  async function handleCredDelete(platform: string) {
    try {
      const token = await getToken();
      if (!token) return;
      await deletePlatformCredential(token, platform);
      loadCreds();
    } catch { /* silent */ }
    finally { setRemoveTarget(null); }
  }

  const configuredPlatforms = new Set(creds.map((c) => c.platform));

  if (loading) return <div style={{ color: "var(--dmuted)", fontSize: 14, lineHeight: "20px" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>White-label Credentials</div>
          <div style={{ fontSize: 14, color: "var(--dmuted)", marginTop: 6 }}>
            Configure your own platform credentials (Native mode). Users will see your app name during OAuth instead of &quot;UniPost&quot; when you onboard them through white-label Connect flows.
          </div>
        </div>
      </div>

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
          White-label credentials are separate from Quickstart
        </div>
        <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>
          Quickstart always uses UniPost&apos;s shared OAuth apps. Save credentials here only if you want to onboard end users through{" "}
          <Link href="/docs/api/connect/sessions/create" style={{ color: "var(--daccent)", textDecoration: "none" }}>
            Connect Sessions
          </Link>{" "}
          or other white-label flows where the platform consent screen should show your own app name.
        </div>
      </div>

      {planAllowsWhiteLabel === false && (
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
              White-label requires the Growth plan or higher
            </div>
            <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.55 }}>
              Your current plan uses UniPost&apos;s shared OAuth credentials (Quickstart mode).
              Upgrade to Growth ($59/mo) or Team ($149/mo) to plug in your own platform apps —
              users will see <em>your</em> app name during OAuth instead of &quot;UniPost&quot;.
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

      <WhiteLabelStats configuredCount={configuredPlatforms.size} totalPlatforms={CRED_PLATFORMS.length} />

      {credError && (
        <div style={{ padding: "10px 14px", borderRadius: 8, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", fontSize: 13, color: "var(--danger)", marginBottom: 20 }}>
          {credError}
        </div>
      )}

      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {CRED_PLATFORMS.map((p) => {
          const configured = configuredPlatforms.has(p.id);
          const cred = creds.find((c) => c.platform === p.id);
          const form = credForms[p.id] || { clientId: "", clientSecret: "" };
          return (
            <div key={p.id} className="settings-section" style={{ marginBottom: 0 }}>
              <div className="settings-section-header">
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <span style={{ fontSize: 14, fontWeight: 600, color: "var(--dtext)" }}>{p.name}</span>
                  {configured && (
                    <span className="dbadge dbadge-green" style={{ fontSize: 10 }}>Configured</span>
                  )}
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  {p.whiteLabelDocs && (
                    <Link href={p.whiteLabelDocs} style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 12, color: "var(--dmuted)", textDecoration: "none" }}>
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
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr auto", gap: 10 }}>
                    <input className="dform-input" placeholder={p.idLabel} value={form.clientId} onChange={(e) => updateCredForm(p.id, "clientId", e.target.value)} />
                    <input className="dform-input" type="password" placeholder={p.secretLabel} value={form.clientSecret} onChange={(e) => updateCredForm(p.id, "clientSecret", e.target.value)} />
                    <button className="dbtn dbtn-primary" onClick={() => handleCredSave(p.id)} disabled={credSaving === p.id || !form.clientId || !form.clientSecret} style={{ padding: "8px 16px" }}>
                      {credSaving === p.id ? "..." : "Save"}
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
        message="Remove these platform credentials? White-label Connect flows for this platform will fall back to UniPost credentials. Quickstart already uses UniPost credentials."
        confirmLabel="Remove"
        variant="danger"
        onConfirm={() => removeTarget && handleCredDelete(removeTarget)}
        onCancel={() => setRemoveTarget(null)}
      />
    </>
  );
}
