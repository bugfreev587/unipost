import Link from "next/link";
import { DocsPage, DocsTable } from "../../../_components/docs-shell";

export default function BrandingPage() {
  return (
    <DocsPage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Profile Branding" },
      ]}
      title="Profile Branding"
      lead="Set the logo, display name, and primary color that render on the UniPost-hosted Connect page. This is how you make the OAuth onboarding flow look like your product instead of UniPost."
    >
      <h2 id="overview">Where the fields appear</h2>
      <p>The hosted Connect page at <code>app.unipost.dev/connect/{"{platform}"}</code> reads these three values at render time — there&rsquo;s no cache layer, so a PATCH is live on the very next page load. See the <Link href="/docs/white-label">Hosted Connect guide</Link> for the setup model that separates Hosted Connect branding, Platform Credentials, and Connect Sessions.</p>
      <p>Plan packaging matters here: Basic and up can brand the hosted Connect page; only Growth and Team can hide the <code>Powered by UniPost</code> footer.</p>
      <DocsTable
        columns={["Field", "Where it appears on the Connect page"]}
        rows={[
          ["branding_logo_url", "Top-left mark"],
          ["branding_display_name", "Page title + tab name"],
          ["branding_primary_color", "Primary button and accent color"],
          ["branding_hide_powered_by", "Hides the footer attribution (Growth / Team only)"],
        ]}
      />

      <h2 id="update">Update branding</h2>
      <p>PATCH is partial — send only the fields you want to change. Each field accepts an empty string to unset (falls back to UniPost defaults).</p>
      <p>The dashboard Hosted Connect page uses this endpoint for display name, color, and attribution. Logo files can be uploaded directly from the same dashboard page or through the multipart logo endpoint below.</p>

      <h2 id="logo-upload">Upload or remove a logo</h2>
      <p>Use the logo upload endpoint when you have a local PNG or JPG file and do not want to host it yourself. UniPost stores the asset in R2, writes the returned public URL to <code>branding_logo_url</code>, and keeps the internal storage key private. R2-managed profile branding objects are retained indefinitely; replacing or removing a logo only changes the profile pointer.</p>
      <DocsTable
        columns={["Endpoint", "Body", "Result"]}
        rows={[
          ["POST /v1/profiles/{id}/branding/logo", "multipart/form-data with file", "Stores PNG/JPG logo and returns the updated profile"],
          ["DELETE /v1/profiles/{id}/branding/logo", "No body", "Clears the profile logo fields; the R2 object is retained"],
        ]}
      />

      <h2 id="fields">Field validation</h2>
      <DocsTable
        columns={["Field", "Rule", "Reason"]}
        rows={[
          [
            "branding_logo_url",
            "Set by logo upload, or HTTPS URL when patched directly; direct URL max 512 chars",
            "The Connect page runs on HTTPS; uploaded assets are served from UniPost-managed R2",
          ],
          [
            "logo upload file",
            "PNG or JPG; ≤ 2 MB; image width and height each ≤ 4096 px",
            "Keeps hosted Connect lightweight and avoids oversized customer assets",
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
          ["401", "UNAUTHORIZED / unauthorized", "Missing or invalid API key"],
          ["402", "PLAN_FEATURE_NOT_AVAILABLE / plan_feature_not_available", "The workspace plan cannot brand hosted Connect"],
          ["404", "NOT_FOUND / not_found", "Profile does not belong to the caller&rsquo;s workspace"],
          ["413", "PAYLOAD_TOO_LARGE / payload_too_large", "Logo file is larger than 2 MB"],
          ["422", "VALIDATION_ERROR / validation_error", "A field failed validation (URL scheme, length, or hex format)"],
          ["503", "STORAGE_NOT_CONFIGURED / storage_not_configured", "Logo upload storage is temporarily unavailable"],
        ]}
      />
      <p>Public API errors also include a lowercase <code>error.normalized_code</code> alias and a top-level <code>request_id</code> for tracing.</p>
    </DocsPage>
  );
}
