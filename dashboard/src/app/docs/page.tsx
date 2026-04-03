import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Documentation — UniPost",
  description: "UniPost API documentation and developer guide",
};

const BASE = "https://api.unipost.dev";

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="scroll-mt-24 mb-16">
      <h2 className="text-2xl font-bold text-zinc-900 mb-4">
        <a href={`#${id}`} className="hover:text-blue-600">{title}</a>
      </h2>
      {children}
    </section>
  );
}

function Endpoint({ method, path, auth, children }: { method: string; path: string; auth: string; children: React.ReactNode }) {
  const colors: Record<string, string> = {
    GET: "bg-green-100 text-green-800",
    POST: "bg-blue-100 text-blue-800",
    PATCH: "bg-yellow-100 text-yellow-800",
    DELETE: "bg-red-100 text-red-800",
  };
  return (
    <div className="border border-zinc-200 rounded-lg mb-6 overflow-hidden">
      <div className="flex items-center gap-3 px-4 py-3 bg-zinc-50 border-b border-zinc-200">
        <span className={`px-2 py-0.5 rounded text-xs font-bold ${colors[method] || "bg-zinc-100"}`}>{method}</span>
        <code className="text-sm font-mono text-zinc-800">{path}</code>
        <span className="text-xs text-zinc-500 ml-auto">{auth}</span>
      </div>
      <div className="px-4 py-4 space-y-4 text-sm text-zinc-700">{children}</div>
    </div>
  );
}

function Code({ children, title }: { children: string; title?: string }) {
  return (
    <div>
      {title && <p className="text-xs text-zinc-500 mb-1">{title}</p>}
      <pre className="bg-zinc-900 text-zinc-100 rounded-lg p-4 text-xs font-mono overflow-x-auto whitespace-pre">
        {children}
      </pre>
    </div>
  );
}

function Param({ name, type, required, children }: { name: string; type: string; required?: boolean; children: React.ReactNode }) {
  return (
    <div className="flex gap-2 py-1">
      <code className="text-sm font-mono text-blue-700 whitespace-nowrap">{name}</code>
      <span className="text-xs text-zinc-400">{type}</span>
      {required && <span className="text-xs text-red-500">required</span>}
      <span className="text-sm text-zinc-600">— {children}</span>
    </div>
  );
}

