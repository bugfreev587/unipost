"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { completeOnboarding } from "@/lib/api";

// Welcome onboarding page for new signups.
//
// Collects:
//   - First Name (required) — used for the user profile and the
//     default workspace name ("{FirstName}'s Workspace") when no
//     organization name is provided.
//   - Organization Name (optional) — when provided, becomes the
//     workspace name instead of "{FirstName}'s Workspace".
//
// Backed by PATCH /v1/me/onboarding, which renames the workspace seeded
// at signup (user.created webhook) and stores the user's first name.
//
// After submit, lands the user on the dashboard root (/).
export default function WelcomePage() {
  const router = useRouter();
  const { getToken } = useAuth();
  const [firstName, setFirstName] = useState("");
  const [orgName, setOrgName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  async function handleContinue(e: React.FormEvent) {
    e.preventDefault();
    const trimmedFirst = firstName.trim();
    if (!trimmedFirst) return;

    setSubmitting(true);
    setError("");
    try {
      const token = await getToken();
      if (!token) {
        setError("Session expired. Please sign in again.");
        setSubmitting(false);
        return;
      }
      await completeOnboarding(token, {
        first_name: trimmedFirst,
        org_name: orgName.trim() || undefined,
        // usage_modes is collected later via the in-dashboard WelcomeModal.
        usage_modes: [],
      });
      router.push("/");
    } catch (err) {
      setError((err as Error).message || "Failed to complete setup. Please try again.");
      setSubmitting(false);
    }
  }

  return (
    <div style={{ width: "100%", maxWidth: 480, padding: "40px 24px", textAlign: "center" }}>
      <div style={{
        width: 56, height: 56, borderRadius: 14,
        background: "linear-gradient(135deg, var(--daccent, #10b981), var(--primary-hover, #059669))",
        display: "flex", alignItems: "center", justifyContent: "center",
        margin: "0 auto 24px", fontSize: 24, fontWeight: 700,
        color: "var(--primary-foreground, #fff)",
      }}>
        U
      </div>

      <h1 style={{ fontSize: 32, fontWeight: 700, color: "var(--dtext)", letterSpacing: -0.5, marginBottom: 8 }}>
        Welcome to UniPost
      </h1>
      <p style={{ fontSize: 15, color: "var(--dmuted)", lineHeight: 1.6, marginBottom: 40 }}>
        Publish to every social platform in one click.<br />
        Let&apos;s get you set up.
      </p>

      <form onSubmit={handleContinue} style={{ textAlign: "left" }}>
        <label style={{ display: "block", fontSize: 12, fontWeight: 600, color: "var(--dmuted)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6 }}>
          First Name <span style={{ color: "var(--danger, #ef4444)" }}>*</span>
        </label>
        <input
          className="dform-input"
          value={firstName}
          onChange={(e) => setFirstName(e.target.value)}
          placeholder="Your first name"
          autoFocus
          disabled={submitting}
          maxLength={80}
          style={{ width: "100%", marginBottom: 16 }}
        />

        <label style={{ display: "block", fontSize: 12, fontWeight: 600, color: "var(--dmuted)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6 }}>
          Organization Name <span style={{ color: "var(--dmuted2)", fontSize: 10, fontWeight: 400 }}>optional</span>
        </label>
        <input
          className="dform-input"
          value={orgName}
          onChange={(e) => setOrgName(e.target.value)}
          placeholder="Your company or team (leave blank for a personal workspace)"
          disabled={submitting}
          maxLength={120}
          style={{ width: "100%", marginBottom: 8 }}
        />
        <div style={{ fontSize: 12, color: "var(--dmuted2)", marginBottom: 28 }}>
          {orgName.trim()
            ? <>Your workspace will be named <strong>{orgName.trim()}</strong>.</>
            : firstName.trim()
              ? <>Your workspace will be named <strong>{firstName.trim()}&apos;s Workspace</strong>.</>
              : <>Your workspace name will be based on the values above.</>}
        </div>

        {error && (
          <div style={{
            padding: "8px 12px", borderRadius: 6,
            background: "rgba(239, 68, 68, 0.1)", border: "1px solid rgba(239, 68, 68, 0.25)",
            fontSize: 13, color: "var(--danger)", marginBottom: 16,
          }}>
            {error}
          </div>
        )}

        <button
          type="submit"
          className="dbtn dbtn-primary"
          disabled={!firstName.trim() || submitting}
          style={{ width: "100%", padding: "12px 0", fontSize: 14, fontWeight: 600 }}
        >
          {submitting ? "Setting up..." : "Continue"}
        </button>
      </form>
    </div>
  );
}
