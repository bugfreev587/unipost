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
      lead="Create an API key, connect an account, and publish your first post — in about five minutes."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="qs-badges">
        <span className="qs-badge">~5 min</span>
        <span className="qs-badge">Node · Python · Go</span>
        <span className="qs-badge">REST · SDK · MCP</span>
        <span className="qs-badge">Free tier</span>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Time to first post", "~5 minutes"],
          ["You'll need", "A UniPost account and one connected social account"],
          ["Languages", "Node.js, Python, or Go (or call the REST API directly)"],
          ["What you'll have at the end", "A live post and an account ID you can publish to"],
          ["Cost", "Free — the free tier covers the quickstart"],
        ]}
      />

      <h2 id="steps">The four steps</h2>
      <div className="qs-flow">
        <a href="#install" className="qs-flow-step">
          <div className="qs-flow-num">1</div>
          <div className="qs-flow-body">
            <div className="qs-flow-title">Install the SDK</div>
            <div className="qs-flow-sub">One command in your language of choice.</div>
          </div>
        </a>
        <a href="#authentication" className="qs-flow-step">
          <div className="qs-flow-num">2</div>
          <div className="qs-flow-body">
            <div className="qs-flow-title">Get your API key</div>
            <div className="qs-flow-sub">Create one in the dashboard, store as <code>UNIPOST_API_KEY</code>.</div>
          </div>
        </a>
        <a href="#connect-account" className="qs-flow-step">
          <div className="qs-flow-num">3</div>
          <div className="qs-flow-body">
            <div className="qs-flow-title">Connect an account</div>
            <div className="qs-flow-sub">From the dashboard or via <code>connect.sessions</code>.</div>
          </div>
        </a>
        <a href="#first-post" className="qs-flow-step">
          <div className="qs-flow-num">4</div>
          <div className="qs-flow-body">
            <div className="qs-flow-title">Publish your first post</div>
            <div className="qs-flow-sub">One <code>POST /v1/posts</code> call.</div>
          </div>
        </a>
      </div>

      <h2 id="key-concepts">Key concepts</h2>
      <DocsTable
        columns={["Object", "What it means"]}
        rows={[
          ["Profiles", "Containers for accounts and branding. Every workspace starts with one Default profile."],
          ["Accounts", "Connected social accounts you can publish to"],
          ["Posts", "One publish request, with one or more platform payloads"],
          ["Webhooks", "Async status updates for publish and account events"],
        ]}
      />

      <h2 id="install">1. Install the SDK</h2>
      <DocsCodeTabs snippets={INSTALL_SNIPPETS} />

      <h2 id="authentication">2. Get your API key</h2>
      <p className="qs-note">Every request uses a Bearer API key. Each SDK reads <code>UNIPOST_API_KEY</code> by default.</p>
      <ul className="qs-checklist">
        <li>Open Dashboard → API Keys</li>
        <li>Click <strong>Create API Key</strong></li>
        <li>Copy the key and store it as <code>UNIPOST_API_KEY</code> in your environment</li>
      </ul>
      <h3 id="init-client">Initialize the client</h3>
      <DocsCodeTabs snippets={INIT_SNIPPETS} />

      <h2 id="connect-account">3. Connect an account</h2>
      <p className="qs-note">Two paths. Pick one — you don't need both.</p>
      <DocsTable
        columns={["Account owner", "How to connect", "Best for"]}
        rows={[
          ["Your team", "Dashboard → Accounts → Connect", "Prototypes, internal tools, your own brand accounts"],
          ["Your customer", <ApiInlineLink key="api-connect" endpoint="POST /v1/connect/sessions" />, "SaaS products onboarding customer accounts"],
        ]}
      />
      <h3 id="connect-session-example">Create a Connect session (customer-owned)</h3>
      <DocsCodeTabs snippets={CONNECT_SNIPPETS} />

      <h3 id="supported-platforms">Supported platforms</h3>
      <DocsTable
        columns={["Platform", "API value", "Guide"]}
        rows={[
          ["X / Twitter", "`twitter`", <Link key="gd-twitter" href="/docs/platforms/twitter">Twitter/X</Link>],
          ["LinkedIn", "`linkedin`", <Link key="gd-linkedin" href="/docs/platforms/linkedin">LinkedIn</Link>],
          ["Instagram", "`instagram`", <Link key="gd-instagram" href="/docs/platforms/instagram">Instagram</Link>],
          ["Threads", "`threads`", <Link key="gd-threads" href="/docs/platforms/threads">Threads</Link>],
          ["TikTok", "`tiktok`", <Link key="gd-tiktok" href="/docs/platforms/tiktok">TikTok</Link>],
          ["YouTube", "`youtube`", <Link key="gd-youtube" href="/docs/platforms/youtube">YouTube</Link>],
          ["Bluesky", "`bluesky`", <Link key="gd-bluesky" href="/docs/platforms/bluesky">Bluesky</Link>],
          ["Facebook (Beta)", "`facebook`", <Link key="gd-facebook" href="/docs/platforms/facebook">Facebook</Link>],
        ]}
      />

      <h2 id="get-account-id">Get your account ID</h2>
      <p className="qs-note">List accounts and grab the UniPost ID you want to publish to.</p>
      <DocsCodeTabs snippets={LIST_SNIPPETS} />

      <h2 id="first-post">4. Publish your first post</h2>
      <p className="qs-note">Use <code>platform_posts[]</code> for new integrations. Add an <code>idempotency_key</code> from day one.</p>
      <DocsCodeTabs snippets={CREATE_POST_SNIPPETS} />

      <h2 id="next-level">Level up</h2>
      <DocsTable
        columns={["Capability", "What it adds", "How"]}
        rows={[
          ["Validate before publish", "Catch caption, media, and thread shape issues before they hit the platform", <ApiInlineLink key="ap-validate" endpoint="POST /v1/posts/validate" />],
          ["Drafts + review", "Create a post in `draft` status for human approval before it goes live", "`status: \"draft\"` on create"],
          ["Cross-post with different copy", "Send different captions per platform in one request", "Multiple entries in `platform_posts[]`"],
          ["Idempotency", "Retries inside 24h return the original response instead of publishing twice", "`idempotency_key` on every create"],
          ["Webhooks", "Get async publish + account events pushed to your server", <Link key="nx-wh" href="/docs/api/webhooks">Webhooks</Link>],
        ]}
      />

      <h3 id="validate-example">Validate example</h3>
      <DocsCodeTabs snippets={VALIDATE_SNIPPETS} />

      <h3 id="draft-example">Draft example</h3>
      <DocsCodeTabs snippets={DRAFT_SNIPPETS} />

      <h2 id="next-steps">Next steps</h2>
      <div className="qs-next">
        <Link href="/docs/platforms" className="qs-next-card">
          <div className="qs-next-kicker">Per platform</div>
          <div className="qs-next-title">Platform guides</div>
          <div className="qs-next-body">Caption limits, media rules, analytics, and inbox support by platform.</div>
        </Link>
        <Link href="/docs/white-label" className="qs-next-card">
          <div className="qs-next-kicker">For customer accounts</div>
          <div className="qs-next-title">White-label</div>
          <div className="qs-next-body">Branded Connect flows on your own OAuth apps.</div>
        </Link>
        <Link href="/docs/api/posts/create" className="qs-next-card">
          <div className="qs-next-kicker">API reference</div>
          <div className="qs-next-title">Create post</div>
          <div className="qs-next-body">Full request / response schema for the publish endpoint.</div>
        </Link>
        <Link href="/docs/mcp" className="qs-next-card">
          <div className="qs-next-kicker">For agents</div>
          <div className="qs-next-title">MCP</div>
          <div className="qs-next-body">Hosted MCP server so LLMs can publish through UniPost.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.qs-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 24px}
