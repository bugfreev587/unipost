import Link from "next/link";
import { DocsCodeTabs, DocsPage } from "../../../_components/docs-shell";

const LIST = `# GET /v1/inbox
curl "https://api.unipost.dev/v1/inbox?source=x_dm&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`;
const REPLY = `# POST /v1/inbox/:id/reply
curl -X POST "https://api.unipost.dev/v1/inbox/inbox_x_dm_01/reply" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: x-dm-inbox-x-dm-01-v1" \\
  -H "Content-Type: application/json" \\
  -d '{"text":"Thanks. A support specialist will follow up here."}'`;
const SYNC = `# POST /v1/inbox/sync
curl -X POST "https://api.unipost.dev/v1/inbox/sync" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"x_backfill":{"account_id":"sa_x_01","lookback_days":30,"max_items":50,"include_replies":false,"include_dms":true}}'`;

export default function XDirectMessagesGuidePage() {
  return (
    <DocsPage
      eyebrow="X Guides"
      title="Receive and reply to X direct messages"
      lead="Use UniPost Inbox for legacy X direct-message events, private threads, bounded history sync, and idempotent replies."
      className="docs-page-guide-redesign"
    >
      <div className="docs-guide-badges">
        <span className="docs-guide-badge">Basic plan or higher</span>
        <span className="docs-guide-badge">Legacy DM API</span>
        <span className="docs-guide-badge">Private content</span>
      </div>

      <h2 id="prerequisites">1. Confirm DM access</h2>
      <p>
        Read <Link href="/docs/api/accounts/capabilities">account capabilities</Link> and require
        <code> x_inbox.dms_enabled</code>. The connected X account needs <code>dm.read</code>, <code>dm.write</code>,
        <code> tweet.read</code>, <code>tweet.write</code>, <code>users.read</code>, and <code>offline.access</code>. If any
        are missing, use the <Link href="/docs/guides/x/reconnect-permissions">reconnect procedure</Link>.
      </p>
      <p>
        With <code>unipost_managed_app</code>, UniPost manages delivery and counts managed reads, inbound events, and
        sent DMs against the workspace allowance. With <code>workspace_x_app</code>, your app needs Client ID, Client
        Secret, app Bearer Token, and Consumer Secret; activity uses your X access and bypasses UniPost X Credits.
      </p>

      <h2 id="list">2. List private threads</h2>
      <p>
        Use the <Link href="/docs/api/inbox/list">Inbox list reference</Link> with <code>source=x_dm</code>. Treat
        <code> body</code>, author identifiers, and conversation metadata as private customer data; do not copy them into
        usage keys, logs, or analytics.
      </p>
      <DocsCodeTabs snippets={[{ lang: "curl", label: "List", code: LIST }]} />

      <h2 id="reply">3. Send an idempotent DM</h2>
      <p>
        The <Link href="/docs/api/inbox/reply">Inbox reply endpoint</Link> resolves the persisted conversation or
        participant and records the outbound item in the same thread. A stable <code>Idempotency-Key</code> prevents an
        uncertain network response from creating a duplicate message.
      </p>
      <DocsCodeTabs snippets={[{ lang: "curl", label: "Reply", code: REPLY }]} />

      <h2 id="sync">4. Backfill recent DM events</h2>
      <p>
        The <Link href="/docs/api/inbox/sync">Inbox sync endpoint</Link> can request up to 30 days of X DM events. A
        high managed-X estimate returns a confirmation token before any paid read. Repeat the exact request with that
        token; changing the account set or request invalidates confirmation.
      </p>
      <DocsCodeTabs snippets={[{ lang: "curl", label: "Sync", code: SYNC }]} />

      <h2 id="limits">5. Branch on stable conditions</h2>
      <ul className="docs-checklist">
        <li><code>plan_feature_not_available</code>: Inbox requires the Basic plan or higher.</li>
        <li><code>x_monthly_usage_limit_exceeded</code>: stop managed-X work until allowance becomes available.</li>
        <li><code>x_inbound_daily_cap_exceeded</code>: new inbound DMs are suppressed at the UTC-day safety boundary.</li>
        <li><code>x_reconnect_required</code>: reconnect and grant the missing DM permissions.</li>
        <li><code>x_write_outcome_pending</code>: reuse the original idempotency key; never blindly resend.</li>
      </ul>
    </DocsPage>
  );
}
