"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { getWorkspace, updateWorkspace, type Workspace } from "@/lib/api";

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
        const res = await getWorkspace(token);
        setWorkspace(res.data);
        setName(res.data.name);
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
      const res = await updateWorkspace(token, { name: name.trim() });
      setWorkspace(res.data);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (err) { console.error("Failed to save:", err); } finally { setSaving(false); }
  }

  if (!workspace) return <div style={{ color: "var(--dmuted)", fontSize: 14, lineHeight: "20px" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Workspace Settings</div>
          <div style={{ fontSize: 14, color: "var(--dmuted)", marginTop: 6 }}>Manage your workspace</div>
        </div>
      </div>

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
            <span style={{ fontSize: 12, lineHeight: "16px", fontWeight: 600, color: "var(--dmuted)" }}>Workspace ID</span>
            <span className="mono">{workspace.id}</span>
          </div>
          <div className="settings-row">
            <span style={{ fontSize: 12, lineHeight: "16px", fontWeight: 600, color: "var(--dmuted)" }}>Created</span>
            <span style={{ fontSize: 13, lineHeight: "18px", color: "var(--dtext)" }}>
              {new Date(workspace.created_at).toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" })}
            </span>
          </div>
        </div>
      </div>
    </>
  );
}
