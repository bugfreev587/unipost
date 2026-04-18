import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

const CONNECT_SNIPPETS = [
  {
    label: "cURL",
    code: `curl -X POST https://api.unipost.dev/v1/social-accounts/connect \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "bluesky",
    "credentials": {
      "handle": "alice.bsky.social",
      "app_password": "xxxx-xxxx-xxxx-xxxx"
    }
  }'`,
  },
  {
    label: "JavaScript",
    code: `const response = await fetch("https://api.unipost.dev/v1/social-accounts/connect", {
  method: "POST",
  headers: {
    Authorization: "Bearer up_live_xxxx",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    platform: "bluesky",
    credentials: {
      handle: "alice.bsky.social",
      app_password: "xxxx-xxxx-xxxx-xxxx",
    },
  }),
});

const { data } = await response.json();`,
  },
];

const LIST_SNIPPETS = [
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
});

const { data: accounts } = await response.json();
const accountId = accounts[0].id;`,
  },
];

const CREATE_POST_SNIPPETS = [
  {
    label: "cURL",
    code: `curl -X POST https://api.unipost.dev/v1/social-posts \\
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
  }'`,
  },
  {
    label: "JavaScript",
    code: `const response = await fetch("https://api.unipost.dev/v1/social-posts", {
  method: "POST",
  headers: {
    Authorization: "Bearer up_live_xxxx",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    platform_posts: [
      {
        account_id: "sa_twitter_123",
        caption: "Shipping on every platform with one API.",
      },
      {
        account_id: "sa_linkedin_456",
        caption: "We shipped a new release today. Here is what changed.",
      },
    ],
    idempotency_key: "launch-2026-04-13-001",
  }),
});

const { data } = await response.json();`,
  },
  {
    label: "Python",
    code: `import requests

response = requests.post(
    "https://api.unipost.dev/v1/social-posts",
    headers={
        "Authorization": "Bearer up_live_xxxx",
        "Content-Type": "application/json",
    },
    json={
        "platform_posts": [
            {
                "account_id": "sa_twitter_123",
                "caption": "Shipping on every platform with one API.",
            },
            {
                "account_id": "sa_linkedin_456",
                "caption": "We shipped a new release today. Here is what changed.",
            },
        ],
        "idempotency_key": "launch-2026-04-13-001",
    },
)`,
  },
];

const VALIDATE_SNIPPETS = [
  {
    label: "cURL",
    code: `curl -X POST https://api.unipost.dev/v1/social-posts/validate \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_twitter_123",
        "caption": "This is a draft to validate before publish"
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
        account_id: "sa_twitter_123",
        caption: draftForX,
      },
    ],
  }),
});

const { data } = await response.json();`,
  },
];

export default function QuickstartPage() {
  return (
    <DocsPage
      eyebrow="Get Started"
      title="Quickstart"
      lead="This is the shortest useful path through UniPost: create an API key, connect an account, fetch its account ID, publish with `platform_posts[]`, and validate before you automate. Follow this page when you want a real integration working fast."
    >
      <h2 id="outcome">What you will finish with</h2>
      <p>UniPost is most useful when you think of it as one publishing layer with three responsibilities: connect accounts, publish content with platform-aware payloads, and read back operational or analytics signals.</p>
      <DocsTable
        columns={["Step", "Why it matters"]}
        rows={[
          ["Create an API key", "Establish auth for every request"],
          ["Connect an account", "Get a valid `social_account_id`"],
          ["List accounts", "Verify the account exists and capture its ID"],
          ["Publish with `platform_posts[]`", "Use the recommended shape for per-platform copy"],
          ["Validate before automation", "Catch platform errors before publish"],
        ]}
      />

      <h2 id="before-you-start">Before you start</h2>
      <ul className="docs-step-list">
        <li>You need a UniPost workspace and an API key from the dashboard.</li>
        <li>This guide assumes you are connecting a team-owned account directly. If customers connect their own accounts, use Connect sessions instead.</li>
        <li>The examples below use Bluesky because it is the shortest direct connect path, but the publish shape applies across platforms.</li>
      </ul>

      <h2 id="step-1">1. Create an API key</h2>
      <p>Create a workspace in UniPost, then generate an API key from the dashboard. Production keys start with <code>up_live_</code>. Test keys start with <code>up_test_</code>.</p>

      <h2 id="step-2">2. Connect an account</h2>
      <p>For your own team-owned accounts, connect directly. For customer-owned accounts, switch to Connect sessions instead. The example below uses Bluesky because it gives you the fastest working path.</p>
      <DocsCodeTabs snippets={CONNECT_SNIPPETS} />

      <h2 id="step-3">3. List accounts and capture the ID</h2>
      <p>After connecting, list the workspace&apos;s social accounts and copy the one you want to publish to. Every publish request ultimately needs a UniPost account ID, not a platform username.</p>
      <DocsCodeTabs snippets={LIST_SNIPPETS} />

      <h2 id="step-4">4. Publish your first post</h2>
      <p>The recommended request shape is <code>platform_posts[]</code>. It keeps each platform&apos;s caption, media, and options separate, and it works better for AI-generated content than the older <code>caption + account_ids</code> shape.</p>
      <DocsCodeTabs snippets={CREATE_POST_SNIPPETS} />
      <p>Add <code>idempotency_key</code> from day one. If you retry the same request within 24 hours, UniPost returns the original response instead of double-posting.</p>

      <h2 id="step-5">5. Validate before publish</h2>
      <p>Before any automated publish path, call Validate with the same body you plan to send to <ApiInlineLink endpoint="POST /v1/social-posts" />. UniPost catches caption overages, unsupported media combinations, and platform-specific field conflicts before the request writes anything.</p>
      <DocsCodeTabs snippets={VALIDATE_SNIPPETS} />

      <h2 id="what-next">What to add next</h2>
      <DocsTable
        columns={["Next capability", "When to add it", "Docs path"]}
        rows={[
          ["Drafts + preview links", "When a human should review before publish", "API Reference → Drafts / Preview Links"],
          ["Connect sessions", "When your customers connect their own end-user accounts", "API References → Connect Sessions"],
          ["Analytics", "When you need reporting or agent feedback loops", "API References → Analytics"],
          ["Platform guides", "When you need exact per-platform content rules", "Platforms"],
        ]}
      />
    </DocsPage>
  );
}
