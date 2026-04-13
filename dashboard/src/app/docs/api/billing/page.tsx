import { DocsPage, DocsTable } from "../../_components/docs-shell";

export default function BillingPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Billing and Usage"
      lead="Billing and usage endpoints help your app understand plan state, publish usage, and workspace limits. This matters most when you are building a SaaS on top of UniPost and need to make plan-aware decisions in your own UI."
    >
      <h2 id="what-it-covers">What it covers</h2>
      <DocsTable
        columns={["Concept", "Why you need it"]}
        rows={[
          ["Plan", "Understand which features or limits apply to the workspace"],
          ["Usage", "Track how much of the current billing period has been consumed"],
          ["Warnings", "Surface approaching-limit or over-limit states in your product"],
        ]}
      />

      <h2 id="where-it-shows-up">Where billing shows up</h2>
      <p>Billing state appears both in dedicated billing endpoints and in response headers such as <code>X-UniPost-Usage</code> and <code>X-UniPost-Warning</code> on publish-related responses.</p>
    </DocsPage>
  );
}
