import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";

const VALIDATE_SNIPPETS = [
  {
    label: "cURL",
    code: `curl -X POST https://api.unipost.dev/v1/social-posts/validate \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_twitter_1",
        "caption": "A platform-specific draft to validate"
      }
    ]
  }'`,
  },
  {
    label: "JavaScript",
    code: `const response = await fetch("https://api.unipost.dev/v1/social-posts/validate", {
  method: "POST",
  headers: {
    Authorization: "Bearer up_live_xxxx",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    platform_posts: [
      {
        account_id: "sa_twitter_1",
        caption: draftForX,
      },
    ],
  }),
});

const { data } = await response.json();`,
  },
];

export default function ValidatePage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Validate"
      lead="Validate is the recommended preflight endpoint for automation and AI workflows. It accepts the same request shape as `POST /v1/social-posts`, but performs checks without creating posts, charging quota, or touching downstream platforms."
    >
      <h2 id="why">Why use Validate</h2>
      <p>Validate is what makes AI-assisted or automated publishing safe. Instead of letting a model guess whether a caption, media mix, or platform-specific option will work, you can ask UniPost before anything is written or published.</p>

      <h2 id="request-shape">Request shape</h2>
      <p>The request body matches the publish endpoint. You can validate either the recommended <code>platform_posts[]</code> shape or the older <code>caption + account_ids</code> shape.</p>
      <DocsCodeTabs snippets={VALIDATE_SNIPPETS} />

      <h2 id="what-it-checks">What it checks</h2>
      <DocsTable
        columns={["Category", "Examples"]}
        rows={[
          ["Account state", "Unknown account, disconnected account, account outside workspace"],
          ["Content limits", "Caption too long, too many media items, unsupported media mix"],
          ["Platform support", "First comment not supported, threading not supported"],
          ["Scheduling", "Scheduled time in the past or too close to now"],
        ]}
      />

      <h2 id="response-shape">Response shape</h2>
      <p>The response is designed to tell your client what to fix, not just that something failed. Fatal issues should block publish. Non-fatal issues can be surfaced as warnings.</p>
      <DocsTable
        columns={["Field", "Meaning"]}
        rows={[
          ["ok", "Whether the request is safe to publish as-is"],
          ["errors[]", "Per-account or per-platform validation issues"],
          ["errors[].fatal", "Whether the issue should block publish"],
        ]}
      />
    </DocsPage>
  );
}
