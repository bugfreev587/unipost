"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { ExternalLink } from "lucide-react";
import { ConfirmModal } from "@/components/confirm-modal";
import { WhiteLabelStats } from "@/components/dashboard/connection-stats";
import {
  getProfile,
  listPlatformCredentials,
  createPlatformCredential,
  deletePlatformCredential,
  type PlatformCredential,
} from "@/lib/api";

const CRED_PLATFORMS = [
  { id: "instagram", name: "Meta (Instagram / Threads)", idLabel: "App ID", secretLabel: "App Secret", docs: "https://developers.facebook.com" },
  { id: "linkedin", name: "LinkedIn", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://developer.linkedin.com" },
  { id: "tiktok", name: "TikTok", idLabel: "Client Key", secretLabel: "Client Secret", docs: "https://developers.tiktok.com" },
  { id: "youtube", name: "YouTube", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://console.cloud.google.com" },
  { id: "twitter", name: "X / Twitter", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://developer.x.com" },
];

export default function NativeModePage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [workspaceId, setWorkspaceId] = useState<string | null>(null);
  const [creds, setCreds] = useState<PlatformCredential[]>([]);
  const [loading, setLoading] = useState(true);
  const [credForms, setCredForms] = useState<Record<string, { clientId: string; clientSecret: string }>>({});
  const [credSaving, setCredSaving] = useState<string | null>(null);
  const [credError, setCredError] = useState("");
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);

  // Resolve the profile's workspace_id once on mount. Platform
  // credentials are workspace-scoped, not profile-scoped, so the URL
  // param (a profile_id) is not directly usable as the resource
  // identifier — we have to look up the parent workspace first.
  const loadCreds = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const profileRes = await getProfile(token, profileId);
      const wsId = profileRes.data.workspace_id;
      setWorkspaceId(wsId);
      const credsRes = await listPlatformCredentials(token, wsId);
      setCreds(credsRes.data ?? []);
    } catch { /* silent */ }
    finally { setLoading(false); }
  }, [getToken, profileId]);

  useEffect(() => { loadCreds(); }, [loadCreds]);

  function updateCredForm(platform: string, field: "clientId" | "clientSecret", value: string) {
    setCredForms((prev) => ({ ...prev, [platform]: { ...prev[platform], [field]: value } }));
  }

  async function handleCredSave(platform: string) {
    const form = credForms[platform];
    if (!form?.clientId || !form?.clientSecret || !workspaceId) return;
    setCredSaving(platform); setCredError("");
    try {
      const token = await getToken();
      if (!token) return;
      await createPlatformCredential(token, workspaceId, {
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
    if (!workspaceId) { setRemoveTarget(null); return; }
    try {
      const token = await getToken();
      if (!token) return;
      await deletePlatformCredential(token, workspaceId, platform);
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
            Configure your own platform credentials (Native mode). Users will see your app name during OAuth instead of &quot;UniPost&quot;.
          </div>
        </div>
      </div>

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
                  <a href={p.docs} target="_blank" rel="noopener noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 12, color: "var(--dmuted)", textDecoration: "none" }}>
                    Docs <ExternalLink style={{ width: 11, height: 11 }} />
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
        message="Remove these platform credentials? New account connections will use Quickstart mode (UniPost credentials) for this platform."
        confirmLabel="Remove"
        variant="danger"
        onConfirm={() => removeTarget && handleCredDelete(removeTarget)}
        onCancel={() => setRemoveTarget(null)}
      />
    </>
  );
}