export default function DocsPage() {
  return (
    <div className="min-h-screen bg-white">
      <div className="max-w-5xl mx-auto px-6 py-12 flex gap-12">
        {/* Sidebar nav */}
        <nav className="hidden lg:block w-56 shrink-0 sticky top-12 self-start">
          <p className="font-bold text-zinc-900 mb-4">
            <Link href="/" className="hover:text-blue-600">UniPost</Link>
          </p>
          <ul className="space-y-1 text-sm">
            {[
              ["overview", "Overview"],
              ["authentication", "Authentication"],
              ["quick-start", "Quick Start"],
              ["social-accounts", "Social Accounts"],
              ["social-posts", "Social Posts"],
              ["webhooks", "Webhooks"],
              ["oauth", "OAuth Flow"],
              ["billing", "Billing & Usage"],
              ["errors", "Error Handling"],
              ["platforms", "Supported Platforms"],
            ].map(([id, label]) => (
              <li key={id}>
                <a href={`#${id}`} className="text-zinc-600 hover:text-blue-600">{label}</a>
              </li>
            ))}
          </ul>
        </nav>

        {/* Main content */}
        <div className="flex-1 min-w-0">
          <h1 className="text-4xl font-bold text-zinc-900 mb-2">UniPost API Documentation</h1>
          <p className="text-lg text-zinc-500 mb-12">
            One API to post across all major social platforms.
          </p>

          {/* Overview */}
          <Section id="overview" title="Overview">
            <p>
              UniPost is a unified social media API that lets developers integrate posting capabilities
              into their products without dealing with each platform individually. Connect social accounts
              once, then publish content to Bluesky, LinkedIn, Instagram, Threads, TikTok, and YouTube
              through a single API call.
            </p>
            <div className="mt-4 grid grid-cols-2 gap-4">
              <div className="border border-zinc-200 rounded-lg p-4">
                <p className="font-semibold text-zinc-900">Base URL</p>
                <code className="text-sm text-blue-700">{BASE}</code>
              </div>
              <div className="border border-zinc-200 rounded-lg p-4">
                <p className="font-semibold text-zinc-900">Response Format</p>
                <code className="text-sm text-blue-700">JSON</code>
              </div>
            </div>
            <div className="mt-4 p-4 bg-zinc-50 rounded-lg border border-zinc-200">
              <p className="font-semibold text-zinc-900 mb-2">All responses follow this structure:</p>
              <Code>{`// Success
{ "data": { ... }, "meta": { "total": 10, "page": 1, "per_page": 20 } }

// Error
{ "error": { "code": "UNAUTHORIZED", "message": "Invalid API key" } }`}</Code>
            </div>
          </Section>

          {/* Authentication */}
          <Section id="authentication" title="Authentication">
            <p>
              All API requests require a Bearer token in the <code className="text-sm bg-zinc-100 px-1 rounded">Authorization</code> header.
              Create API keys in your project dashboard at <a href="https://app.unipost.dev" className="text-blue-600 hover:underline">app.unipost.dev</a>.
            </p>
            <Code title="Example">{`curl ${BASE}/v1/social-accounts \\
  -H "Authorization: Bearer up_live_your_api_key_here"`}</Code>
            <div className="mt-4 space-y-2">
              <p className="text-sm"><strong>Key format:</strong> <code className="bg-zinc-100 px-1 rounded">up_live_</code> (production) or <code className="bg-zinc-100 px-1 rounded">up_test_</code> (test)</p>
              <p className="text-sm"><strong>Security:</strong> Keys are shown only once at creation. Store them securely — never commit to version control.</p>
            </div>
          </Section>

          {/* Quick Start */}
          <Section id="quick-start" title="Quick Start">
            <p className="mb-4">Get posting in 3 steps:</p>

            <div className="space-y-6">
              <div>
                <p className="font-semibold text-zinc-900 mb-2">1. Connect a social account</p>
                <Code>{`curl -X POST ${BASE}/v1/social-accounts/connect \\
  -H "Authorization: Bearer up_live_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "bluesky",
    "credentials": {
      "handle": "yourname.bsky.social",
      "app_password": "xxxx-xxxx-xxxx-xxxx"
    }
  }'`}</Code>
              </div>

              <div>
                <p className="font-semibold text-zinc-900 mb-2">2. Get your account ID from the response</p>
                <Code>{`{
  "data": {
    "id": "sa_abc123",
    "platform": "bluesky",
    "account_name": "yourname.bsky.social",
    "status": "active"
  }
}`}</Code>
              </div>

              <div>
                <p className="font-semibold text-zinc-900 mb-2">3. Create a post</p>
                <Code>{`curl -X POST ${BASE}/v1/social-posts \\
  -H "Authorization: Bearer up_live_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Hello from UniPost!",
    "account_ids": ["sa_abc123"]
  }'`}</Code>
              </div>
            </div>
          </Section>

          {/* Social Accounts */}
          <Section id="social-accounts" title="Social Accounts">
            <p className="mb-6">Connect, list, and disconnect social media accounts.</p>

            <Endpoint method="POST" path="/v1/social-accounts/connect" auth="API Key">
              <p>Connect a new social media account. For Bluesky, provide credentials directly. For OAuth platforms (LinkedIn, Instagram, Threads, TikTok, YouTube), use the <a href="#oauth" className="text-blue-600">OAuth flow</a> instead.</p>
              <p className="font-semibold mt-3">Request Body</p>
              <Param name="platform" type="string" required>Platform identifier: <code className="bg-zinc-100 px-1 rounded">bluesky</code></Param>
              <Param name="credentials" type="object" required>Platform-specific credentials</Param>
              <Code title="Example: Connect Bluesky">{`curl -X POST ${BASE}/v1/social-accounts/connect \\
  -H "Authorization: Bearer up_live_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "bluesky",
    "credentials": {
      "handle": "alice.bsky.social",
      "app_password": "xxxx-xxxx-xxxx-xxxx"
    }
  }'`}</Code>
              <Code title="Response (201)">{`{
  "data": {
    "id": "sa_abc123",
    "platform": "bluesky",
    "account_name": "alice.bsky.social",
    "connected_at": "2026-04-02T10:00:00Z",
    "status": "active"
  }
}`}</Code>
            </Endpoint>

            <Endpoint method="GET" path="/v1/social-accounts" auth="API Key">
              <p>List all connected social accounts for the current project.</p>
              <Code title="Example">{`curl ${BASE}/v1/social-accounts \\
  -H "Authorization: Bearer up_live_your_key"`}</Code>
              <Code title="Response (200)">{`{
  "data": [
    {
      "id": "sa_abc123",
      "platform": "bluesky",
      "account_name": "alice.bsky.social",
      "connected_at": "2026-04-02T10:00:00Z",
      "status": "active"
    },
    {
      "id": "sa_def456",
      "platform": "linkedin",
      "account_name": "Alice Smith",
      "connected_at": "2026-04-02T11:00:00Z",
      "status": "active"
    }
  ],
  "meta": { "total": 2, "page": 1, "per_page": 20 }
}`}</Code>
            </Endpoint>

            <Endpoint method="DELETE" path="/v1/social-accounts/{id}" auth="API Key">
              <p>Disconnect a social account. The account&apos;s tokens are invalidated.</p>
              <Code title="Example">{`curl -X DELETE ${BASE}/v1/social-accounts/sa_abc123 \\
  -H "Authorization: Bearer up_live_your_key"`}</Code>
              <Code title="Response (200)">{`{ "data": { "disconnected": true } }`}</Code>
            </Endpoint>
          </Section>

          {/* Social Posts */}
          <Section id="social-posts" title="Social Posts">
            <p className="mb-6">Create, list, get, and delete social media posts. Posts can be published to multiple accounts simultaneously.</p>

            <Endpoint method="POST" path="/v1/social-posts" auth="API Key">
              <p>Create and publish a post to one or more connected accounts. Posts are published concurrently — one failure won&apos;t block others.</p>
              <p className="font-semibold mt-3">Request Body</p>
              <Param name="caption" type="string" required>The text content of the post</Param>
              <Param name="account_ids" type="string[]" required>Array of social account IDs to post to</Param>
              <Param name="media_urls" type="string[]">Array of media URLs (images for Instagram, videos for TikTok/YouTube)</Param>
              <p className="font-semibold mt-3">Response Headers</p>
              <Param name="X-UniPost-Usage" type="header">Current usage, e.g. <code className="bg-zinc-100 px-1 rounded">450/1000</code></Param>
              <Param name="X-UniPost-Warning" type="header">Warning level: <code className="bg-zinc-100 px-1 rounded">approaching_limit</code> (80%+) or <code className="bg-zinc-100 px-1 rounded">over_limit</code> (100%+)</Param>
              <Code title="Example: Post to multiple platforms">{`curl -X POST ${BASE}/v1/social-posts \\
  -H "Authorization: Bearer up_live_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Hello from UniPost! 🚀",
    "account_ids": ["sa_bluesky_123", "sa_linkedin_456"],
    "media_urls": ["https://example.com/image.jpg"]
  }'`}</Code>
              <Code title="Response (200)">{`{
  "data": {
    "id": "post_xyz789",
    "caption": "Hello from UniPost! 🚀",
    "status": "published",
    "created_at": "2026-04-02T12:00:00Z",
    "results": [
      {
        "social_account_id": "sa_bluesky_123",
        "platform": "bluesky",
        "status": "published",
        "external_id": "at://did:plc:xxx/app.bsky.feed.post/yyy",
        "published_at": "2026-04-02T12:00:01Z"
      },
      {
        "social_account_id": "sa_linkedin_456",
        "platform": "linkedin",
        "status": "published",
        "external_id": "urn:li:share:123456",
        "published_at": "2026-04-02T12:00:01Z"
      }
    ]
  }
}`}</Code>
              <div className="mt-3 p-3 bg-amber-50 border border-amber-200 rounded-lg text-sm">
                <p><strong>Post status values:</strong></p>
                <ul className="list-disc pl-5 mt-1 space-y-1">
                  <li><code className="bg-zinc-100 px-1 rounded">published</code> — all accounts succeeded</li>
                  <li><code className="bg-zinc-100 px-1 rounded">partial</code> — some accounts succeeded, some failed</li>
                  <li><code className="bg-zinc-100 px-1 rounded">failed</code> — all accounts failed</li>
                </ul>
              </div>
            </Endpoint>

            <Endpoint method="GET" path="/v1/social-posts/{id}" auth="API Key">
              <p>Get a post with its per-account results. For TikTok posts, includes real-time publish status from TikTok.</p>
              <Code title="Example">{`curl ${BASE}/v1/social-posts/post_xyz789 \\
  -H "Authorization: Bearer up_live_your_key"`}</Code>
            </Endpoint>

            <Endpoint method="GET" path="/v1/social-posts" auth="API Key">
              <p>List all posts for the current project, ordered by creation date (newest first).</p>
              <Code title="Example">{`curl ${BASE}/v1/social-posts \\
  -H "Authorization: Bearer up_live_your_key"`}</Code>
            </Endpoint>

            <Endpoint method="DELETE" path="/v1/social-posts/{id}" auth="API Key">
              <p>Delete a post. Attempts to delete from all platforms where it was published.</p>
              <Code title="Example">{`curl -X DELETE ${BASE}/v1/social-posts/post_xyz789 \\
  -H "Authorization: Bearer up_live_your_key"`}</Code>
              <Code title="Response (200)">{`{ "data": { "deleted": true } }`}</Code>
            </Endpoint>
          </Section>

          {/* Webhooks */}
          <Section id="webhooks" title="Webhooks">
            <p className="mb-6">Register webhook endpoints to receive notifications about post status changes.</p>

            <Endpoint method="POST" path="/v1/webhooks" auth="API Key">
              <p>Register a webhook endpoint.</p>
              <Param name="url" type="string" required>HTTPS URL to receive events</Param>
              <Param name="events" type="string[]" required>Event types: <code className="bg-zinc-100 px-1 rounded">post.published</code>, <code className="bg-zinc-100 px-1 rounded">post.failed</code>, <code className="bg-zinc-100 px-1 rounded">account.connected</code>, <code className="bg-zinc-100 px-1 rounded">account.disconnected</code></Param>
              <Param name="secret" type="string" required>Secret for HMAC-SHA256 signature verification</Param>
              <Code title="Example">{`curl -X POST ${BASE}/v1/webhooks \\
  -H "Authorization: Bearer up_live_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{
    "url": "https://your-app.com/webhooks/unipost",
    "events": ["post.published", "post.failed"],
    "secret": "your_webhook_secret"
  }'`}</Code>
              <div className="mt-3 p-3 bg-zinc-50 border border-zinc-200 rounded-lg text-sm">
                <p className="font-semibold mb-2">Webhook payload format:</p>
                <Code>{`{
  "event": "post.published",
  "timestamp": "2026-04-02T12:00:01Z",
  "data": {
    "post_id": "post_xyz789",
    "social_account_id": "sa_abc123",
    "platform": "bluesky",
    "external_id": "at://did:plc:xxx/app.bsky.feed.post/yyy"
  }
}`}</Code>
                <p className="mt-2">Verify signatures with the <code className="bg-zinc-100 px-1 rounded">X-UniPost-Signature</code> header:</p>
                <Code>{`expected = HMAC-SHA256(your_secret, request_body)
actual = headers["X-UniPost-Signature"].replace("sha256=", "")
assert expected == actual`}</Code>
              </div>
            </Endpoint>

            <Endpoint method="GET" path="/v1/webhooks" auth="API Key">
              <p>List all registered webhooks for the current project.</p>
              <Code title="Example">{`curl ${BASE}/v1/webhooks \\
  -H "Authorization: Bearer up_live_your_key"`}</Code>
            </Endpoint>
          </Section>

          {/* OAuth */}
          <Section id="oauth" title="OAuth Flow">
            <p className="mb-6">
              For platforms that require OAuth (LinkedIn, Instagram, Threads, TikTok, YouTube),
              use the OAuth connect endpoint to get an authorization URL, then redirect the user.
            </p>

            <Endpoint method="GET" path="/v1/oauth/connect/{platform}" auth="API Key">
              <p>Get an OAuth authorization URL for the specified platform.</p>
              <Param name="platform" type="path" required>One of: <code className="bg-zinc-100 px-1 rounded">linkedin</code>, <code className="bg-zinc-100 px-1 rounded">instagram</code>, <code className="bg-zinc-100 px-1 rounded">threads</code>, <code className="bg-zinc-100 px-1 rounded">tiktok</code>, <code className="bg-zinc-100 px-1 rounded">youtube</code></Param>
              <Param name="redirect_url" type="query">URL to redirect to after authorization</Param>
              <Code title="Example">{`curl "${BASE}/v1/oauth/connect/linkedin?redirect_url=https://your-app.com/callback" \\
  -H "Authorization: Bearer up_live_your_key"`}</Code>
              <Code title="Response (200)">{`{
  "data": {
    "auth_url": "https://www.linkedin.com/oauth/v2/authorization?client_id=...&state=..."
  }
}`}</Code>
              <p className="mt-2 text-sm">Redirect the user to <code className="bg-zinc-100 px-1 rounded">auth_url</code>. After authorization, they&apos;ll be redirected to your <code className="bg-zinc-100 px-1 rounded">redirect_url</code> with <code className="bg-zinc-100 px-1 rounded">?status=success&amp;account_name=...</code> or <code className="bg-zinc-100 px-1 rounded">?status=error&amp;error=...</code>.</p>
            </Endpoint>
          </Section>

          {/* Billing */}
          <Section id="billing" title="Billing & Usage">
            <p className="mb-6">UniPost uses a soft-block quota system. Exceeding your limit won&apos;t interrupt service — you&apos;ll receive warnings via response headers and dashboard notifications.</p>

            <div className="border border-zinc-200 rounded-lg p-4 mb-6">
              <p className="font-semibold text-zinc-900 mb-3">Plans</p>
              <div className="grid grid-cols-4 gap-2 text-sm">
                {[
                  ["Free", "$0/mo", "100"],
                  ["Starter", "$10/mo", "1,000"],
                  ["Growth", "$50/mo", "5,000"],
                  ["Scale", "$150/mo", "20,000"],
                ].map(([name, price, posts]) => (
                  <div key={name} className="border border-zinc-200 rounded p-3 text-center">
                    <p className="font-semibold">{name}</p>
                    <p className="text-zinc-500">{price}</p>
                    <p className="text-xs text-zinc-400">{posts} posts</p>
                  </div>
                ))}
              </div>
              <p className="text-xs text-zinc-500 mt-3">
                Additional tiers available: $25, $75, $300, $500, $1000/mo.
                <a href="https://unipost.dev" className="text-blue-600 ml-1">See full pricing</a>
              </p>
            </div>

            <div className="p-3 bg-zinc-50 border border-zinc-200 rounded-lg text-sm mb-6">
              <p className="font-semibold mb-2">Usage headers on POST /v1/social-posts:</p>
              <Code>{`# Normal usage
X-UniPost-Usage: 450/1000

# Approaching limit (80%+)
X-UniPost-Usage: 820/1000
X-UniPost-Warning: approaching_limit

# Over limit (still processing, upgrade recommended)
X-UniPost-Usage: 1050/1000
X-UniPost-Warning: over_limit`}</Code>
            </div>
          </Section>

          {/* Errors */}
          <Section id="errors" title="Error Handling">
            <div className="space-y-2 text-sm">
              <div className="grid grid-cols-3 gap-4 p-3 bg-zinc-50 rounded-lg font-semibold">
                <span>Code</span><span>Status</span><span>Description</span>
              </div>
              {[
                ["UNAUTHORIZED", "401", "Invalid or missing API key"],
                ["FORBIDDEN", "403", "No access to this resource"],
                ["NOT_FOUND", "404", "Resource not found"],
                ["VALIDATION_ERROR", "422", "Invalid request parameters"],
                ["INTERNAL_ERROR", "500", "Server error"],
              ].map(([code, status, desc]) => (
                <div key={code} className="grid grid-cols-3 gap-4 p-3 border-b border-zinc-100">
                  <code className="text-red-600">{code}</code>
                  <span>{status}</span>
                  <span className="text-zinc-600">{desc}</span>
                </div>
              ))}
            </div>
            <Code title="Error response format">{`{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Caption is required"
  }
}`}</Code>
          </Section>

          {/* Platforms */}
          <Section id="platforms" title="Supported Platforms">
            <div className="space-y-3">
              {[
                { name: "Bluesky", auth: "App Password", content: "Text, Images", notes: "Generate an App Password at bsky.app → Settings → App Passwords" },
                { name: "LinkedIn", auth: "OAuth", content: "Text, Links", notes: "Requires Share on LinkedIn product" },
                { name: "Instagram", auth: "OAuth", content: "Images (required)", notes: "Must have an Instagram Business or Creator account" },
                { name: "Threads", auth: "OAuth", content: "Text, Images", notes: "Uses Meta developer app" },
                { name: "TikTok", auth: "OAuth", content: "Video (required)", notes: "Video must be MP4/H.264 with audio, min 3 seconds" },
                { name: "YouTube", auth: "OAuth", content: "Video (required)", notes: "Requires YouTube Data API v3" },
              ].map((p) => (
                <div key={p.name} className="border border-zinc-200 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-1">
                    <p className="font-semibold text-zinc-900">{p.name}</p>
                    <span className="text-xs text-zinc-500">{p.auth}</span>
                  </div>
                  <p className="text-sm text-zinc-600">Content: {p.content}</p>
                  <p className="text-xs text-zinc-400 mt-1">{p.notes}</p>
                </div>
              ))}
            </div>
          </Section>

          {/* Footer */}
          <div className="border-t border-zinc-200 pt-8 mt-16 text-sm text-zinc-500">
            <p>Need help? Contact <a href="mailto:support@unipost.dev" className="text-blue-600">support@unipost.dev</a></p>
            <p className="mt-1">
              <Link href="/terms" className="text-blue-600 hover:underline">Terms of Service</Link>
              {" · "}
              <Link href="/privacy" className="text-blue-600 hover:underline">Privacy Policy</Link>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
