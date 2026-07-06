import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

const QUICKSTART_SESSION_SNIPPETS = [
  {
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/connect/sessions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "linkedin",
    "profile_id": "pr_brand_us",
    "external_user_id": "creator-user-42",
    "external_user_email": "creator@example.com",
    "return_url": "https://app.example.com/integrations/done",
    "allow_quickstart_creds": true
  }'`,
  },
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const session = await client.connect.createSession({
  platform: "linkedin",
  profileId: "pr_brand_us",
  externalUserId: "creator-user-42",
  externalUserEmail: "creator@example.com",
  returnUrl: "https://app.example.com/integrations/done",
  allowQuickstartCreds: true,
});

console.log(session.url);`,
  },
];

const WHITE_LABEL_SESSION_SNIPPETS = [
  {
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/connect/sessions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "linkedin",
    "profile_id": "pr_brand_us",
    "external_user_id": "customer-user-42",
    "external_user_email": "alex@example.com",
    "return_url": "https://app.example.com/integrations/linkedin/done",
    "allow_quickstart_creds": false
  }'`,
  },
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const session = await client.connect.createSession({
  platform: "linkedin",
  profileId: "pr_brand_us",
  externalUserId: "customer-user-42",
  externalUserEmail: "alex@example.com",
  returnUrl: "https://app.example.com/integrations/linkedin/done",
  allowQuickstartCreds: false,
});

console.log(session.url);`,
  },
];

const WEBHOOK_SNIPPETS = [
  {
    label: "account.connected",
    lang: "json",
    code: `{
  "event": "account.connected",
  "timestamp": "2026-04-08T10:00:00Z",
  "data": {
    "social_account_id": "sa_linkedin_123",
    "profile_id": "pr_brand_us",
    "platform": "linkedin",
    "account_name": "Example Company",
    "external_user_id": "creator-user-42",
    "connection_type": "managed"
  }
}`,
  },
];

const POLLING_SNIPPETS = [
  {
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/connect/sessions/cs_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    label: "Node.js",
    code: `const terminal = new Set(["completed", "expired", "cancelled"]);
const deadline = Date.now() + 31 * 60 * 1000;
let connectedAccountId = null;