.qs-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.qs-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:6px 0 14px;max-width:none}
.qs-note code{font-family:var(--docs-mono);font-size:12.5px}
.qs-flow{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin:14px 0 6px}
.qs-flow-step{display:grid;grid-template-columns:36px 1fr;gap:14px;align-items:start;padding:14px 16px;border:1px solid var(--docs-border);border-radius:14px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.qs-flow-step:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.qs-flow-num{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;border-radius:999px;background:color-mix(in srgb, var(--docs-link) 14%, var(--docs-bg-muted));color:var(--docs-link);font-size:13px;font-weight:700;border:1px solid color-mix(in srgb, var(--docs-link) 22%, var(--docs-border))}
.qs-flow-title{font-size:15px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text);margin-bottom:3px}
.qs-flow-sub{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
.qs-flow-sub code{font-family:var(--docs-mono);font-size:12px}
.qs-checklist{list-style:none;padding:0;margin:10px 0 14px;display:grid;grid-template-columns:1fr;gap:4px}
.qs-checklist li{position:relative;padding-left:22px;font-size:14px;line-height:1.7;color:var(--docs-text-soft)}
.qs-checklist li::before{content:"";position:absolute;left:0;top:9px;width:12px;height:12px;border-radius:4px;border:1.5px solid color-mix(in srgb, var(--docs-link) 45%, var(--docs-border-strong));background:color-mix(in srgb, var(--docs-link) 14%, transparent)}
.qs-checklist li code{font-family:var(--docs-mono);font-size:12.5px}
.qs-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.qs-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.qs-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.qs-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.qs-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.qs-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
@media (max-width:960px){
  .qs-flow{grid-template-columns:1fr}
  .qs-next{grid-template-columns:1fr}
}
`;
