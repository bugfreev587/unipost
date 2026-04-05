"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { getProject, updateProject, deleteProject, type Project } from "@/lib/api";
import { ExternalLink } from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

const CRED_PLATFORMS = [
  { id: "instagram", name: "Meta (Instagram / Threads)", idLabel: "App ID", secretLabel: "App Secret", docs: "https://developers.facebook.com" },
  { id: "linkedin", name: "LinkedIn", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://developer.linkedin.com" },
  { id: "tiktok", name: "TikTok", idLabel: "Client Key", secretLabel: "Client Secret", docs: "https://developers.tiktok.com" },
  { id: "youtube", name: "YouTube", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://console.cloud.google.com" },
  { id: "twitter", name: "X / Twitter", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://developer.x.com" },
];

interface PlatformCred { platform: string; client_id: string; created_at: string; }

export default function SettingsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [creds, setCreds] = useState<PlatformCred[]>([]);
  const [credForms, setCredForms] = useState<Record<string, { clientId: string; clientSecret: string }>>({});
  const [credSaving, setCredSaving] = useState<string | null>(null);
  const [credError, setCredError] = useState("");

  useEffect(() => {
    async function load() {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getProject(token, projectId);
        setProject(res.data); setName(res.data.name);
      } catch (err) { console.error("Failed:", err); }
    }
    load();
  }, [getToken, projectId]);

  const loadCreds = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials`, { headers: { Authorization: `Bearer ${token}` } });
      const data = await res.json();
      setCreds(data.data || []);
    } catch { /* silent */ }
  }, [getToken, projectId]);

  useEffect(() => { loadCreds(); }, [loadCreds]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setSaving(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await updateProject(token, projectId, { name: name.trim() });
      setProject(res.data);
    } catch (err) { console.error("Failed:", err); } finally { setSaving(false); }
  }

  async function handleDelete() {
    if (!confirm("Delete this project? All data will be permanently removed.")) return;
    setDeleting(true);
    try {
      const token = await getToken();
      if (!token) return;
      await deleteProject(token, projectId);
      router.push("/");
    } catch (err) { console.error("Failed:", err); } finally { setDeleting(false); }
  }

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
      const res = await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials`, {
        method: "POST", headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ platform, client_id: form.clientId, client_secret: form.clientSecret }),
      });
      if (!res.ok) { const err = await res.json(); setCredError(err.error?.message || "Failed"); return; }
      setCredForms((prev) => ({ ...prev, [platform]: { clientId: "", clientSecret: "" } }));
      loadCreds();
    } catch { setCredError("Failed to save"); } finally { setCredSaving(null); }
  }

  async function handleCredDelete(platform: string) {
    if (!confirm(`Remove ${platform} credentials?`)) return;
    try {
      const token = await getToken();
      if (!token) return;
      await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials/${platform}`, { method: "DELETE", headers: { Authorization: `Bearer ${token}` } });
      loadCreds();
    } catch { /* silent */ }
  }

  const configuredPlatforms = new Set(creds.map((c) => c.platform));
  const isPaid = project?.mode !== "quickstart" || creds.length > 0;

  if (!project) return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 24 }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 600, letterSpacing: -0.4, color: "var(--dtext)" }}>Settings</div>
          <div style={{ fontSize: 12.5, color: "var(--dmuted)", marginTop: 3 }}>Project configuration and credentials</div>
        </div>
      </div>

      {/* General */}
      <div className="settings-section">
        <div className="settings-section-header">General</div>
        <div className="settings-section-body">
          <form onSubmit={handleSave}>
            <label className="dform-label">Project Name</label>
            <div style={{ display: "flex", gap: 10, marginBottom: 16 }}>
              <input className="dform-input" value={name} onChange={(e) => setName(e.target.value)} style={{ flex: 1 }} />
              <button type="submit" className="dbtn dbtn-primary" disabled={saving || !name.trim()}>
                {saving ? "Saving..." : "Save"}
              </button>
            </div>
          </form>
          <div className="settings-row">
            <span style={{ fontSize: 12.5, fontWeight: 500, color: "var(--dmuted)" }}>Mode</span>
            <span className="dbadge dbadge-green"><span className="dbadge-dot" />{project.mode}</span>
          </div>
          <div className="settings-row">
            <span style={{ fontSize: 12.5, fontWeight: 500, color: "var(--dmuted)" }}>Project ID</span>
            <span className="mono">{project.id}</span>
          </div>
          <div className="settings-row">
            <span style={{ fontSize: 12.5, fontWeight: 500, color: "var(--dmuted)" }}>Created</span>
            <span style={{ fontSize: 12.5, color: "var(--dtext)" }}>
              {new Date(project.created_at).toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" })}
            </span>
          </div>
        </div>
      </div>

      {/* Credentials */}
      <div className="settings-section">
        <div className="settings-section-header">
          Native Mode Credentials
          <span className="dbadge dbadge-amber">BYOC</span>
        </div>
        <div className="settings-section-body">
          <div style={{ color: "var(--dmuted)", fontSize: 12.5, marginBottom: 16, lineHeight: 1.7 }}>
            Bring your own developer credentials so users see your app name during OAuth.
          </div>
          {credError && (
            <div style={{ padding: "8px 12px", borderRadius: 6, background: "#ef444410", border: "1px solid #ef444425", fontSize: 12, color: "var(--danger)", marginBottom: 14 }}>
              {credError}
            </div>
          )}
          {CRED_PLATFORMS.map((p) => {
            const configured = configuredPlatforms.has(p.id);
            const cred = creds.find((c) => c.platform === p.id);
            const form = credForms[p.id] || { clientId: "", clientSecret: "" };
            return (
              <div key={p.id} style={{ marginBottom: 14 }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
                  <label className="dform-label" style={{ marginBottom: 0 }}>{p.name}</label>
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    {configured ? (
                      <>
                        <span className="dbadge dbadge-green" style={{ fontSize: 10 }}>Native</span>
                        <button className="dbtn dbtn-danger" style={{ padding: "2px 8px", fontSize: 10 }} onClick={() => handleCredDelete(p.id)}>Remove</button>
                      </>
                    ) : (
                      <a href={p.docs} target="_blank" rel="noopener noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 11, color: "var(--dmuted2)", textDecoration: "none" }}>
                        Docs <ExternalLink style={{ width: 10, height: 10 }} />
                      </a>
                    )}
                  </div>
                </div>
                {configured ? (
                  <div style={{ fontSize: 12, color: "var(--dmuted)" }}>
                    Client ID: <span className="mono">{cred?.client_id}</span>
                  </div>
                ) : (
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr auto", gap: 8 }}>
                    <input className="dform-input" placeholder={p.idLabel} value={form.clientId} onChange={(e) => updateCredForm(p.id, "clientId", e.target.value)} />
                    <input className="dform-input" type="password" placeholder={p.secretLabel} value={form.clientSecret} onChange={(e) => updateCredForm(p.id, "clientSecret", e.target.value)} />
                    <button className="dbtn dbtn-primary" onClick={() => handleCredSave(p.id)} disabled={credSaving === p.id || !form.clientId || !form.clientSecret} style={{ padding: "6px 12px" }}>
                      {credSaving === p.id ? "..." : "Save"}
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Danger */}
      <div className="settings-section danger-section">
        <div className="settings-section-header">Danger Zone</div>
        <div className="settings-section-body">
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div>
              <div style={{ fontSize: 13, fontWeight: 500, marginBottom: 3, color: "var(--dtext)" }}>Delete Project</div>
              <div style={{ fontSize: 12, color: "var(--dmuted)" }}>Permanently delete this project and all associated data.</div>
            </div>
            <button className="dbtn dbtn-danger" onClick={handleDelete} disabled={deleting}>
              {deleting ? "Deleting..." : "Delete Project"}
            </button>
          </div>
        </div>
      </div>
    </>
  );
}
