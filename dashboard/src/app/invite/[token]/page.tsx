"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import { useAuth, SignInButton, SignUpButton } from "@clerk/nextjs";
import { getPublicInvite, acceptInvite, type PublicInvite } from "@/lib/api";

// /invite/[token] — public-facing invite accept page.
//
// Flow:
//   1. Page mounts → fetches /v1/public/invites/{token} (no auth)
//   2. If invite is invalid (404 / expired / revoked) → show "expired
//      or invalid" message
//   3. If invite is valid:
//      - If user not signed in: show workspace info + Sign in / Sign
//        up CTAs (Clerk routes back to this URL after auth)
//      - If user signed in: show Accept button → POSTs to
//        /v1/invites/{token}/accept → routes to dashboard

export default function InvitePage() {
  const params = useParams<{ token: string }>();
  const token = Array.isArray(params?.token) ? params.token[0] : params?.token;
  const { isSignedIn, isLoaded, getToken } = useAuth();
  const router = useRouter();

  const [invite, setInvite] = useState<PublicInvite | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [accepting, setAccepting] = useState(false);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    (async () => {
      try {
        const res = await getPublicInvite(token);
        if (!cancelled) setInvite(res.data);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "Invalid or expired invite");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [token]);

  const onAccept = useCallback(async () => {
    if (!token || accepting) return;
    setAccepting(true);
    setError(null);
    try {
      const clerkToken = await getToken();
      if (!clerkToken) throw new Error("Sign in required");
      await acceptInvite(clerkToken, token);
      router.push("/projects");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to accept invite");
      setAccepting(false);
    }
  }, [token, getToken, router, accepting]);

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        padding: 20,
        background: "var(--bg, #f8fafc)",
      }}
    >
      <div
        style={{
          maxWidth: 420,
          width: "100%",
          background: "#fff",
          border: "1px solid #e2e8f0",
          borderRadius: 14,
          padding: 32,
          boxShadow: "0 4px 12px rgba(0,0,0,0.04)",
        }}
      >
        <h1 style={{ fontSize: 24, fontWeight: 800, letterSpacing: -0.5, margin: "0 0 16px", color: "#0f172a" }}>
          UniPost workspace invite
        </h1>

        {loading && <p style={{ color: "#64748b" }}>Checking invite…</p>}

        {error && !invite && (
          <div
            style={{
              padding: 14,
              background: "#fef2f2",
              border: "1px solid #fecaca",
              borderRadius: 8,
              color: "#991b1b",
              fontSize: 13.5,
              lineHeight: 1.6,
            }}
          >
            <strong>Invite invalid or expired.</strong>
            <br />
            Ask the workspace admin to resend a fresh invite.
          </div>
        )}

        {invite && (
          <>
            <p style={{ color: "#475569", fontSize: 14, lineHeight: 1.65, margin: "0 0 18px" }}>
              You've been invited to join <strong>{invite.workspace_name || "a UniPost workspace"}</strong> as{" "}
              <strong>{invite.role}</strong>.
            </p>
            <p style={{ color: "#94a3b8", fontSize: 12.5, marginBottom: 24 }}>
              Sent to <span style={{ color: "#475569" }}>{invite.email}</span> · expires{" "}
              {new Date(invite.expires_at).toLocaleDateString()}
            </p>

            {!isLoaded ? (
              <p style={{ color: "#64748b" }}>Loading…</p>
            ) : isSignedIn ? (
              <button
                onClick={() => void onAccept()}
                disabled={accepting}
                style={{
                  width: "100%",
                  padding: "11px 18px",
                  background: "#10b981",
                  color: "#fff",
                  fontSize: 14,
                  fontWeight: 700,
                  border: "none",
                  borderRadius: 9,
                  cursor: accepting ? "wait" : "pointer",
                }}
              >
                {accepting ? "Accepting…" : "Accept invite"}
              </button>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                <SignInButton mode="modal" forceRedirectUrl={`/invite/${token}`}>
                  <button
                    style={{
                      width: "100%",
                      padding: "11px 18px",
                      background: "#0f172a",
                      color: "#fff",
                      fontSize: 14,
                      fontWeight: 700,
                      border: "none",
                      borderRadius: 9,
                      cursor: "pointer",
                    }}
                  >
                    Sign in to accept
                  </button>
                </SignInButton>
                <SignUpButton mode="modal" forceRedirectUrl={`/invite/${token}`}>
                  <button
                    style={{
                      width: "100%",
                      padding: "11px 18px",
                      background: "#fff",
                      color: "#0f172a",
                      fontSize: 14,
                      fontWeight: 600,
                      border: "1px solid #e2e8f0",
                      borderRadius: 9,
                      cursor: "pointer",
                    }}
                  >
                    Don&apos;t have an account? Sign up
                  </button>
                </SignUpButton>
              </div>
            )}

            {error && (
              <p style={{ color: "#dc2626", fontSize: 12.5, marginTop: 12 }}>{error}</p>
            )}
          </>
        )}
      </div>
    </div>
  );
}
