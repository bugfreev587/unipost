import Link from "next/link";
import { DocsPage, DocsTable } from "../../../_components/docs-shell";

export default function BrandingPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Profile Branding"
      lead="Set the logo, display name, and primary color that render on the UniPost-hosted Connect page. This is how you make the OAuth onboarding flow look like your product instead of UniPost."
    >
      <h2 id="overview">Where the fields appear</h2>
      <p>The hosted Connect page at <code>app.unipost.dev/connect/{"{platform}"}</code> reads these three values at render time — there&rsquo;s no cache layer, so a PATCH is live on the very next page load. See the <Link href="/docs/white-label">white-label guide</Link> for the full setup flow that combines branding with platform credentials and Connect sessions.</p>
      <DocsTable
        columns={["Field", "Where it appears on the Connect page"]}
        rows={[
          ["branding_logo_url", "Top-left mark"],
          ["branding_display_name", "Page title + tab name"],
          ["branding_primary_color", "Primary button and accent color"],
        ]}
      />

      <h2 id="update">Update branding</h2>
      <p>PATCH is partial — send only the fields you want to change. Each field accepts an empty string to unset (falls back to UniPost defaults).</p>
      <p>SDK support for profile branding management is coming soon. For now, configure this endpoint from the dashboard or call the REST route directly.</p>

      <h2 id="fields">Field validation</h2>
      <DocsTable
        columns={["Field", "Rule", "Reason"]}
        rows={[
          [
            "branding_logo_url",
            "HTTPS only; common image MIME types; ≤ 2 KB URL length",
            "The Connect page runs on HTTPS; mixed content would be blocked by the browser",
          ],
          [
            "branding_display_name",
            "≤ 60 characters",
            "Keeps the browser title + meta description readable across devices",
          ],
          [
            "branding_primary_color",
            "6-digit hex e.g. #10b981",
            "Matches the color-mix() and other CSS variables in the Connect page stylesheet",
          ],
        ]}
      />
      <p>A bad value returns <code>422 VALIDATION_ERROR</code> with a human-readable message naming the offending field. No partial updates happen on validation failure — the entire PATCH is atomic.</p>

      <h2 id="read">Read current branding</h2>
      <p>Branding fields come back on the standard profile read endpoint alongside <code>id</code>, <code>name</code>, <code>workspace_id</code>, and timestamps.</p>
      <p>Null values mean the field has never been set for this profile and the hosted Connect page will use UniPost&rsquo;s default appearance for that slot.</p>

      <h2 id="multi-profile">Multiple profiles per workspace</h2>
      <p>Branding is scoped per profile, not per workspace. If your workspace has multiple profiles (e.g., separate brands under one parent company), each profile can carry its own logo / name / color. Connect sessions created against a given profile use that profile&rsquo;s branding.</p>

      <h2 id="errors">Errors</h2>
      <DocsTable
        columns={["Status", "Code", "When"]}
        rows={[
          ["401", "UNAUTHORIZED", "Missing or invalid API key"],
          ["404", "NOT_FOUND", "Profile does not belong to the caller&rsquo;s workspace"],
          ["422", "VALIDATION_ERROR", "A field failed validation (URL scheme, length, or hex format)"],
        ]}
      />
    </DocsPage>
  );
}
