"use client";

import { useEffect, useState } from "react";
import { useAuth, useClerk, useUser } from "@clerk/nextjs";
import { useCurrentWorkspace } from "@/lib/use-current-workspace";
import { ConfirmModal } from "@/components/confirm-modal";
import { ExternalLink } from "lucide-react";
import { deleteMe, getMe, setOnboardingIntent, type OnboardingIntent } from "@/lib/api";
import { WelcomeModal } from "@/components/onboarding/welcome-modal";
import { track } from "@/lib/analytics";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";

const INTENT_LABELS: Record<Exclude<OnboardingIntent, "skipped">, string> = {
  exploring: "Just exploring",
  own_accounts: "Publishing to my own accounts",
  building_api: "Building with UniPost API",
};

// Phrase the user must type verbatim to enable the delete button.
// Mirrors Stripe / GitHub's dangerous-action confirmations.
const DELETE_CONFIRM_PHRASE = "delete my account";

const LANDING_URL = process.env.NEXT_PUBLIC_LANDING_URL || "https://unipost.dev";

// Heuristics for the default workspace name seeded on signup
// (see api/internal/handler/webhooks.go:131-133). If the workspace
// name still matches one of these, the user never supplied an
// organization at onboarding and we hide the Organization row.
function isDefaultWorkspaceName(
  workspaceName: string,
  userName: string | null | undefined,
): boolean {
  if (workspaceName === "Default Workspace") return true;
  if (userName && workspaceName === `${userName}'s Workspace`) return true;
  return false;
}

