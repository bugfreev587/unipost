import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";
import { PLATFORM_CREDENTIAL_GUIDES, PLATFORM_CREDENTIAL_GUIDE_ORDER } from "./[platform]/_data";

const platformGuideHrefs = {
  meta: "/docs/platform-credentials/meta",
  linkedin: "/docs/platform-credentials/linkedin",
  tiktok: "/docs/platform-credentials/tiktok",
  youtube: "/docs/platform-credentials/youtube",
  twitter: "/docs/platform-credentials/twitter",
} as const;

const platformRows = PLATFORM_CREDENTIAL_GUIDE_ORDER.map((slug) => {
  const guide = PLATFORM_CREDENTIAL_GUIDES[slug];
  return [
    <Link key={slug} href={platformGuideHrefs[slug]}>{guide.name}</Link>,
    guide.dashboardCard,
    guide.portalName,
    guide.appReview,
  ];
});

export default function PlatformCredentialsOverviewPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Guide"
      title="Platform Credentials"
      lead="Save official platform developer app credentials when you want OAuth consent and API quota to come from your own app instead of UniPost's shared OAuth apps."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="pc-badges">
        <span className="pc-badge">Developer → Platform Credentials</span>
        <span className="pc-badge">Own app identity</span>
        <span className="pc-badge">Own platform quota</span>
        <span className="pc-badge">Separate from Hosted Connect</span>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Dashboard page", "Developer → Platform Credentials"],
          ["What it controls", "Which official platform OAuth app and quota UniPost uses for future OAuth flows"],
          ["What it does not control", <>The UniPost-hosted pre-OAuth page. Configure that in <Link href="/docs/white-label">Hosted Connect (White-label branding)</Link>.</>],
          ["Who can use it", "Qualified paid plans. Basic supports one shared custom platform slot across Platform Credentials and Hosted Connect branding; Growth, Team, and Enterprise support all supported platforms."],
          ["Shared fallback", "Connect Sessions can still use UniPost's shared OAuth app when allow_quickstart_creds is true and no workspace credential exists."],
        ]}
      />

      <h2 id="how-it-works">How it works</h2>
      <p className="pc-note">
        Platform Credentials are workspace-level developer app credentials:
        client ID plus client secret for the official platform. They are
        independent from Hosted Connect branding, but Basic uses one shared
        custom platform slot for both surfaces. Quickstart and branded Hosted
        Connect flows can use either credential source when the plan, selected
        platform slot, and session settings allow it.
      </p>
      <div className="docs-step-flow">
        <div className="docs-step-row">
          <div className="docs-step-number">1</div>
          <div>
            <div className="docs-step-title">Create the app in the platform developer portal</div>
            <div className="docs-step-copy">Use the guide for the platform you need and enable the product, scopes, or API access required by your flow.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">2</div>
          <div>
            <div className="docs-step-title">Allow-list UniPost callback URLs</div>
            <div className="docs-step-copy">Each OAuth platform has exact callback URLs. Copy them from the platform guide or from the dashboard row.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">3</div>
          <div>
            <div className="docs-step-title">Save the client ID and secret</div>
            <div className="docs-step-copy">Open <strong>Developer → Platform Credentials</strong> and paste the app credentials into the matching platform row.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">4</div>
          <div>
            <div className="docs-step-title">Create a fresh Connect Session</div>
            <div className="docs-step-copy">After saving credentials, start a new connection attempt so the OAuth flow uses the current platform app settings.</div>
          </div>
        </div>
      </div>

      <h2 id="platform-guides">Platform guides</h2>
      <DocsTable
        columns={["Platform", "Dashboard row", "Developer portal", "Review / approval"]}
        rows={platformRows}
      />

      <h2 id="related">Related docs</h2>
      <div className="pc-next">
        <Link href="/docs/white-label" className="pc-next-card">
          <div className="pc-next-kicker">Branding</div>
          <div className="pc-next-title">Hosted Connect (White-label branding)</div>
          <div className="pc-next-body">Configure the pre-OAuth page logo, name, color, and attribution.</div>
        </Link>
        <Link href="/docs/connect-sessions" className="pc-next-card">
          <div className="pc-next-kicker">OAuth flow</div>
          <div className="pc-next-title">Connect Sessions</div>
          <div className="pc-next-body">Create connection URLs and decide whether shared OAuth fallback is allowed.</div>
        </Link>
        <Link href="/docs/api/platform-credentials" className="pc-next-card">
          <div className="pc-next-kicker">API reference</div>
          <div className="pc-next-title">Platform Credentials endpoint</div>
          <div className="pc-next-body">Upload, list, and delete platform credentials from scripts or admin tooling.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.pc-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 26px}
.pc-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.pc-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:6px 0 14px;max-width:none}
.pc-next{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.pc-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.pc-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.pc-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.pc-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.pc-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
@media (max-width:960px){
  .pc-next{grid-template-columns:1fr}
}
`;
