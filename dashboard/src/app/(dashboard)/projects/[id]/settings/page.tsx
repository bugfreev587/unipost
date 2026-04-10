"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { getProfile, updateProfile, deleteProfile, type Profile } from "@/lib/api";
import { ConfirmModal } from "@/components/confirm-modal";

export default function SettingsPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const router = useRouter();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getProfile(token, profileId);
        setProfile(res.data); setName(res.data.name);
      } catch (err) { console.error("Failed:", err); }
    }
    load();
  }, [getToken, profileId]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setSaving(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await updateProfile(token, profileId, { name: name.trim() });
      setProfile(res.data);
    } catch (err) { console.error("Failed:", err); } finally { setSaving(false); }
  }

  async function handleDelete() {
    setDeleting(true);
    try {
      const token = await getToken();
      if (!token) return;
      await deleteProfile(token, profileId);
      router.push("/");
    } catch (err) { console.error("Failed:", err); } finally { setDeleting(false); setShowDeleteConfirm(false); }
  }

  if (!profile) return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Settings</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>Profile configuration</div>
        </div>
      </div>

      {/* General */}
      <div className="settings-section">
        <div className="settings-section-header">General</div>
        <div className="settings-section-body">
          <form onSubmit={handleSave}>
            <label className="dform-label">Profile Name</label>
            <div style={{ display: "flex", gap: 10, marginBottom: 16 }}>
              <input className="dform-input" value={name} onChange={(e) => setName(e.target.value)} style={{ flex: 1 }} />
              <button type="submit" className="dbtn dbtn-primary" disabled={saving || !name.trim()}>
                {saving ? "Saving..." : "Save"}
              </button>
            </div>
          </form>
          <div className="settings-row">
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>Profile ID</span>
            <span className="mono">{profile.id}</span>
          </div>
          <div className="settings-row">
            <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dmuted)" }}>Created</span>
            <span style={{ fontSize: 13, color: "var(--dtext)" }}>
              {new Date(profile.created_at).toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" })}
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
              <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 3, color: "var(--dtext)" }}>Delete Profile</div>
              <div style={{ fontSize: 13, color: "var(--dmuted)" }}>Permanently delete this profile and all associated data.</div>
            </div>
            <button className="dbtn dbtn-danger" onClick={() => setShowDeleteConfirm(true)} disabled={deleting}>
              {deleting ? "Deleting..." : "Delete Profile"}
            </button>
          </div>
        </div>
      </div>

      <ConfirmModal
        open={showDeleteConfirm}
        title="Delete Profile"
        message="Are you sure you want to delete this profile? All API keys, connected accounts, and associated data will be permanently removed. This action cannot be undone."
        confirmLabel="Delete Profile"
        variant="danger"
        onConfirm={handleDelete}
        onCancel={() => setShowDeleteConfirm(false)}
      />
    </>
  );
}
