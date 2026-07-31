import Link from "next/link";
import {
  ApiReferencePage,
  CodeTabs,
  DocSection,
  EndpointHeader,
  ErrorTable,
  ParamTable,
  type ParamRow,
} from "../_components/doc-components";
import { X_CREDIT_OPERATIONS, X_CREDITS_CATALOG_VERSION } from "@/data/x-credits-catalog.generated";
import { requirePublicDocsFeature } from "@/lib/public-feature-flags-server";

const RESPONSE_FIELDS: ParamRow[] = [
  { name: "mode", type: "string", required: true, description: 'Always "monthly_allowance" in the bounded-usage phase.' },
  { name: "plan_id", type: "string", required: true, description: "Workspace plan used to select the included allowance." },
  { name: "monthly_allowance", type: "integer | null", required: true, description: "Included X Credits for the billing period. Enterprise returns null for contract-defined capacity." },
  { name: "monthly_used", type: "integer", required: true, description: "Compatibility alias for monthly_effective." },
  { name: "monthly_finalized", type: "integer", required: true, description: "Settled weighted usage in the current billing period." },
  { name: "monthly_pending", type: "integer", required: true, description: "Credits currently reserved or awaiting reconciliation." },
  { name: "monthly_effective", type: "integer", required: true, description: "Finalized plus pending usage used for admission decisions." },
  { name: "monthly_remaining", type: "integer | null", required: true, description: "Remaining included X Credits. Enterprise returns null." },
  { name: "billing_period_start", type: "string", required: true, description: "ISO-8601 start of the current allowance period." },
  { name: "billing_period_end", type: "string", required: true, description: "ISO-8601 reset boundary for the current allowance period." },
  { name: "catalog_version", type: "string", required: true, description: `Operation catalog version. Current value: ${X_CREDITS_CATALOG_VERSION}.` },
  { name: "inbound_daily_usage", type: "integer", required: true, description: "Weighted inbound X usage accepted today in UTC." },
  { name: "inbound_daily_limit", type: "integer | null", required: true, description: "Daily inbound safety limit. Enterprise returns null for contract-defined capacity." },
  { name: "connection_mode_note", type: "string", required: true, description: "Explains that managed X connections consume UniPost X Credits while BYO connections do not." },
];

const RESPONSE_EXAMPLE = `{
  "data": {
    "mode": "monthly_allowance",
    "plan_id": "basic",
    "monthly_allowance": 4000,
    "monthly_used": 215,
    "monthly_finalized": 175,
    "monthly_pending": 40,
    "monthly_effective": 215,
    "monthly_remaining": 3785,
    "billing_period_start": "2026-07-01T00:00:00Z",
    "billing_period_end": "2026-08-01T00:00:00Z",
    "catalog_version": "${X_CREDITS_CATALOG_VERSION}",
    "inbound_daily_usage": 25,
    "inbound_daily_limit": 400,
    "connection_mode_note": "Managed X connections consume this allowance. Bring-your-own X API connections do not consume UniPost X Credits."
  },
  "request_id": "req_123"
}`;

const EVENT_QUERY_FIELDS: ParamRow[] = [
  { name: "account_id", type: "string", required: false, description: "Filter by connected account ID." },
  { name: "external_user_id", type: "string", required: false, description: "Filter by Managed User boundary." },
  { name: "operation", type: "string", required: false, description: "Filter by catalog operation such as user.read or post.read." },
  { name: "status", type: "string", required: false, description: "Filter by reserved, finalized, released, reconciliation_pending, or bypassed." },
  { name: "start_time", type: "string", required: false, description: "Inclusive RFC 3339 lower bound." },
  { name: "end_time", type: "string", required: false, description: "Exclusive RFC 3339 upper bound." },
  { name: "cursor", type: "string", required: false, description: "Opaque cursor from the previous response." },
  { name: "limit", type: "integer", required: false, description: "1 to 100 events. Defaults to 50." },
];

