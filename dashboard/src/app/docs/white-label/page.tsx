import Link from "next/link";
import { DocsCode, DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

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
  {
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/connect/sessions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "linkedin",
    "external_user_id": "acme_user_42",
    "external_user_email": "user42@customer.com",
    "return_url": "https://app.acme.com/integrations/linkedin/done"
  }'`,
  },
];

const UPLOAD_CREDS_SNIPPET = `POST /v1/workspaces/{workspace_id}/platform-credentials

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
          ["Control level", "Full — your app name, your branding, your quotas"],
          ["Best use case", "SaaS products where customers bring their own social accounts"],
          ["Tier", "Paid plans only — Quickstart is free"],
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
          ["Connect page branding", "UniPost's default", "Your logo, name, and brand color"],
          ["Platform API rate limits", "Shared across all UniPost workspaces", "Your own — scales with your tier"],
          ["Platform API cost", "Included in UniPost plan", "Billed to you by each platform"],
          ["Best for", "Your own accounts, prototypes, internal tools", "Customer-facing SaaS onboarding"],
          ["Required on free tier", "✓ Available", "— Paid plans only"],
        ]}
      />
      <div className="wl-pick">
        <div className="wl-pick-card">
          <div className="wl-pick-kicker">Pick Quickstart if</div>
          <ul className="wl-pick-list">
            <li>You publish to social accounts your team owns</li>
            <li>You're validating a prototype or internal tool</li>
            <li>You don't need your brand on the Connect surface</li>
          </ul>
        </div>
        <div className="wl-pick-card wl-pick-card-accent">
          <div className="wl-pick-kicker">Pick White-label if</div>
          <ul className="wl-pick-list">
            <li>Your customers bring their own accounts</li>
            <li>Your customers shouldn't know UniPost exists</li>
            <li>You need your own platform API rate limits</li>
          </ul>
        </div>
      </div>

      <h2 id="how-it-works">How it works</h2>
      <div className="wl-flow">
        <div className="wl-flow-step">
          <div className="wl-flow-num">1</div>
          <div className="wl-flow-body">
            <div className="wl-flow-title">Register OAuth apps</div>
            <div className="wl-flow-sub">One app per platform, in each platform's developer portal.</div>
          </div>
        </div>
        <div className="wl-flow-step">
          <div className="wl-flow-num">2</div>
          <div className="wl-flow-body">
            <div className="wl-flow-title">Whitelist UniPost's redirect</div>
            <div className="wl-flow-sub">Add <code>https://api.unipost.dev/v1/oauth/callback/&#123;platform&#125;</code> as an authorized redirect.</div>
          </div>
        </div>
        <div className="wl-flow-step">
          <div className="wl-flow-num">3</div>
          <div className="wl-flow-body">
            <div className="wl-flow-title">Upload credentials to UniPost</div>
            <div className="wl-flow-sub">Push your client ID and secret to UniPost, once per platform, once per workspace.</div>
          </div>
        </div>
        <div className="wl-flow-step">
          <div className="wl-flow-num">4</div>
          <div className="wl-flow-body">
            <div className="wl-flow-title">Set profile branding</div>
            <div className="wl-flow-sub">Logo, display name, and primary color on the hosted Connect page.</div>
          </div>
        </div>
        <div className="wl-flow-step">
          <div className="wl-flow-num">5</div>
          <div className="wl-flow-body">
            <div className="wl-flow-title">Create a session and publish</div>
            <div className="wl-flow-sub">Call <code>POST /v1/connect/sessions</code>, send the end user to the returned URL, then publish via <code>POST /v1/posts</code>.</div>
          </div>
        </div>
      </div>

      <h2 id="supported-platforms">Supported platforms</h2>
      <DocsTable
        columns={["Platform", "White-label", "Developer portal", "App review"]}
        rows={[
          [
            "Meta (Facebook + Instagram + Threads)",
            "Yes",
            <a key="meta" href="https://developers.facebook.com" target="_blank" rel="noreferrer noopener">developers.facebook.com</a>,
            "Required for public use",
          ],
          [
            "LinkedIn",
            "Yes",
            <a key="li" href="https://www.linkedin.com/developers" target="_blank" rel="noreferrer noopener">linkedin.com/developers</a>,
            "Products auto-approve",
          ],
          [
            "TikTok",
            "Yes",
            <a key="tt" href="https://developers.tiktok.com" target="_blank" rel="noreferrer noopener">developers.tiktok.com</a>,
            "Audit required",
          ],
          [
            "YouTube",
            "Yes",
            <a key="yt" href="https://console.cloud.google.com" target="_blank" rel="noreferrer noopener">console.cloud.google.com</a>,
            "Scope verification recommended",
          ],
          [
            "X / Twitter",
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

      <h2 id="capabilities">Capabilities unlocked</h2>
      <DocsTable
        columns={["Capability", "Quickstart", "White-label"]}
        rows={[
          ["Publishing to connected accounts", "Yes", "Yes"],
          ["Scheduling", "Yes", "Yes"],
          ["Analytics + webhooks", "Yes", "Yes"],
          ["Your brand on Connect page", "No", "Yes"],
          ["Your app on platform consent", "No", "Yes"],
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
        <li>UniPost's callback URL allow-listed on each platform app</li>
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
          <div className="wl-next-kicker">Per-platform</div>
          <div className="wl-next-title">Platform guides</div>
          <div className="wl-next-body">Content rules, media limits, and connection modes per platform.</div>
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
.wl-flow{display:grid;grid-template-columns:1fr;gap:10px;margin:14px 0 6px}
.wl-flow-step{display:grid;grid-template-columns:36px 1fr;gap:14px;align-items:start;padding:14px 16px;border:1px solid var(--docs-border);border-radius:14px;background:var(--docs-bg-elevated)}
.wl-flow-num{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;border-radius:999px;background:color-mix(in srgb, var(--docs-link) 14%, var(--docs-bg-muted));color:var(--docs-link);font-size:13px;font-weight:700;border:1px solid color-mix(in srgb, var(--docs-link) 22%, var(--docs-border))}
.wl-flow-title{font-size:15px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text);margin-bottom:3px}
.wl-flow-sub{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
.wl-flow-sub code{font-family:var(--docs-mono);font-size:12px}
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
