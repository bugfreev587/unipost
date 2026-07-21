import type { Metadata } from "next";
import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

export const metadata: Metadata = {
  title: "Integrate UniPost Inbox into your app",
  description: "Connect app users to UniPost Inbox with managed-user isolation, server-side API keys, scoped real-time events, and a separate owner/admin aggregate.",
  alternates: { canonical: "https://unipost.dev/docs/guides/inbox-integration" },
};

const CONNECT_SESSION = `type AuthenticatedAppUser = {
  id: "app_usr_7f4c91";
  role: "user" | "owner" | "admin";
};

export async function createInboxConnectSession(appUser: AuthenticatedAppUser) {
  const apiKey = process.env.UNIPOST_API_KEY;
  if (!apiKey) throw new Error("UNIPOST_API_KEY is not configured");

  const response = await fetch("https://api.unipost.dev/v1/connect/sessions", {
    method: "POST",
    headers: {
      Authorization: \`Bearer \${apiKey}\`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      platform: "instagram",
      profile_id: "pr_app_inbox",
      external_user_id: appUser.id,
      return_url: "https://app.example.com/integrations/complete",
    }),
  });

  if (!response.ok) {
    throw new Error(\`Connect Session failed with status \${response.status}\`);
  }

  return response.json();
}`;

const SCOPED_HELPER = `type AuthenticatedAppUser = {
  id: string;
  role: "user" | "owner" | "admin";
};

type InboxPath = \`/v1/inbox\${string}\`;

export async function callManagedInbox(
  appUser: AuthenticatedAppUser,
  path: InboxPath,
  init: RequestInit = {},
) {
  const apiKey = process.env.UNIPOST_API_KEY;
  if (!apiKey) throw new Error("UNIPOST_API_KEY is not configured");
  if (!path.startsWith("/v1/inbox")) throw new Error("Invalid Inbox path");

  const url = new URL(path, "https://api.unipost.dev");
  url.searchParams.set("inbox_scope", "managed_user");
  url.searchParams.set("external_user_id", appUser.id);

  return fetch(url, {
    ...init,
    headers: {
      ...init.headers,
      Authorization: \`Bearer \${apiKey}\`,
      "Content-Type": "application/json",
    },
  });
}`;

const READS = `// appUser came from your verified application session.
const items = await callManagedInbox(appUser, "/v1/inbox?limit=50");
const unread = await callManagedInbox(appUser, "/v1/inbox/unread-count");
const item = await callManagedInbox(appUser, "/v1/inbox/inbox_item_7f4c91");

if (item.status === 404) {
  // Missing and cross-user items deliberately look the same.
  return { status: 404, error: "Inbox item unavailable" };
}`;

const WRITES = `await callManagedInbox(appUser, "/v1/inbox/inbox_item_7f4c91/read", {
  method: "POST",
});

await callManagedInbox(appUser, "/v1/inbox/mark-all-read", {
  method: "POST",
});

await callManagedInbox(appUser, "/v1/inbox/inbox_item_7f4c91/reply", {
  method: "POST",
  headers: { "Idempotency-Key": "reply-inbox-item-7f4c91-v1" },
  body: JSON.stringify({ text: "Thanks. We will follow up here." }),
});

await callManagedInbox(appUser, "/v1/inbox/inbox_item_7f4c91/thread-state", {
  method: "POST",
  body: JSON.stringify({ thread_status: "assigned", assigned_to: "support_27" }),
});`;

const ORDINARY_SYNC = `curl -X POST "https://api.unipost.dev/v1/inbox/sync?inbox_scope=managed_user&external_user_id=app_usr_7f4c91" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`;

const X_BACKFILL = `curl -X POST "https://api.unipost.dev/v1/inbox/sync?inbox_scope=managed_user&external_user_id=app_usr_7f4c91" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"x_backfill":{"account_id":"sa_x_7f4c91","lookback_days":7,"max_items":50,"include_replies":true,"include_dms":false}}'

# If confirmation_required is true, repeat the exact request before expiry.
curl -X POST "https://api.unipost.dev/v1/inbox/sync?inbox_scope=managed_user&external_user_id=app_usr_7f4c91" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"x_backfill":{"account_id":"sa_x_7f4c91","lookback_days":7,"max_items":50,"include_replies":true,"include_dms":false,"confirmation_token":"paste-one-time-token"}}'`;

const WS_INSTALL = `npm install ws`;