const EVENT_RESPONSE_FIELDS: ParamRow[] = [
  { name: "data[].operation_id", type: "string", required: true, description: "Stable receipt or usage-event identifier." },
  { name: "data[].account_id", type: "string", required: false, description: "Connected account associated with the operation." },
  { name: "data[].external_user_id", type: "string", required: false, description: "Managed User associated with account reads." },
  { name: "data[].operation", type: "string", required: true, description: "Versioned X Credits catalog operation." },
  { name: "data[].catalog_version", type: "string", required: true, description: "Catalog version frozen when the operation began." },
  { name: "data[].estimated", type: "integer", required: true, description: "Credits estimated before the provider call." },
  { name: "data[].reserved", type: "integer", required: true, description: "Credits held for admission." },
  { name: "data[].charged", type: "integer", required: true, description: "Final settled Credits." },
  { name: "data[].released", type: "integer", required: true, description: "Unused reservation returned to the workspace." },
  { name: "data[].status", type: "string", required: true, description: "Public settlement state." },
  { name: "data[].created_at", type: "string", required: true, description: "RFC 3339 operation creation time." },
  { name: "meta.next_cursor", type: "string", required: false, description: "Next-page cursor when more events exist." },
];

const ALLOWANCE_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/billing/x-credits" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "javascript",
    label: "JavaScript",
    code: `const allowance = await client.billing.getXCredits();

console.log(allowance.request_id);
console.log(allowance.data.monthly_remaining);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `allowance = client.billing.get_x_credits()

print(allowance.request_id)
print(allowance.data.monthly_remaining)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `allowance, err := client.Billing.GetXCredits(ctx)
if err != nil {
  log.Fatal(err)
}

fmt.Println(allowance.RequestID)
fmt.Println(allowance.Data.MonthlyRemaining)`,
  },
  {
    lang: "java",
    label: "Java",
    code: `var allowance = client.billing().getXCredits();

System.out.println(allowance.path("request_id").asText());
System.out.println(allowance.path("data").path("monthly_remaining"));`,
  },
];

const EVENT_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/billing/x-credits/events?account_id=sa_x_123&operation=post.read&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "javascript",
    label: "JavaScript",
    code: `const events = await client.billing.listXCreditEvents({
  accountId: "sa_x_123",
  operation: "post.read",
  limit: 50,
});

console.log(events.request_id);
console.log(events.data, events.meta.next_cursor);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `events = client.billing.list_x_credit_events(
    account_id="sa_x_123",
    operation="post.read",
    limit=50,
)

print(events.request_id)
print(events.data, events.meta.next_cursor)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `events, err := client.Billing.ListXCreditEvents(ctx, &unipost.ListXCreditEventsParams{
  AccountID: "sa_x_123",
  Operation: "post.read",
  Limit: 50,
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(events.RequestID)
fmt.Println(events.Data, events.Meta.NextCursor)`,
  },
  {
    lang: "java",
    label: "Java",
    code: `var events = client.billing().listXCreditEvents(Map.of(
    "account_id", "sa_x_123",
    "operation", "post.read",
    "limit", 50
));

System.out.println(events.path("request_id").asText());
System.out.println(events.path("data"));
System.out.println(events.path("meta").path("next_cursor"));`,
  },
];

