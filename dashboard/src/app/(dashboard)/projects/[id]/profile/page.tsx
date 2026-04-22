"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import {
  listProfiles,
  getProfile,
  createProfile,
  deleteProfile,
  type Profile,
} from "@/lib/api";
import { Plus, Pencil, Trash2, Calendar, ArrowRight } from "lucide-react";
import { ConfirmModal } from "@/components/confirm-modal";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";

export default function ProfilePage() {
  const { id: currentProfileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const router = useRouter();
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);
  const [switchingProfileId, setSwitchingProfileId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Profile | null>(null);
  const [profileError, setProfileError] = useState<{ message: string; topic: string } | null>(null);

  const loadProfiles = useCallback(async () => {
    try {
      setProfileError(null);
      const token = await getToken();
      if (!token) return;
      const res = await listProfiles(token);
      setProfiles(res.data);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load profiles";
      console.error("Failed to load profiles:", err);
      setProfileError({ message, topic: "profiles-load-failure" });
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => {
    loadProfiles();
  }, [loadProfiles]);

  async function handleCreate() {
    if (!newName.trim()) return;
    setCreating(true);
    try {
      setProfileError(null);
      const token = await getToken();
      if (!token) return;
      await createProfile(token, { name: newName.trim() });
      setNewName("");
      setShowCreate(false);
      loadProfiles();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to create profile";
      console.error("Failed to create profile:", err);
      setProfileError({ message, topic: "profile-create-failure" });
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    try {
      setProfileError(null);
      const token = await getToken();
      if (!token) return;
      await deleteProfile(token, deleteTarget.id);
      setDeleteTarget(null);
      // If we deleted the current profile, navigate to the first remaining one
      if (deleteTarget.id === currentProfileId) {
        loadProfiles().then(() => {
          const remaining = profiles.filter((p) => p.id !== deleteTarget.id);
          if (remaining.length > 0) {
            router.push(`/projects/${remaining[0].id}/profile`);
          } else {
            router.push("/projects");
          }
        });
      } else {
        loadProfiles();
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to delete profile";
      console.error("Failed to delete profile:", err);
      setProfileError({ message, topic: "profile-delete-failure" });
    }
  }

  async function handleSwitchProfile(profile: Profile) {
    if (profile.id === currentProfileId || switchingProfileId) return;
    try {
      setProfileError(null);
      setSwitchingProfileId(profile.id);
      const token = await getToken();
      if (!token) return;
      await getProfile(token, profile.id);
      router.push(`/projects/${profile.id}/profile`);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to switch profile";
      console.error("Failed to switch profile:", err);
      setProfileError({ message, topic: "profile-switch-failure" });
    } finally {
      setSwitchingProfileId(null);
    }
  }

  if (loading) return <div style={{ padding: 32, color: "#888" }}>Loading...</div>;

  return (
    <div style={{ padding: 32, maxWidth: 800 }}>
      {profileError && (
        <div
          style={{
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            gap: 16,
            padding: "12px 14px",
            borderRadius: 8,
            background: "var(--danger-soft)",
            border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)",
            fontSize: 13,
            color: "var(--danger)",
            marginBottom: 20,
          }}
        >
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>Profile action failed</div>
            <div>{profileError.message}</div>
          </div>
          <div style={{ display: "flex", gap: 8, flexShrink: 0 }}>
            <a
              href={buildSupportMailto({
                subject: "Profile action failed in dashboard",
                intro: "I ran into a profile-related failure in the dashboard.",
                details: [
                  `Current profile ID: ${currentProfileId || "unknown"}`,
                  `Topic: ${profileError.topic}`,
                  `Error: ${profileError.message}`,
                ],
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Contact support
            </a>
            <a
              href={buildContactPageHref({
                topic: profileError.topic,
                source: "profiles-page",
                profile: currentProfileId,
                error: profileError.message,
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Open help center
            </a>
          </div>
        </div>
      )}

      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 700, color: "var(--dtext)", letterSpacing: -0.5 }}>
            Profiles
          </h1>
          <p style={{ fontSize: 14, color: "#888", marginTop: 6 }}>
            Manage your workspace profiles and their connected accounts.
          </p>
        </div>
        <button
          className="dbtn dbtn-primary"
          style={{ gap: 5 }}
          onClick={() => setShowCreate(true)}
        >
          <Plus style={{ width: 14, height: 14 }} /> New Profile
        </button>
      </div>

      {/* Create form */}
      {showCreate && (
        <div
          style={{
            border: "1px solid var(--dborder)",
            borderRadius: 8,
            padding: 16,
            marginBottom: 16,
            background: "var(--surface1)",
          }}
        >
          <div style={{ fontSize: 13, fontWeight: 600, color: "var(--dtext)", marginBottom: 8 }}>
            New Profile
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <input
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Profile name"
              autoFocus
              onKeyDown={(e) => e.key === "Enter" && handleCreate()}
              style={{
                flex: 1,
                background: "var(--surface2)",
                border: "1px solid var(--dborder)",
                borderRadius: 6,
                padding: "8px 12px",
                fontSize: 13,
                color: "var(--dtext)",
                outline: "none",
                fontFamily: "inherit",
              }}
            />
            <button
              className="dbtn dbtn-primary"
              disabled={creating || !newName.trim()}
              onClick={handleCreate}
              style={{ fontSize: 12 }}
            >
              {creating ? "Creating..." : "Create"}
            </button>
            <button
              className="dbtn dbtn-ghost"
              onClick={() => { setShowCreate(false); setNewName(""); }}
              style={{ fontSize: 12 }}
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Profile list */}
      {profiles.length === 0 ? (
        <div style={{ textAlign: "center", padding: 60, color: "#888" }}>
          <p style={{ fontSize: 15, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>
            No profiles yet
          </p>
          <p style={{ fontSize: 13 }}>
            Create your first profile to start connecting social accounts.
          </p>
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {profiles.map((profile) => {
            const isCurrent = profile.id === currentProfileId;
            return (
              <div
                key={profile.id}
                onClick={() => void handleSwitchProfile(profile)}
                style={{
                  border: `1px solid ${isCurrent ? "var(--daccent)" : "var(--dborder)"}`,
                  borderRadius: 8,
                  padding: "14px 16px",
                  display: "flex",
                  alignItems: "center",
                  gap: 12,
                  cursor: isCurrent ? "default" : "pointer",
                  transition: "all 0.1s",
                  background: isCurrent ? "rgba(16,185,129,0.05)" : "transparent",
                  opacity: switchingProfileId === profile.id ? 0.72 : 1,
                }}
                onMouseEnter={(e) => {
                  if (!isCurrent) e.currentTarget.style.borderColor = "#333";
                }}
                onMouseLeave={(e) => {
                  if (!isCurrent) e.currentTarget.style.borderColor = "var(--dborder)";
                }}
              >
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 600, color: "var(--dtext)" }}>
                    {profile.name}
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
                  <div style={{ fontSize: 12, color: "#888", marginTop: 2, display: "flex", alignItems: "center", gap: 4 }}>
                    <Calendar style={{ width: 10, height: 10 }} />
                    {switchingProfileId === profile.id
                      ? "Switching..."
                      : `Created ${new Date(profile.created_at).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })}`}
                  </div>
                </div>
                <div style={{ display: "flex", gap: 4 }}>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      router.push(`/projects/${profile.id}`);
                    }}
                    className="dbtn dbtn-ghost"
                    style={{ padding: 6 }}
                    title="Open profile"
                  >
                    <ArrowRight style={{ width: 13, height: 13 }} />
                  </button>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      router.push(`/projects/${profile.id}/settings`);
                    }}
                    className="dbtn dbtn-ghost"
                    style={{ padding: 6 }}
                    title="Edit"
                  >
                    <Pencil style={{ width: 13, height: 13 }} />
                  </button>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleteTarget(profile);
                    }}
                    className="dbtn dbtn-ghost"
                    style={{ padding: 6, color: "#888" }}
                    title="Delete"
                  >
                    <Trash2 style={{ width: 13, height: 13 }} />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Delete confirmation */}
      <ConfirmModal
        open={deleteTarget !== null}
        title="Delete Profile"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? This will disconnect all social accounts associated with this profile.`}
        confirmLabel="Delete"
        variant="danger"
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
