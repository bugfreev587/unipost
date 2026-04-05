"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { getProject, updateProject, deleteProject, type Project } from "@/lib/api";

export default function SettingsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);

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

  if (!project) return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Settings</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>Project configuration</div>
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
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>Mode</span>
            <span className="dbadge dbadge-green"><span className="dbadge-dot" />{project.mode}</span>
          </div>
          <div className="settings-row">
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>Project ID</span>
            <span className="mono">{project.id}</span>
          </div>
          <div className="settings-row">
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>Created</span>
            <span style={{ fontSize: 13, color: "var(--dtext)" }}>
              {new Date(project.created_at).toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" })}
            </span>
          </div>
        </div>
      </div>

      {/* Danger */}
      <div className="settings-section danger-section">
        <div className="settings-section-header">Danger Zone</div>
        <div className="settings-section-body">
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div>
              <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 3, color: "var(--dtext)" }}>Delete Project</div>
              <div style={{ fontSize: 13, color: "var(--dmuted)" }}>Permanently delete this project and all associated data.</div>
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
