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
    "platform": "tiktok",
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
  platform: "tiktok",
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

const POLLING_SNIPPETS = [
  {
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/connect/sessions/cs_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    label: "Node.js",
    code: `const session = await client.connect.getSession("cs_abc123");

if (session.status === "completed") {
  console.log(session.managedAccountId);
}`,
  },
];

export default function ConnectSessionsGuidePage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Guide"
      title="Connect Sessions"
      lead="Create hosted account-connection flows for end users. Connect Sessions can use UniPost's Quickstart OAuth apps or your own white-label platform credentials, depending on how you create the session."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="cs-badges">
        <span className="cs-badge">Customer-owned accounts</span>
        <span className="cs-badge">Hosted OAuth</span>
        <span className="cs-badge">Quickstart credentials</span>
        <span className="cs-badge">White-label credentials</span>
      </div>

      <h2 id="when-to-use">When to use Connect Sessions</h2>
      <p className="cs-note">
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
          ["Credential modes", "Quickstart credentials or white-label credentials"],
        ]}
      />

      <h2 id="credential-modes">Credential modes</h2>
      <p className="cs-note">
        The endpoint is the same in both modes. The difference is which OAuth app
        the platform sees during authorization.
      </p>
      <DocsTable
        columns={["Mode", "How to create it", "OAuth app used", "Best for"]}
        rows={[
          [
            "Quickstart Connect Session",
            <><code>allow_quickstart_creds=true</code></>,
            "Workspace credentials if present; otherwise UniPost's shared app",
            "Customer onboarding where UniPost branding on the platform consent screen is acceptable",
          ],
          [
            "White-label Connect Session",
            <><code>allow_quickstart_creds=false</code> and uploaded platform credentials</>,
            "Your platform app credentials stored in UniPost",
            "Customer onboarding where the platform consent screen must show your app",
          ],
        ]}
      />
      <p className="cs-note">
        If a workspace has platform credentials for the requested platform, UniPost
        uses those credentials. <code>allow_quickstart_creds=true</code> only
        permits fallback to UniPost&apos;s shared app when workspace credentials are
        missing.
      </p>

      <h2 id="quickstart-session">Quickstart Connect Session</h2>
      <p className="cs-note">
        Pass <code>allow_quickstart_creds=true</code> when you want hosted
        customer onboarding without asking the customer to create their own
        platform developer app first.
      </p>
      <DocsCodeTabs snippets={QUICKSTART_SESSION_SNIPPETS} />

      <h2 id="white-label-session">White-label Connect Session</h2>
      <p className="cs-note">
        Upload platform credentials first, then create sessions with{" "}
        <code>allow_quickstart_creds=false</code> so missing credentials fail
        immediately instead of silently falling back to UniPost&apos;s shared app.
      </p>
      <DocsCodeTabs snippets={WHITE_LABEL_SESSION_SNIPPETS} />

      <h2 id="callback-vs-return-url">Callback URL vs return_url</h2>
      <p className="cs-note">
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
            "Where the social platform sends the OAuth code. Quickstart users do not configure this; white-label users copy the exact platform callback URL into their developer app.",
          ],
        ]}
      />
      <p className="cs-note">
        For white-label setup, copy callback URLs from the platform guides under{" "}
        <Link href="/docs/white-label">White-label Mode</Link>. Do not replace
        them with your <code>return_url</code>.
      </p>

      <h2 id="polling">Poll for completion</h2>
      <p className="cs-note">
        The hosted URL is browser-facing. Your backend should poll the session
        until it becomes <code>completed</code>, then store the returned managed
        account id for publishing.
      </p>
      <DocsCodeTabs snippets={POLLING_SNIPPETS} />

      <h2 id="next-steps">Next steps</h2>
      <div className="cs-next">
        <Link href="/docs/api/connect/sessions/create" className="cs-next-card">
          <div className="cs-next-kicker">API reference</div>
          <div className="cs-next-title">Create session</div>
          <div className="cs-next-body">Full request and response schema for <code>POST /v1/connect/sessions</code>.</div>
        </Link>
        <Link href="/docs/api/connect/sessions/get" className="cs-next-card">
          <div className="cs-next-kicker">API reference</div>
          <div className="cs-next-title">Get session</div>
          <div className="cs-next-body">Poll status and read the completed managed account id.</div>
        </Link>
        <Link href="/docs/white-label" className="cs-next-card">
          <div className="cs-next-kicker">Branded OAuth</div>
          <div className="cs-next-title">White-label setup</div>
          <div className="cs-next-body">Upload platform credentials, configure branding, and copy platform callback URLs.</div>
        </Link>
        <Link href="/docs/publishing" className="cs-next-card">
          <div className="cs-next-kicker">After connection</div>
          <div className="cs-next-title">Publishing guide</div>
          <div className="cs-next-body">Use the connected account id to publish with hosted URLs or uploaded media.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.cs-badges{display:flex;flex-wrap:wrap;gap:7px;margin:4px 0 28px}
.cs-badge{display:inline-flex;align-items:center;height:26px;padding:0 10px;border-radius:6px;background:#f8fafc;border:1px solid #e5e9f0;color:#4d5565;font-size:11.5px;font-weight:650;letter-spacing:0}
.cs-note{font-size:15px;line-height:1.72;color:var(--docs-text-soft);margin:8px 0 16px;max-width:820px}
.cs-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin-top:10px}
.cs-next-card{display:flex;min-height:138px;flex-direction:column;padding:16px;border:1px solid #e5e9f0;border-radius:8px;background:#ffffff;text-decoration:none;color:inherit;box-shadow:0 1px 0 rgba(15,23,42,.02);transition:border-color .14s ease,background .14s ease,transform .14s ease}
.cs-next-card:hover{border-color:#ccd4df;background:#fbfcfe;transform:translateY(-1px);text-decoration:none!important}
.cs-next-kicker{font-size:10.5px;font-weight:760;letter-spacing:.08em;text-transform:uppercase;color:#6f7685;margin-bottom:10px}
.cs-next-title{font-size:15px;font-weight:720;color:var(--docs-text);margin-bottom:7px;letter-spacing:-.01em}
.cs-next-body{font-size:13px;line-height:1.58;color:var(--docs-text-soft)}
.cs-next-body code{font-family:var(--docs-mono);font-size:12px}
@media (max-width:760px){
  .cs-next{grid-template-columns:1fr}
}
`;
