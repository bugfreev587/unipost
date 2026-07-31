import type { Metadata } from "next";
import Link from "next/link";
import { getPublicDocsFeatureFlags } from "@/lib/public-feature-flags-server";
import { DocsCodeTabs, DocsPage } from "../../../_components/docs-shell";

export const metadata: Metadata = {
  title: "Read X profiles and post history | UniPost Docs",
  description: "Read a connected X account profile and cursor-paginate its authored post history with UniPost SDKs.",
  alternates: { canonical: "https://unipost.dev/docs/guides/x/profile-and-post-history" },
};

const PROFILE_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl --fail-with-body \\
  "https://api.unipost.dev/v1/accounts/sa_x_123/profile?external_user_id=user_42" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: profile-user-42-v1" |
  jq '{request_id, credits: .meta.credits, username: .data.username}'`,
  },
  {
    lang: "javascript",
    label: "JavaScript",
    code: `const profile = await client.accounts.getProfile("sa_x_123", {
  externalUserId: "user_42",
  idempotencyKey: "profile-user-42-v1",
});

console.log(profile.request_id, profile.meta.credits);
console.log(profile.data.username, profile.data.public_metrics);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `profile = client.accounts.get_profile(
    "sa_x_123",
    external_user_id="user_42",
    idempotency_key="profile-user-42-v1",
)

print(profile.request_id, profile.meta.credits)
print(profile.data.username, profile.data.public_metrics)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `profile, err := client.Accounts.Profile(ctx, "sa_x_123", &unipost.XAccountProfileParams{
  ExternalUserID: "user_42",
  IdempotencyKey: "profile-user-42-v1",
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(profile.RequestID, profile.Meta.Credits)
fmt.Println(profile.Data.Username, profile.Data.PublicMetrics)`,
  },
  {
    lang: "java",
    label: "Java",
    code: `var profile = client.accounts().profile("sa_x_123", Map.of(
    "external_user_id", "user_42",
    "idempotency_key", "profile-user-42-v1"
));

System.out.println(profile.path("request_id").asText());
System.out.println(profile.path("meta").path("credits"));
System.out.println(profile.path("data").path("username").asText());`,
  },
];

const POSTS_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl --fail-with-body \\
  "https://api.unipost.dev/v1/accounts/sa_x_123/posts?external_user_id=user_42&limit=20&start_time=2026-07-01T00%3A00%3A00Z&end_time=2026-08-01T00%3A00%3A00Z&exclude_reposts=true" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: posts-user-42-july-page-1" |
  jq '{request_id, credits: .meta.credits, next_cursor: .meta.next_cursor, data}'`,
  },
  {
    lang: "javascript",
    label: "JavaScript",
    code: `const page = await client.accounts.listPosts("sa_x_123", {
  externalUserId: "user_42",
  idempotencyKey: "posts-user-42-july-page-1",
  limit: 20,
  startTime: "2026-07-01T00:00:00Z",
  endTime: "2026-08-01T00:00:00Z",
  excludeReposts: true,
});

console.log(page.request_id, page.meta.credits);
console.log(page.data, page.meta.next_cursor);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `page = client.accounts.list_posts(
    "sa_x_123",
    external_user_id="user_42",
    idempotency_key="posts-user-42-july-page-1",
    limit=20,
    start_time="2026-07-01T00:00:00Z",
    end_time="2026-08-01T00:00:00Z",
    exclude_reposts=True,
)

print(page.request_id, page.meta.credits)
print(page.data, page.meta.next_cursor)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `page, err := client.Accounts.ListPosts(ctx, "sa_x_123", &unipost.XAccountPostsParams{
  ExternalUserID: "user_42",
  IdempotencyKey: "posts-user-42-july-page-1",
  Limit: 20,
  StartTime: "2026-07-01T00:00:00Z",
  EndTime: "2026-08-01T00:00:00Z",
  ExcludeReposts: true,
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(page.RequestID, page.Meta.Credits)
fmt.Println(page.Data, page.Meta.NextCursor)`,
  },
  {
    lang: "java",
    label: "Java",
    code: `var page = client.accounts().listPosts("sa_x_123", Map.of(
    "external_user_id", "user_42",
    "idempotency_key", "posts-user-42-july-page-1",
    "limit", 20,
    "start_time", "2026-07-01T00:00:00Z",
    "end_time", "2026-08-01T00:00:00Z",
    "exclude_reposts", true
));

System.out.println(page.path("request_id").asText());
System.out.println(page.path("meta").path("credits"));
System.out.println(page.path("meta").path("next_cursor").asText());`,
  },
];