while (Date.now() < deadline) {
  const session = await client.connect.getSession("cs_abc123");

  if (session.status === "completed") {
    connectedAccountId = session.managedAccountId;
    break;
  }

  if (terminal.has(session.status)) {
    throw new Error(\`Connect session ended with status: \${session.status}\`);
  }

  await new Promise((resolve) => setTimeout(resolve, 3000));
}

if (!connectedAccountId) {
  throw new Error("Connect session did not complete before timeout");
}

console.log(connectedAccountId);`,
  },
];

const CLOUDFLARE_WORKERS_SNIPPETS = [
  {
    label: "SDK",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
  baseUrl: "https://origin-api.unipost.dev",
});`,
  },
  {
    label: "wrangler.toml",
    lang: "toml",
    code: `compatibility_flags = ["nodejs_compat"]`,
  },
];

export default function ConnectSessionsGuidePage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Guide"
      title="Connect Sessions"
      lead="Create hosted account-connection flows for end users. Connect Sessions can use UniPost's shared OAuth apps or workspace platform credentials, depending on credential availability and allow_quickstart_creds."
    >
      <div className="docs-guide-badges">
        <span className="docs-guide-badge">Customer-owned accounts</span>
        <span className="docs-guide-badge">Hosted OAuth</span>
        <span className="docs-guide-badge">Shared UniPost OAuth app</span>
        <span className="docs-guide-badge">Workspace platform credentials</span>
      </div>

      <h2 id="when-to-use">When to use Connect Sessions</h2>
      <p className="docs-guide-note">
        Use Connect Sessions when your product needs to send an end user through
        account authorization and then publish on behalf of the account they
        connected. Use <ApiInlineLink endpoint="POST /v1/oauth/connect" /> when
        you are connecting accounts owned by your own workspace team.
      </p>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Primary API", <ApiInlineLink key="create-session" endpoint="POST /v1/connect/sessions" />],
          ["Who authorizes", "Your end user, inside UniPost's hosted Connect flow"],
          ["What you store", <><code>external_user_id</code> plus the completed <code>managed_account_id</code></>],
          ["What you publish with", <><code>managed_account_id</code>, used as the UniPost <code>account_id</code> in <code>platform_posts</code></>],
          ["Credential modes", "Shared UniPost OAuth app or workspace platform credentials"],
          ["Session lifetime", "30 minutes. Polling after the TTL returns expired for sessions still pending."],
          ["Plan gates", "Some platforms can require a paid plan before account connection. For example, X / Twitter may return 402 PLAN_PLATFORM_NOT_ALLOWED."],
        ]}
      />

      <h2 id="credential-modes">Credential sources</h2>
      <p className="docs-guide-note">
        The endpoint is the same in both modes. The difference is which OAuth app
        the platform sees during authorization. If you omit{" "}
        <code>allow_quickstart_creds</code>, it defaults to <code>false</code>,
        so OAuth platforms require uploaded workspace Platform Credentials.
      </p>
      <DocsTable
        columns={["Source", "How to create it", "OAuth app used", "Best for"]}
        rows={[
          [
            "Shared UniPost OAuth app",
            <><code>allow_quickstart_creds=true</code></>,
            "Workspace credentials if present; otherwise UniPost's shared OAuth app",
            "Customer onboarding where shared OAuth app fallback is acceptable",
          ],
          [
            "Workspace platform credentials",
            <><code>allow_quickstart_creds=false</code> and uploaded platform credentials</>,
            "Your platform app credentials stored in UniPost",
            "Customer onboarding where the platform consent screen must show your app",
          ],
        ]}
      />
      <p className="docs-guide-note">
        If a workspace has platform credentials for the requested platform, UniPost
        uses those credentials. <code>allow_quickstart_creds=true</code> only
        permits fallback to UniPost&apos;s shared app when workspace credentials are
        missing.
      </p>
      <p className="docs-guide-note">
        On Basic, workspace Platform Credentials are active only for the workspace&apos;s
        shared custom platform slot. Growth, Team, and Enterprise can use workspace
        Platform Credentials across all supported OAuth platforms.
      </p>
      <p className="docs-guide-note">
        These credential modes apply to OAuth platforms. Bluesky uses app
        passwords instead of OAuth apps, so <code>allow_quickstart_creds</code>{" "}
        does not change the Bluesky Connect Session path.
      </p>

      <h2 id="quickstart-session">Shared-app fallback session</h2>
      <p className="docs-guide-note">
        Pass <code>allow_quickstart_creds=true</code> when you want hosted
        customer onboarding without requiring workspace-owned Platform
        Credentials first.
      </p>
      <DocsCodeTabs snippets={QUICKSTART_SESSION_SNIPPETS} />

      <h2 id="white-label-session">Workspace-credential session</h2>
      <p className="docs-guide-note">
        Upload workspace Platform Credentials first, then create sessions with{" "}
        <code>allow_quickstart_creds=false</code> so missing credentials fail
        immediately instead of silently falling back to UniPost&apos;s shared app.
      </p>
      <DocsCodeTabs snippets={WHITE_LABEL_SESSION_SNIPPETS} />

      <h2 id="callback-vs-return-url">Callback URL vs return_url</h2>
      <p className="docs-guide-note">
        These names are easy to mix up, but they control different redirects.
      </p>
      <DocsTable
        columns={["Field or URL", "Who controls it", "Purpose"]}
        rows={[
          [
            <code key="return-url">return_url</code>,
            "Your integration",
            "Where UniPost sends the browser after the Connect Session finishes or fails.",
          ],
          [
            "OAuth callback URL / redirect URI",
            "UniPost and the platform developer console",
            "Where the social platform sends the OAuth code. Shared-app fallback users do not configure this; workspace credential users copy the exact platform callback URL into their developer app.",
          ],
        ]}
      />
      <p className="docs-guide-note">
        For workspace Platform Credentials, copy callback URLs from the platform guides under{" "}
        <Link href="/docs/platform-credentials">Platform Credentials</Link>. Do not replace
        them with your <code>return_url</code>.
      </p>

      <h2 id="completion">Handle completion</h2>
      <p className="docs-guide-note">
        The hosted URL is browser-facing. Your backend should subscribe to the{" "}
        <code>account.connected</code> webhook and store the returned{" "}
        <code>social_account_id</code> as the account id for future publishing.
        This is the recommended production path because UniPost pushes the result
        as soon as the account is connected.
      </p>
      <DocsCodeTabs snippets={WEBHOOK_SNIPPETS} />
      <p className="docs-guide-note">
        See <Link href="/docs/api/webhooks">Developer webhooks</Link> for
        subscription setup, signatures, and retry behavior.
      </p>

      <h2 id="polling-fallback">Poll as a fallback</h2>
      <p className="docs-guide-note">
        Poll <ApiInlineLink endpoint="GET /v1/connect/sessions/:session_id" />{" "}
        for local development, CLI demos, or integrations that cannot receive
        webhooks. Stop polling on every terminal state:
        <code> completed</code>, <code>expired</code>, or <code>cancelled</code>.
        A pending session expires after 30 minutes.
      </p>
      <DocsCodeTabs snippets={POLLING_SNIPPETS} />

      <h2 id="cloudflare-workers">Cloudflare Workers and Wrangler</h2>
      <p className="docs-guide-note">
        If you call Connect Sessions from Cloudflare Workers or local{" "}
        <code>wrangler dev</code> and see an error like{" "}
        <code>internal error; reference = ...</code>, the request may be
        failing inside the workerd runtime before it reaches UniPost. In that
        environment, configure the SDK with UniPost&apos;s DNS-only origin API
        endpoint. Keep using <code>https://api.unipost.dev</code> everywhere
        else.
      </p>
      <DocsCodeTabs snippets={CLOUDFLARE_WORKERS_SNIPPETS} />
      <p className="docs-guide-note">
        No cache clearing is normally required. If the error persists after
        switching <code>baseUrl</code>, restart <code>wrangler dev</code> once
        so workerd picks up the new hostname resolution.
      </p>

      <h2 id="next-steps">Next steps</h2>
      <div className="docs-guide-next">
        <Link href="/docs/api/connect/sessions/create" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">API reference</div>
          <div className="docs-guide-next-title">Create session</div>
          <div className="docs-guide-next-body">Full request and response schema for <code>POST /v1/connect/sessions</code>.</div>
        </Link>
        <Link href="/docs/api/connect/sessions/get" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">API reference</div>
          <div className="docs-guide-next-title">Get session</div>
          <div className="docs-guide-next-body">Fallback polling for status and the completed managed account id.</div>
        </Link>
        <Link href="/docs/local-connect-test" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Local testing</div>
          <div className="docs-guide-next-title">Run Connect from your terminal</div>
          <div className="docs-guide-next-body">Download the helper script and copy the returned hosted OAuth URL into a browser.</div>
        </Link>
        <Link href="/docs/platform-credentials" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Developer apps</div>
          <div className="docs-guide-next-title">Platform Credentials</div>
          <div className="docs-guide-next-body">Upload platform credentials and copy exact OAuth callback URLs.</div>
        </Link>
        <Link href="/docs/white-label" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Branding</div>
          <div className="docs-guide-next-title">Hosted Connect</div>
          <div className="docs-guide-next-body">Configure the white-label page shown before platform OAuth.</div>
        </Link>
        <Link href="/docs/publishing" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">After connection</div>
          <div className="docs-guide-next-title">Publishing guide</div>
          <div className="docs-guide-next-body">Use the connected account id to publish with hosted URLs or uploaded media.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
