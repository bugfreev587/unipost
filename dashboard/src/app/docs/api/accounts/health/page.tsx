import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";

const HEALTH_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const health = await client.accounts.health("sa_twitter_1");
console.log(health.status);`,
  },
];

export default function AccountHealthPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Account Health"
      lead="Account Health gives you an operational signal for one connected account: whether it is healthy, whether it recently published successfully, and whether reconnect attention is needed."
    >
      <h2 id="what-it-returns">What it returns</h2>
      <DocsTable
        columns={["Field", "Meaning"]}
        rows={[
          ["status", "High-level state such as active or reconnect_required"],
          ["token_refreshed_at", "Last successful token refresh"],
          ["last_publish_at", "Most recent publish attempt"],
          ["last_publish_status", "Outcome of the most recent publish"],
          ["last_publish_error", "Most recent platform-side error if one exists"],
        ]}
      />

      <h2 id="example">Example</h2>
      <DocsCodeTabs snippets={HEALTH_SNIPPETS} />
    </DocsPage>
  );
}
