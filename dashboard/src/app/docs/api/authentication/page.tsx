import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";

const AUTH_SNIPPETS = [
  {
    label: "cURL",
    code: `curl https://api.unipost.dev/v1/social-accounts \\
  -H "Authorization: Bearer up_live_xxxx"`,
  },
  {
    label: "JavaScript",
    code: `const response = await fetch("https://api.unipost.dev/v1/social-accounts", {
  headers: {
    Authorization: "Bearer up_live_xxxx",
  },
});`,
  },
  {
    label: "Python",
    code: `import requests

response = requests.get(
    "https://api.unipost.dev/v1/social-accounts",
    headers={"Authorization": "Bearer up_live_xxxx"},
)`,
  },
];

export default function AuthenticationPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Authentication"
      lead="Every public UniPost API request uses Bearer authentication with your API key. If the key is missing, malformed, revoked, or from the wrong environment, the request fails before any business logic runs."
    >
      <h2 id="how-it-works">How it works</h2>
      <p>Send your UniPost API key in the <code>Authorization</code> header as <code>Bearer &lt;key&gt;</code>. Keys are workspace-scoped and can be generated from the UniPost dashboard.</p>
      <DocsCodeTabs snippets={AUTH_SNIPPETS} />

      <h2 id="key-types">Key types</h2>
      <DocsTable
        columns={["Prefix", "Environment", "Use case"]}
        rows={[
          ["up_live_", "Production", "Real publishing and production traffic"],
          ["up_test_", "Test", "Development, staging, and non-production integration work"],
        ]}
      />

      <h2 id="best-practices">Best practices</h2>
      <ul className="docs-list">
        <li>Store keys in environment variables, not in client-side source code or version control.</li>
        <li>Use different keys for staging and production to keep telemetry and quota usage separate.</li>
        <li>Rotate keys if a credential is exposed or if a teammate no longer needs access.</li>
      </ul>

      <h2 id="common-failures">Common failures</h2>
      <DocsTable
        columns={["HTTP", "Code", "What it usually means"]}
        rows={[
          ["401", "UNAUTHORIZED", "The API key is missing, invalid, or revoked"],
          ["403", "FORBIDDEN", "The key is valid but the workspace or plan does not allow the action"],
          ["429", "RATE_LIMITED", "The key exceeded request limits for the current time window"],
        ]}
      />
    </DocsPage>
  );
}
