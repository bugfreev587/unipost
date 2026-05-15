import Link from "next/link";
import { DocsCode, DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";

const CREATE_SESSION_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const session = await client.connect.createSession({
  platform: "linkedin",
  externalUserId: "acme_user_42",
  externalUserEmail: "user42@customer.com",
  returnUrl: "https://app.acme.com/integrations/linkedin/done",
  allowQuickstartCreds: false,
});

console.log(session.url);`,
  },
  {
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/connect/sessions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "linkedin",
    "external_user_id": "acme_user_42",
    "external_user_email": "user42@customer.com",
    "return_url": "https://app.acme.com/integrations/linkedin/done",
    "allow_quickstart_creds": false
  }'`,
  },
];

const UPLOAD_CREDS_SNIPPET = `POST /v1/platform-credentials

{
  "platform": "linkedin",
  "client_id": "your-linkedin-client-id",
  "client_secret": "your-linkedin-client-secret"
}`;

const PATCH_BRANDING_SNIPPET = `PATCH /v1/profiles/{profile_id}

{
  "branding_logo_url": "https://cdn.acme.com/logo.svg",
  "branding_display_name": "Acme",
  "branding_primary_color": "#10b981"
}`;

export default function WhiteLabelPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Guide"
      title="White-label Mode"
      lead="Run UniPost's Connect flow against your own OAuth apps, branded as your product. Your customers connect their accounts without ever seeing UniPost."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="wl-badges">
        <span className="wl-badge">Full control</span>
        <span className="wl-badge">API-based</span>
        <span className="wl-badge">Multi-tenant</span>
        <span className="wl-badge">Advanced setup</span>
        <span className="wl-badge wl-badge-paid">Paid plans only</span>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Who is it for", "Developers and product teams onboarding customer accounts"],
          ["Who pays the platform API cost", "You — your OAuth app, your platform tier"],
          ["Setup complexity", "Medium — one-time per platform, ~30 min"],
          ["Control level", "Basic: 1 branded platform with UniPost attribution. Growth+: full multi-platform control with optional attribution removal."],
          ["Best use case", "SaaS products where customers bring their own social accounts"],
          ["Tier", "Basic and up — Quickstart is free"],
        ]}
      />

      <h2 id="quickstart-vs-white-label">Quickstart vs White-label</h2>
      <p className="wl-note">The single question this page exists to answer: which one should you pick?</p>
      <DocsTable
        columns={["Dimension", "Quickstart", "White-label"]}
        rows={[
          ["Setup time", "Minutes", "~30 min per platform (one-time)"],
          ["OAuth app", "UniPost-managed", "Your own — uploaded per workspace"],
          ["Platform consent screen", "Shows “UniPost”", "Shows your app name + logo"],
          ["Connect page branding", "UniPost's default", "Your logo, name, and brand color (Basic+)"],
          ["Hosted onboarding attribution", "Always shown", "Shown on Basic, optional on Growth / Team"],
          ["Platform API rate limits", "Shared across all UniPost workspaces", "Your own — scales with your tier"],
          ["Platform API cost", "Included in UniPost plan", "Billed to you by each platform"],
          ["Best for", "Your own accounts, prototypes, internal tools", "Customer-facing SaaS onboarding"],
          ["Required on free tier", "✓ Available", "— Paid plans only"],
        ]}
      />
      <p className="wl-note">
        In practical terms: Quickstart uses UniPost&apos;s own OAuth apps, so your end users will see <code>UniPost</code> on the platform consent screen. White-label uses your uploaded platform credentials instead, so the same end users see your app name and brand assets on the consent screen once that platform app is configured and approved. Connect sessions default to this white-label behavior; only sessions created with <code>allow_quickstart_creds=true</code> may fall back to UniPost&apos;s shared OAuth apps. Basic unlocks that for one platform while still keeping <code>Powered by UniPost</code> on the hosted Connect page; Growth and Team remove the one-platform cap and can hide that attribution.
      </p>
      <div className="wl-pick">
        <div className="wl-pick-card">
          <div className="wl-pick-kicker">Pick Quickstart if</div>
          <ul className="wl-pick-list">
            <li>You publish to social accounts your team owns</li>
            <li>You&apos;re validating a prototype or internal tool</li>
            <li>You don&apos;t need your brand on the Connect surface</li>
          </ul>
        </div>
        <div className="wl-pick-card wl-pick-card-accent">
          <div className="wl-pick-kicker">Pick White-label if</div>
          <ul className="wl-pick-list">
            <li>Your customers bring their own accounts</li>
            <li>Your customers shouldn&apos;t know UniPost exists</li>
            <li>You need your own platform API rate limits</li>
          </ul>
        </div>
      </div>

      <h2 id="how-it-works">How it works</h2>
      <p className="wl-note">White-label is not just &quot;paste credentials into UniPost&quot;. The real flow starts in each platform&apos;s official developer console, then comes back into UniPost after your app credentials and platform review path are in place.</p>
      <div className="docs-step-flow">
        <div className="docs-step-row">
          <div className="docs-step-number">1</div>
          <div>
            <div className="docs-step-title">Create the official platform app</div>
            <div className="docs-step-copy">In the platform&apos;s own developer portal, create the OAuth app your customers will authorize against. This is where the eventual consent-screen brand comes from.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">2</div>
          <div>
            <div className="docs-step-title">Generate client ID and client secret</div>
            <div className="docs-step-copy">Finish the platform-side setup needed to mint the real OAuth credentials. Depending on the platform, this can involve enabling APIs, creating OAuth clients, and choosing the right app type.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">3</div>
          <div>
            <div className="docs-step-title">Whitelist UniPost&apos;s callback URL</div>
            <div className="docs-step-copy">Each platform has its own exact callback path. Use the platform-specific white-label guide below and copy the URL verbatim into that platform&apos;s developer console.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">4</div>
          <div>
            <div className="docs-step-title">Upload credentials to UniPost</div>
            <div className="docs-step-copy">Push your client ID and secret to UniPost. Basic supports one platform; Growth and Team support all supported platforms.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">5</div>
          <div>
            <div className="docs-step-title">Complete platform verification or app review</div>
            <div className="docs-step-copy">Some platforms let you test before review, but branded public rollout usually still depends on the platform approving your app, scopes, or brand. Plan for that review loop instead of treating it as an optional afterthought.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">6</div>
          <div>
            <div className="docs-step-title">Set profile branding</div>
            <div className="docs-step-copy">Logo, display name, and primary color on the hosted Connect page. Growth and Team can also hide the UniPost attribution footer.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">7</div>
          <div>
            <div className="docs-step-title">Create a session and publish</div>
            <div className="docs-step-copy">Call <code>POST /v1/connect/sessions</code>, send the end user to the returned URL, then publish via <code>POST /v1/posts</code>.</div>
          </div>
        </div>
      </div>

      <h2 id="supported-platforms">Supported platforms</h2>
      <DocsTable
        columns={["Platform", "White-label", "Developer portal", "App review"]}
        rows={[
          [
            <Link key="meta-guide" href="/docs/white-label/meta">Meta (Facebook + Instagram + Threads)</Link>,
            "Yes",
            <a key="meta" href="https://developers.facebook.com" target="_blank" rel="noreferrer noopener">developers.facebook.com</a>,
            "Required for public use",
          ],
          [
            <Link key="linkedin-guide" href="/docs/white-label/linkedin">LinkedIn</Link>,
            "Yes",
            <a key="li" href="https://www.linkedin.com/developers" target="_blank" rel="noreferrer noopener">linkedin.com/developers</a>,
            "Products auto-approve",
          ],
          [
            <Link key="tiktok-guide" href="/docs/white-label/tiktok">TikTok</Link>,
            "Yes",
            <a key="tt" href="https://developers.tiktok.com" target="_blank" rel="noreferrer noopener">developers.tiktok.com</a>,
            "Audit required",
          ],
          [
            "Pinterest",
            "Yes",
            <a key="pin" href="https://developers.pinterest.com" target="_blank" rel="noreferrer noopener">developers.pinterest.com</a>,
            "Production API access required for broad use",
          ],
          [
            <Link key="youtube-guide" href="/docs/white-label/youtube">YouTube</Link>,
            "Yes",
            <a key="yt" href="https://console.cloud.google.com" target="_blank" rel="noreferrer noopener">console.cloud.google.com</a>,
            "Scope verification recommended",
          ],
          [
            <Link key="twitter-guide" href="/docs/white-label/twitter">X / Twitter</Link>,
            "Yes",
            <a key="x" href="https://developer.x.com" target="_blank" rel="noreferrer noopener">developer.x.com</a>,
            "Tier-dependent",
          ],
          [
            "Bluesky",
            "No",
            "—",
            "Not applicable — no OAuth apps",
          ],
        ]}
      />

      <h2 id="platform-setup-guides">Platform setup guides</h2>
      <p className="wl-note">These pages are written from the white-label customer&apos;s point of view: where to create the app, which callback URL to allow-list, what to paste into UniPost, and how to know the setup is actually done.</p>
      <div className="wl-next">
        <Link href="/docs/white-label/meta" className="wl-next-card">
          <div className="wl-next-kicker">Meta</div>
          <div className="wl-next-title">Instagram, Threads, and Facebook</div>
          <div className="wl-next-body">One Meta app, multiple UniPost callback paths, and the quickest route to a working branded consent flow.</div>
        </Link>
        <Link href="/docs/white-label/linkedin" className="wl-next-card">
          <div className="wl-next-kicker">LinkedIn</div>
          <div className="wl-next-title">Fastest enterprise rollout</div>
          <div className="wl-next-body">A low-friction first white-label setup with exact redirect guidance and first-test criteria.</div>
        </Link>
        <Link href="/docs/white-label/tiktok" className="wl-next-card">
          <div className="wl-next-kicker">TikTok</div>
          <div className="wl-next-title">Creator onboarding setup</div>
          <div className="wl-next-body">How to get one creator-account smoke test working before you tackle broader review and rollout.</div>
        </Link>
        <Link href="/docs/white-label/youtube" className="wl-next-card">
          <div className="wl-next-kicker">YouTube</div>
          <div className="wl-next-title">Google Cloud checklist</div>
          <div className="wl-next-body">The exact OAuth pieces UniPost needs from Google Cloud, without extra Console wandering.</div>
        </Link>
        <Link href="/docs/white-label/twitter" className="wl-next-card">
          <div className="wl-next-kicker">X / Twitter</div>
          <div className="wl-next-title">Tier-aware app setup</div>
          <div className="wl-next-body">Callback wiring, credential storage, and the operational checkpoints that matter before launch.</div>
        </Link>
      </div>

      <h2 id="capabilities">Capabilities unlocked</h2>
      <DocsTable
        columns={["Capability", "Quickstart", "White-label"]}
        rows={[
          ["Publishing to connected accounts", "Yes", "Yes"],
          ["Scheduling", "Yes", "Yes"],
          ["Analytics + webhooks", "Yes", "Yes"],
          ["Your brand on Connect page", "No", "Yes (Basic+)"],
          ["Your app on platform consent", "No", "Yes (Basic+ for 1 platform, Growth+ for all supported platforms)"],
          ["Remove Powered by UniPost", "No", "Growth / Team only"],
          ["Onboard customers at scale", "No", "Yes"],
          ["Own platform rate-limit tier", "No", "Yes"],
          ["`external_user_id` mapping", "No", "Yes"],
        ]}
      />

      <h2 id="cost-model">Cost model</h2>
      <DocsTable
        columns={["Item", "Quickstart", "White-label"]}
        rows={[
          ["Platform API fees", "Absorbed by UniPost plan quota", "Billed directly to you by each platform"],
          ["UniPost plan", "Free tier available", "Paid plan required"],
          ["Rate-limit headroom", "Shared pool across all Quickstart users", "Yours alone — scales with your platform tier"],
          ["Upgrade cost trigger", "Hit plan quota → upgrade UniPost", "Hit platform tier → upgrade with platform"],
        ]}
      />

      <h2 id="setup-checklist">Setup checklist</h2>
      <p className="wl-note">Gather these before you start — it avoids round-trips during setup.</p>
      <ul className="docs-checklist docs-checklist-2col">
        <li>Developer accounts on each platform you plan to support</li>
        <li>OAuth <code>client_id</code> + <code>client_secret</code> per platform</li>
        <li>UniPost&apos;s callback URL allow-listed on each platform app</li>
        <li>A logo URL (HTTPS, ≤ 2 KB), brand display name, and hex color</li>
        <li>A stable <code>external_user_id</code> format owned by your backend</li>
        <li>A <code>return_url</code> on your product to land users after Connect</li>
      </ul>

      <h2 id="three-layers">What you actually configure</h2>
      <DocsTable
        columns={["Layer", "What it controls", "Default if unset"]}
        rows={[
          ["OAuth credentials", "Which platform app is used when the end user authorizes", "UniPost's global app — consent shows “UniPost”"],
          ["Profile branding", "Logo, display name, and primary color on the hosted Connect page", "UniPost's default look"],
          ["Connect URL", "The public URL your backend hands to the end user", "Always required — nothing to fall back to"],
        ]}
      />
      <p className="wl-note">
        This is the core mental model: if you never upload your own platform credentials, the flow falls back to UniPost&apos;s app and the consent screen shows <code>UniPost</code>. If you do upload your own approved credentials, the consent screen can show your app identity instead.
      </p>

      <h2 id="api-examples">API examples</h2>

      <h3 id="create-session">Create a Connect session</h3>
      <p className="wl-note">Start the branded OAuth flow for one end user.</p>
      <DocsCodeTabs snippets={CREATE_SESSION_SNIPPETS} />

      <h3 id="upload-credentials">Upload platform credentials</h3>
      <p className="wl-note">Workspace admins only. Push once per platform.</p>
      <DocsCode code={UPLOAD_CREDS_SNIPPET} language="http" />

      <h3 id="patch-branding">Set profile branding</h3>
      <p className="wl-note">Pulled at Connect-page render time — no cache in between.</p>
      <DocsCode code={PATCH_BRANDING_SNIPPET} language="http" />

      <h2 id="limitations">Limitations &amp; notes</h2>
      <DocsTable
        columns={["Limitation", "Reason"]}
        rows={[
          ["Requires platform developer apps", "Each platform issues OAuth credentials individually — UniPost can't proxy someone else's app"],
          ["Platform API rate limits are yours", "The platform enforces them against your client ID, not UniPost"],
          ["Some platforms require app review", "Meta and TikTok require review before public use — UniPost can't shortcut this"],
          ["Bluesky is not applicable", "Bluesky uses app passwords, not OAuth apps"],
          ["Paid plan required", "White-label is a paid feature — free tier users stay on Quickstart"],
        ]}
      />

      <h2 id="next-steps">Next steps</h2>
      <div className="wl-next">
        <Link href="/docs/api/connect/sessions" className="wl-next-card">
          <div className="wl-next-kicker">API reference</div>
          <div className="wl-next-title">Connect sessions</div>
          <div className="wl-next-body">Full request / response schema for <code>POST /v1/connect/sessions</code>.</div>
        </Link>
        <Link href="/docs/api/white-label/credentials" className="wl-next-card">
          <div className="wl-next-kicker">API reference</div>
          <div className="wl-next-title">Platform credentials</div>
          <div className="wl-next-body">Field-by-field details for uploading your OAuth apps.</div>
        </Link>
        <Link href="/docs/api/white-label/branding" className="wl-next-card">
          <div className="wl-next-kicker">API reference</div>
          <div className="wl-next-title">Profile branding</div>
          <div className="wl-next-body">Validation rules for logo, name, and color.</div>
        </Link>
        <Link href="/docs/platforms" className="wl-next-card">
          <div className="wl-next-kicker">Publishing details</div>
          <div className="wl-next-title">Platform guides</div>
          <div className="wl-next-body">Content rules, media limits, and post behavior after the white-label setup is done.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.wl-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 26px}
.wl-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.wl-badge-paid{background:color-mix(in srgb, var(--docs-link) 12%, var(--docs-bg-muted));border-color:color-mix(in srgb, var(--docs-link) 30%, var(--docs-border));color:var(--docs-link)}
.wl-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:6px 0 14px;max-width:none}
.wl-pick{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:18px 0 6px}
.wl-pick-card{padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated)}
.wl-pick-card-accent{border-color:color-mix(in srgb, var(--docs-link) 32%, var(--docs-border));background:color-mix(in srgb, var(--docs-link) 5%, var(--docs-bg-elevated))}
.wl-pick-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint);margin-bottom:10px}
.wl-pick-card-accent .wl-pick-kicker{color:var(--docs-link)}
.wl-pick-list{margin:0;padding-left:18px;color:var(--docs-text-soft)}
.wl-pick-list li{font-size:14px;line-height:1.65;margin-bottom:4px}
.wl-pick-list li:last-child{margin-bottom:0}
.wl-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.wl-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.wl-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.wl-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.wl-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.wl-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
.wl-next-body code{font-family:var(--docs-mono);font-size:12px}
@media (max-width:960px){
  .wl-pick{grid-template-columns:1fr}
  .wl-next{grid-template-columns:1fr}
}
`;
