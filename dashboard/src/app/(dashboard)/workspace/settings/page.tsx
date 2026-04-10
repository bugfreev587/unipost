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

  async function handleUsageModesChange(modes: string[]) {
    if (!workspace) return;
    try {
      const token = await getToken();
      if (!token) return;
      const res = await updateWorkspace(token, workspace.id, { name: workspace.name, usage_modes: modes } as any);
      setWorkspace(res.data);
    } catch (err) { console.error("Failed to update:", err); }
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

      {/* Dashboard Features */}
      <div className="settings-section">
        <div className="settings-section-header">Dashboard Features</div>
        <div className="settings-section-body">
          <p style={{ fontSize: 13, color: "var(--dmuted)", marginBottom: 16, lineHeight: 1.6 }}>
            Controls which tools are visible in the sidebar. You can enable or disable features anytime.
          </p>
          {[
            { id: "personal", label: "Post to my own accounts", desc: "Connect accounts via Quickstart and publish directly." },
            { id: "whitelabel", label: "Post with my own app credentials", desc: "Use your own OAuth apps for branded authorization." },
            { id: "api", label: "Build an app on UniPost API", desc: "Integrate UniPost into your product via API + Connect Flow." },
          ].map((feature) => {
            const checked = workspace.usage_modes.includes(feature.id);
            return (
              <label key={feature.id} style={{
                display: "flex", alignItems: "flex-start", gap: 12, padding: "10px 0",
                cursor: "pointer", borderBottom: "1px solid var(--dborder)",
              }}>
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => {
                    const next = checked
                      ? workspace.usage_modes.filter((m) => m !== feature.id)
                      : [...workspace.usage_modes, feature.id];
                    handleUsageModesChange(next);
                  }}
                  style={{ marginTop: 2, accentColor: "#10b981" }}
                />
                <div>
                  <div style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)", marginBottom: 2 }}>{feature.label}</div>
                  <div style={{ fontSize: 12.5, color: "var(--dmuted)" }}>{feature.desc}</div>
                </div>
              </label>
            );
          })}
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
