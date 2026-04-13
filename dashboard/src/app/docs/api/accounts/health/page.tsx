import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";

const HEALTH_SNIPPETS = [
  {
    label: "cURL",
    code: `curl https://api.unipost.dev/v1/social-accounts/sa_twitter_1/health \\
  -H "Authorization: Bearer up_live_xxxx"`,
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