const WS_RELAY = `import WebSocket from "ws";

type AuthenticatedAppUser = { id: string };
type Relay = (externalUserId: string, payload: unknown) => void;
type Refresh = (externalUserId: string) => Promise<void>;

export function connectInboxRelay(
  appUser: AuthenticatedAppUser,
  relayToAppUser: Relay,
  refreshInboxState: Refresh,
) {
  const apiKey = process.env.UNIPOST_API_KEY;
  if (!apiKey) throw new Error("UNIPOST_API_KEY is not configured");

  const url = new URL("wss://api.unipost.dev/v1/inbox/ws");
  url.searchParams.set("inbox_scope", "managed_user");
  url.searchParams.set("external_user_id", appUser.id);

  const socket = new WebSocket(url, {
    headers: { Authorization: \`Bearer \${apiKey}\` },
  });

  socket.on("message", (raw) => {
    const payload = JSON.parse(raw.toString());
    if (payload.external_user_id !== appUser.id) return;
    relayToAppUser(appUser.id, payload);
  });

  socket.on("close", async () => {
    await refreshInboxState(appUser.id); // refresh list and unread count
  });

  return socket;
}`;

const AGGREGATE = `type AuthenticatedAppUser = {
  id: string;
  role: "user" | "owner" | "admin";
};

export async function loadOwnerAdminInbox(appUser: AuthenticatedAppUser) {
  if (appUser.role !== "owner" && appUser.role !== "admin") {
    return new Response("Forbidden", { status: 403 });
  }

  const response = await fetch(
    "https://api.unipost.dev/v1/inbox?inbox_scope=workspace&limit=100",
    { headers: { Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\` } },
  );

  return new Response(await response.text(), {
    status: response.status,
    headers: { "Content-Type": "application/json" },
  });
}`;

const ACCEPTANCE = `# Use only app-owned synthetic fixtures, never a customer account.
SYNTHETIC_USER_A="inbox_accept_a"
SYNTHETIC_USER_B="inbox_accept_b"

# For each user, call the managed-user list through your own backend boundary.
# Then verify these invariants in your acceptance suite:
# 1. A list excludes B; B list excludes A.
# 2. Cross-scope get, read, reply, and thread-state return 404.
# 3. Workspace aggregate sees A and B.
# 4. A's real-time channel never receives B's exact event.
# 5. Remove every synthetic fixture and assert residual counts are zero.`;

