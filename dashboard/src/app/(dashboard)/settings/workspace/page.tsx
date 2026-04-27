"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { getWorkspace, updateWorkspace, type Workspace } from "@/lib/api";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";

export default function WorkspaceSettingsTab() {
  const { getToken } = useAuth();
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [workspaceError, setWorkspaceError] = useState<{ message: string; topic: string } | null>(null);

  useEffect(() => {
    async function load() {
      try {
        setWorkspaceError(null);
        const token = await getToken();
        if (!token) return;
        const res = await getWorkspace(token);
        setWorkspace(res.data);
        setName(res.data.name);
      } catch (err) {
        const message = err instanceof Error ? err.message : "Failed to load workspace";
        console.error("Failed to load workspace:", err);
        setWorkspaceError({ message, topic: "workspace-load-failure" });
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
      setWorkspaceError(null);
      const token = await getToken();
      if (!token) return;
      const res = await updateWorkspace(token, { name: name.trim() });
      setWorkspace(res.data);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to save workspace";
      console.error("Failed to save:", err);
      setWorkspaceError({ message, topic: "workspace-save-failure" });
    } finally {
      setSaving(false);
    }
  }

  if (!workspace) {
    if (workspaceError) {
      return (
        <div
          style={{
            padding: "12px 14px",
            borderRadius: 8,
            background: "#ef444410",
            border: "1px solid #ef444425",
            fontSize: 13,
            color: "var(--danger)",
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 4 }}>Workspace action failed</div>
          <div style={{ marginBottom: 10 }}>{workspaceError.message}</div>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <a
              href={buildSupportMailto({
                subject: "Workspace action failed in dashboard",
                intro: "I ran into a workspace-related failure in the dashboard.",
                details: [
                  `Topic: ${workspaceError.topic}`,
                  `Error: ${workspaceError.message}`,
                ],
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Contact support
            </a>
            <a
              href={buildContactPageHref({
                topic: workspaceError.topic,
                source: "workspace-settings",
                error: workspaceError.message,
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Open help center
            </a>
          </div>
        </div>
      );
    }
    return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;
  }

  return (
    <>
      {workspaceError && (
        <div
          style={{
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            gap: 16,
            padding: "12px 14px",
            borderRadius: 8,
            background: "#ef444410",
            border: "1px solid #ef444425",
            fontSize: 13,
            color: "var(--danger)",
            marginBottom: 20,
          }}
        >
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>Workspace action failed</div>
            <div>{workspaceError.message}</div>
          </div>
          <div style={{ display: "flex", gap: 8, flexShrink: 0 }}>
            <a
              href={buildSupportMailto({
                subject: "Workspace action failed in dashboard",
                intro: "I ran into a workspace-related failure in the dashboard.",
                details: [
                  workspace ? `Workspace ID: ${workspace.id}` : undefined,
                  `Topic: ${workspaceError.topic}`,
                  `Error: ${workspaceError.message}`,
                ],
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Contact support
            </a>
            <a
              href={buildContactPageHref({
                topic: workspaceError.topic,
                source: "workspace-settings",
                workspace: workspace?.id,
                error: workspaceError.message,
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Open help center
            </a>
          </div>
        </div>
      )}

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
            <span className="dt-label-plain">
              Workspace ID
            </span>
            <span className="mono">{workspace.id}</span>
          </div>
          <div className="settings-row">
            <span className="dt-label-plain">Created</span>
            <span className="dt-body-sm" style={{ color: "var(--dtext)" }}>
              {new Date(workspace.created_at).toLocaleDateString("en-US", {
                month: "long",
                day: "numeric",
                year: "numeric",
              })}
            </span>
          </div>
        </div>
      </div>

    </>
  );
}
