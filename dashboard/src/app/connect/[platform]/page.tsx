// Hosted Connect page (Sprint 3 PR6).
//
// Mounts at app.unipost.dev/connect/<platform>?session=<id>&state=<oauth_state>.
// The customer creates a session via POST /v1/connect/sessions and emails
// the URL to their end user; the user lands here.
//
// SERVER COMPONENT — no "use client". This is intentional:
//
//   1. The Bluesky form is a NATIVE cross-origin HTML form whose
//      action targets api.unipost.dev. The app password travels in
//      a normal application/x-www-form-urlencoded POST body and never
//      lives in dashboard JS where DevTools could inspect it. Per
//      Sprint 3 founder decision #6.
//
//   2. The Twitter / LinkedIn variants are a single anchor / button
//      that GETs /v1/public/connect/sessions/{id}/authorize on the
//      API origin. Server-rendered links don't need React state.
//
// This page is unauthenticated — the oauth_state in the URL is the
// bearer for the public session lookup endpoint.

import Link from "next/link";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.unipost.dev";

type SessionStatus = "pending" | "completed" | "expired" | "cancelled";

type PublicConnectSession = {
  platform: "twitter" | "linkedin" | "bluesky";
  project_name: string;
  status: SessionStatus;
  return_url?: string;
  expires_at: string;
};

type ApiEnvelope<T> = { data?: T; error?: { code: string; message: string } };

const PLATFORM_LABEL: Record<string, string> = {
  twitter: "Twitter / X",
  linkedin: "LinkedIn",
  bluesky: "Bluesky",
};

// Server-side fetch of the public session view. Forwards the
// oauth_state query param as the bearer. Cache: never (the lookup is
// state-bearer-protected, not safe to cache).
async function loadSession(
  sessionID: string,
  state: string,
): Promise<{ session?: PublicConnectSession; error?: string }> {
  if (!sessionID || !state) {
    return { error: "Connect link is missing required parameters." };
  }
  try {
    const res = await fetch(
      `${API_URL}/v1/public/connect/sessions/${encodeURIComponent(
        sessionID,
      )}?state=${encodeURIComponent(state)}`,
      { cache: "no-store" },
    );
    if (!res.ok) {
      return { error: "This Connect link is invalid or has expired." };
    }
    const body: ApiEnvelope<PublicConnectSession> = await res.json();
    if (!body.data) {
      return { error: "This Connect link is invalid or has expired." };
    }
    return { session: body.data };
  } catch {
    return { error: "Couldn't reach the UniPost API. Please try again." };
  }
}

type PageProps = {
  params: Promise<{ platform: string }>;
  searchParams: Promise<{ session?: string; state?: string; connect_status?: string; reason?: string }>;
};

export default async function ConnectPage({ params, searchParams }: PageProps) {
  const { platform } = await params;
  const { session: sessionID = "", state = "", connect_status, reason } = await searchParams;

  // Show success / cancellation pages BEFORE looking up the session —
  // the OAuth callback redirects back here with ?connect_status=...
  // when no return_url is set.
  if (connect_status === "success") {
    return <SuccessPage />;
  }
  if (connect_status === "cancelled") {
    return <ErrorPage title="Connection cancelled" body="You can close this window." />;
  }
  if (connect_status === "error") {
    return (
      <ErrorPage
        title="Connection failed"
        body={`Something went wrong: ${reason || "unknown error"}. Please try the link again.`}
      />
    );
  }

  const { session, error } = await loadSession(sessionID, state);
  if (error || !session) {
    return <ErrorPage title="Link unavailable" body={error || "Unknown error."} />;
  }
  if (session.platform !== platform) {
    return (
      <ErrorPage
        title="Wrong link"
        body="This Connect link is for a different platform. Please contact the developer who sent it."
      />
    );
  }
  if (session.status === "expired") {
    return <ErrorPage title="Link expired" body="This Connect link has expired (30-minute window)." />;
  }
  if (session.status === "completed") {
    return <SuccessPage />;
  }
  if (session.status === "cancelled") {
    return <ErrorPage title="Cancelled" body="This Connect link was cancelled." />;
  }

  // status === "pending" — render the platform-specific UI.
  if (platform === "bluesky") {
    return <BlueskyForm session={session} sessionID={sessionID} state={state} />;
  }
  return <OAuthPrompt session={session} sessionID={sessionID} state={state} platform={platform} />;
}

// ── Layout primitives ─────────────────────────────────────────────