export default function AccountSettingsPage() {
  const { user, isLoaded } = useUser();
  const { signOut, openUserProfile } = useClerk();
  const { workspace, loading: workspaceLoading } = useCurrentWorkspace();

  const { getToken } = useAuth();
  const [showDelete, setShowDelete] = useState(false);
  const [confirmText, setConfirmText] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [intent, setIntent] = useState<OnboardingIntent | null>(null);
  const [intentModalOpen, setIntentModalOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getMe(token);
        if (!cancelled) setIntent(res.data.onboarding_intent ?? null);
      } catch { /* silent */ }
    })();
    return () => { cancelled = true; };
  }, [getToken]);

  async function handleIntentChange(newIntent: Exclude<OnboardingIntent, "skipped">) {
    setIntentModalOpen(false);
    setIntent(newIntent);
    track("onboarding_intent_changed", { intent: newIntent });
    try {
      const token = await getToken();
      if (token) await setOnboardingIntent(token, newIntent);
    } catch { /* non-blocking */ }
  }

  if (!isLoaded || workspaceLoading) {
    return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;
  }

  if (!user) {
    return <div style={{ color: "var(--dmuted)" }}>Not signed in.</div>;
  }

  const displayName =
    [user.firstName, user.lastName].filter(Boolean).join(" ") ||
    user.username ||
    "—";
  const primaryEmail = user.primaryEmailAddress?.emailAddress ?? "—";
  const orgName =
    workspace && !isDefaultWorkspaceName(workspace.name, user.firstName ?? user.username)
      ? workspace.name
      : null;

  async function handleDelete() {
    if (confirmText !== DELETE_CONFIRM_PHRASE || !user) return;
    setDeleting(true);
    setDeleteError("");
    try {
      // Delete via our backend, which uses Clerk's server-side SDK
      // (CLERK_SECRET_KEY) to bypass the client-side "reauthentication
      // required" check. Clerk fires a user.deleted webhook that our
      // api/internal/handler/webhooks.go handles, cascading DB cleanup
      // through workspaces/profiles/social_accounts/api_keys/posts
      // via ON DELETE CASCADE foreign keys (migration 025).
      const token = await getToken();
      if (!token) {
        setDeleteError("Session expired. Please sign in again.");
        setDeleting(false);
        return;
      }
      await deleteMe(token);
      // Clerk has deleted the user on their side. Clean up our local
      // session and bounce to the marketing landing page.
      try {
        await signOut();
      } catch {
        // ignore — user is already gone
      }
      window.location.href = LANDING_URL;
    } catch (err) {
      setDeleteError((err as Error).message || "Failed to delete account");
      setDeleting(false);
    }
  }

  return (
    <>
      <div className="settings-section">
        <div className="settings-section-header">Profile</div>
        <div className="settings-section-body">
          <div className="settings-row">
            <span className="dt-label-plain">Name</span>
            <span className="dt-body-sm" style={{ color: "var(--dtext)" }}>{displayName}</span>
          </div>
          <div className="settings-row">
            <span className="dt-label-plain">Email</span>
            <span className="dt-body-sm" style={{ color: "var(--dtext)" }}>{primaryEmail}</span>
          </div>
          {orgName && (
            <div className="settings-row">
              <span className="dt-label-plain">
                Organization
              </span>
              <span className="dt-body-sm" style={{ color: "var(--dtext)" }}>{orgName}</span>
            </div>
          )}
          <div className="settings-row">
            <span className="dt-label-plain">User ID</span>
            <span className="mono">{user.id}</span>
          </div>
          <div className="settings-row">
            <span className="dt-label-plain">Primary use case</span>
            <span style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span className="dt-body-sm" style={{ color: "var(--dtext)" }}>
                {intent && intent !== "skipped" ? INTENT_LABELS[intent] : "Not set"}
              </span>
              <button
                type="button"
                onClick={() => setIntentModalOpen(true)}
                className="dt-body-sm"
                style={{
                  padding: "4px 10px",
                  borderRadius: 6,
                  border: "1px solid var(--dborder)",
                  background: "transparent",
                  color: "var(--dtext)",
                  cursor: "pointer",
                  fontFamily: "inherit",
                }}
              >
                Change
              </button>
            </span>
          </div>
          <div style={{ marginTop: 16, paddingTop: 16, borderTop: "1px solid var(--dborder)" }}>
            <button
              className="dbtn dbtn-ghost"
              onClick={() => openUserProfile()}
              style={{ fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6 }}
            >
              Manage email, password, and 2FA
              <ExternalLink style={{ width: 11, height: 11 }} />
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
              gap: 16,
            }}
          >
            <div>
              <div className="dt-body" style={{ fontWeight: 500, marginBottom: 3 }}>
                Delete Account
              </div>
              <div className="dt-body-sm">
                Permanently delete your account and all associated data. This cannot be undone.
              </div>
            </div>
            <button
              className="dbtn dbtn-danger"
              onClick={() => {
                setConfirmText("");
                setDeleteError("");
                setShowDelete(true);
              }}
            >
              Delete Account
            </button>
          </div>
        </div>
      </div>

      <ConfirmModal
        open={showDelete}
        title="Delete your account?"
        variant="danger"
        wide
        confirmLabel={deleting ? "Deleting..." : "Delete my account"}
        confirmDisabled={confirmText !== DELETE_CONFIRM_PHRASE || deleting}
        onCancel={() => {
          if (!deleting) setShowDelete(false);
        }}
        onConfirm={handleDelete}
        message={
          <div>
            <div
              style={{
                fontSize: 13,
                color: "var(--dtext)",
                lineHeight: 1.6,
                marginBottom: 12,
              }}
            >
              This action is <strong>irreversible</strong>. Deleting your account will
              permanently remove:
            </div>
            <ul
              style={{
                fontSize: 13,
                color: "var(--dmuted)",
                lineHeight: 1.6,
                marginBottom: 16,
                paddingLeft: 20,
              }}
            >
              <li>Your user profile and Clerk session</li>
              <li>All workspaces, profiles, and their settings</li>
              <li>All connected social accounts (you&apos;ll need to reconnect on reinstall)</li>
              <li>All posts, analytics, API keys, and webhooks</li>
              <li>Any managed users connected through your Connect integration</li>
            </ul>
            <div
              style={{
                fontSize: 13,
                color: "var(--dtext)",
                marginBottom: 8,
              }}
            >
              Type{" "}
              <code
                style={{
                  background: "var(--surface2)",
                  padding: "1px 6px",
                  borderRadius: 4,
                  fontFamily: "var(--font-geist-mono), monospace",
                  fontSize: 12,
                  color: "var(--danger)",
                }}
              >
                {DELETE_CONFIRM_PHRASE}
              </code>{" "}
              to confirm:
            </div>
            <input
              className="dform-input"
              type="text"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder={DELETE_CONFIRM_PHRASE}
              autoFocus
              disabled={deleting}
              style={{ width: "100%" }}
            />
            {deleteError && (
              <div
                style={{
                  marginTop: 10,
                  padding: "8px 12px",
                  borderRadius: 6,
                  background: "#ef444410",
                  border: "1px solid #ef444425",
                  fontSize: 13,
                  color: "var(--danger)",
                }}
              >
                <div style={{ marginBottom: 8 }}>{deleteError}</div>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <a
                    href={buildSupportMailto({
                      subject: "Delete account failed in dashboard",
                      intro: "I tried to delete my account and the action failed.",
                      details: [
                        `User ID: ${user.id}`,
                        orgName ? `Organization: ${orgName}` : undefined,
                        `Error: ${deleteError}`,
                      ],
                    })}
                    style={{ color: "var(--danger)", fontWeight: 600, textDecoration: "underline" }}
                  >
                    Contact support
                  </a>
                  <a
                    href={buildContactPageHref({
                      topic: "delete-account-failure",
                      source: "account-settings",
                      error: deleteError,
                    })}
                    style={{ color: "var(--danger)", fontWeight: 600, textDecoration: "underline" }}
                  >
                    Open help center
                  </a>
                </div>
              </div>
            )}
          </div>
        }
      />

      {/* Intent edit modal — reuses the Welcome modal component. */}
      <WelcomeModal
        open={intentModalOpen}
        initialIntent={intent || undefined}
        onSelect={handleIntentChange}
        onSkip={() => setIntentModalOpen(false)}
      />
    </>
  );
}
