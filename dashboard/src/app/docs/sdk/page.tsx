import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";

const INSTALL_SNIPPETS = [
  { label: "JavaScript", code: `npm install @unipost/sdk` },
  { label: "Python", code: `pip install unipost` },
  { label: "Go", code: `go get github.com/unipost-dev/sdk-go` },
];

const INIT_SNIPPETS = [
  {
    label: "JavaScript",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});`,
  },
  {
    label: "Python",
    code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])`,
  },
  {
    label: "Go",
    code: `package main

import (
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

client := unipost.NewClient(
  unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
)`,
  },
];

const LIST_ACCOUNTS_SNIPPETS = [
  {
    label: "JavaScript",
    code: `const { data: accounts } = await client.accounts.list();

const twitterAccounts = await client.accounts.list({
  platform: "twitter",
});`,
  },
  {
    label: "Python",
    code: `accounts = client.accounts.list()
twitter_accounts = client.accounts.list(platform="twitter")`,
  },
  {
    label: "Go",
    code: `accounts, err := client.Accounts.List(ctx, nil)
if err != nil {
  return err
}`,
  },
];

const CREATE_POST_SNIPPETS = [
  {
    label: "JavaScript",
    code: `const post = await client.posts.create({
  platformPosts: [
    {
      accountId: "sa_twitter_123",
      caption: "Short version for X",
    },
    {
      accountId: "sa_linkedin_456",
      caption: "Longer version for LinkedIn with more context.",
    },
  ],
  idempotencyKey: "launch-2026-04-13-001",
});`,
  },
  {
    label: "Python",
    code: `post = client.posts.create(
  platform_posts=[
    {
      "account_id": "sa_twitter_123",
      "caption": "Short version for X",
    },
    {
      "account_id": "sa_linkedin_456",
      "caption": "Longer version for LinkedIn with more context.",
    },
  ],
  idempotency_key="launch-2026-04-13-001",
)`,
  },
  {
    label: "Go",
    code: `post, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
  PlatformPosts: []unipost.PlatformPost{
    {
      AccountID: "sa_twitter_123",
      Caption:   "Short version for X",
    },
    {
      AccountID: "sa_linkedin_456",
      Caption:   "Longer version for LinkedIn with more context.",
    },
  },
  IdempotencyKey: "launch-2026-04-13-001",
})`,
  },
];

const VALIDATE_SNIPPETS = [
  {
    label: "JavaScript",
    code: `const result = await client.posts.validate({
  platformPosts: [
    {
      accountId: "sa_twitter_123",
      caption: draftForX,
    },
  ],
});

if (!result.valid) {
  console.log(result.issues);
}`,
  },
  {
    label: "Python",
    code: `result = client.posts.validate(
  platform_posts=[
    {
      "account_id": "sa_twitter_123",
      "caption": draft_for_x,
    }
  ]
)

if not result["valid"]:
  print(result["issues"])`,
  },
  {
    label: "Go",
    code: `validation, err := client.Posts.Validate(ctx, &unipost.ValidatePostParams{
  PlatformPosts: []unipost.PlatformPost{
    {
      AccountID: "sa_twitter_123",
      Caption:   draftForX,
    },
  },
})
if err != nil {
  return err
}`,
  },
];

const DRAFT_SNIPPETS = [
  {
    label: "JavaScript",
    code: `const draft = await client.posts.create({
  accountIds: ["sa_twitter_123"],
  caption: "Work in progress",
  status: "draft",
});

await client.posts.publish(draft.id);`,
  },
  {
    label: "Python",
    code: `draft = client.posts.create(
  account_ids=["sa_twitter_123"],
  caption="Work in progress",
  status="draft",
)

client.posts.publish(draft.id)`,
  },
  {
    label: "Go",
    code: `draft, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
  AccountIDs: []string{"sa_twitter_123"},
  Caption:    "Work in progress",
  Status:     "draft",
})

_, err = client.Posts.Publish(ctx, draft.ID)`,
  },
];

const ANALYTICS_SNIPPETS = [
  {
    label: "JavaScript",
    code: `const analytics = await client.posts.analytics("post_abc123");

const rollup = await client.analytics.rollup({
  from: "2026-04-01T00:00:00Z",
  to: "2026-04-30T00:00:00Z",
  granularity: "day",
});`,
  },
  {
    label: "Python",
    code: `analytics = client.posts.analytics("post_abc123")

rollup = client.analytics.rollup(
  from_date="2026-04-01T00:00:00Z",
  to_date="2026-04-30T00:00:00Z",
  granularity="day",
)`,
  },
  {
    label: "Go",
    code: `analytics, err := client.Posts.Analytics(ctx, "post_abc123")

rollup, err := client.Analytics.Rollup(ctx, &unipost.RollupParams{
  From:        "2026-04-01T00:00:00Z",
  To:          "2026-04-30T00:00:00Z",
  Granularity: "day",
})`,
  },
];

