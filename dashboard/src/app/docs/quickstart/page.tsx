import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";
import { DocsQuickstartCard } from "@/components/tutorials/docs-quickstart-card";

const INSTALL_SNIPPETS = [
  { label: "Node.js", code: "npm install @unipost/sdk" },
  { label: "Python", code: "pip install unipost" },
  { label: "Go", code: "go get github.com/unipost-dev/sdk-go" },
  { label: "Java", code: "implementation(\"dev.unipost:sdk-java:0.2.5\")" },
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
  {
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();`,
  },
];

const CREATE_PROFILE_SNIPPETS = [
  {
    label: "Node.js",
    code: `const profile = await client.profiles.create({
  name: "API Quickstart",
});

console.log(profile.id);`,
  },
  {
    label: "Python",
    code: `profile = client.profiles.create(
  name="API Quickstart",
)

print(profile["data"]["id"])`,
  },
  {
    label: "Go",
    code: `profile, err := client.Profiles.Create(ctx, &unipost.CreateProfileParams{
  Name: "API Quickstart",
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(profile.ID)`,
  },
  {
    label: "Java",
    code: `var profile = client.profiles().create(Map.of(
    "name", "API Quickstart"
));

System.out.println(profile.get("id").asText());`,
  },
];

const CONNECT_AUTH_SNIPPETS = [
  {
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/profiles/pr_brand_us/oauth/connect/linkedin?redirect_url=https://app.acme.com/integrations/done" \
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    label: "Node.js",
    code: `const { auth_url } = await client.connect.getConnectUrl({
  profileId: "pr_brand_us",
  platform: "linkedin",
  redirectUrl: "https://app.acme.com/integrations/done", // optional
});

console.log(auth_url);`,
  },
  {
    label: "Python",
    code: `connect = client.connect.get_connect_url(
  profile_id="pr_brand_us",
  platform="linkedin",
  redirect_url="https://app.acme.com/integrations/done",  # optional
)

print(connect.auth_url)`,
  },
  {
    label: "Go",
    code: `connect, err := client.Connect.GetConnectURL(ctx, &unipost.GetConnectURLParams{
  ProfileID:   "pr_brand_us",
  Platform:    "linkedin",
  RedirectURL: "https://app.acme.com/integrations/done", // optional
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(connect.AuthURL)`,
  },
  {
    label: "Java",
    code: `var connect = client.connect().getConnectUrl(Map.of(
    "profile_id", "pr_brand_us",
    "platform", "linkedin",
    "redirect_url", "https://app.acme.com/integrations/done" // optional
));

System.out.println(connect.get("auth_url").asText());`,
  },
];

const LIST_SNIPPETS = [
  {
    label: "Node.js",
    code: `const { data: accounts } = await client.accounts.list({
  profileId: "pr_brand_us",
});

const accountId = accounts[0]?.id;
console.log(accountId);`,
  },
  {
    label: "Python",
    code: `accounts = client.accounts.list(
  profile_id="pr_brand_us"
)

account_id = accounts["data"][0]["id"]
print(account_id)`,
  },
  {
    label: "Go",
    code: `accounts, err := client.Accounts.List(context.Background(), &unipost.ListAccountsParams{
  ProfileID: "pr_brand_us",
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(accounts[0].ID)`,
  },
  {
    label: "Java",
    code: `var accounts = client.accounts().list(Map.of(
    "profile_id", "pr_brand_us"
)).getData();

System.out.println(accounts.get(0).get("id").asText());`,
  },
];

const CREATE_POST_SNIPPETS = [
  {
    label: "Immediate",
    code: `const post = await client.posts.create({
  platformPosts: [
    {
      accountId: "sa_linkedin_123",
      caption: "Shipping on every platform with one API.",
    },
    {
      accountId: "sa_twitter_456",
      caption: "Same launch, different copy for X.",
    },
  ],
  idempotencyKey: "launch-2026-05-12-001",
});`,
  },
  {
    label: "Scheduled",
    code: `const post = await client.posts.create({
  platformPosts: [
    {
      accountId: "sa_linkedin_123",
      caption: "This will publish later today.",
    },
  ],
  publishAt: "2026-05-12T18:30:00Z",
  idempotencyKey: "launch-2026-05-12-002",
});`,
  },
];

export default function QuickstartPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="API Quickstart"
      title="API Quickstart"
      lead="Create an API key, create a profile, connect your first social account, and publish your first post."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />
      <DocsQuickstartCard tutorialId="post_with_api" fallbackHref="/docs/dashboard-quickstart" />

      <div className="qs-badges">
        <span className="qs-badge">API-first</span>
        <span className="qs-badge">Self-owned accounts</span>
        <span className="qs-badge">Node · Python · Go · Java</span>
        <span className="qs-badge">Free tier</span>
      </div>

      <h2 id="prerequisite">Prerequisite</h2>
      <p className="qs-note">Create an API key in the dashboard and store it in <code>UNIPOST_API_KEY</code>. The SDKs read it automatically.</p>
      <ul className="docs-checklist">
        <li>Open Dashboard → API Keys</li>
        <li>Click <strong>Create API Key</strong></li>
        <li>Save it as <code>UNIPOST_API_KEY</code> in your environment</li>
      </ul>

      <h2 id="steps">The four steps</h2>
      <div className="qs-flow">
        <a href="#install" className="qs-flow-step">
          <div className="qs-flow-num">1</div>
          <div className="qs-flow-body">
            <div className="qs-flow-title">Create a profile</div>
            <div className="qs-flow-sub">Call <code>POST /v1/profiles</code> and keep the returned <code>profile_id</code>.</div>
          </div>
        </a>
        <a href="#connect-account" className="qs-flow-step">
          <div className="qs-flow-num">2</div>
          <div className="qs-flow-body">
            <div className="qs-flow-title">Connect an account</div>
            <div className="qs-flow-sub">Call the OAuth connect endpoint and open the returned <code>auth_url</code> in a browser.</div>
          </div>
        </a>
        <a href="#get-account-id" className="qs-flow-step">
          <div className="qs-flow-num">3</div>
          <div className="qs-flow-body">
            <div className="qs-flow-title">List connected accounts</div>
            <div className="qs-flow-sub">Fetch your new UniPost <code>account_id</code>.</div>
          </div>
        </a>
        <a href="#first-post" className="qs-flow-step">
          <div className="qs-flow-num">4</div>
          <div className="qs-flow-body">
            <div className="qs-flow-title">Publish or schedule</div>
            <div className="qs-flow-sub">Send one post immediately or schedule one for later.</div>
          </div>
        </a>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Account owner", "You or your team — not your customers"],
          ["OAuth style", "Self-owned account connection via auth_url"],
          ["What you get back", "A profile_id, then an auth_url, then one or more account IDs"],
          ["Best for", "Prototypes, internal tools, your own brand accounts"],
          ["Need customer onboarding?", <Link key="quickstart-wl" href="/docs/white-label">Use White-label / Connect Sessions instead</Link>],
        ]}
      />

      <h2 id="install">Install and initialize</h2>
      <DocsCodeTabs snippets={INSTALL_SNIPPETS} />
      <h3 id="init-client">Initialize the client</h3>
      <DocsCodeTabs snippets={INIT_SNIPPETS} />

      <h2 id="create-profile">1. Create a profile</h2>
      <p className="qs-note">
        Profiles are the container for connected accounts. Every new API-first integration should create one deliberately instead of relying on the dashboard default.
      </p>
      <DocsCodeTabs snippets={CREATE_PROFILE_SNIPPETS} />
      <p className="qs-note">
        Full schema: <ApiInlineLink endpoint="POST /v1/profiles" />.
      </p>

      <h2 id="connect-account">2. Connect an account</h2>
      <p className="qs-note">
        For OAuth platforms like LinkedIn, X, YouTube, Instagram, Threads, TikTok, and Pinterest, call the profile-scoped connect URL API. It returns an <code>auth_url</code>. Open that URL in a browser and complete OAuth there.
      </p>
      <DocsCodeTabs snippets={CONNECT_AUTH_SNIPPETS} />
      <ul className="docs-checklist">
        <li>Call <code>GET /v1/profiles/{"{profile_id}"}/oauth/connect/{"{platform}"}</code></li>
        <li>Read <code>data.auth_url</code> from the response</li>
        <li>Open it in a browser</li>
        <li>Complete OAuth and let UniPost redirect back to your <code>redirect_url</code></li>
      </ul>
      <p className="qs-note">
        <code>redirect_url</code> is your own app page for “OAuth finished”. It is optional. If you omit it, UniPost still connects the account and sends the browser back to the UniPost app after OAuth completes. Endpoint reference:{" "}
        <ApiInlineLink endpoint="GET /v1/profiles/{profile_id}/oauth/connect/{platform}" />.
      </p>
      <p className="qs-note">
        Bluesky is the exception. It uses an app password, not OAuth, so connect it through <ApiInlineLink endpoint="POST /v1/accounts/connect" /> instead.
      </p>

      <h2 id="get-account-id">3. Get your connected account</h2>
      <p className="qs-note">List accounts for the profile you created and keep the returned UniPost <code>account_id</code>.</p>
      <DocsCodeTabs snippets={LIST_SNIPPETS} />
      <p className="qs-note">
        Reference: <ApiInlineLink endpoint="GET /v1/accounts" />.
      </p>

      <h2 id="first-post">4. Schedule your first post</h2>
      <p className="qs-note">
        You can publish immediately or schedule for later. You can also target one platform or multiple platforms in the same request.
      </p>
      <DocsCodeTabs snippets={CREATE_POST_SNIPPETS} />
      <DocsTable
        columns={["Pattern", "How it works"]}
        rows={[
          ["Immediate, single-platform", "One entry in platform_posts and no publishAt"],
          ["Immediate, multi-platform", "Multiple entries in platform_posts with different accountId / caption values"],
          ["Scheduled, single-platform", "Set publishAt and include one account"],
          ["Scheduled, multi-platform", "Set publishAt and include multiple accounts in platform_posts"],
        ]}
      />
      <p className="qs-note">
        Reference: <ApiInlineLink endpoint="POST /v1/posts/create" />.
      </p>

      <h2 id="what-this-is-not">What this quickstart is not</h2>
      <DocsTable
        columns={["Flow", "Use this page?", "Where to go instead"]}
        rows={[
          ["Connect your own accounts with OAuth auth_url", "Yes", "This page"],
          ["Connect customer-owned accounts", "No", <Link key="wl" href="/docs/white-label">White-label / Connect Sessions</Link>],
          ["Connect Bluesky with app password", "Partially", <Link key="acct-connect" href="/docs/api/accounts/connect">POST /v1/accounts/connect</Link>],
        ]}
      />

      <h2 id="next-steps">Next steps</h2>
      <div className="qs-next">
        <Link href="/docs/api/profiles/create" className="qs-next-card">
          <div className="qs-next-kicker">API reference</div>
          <div className="qs-next-title">Create profile</div>
          <div className="qs-next-body">Full request and response schema for <code>POST /v1/profiles</code>.</div>
        </Link>
        <Link href="/docs/api/accounts/connect" className="qs-next-card">
          <div className="qs-next-kicker">Bluesky / direct credentials</div>
          <div className="qs-next-title">Connect account</div>
          <div className="qs-next-body">Use this direct-connect endpoint for Bluesky and other non-OAuth credential flows.</div>
        </Link>
        <Link href="/docs/white-label" className="qs-next-card">
          <div className="qs-next-kicker">For customer accounts</div>
          <div className="qs-next-title">White-label</div>
          <div className="qs-next-body">Customer-owned onboarding with Connect Sessions, external_user_id, and branded OAuth.</div>
        </Link>
        <Link href="/docs/api/posts/create" className="qs-next-card">
          <div className="qs-next-kicker">Publish</div>
          <div className="qs-next-title">Create post</div>
          <div className="qs-next-body">Full publish schema for immediate and scheduled posts.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.qs-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 26px}
.qs-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.qs-flow{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin:16px 0 28px}
.qs-flow-step{text-decoration:none;color:inherit;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);display:flex;gap:12px;align-items:flex-start}
.qs-flow-step:hover{border-color:color-mix(in srgb,var(--docs-link) 34%, var(--docs-border));transform:translateY(-1px)}
.qs-flow-num{width:28px;height:28px;border-radius:999px;background:color-mix(in srgb,var(--docs-link) 12%, var(--docs-bg-muted));color:var(--docs-link);font-weight:700;font-size:13px;display:flex;align-items:center;justify-content:center;flex-shrink:0}
.qs-flow-title{font-size:14px;font-weight:700;color:var(--docs-text)}
.qs-flow-sub{font-size:13px;line-height:1.55;color:var(--docs-text-soft);margin-top:4px}
.qs-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:8px 0 14px;max-width:none}
.qs-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin-top:8px}
.qs-next-card{padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit}
.qs-next-card:hover{border-color:color-mix(in srgb,var(--docs-link) 34%, var(--docs-border));transform:translateY(-1px)}
.qs-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint);margin-bottom:8px}
.qs-next-title{font-size:15px;font-weight:700;color:var(--docs-text);margin-bottom:6px}
.qs-next-body{font-size:13px;line-height:1.6;color:var(--docs-text-soft)}
@media (max-width: 980px){
  .qs-flow{grid-template-columns:repeat(2,minmax(0,1fr))}
  .qs-next{grid-template-columns:1fr}
}
@media (max-width: 640px){
  .qs-flow{grid-template-columns:1fr}
}
`;
