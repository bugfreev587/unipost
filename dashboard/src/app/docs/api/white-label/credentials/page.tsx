import Link from "next/link";
import { DocsPage, DocsTable } from "../../../_components/docs-shell";

export default function PlatformCredentialsPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Platform Credentials"
      lead="Upload your own OAuth App client_id + client_secret for a platform. Once stored, every Connect session on the workspace for that platform runs against your App — the end user&rsquo;s consent screen shows your brand, not UniPost&rsquo;s."
    >
      <h2 id="when-to-use">When to use this endpoint</h2>
      <p>Call this once per platform during white-label onboarding. If you never upload credentials for a platform, Connect sessions for that platform fall back to UniPost&rsquo;s global App, and the OAuth consent page shows &ldquo;UniPost&rdquo; as the requesting application. See the <Link href="/docs/white-label">white-label guide</Link> for the full integration walkthrough.</p>

      <h2 id="paid-plan">Paid plan required</h2>
      <p>The Create endpoint rejects calls from free-plan workspaces with <code>403 FORBIDDEN</code>. Upgrade the workspace before attempting to upload credentials.</p>

      <h2 id="create">Upload credentials</h2>
      <p>SDK support for platform credential management is coming soon. For now, upload credentials from the dashboard or call the REST endpoint directly.</p>
      <DocsTable
        columns={["Field", "Required", "Notes"]}
        rows={[
          ["platform", "Yes", "One of linkedin, twitter, tiktok, youtube, meta"],
          ["client_id", "Yes", "Client / App ID from the platform developer portal"],
          ["client_secret", "Yes", "Stored encrypted at rest (AES-256-GCM). Never returned in any read endpoint."],
        ]}
      />
      <p>A successful upload replaces any previous credentials for the same platform in this workspace — uploading a second set overwrites the first.</p>

      <h2 id="list">List configured platforms</h2>
      <p>Returns one row per platform that has credentials stored. <code>client_secret</code> is never included — there is no read endpoint that exposes it.</p>
      <DocsTable
        columns={["Response field", "Notes"]}
        rows={[
          ["platform", "Which platform these credentials are for"],
          ["client_id", "The public App ID, safe to return"],
          ["created_at", "ISO timestamp of when the credentials were uploaded"],
        ]}
      />

      <h2 id="delete">Remove credentials</h2>
      <p>Deletes the stored credentials for one platform. After a successful delete, Connect sessions for that platform fall back to UniPost&rsquo;s global App. Existing already-connected accounts continue to publish using their stored Page tokens — deletion only affects <em>future</em> OAuth flows.</p>
      <p>Returns <code>204 No Content</code> on success; safe to call when no credentials exist for that platform.</p>

      <h2 id="errors">Errors</h2>
      <DocsTable
        columns={["Status", "Code", "When"]}
        rows={[
          ["401", "UNAUTHORIZED", "Missing or invalid API key / session"],
          ["403", "FORBIDDEN", "Workspace is on the free plan"],
          ["404", "NOT_FOUND", "Workspace does not belong to the caller"],
          ["422", "VALIDATION_ERROR", "Missing platform, client_id, or client_secret"],
        ]}
      />

      <h2 id="auth-modes">Auth modes</h2>
      <DocsTable
        columns={["Mode", "Auth", "Use case"]}
        rows={[
          ["Workspace API key", "Bearer up_live_xxxx", "Programmatic onboarding (CI, admin scripts, customer integrations)"],
          ["Clerk session (Dashboard)", "Browser cookie", "Human uploading creds through Accounts → White-label Credentials"],
        ]}
      />
    </DocsPage>
  );
}
