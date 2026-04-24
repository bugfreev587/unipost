import Link from "next/link";
import { DocsCode, DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

const UPLOAD_CREDS_SNIPPETS = [
  {
    label: "Note",
    code: `SDK support for platform credential management is coming soon.
Use the dashboard or the REST endpoint for this workspace-admin step today.`,
  },
];

const PATCH_BRANDING_SNIPPETS = [
  {
    label: "Note",
    code: `SDK support for profile branding management is coming soon.
Use the dashboard or the REST endpoint for this profile-admin step today.`,
  },
];

const CREATE_SESSION_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const session = await client.connect.createSession({
  platform: "linkedin",
  externalUserId: "acme_user_42",
  externalUserEmail: "user42@customer.com",
  returnUrl: "https://app.acme.com/integrations/linkedin/done",
});

console.log(session.url);`,
  },
];

export default function WhiteLabelPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Guide"
      title="White-label"
      lead="White-label lets your customers connect their own social accounts through a UniPost-hosted flow that shows your brand, runs against your OAuth apps, and never mentions UniPost to the end user."
    >
      <h2 id="when-to-use">When to use white-label</h2>
      <p>Use white-label when the social accounts you publish to belong to your <em>customers&apos;</em> end users, not to you. Without it, the OAuth consent page says &ldquo;UniPost&rdquo; and the hosted Connect page falls back to UniPost&apos;s default look. Both are fine for internal tools but wrong for a customer-facing product.</p>
      <DocsTable
        columns={["Scenario", "Pick this"]}
        rows={[
          ["You publish to social accounts your team owns", "Quickstart (no white-label)"],
          ["Your customers bring their own accounts via a hosted flow", "White-label"],
          ["Your customers shouldn’t know UniPost exists", "White-label (required)"],
          ["Your product is on the free tier", "Quickstart — white-label is a paid feature"],
        ]}
      />

      <h2 id="the-three-layers">The three layers</h2>
      <p>White-label composes three independent pieces. You can configure them separately, and each has a clean fallback if it isn&apos;t set.</p>
      <DocsTable
        columns={["Layer", "What it controls", "Default if unset"]}
        rows={[
          [
            "OAuth credentials",
            "Which platform App ID / secret is used when the end user authorizes",
            "UniPost’s global App — consent shows “UniPost”",
          ],
          [
            "Profile branding",
            "Logo, display name, and primary color on the hosted Connect page",
            "UniPost’s default look",
          ],
          [
            "Connect URL",
            "Public URL your backend hands to the end user to start OAuth",
            "Always required — nothing to fall back to",
          ],
        ]}
      />

      <h2 id="setup">Setup (5 steps)</h2>
      <p>A complete white-label setup takes about 30 minutes the first time. Skipping any step degrades to a Quickstart-style experience for that piece — the end user still completes OAuth, they just see UniPost&apos;s branding or OAuth app instead of yours.</p>

      <h3 id="step-1">1. Register platform OAuth apps</h3>
      <p>Create an App in each platform&apos;s developer portal for the platforms you plan to support. This is the work you can&apos;t skip — UniPost can&apos;t proxy someone else&apos;s App credentials.</p>
      <DocsTable
        columns={["Platform", "Portal", "Needs App Review?"]}
        rows={[
          [
            "Meta (Facebook + Instagram + Threads)",
            <a key="meta" href="https://developers.facebook.com" target="_blank" rel="noreferrer noopener">developers.facebook.com</a>,
            "Yes, for public use",
          ],
          [
            "LinkedIn",
            <a key="li" href="https://www.linkedin.com/developers" target="_blank" rel="noreferrer noopener">www.linkedin.com/developers</a>,
            "No — products auto-approve",
          ],
          [
            "TikTok",
            <a key="tt" href="https://developers.tiktok.com" target="_blank" rel="noreferrer noopener">developers.tiktok.com</a>,
            "Yes — audit required",
          ],
          [
            "YouTube",
            <a key="yt" href="https://console.cloud.google.com" target="_blank" rel="noreferrer noopener">console.cloud.google.com</a>,
            "No, but scope approval helps",
          ],
          [
            "X / Twitter",
            <a key="x" href="https://developer.x.com" target="_blank" rel="noreferrer noopener">developer.x.com</a>,
            "Tier-dependent",
          ],
        ]}
      />

      <h3 id="step-2">2. Whitelist the redirect URI</h3>
      <p>In each platform&apos;s App settings, add UniPost&apos;s OAuth callback as an authorized redirect URI. The platform refuses to redirect back with the <code>code</code> otherwise and your users see a generic error page.</p>
      <DocsCode code={`https://api.unipost.dev/v1/oauth/callback/{platform}`} language="text" />
      <p>Substitute <code>{"{platform}"}</code> with <code>linkedin</code>, <code>twitter</code>, <code>tiktok</code>, <code>youtube</code>, or <code>meta</code> — one entry per platform whose credentials you&apos;ll upload. This is UniPost&apos;s fixed callback path; the full OAuth round-trip is documented in the <Link href="/docs/api/connect/sessions">Connect Sessions reference</Link>.</p>

      <h3 id="step-3">3. Upload credentials to UniPost</h3>
      <p>Upload your Client ID and Client Secret for each platform. Either from the dashboard at Accounts → White-label Credentials, or via <ApiInlineLink endpoint="POST /v1/workspaces/{id}/platform-credentials" />. This workspace-admin endpoint is not in the SDK yet:</p>
      <DocsCodeTabs snippets={UPLOAD_CREDS_SNIPPETS} />
      <p>UniPost encrypts the secret at rest. Once uploaded, every Connect session on this workspace for that platform runs against <em>your</em> App — the platform consent screen shows your App name, your privacy policy URL, your logo.</p>

      <h3 id="step-4">4. Set profile branding</h3>
      <p><ApiInlineLink endpoint="PATCH /v1/profiles/{id}" /> the three branding fields on the profile that will own the Connect sessions. The hosted Connect page pulls these values at render time; there&apos;s no cache layer in between, so the change is immediate. This profile-admin endpoint is not in the SDK yet:</p>
      <DocsCodeTabs snippets={PATCH_BRANDING_SNIPPETS} />
      <DocsTable
        columns={["Field", "Validation", "Where it appears"]}
        rows={[
          ["branding_logo_url", "HTTPS URL, ≤ 2 KB, common image formats", "Top-left of hosted Connect page"],
          ["branding_display_name", "≤ 60 characters", "Page title + meta description"],
          ["branding_primary_color", "6-digit hex e.g. #10b981", "Button / accent color"],
        ]}
      />

      <h3 id="step-5">5. Create a session and test</h3>
      <p>Call <ApiInlineLink endpoint="POST /v1/connect/sessions" /> for the platform, with a stable <code>external_user_id</code> you own, and a <code>return_url</code> where you want the end user to land when they&apos;re done.</p>
      <DocsCodeTabs snippets={CREATE_SESSION_SNIPPETS} />
      <p>Response includes a <code>url</code> field — that&apos;s the hosted Connect URL. Open it in a private / incognito window (so you experience the flow the way a fresh end user will) and verify: your branding renders, OAuth redirects to <em>your</em> App&apos;s consent screen, and you land back at <code>return_url</code> after authorizing.</p>
      <p>A reference test script in the repo, <code>scripts/test_whitelabel_linkedin.py</code>, walks the full flow for LinkedIn end-to-end: profile + branding, session creation, OAuth wait, publish, analytics, cleanup.</p>

      <h2 id="end-user-journey">What your end users see</h2>
      <p>After setup, the end-user journey is short and never references UniPost. At most three pages, often two.</p>
      <DocsTable
        columns={["Step", "Page", "Branded as"]}
        rows={[
          ["Your app triggers Connect", "(no page — server-to-server)", "—"],
          ["End user lands on Connect URL", "app.unipost.dev/connect/{platform}", "Your brand (logo + color)"],
          ["End user clicks Authorize", "Platform OAuth (e.g. linkedin.com)", "Your App (name + icon)"],
          ["End user lands back on your product", "Your return_url", "Your brand"],
        ]}
      />
      <p>No UniPost login page, no UniPost account creation. The Connect URL authenticates via the <code>session_id + state</code> pair it carries; whoever holds the URL is treated as the authorized end user.</p>

      <h2 id="completion">Knowing when the flow completes</h2>
      <p>Two options. Pick one — don&apos;t do both.</p>
      <DocsTable
        columns={["Method", "Good for", "Trade-off"]}
        rows={[
          [
            "Webhook",
            "Production — server-to-server, instant, handles browser back / close edge cases",
            "Requires you to run a public HTTPS receiver",
          ],
          [
            "Polling",
            "Local dev, quick scripts, and products without a webhook receiver yet",
            "3-second lag, wastes API calls if user abandons the flow",
          ],
        ]}
      />
      <p>Polling example: <ApiInlineLink endpoint="GET /v1/connect/sessions/{id}" href="/docs/api/connect/sessions" />. Webhook payload format: <Link href="/docs/api/webhooks">see the webhooks reference</Link>.</p>

      <h2 id="troubleshooting">Troubleshooting</h2>
      <DocsTable
        columns={["Symptom", "Likely cause", "Fix"]}
        rows={[
          [
            "Platform consent says “UniPost”, not my App name",
            "Credentials for this platform weren’t uploaded to this workspace",
            "Step 3 — POST platform-credentials with your client_id + client_secret",
          ],
          [
            "“Redirect URI mismatch” error from the platform",
            "Platform App’s allowed redirect list doesn’t include UniPost’s callback",
            "Step 2 — add /v1/oauth/callback/{platform} to the platform App",
          ],
          [
            "End user sees a UniPost login page before the Connect page",
            "Deployment is older than the /connect allowlist fix",
            "Report to UniPost support with your dashboard host",
          ],
          [
            "“App not approved for public use” on Meta / TikTok",
            "Your platform App hasn’t completed App Review",
            "Complete the platform’s App Review — UniPost can’t shortcut this",
          ],
          [
            "Hosted Connect page still shows UniPost branding",
            "Profile branding fields are empty or point at the wrong profile",
            "Step 4 — PATCH branding, verify via GET on the same profile",
          ],
        ]}
      />

      <h2 id="what-next">What to add next</h2>
      <DocsTable
        columns={["Next capability", "When to add it", "Docs path"]}
        rows={[
          [
            "Platform Credentials reference",
            "Field-by-field details for uploading your OAuth apps",
            <Link key="cr" href="/docs/api/white-label/credentials">API References → White-label → Platform Credentials</Link>,
          ],
          [
            "Profile Branding reference",
            "Validation rules for logo, display name, and color",
            <Link key="br" href="/docs/api/white-label/branding">API References → White-label → Profile Branding</Link>,
          ],
          [
            "Connect Sessions reference",
            "When you need every field + status the sessions API exposes",
            <Link key="cs" href="/docs/api/connect/sessions">API References → Connect Sessions</Link>,
          ],
          [
            "Webhooks",
            "Before production — replace polling with push delivery",
            <Link key="wh" href="/docs/api/webhooks">API References → Webhooks</Link>,
          ],
          [
            "Managed Users",
            "When you need to query which end users are connected to which accounts",
            <Link key="mu" href="/docs/api/users">API References → Managed Users</Link>,
          ],
          [
            "Platform guides",
            "Per-platform content rules, caption limits, supported media",
            <Link key="pl" href="/docs/platforms">Platforms</Link>,
          ],
        ]}
      />
    </DocsPage>
  );
}
