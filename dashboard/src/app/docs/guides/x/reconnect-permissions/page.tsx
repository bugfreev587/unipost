import Link from "next/link";
import { DocsCodeTabs, DocsPage } from "../../../_components/docs-shell";

const CAPABILITIES = `curl "https://api.unipost.dev/v1/accounts/sa_x_01/capabilities" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`;
const LIST = `# GET /v1/inbox
curl "https://api.unipost.dev/v1/inbox?source=x_dm&limit=10" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`;
const REPLY = `# POST /v1/inbox/:id/reply
curl -X POST "https://api.unipost.dev/v1/inbox/inbox_x_dm_01/reply" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: verify-x-reconnect-01" \\
  -H "Content-Type: application/json" \\
  -d '{"text":"Permission check complete."}'`;
const SYNC = `# POST /v1/inbox/sync
curl -X POST "https://api.unipost.dev/v1/inbox/sync" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"x_backfill":{"account_id":"sa_x_01","lookback_days":7,"max_items":5,"include_replies":true,"include_dms":true}}'`;

export default function XReconnectPermissionsGuidePage() {
  return (
    <DocsPage
      eyebrow="X Guides"
      title="Reconnect X Inbox permissions"
      lead="Diagnose the account-specific X Inbox capability, correct workspace-app credentials when needed, then repeat OAuth so UniPost receives the current scopes."
      className="docs-page-guide-redesign"
    >
      <h2 id="inspect">1. Inspect the exact capability state</h2>
      <p>
        Call <Link href="/docs/api/accounts/capabilities">GET account capabilities</Link>. Read
        <code> x_inbox.app_mode</code>, <code>missing_scopes</code>, <code>missing_app_credentials</code>,
        <code> reconnect_required</code>, and <code>delivery_status</code>. Inbox requires the Basic plan or higher;
        <code> paused_plan</code> is fixed by plan eligibility, not OAuth.
      </p>
      <DocsCodeTabs snippets={[{ lang: "curl", label: "Capabilities", code: CAPABILITIES }]} />

      <h2 id="workspace-app">2. Complete workspace X app credentials</h2>
      <p>
        Skip this step for <code>unipost_managed_app</code>. For <code>workspace_x_app</code>, open Dashboard → Settings →
        Platform Credentials → X / Twitter and save all four values: Client ID, Client Secret, app Bearer Token, and
        Consumer Secret. Saving only the OAuth client keeps publishing available but leaves X Inbox delivery disabled.
        API users can use the <Link href="/docs/api/platform-credentials/create">Platform Credentials reference</Link>.
      </p>

      <h2 id="oauth">3. Reconnect the account</h2>
      <ol className="docs-checklist">
        <li>Open Dashboard → Project → Accounts.</li>
        <li>Find the affected X / Twitter account and choose Reconnect.</li>
        <li>On X, approve the complete requested permission set: tweet.read, tweet.write, users.read, offline.access, dm.read, and dm.write.</li>
        <li>Return to UniPost and wait for the account to show Active.</li>
        <li>Call capabilities again. Require an empty <code>missing_scopes</code> array and the needed comments or DM capability to be true.</li>
      </ol>

      <h2 id="verify">4. Verify list, reply, and sync</h2>
      <p>
        Start with read-only calls. The exact contracts are in <Link href="/docs/api/inbox/list">List Inbox</Link>,{" "}
        <Link href="/docs/api/inbox/reply">Reply to Inbox</Link>, and <Link href="/docs/api/inbox/sync">Sync Inbox</Link>.
        Only send the reply example to a controlled account when you intend to create a real X message.
      </p>
      <DocsCodeTabs snippets={[{ lang: "curl", label: "List", code: LIST }, { lang: "curl", label: "Reply", code: REPLY }, { lang: "curl", label: "Sync", code: SYNC }]} />

      <h2 id="errors">5. Interpret remaining boundaries</h2>
      <p>
        <code>x_reconnect_required</code> means scopes are still missing. <code>x_monthly_usage_limit_exceeded</code> and
        <code> x_inbound_daily_cap_exceeded</code> are allowance boundaries and are not fixed by reconnecting. Managed
        mode consumes UniPost X Credits; <code>workspace_x_app</code> uses your X app access and bypasses the UniPost
        allowance.
      </p>

      <h2 id="related">Related Inbox and X Credits docs</h2>
      <p>
        Review the <Link href="/docs/api/inbox">Inbox API overview</Link>, then choose the{" "}
        <Link href="/docs/guides/x/comments">X comments guide</Link> or{" "}
        <Link href="/docs/guides/x/direct-messages">X direct messages guide</Link>. Use the{" "}
        <Link href="/docs/api/x-credits">X Credits API reference</Link> to inspect current boundaries and the{" "}
        <Link href="/docs/guides/x/credits">X Credits guide</Link> to plan managed-X usage.
      </p>
    </DocsPage>
  );
}
