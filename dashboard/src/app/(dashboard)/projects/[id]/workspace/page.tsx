"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { listWorkspaces, updateWorkspace, type Workspace } from "@/lib/api";

export default function WorkspaceSettingsPage() {
  const { getToken } = useAuth();
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await listWorkspaces(token);
        if (res.data.length > 0) {
          setWorkspace(res.data[0]);
          setName(res.data[0].name);
        }
      } catch (err) { console.error("Failed to load workspace:", err); }
    }
    load();
  }, [getToken]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!workspace || !name.trim()) return;
    setSaving(true);
    setSaved(false);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await updateWorkspace(token, workspace.id, { name: name.trim() });
      setWorkspace(res.data);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (err) { console.error("Failed to save:", err); } finally { setSaving(false); }
  }

  if (!workspace) return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Workspace Settings</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>Manage your workspace</div>
        </div>
      </div>

      {/* General */}
      <div className="settings-section">
        <div className="settings-section-header">General</div>
        <div className="settings-section-body">
          <form onSubmit={handleSave}>
            <label className="dform-label">Workspace Name</label>
            <div style={{ display: "flex", gap: 10, marginBottom: 16 }}>
              <input className="dform-input" value={name} onChange={(e) => setName(e.target.value)} style={{ flex: 1 }} />
              <button type="submit" className="dbtn dbtn-primary" disabled={saving || !name.trim()}>
                {saving ? "Saving..." : saved ? "✓ Saved" : "Save"}
              </button>
            </div>
          </form>
          <div className="settings-row">
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>Workspace ID</span>
            <span className="mono">{workspace.id}</span>
          </div>
          <div className="settings-row">
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>Created</span>
            <span style={{ fontSize: 13, color: "var(--dtext)" }}>
              {new Date(workspace.created_at).toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" })}
            </span>
          </div>
        </div>
      </div>

      {/* Multi-Workspace */}
      <div className="settings-section">
        <div className="settings-section-header">Multi-Workspace</div>
        <div className="settings-section-body">
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
            <div>
              <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 3, color: "var(--dtext)" }}>Create New Workspace</div>
              <div style={{ fontSize: 13, color: "var(--dmuted)" }}>Separate security boundary with its own API keys, billing, and posts.</div>
            </div>
            <button className="dbtn" disabled style={{ opacity: 0.4, cursor: "not-allowed" }}>
              Coming Soon
            </button>
          </div>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div>
              <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 3, color: "var(--dtext)" }}>Switch Workspace</div>
              <div style={{ fontSize: 13, color: "var(--dmuted)" }}>Switch between your workspaces.</div>
            </div>
            <button className="dbtn" disabled style={{ opacity: 0.4, cursor: "not-allowed" }}>
              Coming Soon
            </button>
          </div>
        </div>
      </div>

      {/* Danger Zone */}
      <div className="settings-section danger-section">
        <div className="settings-section-header">Danger Zone</div>
        <div className="settings-section-body">
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div>
              <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 3, color: "var(--dtext)" }}>Delete Workspace</div>
              <div style={{ fontSize: 13, color: "var(--dmuted)" }}>Cannot delete your only workspace.</div>
            </div>
            <button className="dbtn dbtn-danger" disabled style={{ opacity: 0.4, cursor: "not-allowed" }}>
              Delete Workspace
            </button>
          </div>
        </div>
      </div>
    </>
  );
}
