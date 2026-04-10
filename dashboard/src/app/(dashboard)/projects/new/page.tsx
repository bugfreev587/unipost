"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { createProfile } from "@/lib/api";
import Link from "next/link";

export default function NewProfilePage() {
  const { getToken } = useAuth();
  const router = useRouter();
  const [name, setName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setLoading(true); setError("");
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createProfile(token, { name: name.trim() });
      router.push(`/projects/${res.data.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create profile");
    } finally { setLoading(false); }
  }

  return (
    <div style={{ maxWidth: 440, margin: "0 auto", paddingTop: 32 }}>
      <Link href="/projects" style={{ fontSize: 12.5, color: "var(--dmuted)", textDecoration: "none", display: "inline-flex", alignItems: "center", gap: 4, marginBottom: 20 }}>
        ← Back to Profiles
      </Link>

      <div className="settings-section">
        <div className="settings-section-header">New Profile</div>
        <div className="settings-section-body">
          <div style={{ fontSize: 12.5, color: "var(--dmuted)", marginBottom: 16, lineHeight: 1.6 }}>
            Each profile has isolated API keys, accounts, and billing.
          </div>
          <form onSubmit={handleSubmit}>
            <div style={{ marginBottom: 16 }}>
              <label className="dform-label">Profile Name</label>
              <input
                className="dform-input"
                placeholder="My App"
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoFocus
                required
              />
            </div>
            {error && <div style={{ fontSize: 12, color: "var(--danger)", marginBottom: 12 }}>{error}</div>}
            <div style={{ display: "flex", gap: 8 }}>
              <button type="button" className="dbtn dbtn-ghost" onClick={() => router.back()}>Cancel</button>
              <button type="submit" className="dbtn dbtn-primary" disabled={loading || !name.trim()}>
                {loading ? "Creating..." : "Create Profile"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
