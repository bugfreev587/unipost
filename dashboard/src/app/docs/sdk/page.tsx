import { DocsCode, DocsPage, DocsTable } from "../_components/docs-shell";

export default function SdkPage() {
  return (
    <DocsPage
      eyebrow="Get Started"
      title="SDKs"
      lead="UniPost should feel easy from whichever stack your team already uses. The docs structure follows the same flow across JavaScript, Python, Go, and raw HTTP."
    >
      <h2 id="supported">Supported SDKs</h2>
      <DocsTable
        columns={["SDK", "Best for", "Status"]}
        rows={[
          ["JavaScript / TypeScript", "Next.js apps, backend services, edge workers", "Primary"],
          ["Python", "Automation, agents, data workflows", "Primary"],
          ["Go", "Backend services and job runners", "Primary"],
          ["HTTP / cURL", "Direct integrations and debugging", "Always available"],
        ]}
      />

      <h2 id="shape">Recommended shape</h2>
      <p>Across every SDK, the recommended publishing shape is `platform_posts[]` plus `idempotency_key`. That keeps retries safe and lets you adapt caption tone per platform.</p>
      <DocsCode
        code={`client.socialPosts.create({
  platform_posts: [
    { account_id: "sa_twitter_123", caption: "Shipped 🚀" },
    { account_id: "sa_linkedin_456", caption: "We shipped a new release today." }
  ],
  idempotency_key: "release-2026-04-13-001"
})`}
      />

      <h2 id="what-next">What comes next</h2>
      <p>Once the SDK landing pages are approved, each SDK can get its own installation, auth, publish, validate, drafts, and analytics pages using the same structure as the platform guides.</p>
    </DocsPage>
  );
}