const NEXT_PAGE = `const nextPage = await client.accounts.listPosts("sa_x_123", {
  externalUserId: "user_42",
  idempotencyKey: "posts-user-42-july-page-2", // New logical-page key.
  limit: 20,
  startTime: "2026-07-01T00:00:00Z",
  endTime: "2026-08-01T00:00:00Z",
  excludeReposts: true,
  cursor: page.meta.next_cursor,
});`;

export default async function XProfilePostHistoryGuidePage() {
  const publicFeatureFlags = await getPublicDocsFeatureFlags();
  const xCreditsEnabled = publicFeatureFlags.x_credits_billing_v1;

  return (
    <DocsPage
      eyebrow="X Guides"
      title="Read an X profile and authored post history"
      lead="Use a connected account to read its current X profile and walk authored posts without exposing credentials, crossing Managed User boundaries, or duplicating a paid read."
      className="docs-page-guide-redesign"
    >
      <div className="docs-guide-badges">
        <span className="docs-guide-badge">X only</span>
        <span className="docs-guide-badge">Server-side API key</span>
        <span className="docs-guide-badge">Idempotent reads</span>
        <span className="docs-guide-badge">Cursor pagination</span>
      </div>

      <h2 id="prerequisites">1. Verify the connection and ownership boundary</h2>
      <p>
        Keep the UniPost API key on your server and derive <code>external_user_id</code> from the authenticated app
        user. It must match the Managed User that owns the connected account. The connection must retain its{" "}
        <strong>persisted X app identity</strong>: UniPost-managed credentials stay managed, while a workspace-owned X
        app stays customer-owned.
      </p>
      <p>
        The profile read requires <code>users.read</code> and <code>offline.access</code>. Authored post history also
        requires <code>tweet.read</code>. Call{" "}
        <Link href="/docs/api/accounts/capabilities">Get account capabilities</Link> first. If the response reports
        <code> reconnect_required</code> or the read returns <code>ACCOUNT_REAUTHORIZATION_REQUIRED</code>, follow the{" "}
        <Link href="/docs/guides/x/reconnect-permissions">X reconnect guide</Link>.
      </p>

      <h2 id="profile">2. Read the live profile</h2>
      <p>
        Choose a caller-owned <code>Idempotency-Key</code> for this logical read. An exact retry uses the same key and
        identical account and <code>external_user_id</code>. A different read uses a new key. See the{" "}
        <Link href="/docs/api/accounts/profile">profile API Reference</Link> for the complete response envelope.
      </p>
      <DocsCodeTabs snippets={PROFILE_SNIPPETS} />

      <h2 id="posts">3. Read the first authored-post page</h2>
      <p>
        Set <code>limit</code> from 5 through 100. <code>start_time</code> is inclusive and <code>end_time</code> is
        exclusive. Reposts and replies-to-others are filtered after the upstream scan, so{" "}
        <code>meta.returned_count</code> can be smaller than <code>meta.scanned_count</code>. See the{" "}
        <Link href="/docs/api/accounts/posts">authored posts API Reference</Link> for all normalized post fields.
      </p>
      <DocsCodeTabs snippets={POSTS_SNIPPETS} />

      <h2 id="pagination">4. Continue with an opaque cursor</h2>
      <p>
        When <code>meta.next_cursor</code> is present, submit it with the same time bounds, filters, account, and Managed
        User. That cursor selects a new logical page, so give it a new logical-page key. If the network result is
        uncertain, retry that exact page with the same key. Do not decode or edit cursors, and do not reuse a key across
        a different cursor or filter set.
      </p>
      <DocsCodeTabs snippets={[{ lang: "javascript", label: "JavaScript", code: NEXT_PAGE }]} />
      <p className="docs-guide-note">
        Cursors expire. <code>INVALID_CURSOR</code> means restart from a new first page. If a retriable error includes
        <code>error.details.retry_cursor</code> and <code>retry_cursor_expires_at</code>, use that cursor with the same
        logical-page key.
      </p>

      <h2 id="errors">5. Handle retries by stable error code</h2>
      <p>
        Read <code>error.code</code>, <code>error.is_retriable</code>, the HTTP <code>Retry-After</code> header, and
        <code>request_id</code>. Preserve the original key and exact parameters for retriable states; never rotate a key
        merely because a timeout made the outcome uncertain.
      </p>
      <ul className="docs-checklist">
        <li><code>VALIDATION_ERROR</code> or <code>IDEMPOTENCY_KEY_REQUIRED</code>: correct the request before sending a new logical operation.</li>
        <li><code>ACCOUNT_NOT_FOUND</code>, <code>WRONG_PLATFORM</code>, or <code>ACCOUNT_ACCESS_DENIED</code>: re-check the account and Managed User boundary.</li>
        <li><code>ACCOUNT_REAUTHORIZATION_REQUIRED</code>: reconnect the X account with the required scopes.</li>
        <li><code>INVALID_CURSOR</code>: start a fresh traversal unless a supplied retry cursor is still valid.</li>
        <li><code>IDEMPOTENCY_CONFLICT</code>: the key was reused with different inputs; create a new logical key for the corrected request.</li>
        <li><code>READ_IN_PROGRESS</code> or <code>READ_SETTLEMENT_PENDING</code>: wait for <code>Retry-After</code>, then retry with the same key.</li>
        <li><code>RATE_LIMITED</code> or <code>X_UPSTREAM_ERROR</code>: retry only when <code>is_retriable</code> is true.</li>
      </ul>

      <h2 id="credits">6. Inspect Credits policy and receipts</h2>
      {xCreditsEnabled ? (
        <>
          <p>
            For a UniPost-managed X connection, the service estimates and reserves Credits before calling X. A profile
            read charges the catalog <code>user.read</code> amount; a post page estimates from <code>limit</code> and
            settles from <code>scanned_count</code>. Inspect <code>meta.credits</code> and retain{" "}
            <code>request_id</code> for reconciliation.
          </p>
          <p>
            <code>INSUFFICIENT_X_CREDITS</code> is denied before the provider call. Use{" "}
            <code>error.details.max_affordable_limit</code> only to plan a smaller new logical request with a new key;
            UniPost never silently lowers the submitted limit. Check the{" "}
            <Link href="/docs/api/x-credits">X Credits allowance API</Link> before large batches.
          </p>
        </>
      ) : (
        <p>
          Customer X Credits accounting is disabled by the current rollout flag. Reads remain available and return{" "}
          <code>meta.credits.accounting_enabled=false</code> with <code>bypass_reason=feature_disabled</code>.
          Workspace-owned X credentials return <code>bypass_reason=customer_x_app</code>. These bypass states do not
          remove X provider rate limits or authorization requirements.
        </p>
      )}

      <h2 id="related">Related references</h2>
      <div className="docs-guide-next">
        <Link href="/docs/api/accounts/profile" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">API Reference</div>
          <div className="docs-guide-next-title">Get X account profile</div>
          <div className="docs-guide-next-body">Request, response, idempotency, and stable errors.</div>
        </Link>
        <Link href="/docs/api/accounts/posts" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">API Reference</div>
          <div className="docs-guide-next-title">List X authored posts</div>
          <div className="docs-guide-next-body">Filters, cursor contract, normalized fields, and retry metadata.</div>
        </Link>
        <Link href="/docs/api/accounts/capabilities" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Preflight</div>
          <div className="docs-guide-next-title">Get account capabilities</div>
          <div className="docs-guide-next-body">Authorization, reconnect state, and account-read policy.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