export default async function XCreditsReferencePage() {
  const publicFeatureFlags = await requirePublicDocsFeature("x_credits_billing_v1");

  return (
    <ApiReferencePage
      breadcrumbItems={[{ label: "API Reference", href: "/docs/api" }, { label: "X Credits" }]}
      section="api"
      title="X Credits"
      description="Inspect the workspace's included managed-X allowance after X Credits billing is enabled. The endpoint is unavailable during the controlled rollout for regular workspaces."
    >
      <EndpointHeader
        method="GET"
        path="/v1/billing/x-credits"
        description="Returns the authenticated workspace's bounded monthly X Credits allowance."
        badges={["Bearer token", "Workspace scoped"]}
      />

      <div style={{ display: "grid", gap: 34 }}>
        <DocSection id="request" title="Request">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            X Credits billing is controlled by <code>x_credits_billing_v1</code>. Until it is enabled for the workspace,
            managed X calls do not count against or block on the customer monthly balance and this endpoint returns
            <code> FEATURE_NOT_AVAILABLE</code>. The 20 X publishes/account/day limit and internal inbound safety cap
            remain active.
          </p>
          <CodeTabs snippets={ALLOWANCE_SNIPPETS} />
        </DocSection>

        <DocSection id="response" title="Response">
          <ParamTable params={RESPONSE_FIELDS} />
          <div style={{ marginTop: 18 }}>
            <CodeTabs snippets={[{ lang: "json", label: "200", code: RESPONSE_EXAMPLE }]} />
          </div>
        </DocSection>

        <DocSection id="events" title="Credits events">
          <EndpointHeader
            method="GET"
            path="/v1/billing/x-credits/events"
            description="Lists workspace-scoped reservations and settlement outcomes for reconciliation. The ledger never includes post text, profile biographies, access tokens, or raw provider payloads."
            badges={["Bearer token", "Workspace scoped", "Cursor paginated"]}
          />
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            This endpoint uses the same <code>x_credits_billing_v1</code> gate as the allowance snapshot. Use
            <code> operation_id</code> to reconcile an API response with the ledger without logging customer content.
          </p>
          <ParamTable params={EVENT_QUERY_FIELDS} />
          <div style={{ marginTop: 18 }}>
            <CodeTabs snippets={EVENT_SNIPPETS} />
          </div>
          <div style={{ marginTop: 18 }}>
            <ParamTable params={EVENT_RESPONSE_FIELDS} />
          </div>
          <div style={{ marginTop: 18 }}>
            <CodeTabs snippets={[{
              lang: "json",
              label: "200",
              code: `{
  "data": [{
    "operation_id": "xread_01J...",
    "account_id": "sa_x_123",
    "external_user_id": "user_42",
    "operation": "post.read",
    "catalog_version": "${X_CREDITS_CATALOG_VERSION}",
    "estimated": 100,
    "reserved": 100,
    "charged": 75,
    "released": 25,
    "status": "finalized",
    "created_at": "2026-07-29T18:42:10Z",
    "updated_at": "2026-07-29T18:42:11Z",
    "finalized_at": "2026-07-29T18:42:11Z"
  }],
  "meta": { "has_more": false, "limit": 50 },
  "request_id": "req_123"
}`,
            }]} />
          </div>
        </DocSection>

        <DocSection id="operation-catalog" title="Operation catalog">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            The public catalog is versioned. X Credits are weighted units, not dollars, and are separate from the
            workspace&apos;s posts/month allowance. The table includes the shipped X Inbox read, inbound, reply, and legacy
            DM operations plus live X profile and authored-post reads.
          </p>
          <div style={{ overflowX: "auto" }}>
            <table className="docs-table">
              <thead>
                <tr><th>Operation key</th><th>Description</th><th>Credits</th></tr>
              </thead>
              <tbody>
                {X_CREDIT_OPERATIONS.filter((operation) => operation.phase === "mvp").map((operation) => (
                  <tr key={operation.key}>
                    <td><code>{operation.key}</code></td>
                    <td>{operation.label}</td>
                    <td>{operation.credits}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </DocSection>

        <DocSection id="connection-modes" title="Managed versus BYO X connections">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            Managed X connections use UniPost&apos;s X developer app and consume this allowance. Bring-your-own X API
            connections use the customer&apos;s developer credentials and do not consume UniPost X Credits. Platform-owned
            limits, abuse controls, and the independent 20-post daily X safety cap still apply to both modes.
          </p>
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            Account capabilities expose these identities as <code>unipost_managed_app</code> and
            <code> workspace_x_app</code>. X Inbox requires the Basic plan or higher. Managed inbound delivery also stops
            at <code>x_inbound_daily_cap_exceeded</code> independently of the monthly allowance.
          </p>
        </DocSection>

        <DocSection id="errors" title="Errors">
          <ErrorTable
            errors={[
              { code: "unauthorized", http: 401, description: "The request is missing valid workspace authentication." },
              { code: "feature_not_available", http: 403, description: "X Credits billing is not enabled for this workspace yet." },
              { code: "x_monthly_usage_limit_exceeded", http: 402, description: "The managed-X hard limit has been reached for this billing period. Wait for reset or upgrade/contact UniPost." },
              { code: "internal_error", http: 500, description: "The allowance snapshot could not be loaded. Retry and include request_id if contacting support." },
            ]}
          />
        </DocSection>

        <DocSection id="next" title="Next steps">
          <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 0 }}>
            Use the <Link href="/docs/guides/x/credits">X Credits guide</Link> to estimate operations and handle
            exhaustion. Continue with <Link href="/docs/guides/x/comments">X comments</Link>
            {publicFeatureFlags.x_dms_v1 ? <> or <Link href="/docs/guides/x/direct-messages">X direct messages</Link></> : null}.
            Compare included plan capacity in{" "}
            <Link href="/docs/pricing">Plans and limits</Link>.
          </p>
        </DocSection>
      </div>
    </ApiReferencePage>
  );
}
