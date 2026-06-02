import Link from "next/link";
import { DocsPage, DocsTable } from "../../_components/docs-shell";

export default function PlatformCredentialsPage() {
  return (
    <DocsPage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Platform Credentials" },
      ]}
      title="Platform Credentials"
      lead="Upload your own OAuth App client_id + client_secret for a platform. Platform Credentials control the platform OAuth app and quota source; Hosted Connect branding controls UniPost's pre-OAuth page."
    >
      <h2 id="when-to-use">When to use this endpoint</h2>
      <p>Use this endpoint when a workspace should connect accounts through its own platform developer app instead of UniPost&rsquo;s shared app. Platform Credentials are separate from Hosted Connect branding: branding changes the page UniPost hosts, while credentials change the app identity and quota the upstream platform sees. If you intentionally want to allow UniPost&rsquo;s shared Quickstart OAuth app for a specific session, create that session with <code>allow_quickstart_creds=true</code>. See the <Link href="/docs/platform-credentials">Platform Credentials guide</Link> for the full setup walkthrough.</p>

      <h2 id="paid-plan">Paid plan required</h2>
      <p>Basic and up can upload platform credentials. Basic supports 1 platform slot; Growth and Team support all supported platforms. Free and API workspaces can still use UniPost&apos;s shared Quickstart OAuth apps only when a session is created with <code>allow_quickstart_creds=true</code>.</p>

      <h2 id="create">Upload credentials</h2>
      <p>SDK support for platform credential management is coming soon. For now, upload credentials from the dashboard or call the REST endpoint directly.</p>
      <DocsTable
        columns={["Field", "Required", "Notes"]}
        rows={[
          ["platform", "Yes", <>One of <code>facebook</code>, <code>instagram</code>, <code>linkedin</code>, <code>pinterest</code>, <code>tiktok</code>, <code>youtube</code>, or <code>twitter</code>. The dashboard groups Instagram and Threads under the Meta credential card; Facebook Page has its own row so Connect sessions can require Facebook-specific workspace credentials. See <Link href="/docs/platforms#platform-names">available platform names</Link>.</>],
          ["client_id", "Yes", "Client / App ID from the platform developer portal"],
          ["client_secret", "Yes", "Stored encrypted at rest (AES-256-GCM). Never returned in any read endpoint."],
        ]}
      />
      <p>A successful upload replaces any previous credentials for the same platform in this workspace — uploading a second set for the same platform overwrites the first. On Basic, creating a second platform row is rejected until you upgrade.</p>

      <h2 id="list">List configured platforms</h2>
      <p>Returns one row per platform that has credentials stored. <code>client_secret</code> is never included — there is no read endpoint that exposes it.</p>
      <DocsTable
        columns={["Response field", "Notes"]}
        rows={[
          ["platform", "Which platform these credentials are for"],
          ["client_id", "The public App ID, safe to return"],
          ["created_at", "ISO timestamp of when the credentials were uploaded"],
          ["meta.total", "Total configured platform credential rows returned by the list endpoint"],
          ["meta.limit", "Applied list limit when the endpoint starts supporting partial reads"],
          ["request_id", "Request identifier for debugging and support"],
        ]}
      />

      <h2 id="delete">Remove credentials</h2>
      <p>Deletes the stored credentials for one platform. After a successful delete, future Connect sessions for that platform will fail validation unless they are explicitly created with <code>allow_quickstart_creds=true</code>. Existing already-connected accounts continue to publish using their stored tokens — deletion only affects <em>future</em> OAuth flows.</p>
      <p>Returns <code>204 No Content</code> on success; safe to call when no credentials exist for that platform.</p>

      <h2 id="errors">Errors</h2>
      <DocsTable
        columns={["Status", "Code", "When"]}
        rows={[
          ["401", "UNAUTHORIZED / unauthorized", "Missing or invalid API key / session"],
          ["402", "PLAN_FEATURE_NOT_AVAILABLE / plan_feature_not_available", "Workspace plan does not include platform credentials, or Basic has already used its 1 platform slot"],
          ["404", "NOT_FOUND / not_found", "Workspace does not belong to the caller"],
          ["422", "VALIDATION_ERROR / validation_error", "Missing platform, client_id, or client_secret"],
        ]}
      />

      <h2 id="auth-modes">Auth modes</h2>
      <DocsTable
        columns={["Mode", "Auth", "Use case"]}
        rows={[
          ["Workspace API key", "Bearer up_live_xxxx", "Programmatic onboarding (CI, admin scripts, customer integrations)"],
          ["Clerk session (Dashboard)", "Browser cookie", "Human uploading credentials through Developer → Platform Credentials"],
        ]}
      />
    </DocsPage>
  );
}