export default function InboxIntegrationGuidePage() {
  return (
    <DocsPage
      eyebrow="Inbox Guides"
      title="Integrate UniPost Inbox into your app"
      lead="Map each authenticated app user to one stable external_user_id, keep the workspace API key behind your backend, and use an explicit Inbox scope for every read, write, and real-time connection."
      className="docs-page-guide-redesign"
    >
      <div className="docs-guide-badges">
        <span className="docs-guide-badge">Server-only API key</span>
        <span className="docs-guide-badge">Managed-user isolation</span>
        <span className="docs-guide-badge">Owner/admin aggregate</span>
        <span className="docs-guide-badge">Production acceptance</span>
      </div>

      <p className="docs-guide-note">
        The browser must not receive the UniPost API key. It authenticates only to your app; your backend derives the
        selected UniPost scope from that verified app session.
      </p>

      <h2 id="build">What you will build</h2>
      <ol className="docs-step-list">
        <li>Connect each app user&apos;s social account with the same stable app-owned identity used for Inbox access.</li>
        <li>Proxy managed-user reads and writes through one server-side boundary.</li>
        <li>Relay scoped WebSocket events through your app&apos;s own authenticated channel.</li>
        <li>Keep owner/admin aggregation on a separate route with a second authorization check.</li>
        <li>Prove user A and user B cannot cross scope before production rollout.</li>
      </ol>

      <h2 id="prerequisites">Prerequisites</h2>
      <ul className="docs-checklist">
        <li>An Inbox-eligible workspace plan.</li>
        <li>A workspace API key stored as a backend secret, never in client code or browser storage.</li>
        <li>A UniPost profile that will own the managed social accounts.</li>
        <li>Authenticated app users with stable internal IDs and explicit app owner/admin roles.</li>
      </ul>

      <h2 id="identity">1. Choose one external user ID</h2>
      <p>
        Use an opaque, immutable app user ID such as <code>app_usr_7f4c91</code>. Derive it from the authenticated app user
        on your server. Email is mutable and should not be the primary identity; pass it only as optional reconciliation
        data. Never reuse one <code>external_user_id</code> for multiple app users.
      </p>
      <p>
        The same value must be used when the user connects an account and whenever your backend requests that user&apos;s
        Inbox. This identity continuity is the boundary that associates social accounts, comments, and DMs with the
        intended managed user.
      </p>

      <h2 id="connect">2. Connect the user&apos;s account</h2>
      <p>
        Call <Link href="/docs/api/connect/sessions/create">POST /v1/connect/sessions</Link> from your backend, then redirect
        the browser only to the returned hosted URL. The example uses <code>app_usr_7f4c91</code> from the verified app
        session rather than a browser-supplied request field.
      </p>
      <DocsCodeTabs snippets={[{ lang: "javascript", label: "TypeScript", code: CONNECT_SESSION }]} />
      <p className="docs-guide-note">
        An ownership conflict fails closed. If the provider account is already associated with another
        <code> external_user_id</code>, hosted Connect returns HTTP <code>409</code> and says the social account cannot be
        reassigned. Resolve the account ownership issue; do not change IDs or silently move the account.
      </p>

      <h2 id="backend-boundary">3. Build the managed-user backend boundary</h2>
      <p>
        Authenticate the app request first, then pass the resulting user object into a server-only helper. Do not expose
        <code> inbox_scope</code> or <code>external_user_id</code> as unrestricted browser parameters. Every managed-user
        request must resolve to <code>inbox_scope=managed_user</code> and the app-derived identity.
      </p>
      <DocsCodeTabs snippets={[{ lang: "javascript", label: "TypeScript", code: SCOPED_HELPER }]} />

      <h2 id="reads">4. Read one user&apos;s Inbox</h2>
      <p>
        Read the list with <ApiInlineLink endpoint="GET /v1/inbox" />, get the badge count from
        <code> GET /v1/inbox/unread-count</code>, and fetch a single item with <code>GET /v1/inbox/{id}</code>. The list
        <code> limit</code> defaults to <code>50</code> and caps at <code>500</code>; it is a bounded result size, not cursor
        pagination or an unbounded history export.
      </p>
      <DocsCodeTabs snippets={[{ lang: "javascript", label: "TypeScript", code: READS }]} />
      <p>
        Treat a scoped <code>404</code> as unavailable. Do not attempt to distinguish a missing ID from an item owned by a
        different managed user; that indistinguishability is part of the isolation contract.
      </p>

      <h2 id="writes">5. Perform scoped writes</h2>
      <p>
        Use <code>POST /v1/inbox/{id}/read</code>, <code>POST /v1/inbox/mark-all-read</code>,
        <ApiInlineLink endpoint="POST /v1/inbox/{id}/reply" />, and <code>POST /v1/inbox/{id}/thread-state</code> through the
        same helper. UniPost rechecks the selected scope for each item ID; cross-scope item operations return
        <code> 404</code>.
      </p>
      <DocsCodeTabs snippets={[{ lang: "javascript", label: "TypeScript", code: WRITES }]} />
      <p className="docs-guide-note">
        For an X write with an uncertain outcome, retry with the same idempotency key and payload. Never generate a new
        key just because the network response was lost.
      </p>

      <h2 id="sync">6. Sync the selected scope</h2>
      <p>
        <ApiInlineLink endpoint="POST /v1/inbox/sync" /> without <code>x_backfill</code> runs the ordinary selected-scope
        polling path. It is not the same operation as metered X history lookup.
      </p>
      <DocsCodeTabs snippets={[{ lang: "bash", label: "Ordinary sync", code: ORDINARY_SYNC }]} />
      <p>
        Add <code>x_backfill</code> only when you intend to read bounded X history. Managed-X reads can consume X Credits.
        Inspect <code>estimated_x_credits</code>; when <code>confirmation_required</code> is true, repeat the exact scope,
        account, and request with the short-lived <code>confirmation_token</code>. X DMs remain controlled by the
        <code> x_dms_v1</code> workspace rollout; do not request <code>include_dms</code> unless the capability is available.
      </p>
      <DocsCodeTabs snippets={[{ lang: "bash", label: "X backfill", code: X_BACKFILL }]} />

      <h2 id="realtime">7. Relay real-time events from your backend</h2>
      <p>
        Native browser WebSocket cannot set the required API-key <code>Authorization</code> header. Open
        <code> GET /v1/inbox/ws</code> from a trusted backend process, then relay only the allowed event to the app user&apos;s
        existing authenticated WebSocket or SSE channel.
      </p>
      <DocsCodeTabs snippets={[
        { lang: "bash", label: "Install", code: WS_INSTALL },
        { lang: "javascript", label: "TypeScript", code: WS_RELAY },
      ]} />
      <p>
        Never put a UniPost API key in the WebSocket URL. After a disconnect, reconnect with bounded backoff and refresh
        <ApiInlineLink endpoint="GET /v1/inbox" /> plus <code>GET /v1/inbox/unread-count</code>; real-time delivery is a
        notification path, while the HTTP reads remain authoritative.
      </p>

      <h2 id="aggregate">8. Add a separate owner/admin aggregate</h2>
      <p>
        Check the role in your own app before calling UniPost. Then use <code>inbox_scope=workspace</code> without
        <code> external_user_id</code>. The workspace API key must be creator-bound, and its creator must still hold the
        UniPost owner or admin role. Keep this route separate from every managed-user handler.
      </p>
      <DocsCodeTabs snippets={[{ lang: "javascript", label: "TypeScript", code: AGGREGATE }]} />

      <h2 id="errors">9. Handle errors without weakening scope</h2>
      <p>
        Authentication and explicit Inbox scope resolution run before the plan gate, then the selected endpoint runs. A
        malformed scope can therefore return before a possible <code>402</code>. Never fall back from managed-user scope
        to workspace scope after an error.
      </p>
      <DocsTable
        columns={["Status", "Meaning", "App behavior"]}
        rows={[
          [<code key="400">400</code>, "Missing, duplicate, invalid, or disallowed scope fields.", "Fix the server request; do not retry unchanged."],
          [<code key="401">401</code>, "Invalid, missing, or inactive credentials.", "Stop and replace or reactivate the credential."],
          [<code key="402">402</code>, "The workspace plan does not allow Inbox.", "Show an upgrade path; do not retry automatically."],
          [<code key="403">403</code>, "Insufficient role, creatorless aggregate key, or controlled feature denial.", "Correct authorization or feature eligibility."],
          [<code key="404">404</code>, "Managed user missing, item missing, or item outside the selected scope.", "Return unavailable without disclosing ownership."],
          [<code key="409">409</code>, "Connect ownership conflict or a conflicting durable operation state.", "Resolve ownership or inspect the operation; never reassign silently."],
          [<code key="500">500</code>, <><code>INBOX_SCOPE_LOOKUP_FAILED</code> is a transient pre-handler lookup failure.</>, "Retry the same request with bounded exponential backoff."],
        ]}
      />
      <p className="docs-guide-note">
        The narrow retry guidance above applies to <code>INBOX_SCOPE_LOOKUP_FAILED</code> because the selected endpoint did
        not run. Do not assume every <code>5xx</code> is safe to retry after a write.
      </p>

      <h2 id="security">10. Production security checklist</h2>
      <ul className="docs-checklist">
        <li>Store the API key only in the backend secret manager and redact it from logs.</li>
        <li>Verify the app session before deriving <code>external_user_id</code>.</li>
        <li>Keep managed-user handlers and owner/admin aggregate handlers separate.</li>
        <li>Let UniPost enforce selected scope again for item, reply, and thread IDs.</li>
        <li>Relay real-time events only to the app channel authorized for the matching external user.</li>
        <li>Exclude private DM bodies, participants, access tokens, and raw provider errors from logs and analytics.</li>
        <li>Fail closed; never change to workspace scope because a managed-user lookup failed.</li>
      </ul>

      <h2 id="acceptance">11. Prove A/B isolation before production</h2>
      <p>
        Use only app-owned test identities: synthetic user A and synthetic user B. Do not run this acceptance against a
        customer account. Create controlled Inbox fixtures for each identity, verify every assertion, then remove all
        synthetic fixtures and confirm residual counts are zero.
      </p>
      <DocsCodeTabs snippets={[{ lang: "bash", label: "Acceptance invariants", code: ACCEPTANCE }]} />

      <h2 id="references">12. Endpoint references</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/connect/sessions/create" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Create Connect Session</div>
          <div className="docs-next-body">Bind a hosted account connection to the app-owned external user ID.</div>
        </Link>
        <Link href="/docs/api/inbox/list" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">List Inbox items</div>
          <div className="docs-next-body">Filters, bounded list size, selected-scope response, and errors.</div>
        </Link>
        <Link href="/docs/api/inbox/reply" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Reply to an item</div>
          <div className="docs-next-body">Supported sources, idempotency, and provider-specific outcomes.</div>
        </Link>
        <Link href="/docs/api/inbox/sync" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Sync and X backfill</div>
          <div className="docs-next-body">Ordinary polling, X Credit estimates, and confirmation tokens.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