const STYLES = `
  body{font-family:system-ui,-apple-system,sans-serif;background:#fafafa;color:#111;margin:0}
  .wrap{max-width:480px;margin:48px auto;padding:0 24px;line-height:1.5}
  h1{font-size:24px;margin-bottom:8px;letter-spacing:-0.3px}
  p{color:#444}
  .panel{background:#fff;border:1px solid #e5e5e5;border-radius:12px;padding:24px;margin-top:16px}
  .err{background:#fef2f2;border:1px solid #fecaca;color:#991b1b;padding:12px 16px;border-radius:8px;margin:16px 0;font-size:14px}
  .ok{background:#f0fdf4;border:1px solid #bbf7d0;color:#166534;padding:12px 16px;border-radius:8px;margin:16px 0}
  .btn{display:block;width:100%;background:#111;color:#fff;border:0;padding:14px;border-radius:8px;font-size:15px;text-align:center;text-decoration:none;cursor:pointer;font-weight:500}
  .btn:hover{background:#000}
  label{display:block;margin-top:16px;font-size:14px;font-weight:500}
  input{width:100%;padding:10px 12px;border:1px solid #d1d5db;border-radius:6px;font-size:15px;margin-top:4px;box-sizing:border-box}
  input:focus{outline:none;border-color:#111}
  .footer{font-size:12px;color:#888;margin-top:32px;text-align:center}
  .small{font-size:13px;color:#666}
  .warn{background:#fffbeb;border:1px solid #fde68a;color:#92400e;padding:12px 16px;border-radius:8px;margin-top:16px;font-size:13px}
  .warn strong{font-weight:600}
  .check{font-size:48px;color:#166534;text-align:center}
`;

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: STYLES }} />
      <div className="wrap">
        {children}
        <div className="footer">
          Powered by{" "}
          <Link href="/" style={{ color: "#666", textDecoration: "none" }}>
            UniPost
          </Link>
        </div>
      </div>
    </>
  );
}

// ── Variants ──────────────────────────────────────────────────────

function ErrorPage({ title, body }: { title: string; body: string }) {
  return (
    <Layout>
      <h1>{title}</h1>
      <div className="panel">
        <div className="err">{body}</div>
        <p className="small">If you reached this page by mistake, contact the developer who sent you the link.</p>
      </div>
    </Layout>
  );
}

function SuccessPage() {
  return (
    <Layout>
      <div className="panel" style={{ textAlign: "center" }}>
        <div className="check">✓</div>
        <h1 style={{ marginTop: 8 }}>Connected!</h1>
        <p>You can close this window now.</p>
      </div>
    </Layout>
  );
}

function OAuthPrompt({
  session,
  sessionID,
  state,
  platform,
}: {
  session: PublicConnectSession;
  sessionID: string;
  state: string;
  platform: string;
}) {
  const label = PLATFORM_LABEL[platform] || platform;
  // Browser GETs the API authorize endpoint, which 302s to the
  // platform's authorize URL after computing the PKCE challenge.
  const authorizeHref = `${API_URL}/v1/public/connect/sessions/${encodeURIComponent(
    sessionID,
  )}/authorize?state=${encodeURIComponent(state)}`;
  return (
    <Layout>
      <h1>Connect {label}</h1>
      <p>
        <strong>{session.project_name}</strong> wants to publish posts to your {label} account on your behalf.
      </p>
      <div className="panel">
        <a className="btn" href={authorizeHref}>
          Authorize {label}
        </a>
        <p className="small" style={{ marginTop: 16 }}>
          You&apos;ll be redirected to {label} to sign in. UniPost never sees your password.
        </p>
      </div>
    </Layout>
  );
}

function BlueskyForm({
  session,
  sessionID,
  state,
}: {
  session: PublicConnectSession;
  sessionID: string;
  state: string;
}) {
  // Native cross-origin form POST. The action targets api.unipost.dev
  // directly so the password is sent as form-urlencoded by the browser
  // and never touches dashboard JS. The API returns either a 302
  // redirect (success) or a server-rendered HTML page with inline
  // errors (failure) — both render correctly in the browser without
  // any client-side JS on this page.
  const action = `${API_URL}/v1/public/connect/sessions/${encodeURIComponent(
    sessionID,
  )}/bluesky?state=${encodeURIComponent(state)}`;
  return (
    <Layout>
      <h1>Connect Bluesky</h1>
      <p>
        <strong>{session.project_name}</strong> wants to publish posts to your Bluesky account on your behalf.
      </p>
      <div className="panel">
        <form method="POST" action={action}>
          <label>
            Handle
            <input
              name="handle"
              type="text"
              placeholder="you.bsky.social"
              autoCapitalize="off"
              autoCorrect="off"
              required
            />
          </label>
          <label>
            App password
            <input name="app_password" type="password" placeholder="xxxx-xxxx-xxxx-xxxx" required />
          </label>
          <button className="btn" type="submit" style={{ marginTop: 20 }}>
            Connect Bluesky
          </button>
        </form>
        <div className="warn">
          <strong>Important:</strong> use an app password from{" "}
          <a href="https://bsky.app/settings/app-passwords" target="_blank" rel="noopener noreferrer">
            bsky.app/settings/app-passwords
          </a>
          , <strong>not</strong> your main Bluesky password. App passwords can be revoked individually
          and only grant the permissions you choose.
        </div>
      </div>
    </Layout>
  );
}

// Tell Next.js this page is dynamic — every request must hit the
// server, since the URL params drive the response and there's no
// safe way to cache a state-bearer-protected page.
export const dynamic = "force-dynamic";
export const fetchCache = "force-no-store";
