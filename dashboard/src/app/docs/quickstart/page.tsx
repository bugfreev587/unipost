import { DocsCode, DocsPage } from "../_components/docs-shell";

const QUICKSTART_CODE = `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_twitter_123",
        "caption": "Shipping on every platform with one API."
      },
      {
        "account_id": "sa_linkedin_456",
        "caption": "We shipped a new release today. Here is what changed."
      }
    ],
    "idempotency_key": "launch-2026-04-13-001"
  }'`;

export default function QuickstartPage() {
  return (
    <DocsPage
      eyebrow="Get Started"
      title="Quickstart"
      lead="The shortest path through UniPost is: create an API key, connect an account, publish with `platform_posts[]`, and validate before you automate. This page is written for that path."
    >
      <h2 id="step-1">1. Create an API key</h2>
      <p>Create a workspace in UniPost, then generate an API key from the dashboard. Use test or production keys depending on your environment.</p>

      <h2 id="step-2">2. Connect an account</h2>
      <p>For your own workspace, connect accounts directly in the dashboard. For end-user onboarding, use Connect sessions and map them to your own `external_user_id`.</p>

      <h2 id="step-3">3. Publish your first post</h2>
      <p>The recommended request shape is `platform_posts[]`. It keeps each platform&apos;s copy, media, and options separate and works better for AI-generated content than the older `caption + account_ids` shape.</p>
      <DocsCode code={QUICKSTART_CODE} />

      <h2 id="step-4">4. Validate before publish</h2>
      <p>Use the Validate endpoint before auto-publishing. UniPost will catch platform-specific errors like caption limits, unsupported media combinations, and missing required fields.</p>

      <h2 id="step-5">5. Add drafts and analytics</h2>
      <p>Once your core publish path works, add drafts, preview links, and analytics so your product has both safety and feedback loops.</p>
    </DocsPage>
  );
}