const ERROR_SNIPPETS = [
  {
    label: "JavaScript",
    code: `import { AuthError, RateLimitError, UniPostError } from "@unipost/sdk";

try {
  await client.posts.create({...});
} catch (error) {
  if (error instanceof AuthError) {
    console.error("Bad API key");
  } else if (error instanceof RateLimitError) {
    console.error("Retry after", error.retryAfter);
  } else if (error instanceof UniPostError) {
    console.error(error.status, error.code, error.message);
  }
}`,
  },
  {
    label: "Python",
    code: `try:
  client.posts.create(...)
except Exception as exc:
  print(exc)`,
  },
  {
    label: "Go",
    code: `post, err := client.Posts.Create(ctx, params)
if err != nil {
  return err
}`,
  },
];

export default function SdkPage() {
  return (
    <DocsPage
      eyebrow="Get Started"
      title="SDKs"
      lead="UniPost has first-party SDKs for JavaScript, Python, and Go. This page shows how to install them, initialize a client, publish with the recommended request shape, validate drafts, work with analytics, and handle errors in each language."
    >
      <h2 id="overview">Overview</h2>
      <p>The SDK pages should help a developer ship, not just confirm that a package exists. UniPost&apos;s SDKs all map to the same mental model: connect accounts, publish with <code>platform_posts[]</code>, validate before publish, then add drafts, preview, and analytics.</p>

      <h2 id="supported">Supported SDKs</h2>
      <DocsTable
        columns={["SDK", "Best for", "Status"]}
        rows={[
          ["JavaScript / TypeScript", "Next.js apps, backend services, edge workers", "Primary"],
          ["Python", "Automation, agents, data workflows", "Primary"],
          ["Go", "Backend services and job runners", "Primary"],
        ]}
      />

      <h2 id="install">Install</h2>
      <p>Pick the SDK that matches your runtime. If you just need raw HTTP examples, those belong in the API reference, not in the SDK guide.</p>
      <DocsCodeTabs snippets={INSTALL_SNIPPETS} />

      <h2 id="initialize">Initialize a client</h2>
      <p>Every SDK uses the same API key and the same resource model. Once the client is initialized, the rest of the examples translate directly across languages.</p>
      <DocsCodeTabs snippets={INIT_SNIPPETS} />

      <h2 id="list-accounts">List connected accounts</h2>
      <p>Most integrations start by listing accounts and selecting the right <code>account_id</code>. If you are building for end users, this is also where you look up accounts connected through UniPost Connect sessions.</p>
      <DocsCodeTabs snippets={LIST_ACCOUNTS_SNIPPETS} />

      <h2 id="publish">Publish with the recommended request shape</h2>
      <p>Across every SDK, the recommended publishing shape is <code>platform_posts[]</code> plus <code>idempotency_key</code>. It keeps retries safe and lets you adapt caption tone per platform instead of flattening everything into one caption.</p>
      <DocsCodeTabs snippets={CREATE_POST_SNIPPETS} />

      <h2 id="validate">Validate before publish</h2>
      <p>Use validation before any automated publish, especially if your content comes from an LLM. UniPost checks platform-specific constraints before the request creates a post or touches downstream platforms.</p>
      <DocsCodeTabs snippets={VALIDATE_SNIPPETS} />

      <h2 id="drafts">Work with drafts</h2>
      <p>Drafts are the safest way to build review workflows. Save first, create a preview link if needed, then publish when your product or operator is ready.</p>
      <DocsCodeTabs snippets={DRAFT_SNIPPETS} />

      <h2 id="analytics">Read analytics</h2>
      <p>The SDKs expose both per-post analytics and workspace-level rollups. That gives you enough surface area to build dashboards, reports, and agent workflows without dropping into raw HTTP.</p>
      <DocsCodeTabs snippets={ANALYTICS_SNIPPETS} />

      <h2 id="errors">Handle errors</h2>
      <p>The JavaScript SDK exposes richer typed errors today, while Python and Go currently follow their host language&apos;s more conventional patterns. In every case, validation failures, auth issues, and rate limits should be handled explicitly.</p>
      <DocsCodeTabs snippets={ERROR_SNIPPETS} />

      <h2 id="choose">Which SDK should you use?</h2>
      <DocsTable
        columns={["If you are building...", "Recommended SDK", "Why"]}
        rows={[
          ["A web app or backend in TypeScript", "JavaScript / TypeScript", "Best typed experience and the richest helper surface today"],
          ["Automation, data workflows, or AI backends", "Python", "Natural fit for automation and model-heavy pipelines"],
          ["A service, worker, or internal tool in Go", "Go", "Good fit for backend jobs and infrastructure-oriented services"],
        ]}
      />
    </DocsPage>
  );
}
