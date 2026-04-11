"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { listWorkspaces, updateWorkspace, type Workspace } from "@/lib/api";

export default function WorkspaceSettingsTab() {
  const { getToken } = useAuth();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
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
        setWorkspaces(res.data);
        if (res.data.length > 0) {
          setWorkspace(res.data[0]);
          setName(res.data[0].name);
        }
      } catch (err) {
        console.error("Failed to load workspaces:", err);
      }
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
      setWorkspaces((prev) => prev.map((w) => (w.id === res.data.id ? res.data : w)));
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (err) {
      console.error("Failed to save:", err);
    } finally {
      setSaving(false);
    }
  }

  if (!workspace) return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;

  const onlyWorkspace = workspaces.length <= 1;

  return (
    <>
      <div className="settings-section">
        <div className="settings-section-header">Current Workspace</div>
        <div className="settings-section-body">
          <form onSubmit={handleSave}>
            <label className="dform-label">Workspace Name</label>
            <div style={{ display: "flex", gap: 10, marginBottom: 16 }}>
              <input
                className="dform-input"
                value={name}
                onChange={(e) => setName(e.target.value)}
                style={{ flex: 1 }}
              />
              <button
                type="submit"
                className="dbtn dbtn-primary"
                disabled={saving || !name.trim()}
              >
                {saving ? "Saving..." : saved ? "✓ Saved" : "Save"}
              </button>
            </div>
          </form>
          <div className="settings-row">
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>
              Workspace ID
            </span>
            <span className="mono">{workspace.id}</span>
          </div>
          <div className="settings-row">
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>Created</span>
            <span style={{ fontSize: 13, color: "var(--dtext)" }}>
              {new Date(workspace.created_at).toLocaleDateString("en-US", {
                month: "long",
                day: "numeric",
                year: "numeric",
              })}
            </span>
          </div>
        </div>
      </div>

      <div className="settings-section">
        <div className="settings-section-header">All Workspaces</div>
        <div className="settings-section-body">
          {workspaces.length === 0 ? (
            <div style={{ fontSize: 13, color: "var(--dmuted)" }}>No workspaces yet.</div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {workspaces.map((ws) => {
                const isCurrent = ws.id === workspace.id;
                return (
                  <div
                    key={ws.id}
                    style={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      padding: "10px 14px",
                      border: `1px solid ${isCurrent ? "var(--daccent)" : "var(--dborder)"}`,
                      borderRadius: 8,
                      background: isCurrent ? "rgba(16,185,129,0.05)" : "transparent",
                    }}
                  >
                    <div>
                      <div
                        style={{
                          fontSize: 13,
                          fontWeight: 600,
                          color: "var(--dtext)",
                        }}
                      >
                        {ws.name}
                        {isCurrent && (
                          <span
                            style={{
                              fontSize: 10,
                              color: "var(--daccent)",
                              marginLeft: 8,
                              fontWeight: 500,
                            }}
                          >
                            CURRENT
                          </span>
                        )}
                      </div>
                      <div
                        className="mono"
                        style={{
                          fontSize: 11,
                          color: "var(--dmuted)",
                          marginTop: 2,
                        }}
                      >
                        {ws.id}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      <div className="settings-section">
        <div className="settings-section-header">Multi-Workspace</div>
        <div className="settings-section-body">
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              marginBottom: 16,
            }}
          >
            <div>
              <div
                style={{
                  fontSize: 14,
                  fontWeight: 500,
                  marginBottom: 3,
                  color: "var(--dtext)",
                }}
              >
                Create New Workspace
              </div>
              <div style={{ fontSize: 13, color: "var(--dmuted)" }}>
                Separate security boundary with its own API keys, billing, and posts.
              </div>
            </div>
            <button className="dbtn" disabled style={{ opacity: 0.4, cursor: "not-allowed" }}>
              Coming Soon
            </button>
          </div>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
            }}
          >
            <div>
              <div
                style={{
                  fontSize: 14,
                  fontWeight: 500,
                  marginBottom: 3,
                  color: "var(--dtext)",
                }}
              >
                Switch Workspace
              </div>
              <div style={{ fontSize: 13, color: "var(--dmuted)" }}>
                Switch between your workspaces.
              </div>
            </div>
            <button className="dbtn" disabled style={{ opacity: 0.4, cursor: "not-allowed" }}>
              Coming Soon
            </button>
          </div>
        </div>
      </div>

      <div className="settings-section danger-section">
        <div className="settings-section-header">Danger Zone</div>
        <div className="settings-section-body">
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
            }}
          >
            <div>
              <div
                style={{
                  fontSize: 14,
                  fontWeight: 500,
                  marginBottom: 3,
                  color: "var(--dtext)",
                }}
              >
                Delete Workspace
              </div>
              <div style={{ fontSize: 13, color: "var(--dmuted)" }}>
                {onlyWorkspace
                  ? "Cannot delete your only workspace. Default workspaces (auto-created at signup) also cannot be deleted."
                  : "Deleting a workspace removes all its profiles, accounts, and posts."}
              </div>
            </div>
            <button
              className="dbtn dbtn-danger"
              disabled
              style={{ opacity: 0.4, cursor: "not-allowed" }}
            >
              Delete Workspace
            </button>
          </div>
        </div>
      </div>
    </>
  );
}
