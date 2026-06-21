import Link from "next/link";
import { DocsCode, DocsPage, DocsTable } from "../_components/docs-shell";

const PATCH_BRANDING_SNIPPET = `PATCH /v1/profiles/{profile_id}

{
  "branding_logo_url": "https://cdn.example.com/logo.svg",
  "branding_display_name": "Your Product",
  "branding_primary_color": "#10b981"
}`;

export default function WhiteLabelPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Guide"
      title="Hosted Connect (White-label branding)"
      lead="Configure the Hosted Connect Profile your customers see before platform OAuth. This page maps to Developer → Hosted Connect in the dashboard; Platform Credentials live in their own guide."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="wl-badges">
        <span className="wl-badge">Hosted Connect Profile</span>
        <span className="wl-badge">White-label branding</span>
        <span className="wl-badge">Dashboard setup</span>
        <span className="wl-badge">Credential-source agnostic</span>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Dashboard page", "Developer → Hosted Connect"],
          ["What it controls", "The UniPost-hosted pre-OAuth page: logo, display name, primary color, and attribution"],
          ["What it does not control", <>The official platform OAuth app, consent screen, app review, or quota. Configure those in <Link href="/docs/platform-credentials">Platform Credentials</Link>.</>],
          ["Credential source", "UniPost shared OAuth app or workspace Platform Credentials"],
          ["Plan shape", "Hosted Connect profile branding starts on Basic. Basic applies branding to one shared custom platform slot; Growth, Team, and Enterprise apply it to all supported platforms. Attribution removal depends on plan."],
          ["Best use case", "Customer-facing SaaS onboarding where the connection handoff should look like your product"],
        ]}
      />

      <h2 id="mental-model">Mental model</h2>
      <p className="wl-note">
        White-label branding is the Hosted Connect Profile your end users see
        before they continue to Instagram, YouTube, TikTok, LinkedIn, or another
        platform. It is independent from Platform Credentials. You can combine either Hosted Connect profile with either credential source; on Basic, both surfaces must use the same selected platform slot.
      </p>
      <DocsTable
        columns={["Layer", "Dashboard page", "What it changes"]}
        rows={[
          ["Hosted Connect Profile", "Developer → Hosted Connect", "Logo, display name, color, and Powered by UniPost attribution on the UniPost page"],
          ["Platform Credentials", "Developer → Platform Credentials", "Which official platform app and quota are used during OAuth"],
          ["Connect Sessions", "API", "The URL your backend gives an end user to start account connection"],
        ]}
      />

      <h2 id="dashboard-setup">Dashboard setup</h2>
      <div className="docs-step-flow">
        <div className="docs-step-row">
          <div className="docs-step-number">1</div>
          <div>
            <div className="docs-step-title">Open Hosted Connect</div>
            <div className="docs-step-copy">In the UniPost dashboard, open <strong>Developer → Hosted Connect</strong> for the workspace and profile you want customers to see.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">2</div>
          <div>
            <div className="docs-step-title">Choose the custom platform scope</div>
            <div className="docs-step-copy">On Basic, pick the one platform this workspace customizes. That same slot applies to Hosted Connect branding and Platform Credentials. Growth, Team, and Enterprise use all supported platforms.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">3</div>
          <div>
            <div className="docs-step-title">Set the display name</div>
            <div className="docs-step-copy">Use the product or tenant name that should appear on the UniPost-hosted Connect page before the platform OAuth screen.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">4</div>
          <div>
            <div className="docs-step-title">Upload the logo</div>
            <div className="docs-step-copy">Upload a PNG or JPG logo. UniPost stores it and renders it on the hosted Connect page for new sessions that use this profile.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">5</div>
          <div>
            <div className="docs-step-title">Choose the primary color</div>
            <div className="docs-step-copy">Pick the accent color used by the Hosted Connect page. Keep it close to the customer-facing product brand so the handoff feels familiar.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">6</div>
          <div>
            <div className="docs-step-title">Save and test a Connect Session</div>
            <div className="docs-step-copy">Create a Connect Session with this profile and open the returned URL. Confirm the pre-OAuth page shows the correct name, logo, and color.</div>
          </div>
        </div>
      </div>

      <h2 id="api-setup">API setup</h2>
      <p className="wl-note">
        The dashboard is the simplest path. If you manage profile branding from
        your own admin tooling, update the profile directly. These fields are
        read at render time by Hosted Connect.
      </p>
      <DocsCode code={PATCH_BRANDING_SNIPPET} language="http" />

      <h2 id="credential-source">Credential source</h2>
      <p className="wl-note">
        Hosted Connect branding does not decide quota ownership. If a workspace
        has approved Platform Credentials for the requested platform, UniPost
        can use that app and quota. If a session allows quickstart fallback and
        no workspace credentials exist, UniPost can use its shared OAuth app.
        On Basic, the requested platform must match the workspace&apos;s shared custom platform slot for branding or Platform Credentials to apply.
      </p>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Can a default Hosted Connect page use workspace Platform Credentials?", "Yes, when the workspace plan and custom platform slot allow that platform"],
          ["Can a branded Hosted Connect page use UniPost shared OAuth?", "Yes, when quickstart fallback is allowed and the requested platform is inside the plan's branding scope"],
          ["Where do I configure developer app client ID and secret?", <Link key="pc" href="/docs/platform-credentials">Platform Credentials</Link>],
          ["Where do I configure logo/name/color?", "Developer → Hosted Connect"],
        ]}
      />

      <h2 id="next-steps">Next steps</h2>
      <div className="wl-next">
        <Link href="/docs/platform-credentials" className="wl-next-card">
          <div className="wl-next-kicker">Developer apps</div>
          <div className="wl-next-title">Platform Credentials</div>
          <div className="wl-next-body">Set official platform client IDs and secrets when you want your own app identity and quota.</div>
        </Link>
        <Link href="/docs/connect-sessions" className="wl-next-card">
          <div className="wl-next-kicker">Account connection</div>
          <div className="wl-next-title">Connect Sessions</div>
          <div className="wl-next-body">Create hosted account connection URLs and choose how shared fallback behaves.</div>
        </Link>
        <Link href="/docs/api/white-label/branding" className="wl-next-card">
          <div className="wl-next-kicker">API reference</div>
          <div className="wl-next-title">Profile branding</div>
          <div className="wl-next-body">Validation rules for Hosted Connect logo, display name, and color.</div>
        </Link>
        <Link href="/docs/api/platform-credentials" className="wl-next-card">
          <div className="wl-next-kicker">API reference</div>
          <div className="wl-next-title">Platform Credentials endpoint</div>
          <div className="wl-next-body">Field-by-field behavior for storing and deleting platform OAuth credentials.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.wl-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 26px}
.wl-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.wl-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:6px 0 14px;max-width:none}
.wl-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.wl-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.wl-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.wl-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.wl-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.wl-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
.wl-next-body code{font-family:var(--docs-mono);font-size:12px}
@media (max-width:960px){
  .wl-next{grid-template-columns:1fr}
}
`;
