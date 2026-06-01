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
  allowQuickstartCreds: true,
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
    "allow_quickstart_creds": true
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
      lead="Configure the Hosted Connect profile your customers see before OAuth. Platform Credentials are separate: use them when you want the platform consent screen and quota to come from your own developer apps."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="wl-badges">
        <span className="wl-badge">Hosted Connect Profile</span>
        <span className="wl-badge">Branding</span>
        <span className="wl-badge">Multi-tenant</span>
        <span className="wl-badge">Credential-source agnostic</span>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Who is it for", "Developers and product teams onboarding customer accounts"],
          ["What white-label controls", "The UniPost-hosted Connect page: logo, display name, primary color, and attribution"],
          ["What it does not control", "The platform-owned OAuth consent screen. Configure Platform Credentials for that."],
          ["Who pays the platform API cost", "UniPost for shared OAuth credentials; you for workspace Platform Credentials"],
          ["Setup complexity", "Low for profile branding; platform credentials and app review are separate setup tracks"],
          ["Control level", "Basic: branded Hosted Connect profile with UniPost attribution. Growth+: optional attribution removal."],
          ["Best use case", "SaaS products where customers bring their own social accounts"],
          ["Tier", "Hosted Connect profile branding starts on Basic; shared OAuth fallback can still be used without branding"],
          ["Connect Sessions", <Link key="connect-sessions" href="/docs/connect-sessions">Same API as Quickstart Connect Sessions</Link>],
        ]}
      />

      <h2 id="hosted-connect-profile">Hosted Connect Profile</h2>
      <p className="wl-note">
        White-label is the Hosted Connect profile your end users see before they
        continue to Instagram, YouTube, TikTok, LinkedIn, or another platform.
        It is independent from Platform Credentials. You can combine either Hosted Connect profile with either credential source.
      </p>
      <DocsTable
        columns={["Dimension", "Default profile", "Branded Hosted Connect Profile"]}
        rows={[
          ["Connect page logo/name/color", "UniPost defaults", "Your product profile"],
          ["Hosted onboarding attribution", "Powered by UniPost shown", "Shown on Basic, optional on Growth / Team"],
          ["Platform consent screen", "Depends on credential source", "Depends on credential source"],
          ["Platform API rate limits", "Depends on credential source", "Depends on credential source"],
          ["Best for", "Fast setup and internal testing", "Customer-facing SaaS onboarding"],
        ]}
      />
      <p className="wl-note">
        Platform Credentials decide which OAuth app the platform sees. If you
        store approved credentials for a platform, UniPost uses your app and
        your platform quota. If you allow quickstart fallback and no workspace
        credentials exist, UniPost uses its shared app for that session.
      </p>
      <div className="wl-pick">
        <div className="wl-pick-card">
          <div className="wl-pick-kicker">Use default profile if</div>
          <ul className="wl-pick-list">
            <li>You are validating an internal flow</li>
            <li>Your users do not need custom Connect-page branding yet</li>
            <li>You want the shortest route to a working connection</li>
          </ul>
        </div>
        <div className="wl-pick-card wl-pick-card-accent">
          <div className="wl-pick-kicker">Use branded profile if</div>
          <ul className="wl-pick-list">
            <li>Your customers bring their own accounts</li>
            <li>The pre-OAuth page should look like your product</li>
            <li>You want a cleaner customer onboarding handoff</li>
          </ul>
        </div>
      </div>

      <h2 id="how-it-works">How it works</h2>
      <p className="wl-note">Treat Hosted Connect branding and Platform Credentials as separate setup jobs. Branding changes what UniPost hosts. Platform Credentials change which official platform app and quota are used during OAuth.</p>
      <div className="docs-step-flow">
        <div className="docs-step-row">
          <div className="docs-step-number">1</div>
          <div>
            <div className="docs-step-title">Set the Hosted Connect profile</div>
            <div className="docs-step-copy">In Developer → Hosted Connect, configure logo, display name, primary color, and attribution for the page UniPost shows before OAuth.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">2</div>
          <div>
            <div className="docs-step-title">Choose the credential source</div>
            <div className="docs-step-copy">Use UniPost&apos;s shared app for quickstart fallback, or save your own Platform Credentials when you need your app identity and quota.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">3</div>
          <div>
            <div className="docs-step-title">Create the official platform app when needed</div>
            <div className="docs-step-copy">If you choose workspace credentials, create the OAuth app in the platform&apos;s developer portal and enable the products or scopes your integration needs.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">4</div>
          <div>
            <div className="docs-step-title">Allow-list UniPost&apos;s callback URL</div>
            <div className="docs-step-copy">Each OAuth platform has its own exact callback path. Copy it verbatim from UniPost&apos;s Platform Credentials page or the platform-specific guide.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">5</div>
          <div>
            <div className="docs-step-title">Upload Platform Credentials to UniPost</div>
            <div className="docs-step-copy">Save your client ID and secret in Developer → Platform Credentials. Basic supports one platform credential slot; Growth and Team support all supported platforms.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">6</div>
          <div>
            <div className="docs-step-title">Complete platform verification or app review</div>
            <div className="docs-step-copy">Some platforms let you test before review, but public rollout can depend on approval for the app, scopes, or brand.</div>
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

      <h2 id="supported-platforms">Supported Platform Credentials</h2>
      <DocsTable
        columns={["Platform", "Workspace credentials", "Developer portal", "App review"]}
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
      <p className="wl-note">These pages are written for workspace-owned Platform Credentials: where to create the app, which callback URL to allow-list, what to paste into UniPost, and how to know the setup is actually done.</p>
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

      <h2 id="capabilities">Capabilities</h2>
      <DocsTable
        columns={["Capability", "Default profile + shared app", "Branded profile + workspace credentials"]}
        rows={[
          ["Publishing to connected accounts", "Yes", "Yes"],
          ["Scheduling", "Yes", "Yes"],
          ["Analytics + webhooks", "Yes", "Yes"],
          ["Your brand on Connect page", "No", "Yes, with Hosted Connect profile"],
          ["Your app on platform consent", "No", "Yes, with Platform Credentials"],
          ["Remove Powered by UniPost", "No", "Growth / Team"],
          ["Onboard customers at scale", "Yes via Connect Sessions", "Yes via Connect Sessions"],
          ["Own platform rate-limit tier", "No", "Yes, when workspace credentials are used"],
          ["`external_user_id` mapping", "Yes via Connect Sessions", "Yes"],
        ]}
      />

      <h2 id="cost-model">Cost model</h2>
      <DocsTable
        columns={["Item", "Quickstart", "White-label"]}
        rows={[
          ["Platform API fees", "Included in UniPost plan quota", "Billed directly to you by each platform"],
          ["Hosted Connect branding", "Default UniPost profile", "Plan-gated branded profile"],
          ["Rate-limit headroom", "Shared pool across quickstart fallback users", "Yours alone when workspace credentials are used"],
          ["Upgrade cost trigger", "Hit UniPost plan quota → upgrade UniPost", "Hit platform tier → upgrade with platform"],
        ]}
      />

      <h2 id="setup-checklist">Setup checklist</h2>
      <p className="wl-note">Gather these before you start — it avoids round-trips during setup.</p>
      <ul className="docs-checklist docs-checklist-2col">
        <li>Developer accounts on each platform you plan to support</li>
        <li>OAuth <code>client_id</code> + <code>client_secret</code> per platform</li>
        <li>UniPost&apos;s callback URL allow-listed on each platform app</li>
        <li>A logo image, brand display name, and hex color for Hosted Connect</li>
        <li>A stable <code>external_user_id</code> format owned by your backend</li>
        <li>A <code>return_url</code> on your product to land users after Connect</li>
      </ul>

      <h2 id="three-layers">What you actually configure</h2>
      <DocsTable
        columns={["Layer", "What it controls", "Default if unset"]}
        rows={[
          ["Hosted Connect Profile", "Logo, display name, primary color, and attribution on the UniPost-hosted pre-OAuth page", "UniPost's default look"],
          ["Platform Credentials", "Which platform app and quota are used when the end user authorizes", "UniPost's shared app if quickstart fallback is allowed"],
          ["Connect URL", "The public URL your backend hands to the end user", "Always required — nothing to fall back to"],
        ]}
      />
      <p className="wl-note">
        This is the core mental model: the Hosted Connect Profile changes the
        page UniPost controls; Platform Credentials change the OAuth app the
        platform controls.
      </p>

      <h2 id="api-examples">API examples</h2>

      <h3 id="create-session">Create a Connect session</h3>
      <p className="wl-note">
        Start the branded OAuth flow for one end user. For the mode-neutral flow,
        see the <Link href="/docs/connect-sessions">Connect Sessions guide</Link>.
      </p>
      <DocsCodeTabs snippets={CREATE_SESSION_SNIPPETS} />

      <h3 id="upload-credentials">Upload platform credentials</h3>
      <p className="wl-note">Workspace admins only. Push once per platform when you want your own OAuth app and quota.</p>
      <DocsCode code={UPLOAD_CREDS_SNIPPET} language="http" />

      <h3 id="patch-branding">Set profile branding</h3>
      <p className="wl-note">Pulled at Connect-page render time — no cache in between.</p>
      <DocsCode code={PATCH_BRANDING_SNIPPET} language="http" />

      <h2 id="limitations">Limitations &amp; notes</h2>
      <DocsTable
        columns={["Limitation", "Reason"]}
        rows={[
          ["Platform developer apps are separate", "Hosted Connect branding does not create or approve an official platform app"],
          ["Platform API rate limits depend on credential source", "Shared fallback uses UniPost's app; workspace credentials use your app"],
          ["Some platforms require app review", "Meta and TikTok require review before broad public use — UniPost can't shortcut this"],
          ["Bluesky is not applicable", "Bluesky uses app passwords, not OAuth apps"],
          ["Plan-gated controls", "Branding, attribution removal, and credential slot count depend on your UniPost plan"],
        ]}
      />

      <h2 id="next-steps">Next steps</h2>
      <div className="wl-next">
        <Link href="/docs/connect-sessions" className="wl-next-card">
          <div className="wl-next-kicker">Hosted onboarding</div>
          <div className="wl-next-title">Connect sessions</div>
          <div className="wl-next-body">Mode-neutral guide for customer account onboarding and credential source selection.</div>
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
          <div className="wl-next-body">Content rules, media limits, and post behavior after account connection is done.</div>
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
