import type { Metadata } from "next";
import Link from "next/link";
import { getPublicDocsFeatureFlags } from "@/lib/public-feature-flags-server";
import { DocsCodeTabs, DocsPage } from "../../../_components/docs-shell";

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/guides/x/comments" },
};

const LIST = `# GET /v1/inbox
curl "https://api.unipost.dev/v1/inbox?inbox_scope=managed_user&external_user_id=user_123&source=x_reply&is_own=false&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`;
const REPLY = `# POST /v1/inbox/:id/reply
curl -X POST "https://api.unipost.dev/v1/inbox/inbox_x_01/reply?inbox_scope=managed_user&external_user_id=user_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: x-comment-inbox-x-01-v1" \\
  -H "Content-Type: application/json" \\
  -d '{"text":"The release notes are available at docs.example.com/releases."}'`;
const SYNC = `# POST /v1/inbox/sync
curl -X POST "https://api.unipost.dev/v1/inbox/sync?inbox_scope=managed_user&external_user_id=user_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"x_backfill":{"account_id":"sa_x_01","lookback_days":7,"max_items":50,"include_replies":true,"include_dms":false}}'`;
const CONFIRMED_SYNC = `# Repeat the exact request when confirmation_required is true.
CONFIRMATION_TOKEN="paste-confirmation-token"
curl -X POST "https://api.unipost.dev/v1/inbox/sync?inbox_scope=managed_user&external_user_id=user_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  --data-binary @- <<JSON
{"x_backfill":{"account_id":"sa_x_01","lookback_days":7,"max_items":50,"include_replies":true,"include_dms":false,"confirmation_token":"$CONFIRMATION_TOKEN"}}
JSON`;

export default async function XCommentsGuidePage() {
  const publicFeatureFlags = await getPublicDocsFeatureFlags();
  const xDMsEnabled = publicFeatureFlags.x_dms_v1;
  const xCreditsEnabled = publicFeatureFlags.x_credits_billing_v1;

  return (
    <DocsPage
      eyebrow="X Guides"
      title="Receive and reply to X comments"
      lead="Bring eligible X public replies into UniPost as x_reply, list them through the unified Inbox, and respond with a durable idempotency key."
      className="docs-page-guide-redesign"
    >
      <div className="docs-guide-badges">
        <span className="docs-guide-badge">Basic plan or higher</span>
        <span className="docs-guide-badge">Eligible summons only</span>
        <span className="docs-guide-badge">Managed or workspace X app</span>
      </div>
      <p className="docs-guide-note">
        X comments use OAuth 2.0 and remain available independently. UniPost still enforces its internal inbound safety
        boundary.
      </p>
      <p className="docs-guide-note">
        Keep the workspace API key server-side and derive <code>external_user_id</code> from the authenticated app user.
        Use <code>inbox_scope=workspace</code> only for an owner/admin aggregate.
      </p>

      <h2 id="prerequisites">1. Verify account capability</h2>
      <p>
        Call <Link href="/docs/api/accounts/capabilities">Get account capabilities</Link> and require
        <code> x_inbox.comments_enabled</code>. For <code>unipost_managed_app</code>, UniPost operates the X app and
        managed reads, inbound events, and replies use the managed X connection. For <code>workspace_x_app</code>,
        your X app must have Client ID, Client Secret, app Bearer Token, and Consumer Secret configured; those operations
        use your X access.
        {xCreditsEnabled ? " Managed-X operations consume the workspace X Credits allowance; workspace-app operations do not." : null}
      </p>
      <p>
        Inbox is available on Basic and higher plans. If the API returns <code>plan_feature_not_available</code>, upgrade
        before retrying. If <code>reconnect_required</code> is true, follow the{" "}
        <Link href="/docs/guides/x/reconnect-permissions">X reconnect guide</Link>.
      </p>

      <h2 id="list">2. List public replies</h2>
      <p>
        Filter the <Link href="/docs/api/inbox/list">Inbox list reference</Link> by <code>x_reply</code>. A persisted X
        item includes its account, reply-tree keys, author, text, timestamp, and a permalink when X provides one.
      </p>
      <DocsCodeTabs snippets={[{ lang: "curl", label: "List", code: LIST }]} />

      <h2 id="reply">3. Reply once</h2>
      <p>
        Use the <Link href="/docs/api/inbox/reply">Inbox reply reference</Link>. UniPost accepts a public X reply only
        when the inbound record proves that the author summoned the connected account. Always send a stable
        <code> Idempotency-Key</code>; retry an uncertain result with the same key and payload.
      </p>
      <DocsCodeTabs snippets={[{ lang: "curl", label: "Reply", code: REPLY }]} />

      <h2 id="sync">4. Run a bounded backfill</h2>
      <p>
        Use the <Link href="/docs/api/inbox/sync">Inbox sync reference</Link> for up to seven days of X replies. If the
        response has <code>confirmation_required: true</code>, repeat the exact account, lookback, maximum, and include
        fields with the returned confirmation token before it expires.
      </p>
      <DocsCodeTabs snippets={[
        { lang: "curl", label: "Estimate", code: SYNC },
        { lang: "curl", label: "Confirm", code: CONFIRMED_SYNC },
      ]} />

      <h2 id="limits">5. Handle allowance and cap boundaries</h2>
      <ul className="docs-checklist">
        {xCreditsEnabled ? <li><code>x_monthly_usage_limit_exceeded</code>: managed-X allowance is exhausted; stop automatic retries.</li> : null}
        <li><code>x_inbound_daily_cap_exceeded</code>: inbound admission paused for the current UTC day; resume after reset or an administrator changes the cap.</li>
        <li><code>x_write_outcome_pending</code>: do not resend with a new key; poll or retry with the original key.</li>
        <li><code>x_reconnect_required</code>: reconnect with tweet.read, tweet.write, users.read, and offline.access.</li>
      </ul>

      <h2 id="related">Related Inbox docs</h2>
      <p>
        Start from the <Link href="/docs/api/inbox">Inbox API overview</Link>. Permission problems belong in the{" "}
        <Link href="/docs/guides/x/reconnect-permissions">reconnect guide</Link>.
        {xDMsEnabled ? <> For private conversations, continue to the{" "}
          <Link href="/docs/guides/x/direct-messages">X direct messages guide</Link>.</> : null}
        {xCreditsEnabled ? <> Use the <Link href="/docs/api/x-credits">X Credits API reference</Link> for live allowance
          fields and the <Link href="/docs/guides/x/credits">X Credits guide</Link> for operation planning.</> : null}
      </p>
    </DocsPage>
  );
}
