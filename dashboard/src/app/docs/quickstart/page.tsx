import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

const INSTALL_SNIPPETS = [
  { label: "Node.js", code: "npm install @unipost/sdk" },
  { label: "Python", code: "pip install unipost" },
  { label: "Go", code: "go get github.com/unipost-dev/sdk-go" },
];

const INIT_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();`,
  },
  {
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()`,
  },
  {
    label: "Go",
    code: `client := unipost.NewClient()`,
  },
];

const CONNECT_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const session = await client.connect.createSession({
  platform: "bluesky",
  externalUserId: "user_123",
  returnUrl: "https://app.acme.com/integrations/done",
});

console.log(session.url);`,
  },
];

const LIST_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const { data: accounts } = await client.accounts.list();
const accountId = accounts[0]?.id;`,
  },
  {
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

accounts = client.accounts.list()
account_id = accounts["data"][0]["id"]`,
  },
  {
    label: "Go",
    code: `package main

import (
  "context"
  "log"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient(
    unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
  )

  accounts, err := client.Accounts.List(context.Background(), nil)
  if err != nil {
    log.Fatal(err)
  }

  accountID := accounts[0].ID
  _ = accountID
}`,
  },
];

const CREATE_POST_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const post = await client.posts.create({
  platformPosts: [
    {
      accountId: "sa_twitter_123",
      caption: "Shipping on every platform with one API.",
    },
    {
      accountId: "sa_linkedin_456",
      caption: "We shipped a new release today. Here is what changed.",
    },
  ],
  idempotencyKey: "launch-2026-04-13-001",
});

console.log(post.id);`,
  },
  {
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

post = client.posts.create(
  platform_posts=[
    {
      "account_id": "sa_twitter_123",
      "caption": "Shipping on every platform with one API.",
    },
    {
      "account_id": "sa_linkedin_456",
      "caption": "We shipped a new release today. Here is what changed.",
    },
  ],
  idempotency_key="launch-2026-04-13-001",
)`,
  },
  {
    label: "Go",
    code: `package main

import (
  "context"
  "log"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient(
    unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
  )

  post, err := client.Posts.Create(context.Background(), &unipost.CreatePostParams{
    PlatformPosts: []unipost.PlatformPost{
      {
        AccountID: "sa_twitter_123",
        Caption:   "Shipping on every platform with one API.",
      },
      {
        AccountID: "sa_linkedin_456",
        Caption:   "We shipped a new release today. Here is what changed.",
      },
    },
    IdempotencyKey: "launch-2026-04-13-001",
  })
  if err != nil {
    log.Fatal(err)
  }

  _ = post.ID
}`,
  },
];

const VALIDATE_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const result = await client.posts.validate({
  platformPosts: [
    {
      accountId: "sa_twitter_123",
      caption: draftForX,
    },
  ],
});

console.log(result);`,
  },
  {
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

result = client.posts.validate(
  platform_posts=[
    {
      "account_id": "sa_twitter_123",
      "caption": draft_for_x,
    }
  ]
)`,
  },
  {
    label: "Go",
    code: `package main

import (
  "context"
  "log"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient(
    unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
  )

  validation, err := client.Posts.Validate(context.Background(), &unipost.ValidatePostParams{
    PlatformPosts: []unipost.PlatformPost{
      {
        AccountID: "sa_twitter_123",
        Caption:   draftForX,
      },
    },
  })
  if err != nil {
    log.Fatal(err)
  }

  _ = validation
}`,
  },
];

const DRAFT_SNIPPETS = [
  {
    label: "Node.js",
    code: `const draft = await client.posts.create({
  platformPosts: [
    {
      accountId: "sa_bluesky_123",
      caption: "Review this before publish",
    },
  ],
  status: "draft",
});`,
  },
  {
    label: "Python",
    code: `draft = client.posts.create(
  platform_posts=[
    {
      "account_id": "sa_bluesky_123",
      "caption": "Review this before publish",
    },
  ],
  status="draft",
)`,
  },
  {
    label: "Go",
    code: `draft, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
  PlatformPosts: []unipost.PlatformPost{
    {
      AccountID: "sa_bluesky_123",
      Caption:   "Review this before publish",
    },
  },
  Status: "draft",
})`,
  },
];

export default function QuickstartPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Get Started"
      title="Quickstart"
      lead="Create an API key, connect an account, fetch its ID, and publish your first post."
    >
      <h2 id="install-sdk">Install the SDK</h2>
      <DocsCodeTabs snippets={INSTALL_SNIPPETS} />

      <h2 id="authentication">Authentication</h2>
      <p>Every request uses a Bearer API key. The SDKs read <code>UNIPOST_API_KEY</code> by default.</p>

      <h3 id="get-api-key">Get your API key</h3>
      <p>When signed up, UniPost automatically creates one workspace and one Default profile for you.</p>
      <ul className="docs-step-list">
        <li>Step 1: Open Dashboard → API Keys.</li>
        <li>Step 2: Click Create API Key.</li>
        <li>Step 3: Copy the key and store it as <code>UNIPOST_API_KEY</code>.</li>
      </ul>

      <h3 id="set-up-client">Set up the client</h3>
      <p>Set <code>UNIPOST_API_KEY</code> as env var, the SDK clients load it automatically.</p>
      <DocsCodeTabs snippets={INIT_SNIPPETS} />

      <h2 id="key-concepts">Key concepts</h2>
      <DocsTable
        columns={["Object", "What it means"]}
        rows={[
          ["Profiles", "Containers for accounts and branding. Every new workspace starts with one Default profile."],
          ["Accounts", "Connected social accounts you can publish to"],
          ["Posts", "One publish request, with one or more platform payloads"],
          ["Webhooks", "Async status updates for publish and account events"],
        ]}
      />

      <h2 id="step-1-connect-account">Step 1: Connect an account</h2>
      <p>For customer-owned accounts, create a Connect session. For workspace-owned accounts, connect once from the dashboard.</p>
      <DocsCodeTabs snippets={CONNECT_SNIPPETS} />

      <h3 id="available-platforms">Available platforms</h3>
      <DocsTable
        columns={["Platform", "API Value", "Guide"]}
        rows={[
          ["X / Twitter", "`twitter`", <Link key="twitter-guide" href="/docs/platforms/twitter">Twitter/X Guide</Link>],
          ["LinkedIn", "`linkedin`", <Link key="linkedin-guide" href="/docs/platforms/linkedin">LinkedIn Guide</Link>],
          ["Instagram", "`instagram`", <Link key="instagram-guide" href="/docs/platforms/instagram">Instagram Guide</Link>],
          ["TikTok", "`tiktok`", <Link key="tiktok-guide" href="/docs/platforms/tiktok">TikTok Guide</Link>],
          ["YouTube", "`youtube`", <Link key="youtube-guide" href="/docs/platforms/youtube">YouTube Guide</Link>],
          ["Bluesky", "`bluesky`", <Link key="bluesky-guide" href="/docs/platforms/bluesky">Bluesky Guide</Link>],
        ]}
      />

      <h2 id="step-2-get-connected-accounts">Step 2: Get your connected accounts</h2>
      <p>List accounts and capture the UniPost account ID you want to publish to.</p>
      <DocsCodeTabs snippets={LIST_SNIPPETS} />

      <h2 id="step-3-publish-first-post">Step 3: Publish your first post</h2>
      <p>Use <code>platform_posts[]</code> for new integrations.</p>
      <DocsCodeTabs snippets={CREATE_POST_SNIPPETS} />

      <h3 id="posting-multiple-platforms">Posting to multiple platforms</h3>
      <p>Add more entries to <code>platform_posts[]</code> when you want different copy per destination.</p>

      <h3 id="publishing-immediately">Publishing immediately</h3>
      <p>Add <code>idempotency_key</code> from day one. Retries inside the 24-hour window return the original response instead of posting twice.</p>

      <h3 id="creating-draft">Creating a draft</h3>
      <p>Set <code>status: &quot;draft&quot;</code> when a human should review before publish.</p>
      <DocsCodeTabs snippets={DRAFT_SNIPPETS} />

      <h2 id="validate-before-publish">Validate before publish</h2>
      <p>Call Validate with the same body you plan to send to <ApiInlineLink endpoint="POST /v1/posts" />.</p>
      <DocsCodeTabs snippets={VALIDATE_SNIPPETS} />

      <h2 id="what-next">What&apos;s next?</h2>
      <DocsTable
        columns={["Next capability", "When to add it", "Docs path"]}
        rows={[
          [
            "Drafts + review flows",
            "When a human should approve content before publish",
            <Link key="next-drafts" href="/docs/api/posts/drafts">API Reference → Drafts</Link>,
          ],
          [
            "Connect sessions",
            "When your customers connect their own accounts",
            <Link key="next-connect" href="/docs/api/connect/sessions">API Reference → Connect Sessions</Link>,
          ],
          [
            "Analytics",
            "When you need reporting, performance tracking, or automation feedback loops",
            <Link key="next-analytics" href="/docs/api/analytics">API Reference → Analytics</Link>,
          ],
          [
            "Platform guides",
            "When you need platform-specific media limits, caption rules, or examples",
            <Link key="next-platforms" href="/docs/platforms">Platforms</Link>,
          ],
        ]}
      />
    </DocsPage>
  );
}
